package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// defaultRecruitmentParse 新注册用户写入的招新轮次。
//
// 取值必须**非 0**：按 domain 的约定 0 表示"工作室成员"，而新注册的人显然是报名者。
// 原实现不设置该字段，落库即 0，于是每个报名者都被记成工作室成员。
// 换下一轮招新时改这一个常量即可（或将来改成从配置读）。
const defaultRecruitmentParse = 1

// TokenIssuer 认证服务对"签发与校验令牌"的全部依赖。
//
// 收窄成接口的直接收益是可测：真实实现要连 Redis 写会话唯一码，
// 而角色判定与登录失败的锁定逻辑都不该因此变成"只能靠实机验证"。
// *utils.TokenManager 天然满足它，wire 需要显式 Bind。
type TokenIssuer interface {
	GenerateTokens(ctx context.Context, userID int, username string, role string) (accessToken string, refreshToken string, err error)
	ValidateRefreshToken(ctx context.Context, tokenString string) (userID int, username string, role string, err error)
	RevokeSession(ctx context.Context, accessToken string, refreshToken string) error
}

// NewTokenIssuer 把 *utils.TokenManager 作为 TokenIssuer 提供给装配层。
//
// 为什么转换函数在 service 而不是 utils：接口是**消费者**声明的（这里收窄是因为
// 单测要注入桩），而 utils 反过来 import service 会成环 —— 只有 service 能同时看到两边。
func NewTokenIssuer(manager *utils.TokenManager) TokenIssuer {
	return manager
}

// VerifyCodeChecker 认证服务对验证码的全部依赖。
type VerifyCodeChecker interface {
	VerifyCode(ctx context.Context, verifier string, code string) error
	SendVerifyCode(ctx context.Context, verifier string, verifierType utils.VerifierType) error
}

// NewVerifyCodeChecker 与 NewTokenIssuer 同理，只是换了 CodeManager。
func NewVerifyCodeChecker(manager *utils.CodeManager) VerifyCodeChecker {
	return manager
}

// AuthUseCase 只负责"谁能登录"。
//
// 管理端的用户列表、角色变更都在 AdminUseCase（admin_usec.go）：
// 认证与管理是两件事，混在一起会让登录逻辑反复被无关改动牵连。
type AuthUseCase struct {
	userRepo     repository.UserRepo
	tokenManager TokenIssuer
	codeManager  VerifyCodeChecker
	admins       utils.AdminDirectory
	limiter      *utils.Limiter
}

func NewAuthUseCase(
	repo repository.UserRepo,
	tokenManager TokenIssuer,
	codeManager VerifyCodeChecker,
	admins utils.AdminDirectory,
	limiter *utils.Limiter,
) *AuthUseCase {
	return &AuthUseCase{
		userRepo:     repo,
		tokenManager: tokenManager,
		codeManager:  codeManager,
		admins:       admins,
		limiter:      limiter,
	}
}

/* ------------------------------------------------------------------ *
 * 对外错误
 * ------------------------------------------------------------------ */

// ErrInvalidCredentials 登录凭据错误的**唯一**对外错误。
//
// 刻意不区分"用户不存在"与"密码错误"：区分开就等于提供用户名枚举接口，
// 攻击者可以先确认哪些账号存在再定点爆破。
var ErrInvalidCredentials = errors.New("用户名或密码错误")

// ErrAccountLocked 连续失败次数过多，账号被临时锁定。
var ErrAccountLocked = errors.New("登录失败次数过多，请 15 分钟后再试或重置密码")

// ErrEmailTaken 邮箱已被注册。
var ErrEmailTaken = errors.New("该邮箱已被注册，请直接登录或更换邮箱")

/* ------------------------------------------------------------------ *
 * 角色
 * ------------------------------------------------------------------ */

// RoleOf 返回用户当前的真实角色。
//
// 角色真源是 Redis 里的管理员名单（userID 集合），不是 JWT 里的 role claim：
// claim 是签发时刻的快照，撤销管理员后旧令牌仍会声称 admin。
// 接口层用它回答 /me，保证前端看到的与服务端强制的完全一致。
func (auc *AuthUseCase) RoleOf(ctx context.Context, userID int) (string, error) {
	return auc.roleFor(ctx, userID)
}

func (auc *AuthUseCase) roleFor(ctx context.Context, userID int) (string, error) {
	isAdmin, err := auc.admins.IsAdmin(ctx, userID)
	if err != nil {
		// 读失败时把错误透出去而不是降级成 member：静默降级会让管理员
		// "忽然进不去管理端"且没有任何线索；而登录本来就要写 Redis 会话唯一码，
		// Redis 挂了登录也成不了。
		return "", fmt.Errorf("无法确认账号角色: %w", err)
	}
	if isAdmin {
		return middleware.RoleAdmin, nil
	}
	return middleware.RoleMember, nil
}

/* ------------------------------------------------------------------ *
 * 登录
 * ------------------------------------------------------------------ */

