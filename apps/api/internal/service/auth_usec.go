package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// AuthUseCase 只负责"谁能登录"。
//
// 管理端的用户列表、角色变更都在 AdminUseCase（admin_usec.go）：
// 认证与管理是两件事，混在一起会让登录逻辑反复被无关改动牵连。
type AuthUseCase struct {
	userRepo     repository.UserRepo
	tokenManager *utils.TokenManager
	codeManager  *utils.CodeManager
	admins       utils.AdminDirectory
}

func NewAuthUseCase(repo repository.UserRepo, tokenManager *utils.TokenManager, codeManager *utils.CodeManager, admins utils.AdminDirectory) *AuthUseCase {
	return &AuthUseCase{
		userRepo:     repo,
		tokenManager: tokenManager,
		codeManager:  codeManager,
		admins:       admins,
	}
}

// roleFor 决定写进 JWT 的角色。
//
// 角色真源是 Redis 里的管理员名单（userID 集合），**不是**配置里的用户名白名单：
// 配置只用于启动引导（见 AdminUseCase.SeedAdmins）。
//
// 读失败时把错误透出去而不是降级成 member：静默降级会让管理员"忽然进不去管理端"
// 且没有任何线索；而登录本来就要写 Redis 会话唯一码，Redis 挂了登录也成不了。
func (auc *AuthUseCase) roleFor(ctx context.Context, userID int) (string, error) {
	isAdmin, err := auc.admins.IsAdmin(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("无法确认账号角色: %w", err)
	}
	if isAdmin {
		return middleware.RoleAdmin, nil
	}
	return middleware.RoleMember, nil
}

func (auc *AuthUseCase) Login(ctx context.Context, req *request.LoginReq) (*response.LoginResp, error) {
	user, err := auc.userRepo.GetByName(ctx, req.Username)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if !utils.CheckPassword(req.Password, user.Password) {
		return nil, errors.New("password incorrect")
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

func (auc *AuthUseCase) Register(ctx context.Context, req *request.RegisterReq) (*response.LoginResp, error) {
	if err := auc.codeManager.VerifyCode(ctx, req.Email, req.Code); err != nil {
		return nil, err
	}
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := domain.User{
		Name:     req.Name,
		Password: hashedPassword,
		Detail:   domain.Detail{Email: req.Email},
	}
	if err := auc.userRepo.Save(ctx, &user); err != nil {
		return nil, err
	}

	// 新注册的账号不可能是管理员（名单是空的，或至少没有这个新 ID），
	// 但仍走同一个 roleFor：角色只有一个判定入口，避免"注册这条路忘了同步"。
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

func (auc *AuthUseCase) Refresh(ctx context.Context, refreshToken string) (*response.LoginResp, error) {
	accessToken, newRefreshToken, err := auc.tokenManager.RefreshAccessToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return &response.LoginResp{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		UserID:       -1,
	}, nil
}

func (auc *AuthUseCase) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	return auc.tokenManager.RevokeSession(ctx, accessToken, refreshToken)
}

func (auc *AuthUseCase) SendVerifyCode(ctx context.Context, verifier string, verifierType string) error {
	return auc.codeManager.SendVerifyCode(ctx, verifier, utils.VerifierType(verifierType))
}
