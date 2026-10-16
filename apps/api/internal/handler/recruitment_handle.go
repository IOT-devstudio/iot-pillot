package handler

import (
	"strconv"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// RecruitmentHandler 招新模块。
//
// 用户侧接口走 authRequired，管理侧接口走 admin 组（authRequired + requireAdmin）。
// 归属与权限由中间件 + 服务层校验，handler 只做参数绑定与响应封装。
//
// formService 的存在只为方向列表组装「该方向的面试表单」（#135 §2.2）：
// 跨域依赖停在 handler 层，不让 RecruitmentUseCase 反向依赖表单仓储，
// 两个域的边界因此保持干净。
type RecruitmentHandler struct {
	recruitmentService *service.RecruitmentUseCase
	formService        *service.FormUseCase
}

func NewRecruitmentHandler(recruitmentService *service.RecruitmentUseCase, formService *service.FormUseCase) *RecruitmentHandler {
	return &RecruitmentHandler{recruitmentService: recruitmentService, formService: formService}
}

// failRecruitment 把招新域错误映射成 HTTP 响应。
//
// 业务错误（关闭方向 / 重复报名 / 无开放轮次 / 非法流转）一律 400 + 可读中文。
// 未找到错误也走 400（参数或资源不存在），不让调用方猜 404 vs 500。
func failRecruitment(c *gin.Context, err error) {
	switch err {
	case service.ErrDirectionClosed:
		response.FailInvalidParam(c, err.Error())
	case service.ErrNoActiveRound:
		response.FailInvalidParam(c, err.Error())
	case service.ErrRoundClosed:
		response.FailInvalidParam(c, err.Error())
	case service.ErrInvalidTransition:
		response.FailInvalidParam(c, err.Error())
	case service.ErrNotOwned:
		response.FailForbidden(c, err.Error())
	case service.ErrInvalidDirectionCode:
		response.FailInvalidParam(c, err.Error())
	case service.ErrInvalidDirectionName:
		response.FailInvalidParam(c, err.Error())
	case repository.ErrDirectionCodeExists:
		response.FailInvalidParam(c, err.Error())
	case repository.ErrDirectionNotFound:
		response.FailInvalidParam(c, "招新方向不存在")
	case repository.ErrRoundNotFound:
		response.FailInvalidParam(c, "招新轮次不存在")
	case repository.ErrApplicationNotFound:
		response.FailInvalidParam(c, "申请不存在")
	case repository.ErrDuplicateApplication:
		response.FailInvalidParam(c, err.Error())
	default:
		failInternal(c, err)
	}
}

/* ------------------------------------------------------------------ *
 * 用户侧
 * ------------------------------------------------------------------ */

// ListActiveDirections 获取可报名方向。
// @Summary 可报名方向列表
// @Description 返回当前开放报名的招新方向，供用户报名页渲染可选方向。每个方向带 form（该方向的面试表单定位信息），无开放表单时为 null。
// @Tags 招新模块
// @Produce json
// @Success 200 {object} response.Result{data=response.RecruitmentDirectionListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/recruitment/directions [get]
func (h *RecruitmentHandler) ListActiveDirections(c *gin.Context) {
	ctx := c.Request.Context()

	dirs, err := h.recruitmentService.ListActiveDirections(ctx)
	if err != nil {
		failInternal(c, err)
		return
	}

	// 取「当前开放轮次」下各方向的面试表单。没有开放轮次时 roundID = 0，
	// formService 直接返回空 map —— 方向列表照常返回，只是 form 全为 null，
	// 不因为"现在没开招新"就让整个接口报错。
	roundID, err := h.recruitmentService.ActiveRoundID(ctx)
	if err != nil {
		failInternal(c, err)
		return
	}

	forms, err := h.formService.OpenFormsByRound(ctx, roundID)
	if err != nil {
		failInternal(c, err)
		return
	}

	items := make([]response.RecruitmentDirectionResp, 0, len(dirs))
	for _, d := range dirs {
		item := response.NewRecruitmentDirectionResp(d)
		// 方向没有开放表单时 form 保持 nil → 序列化成 null（前端靠 form === null 降级）
		item.Form = response.NewRecruitmentDirectionFormResp(forms[d.Code])
		items = append(items, item)
	}

	response.OK(c, response.RecruitmentDirectionListResp{Items: items})
}

// SubmitApplication 用户报名。
// @Summary 提交报名
// @Description 登录用户报名某个开放方向。归属由令牌决定，body 不接收 user_id。
// @Tags 招新模块
// @Accept json
// @Produce json
// @Param request body request.RecruitmentSubmitReq true "报名请求"
// @Success 200 {object} response.Result{data=response.RecruitmentApplicationResp} "成功"
// @Failure 400 {object} response.Result "方向关闭或重复报名"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/recruitment/applications [post]
func (h *RecruitmentHandler) SubmitApplication(c *gin.Context) {
	var req request.RecruitmentSubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	app, err := h.recruitmentService.Submit(c.Request.Context(), userID, req.DirectionCode)
	if err != nil {
		failRecruitment(c, err)
		return
	}

	response.OKWithMsg(c, "报名成功", response.NewRecruitmentApplicationResp(app))
}

// ListMyApplications 我的报名列表。
// @Summary 我的报名列表
// @Description 分页列出当前用户本人的报名申请。
// @Tags 招新模块
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.RecruitmentApplicationListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/recruitment/applications [get]
func (h *RecruitmentHandler) ListMyApplications(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	apps, total, err := h.recruitmentService.ListMyApplications(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}

	items := make([]response.RecruitmentApplicationResp, 0, len(apps))
	for _, a := range apps {
		items = append(items, response.NewRecruitmentApplicationResp(a))
	}

	response.OK(c, response.RecruitmentApplicationListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetMyApplication 我的报名详情。
// @Summary 我的报名详情
// @Description 取当前用户本人的一条报名申请（含时间线）。非本人申请返回 403。
// @Tags 招新模块
// @Produce json
// @Param id path int true "申请 ID"
// @Success 200 {object} response.Result{data=response.RecruitmentApplicationDetailResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非本人申请"
// @Security BearerAuth
// @Router /api/v1/recruitment/applications/{id} [get]
func (h *RecruitmentHandler) GetMyApplication(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return
	}

	app, timeline, err := h.recruitmentService.GetMyApplication(c.Request.Context(), userID, id)
	if err != nil {
		failRecruitment(c, err)
		return
	}

	response.OK(c, response.NewRecruitmentApplicationDetailResp(app, timeline))
}

/* ------------------------------------------------------------------ *
 * 管理端
 * ------------------------------------------------------------------ */

// AdminListRounds 招新轮次列表。
// @Summary 招新轮次列表
// @Description 列出全部招新轮次，含开放状态。
// @Tags 管理端-招新
// @Produce json
// @Success 200 {object} response.Result{data=response.RecruitmentRoundListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/rounds [get]
func (h *RecruitmentHandler) AdminListRounds(c *gin.Context) {
	rounds, err := h.recruitmentService.ListRounds(c.Request.Context())
	if err != nil {
		failInternal(c, err)
		return
	}

	items := make([]response.RecruitmentRoundResp, 0, len(rounds))
	for _, r := range rounds {
		items = append(items, response.NewRecruitmentRoundResp(r))
	}

	response.OK(c, response.RecruitmentRoundListResp{Items: items})
}

// AdminSetRound 设置轮次开放状态。
// @Summary 设置轮次开放状态
// @Description 开启或关闭某轮的报名窗口。
// @Tags 管理端-招新
// @Accept json
// @Produce json
// @Param id path int true "轮次 ID"
// @Param request body request.RecruitmentSetRoundReq true "开放状态"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "轮次不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/rounds/{id} [patch]
func (h *RecruitmentHandler) AdminSetRound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return
	}

	var req request.RecruitmentSetRoundReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	if err := h.recruitmentService.SetRoundAccepting(c.Request.Context(), id, req.Accepting); err != nil {
		failRecruitment(c, err)
		return
	}

	response.OKWithMsg(c, "轮次状态已更新", nil)
}

