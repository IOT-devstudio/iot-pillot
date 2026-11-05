package handler

import (
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 请求体
 * ------------------------------------------------------------------ */

// forumCreatePostReq 新建帖子请求体。
//
// direction_code 用指针区分「没传」（nil，走回落）与「传了空串」（视同没选，也走回落）。
// scope 不在这里：它由路由分组决定，放进请求体会让用户能把招新帖写进内部论坛。
type forumCreatePostReq struct {
	Title         string  `json:"title"`
	Body          string  `json:"body"`
	DirectionCode *string `json:"direction_code"`
}

// forumUpdatePostReq 编辑帖子请求体（方向不可改）。
type forumUpdatePostReq struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// forumReplyReq 新建/编辑回复请求体（两者字段相同）。
type forumReplyReq struct {
	Body string `json:"body"`
}

/* ------------------------------------------------------------------ *
 * 帖子写入
 * ------------------------------------------------------------------ */

// CreateForumPost 新建帖子。
// @Summary 新建帖子
// @Description 登录用户可发帖。未选方向时服务端按「资料方向 → 最新报名方向 → 留空」回落。
// @Tags 招新问答
// @Accept json
// @Produce json
// @Param body body forumCreatePostReq true "标题与正文"
// @Success 200 {object} response.Result{data=response.ForumPostDetailResp} "成功"
// @Failure 400 {object} response.Result "参数不合法（标题为空/超长等）"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/forum/posts [post]
//
// 内部论坛复用同一实现（POST /api/v1/studio/forum/posts，仅管理员）。
func (h *ForumHandler) CreateForumPost(c *gin.Context, scope domain.ForumScope) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}
	var req forumCreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求体格式错误")
		return
	}

	post, err := h.forum.CreatePost(c.Request.Context(), userID, scope, service.CreateForumPostInput{
		Title:         req.Title,
		Body:          req.Body,
		DirectionCode: req.DirectionCode,
	})
	if err != nil {
		failForum(c, err)
		return
	}
	// 新建后回复列表必然为空，直接用详情 DTO 返回，前端拿到即可渲染。
	response.OK(c, response.NewForumPostDetailResp(post, nil, nil))
}

// UpdateForumPost 编辑帖子。
// @Summary 编辑帖子
// @Description 仅作者本人或管理员可编辑；编辑后记录 edited_at。无权与不存在一律 404。
// @Tags 招新问答
// @Accept json
// @Produce json
// @Param id path int true "帖子 id"
// @Param body body forumUpdatePostReq true "新标题与正文"
// @Success 200 {object} response.Result{data=response.ForumPostDetailResp} "成功"
// @Failure 400 {object} response.Result "参数不合法"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "帖子不存在或无权编辑"
// @Security BearerAuth
// @Router /api/v1/forum/posts/{id} [patch]
func (h *ForumHandler) UpdateForumPost(c *gin.Context, scope domain.ForumScope) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}
	id, ok := parseForumID(c)
	if !ok {
		return
	}
	if !h.assertPostScope(c, id, scope) {
		return
	}

	var req forumUpdatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求体格式错误")
		return
	}

	post, err := h.forum.UpdatePost(c.Request.Context(), userID, id, service.UpdateForumPostInput{
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		failForum(c, err)
		return
	}
	response.OK(c, response.NewForumPostDetailResp(post, nil, nil))
}

// DeleteForumPost 软删帖子。
// @Summary 删除帖子
// @Description 仅作者本人或管理员可删除；软删后列表与详情的搜索与访问都会 404。附件行与对象保留。
// @Tags 招新问答
// @Produce json
// @Param id path int true "帖子 id"
// @Success 200 {object} response.Result "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "帖子不存在或无权删除"
// @Security BearerAuth
// @Router /api/v1/forum/posts/{id} [delete]
func (h *ForumHandler) DeleteForumPost(c *gin.Context, scope domain.ForumScope) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}
	id, ok := parseForumID(c)
	if !ok {
		return
	}
	if !h.assertPostScope(c, id, scope) {
		return
	}

	if err := h.forum.DeletePost(c.Request.Context(), userID, id); err != nil {
		failForum(c, err)
		return
	}
	response.OK(c, nil)
}

/* ------------------------------------------------------------------ *
 * 回复写入
 * ------------------------------------------------------------------ */

