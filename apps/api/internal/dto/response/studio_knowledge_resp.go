package response

import "github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

// StudioKnowledgeEntryResp 知识库条目（管理端读，number ID）。
//
// 与 packages/shared-types/src/studio.ts 的 StudioKnowledgeEntry 一一对应。
// is_public 一并返回（迁移里的兼容列），但它是从 permission 派生的只读值——
// 前端不要用它反推权限，三档权限只有 permission 表达得了。
type StudioKnowledgeEntryResp struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	AuthorID   int    `json:"author_id"`
	IsPublic   bool   `json:"is_public"`
	Permission string `json:"permission"`
	ParentID   *int   `json:"parent_id"`
	Version    int    `json:"version"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// StudioKnowledgeListResp 条目分页列表。
type StudioKnowledgeListResp struct {
	Items    []StudioKnowledgeEntryResp `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

// StudioKnowledgeRevisionResp 一条修订快照。
type StudioKnowledgeRevisionResp struct {
	ID        int    `json:"id"`
	EntryID   int    `json:"entry_id"`
	Version   int    `json:"version"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	EditorID  int    `json:"editor_id"`
	CreatedAt string `json:"created_at"`
}

// StudioKnowledgeRevisionListResp 修订列表（版本倒序）。
type StudioKnowledgeRevisionListResp struct {
	Items []StudioKnowledgeRevisionResp `json:"items"`
	Total int                           `json:"total"`
}

const knowledgeTimeLayout = "2006-01-02T15:04:05Z07:00"

// NewStudioKnowledgeEntryResp 领域条目 → 响应 DTO。
func NewStudioKnowledgeEntryResp(e *domain.StudioKnowledgeEntry) StudioKnowledgeEntryResp {
	return StudioKnowledgeEntryResp{
		ID:         e.ID,
		Title:      e.Title,
		Body:       e.Body,
		AuthorID:   e.AuthorID,
		IsPublic:   e.IsPublic,
		Permission: string(e.Permission),
		ParentID:   e.ParentID,
		Version:    e.Version,
		CreatedAt:  e.CreatedAt.Format(knowledgeTimeLayout),
		UpdatedAt:  e.UpdatedAt.Format(knowledgeTimeLayout),
	}
}

// NewStudioKnowledgeEntryResps 批量转换（保证空列表序列化成 []，不是 null）。
func NewStudioKnowledgeEntryResps(entries []*domain.StudioKnowledgeEntry) []StudioKnowledgeEntryResp {
	out := make([]StudioKnowledgeEntryResp, 0, len(entries))
	for _, e := range entries {
		out = append(out, NewStudioKnowledgeEntryResp(e))
	}
	return out
}

// NewStudioKnowledgeRevisionResp 领域修订 → 响应 DTO。
func NewStudioKnowledgeRevisionResp(r *domain.StudioKnowledgeRevision) StudioKnowledgeRevisionResp {
	return StudioKnowledgeRevisionResp{
		ID:        r.ID,
		EntryID:   r.EntryID,
		Version:   r.Version,
		Title:     r.Title,
		Body:      r.Body,
		EditorID:  r.EditorID,
		CreatedAt: r.CreatedAt.Format(knowledgeTimeLayout),
	}
}

// NewStudioKnowledgeRevisionResps 批量转换。
func NewStudioKnowledgeRevisionResps(revisions []*domain.StudioKnowledgeRevision) []StudioKnowledgeRevisionResp {
	out := make([]StudioKnowledgeRevisionResp, 0, len(revisions))
	for _, r := range revisions {
		out = append(out, NewStudioKnowledgeRevisionResp(r))
	}
	return out
}