// AdminListDirections 管理端方向全量列表。
// @Summary 管理端方向全量列表
// @Description 返回全部招新方向（含关闭中的），按 sort_order 稳定排序。
// 供管理端开关条与「新增方向」入口渲染；用户端可报名列表走 GET /recruitment/directions。
// @Tags 管理端-招新
// @Produce json
// @Success 200 {object} response.Result{data=response.RecruitmentDirectionListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/directions [get]
func (h *RecruitmentHandler) AdminListDirections(c *gin.Context) {
	dirs, err := h.recruitmentService.ListDirections(c.Request.Context())
	if err != nil {
		failInternal(c, err)
		return
	}

	items := make([]response.RecruitmentDirectionResp, 0, len(dirs))
	for _, d := range dirs {
		items = append(items, response.NewRecruitmentDirectionResp(d))
	}

	response.OK(c, response.RecruitmentDirectionListResp{Items: items})
}

// AdminCreateDirection 创建招新方向。
// @Summary 创建招新方向
// @Description 管理员自助创建方向（issue #128）。code 为小写字母/数字/连字符（1~32），
// 与已有行重复返回 400；创建后 open=false，不出现在用户端可报名列表，
// 需 PATCH /directions/{code} 显式开启。
// @Tags 管理端-招新
// @Accept json
// @Produce json
// @Param request body request.RecruitmentCreateDirectionReq true "创建方向请求"
// @Success 200 {object} response.Result{data=response.RecruitmentDirectionResp} "成功"
// @Failure 400 {object} response.Result "参数错误或 code 已存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/directions [post]
func (h *RecruitmentHandler) AdminCreateDirection(c *gin.Context) {
	var req request.RecruitmentCreateDirectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	dir, err := h.recruitmentService.CreateDirection(c.Request.Context(), req.Code, req.Name, req.SortOrder)
	if err != nil {
		failRecruitment(c, err)
		return
	}

	response.OKWithMsg(c, "方向创建成功", response.NewRecruitmentDirectionResp(dir))
}

