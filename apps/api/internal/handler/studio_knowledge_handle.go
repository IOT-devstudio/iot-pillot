package handler

import (
	"errors"
	"strconv"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// StudioKnowledgeHandler 工作室知识库（#196）。
//
// 全部端点挂在 admin 组（authRequired + requireAdmin）。
// permission 字段可写但**不参与鉴权**：首期不为 member/public 挂任何读取入口
// （ADR 0001 §3：字段先落地，路由后开）。
type StudioKnowledgeHandler struct {
	knowledge *service.StudioKnowledgeUseCase
}

// NewStudioKnowledgeHandler 构造函数。
func NewStudioKnowledgeHandler(knowledge *service.StudioKnowledgeUseCase) *StudioKnowledgeHandler {
	return &StudioKnowledgeHandler{knowledge: knowledge}
}

// failStudioKnowledge 把知识库域错误映射成 HTTP 响应。
//
// 业务错误一律 400 + 可读中文（与成员名录/表单模块的既有约定一致）。
// 版本冲突也走 400：它不是服务端故障，调用方要做的是「刷新后重试」，
// 而 409 在本仓库没有先例，前端也没有对应的处理分支。
func failStudioKnowledge(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrStudioKnowledgeInvalid):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrStudioKnowledgeParentInvalid):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrKnowledgeVersionConflict):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrKnowledgeEntryNotFound):
		response.FailInvalidParam(c, "知识库条目不存在")
	case errors.Is(err, repository.ErrKnowledgeRevisionNotFound):
		response.FailInvalidParam(c, err.Error())
	default:
		failInternal(c, err)
	}
}

// parseKnowledgeID 解析路径里的条目 id。
func parseKnowledgeID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return 0, false
	}
	return id, true
}

// parseOptionalParentID 解析三态 parent_id 查询参数。
//
// 语义与请求体里的 parent_id 不同（这里是**筛选**而不是赋值）：
//
//	参数缺省       = 不按父子筛选
//	parent_id=none = 只看顶层（parent_id IS NULL）
//	parent_id=N    = 只看该条目的直接子条目
//
// 用 "none" 而不是空串表达「顶层」：空串在 query 里与「没传」不可区分，
// 而这两件事在这里必须分开。
func parseOptionalParentID(raw string) **int {
	switch raw {
	case "":
		return nil
	case "none", "null":
		var topLevel *int
		return &topLevel
	default:
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			// 写错的值按「不筛选」处理，与 parseBoundedInt 的取舍一致：
			// 查询参数写错不该让整个列表接口失败。
			return nil
		}
		// 两层指针：外层非 nil 表示「要按父条目筛选」，
		// 内层非 nil 表示「筛这一条」。&value 只是 *int，少了外层，
		// 所以要先取一层地址再取一层。
		value := id
		inner := &value
		return &inner
	}
}

