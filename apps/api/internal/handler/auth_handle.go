package handler

import (
	"strconv"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthUseCase
}

func NewAuthHandler(authService *service.AuthUseCase) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login 用户登录接口
// @Summary 用户登录
// @Description 通过用户名密码进行登录，返回双令牌（access_token + refresh_token）
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.LoginReq true "登录请求参数"
// @Success 200 {object} response.Result{data=response.LoginResp} "成功返回双令牌"
// @Failure 400 {object} response.Result "请求参数错误"
// @Router /api/v1/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var dto request.LoginReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	token, err := h.authService.Login(c, &dto)
	if err != nil {
		response.FailUnauthorized(c, err.Error())
		return
	}

	response.OK(c, token)
}

// Register 用户注册接口
// @Summary 用户注册
// @Description 注册新用户，返回双令牌（access_token + refresh_token）
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.RegisterReq true "注册请求参数"
// @Success 200 {object} response.Result{data=response.LoginResp} "成功返回双令牌"
// @Failure 400 {object} response.Result "请求参数错误"
// @Router /api/v1/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var dto request.RegisterReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	token, err := h.authService.Register(c, &dto)
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	response.OK(c, token)
}

// Refresh 刷新令牌接口
// @Summary 刷新令牌
// @Description 使用 refresh token 获取新的双令牌
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param request body request.RefreshReq true "刷新令牌请求参数"
// @Success 200 {object} response.Result "成功返回新的双令牌"
// @Failure 400 {object} response.Result "请求参数错误"
// @Router /api/v1/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req request.RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	result, err := h.authService.Refresh(c, req.RefreshToken)
	if err != nil {
		response.FailUnauthorized(c, err.Error())
		return
	}
	response.OK(c, result)
}

// SendVerifyCode 发送验证码接口
// @Summary 发送验证码接口
// @Description 发送验证码到指定邮箱
// @Tags 认证模块
// @Accept json
// @Produce json
// @Param verifier query string true "验证目标（邮箱）"
// @Param verifier_type query string true "验证类型（email）"
// @Success 200 {object} response.Result "发送验证码成功"
// @Failure 400 {object} response.Result "请求参数错误"
// @Router /api/v1/send-verify-code [post]
func (h *AuthHandler) SendVerifyCode(c *gin.Context) {
	var req request.VerifyCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	err := h.authService.SendVerifyCode(c, req.Verifier, req.VerifierType)
	if err != nil {
		response.FailInvalidParam(c, err.Error())
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

	err := h.authService.Logout(c, accessToken, req.RefreshToken)
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	response.OKWithMsg(c, "退出登录成功", nil)
}

// Me 返回当前登录用户的身份与角色
// @Summary 获取当前登录用户
// @Description 从 access token 解出身份与角色。角色来源是部署配置里的管理员白名单，
// 前端据此决定是否显示管理端入口；真正的强制在后端路由的 RequireRole 中间件。
// @Tags 认证模块
// @Produce json
// @Success 200 {object} response.Result{data=response.MeResp} "成功"
// @Failure 401 {object} response.Result "未认证或令牌失效"
// @Router /api/v1/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, username, role, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "缺少认证信息")
		return
	}

	response.OK(c, response.MeResp{
		UserID:   userID,
		Username: username,
		Role:     role,
	})
}

// ListUsers 用户列表（仅管理员）
// @Summary 用户列表
// @Description 分页读取已注册用户。响应不含密码哈希。需要 admin 角色。
// @Tags 管理端
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.AdminUserListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Router /api/v1/admin/users [get]
func (h *AuthHandler) ListUsers(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	result, err := h.authService.ListUsers(c, page, pageSize)
	if err != nil {
		response.FailServer(c, err.Error())
		return
	}

	response.OK(c, result)
}

// parseBoundedInt 解析查询参数里的整数并夹到 [min, max]。
//
// 非法输入（空串、字母、负数）一律回落到 fallback，而不是报 400：
// 分页参数脏了不该让整个列表接口失败。
func parseBoundedInt(raw string, fallback int, min int, max int) int {
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	if value < min {
		return min
	}
	if value > max {
		return max
	}

	return value
}

// ValidateToken 校验当前 token 是否有效
// @Summary 校验 token 是否有效
// @Description 校验 Authorization 中的 token（JWT 签名 + Redis 会话唯一码），
// 有效返回 200；无效由 AuthRequired 中间件返回 401
// @Tags 认证模块
// @Produce json
// @Success 200 {object} response.Result "有效 data: {"valid": true}"
// @Failure 401 {object} response.Result "无效"
// @Router /api/v2/token/check [get]
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	response.OK(c, gin.H{"valid": true})
}
