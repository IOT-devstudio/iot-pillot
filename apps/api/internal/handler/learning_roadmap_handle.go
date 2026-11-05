package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"
)

// LearningRoadmapHandler 学习路线管理端接口（#223）。
type LearningRoadmapHandler struct {
	useCase *service.LearningRoadmapUseCase
}

// NewLearningRoadmapHandler 构造函数。
func NewLearningRoadmapHandler(useCase *service.LearningRoadmapUseCase) *LearningRoadmapHandler {
	return &LearningRoadmapHandler{useCase: useCase}
}

// learningRoadmapResourceReq 资源链接入参。
type learningRoadmapResourceReq struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// learningRoadmapStageReq 阶段入参。
//
// ID 由服务端生成，**前端不生成**（ADR 0006 决策 3）。
// 保存时带上的 id 只在「库里确实存在」时才被保留。
type learningRoadmapStageReq struct {
	ID        string                       `json:"id"`
	Title     string                       `json:"title"`
	Summary   string                       `json:"summary"`
	Resources []learningRoadmapResourceReq `json:"resources"`
}

// learningRoadmapWriteReq 创建与整体保存共用的请求体。
type learningRoadmapWriteReq struct {
	Title         string                    `json:"title"`
	Body          string                    `json:"body"`
	DirectionCode string                    `json:"direction_code"`
	Stages        []learningRoadmapStageReq `json:"stages"`
	// Version 只在保存时有意义（乐观锁）；创建时忽略。
	Version int `json:"version"`
}

func (r learningRoadmapWriteReq) toStages() domain.RoadmapStages {
	out := make(domain.RoadmapStages, 0, len(r.Stages))
	for _, s := range r.Stages {
		res := make([]domain.RoadmapResource, 0, len(s.Resources))
		for _, x := range s.Resources {
			res = append(res, domain.RoadmapResource{Title: x.Title, URL: x.URL})
		}
		out = append(out, domain.RoadmapStage{
			ID: s.ID, Title: s.Title, Summary: s.Summary, Resources: res,
		})
	}
	return out
}

// ListLearningRoadmaps 分页列出路线。
//
// @Summary      学习路线列表
// @Description  管理端与用户端共用；支持按方向筛选，响应一并返回只含路线的方向筛选项。
// @Tags         学习路线
// @Produce      json
// @Param        direction  query  string  false  "方向 code，缺省为全部"
// @Param        page       query  int     false  "页码，默认 1"
// @Param        page_size  query  int     false  "每页条数，默认 20，上限 100"
// @Success      200  {object}  response.Response{data=learningRoadmapListResp}
// @Router       /learning/roadmaps [get]
func (h *LearningRoadmapHandler) ListLearningRoadmaps(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 1<<20)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	rows, total, err := h.useCase.List(c.Request.Context(), repository.LearningRoadmapListFilter{
		DirectionCode: c.Query("direction"),
	}, page, pageSize)
	if err != nil {
		h.fail(c, err)
		return
	}

	// 方向筛选项随列表返回（ADR 0006 决策 8）：不查招新方向接口，
	// 否则招新结束后方向全关、路线页会一个方向都不剩。
	directions, err := h.useCase.Directions(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}

	items := make([]learningRoadmapItemResp, 0, len(rows))
	for _, r := range rows {
		items = append(items, newLearningRoadmapItemResp(r))
	}
	dirs := make([]learningRoadmapDirectionResp, 0, len(directions))
	for _, d := range directions {
		dirs = append(dirs, learningRoadmapDirectionResp{Code: d.Code, Name: d.Name, Count: d.Count})
	}

	response.OK(c, learningRoadmapListResp{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		Directions: dirs,
	})
}

// GetLearningRoadmap 取单条路线详情。
//
// @Summary      学习路线详情
// @Tags         学习路线
// @Produce      json
// @Param        id  path  int  true  "路线 ID"
// @Success      200  {object}  response.Response{data=learningRoadmapDetailResp}
// @Router       /learning/roadmaps/{id} [get]
func (h *LearningRoadmapHandler) GetLearningRoadmap(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	row, err := h.useCase.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, newLearningRoadmapDetailResp(row))
}

// CreateLearningRoadmap 新建路线。
//
// @Summary      新建学习路线
// @Tags         学习路线
// @Accept       json
// @Produce      json
// @Param        body  body  learningRoadmapWriteReq  true  "路线内容"
// @Success      200  {object}  response.Response{data=learningRoadmapDetailResp}
// @Router       /studio/roadmaps [post]
func (h *LearningRoadmapHandler) CreateLearningRoadmap(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		response.FailUnauthorized(c, "登录已失效，请重新登录")
		return
	}
	var req learningRoadmapWriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求内容格式不正确")
		return
	}

	row, err := h.useCase.Create(c.Request.Context(), userID, service.CreateLearningRoadmapInput{
		Title:         req.Title,
		Body:          req.Body,
		DirectionCode: req.DirectionCode,
		Stages:        req.toStages(),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, newLearningRoadmapDetailResp(row))
}

// UpdateLearningRoadmap 整体保存路线（版本号乐观锁）。
//
// @Summary      保存学习路线
// @Description  整体覆盖标题、正文与阶段列表；version 必须与当前版本一致，否则返回冲突。
// @Tags         学习路线
// @Accept       json
// @Produce      json
// @Param        id    path  int                      true  "路线 ID"
// @Param        body  body  learningRoadmapWriteReq  true  "路线内容"
// @Success      200  {object}  response.Response{data=learningRoadmapDetailResp}
// @Router       /studio/roadmaps/{id} [put]
func (h *LearningRoadmapHandler) UpdateLearningRoadmap(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	userID := currentUserID(c)
	if userID == 0 {
		response.FailUnauthorized(c, "登录已失效，请重新登录")
		return
	}
	var req learningRoadmapWriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求内容格式不正确")
		return
	}

	row, err := h.useCase.Update(c.Request.Context(), id, userID, service.UpdateLearningRoadmapInput{
		Title:         req.Title,
		Body:          req.Body,
		DirectionCode: req.DirectionCode,
		Stages:        req.toStages(),
	}, req.Version)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, newLearningRoadmapDetailResp(row))
}

