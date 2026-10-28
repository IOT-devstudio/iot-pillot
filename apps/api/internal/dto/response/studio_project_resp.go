package response

import (
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

// StudioProjectResp 工作室项目（管理端读，number ID）。
//
// 与 packages/shared-types/src/studio.ts 的 StudioProject 一一对应。
type StudioProjectResp struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Year      string `json:"year"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// StudioProjectListResp 项目分页列表。
type StudioProjectListResp struct {
	Items    []StudioProjectResp `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// StudioProjectMemberResp 一条项目参与。
//
// Role 与 MemberID 同时返回：MemberID 可空表示外部参与者，
// 前端据此显示「仅外部姓名」或「已关联成员档案」。
type StudioProjectMemberResp struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"project_id"`
	MemberID  *int   `json:"member_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	StartedAt string `json:"started_at"`
	// EndedAt 为 null 表示「仍在参与」；用 *string 而不是空串，
	// 前端靠 null 判断，与成员档案的 archived_at 同一口径。
	EndedAt   *string `json:"ended_at"`
	CreatedAt string  `json:"created_at"`
}

// StudioProjectMemberListResp 参与名单。
type StudioProjectMemberListResp struct {
	Items []StudioProjectMemberResp `json:"items"`
	Total int                       `json:"total"`
}

// StudioProjectDetailResp 项目详情 = 元数据 + 参与名单。
type StudioProjectDetailResp struct {
	Project StudioProjectResp         `json:"project"`
	Members []StudioProjectMemberResp `json:"members"`
}

// projectRespTimeLayout 与领域层保持一致：响应里的时间必须和前端发回来的
// 格式对得上，否则「开始时间」会被解析成零值，参与行就被静默当成「还没开始」。
const projectRespTimeLayout = domain.ProjectTimeLayout

// isoOrEmpty 时间 → RFC3339 字符串；零值返回空串。
func isoOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(projectRespTimeLayout)
}

// isoOrEmptyAt 指针时间 → RFC3339 字符串；nil 返回空串。
func isoOrEmptyAt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return isoOrEmpty(*t)
}

// isoOrEmptyPtr 指针时间 → RFC3339 字符串指针；nil 返回 nil（序列化成 null）。
func isoOrEmptyPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := isoOrEmpty(*t)
	return &formatted
}

// NewStudioProjectResp 领域项目 → 响应 DTO。
func NewStudioProjectResp(p *domain.StudioProject) StudioProjectResp {
	return StudioProjectResp{
		ID:        p.ID,
		Name:      p.Name,
		Year:      p.Year,
		Summary:   p.Summary,
		Status:    string(p.Status),
		CreatedAt: isoOrEmpty(p.CreatedAt),
		UpdatedAt: isoOrEmpty(p.UpdatedAt),
	}
}

// NewStudioProjectResps 批量转换（保证空列表序列化成 []，不是 null）。
func NewStudioProjectResps(projects []*domain.StudioProject) []StudioProjectResp {
	out := make([]StudioProjectResp, 0, len(projects))
	for _, p := range projects {
		out = append(out, NewStudioProjectResp(p))
	}
	return out
}

// NewStudioProjectMemberResp 领域参与行 → 响应 DTO。
func NewStudioProjectMemberResp(m *domain.StudioProjectMember) StudioProjectMemberResp {
	return StudioProjectMemberResp{
		ID:        m.ID,
		ProjectID: m.ProjectID,
		MemberID:  m.MemberID,
		Name:      m.Name,
		Role:      string(m.Role),
		StartedAt: isoOrEmpty(m.StartedAt),
		EndedAt:   isoOrEmptyPtr(m.EndedAt),
		CreatedAt: isoOrEmpty(m.CreatedAt),
	}
}

// NewStudioProjectMemberResps 批量转换。
func NewStudioProjectMemberResps(members []*domain.StudioProjectMember) []StudioProjectMemberResp {
	out := make([]StudioProjectMemberResp, 0, len(members))
	for _, m := range members {
		out = append(out, NewStudioProjectMemberResp(m))
	}
	return out
}

// NewStudioProjectDetailResp 组装项目详情。
func NewStudioProjectDetailResp(p *domain.StudioProject, members []*domain.StudioProjectMember) StudioProjectDetailResp {
	return StudioProjectDetailResp{
		Project: NewStudioProjectResp(p),
		Members: NewStudioProjectMemberResps(members),
	}
}
