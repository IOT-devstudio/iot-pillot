package service

import (
	"context"
	"errors"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

type AuthUseCase struct {
	userRepo     repository.UserRepo
	tokenManager *utils.TokenManager
	codeManager  *utils.CodeManager
	authConfig   *config.AuthConfig
}

func NewAuthUseCase(repo repository.UserRepo, tokenManager *utils.TokenManager, codeManager *utils.CodeManager, authConfig *config.AuthConfig) *AuthUseCase {
	return &AuthUseCase{
		userRepo:     repo,
		tokenManager: tokenManager,
		codeManager:  codeManager,
		authConfig:   authConfig,
	}
}

// roleFor 决定用户名对应的角色。
//
// 角色来自部署配置里的白名单（apps/api/internal/config），不落库：
// 改角色 = 改配置 + 重启，走发布流程，天然可审计。
// 不在白名单里的人一律是 member —— 默认最小权限，而不是默认放行。
func (auc *AuthUseCase) roleFor(username string) string {
	if auc.authConfig.IsAdmin(username) {
		return middleware.RoleAdmin
	}
	return middleware.RoleMember
}

func (auc *AuthUseCase) Login(ctx context.Context, req *request.LoginReq) (*response.LoginResp, error) {
	user, err := auc.userRepo.GetByName(ctx, req.Username)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if !utils.CheckPassword(req.Password, user.Password) {
		return nil, errors.New("password incorrect")
	}
	accessToken, refreshToken, err := auc.tokenManager.GenerateTokens(ctx, user.ID, user.Name, auc.roleFor(user.Name))
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
	}
	if err := auc.userRepo.Save(ctx, &user); err != nil {
		return nil, err
	}
	accessToken, refreshToken, err := auc.tokenManager.GenerateTokens(ctx, user.ID, user.Name, auc.roleFor(user.Name))
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

// ListUsers 分页读取用户列表（管理端专用）。
//
// 只映射出 id / name / created_at：密码哈希绝不能出现在任何响应里，
// 所以这里显式构造 DTO，而不是把 domain.User 直接交给序列化。
func (auc *AuthUseCase) ListUsers(ctx context.Context, page int, pageSize int) (*response.AdminUserListResp, error) {
	users, total, err := auc.userRepo.GetAll(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]response.AdminUserResp, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		items = append(items, response.AdminUserResp{
			UserID:    user.ID,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		})
	}

	return &response.AdminUserListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
