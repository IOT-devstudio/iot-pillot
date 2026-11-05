package handler

import (
	"errors"
	"strconv"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// ForumHandler 招新问答 / 内部论坛的只读端点（#219）。
//
// 写入（新建帖/回复、软删、标记已解决）属于 #220/#221，本期不暴露。
//
// scope 由路由分组决定（recruitment 挂登录用户组、studio 挂 admin 组），
// 这里只接收、不解析 URL —— 角色隔离在 router.go 的 Group 层完成（ADR 0003 决策 3）。
type ForumHandler struct {
	forum *service.ForumUseCase
}

// NewForumHandler 构造函数。
func NewForumHandler(forum *service.ForumUseCase) *ForumHandler {
	return &ForumHandler{forum: forum}
}

// failForum 把论坛域错误映射成 HTTP 响应。
//
// ErrForumPostNotFound 一律 404（不是 403）：软删与跨域越权都不该向调用方暴露
// 「这个 id 存在」，否则可被用来枚举内部论坛规模（ADR 0004 §1 同口径）。
func failForum(c *gin.Context, err error) {
	if errors.Is(err, service.ErrForumPostNotFound) {
		response.FailNotFound(c, "帖子不存在")
		return
	}
	failInternal(c, err)
}

// parseForumID 解析路径里的帖子 id（正整数）。
func parseForumID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return 0, false
	}
	return id, true
}

// parseDirectionCode 解析方向筛选参数。
//
// 缺省（空串）= 全部：不加方向条件，空方向的帖子也出现（ADR 0003 决策 7）；
// 传具体值 = 只看该方向（direction_code = ? 自然排除 NULL）。
func parseDirectionCode(raw string) *string {
	if raw == "" {
		return nil
	}
	v := raw
	return &v
}

// ListForumPosts 帖子列表（按 scope 区分招新问答 / 内部论坛）。
// @Summary 帖子列表
// @Description 分页列出某内容域下未软删的帖子，支持关键词（标题/正文）与方向筛选。登录用户可读招新问答，管理员可读内部论坛。
// @Tags 招新问答
// @Produce json
// @Param q query string false "关键词（标题与正文模糊匹配，不搜回复）"
// @Param direction query string false "方向码筛选；缺省为全部（含空方向）"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.ForumPostListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "帖子不存在"
// @Security BearerAuth
// @Router /api/v1/forum/posts [get]
func (h *ForumHandler) ListForumPosts(c *gin.Context, scope domain.ForumScope) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	filter := repository.ForumPostListFilter{
		Query:         c.Query("q"),
		DirectionCode: parseDirectionCode(c.Query("direction")),
	}

	items, total, err := h.forum.ListPosts(c.Request.Context(), scope, filter, page, pageSize)
	if err != nil {
		failForum(c, err)
		return
	}
	response.OK(c, response.NewForumPostListResp(items, total, page, pageSize))
}

// GetForumPost 帖子详情（按 scope 区分招新问答 / 内部论坛）。
// @Summary 帖子详情
// @Description 取帖子正文与回复列表；回复按官方回答（管理员回复）置顶后时间正序。软删或不存在一律 404。
// @Tags 招新问答
// @Produce json
// @Param id path int true "帖子 id"
// @Success 200 {object} response.Result{data=response.ForumPostDetailResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "帖子不存在"
// @Security BearerAuth
// @Router /api/v1/forum/posts/{id} [get]
//
// 内部论坛（studio scope）复用同一对 handler，仅路由分组不同：
//   - GET /api/v1/studio/forum/posts          （仅管理员）
//   - GET /api/v1/studio/forum/posts/{id}     （仅管理员）
// 端点契约与招新问答一致，区别只在 scope 注入与鉴权分组（ADR 0003 决策 3）。
func (h *ForumHandler) GetForumPost(c *gin.Context, scope domain.ForumScope) {
	id, ok := parseForumID(c)
	if !ok {
		return
	}

	detail, err := h.forum.GetPostDetail(c.Request.Context(), id)
	if err != nil {
		failForum(c, err)
		return
	}
	// scope 由路由分组保证，这里二次校验帖子确实属于该 scope：
	// 跨域直访（拿招新问答的 id 打 /studio/forum）返回 404 而非 403（ADR 0003 决策 3）。
	if detail.Post.Scope != scope {
		response.FailNotFound(c, "帖子不存在")
		return
	}
	response.OK(c, response.NewForumPostDetailResp(detail.Post, detail.Replies, detail.Official))
}
