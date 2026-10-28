package handler

import (
	"errors"
	"strconv"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// StudioProjectHandler 工作室项目与参与（#198）。
//
// 全部端点挂在 admin 组（authRequired + requireAdmin）。
type StudioProjectHandler struct {
	projects *service.StudioProjectUseCase
}

// NewStudioProjectHandler 构造函数。
func NewStudioProjectHandler(projects *service.StudioProjectUseCase) *StudioProjectHandler {
	return &StudioProjectHandler{projects: projects}
}

// failStudioProject 把项目域错误映射成 HTTP 响应。
//
// 业务错误一律 400 + 可读中文（与成员名录/知识库模块的既有约定一致）。
func failStudioProject(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrStudioProjectInvalid):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioProjectNotFound):
		response.FailInvalidParam(c, "工作室项目不存在")
	case errors.Is(err, repository.ErrStudioProjectDuplicateName):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioProjectMemberNotFound):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioProjectMemberDuplicate):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrStudioProjectMemberOwnerNotFound):
		response.FailInvalidParam(c, err.Error())
	default:
		failInternal(c, err)
	}
}

// parseProjectID 解析路径里的项目 id。
func parseProjectID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return 0, false
	}
	return id, true
}

// parseMemberID 解析路径里的参与行 id。
func parseMemberID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("member_id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "member_id 必须是正整数")
		return 0, false
	}
	return id, true
}

// parseTimeValue 解析请求体里的时间字符串。
//
// 返回值：(时间, 是否解析成功, 错误)。nil / 空串都表示「没有这个时间」，
// 返回零值与 false 而不是错误 —— 参与行允许没有结束时间（仍在参与）。
//
// 但格式非法必须报错：把 "2024/01/01" 当成「还没解析」会被静默当成
// 「仍在参与」，把已结束的参与行悄悄改回进行中。
func parseTimeValue(value *string, layout string) (time.Time, bool, error) {
	if value == nil {
		return time.Time{}, false, nil
	}
	trimmed := ""
	if value != nil {
		trimmed = *value
	}
	if trimmed == "" || trimmed == "null" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse(layout, trimmed)
	if err != nil {
		return time.Time{}, false, errors.New("时间格式须为 " + layout)
	}
	return t, true, nil
}

// ptrProjectStatus 把状态字符串指针转成领域枚举指针；nil 保持 nil（= 不改）。
func ptrProjectStatus(p *string) *domain.StudioProjectStatus {
	if p == nil {
		return nil
	}
	value := domain.StudioProjectStatus(*p)
	return &value
}

// ptrProjectRole 把角色字符串指针转成领域枚举指针；nil 保持 nil（= 不改）。
func ptrProjectRole(p *string) *domain.StudioProjectMemberRole {
	if p == nil {
		return nil
	}
	value := domain.StudioProjectMemberRole(*p)
	return &value
}

// optionalIntPtr 把三态整数转成双层指针；键没出现时返回 nil（= 不改）。
func optionalIntPtr(o request.OptionalInt) **int {
	if !o.Set {
		return nil
	}
	value := o.Value
	return &value
}

/* ------------------------------------------------------------------ *
 * 项目
 * ------------------------------------------------------------------ */

