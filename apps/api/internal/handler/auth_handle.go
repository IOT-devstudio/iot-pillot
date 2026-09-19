package handler

import (
	"errors"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthUseCase
	limiter     *utils.Limiter
}

func NewAuthHandler(authService *service.AuthUseCase, limiter *utils.Limiter) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		limiter:     limiter,
	}
}

// Login 用户登录接口
// @Summary 用户登录
// @Description 通过用户名**或邮箱** + 密码登录，返回双令牌（access_token + refresh_token）。
// 用户名不唯一时请用邮箱登录。
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.LoginReq true "登录请求参数"
// @Success 200 {object} response.Result{data=response.LoginResp} "成功返回双令牌"
// @Failure 400 {object} response.Result "请求参数错误"
// @Failure 401 {object} response.Result "用户名或密码错误（不区分二者）"
// @Router /api/v1/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var dto request.LoginReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	token, err := h.authService.Login(c.Request.Context(), &dto)
	if err != nil {
		failAuth(c, err)
		return
	}

	response.OK(c, token)
}

// Register 用户注册接口
// @Summary 用户注册
// @Description 注册新用户，返回双令牌。邮箱唯一；必须先通过 /send-verify-code 取得验证码。
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.RegisterReq true "注册请求参数"
// @Success 200 {object} response.Result{data=response.LoginResp} "成功返回双令牌"
// @Failure 400 {object} response.Result "参数错误、邮箱已注册或验证码错误"
// @Router /api/v1/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var dto request.RegisterReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	token, err := h.authService.Register(c.Request.Context(), &dto)
	if err != nil {
		failAuth(c, err)
		return
	}
	response.OK(c, token)
}

// Refresh 刷新令牌接口
// @Summary 刷新令牌
// @Description 使用 refresh token 获取新的一对令牌，旧令牌立即失效（单端登录）。不返回用户 ID。
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.RefreshReq true "刷新令牌请求参数"
// @Success 200 {object} response.Result{data=response.RefreshResp} "成功返回新的双令牌"
// @Failure 400 {object} response.Result "请求参数错误"
// @Failure 401 {object} response.Result "refresh token 无效或已失效"
// @Router /api/v1/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req request.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		failAuth(c, err)
		return
	}
	response.OK(c, result)
}

// SendVerifyCode 发送验证码接口
// @Summary 发送验证码接口
// @Description 发送邮箱验证码，5 分钟内有效、只能使用一次、猜错 5 次即作废。
// 同一邮箱 60 秒冷却，同一 IP 每小时 10 次。
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.VerifyCodeReq true "验证目标"
// @Success 200 {object} response.Result "发送验证码成功"
// @Failure 400 {object} response.Result "请求参数错误"
// @Failure 429 {object} response.Result "请求过于频繁"
// @Router /api/v1/send-verify-code [post]
func (h *AuthHandler) SendVerifyCode(c *gin.Context) {
	var req request.VerifyCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	// 冷却 + 配额两道限流都放在发信之前：这个接口未认证即可调用，
	// 无限流就能被用来对任意邮箱刷信（烧 SMTP 配额、毁发信信誉）。
	allowed, err := h.limiter.AllowOnce(ctx, utils.VerifyCooldownKey(req.Verifier), utils.VerifyResendCooldown)
	if err != nil {
		failInternal(c, err)
		return
	}
	if !allowed {
		response.FailTooManyRequests(c, "验证码已发送，请 60 秒后再试")
		return
	}

	allowed, err = h.limiter.AllowWithin(
		ctx,
		utils.VerifyIPQuotaKey(c.ClientIP()),
		int64(utils.VerifyIPHourlyQuota),
		utils.VerifyIPQuotaWindow,
	)
	if err != nil {
		failInternal(c, err)
		return
	}
	if !allowed {
		response.FailTooManyRequests(c, "请求过于频繁，请稍后再试")
		return
	}

	if err := h.authService.SendVerifyCode(ctx, req.Verifier, req.VerifierType); err != nil {
		failInternal(c, err)
		return
	}
	response.OKWithMsg(c, "发送验证码成功", nil)
}

// Logout 用户退出登录接口
// @Summary 用户退出登录接口
// @Description 退出登录接口（需要 access token 和 refresh token）
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.RefreshReq true "退出登录请求参数（refresh_token）"
// @Success 200 {object} response.Result "退出登录成功"
// @Failure 400 {object} response.Result "请求参数错误"
// @Router /api/v1/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	var req request.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	err := h.authService.Logout(c.Request.Context(), accessToken, req.RefreshToken)
	if err != nil {
		// "token 无效，无法登出" 是域错误，可以外发；Redis 故障走脱敏
		if strings.Contains(err.Error(), "token 无效") {
			response.FailInvalidParam(c, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OKWithMsg(c, "退出登录成功", nil)
}

// Me 返回当前登录用户的身份与角色
// @Summary 获取当前登录用户
// @Description 角色由服务端**现查 Redis 管理员名单**得到，而不是回显令牌里的快照，
// 因此前端可见性与服务端强制永远一致（撤销管理员后立刻反映）。
// @Tags 认证模块
// @Produce json
// @Success 200 {object} response.Result{data=response.MeResp} "成功"
// @Failure 401 {object} response.Result "未认证或令牌失效"
// @Router /api/v1/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, username, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "缺少认证信息")
		return
	}

	role, err := h.authService.RoleOf(c.Request.Context(), userID)
	if err != nil {
		failInternal(c, err)
		return
	}

	response.OK(c, response.MeResp{
		UserID:   userID,
		Username: username,
		Role:     role,
	})
}

// failAuth 把认证域的错误映射成合适的状态码。
//
// 只有**已知的域错误**才把原文外发；其余（Redis/JWT/数据库故障）一律走
// failInternal 脱敏，否则 "无法确认账号角色: dial tcp ..." 这类信息会直接
// 回到未认证调用方手里。
func failAuth(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		response.FailUnauthorized(c, err.Error())

	case errors.Is(err, service.ErrAccountLocked):
		response.FailTooManyRequests(c, err.Error())

	case errors.Is(err, service.ErrEmailTaken),
		errors.Is(err, repository.ErrAmbiguousUsername),
		errors.Is(err, utils.ErrVerifyCodeMismatch),
		errors.Is(err, utils.ErrVerifyCodeExhausted):
		response.FailInvalidParam(c, err.Error())

	default:
		failInternal(c, err)
	}
}