// findByLogin 按"用户名或邮箱"查用户。
//
// 两种标识都支持的原因：用户名唯一约束已移除（唯一性改到邮箱上），
// 重名账号必须有一条可靠的登录路径，而邮箱保证唯一。
// 用户名命中多行时返回 ErrAmbiguousUsername，由调用方提示改用邮箱。
func (auc *AuthUseCase) findByLogin(ctx context.Context, identifier string) (*domain.User, error) {
	user, err := auc.userRepo.GetByName(ctx, identifier)
	if err == nil && user != nil {
		return user, nil
	}
	if errors.Is(err, repository.ErrAmbiguousUsername) {
		return nil, repository.ErrAmbiguousUsername
	}
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	// 用户名没命中，再按邮箱试一次
	byEmail, emailErr := auc.userRepo.GetByEmail(ctx, identifier)
	if emailErr == nil && byEmail != nil {
		return byEmail, nil
	}
	if emailErr != nil && !errors.Is(emailErr, repository.ErrUserNotFound) {
		return nil, emailErr
	}

	return nil, nil
}

func (auc *AuthUseCase) Login(ctx context.Context, req *request.LoginReq) (*response.LoginResp, error) {
	identifier := strings.TrimSpace(req.Username)
	failureKey := utils.LoginFailureKey(strings.ToLower(identifier))

	// 锁定期内直接拒绝，连密码都不比对：否则锁定只是"多错几次"的提示，
	// 爆破仍然在跑。
	if count, err := auc.limiter.FailureCount(ctx, failureKey); err == nil && count >= utils.LoginMaxFailures {
		return nil, ErrAccountLocked
	}

	user, err := auc.findByLogin(ctx, identifier)
	if err != nil {
		return nil, err
	}

	// 用户不存在与密码错误走同一分支、返回同一个错误
	if user == nil || !utils.CheckPassword(req.Password, user.Password) {
		auc.recordLoginFailure(ctx, failureKey)
		return nil, ErrInvalidCredentials
	}

	role, err := auc.roleFor(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := auc.tokenManager.GenerateTokens(ctx, user.ID, user.Name, role)
	if err != nil {
		return nil, err
	}

	// 登录成功清零失败计数，避免正常用户被自己的历史手误拖累
	_ = auc.limiter.ClearFailures(ctx, failureKey)

	return &response.LoginResp{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
	}, nil
}

// recordLoginFailure 记一次失败并检查是否达到锁定阈值。
//
// 记失败本身失败（Redis 故障）时只放弃计数，不让登录流程返回 500：
// 记不上计数的最坏结果是"锁定没生效"，而拒绝登录会让所有人都进不来。
// 这种降级是刻意的，且会在日志里留下痕迹。
func (auc *AuthUseCase) recordLoginFailure(ctx context.Context, failureKey string) {
	_, locked, err := auc.limiter.RecordFailure(ctx, failureKey, utils.LoginMaxFailures, utils.LoginLockWindow)
	if err != nil {
		log.Printf("[auth] 记录登录失败次数失败（本轮锁定未生效）: %v", err)
		return
	}
	if locked {
		log.Printf("[auth] 账号 %s 连续失败达到 %d 次，已锁定 %s",
			failureKey, utils.LoginMaxFailures, utils.LoginLockWindow)
	}
}

/* ------------------------------------------------------------------ *
 * 注册
 * ------------------------------------------------------------------ */

func (auc *AuthUseCase) Register(ctx context.Context, req *request.RegisterReq) (*response.LoginResp, error) {
	email := strings.TrimSpace(req.Email)

	// 邮箱唯一性先查一次，给出可读提示；数据库层的唯一索引负责并发兜底。
	// 顺序刻意放在校验验证码之前：邮箱已注册就不该消耗掉用户的验证码。
	if existing, err := auc.userRepo.GetByEmail(ctx, email); err == nil && existing != nil {
		return nil, ErrEmailTaken
	} else if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	if err := auc.codeManager.VerifyCode(ctx, email, req.Code); err != nil {
		return nil, err
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := domain.User{
		Name:     req.Name,
		Password: hashedPassword,
		Detail:   domain.Detail{Email: email},
		// 新注册的人不是工作室成员，必须显式写轮次（见 defaultRecruitmentParse）
		Parse: defaultRecruitmentParse,
	}
	if err := auc.userRepo.Save(ctx, &user); err != nil {
		if repository.IsDuplicateKey(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	role, err := auc.roleFor(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := auc.tokenManager.GenerateTokens(ctx, user.ID, user.Name, role)
	if err != nil {
		return nil, err
	}

	return &response.LoginResp{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
	}, nil
}

/* ------------------------------------------------------------------ *
 * 刷新与登出
 * ------------------------------------------------------------------ */

// Refresh 用 refresh 令牌换一对新令牌。
//
// 刻意不调用 utils.TokenManager.RefreshAccessToken：那个方法会把**旧令牌里的 role**
// 原样带进新令牌，于是被撤销管理员的人只要刷新一次就能拿到一张声称 admin 的令牌。
// 这里先从 Redis 取真角色再签发，刷新后的令牌与服务端判定永远一致。
func (auc *AuthUseCase) Refresh(ctx context.Context, refreshToken string) (*response.RefreshResp, error) {
	userID, username, _, err := auc.tokenManager.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	role, err := auc.roleFor(ctx, userID)
	if err != nil {
		return nil, err
	}

	accessToken, newRefreshToken, err := auc.tokenManager.GenerateTokens(ctx, userID, username, role)
	if err != nil {
		return nil, err
	}

	return &response.RefreshResp{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (auc *AuthUseCase) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	return auc.tokenManager.RevokeSession(ctx, accessToken, refreshToken)
}

func (auc *AuthUseCase) SendVerifyCode(ctx context.Context, verifier string, verifierType string) error {
	return auc.codeManager.SendVerifyCode(ctx, verifier, utils.VerifierType(verifierType))
}