// CreateForumReply 新建回复。
// @Summary 新建回复
// @Description 所有登录用户都能回复招新问答；帖子已软删时回复 404。
// @Tags 招新问答
// @Accept json
// @Produce json
// @Param id path int true "帖子 id"
// @Param body body forumReplyReq true "回复正文"
// @Success 200 {object} response.Result{data=response.ForumReplyResp} "成功"
// @Failure 400 {object} response.Result "参数不合法"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "帖子不存在"
// @Security BearerAuth
// @Router /api/v1/forum/posts/{id}/replies [post]
func (h *ForumHandler) CreateForumReply(c *gin.Context, scope domain.ForumScope) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}
	id, ok := parseForumID(c)
	if !ok {
		return
	}
	if !h.assertPostScope(c, id, scope) {
		return
	}

	var req forumReplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求体格式错误")
		return
	}

	reply, err := h.forum.CreateReply(c.Request.Context(), userID, id, service.CreateForumReplyInput{Body: req.Body})
	if err != nil {
		failForum(c, err)
		return
	}
	// 新建回复的官方标记需要现查管理员名单，交给 service 统一的详情路径更稳妥；
	// 这里只回单条，is_official 由前端下次拉详情时刷新（避免多一次 Redis 查询）。
	response.OK(c, response.ForumReplyResp{
		ID:        reply.ID,
		PostID:    reply.PostID,
		AuthorID:  reply.AuthorID,
		Body:      reply.Body,
		CreatedAt: reply.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdateForumReply 编辑回复。
// @Summary 编辑回复
// @Description 仅作者本人或管理员可编辑；编辑不触发邮件通知。
// @Tags 招新问答
// @Accept json
// @Produce json
// @Param id path int true "回复 id"
// @Param body body forumReplyReq true "新正文"
// @Success 200 {object} response.Result{data=response.ForumReplyResp} "成功"
// @Failure 400 {object} response.Result "参数不合法"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "回复不存在或无权编辑"
// @Security BearerAuth
// @Router /api/v1/forum/replies/{id} [patch]
func (h *ForumHandler) UpdateForumReply(c *gin.Context, scope domain.ForumScope) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}
	id, ok := parseForumID(c)
	if !ok {
		return
	}

	var req forumReplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求体格式错误")
		return
	}

	reply, err := h.forum.UpdateReply(c.Request.Context(), userID, id, service.CreateForumReplyInput{Body: req.Body})
	if err != nil {
		failForum(c, err)
		return
	}
	resp := response.ForumReplyResp{
		ID:        reply.ID,
		PostID:    reply.PostID,
		AuthorID:  reply.AuthorID,
		Body:      reply.Body,
		CreatedAt: reply.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if reply.EditedAt != nil {
		s := reply.EditedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.EditedAt = &s
	}
	response.OK(c, resp)
}

// DeleteForumReply 软删回复。
// @Summary 删除回复
// @Description 仅作者本人或管理员可删除；软删后详情里不再出现。
// @Tags 招新问答
// @Produce json
// @Param id path int true "回复 id"
// @Success 200 {object} response.Result "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "回复不存在或无权删除"
// @Security BearerAuth
// @Router /api/v1/forum/replies/{id} [delete]
func (h *ForumHandler) DeleteForumReply(c *gin.Context, scope domain.ForumScope) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}
	id, ok := parseForumID(c)
	if !ok {
		return
	}

	if err := h.forum.DeleteReply(c.Request.Context(), userID, id); err != nil {
		failForum(c, err)
		return
	}
	response.OK(c, nil)
}

/* ------------------------------------------------------------------ *
 * 辅助
 * ------------------------------------------------------------------ */

// assertPostScope 校验路径里的帖子确实属于当前路由分组的 scope。
//
// 存在的必要性：招新问答与内部论坛共用 handler，若不做这层校验，
// 管理员能用招新问答的 URL 操作内部论坛的帖子（反之亦然），
// 而 UI 上两条路径本该互不可见。跨域一律 404，不暴露存在性。
func (h *ForumHandler) assertPostScope(c *gin.Context, postID int, scope domain.ForumScope) bool {
	detail, err := h.forum.GetPostDetail(c.Request.Context(), postID)
	if err != nil {
		failForum(c, err)
		return false
	}
	if detail.Post.Scope != scope {
		response.FailNotFound(c, "帖子不存在")
		return false
	}
	return true
}