// AdminSetDirection 设置方向开关。
// @Summary 设置方向开关
// @Description 开启或关闭某个招新方向的报名开关。表中不存在的 code 返回 400。
// @Tags 管理端-招新
// @Accept json
// @Produce json
// @Param code path string true "方向 code（如 front-end）"
// @Param request body request.RecruitmentSetDirectionReq true "开关状态"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "方向不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/directions/{code} [patch]
func (h *RecruitmentHandler) AdminSetDirection(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.FailInvalidParam(c, "方向 code 不能为空")
		return
	}

	var req request.RecruitmentSetDirectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	if err := h.recruitmentService.SetDirectionOpen(c.Request.Context(), code, req.Open); err != nil {
		failRecruitment(c, err)
		return
	}

	response.OKWithMsg(c, "方向状态已更新", nil)
}

// AdminListApplications 报名申请列表。
// @Summary 报名申请列表
// @Description 分页列出全部报名申请，可按轮次/方向/状态筛选。
// @Tags 管理端-招新
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param round_id query int false "按轮次筛选"
// @Param direction_code query string false "按方向筛选"
// @Param status query string false "按状态筛选（applied/reviewing/accepted/rejected）"
// @Success 200 {object} response.Result{data=response.RecruitmentApplicationListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/applications [get]
func (h *RecruitmentHandler) AdminListApplications(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	filter := repository.ApplicationListFilter{
		RoundID:       parseBoundedInt(c.Query("round_id"), 0, 0, 1000000),
		DirectionCode: c.Query("direction_code"),
		Status:        c.Query("status"),
	}

	apps, total, err := h.recruitmentService.AdminListApplications(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}

	items := make([]response.RecruitmentApplicationResp, 0, len(apps))
	for _, a := range apps {
		items = append(items, response.NewRecruitmentApplicationResp(a))
	}

	response.OK(c, response.RecruitmentApplicationListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// AdminGetApplication 报名申请详情。
// @Summary 报名申请详情
// @Description 取一条申请详情（含时间线）。
// @Tags 管理端-招新
// @Produce json
// @Param id path int true "申请 ID"
// @Success 200 {object} response.Result{data=response.RecruitmentApplicationDetailResp} "成功"
// @Failure 400 {object} response.Result "申请不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/applications/{id} [get]
func (h *RecruitmentHandler) AdminGetApplication(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return
	}

	app, timeline, err := h.recruitmentService.AdminGetApplication(c.Request.Context(), id)
	if err != nil {
		failRecruitment(c, err)
		return
	}

	response.OK(c, response.NewRecruitmentApplicationDetailResp(app, timeline))
}

// AdminReviewApplication 审核报名申请。
// @Summary 审核报名申请
// @Description 推进申请到下一状态（applied→reviewing，reviewing→accepted/rejected）。
// @Tags 管理端-招新
// @Accept json
// @Produce json
// @Param id path int true "申请 ID"
// @Param request body request.RecruitmentReviewReq true "审核目标"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "非法状态流转"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/recruitment/applications/{id}/review [patch]
func (h *RecruitmentHandler) AdminReviewApplication(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return
	}

	var req request.RecruitmentReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	target := domain.ApplicationStatus(req.Target)
	if err := h.recruitmentService.Review(c.Request.Context(), id, target); err != nil {
		failRecruitment(c, err)
		return
	}

	response.OKWithMsg(c, "审核状态已更新", nil)
}
