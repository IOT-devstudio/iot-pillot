package handler

import (
	"errors"
	"strconv"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// StudioMemberHandler 工作室成员名录（#195）。
//
// 全部端点挂在 admin 组（authRequired + requireAdmin），
// 首期不提供 member 只读或游客公开接口（ADR 0001 §权限拍板）。
type StudioMemberHandler struct {
	members *service.StudioMemberUseCase
}

// NewStudioMemberHandler 构造函数。
func NewStudioMemberHandler(members *service.StudioMemberUseCase) *StudioMemberHandler {
	return &StudioMemberHandler{members: members}
}

// failStudioMember 把成员域错误映射成 HTTP 响应。
//
// 业务错误一律 400 + 可读中文（与招新/表单模块的既有约定一致），
// 只有「归档一个已归档成员」这种状态冲突也走 400：它不是权限问题，
// 也不该让调用方去猜 404 / 409 / 500 的区别。
func failStudioMember(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrStudioMemberInvalid):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioMemberNotFound):
		response.FailInvalidParam(c, "工作室成员不存在")
	case errors.Is(err, repository.ErrStudioMemberDuplicate):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioMemberArchived):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioMemberUserNotFound):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrCandidateNotHired):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrCandidateAlreadyConverted):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrCandidateMissing):
		response.FailInvalidParam(c, "候选人不存在")
	default:
		failInternal(c, err)
	}
}

// parseStudioMemberID 解析路径里的成员 id。
func parseStudioMemberID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return 0, false
	}
	return id, true
}

// parseOptionalBool 解析三态布尔查询参数。
//
// 缺省 / 空串 / 无法识别都返回 nil（= 不筛选），而不是报错：
// 查询参数写错不该让整个列表接口失败（与 parseBoundedInt 的取舍一致）。
func parseOptionalBool(raw string) *bool {
	switch raw {
	case "true", "1", "TRUE", "True":
		value := true
		return &value
	case "false", "0", "FALSE", "False":
		value := false
		return &value
	default:
		return nil
	}
}

// ConvertCandidateToMember hired 候选人转成员（#197）。
// @Summary 将 hired 候选人转为工作室成员
// @Description 只有已录用（hired）的候选人可转入；同一候选人只能转入一次。
// @Description 姓名与方向取自申请，出生年 / 届别 / 职能由管理员补全。不改招新状态机。
// @Tags 管理端-工作室成员
// @Accept json
// @Produce json
// @Param request body request.ConvertCandidateToMemberReq true "转入信息"
// @Success 200 {object} response.Result{data=response.StudioMemberResp} "成功"
// @Failure 400 {object} response.Result "候选人不存在 / 非 hired / 已转入 / 缺必填项"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/members/from-candidate [post]
func (h *StudioMemberHandler) ConvertCandidateToMember(c *gin.Context) {
	var req request.ConvertCandidateToMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	member, err := h.members.ConvertFromCandidate(c.Request.Context(), service.ConvertCandidateInput{
		CandidateID: req.CandidateID,
		Fields: domain.StudioMemberFields{
			BirthYear:    req.BirthYear,
			Cohort:       req.Cohort,
			RoleInStudio: req.RoleInStudio,
			Internship:   req.Internship,
			Employment:   req.Employment,
		},
	})
	if err != nil {
		failStudioMember(c, err)
		return
	}

	response.OKWithMsg(c, "候选人已转为工作室成员", response.NewStudioMemberResp(member))
}

