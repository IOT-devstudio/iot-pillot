package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

/* ------------------------------------------------------------------ *
 * 状态与角色枚举
 * ------------------------------------------------------------------ */

// StudioProjectStatus 项目状态（ADR 0001 §2）。
//
// 与迁移 0014 的 chk_studio_projects_status 一一对应：
// 两边任何一处新增取值都必须同时改，否则合法值会在写库时被 CHECK 拒绝。
type StudioProjectStatus string

const (
	ProjectActive    StudioProjectStatus = "active"
	ProjectCompleted StudioProjectStatus = "completed"
	ProjectArchived  StudioProjectStatus = "archived"
)

// Valid 报告状态取值是否在支持范围内。
func (s StudioProjectStatus) Valid() bool {
	switch s {
	case ProjectActive, ProjectCompleted, ProjectArchived:
		return true
	}
	return false
}

// StudioProjectStatuses 全部可用状态（校验错误提示里列举用）。
func StudioProjectStatuses() []string {
	return []string{string(ProjectActive), string(ProjectCompleted), string(ProjectArchived)}
}

// StudioProjectMemberRole 项目参与角色。
type StudioProjectMemberRole string

const (
	ProjectMemberLead        StudioProjectMemberRole = "lead"
	ProjectMemberContributor StudioProjectMemberRole = "contributor"
)

// Valid 报告角色取值是否在支持范围内。
func (r StudioProjectMemberRole) Valid() bool {
	switch r {
	case ProjectMemberLead, ProjectMemberContributor:
		return true
	}
	return false
}

// StudioProjectMemberRoles 全部可用角色。
func StudioProjectMemberRoles() []string {
	return []string{string(ProjectMemberLead), string(ProjectMemberContributor)}
}

/* ------------------------------------------------------------------ *
 * 实体
 * ------------------------------------------------------------------ */

