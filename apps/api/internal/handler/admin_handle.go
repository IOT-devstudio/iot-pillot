package handler

import (
	"strconv"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminHandler 管理端接口。
//
// 准入不在这里判断：路由上的 AuthRequired + RequireAdmin 已经拦过一道，
// 这里再次依赖中间件的结果只会形成第二个权限判定入口（两处漂移就是漏洞）。
type AdminHandler struct {
	adminService *service.AdminUseCase
}

func NewAdminHandler(adminService *service.AdminUseCase) *AdminHandler {
	return &AdminHandler{adminService: adminService}
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

// ListUsers 用户列表（仅管理员）
// @Summary 用户列表
// @Description 分页读取已注册用户，并标注其当前是否为管理员。响应不含密码哈希。
// @Tags 管理端
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.AdminUserListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Router /api/v1/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	result, err := h.adminService.ListUsers(c.Request.Context(), page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}

	response.OK(c, result)
}

// ListAdmins 管理员名单（仅管理员）
// @Summary 管理员名单
// @Description 列出当前具备管理权限的账号。名单真源在 Redis。
// @Tags 管理端
// @Produce json
// @Success 200 {object} response.Result{data=response.AdminListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Router /api/v1/admin/admins [get]
func (h *AdminHandler) ListAdmins(c *gin.Context) {
	result, err := h.adminService.ListAdmins(c.Request.Context())
	if err != nil {
		failInternal(c, err)
		return
	}

	response.OK(c, result)
}

// GrantAdmin 提升为管理员（仅管理员）
// @Summary 提升为管理员
// @Description 把已注册用户加入管理员名单，并撤销其现有会话（旧令牌的 role 已过时，需重新登录）。
// @Tags 管理端
// @Accept json
// @Produce json
// @Param request body request.AdminUserIDReq true "目标用户"
// @Success 200 {object} response.Result{data=response.AdminMutationResp} "成功"
// @Failure 400 {object} response.Result "参数错误或用户不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Router /api/v1/admin/admins [post]
func (h *AdminHandler) GrantAdmin(c *gin.Context) {
	var req request.AdminUserIDReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.adminService.GrantAdmin(c.Request.Context(), req.UserID)
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	response.OKWithMsg(c, "已提升为管理员，对方需要重新登录", result)
}

// RevokeAdmin 撤销管理员（仅管理员）
// @Summary 撤销管理员
// @Description 把用户移出管理员名单，并撤销其现有会话。服务端强制 403 读的就是这份名单，因此立即生效。
// @Tags 管理端
// @Produce json
// @Param user_id path int true "目标用户 ID"
// @Success 200 {object} response.Result{data=response.AdminMutationResp} "成功"
// @Failure 400 {object} response.Result "参数错误"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Router /api/v1/admin/admins/{user_id} [delete]
func (h *AdminHandler) RevokeAdmin(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		response.FailInvalidParam(c, "user_id 必须是正整数")
		return
	}

	// 不允许撤销自己：把自己踢出管理端之后，如果系统里没有别的管理员，
	// 就再也没人能加回来了（只能改配置重启）。这类"自锁"操作必须在服务端挡住。
	if currentID, _, _, ok := middleware.CurrentUser(c); ok && currentID == userID {
		response.FailInvalidParam(c, "不能撤销自己的管理员身份，请让另一位管理员操作")
		return
	}

	result, err := h.adminService.RevokeAdmin(c.Request.Context(), userID)
	if err != nil {
		failInternal(c, err)
		return
	}

	response.OKWithMsg(c, "已撤销管理员，对方需要重新登录", result)
}
