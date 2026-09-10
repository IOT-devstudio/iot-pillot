package service

import (
	"context"
	"errors"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

type AuthUseCase struct {
	userRepo     repository.UserRepo
	tokenManager *utils.TokenManager
	codeManager  *utils.CodeManager
}

func NewAuthUseCase(repo repository.UserRepo, tokenManager *utils.TokenManager, codeManager *utils.CodeManager) *AuthUseCase {
	return &AuthUseCase{
		userRepo:     repo,
		tokenManager: tokenManager,
		codeManager:  codeManager,
	}
}

func (auc *AuthUseCase) Login(ctx context.Context, req *request.LoginReq) (*response.LoginResp, error) {
	user, err := auc.userRepo.GetByName(ctx, req.Username)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if !utils.CheckPassword(req.Password, user.Password) {
		return nil, errors.New("password incorrect")
	}
	accessToken, refreshToken, err := auc.tokenManager.GenerateTokens(ctx, user.ID, user.Name, "user")
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
	accessToken, refreshToken, err := auc.tokenManager.GenerateTokens(ctx, user.ID, user.Name, "user")
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
