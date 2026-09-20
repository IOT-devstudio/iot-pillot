package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/rueidis"
	"golang.org/x/crypto/bcrypt"
)

type TokenManager struct {
	*config.JWTConfig
	redis rueidis.Client
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPassword(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func NewTokenManager(redis rueidis.Client, cfg *config.Config) *TokenManager {
	return &TokenManager{
		JWTConfig: cfg.JWT,
		redis:     redis,
	}
}

// ==================== 会话唯一码（版本号） ====================
//
// 采用"每用户一个版本号"设计（类似 Java jjwt 的 uniqueCode 方案）：
//   - Redis 只保存一个 key：jwt:unique:{userID}，value 为随机唯一码；
//   - JWT 的 subject 携带该唯一码；
//   - 校验时比对 subject 与 Redis 中的唯一码，不一致即认为 token 已失效；
//   - 每次登录/刷新都会替换唯一码 → 旧 token 全部立即失效（单端登录），
//     登出只需删除该 key。
//
// 相比"每个 token 一个 key"（jwt:access:{token}）的方案，避免同一用户
// 多次登录积累大量 key，且天然支持一键踢掉该用户全部会话。
//
// 注意：唯一码的 TTL 决定了 token 的有效上限，必须取 refresh 有效期（见
// refreshTTLSeconds / storeUnique），否则 30 天有效的 refresh 令牌会因为
// 唯一码先过期而提前失效。
//
// 提醒：唯一码是"每用户一个"，所以同一账号在新设备登录会把旧设备顶下线，
// 这是单端登录的预期行为。

// refreshTokenTTLMultiplier refresh 令牌相对 access 的倍数。
//
// jwt.expire 是 access 的有效期（默认 3600 秒 = 1 小时），refresh 取它的 720 倍
// （= 30 天）。想改这两个量就改这里和 jwt.expire，不要在别处再写乘数。
const refreshTokenTTLMultiplier = 720

// accessTTLSeconds access 令牌有效期（秒）。
func (tm *TokenManager) accessTTLSeconds() int {
	return tm.Expire
}

// refreshTTLSeconds refresh 令牌有效期（秒），同时也是会话唯一码的 TTL。
func (tm *TokenManager) refreshTTLSeconds() int {
	return tm.Expire * refreshTokenTTLMultiplier
}

// uniqueKey 会话唯一码在 Redis 中的 key
func (tm *TokenManager) uniqueKey(userID int) string {
	return fmt.Sprintf("jwt:unique:%d", userID)
}

func generateUniqueCode() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// storeUnique 写入（或替换）用户会话唯一码。
//
// TTL 必须取 **refresh 有效期**而不是 access 有效期：
// 校验令牌时要比对 Redis 里的唯一码，如果这个键只活 1 小时，那么 30 天有效的
// refresh 令牌在 1 小时后就会因为"查不到唯一码"而失效——refresh 形同废纸，
// 用户每小时被强制登出。（原来的实现正是如此：键 1h、注释却写着 7 天。）
func (tm *TokenManager) storeUnique(ctx context.Context, userID int, uniqueCode string) error {
	cmd := tm.redis.B().Set().
		Key(tm.uniqueKey(userID)).
		Value(uniqueCode).
		ExSeconds(int64(tm.refreshTTLSeconds())).
		Build()
	return tm.redis.Do(ctx, cmd).Error()
}

// ==================== 令牌生成 ====================

// GenerateTokens 生成双令牌：每次调用都会替换用户的会话唯一码，
// 使该用户之前签发的所有 token 立即失效（单端登录）。
func (tm *TokenManager) GenerateTokens(ctx context.Context, userID int, username string, role string) (accessToken string, refreshToken string, err error) {
	uniqueCode := generateUniqueCode()
	if err := tm.storeUnique(ctx, userID, uniqueCode); err != nil {
		return "", "", fmt.Errorf("写入会话唯一码失败: %w", err)
	}

	accessToken, err = generateToken(userID, username, role, "access", tm.accessTTLSeconds(), tm.Secret, uniqueCode)
	if err != nil {
		return "", "", err
	}
	refreshToken, err = generateToken(userID, username, role, "refresh", tm.refreshTTLSeconds(), tm.Secret, uniqueCode)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func generateToken(userID int, username string, role string, tokenType string, expireSeconds int, secret string, uniqueCode string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"type":     tokenType,
		"sub":      uniqueCode, // 会话唯一码：与 Redis jwt:unique:{userID} 比对
		"exp":      time.Now().Add(time.Duration(expireSeconds) * time.Second).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ==================== 令牌校验 ====================

// parseAndValidateToken 仅校验 JWT 签名 / 类型 / 过期，返回 claims 信息（不做 Redis 比对）
func parseAndValidateToken(tokenString string, secret string, expectedType string) (int, string, string, string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("未知的签名方法: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, "", "", "", errors.New("token 已过期")
		}
		return 0, "", "", "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, "", "", "", errors.New("无效的 token")
	}

	if claims["type"] != expectedType {
		return 0, "", "", "", errors.New("token 类型不匹配")
	}

	userIDFloat, okID := claims["user_id"].(float64)
	username, okName := claims["username"].(string)
	role, _ := claims["role"].(string)
	sub, _ := claims["sub"].(string)
	if !okID || !okName {
		return 0, "", "", "", errors.New("token 缺少必要的用户信息字段")
	}

	return int(userIDFloat), username, role, sub, nil
}

// validateToken 完整校验：JWT 签名/类型/过期 + Redis 会话唯一码比对
// （唯一码不一致说明该用户已重新登录或已登出，token 失效）
func (tm *TokenManager) validateToken(ctx context.Context, tokenString string, expectedType string) (int, string, string, error) {
	userID, username, role, sub, err := parseAndValidateToken(tokenString, tm.Secret, expectedType)
	if err != nil {
		return 0, "", "", err
	}

	getCmd := tm.redis.B().Get().Key(tm.uniqueKey(userID)).Build()
	current, err := tm.redis.Do(ctx, getCmd).ToString()
	if err != nil || current == "" || current != sub {
		return 0, "", "", errors.New("token 已失效（会话已登出或已被新登录顶替）")
	}
	return userID, username, role, nil
}

// ValidateAccessToken 校验 access token（含 Redis 会话唯一码比对）
func (tm *TokenManager) ValidateAccessToken(ctx context.Context, tokenString string) (int, string, string, error) {
	return tm.validateToken(ctx, tokenString, "access")
}

// ValidateRefreshToken 校验 refresh token（含 Redis 会话唯一码比对）
func (tm *TokenManager) ValidateRefreshToken(ctx context.Context, tokenString string) (int, string, string, error) {
	return tm.validateToken(ctx, tokenString, "refresh")
}

// ==================== 刷新与登出 ====================
//
// 这里刻意**没有** RefreshAccessToken（原来的实现）：
// 它把旧令牌里的 role 原样带进新令牌，于是被撤销管理员的人只要刷新一次
// 就能拿到一张声称 admin 的令牌。
// 刷新逻辑现在放在 service.AuthUseCase.Refresh：先从 Redis 取真角色再签发。

// RevokeToken 撤销单个 token（兼容旧接口，仅删除会话唯一码使该用户全部 token 失效）
func (tm *TokenManager) RevokeToken(ctx context.Context, token string) error {
	// 从 token 中解析 userID（仅 JWT 解析，不做 Redis 比对）
	userID, _, _, _, err := parseAndValidateToken(token, tm.Secret, "access")
	if err != nil {
		return err
	}
	return tm.revokeByUserID(ctx, userID)
}

// RevokeSession 完整登出：删除用户的会话唯一码，其所有 token 立即失效。
// 从 access/refresh token 中解析出 userID（任一可用即可）。
func (tm *TokenManager) RevokeSession(ctx context.Context, accessToken string, refreshToken string) error {
	userID := -1
	if uid, _, _, _, err := parseAndValidateToken(accessToken, tm.Secret, "access"); err == nil {
		userID = uid
	} else if uid, _, _, _, err := parseAndValidateToken(refreshToken, tm.Secret, "refresh"); err == nil {
		userID = uid
	}
	if userID < 0 {
		return errors.New("token 无效，无法登出")
	}
	return tm.revokeByUserID(ctx, userID)
}

func (tm *TokenManager) revokeByUserID(ctx context.Context, userID int) error {
	delCmd := tm.redis.B().Del().Key(tm.uniqueKey(userID)).Build()
	return tm.redis.Do(ctx, delCmd).Error()
}

// RevokeUserSessions 撤销某用户的**全部**会话（删除会话唯一码），其所有令牌立即失效。
//
// 用途：角色变更（提升/撤销管理员）后必须让旧令牌失效。
// JWT 里的 role claim 是签发时写死的，旧令牌会一直声称"我是 member"（或反过来
// 一直声称 admin），前端守卫据此判断就会与服务端不一致。
// 强制重新登录后拿到的令牌才与 Redis 里的真实角色一致。
func (tm *TokenManager) RevokeUserSessions(ctx context.Context, userID int) error {
	return tm.revokeByUserID(ctx, userID)
}

// IsTokenInRedis 检查 token 是否有效（JWT 解析 + Redis 会话唯一码比对）。
// 保留此方法供需要单独判断 token 状态的地方使用。
func (tm *TokenManager) IsTokenInRedis(ctx context.Context, token string) bool {
	_, _, _, err := tm.ValidateAccessToken(ctx, token)
	return err == nil
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