// ListStudioKnowledge 条目列表。
// @Summary 知识库条目列表
// @Description 分页列出知识库条目，支持关键词（标题/正文）、权限与父条目筛选。仅管理员可读。
// @Tags 管理端-工作室知识库
// @Produce json
// @Param q query string false "关键词（标题/正文模糊匹配）"
// @Param permission query string false "权限：admin / member / public"
// @Param parent_id query string false "父条目筛选：缺省不筛选，none 只看顶层，数字看直接子条目"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.StudioKnowledgeListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge [get]
func (h *StudioKnowledgeHandler) ListStudioKnowledge(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	entries, total, err := h.knowledge.List(
		c.Request.Context(),
		repository.StudioKnowledgeListFilter{
			Query:      c.Query("q"),
			Permission: c.Query("permission"),
			ParentID:   parseOptionalParentID(c.Query("parent_id")),
		},
		page,
		pageSize,
	)
	if err != nil {
		failStudioKnowledge(c, err)
		return
	}

	response.OK(c, response.StudioKnowledgeListResp{
		Items:    response.NewStudioKnowledgeEntryResps(entries),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetStudioKnowledge 条目当前正文。
// @Summary 知识库条目详情
// @Description 按 id 取条目当前正文与版本号。仅管理员可读。
// @Tags 管理端-工作室知识库
// @Produce json
// @Param id path int true "条目 ID"
// @Success 200 {object} response.Result{data=response.StudioKnowledgeEntryResp} "成功"
// @Failure 400 {object} response.Result "条目不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge/{id} [get]
func (h *StudioKnowledgeHandler) GetStudioKnowledge(c *gin.Context) {
	id, ok := parseKnowledgeID(c)
	if !ok {
		return
	}

	entry, err := h.knowledge.Get(c.Request.Context(), id)
	if err != nil {
		failStudioKnowledge(c, err)
		return
	}
	response.OK(c, response.NewStudioKnowledgeEntryResp(entry))
}

// CreateStudioKnowledge 创建条目（同时写入 v1 修订）。
// @Summary 创建知识库条目
// @Description 管理员创建条目，版本从 1 起并同时写入 v1 修订快照。作者取自登录身份。
// @Tags 管理端-工作室知识库
// @Accept json
// @Produce json
// @Param request body request.StudioKnowledgeCreateReq true "条目内容"
// @Success 200 {object} response.Result{data=response.StudioKnowledgeEntryResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 父条目不可用"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge [post]
func (h *StudioKnowledgeHandler) CreateStudioKnowledge(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	var req request.StudioKnowledgeCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	entry, err := h.knowledge.Create(c.Request.Context(), service.CreateStudioKnowledgeInput{
		Fields: domain.StudioKnowledgeFields{
			Title:      req.Title,
			Body:       req.Body,
			Permission: domain.StudioKnowledgePermission(req.Permission),
			ParentID:   req.ParentID,
		},
		AuthorID: userID,
	})
	if err != nil {
		failStudioKnowledge(c, err)
		return
	}

	response.OKWithMsg(c, "条目已创建", response.NewStudioKnowledgeEntryResp(entry))
}

// UpdateStudioKnowledge 保存条目（版本 +1 并写入新修订）。
// @Summary 保存知识库条目
// @Description 补丁式保存：只传要改的项。每次成功保存版本 +1 并写入对应修订快照。
// @Description version 是乐观锁：带上你读到的版本号，被他人抢先保存时返回 400 冲突。
// @Tags 管理端-工作室知识库
// @Accept json
// @Produce json
// @Param id path int true "条目 ID"
// @Param request body request.StudioKnowledgeUpdateReq true "更新内容"
// @Success 200 {object} response.Result{data=response.StudioKnowledgeEntryResp} "成功"
// @Failure 400 {object} response.Result "字段不合法 / 条目不存在 / 父条目不可用 / 版本冲突"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge/{id} [put]
func (h *StudioKnowledgeHandler) UpdateStudioKnowledge(c *gin.Context) {
	id, ok := parseKnowledgeID(c)
	if !ok {
		return
	}
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	var req request.StudioKnowledgeUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	var permission *domain.StudioKnowledgePermission
	if req.Permission != nil {
		value := domain.StudioKnowledgePermission(*req.Permission)
		permission = &value
	}

	// parent_id 是三态，必须按「键是否出现」翻译：
	// 没出现 = 不改；出现且为 null = 移到顶层；出现且有值 = 挂到该条目下。
	var parentID **int
	if req.ParentID.Set {
		value := req.ParentID.Value
		parentID = &value
	}

	entry, err := h.knowledge.Update(c.Request.Context(), id, service.UpdateStudioKnowledgeInput{
		Title:      req.Title,
		Body:       req.Body,
		Permission: permission,
		ParentID:   parentID,
		EditorID:   userID,
	}, req.Version)
	if err != nil {
		failStudioKnowledge(c, err)
		return
	}

	response.OKWithMsg(c, "条目已保存", response.NewStudioKnowledgeEntryResp(entry))
}

// DeleteStudioKnowledge 删除条目。
// @Summary 删除知识库条目
// @Description 删除条目；修订由数据库级联删除，子条目的 parent 置空（不连带删子树）。
// @Tags 管理端-工作室知识库
// @Produce json
// @Param id path int true "条目 ID"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "条目不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge/{id} [delete]
func (h *StudioKnowledgeHandler) DeleteStudioKnowledge(c *gin.Context) {
	id, ok := parseKnowledgeID(c)
	if !ok {
		return
	}

	if err := h.knowledge.Delete(c.Request.Context(), id); err != nil {
		failStudioKnowledge(c, err)
		return
	}
	response.OKWithMsg(c, "条目已删除", nil)
}

// ListStudioKnowledgeRevisions 修订列表。
// @Summary 知识库修订列表
// @Description 按版本倒序列出某条目的全部修订快照。
// @Tags 管理端-工作室知识库
// @Produce json
// @Param id path int true "条目 ID"
// @Success 200 {object} response.Result{data=response.StudioKnowledgeRevisionListResp} "成功"
// @Failure 400 {object} response.Result "条目不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge/{id}/revisions [get]
func (h *StudioKnowledgeHandler) ListStudioKnowledgeRevisions(c *gin.Context) {
	id, ok := parseKnowledgeID(c)
	if !ok {
		return
	}

	revisions, err := h.knowledge.Revisions(c.Request.Context(), id)
	if err != nil {
		failStudioKnowledge(c, err)
		return
	}

	response.OK(c, response.StudioKnowledgeRevisionListResp{
		Items: response.NewStudioKnowledgeRevisionResps(revisions),
		Total: len(revisions),
	})
}

// GetStudioKnowledgeRevision 指定版本快照。
// @Summary 知识库修订详情
// @Description 取某条目的指定版本快照（用于「看看上一版写了什么」）。
// @Tags 管理端-工作室知识库
// @Produce json
// @Param id path int true "条目 ID"
// @Param version path int true "版本号"
// @Success 200 {object} response.Result{data=response.StudioKnowledgeRevisionResp} "成功"
// @Failure 400 {object} response.Result "该版本不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/studio/knowledge/{id}/revisions/{version} [get]
func (h *StudioKnowledgeHandler) GetStudioKnowledgeRevision(c *gin.Context) {
	id, ok := parseKnowledgeID(c)
	if !ok {
		return
	}

	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		response.FailInvalidParam(c, "version 必须是正整数")
		return
	}

	revision, err := h.knowledge.Revision(c.Request.Context(), id, version)
	if err != nil {
		failStudioKnowledge(c, err)
		return
	}
	response.OK(c, response.NewStudioKnowledgeRevisionResp(revision))
}
