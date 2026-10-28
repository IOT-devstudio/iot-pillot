package response

import "github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

// StudioMemberResp 工作室成员档案（管理端读，number ID）。
//
// 与 packages/shared-types/src/studio.ts 的 StudioMember 一一对应：
// 字段名保持 snake_case，时间是 RFC3339 字符串，空值用 null 而不是空串——
// 前端靠 null 判断「没有关联账号 / 未归档」。
type StudioMemberResp struct {
	ID           int    `json:"id"`
	UserID       *int   `json:"user_id"`
	Name         string `json:"name"`
	BirthYear    int    `json:"birth_year"`
	Cohort       string `json:"cohort"`
	Direction    string `json:"direction"`
	RoleInStudio string `json:"role_in_studio"`
	Internship   string `json:"internship"`
	Employment   string `json:"employment"`
	IsActive     bool   `json:"is_active"`
	// ArchivedAt 归档时刻；未归档为 null（不是空串）——
	// 共享类型里它是 ISODateTime | null，前端直接用 null 判断归档态。
	ArchivedAt        *string `json:"archived_at"`
	SourceCandidateID *int    `json:"source_candidate_id"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// StudioMemberListResp 成员分页列表。
type StudioMemberListResp struct {
	Items    []StudioMemberResp `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// NewStudioMemberResp 领域成员 → 响应 DTO。
func NewStudioMemberResp(m *domain.StudioMember) StudioMemberResp {
	var archivedAt *string
	if m.ArchivedAt != nil {
		stamp := m.ArchivedAt.Format("2006-01-02T15:04:05Z07:00")
		archivedAt = &stamp
	}
	return StudioMemberResp{
		ID:                m.ID,
		UserID:            m.UserID,
		Name:              m.Name,
		BirthYear:         m.BirthYear,
		Cohort:            m.Cohort,
		Direction:         m.Direction,
		RoleInStudio:      m.RoleInStudio,
		Internship:        m.Internship,
		Employment:        m.Employment,
		IsActive:          m.IsActive,
		ArchivedAt:        archivedAt,
		SourceCandidateID: m.SourceCandidateID,
		CreatedAt:         m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// NewStudioMemberResps 批量转换（保证空列表序列化成 []，不是 null）。
func NewStudioMemberResps(members []*domain.StudioMember) []StudioMemberResp {
	out := make([]StudioMemberResp, 0, len(members))
	for _, m := range members {
		out = append(out, NewStudioMemberResp(m))
	}
	return out
}