// ListStudioProjects 项目列表。
// @Summary 工作室项目列表
// @Description 分页列出项目，支持关键词（名称/简介）、状态与年份筛选。仅管理员可读。
// @Tags 管理端-工作室项目
// @Produce json
// @Param q query string false "关键词（名称/简介模糊匹配）"
// @Param status query string false "状态：active / completed / archived"
// @Param year query string false "年份精确匹配"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.StudioProjectListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects [get]
func (h *StudioProjectHandler) ListStudioProjects(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	projects, total, err := h.projects.List(
		c.Request.Context(),
		repository.StudioProjectListFilter{
			Query:  c.Query("q"),
			Status: c.Query("status"),
			Year:   c.Query("year"),
		},
		page,
		pageSize,
	)
	if err != nil {
		failStudioProject(c, err)
		return
	}

	response.OK(c, response.StudioProjectListResp{
		Items:    response.NewStudioProjectResps(projects),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetStudioProject 项目详情（含参与名单）。
// @Summary 工作室项目详情
// @Description 按 id 取项目元数据与完整参与名单。仅管理员可读。
// @Tags 管理端-工作室项目
// @Produce json
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Result{data=response.StudioProjectDetailResp} "成功"
// @Failure 400 {object} response.Result "项目不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id} [get]
func (h *StudioProjectHandler) GetStudioProject(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}

	project, members, err := h.projects.Get(c.Request.Context(), id)
	if err != nil {
		failStudioProject(c, err)
		return
	}
	response.OK(c, response.NewStudioProjectDetailResp(project, members))
}

// CreateStudioProject 创建项目。
// @Summary 创建工作室项目
// @Description 管理员创建项目；项目名全局唯一，冲突返回 400。
// @Tags 管理端-工作室项目
// @Accept json
// @Produce json
// @Param request body request.StudioProjectCreateReq true "项目信息"
// @Success 200 {object} response.Result{data=response.StudioProjectResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 项目名已存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects [post]
func (h *StudioProjectHandler) CreateStudioProject(c *gin.Context) {
	var req request.StudioProjectCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	project, err := h.projects.Create(c.Request.Context(), service.CreateStudioProjectInput{
		Fields: domain.StudioProjectFields{
			Name:    req.Name,
			Year:    req.Year,
			Summary: req.Summary,
			Status:  domain.StudioProjectStatus(req.Status),
		},
	})
	if err != nil {
		failStudioProject(c, err)
		return
	}

	response.OKWithMsg(c, "项目已创建", response.NewStudioProjectResp(project))
}

// UpdateStudioProject 更新项目元数据。
// @Summary 更新工作室项目
// @Description 补丁式更新：只传要改的项。
// @Tags 管理端-工作室项目
// @Accept json
// @Produce json
// @Param id path int true "项目 ID"
// @Param request body request.StudioProjectUpdateReq true "更新内容"
// @Success 200 {object} response.Result{data=response.StudioProjectResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 项目不存在 / 项目名已存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id} [put]
func (h *StudioProjectHandler) UpdateStudioProject(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}

	var req request.StudioProjectUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	project, err := h.projects.Update(c.Request.Context(), id, service.UpdateStudioProjectInput{
		Name:    req.Name,
		Year:    req.Year,
		Summary: req.Summary,
		Status:  ptrProjectStatus(req.Status),
	})
	if err != nil {
		failStudioProject(c, err)
		return
	}

	response.OKWithMsg(c, "项目已更新", response.NewStudioProjectResp(project))
}

// DeleteStudioProject 删除项目。
// @Summary 删除工作室项目
// @Description 删除项目；参与行由数据库级联删除。
// @Tags 管理端-工作室项目
// @Produce json
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "项目不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id} [delete]
func (h *StudioProjectHandler) DeleteStudioProject(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}

	if err := h.projects.Delete(c.Request.Context(), id); err != nil {
		failStudioProject(c, err)
		return
	}
	response.OKWithMsg(c, "项目已删除", nil)
}

/* ------------------------------------------------------------------ *
 * 参与
 * ------------------------------------------------------------------ */

// ListStudioProjectMembers 参与名单。
// @Summary 项目参与列表
// @Description 按开始时间升序列出某项目的参与行。
// @Tags 管理端-工作室项目
// @Produce json
// @Param id path int true "项目 ID"
// @Success 200 {object} response.Result{data=response.StudioProjectMemberListResp} "成功"
// @Failure 400 {object} response.Result "项目不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id}/members [get]
func (h *StudioProjectHandler) ListStudioProjectMembers(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}

	members, err := h.projects.ListMembers(c.Request.Context(), id)
	if err != nil {
		failStudioProject(c, err)
		return
	}

	response.OK(c, response.StudioProjectMemberListResp{
		Items: response.NewStudioProjectMemberResps(members),
		Total: len(members),
	})
}

