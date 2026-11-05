package response

import (
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// ForumPostSummaryResp 列表项（ADR 0003：列表不渲染正文，只显示摘要与元信息）。
type ForumPostSummaryResp struct {
	ID            int     `json:"id"`
	Scope         string  `json:"scope"`
	Title         string  `json:"title"`
	AuthorID      int     `json:"author_id"`
	AuthorName    string  `json:"author_name"`
	DirectionCode *string `json:"direction_code"`
	// Body 下发给前端，由 markdownExcerpt 生成纯文本预览（不渲染 Markdown VNode，
	// 见 ADR 0005 决策 8）；列表本身不渲染正文。
	Body          string  `json:"body"`
	ReplyCount    int64   `json:"reply_count"`
	HasImage      bool    `json:"has_image"`
	IsResolved    bool    `json:"is_resolved"`
	LastActiveAt  string  `json:"last_active_at"`
	CreatedAt     string  `json:"created_at"`
}

// ForumPostListResp 帖子分页列表。
type ForumPostListResp struct {
	Items    []ForumPostSummaryResp `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// ForumReplyResp 一条回复（含是否官方回答）。
type ForumReplyResp struct {
	ID        int    `json:"id"`
	PostID    int    `json:"post_id"`
	AuthorID  int    `json:"author_id"`
	Body      string `json:"body"`
	IsOfficial bool  `json:"is_official"`
	EditedAt  *string `json:"edited_at"`
	CreatedAt string `json:"created_at"`
}

// ForumPostDetailResp 帖子详情：正文 + 回复列表（官方回答已置顶）。
type ForumPostDetailResp struct {
	ID            int              `json:"id"`
	Scope         string           `json:"scope"`
	Title         string           `json:"title"`
	AuthorID      int              `json:"author_id"`
	DirectionCode *string          `json:"direction_code"`
	Body          string           `json:"body"`
	IsResolved    bool             `json:"is_resolved"`
	ResolvedAt    *string          `json:"resolved_at"`
	EditedAt      *string          `json:"edited_at"`
	CreatedAt     string           `json:"created_at"`
	Replies       []ForumReplyResp `json:"replies"`
}

const forumTimeLayout = "2006-01-02T15:04:05Z07:00"

func ptrTimeStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(forumTimeLayout)
	return &s
}

// NewForumPostSummaryResp 仓储摘要 → 响应 DTO。
func NewForumPostSummaryResp(s *repository.ForumPostSummary) ForumPostSummaryResp {
	return ForumPostSummaryResp{
		ID:            s.ID,
		Scope:         s.Scope,
		Title:         s.Title,
		AuthorID:      s.AuthorID,
		AuthorName:    s.AuthorName,
		DirectionCode: s.DirectionCode,
		Body:          s.Body,
		ReplyCount:    s.ReplyCount,
		HasImage:      s.HasImage,
		IsResolved:    s.IsResolved,
		LastActiveAt:  s.LastActiveAt.Format(forumTimeLayout),
		CreatedAt:     s.CreatedAt.Format(forumTimeLayout),
	}
}

// NewForumPostSummaryResps 批量转换，空列表序列化为 [] 而非 null。
func NewForumPostSummaryResps(items []*repository.ForumPostSummary) []ForumPostSummaryResp {
	out := make([]ForumPostSummaryResp, 0, len(items))
	for _, s := range items {
		out = append(out, NewForumPostSummaryResp(s))
	}
	return out
}

// NewForumPostListResp 组装分页响应（data 内带 total/page/page_size，见 Result 无 meta 约定）。
func NewForumPostListResp(items []*repository.ForumPostSummary, total int64, page int, pageSize int) ForumPostListResp {
	return ForumPostListResp{
		Items:    NewForumPostSummaryResps(items),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// NewForumPostDetailResp 帖子详情 → 响应 DTO。
//
// replies 已按官方回答置顶排好，official[i] 与 replies[i] 对齐。
func NewForumPostDetailResp(post *domain.ForumPost, replies []*domain.ForumReply, official []bool) ForumPostDetailResp {
	replyResps := make([]ForumReplyResp, 0, len(replies))
	for i, r := range replies {
		replyResps = append(replyResps, ForumReplyResp{
			ID:         r.ID,
			PostID:     r.PostID,
			AuthorID:   r.AuthorID,
			Body:       r.Body,
			IsOfficial: i < len(official) && official[i],
			EditedAt:   ptrTimeStr(r.EditedAt),
			CreatedAt:  r.CreatedAt.Format(forumTimeLayout),
		})
	}
	return ForumPostDetailResp{
		ID:            post.ID,
		Scope:         string(post.Scope),
		Title:         post.Title,
		AuthorID:      post.AuthorID,
		DirectionCode: post.DirectionCode,
		Body:          post.Body,
		IsResolved:    post.ResolvedAt != nil,
		ResolvedAt:    ptrTimeStr(post.ResolvedAt),
		EditedAt:      ptrTimeStr(post.EditedAt),
		CreatedAt:     post.CreatedAt.Format(forumTimeLayout),
		Replies:       replyResps,
	}
}