// StudioProject 工作室项目（迁移 0014）。
//
// Name 全局唯一（迁移的 idx_studio_projects_name）：项目名是人对项目的
// 唯一称呼，同名会让「哪个是哪个」永远说不清。
type StudioProject struct {
	ID        int                 `json:"id" gorm:"primaryKey"`
	Name      string              `json:"name" gorm:"column:name"`
	Year      string              `json:"year" gorm:"column:year"`
	Summary   string              `json:"summary" gorm:"column:summary"`
	Status    StudioProjectStatus `json:"status" gorm:"column:status"`
	CreatedAt time.Time           `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time           `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 显式表名（表名归迁移所有，不靠 GORM 复数推导）。
func (StudioProject) TableName() string { return "studio_projects" }

// StudioProjectMember 一条项目参与（迁移 0014）。
//
// Name 是**姓名快照**：成员改名或档案被删，历史参与仍要显示当时的名字。
// MemberID 可空 —— 外部参与者（没有成员档案）也能记录。
type StudioProjectMember struct {
	ID        int                     `json:"id" gorm:"primaryKey"`
	ProjectID int                     `json:"project_id" gorm:"column:project_id;index"`
	MemberID  *int                    `json:"member_id" gorm:"column:member_id"`
	Name      string                  `json:"name" gorm:"column:name"`
	Role      StudioProjectMemberRole `json:"role" gorm:"column:role"`
	StartedAt time.Time               `json:"started_at" gorm:"column:started_at"`
	EndedAt   *time.Time              `json:"ended_at" gorm:"column:ended_at"`
	CreatedAt time.Time               `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// TableName 显式表名。
func (StudioProjectMember) TableName() string { return "studio_project_members" }

/* ------------------------------------------------------------------ *
 * 字段口径
 * ------------------------------------------------------------------ */

const (
	// StudioProjectNameMaxRunes 项目名上限。
	StudioProjectNameMaxRunes = 100
	// StudioProjectYearMaxRunes 年份上限（展示用字符串，如 "2023" 或 "2023-2024"）。
	StudioProjectYearMaxRunes = 20
	// StudioProjectSummaryMaxRunes 项目简介上限（检票报告 §5 建议 ≤ 2000）。
	StudioProjectSummaryMaxRunes = 2000
	// StudioProjectMemberNameMaxRunes 参与者姓名快照上限。
	StudioProjectMemberNameMaxRunes = 40
)

// ProjectTimeLayout 项目参与时间的解析与序列化布局。
//
// 放在领域层而不是 request / handler：这是全域时间的口径，
// 服务层、请求层、响应层都按它解析和格式化，避免三处各写一份。
const ProjectTimeLayout = "2006-01-02T15:04:05Z07:00"

// ErrStudioProjectInvalid 项目字段不合法（消息里带具体原因）。
//
// 用哨兵而不是裸 errors.New：handler 必须能用 errors.Is 认出「这是校验失败」
// 并映射成 400，而不是掉进 500 的兜底分支。
var ErrStudioProjectInvalid = errors.New("工作室项目不合法")

// StudioProjectFields 项目字段快照（校验的输入）。
type StudioProjectFields struct {
	Name    string
	Year    string
	Summary string
	Status  StudioProjectStatus
}

// Normalize 去首尾空白并给状态兜默认值。
//
// 状态留空按 active 处理：与迁移 0014 的列默认值一致，
// 避免「不传 status」和「传 active」写出两种不同的行。
func (f StudioProjectFields) Normalize() StudioProjectFields {
	f.Name = strings.TrimSpace(f.Name)
	f.Year = strings.TrimSpace(f.Year)
	f.Summary = strings.TrimSpace(f.Summary)
	if f.Status == "" {
		f.Status = ProjectActive
	}
	return f
}

// Validate 校验字段口径。
func (f StudioProjectFields) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("%w: 项目名必填", ErrStudioProjectInvalid)
	}
	if utf8.RuneCountInString(f.Name) > StudioProjectNameMaxRunes {
		return fmt.Errorf("%w: 项目名最多 %d 个字符", ErrStudioProjectInvalid, StudioProjectNameMaxRunes)
	}
	if f.Year == "" {
		return fmt.Errorf("%w: 年份必填（如「2023」）", ErrStudioProjectInvalid)
	}
	if utf8.RuneCountInString(f.Year) > StudioProjectYearMaxRunes {
		return fmt.Errorf("%w: 年份最多 %d 个字符", ErrStudioProjectInvalid, StudioProjectYearMaxRunes)
	}
	if utf8.RuneCountInString(f.Summary) > StudioProjectSummaryMaxRunes {
		return fmt.Errorf("%w: 项目简介最多 %d 个字符", ErrStudioProjectInvalid, StudioProjectSummaryMaxRunes)
	}
	if !f.Status.Valid() {
		return fmt.Errorf("%w: 状态只能是 %s", ErrStudioProjectInvalid, strings.Join(StudioProjectStatuses(), " / "))
	}
	return nil
}

// ApplyTo 把字段写进实体。
func (f StudioProjectFields) ApplyTo(p *StudioProject) {
	p.Name = f.Name
	p.Year = f.Year
	p.Summary = f.Summary
	p.Status = f.Status
}

// StudioProjectMemberFields 参与行字段快照。
type StudioProjectMemberFields struct {
	// MemberID 可选：nil = 纯外部参与者（没有成员档案）。
	MemberID *int
	Name     string
	Role     StudioProjectMemberRole
	// StartedAt / EndedAt 参与起止；EndedAt 为 nil 表示仍在参与。
	StartedAt time.Time
	EndedAt   *time.Time
}

// Normalize 去首尾空白并给角色兜默认值。
func (f StudioProjectMemberFields) Normalize() StudioProjectMemberFields {
	f.Name = strings.TrimSpace(f.Name)
	if f.Role == "" {
		f.Role = ProjectMemberContributor
	}
	return f
}

// Validate 校验参与行字段，含「结束时间不得早于开始时间」。
func (f StudioProjectMemberFields) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("%w: 参与者姓名必填", ErrStudioProjectInvalid)
	}
	if utf8.RuneCountInString(f.Name) > StudioProjectMemberNameMaxRunes {
		return fmt.Errorf("%w: 参与者姓名最多 %d 个字符", ErrStudioProjectInvalid, StudioProjectMemberNameMaxRunes)
	}
	if !f.Role.Valid() {
		return fmt.Errorf("%w: 参与角色只能是 %s", ErrStudioProjectInvalid, strings.Join(StudioProjectMemberRoles(), " / "))
	}
	if f.StartedAt.IsZero() {
		return fmt.Errorf("%w: 参与开始时间必填", ErrStudioProjectInvalid)
	}
	// 结束早于开始是一行永远不成立的时间区间，任何展示都会变成负数时长。
	if f.EndedAt != nil && f.EndedAt.Before(f.StartedAt) {
		return fmt.Errorf("%w: 参与结束时间不能早于开始时间", ErrStudioProjectInvalid)
	}
	if f.MemberID != nil && *f.MemberID <= 0 {
		return fmt.Errorf("%w: member_id 必须是正整数", ErrStudioProjectInvalid)
	}
	return nil
}

// ApplyTo 把字段写进参与行。
func (f StudioProjectMemberFields) ApplyTo(m *StudioProjectMember) {
	m.MemberID = f.MemberID
	m.Name = f.Name
	m.Role = f.Role
	m.StartedAt = f.StartedAt
	m.EndedAt = f.EndedAt
}