// ListStudioMembers 成员列表。
// @Summary 工作室成员列表
// @Description 分页列出成员名录，支持关键词（姓名/届别/方向/职能）、方向与在读状态筛选。仅管理员可读。
// @Tags 管理端-工作室成员
// @Produce json
// @Param q query string false "关键词（姓名/届别/方向/职能模糊匹配）"
// @Param direction query string false "方向精确匹配"
// @Param is_active query bool false "在读状态；缺省返回全部（含已归档）"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.StudioMemberListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/members [get]
func (h *StudioMemberHandler) ListStudioMembers(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	members, total, err := h.members.List(
		c.Request.Context(),
		repository.StudioMemberListFilter{
			Query:     c.Query("q"),
			Direction: c.Query("direction"),
			IsActive:  parseOptionalBool(c.Query("is_active")),
		},
		page,
		pageSize,
	)
	if err != nil {
		failStudioMember(c, err)
		return
	}

	response.OK(c, response.StudioMemberListResp{
		Items:    response.NewStudioMemberResps(members),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetStudioMember 成员详情。
// @Summary 工作室成员详情
// @Description 按 id 取成员档案。仅管理员可读。
// @Tags 管理端-工作室成员
// @Produce json
// @Param id path int true "成员 ID"
// @Success 200 {object} response.Result{data=response.StudioMemberResp} "成功"
// @Failure 400 {object} response.Result "成员不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/members/{id} [get]
func (h *StudioMemberHandler) GetStudioMember(c *gin.Context) {
	id, ok := parseStudioMemberID(c)
	if !ok {
		return
	}

	member, err := h.members.Get(c.Request.Context(), id)
	if err != nil {
		failStudioMember(c, err)
		return
	}
	response.OK(c, response.NewStudioMemberResp(member))
}

// CreateStudioMember 录入成员。
// @Summary 录入工作室成员
// @Description 管理员录入成员档案。user_id 可空（收录无系统账号的校友/外部人员）；同名+出生年冲突返回 400。
// @Tags 管理端-工作室成员
// @Accept json
// @Produce json
// @Param request body request.StudioMemberCreateReq true "成员档案"
// @Success 200 {object} response.Result{data=response.StudioMemberResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 同名同出生年已存在 / user_id 无效"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/members [post]
func (h *StudioMemberHandler) CreateStudioMember(c *gin.Context) {
	var req request.StudioMemberCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	member, err := h.members.Create(c.Request.Context(), service.CreateStudioMemberInput{
		Fields: domain.StudioMemberFields{
			Name:         req.Name,
			BirthYear:    req.BirthYear,
			Cohort:       req.Cohort,
			Direction:    req.Direction,
			RoleInStudio: req.RoleInStudio,
			Internship:   req.Internship,
			Employment:   req.Employment,
		},
		UserID: req.UserID,
	})
	if err != nil {
		failStudioMember(c, err)
		return
	}

	response.OKWithMsg(c, "成员已录入", response.NewStudioMemberResp(member))
}

// UpdateStudioMember 更新成员。
// @Summary 更新工作室成员
// @Description 补丁式更新：只传要改的项。user_id 传 null 表示解除账号绑定；is_active=true 表示恢复在读。
// @Tags 管理端-工作室成员
// @Accept json
// @Produce json
// @Param id path int true "成员 ID"
// @Param request body request.StudioMemberUpdateReq true "更新内容"
// @Success 200 {object} response.Result{data=response.StudioMemberResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 成员不存在 / 同名同出生年已存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/members/{id} [put]
func (h *StudioMemberHandler) UpdateStudioMember(c *gin.Context) {
	id, ok := parseStudioMemberID(c)
	if !ok {
		return
	}

	var req request.StudioMemberUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	// user_id 是三态，必须按「键是否出现」翻译，不能直接传值：
	// 没出现 = 不改；出现且为 null = 解除绑定；出现且有值 = 绑定该账号。
	var userID **int
	if req.UserID.Set {
		value := req.UserID.Value
		userID = &value
	}

	member, err := h.members.Update(c.Request.Context(), id, service.UpdateStudioMemberInput{
		Name:         req.Name,
		BirthYear:    req.BirthYear,
		Cohort:       req.Cohort,
		Direction:    req.Direction,
		RoleInStudio: req.RoleInStudio,
		Internship:   req.Internship,
		Employment:   req.Employment,
		UserID:       userID,
		IsActive:     req.IsActive,
	})
	if err != nil {
		failStudioMember(c, err)
		return
	}

	response.OKWithMsg(c, "成员已更新", response.NewStudioMemberResp(member))
}

// ArchiveStudioMember 归档成员。
// @Summary 归档工作室成员
// @Description 归档（置 archived_at 与 is_active=false），不物理删除，以保住后续项目参与历史。重复归档返回 400。
// @Tags 管理端-工作室成员
// @Produce json
// @Param id path int true "成员 ID"
// @Success 200 {object} response.Result{data=response.StudioMemberResp} "成功"
// @Failure 400 {object} response.Result "成员不存在或已归档"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/members/{id}/archive [post]
func (h *StudioMemberHandler) ArchiveStudioMember(c *gin.Context) {
	id, ok := parseStudioMemberID(c)
	if !ok {
		return
	}

	member, err := h.members.Archive(c.Request.Context(), id)
	if err != nil {
		failStudioMember(c, err)
		return
	}

	response.OKWithMsg(c, "成员已归档", response.NewStudioMemberResp(member))
}