// AddStudioProjectMember 添加参与。
// @Summary 添加项目参与
// @Description 姓名为必填快照；可关联已有成员或仅记外部姓名。
// @Tags 管理端-工作室项目
// @Accept json
// @Produce json
// @Param id path int true "项目 ID"
// @Param request body request.StudioProjectMemberAddReq true "参与信息"
// @Success 200 {object} response.Result{data=response.StudioProjectMemberResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 项目不存在 / 同名冲突 / 成员不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id}/members [post]
func (h *StudioProjectHandler) AddStudioProjectMember(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}

	var req request.StudioProjectMemberAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	startedAt, startedOK, err := parseTimeValue(req.StartedAt, domain.ProjectTimeLayout)
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	// 开始时间没传就是「今天加入」：让服务层填当前时间。
	if !startedOK {
		startedAt = time.Now()
	}

	var endedAt *time.Time
	if parsed, ok, err := parseTimeValue(req.EndedAt, domain.ProjectTimeLayout); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	} else if ok {
		endedAt = &parsed
	}

	member, err := h.projects.AddMember(c.Request.Context(), id, service.AddProjectMemberInput{
		Fields: domain.StudioProjectMemberFields{
			MemberID:  req.MemberID,
			Name:      req.Name,
			Role:      domain.StudioProjectMemberRole(req.Role),
			StartedAt: startedAt,
			EndedAt:   endedAt,
		},
	})
	if err != nil {
		failStudioProject(c, err)
		return
	}

	response.OKWithMsg(c, "参与已添加", response.NewStudioProjectMemberResp(member))
}

// UpdateStudioProjectMember 更新参与行。
// @Summary 更新项目参与
// @Description 补丁式更新。ended_at 传 null 表示仍在参与，传时间表示已结束。
// @Tags 管理端-工作室项目
// @Accept json
// @Produce json
// @Param id path int true "项目 ID"
// @Param member_id path int true "参与行 ID"
// @Param request body request.StudioProjectMemberUpdateReq true "更新内容"
// @Success 200 {object} response.Result{data=response.StudioProjectMemberResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 参与行不存在 / 同名冲突 / 结束早于开始"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id}/members/{member_id} [put]
func (h *StudioProjectHandler) UpdateStudioProjectMember(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	memberID, ok := parseMemberID(c)
	if !ok {
		return
	}

	var req request.StudioProjectMemberUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	// ended_at 三态：键不出现 = 不改；传 null = 仍在参与；传时间 = 已结束。
	var endedAt **time.Time
	if req.EndedAt.Set {
		var inner *time.Time
		if req.EndedAt.Value != nil {
			parsed, ok, err := parseTimeValue(req.EndedAt.Value, domain.ProjectTimeLayout)
			if err != nil {
				response.FailInvalidParam(c, err.Error())
				return
			}
			if ok {
				inner = &parsed
			}
		}
		endedAt = &inner
	}

	var startedAt *time.Time
	if parsed, ok, err := parseTimeValue(req.StartedAt, domain.ProjectTimeLayout); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	} else if ok {
		startedAt = &parsed
	}

	member, err := h.projects.UpdateMember(c.Request.Context(), id, memberID, service.UpdateProjectMemberInput{
		Name:      req.Name,
		Role:      ptrProjectRole(req.Role),
		MemberID:  optionalIntPtr(req.MemberID),
		StartedAt: startedAt,
		EndedAt:   endedAt,
	})
	if err != nil {
		failStudioProject(c, err)
		return
	}

	response.OKWithMsg(c, "参与已更新", response.NewStudioProjectMemberResp(member))
}

// DeleteStudioProjectMember 移除参与行。
// @Summary 移除项目参与
// @Description 删除一条参与行；不影响成员档案本身。
// @Tags 管理端-工作室项目
// @Produce json
// @Param id path int true "项目 ID"
// @Param member_id path int true "参与行 ID"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "参与行不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/projects/{id}/members/{member_id} [delete]
func (h *StudioProjectHandler) DeleteStudioProjectMember(c *gin.Context) {
	id, ok := parseProjectID(c)
	if !ok {
		return
	}
	memberID, ok := parseMemberID(c)
	if !ok {
		return
	}

	if err := h.projects.RemoveMember(c.Request.Context(), id, memberID); err != nil {
		failStudioProject(c, err)
		return
	}
	response.OKWithMsg(c, "参与已移除", nil)
}