// DeleteLearningRoadmap 软删路线。
//
// @Summary      删除学习路线
// @Description  软删除并记录操作人，不做物理删除。
// @Tags         学习路线
// @Produce      json
// @Param        id  path  int  true  "路线 ID"
// @Success      200  {object}  response.Response
// @Router       /studio/roadmaps/{id} [delete]
func (h *LearningRoadmapHandler) DeleteLearningRoadmap(c *gin.Context) {
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	userID := currentUserID(c)
	if userID == 0 {
		response.FailUnauthorized(c, "登录已失效，请重新登录")
		return
	}
	if err := h.useCase.Delete(c.Request.Context(), id, userID); err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// fail 把领域错误映射成 HTTP 响应。
//
// 404 而不是 403：软删的路线当作不存在，避免用 403 暴露资源存在（ADR 0004 §1 同口径）。
func (h *LearningRoadmapHandler) fail(c *gin.Context, err error) {
	var validation *domain.RoadmapValidationError
	switch {
	case errors.As(err, &validation):
		// 带可读原因：用户要知道「第 3 个阶段的第 1 个资源链接不合规」，
		// 而不是笼统的「内容不合法」。
		response.FailInvalidParam(c, validation.Reason)
	case errors.Is(err, domain.ErrRoadmapVersionConflict):
		// 版本冲突也走 400：它不是服务端故障，调用方要做的是「刷新后重试」。
		// 409 在本仓库没有先例（知识库 #196 同样选择 400），前端也没有
		// 对应的处理分支——引入一个新状态码会让它变成一个没人处理的分支。
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, domain.ErrRoadmapNotFound):
		response.FailNotFound(c, domain.ErrRoadmapNotFound.Error())
	default:
		response.FailServer(c, "操作失败，请稍后重试")
	}
}

/* ------------------------------------------------------------------ *
 * 响应形态
 * ------------------------------------------------------------------ */

type learningRoadmapResourceResp struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type learningRoadmapStageResp struct {
	ID        string                        `json:"id"`
	Title     string                        `json:"title"`
	Summary   string                        `json:"summary"`
	Resources []learningRoadmapResourceResp `json:"resources"`
}

// learningRoadmapItemResp 列表项。
//
// 正文只在详情返回：列表页不渲染正文，读整列没有意义（ADR 0006 决策 7）。
// 同一条理由也决定列表不返回阶段标题——只给阶段数。
type learningRoadmapItemResp struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	DirectionCode string `json:"direction_code"`
	StageCount    int    `json:"stage_count"`
	UpdatedAt     string `json:"updated_at"`
}

type learningRoadmapDetailResp struct {
	ID            int                        `json:"id"`
	Title         string                     `json:"title"`
	Body          string                     `json:"body"`
	DirectionCode string                     `json:"direction_code"`
	Stages        []learningRoadmapStageResp `json:"stages"`
	// Version 回给前端用于下一次保存的乐观锁。
	Version    int    `json:"version"`
	StageCount int    `json:"stage_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type learningRoadmapDirectionResp struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type learningRoadmapListResp struct {
	Items    []learningRoadmapItemResp `json:"items"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	// Directions 只含**有路线**的方向，不看招新开关（ADR 0006 决策 8）。
	Directions []learningRoadmapDirectionResp `json:"directions"`
}

func newLearningRoadmapItemResp(r *domain.LearningRoadmap) learningRoadmapItemResp {
	return learningRoadmapItemResp{
		ID:            r.ID,
		Title:         r.Title,
		DirectionCode: r.DirectionCode,
		StageCount:    len(r.Stages),
		UpdatedAt:     r.UpdatedAt.Format(rfc3339Layout),
	}
}

func newLearningRoadmapDetailResp(r *domain.LearningRoadmap) learningRoadmapDetailResp {
	stages := make([]learningRoadmapStageResp, 0, len(r.Stages))
	for _, s := range r.Stages {
		res := make([]learningRoadmapResourceResp, 0, len(s.Resources))
		for _, x := range s.Resources {
			res = append(res, learningRoadmapResourceResp{Title: x.Title, URL: x.URL})
		}
		stages = append(stages, learningRoadmapStageResp{
			ID: s.ID, Title: s.Title, Summary: s.Summary, Resources: res,
		})
	}
	return learningRoadmapDetailResp{
		ID:            r.ID,
		Title:         r.Title,
		Body:          r.Body,
		DirectionCode: r.DirectionCode,
		Stages:        stages,
		Version:       r.Version,
		StageCount:    len(r.Stages),
		CreatedAt:     r.CreatedAt.Format(rfc3339Layout),
		UpdatedAt:     r.UpdatedAt.Format(rfc3339Layout),
	}
}

// rfc3339Layout 时间序列化格式。
//
// 与论坛/知识库的响应保持一致（同一条链路上不该有两种时间格式）。
const rfc3339Layout = "2006-01-02T15:04:05Z07:00"

// parsePathID 解析路径里的 :id。
//
// 非正整数也回 404 而不是 400：用不同状态码区分「id 格式不对」与「id 不存在」
// 会让调用方多一条要处理的分支，而两者对用户是同一件事——这里没有这个资源。
func parsePathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailNotFound(c, domain.ErrRoadmapNotFound.Error())
		return 0, false
	}
	return id, true
}
