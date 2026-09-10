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
// 注意：唯一码的 TTL 决定了 token 的有效上限，这里取 refresh 有效期
// （默认 7 天），避免像 Java 示例那样把唯一码 TTL 设成 1 小时导致
// 所有 token 实际寿命被压缩到 1 小时。

// uniqueKey 会话唯一码在 Redis 中的 key
func (tm *TokenManager) uniqueKey(userID int) string {
	return fmt.Sprintf("jwt:unique:%d", userID)
}

func generateUniqueCode() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// storeUnique 写入（或替换）用户会话唯一码
func (tm *TokenManager) storeUnique(ctx context.Context, userID int, uniqueCode string) error {
	cmd := tm.redis.B().Set().Key(tm.uniqueKey(userID)).Value(uniqueCode).ExSeconds(int64(tm.Expire)).Build()
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

	accessToken, err = generateToken(userID, username, role, "access", tm.Expire, tm.Secret, uniqueCode)
	if err != nil {
		return "", "", err
	}
	refreshToken, err = generateToken(userID, username, role, "refresh", tm.Expire, tm.Secret, uniqueCode)
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

// RefreshAccessToken 轮换 refresh token：校验通过后生成新令牌，
// 并替换会话唯一码，使旧 token 全部立即失效。
func (tm *TokenManager) RefreshAccessToken(ctx context.Context, refreshToken string) (newAccessToken string, newRefreshToken string, err error) {
	userID, username, role, err := tm.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		return "", "", err
	}
	return tm.GenerateTokens(ctx, userID, username, role)
}

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
