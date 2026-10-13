package response

import "time"

// InvitationResp 邀请（管理端视图）。
//
// Token 只发给归属人（用户端接受时使用）与管理端（重发场景），不进日志。
type InvitationResp struct {
	ID            int        `json:"id"`
	ApplicationID int        `json:"application_id"`
	Status        string     `json:"status"`
	ExpiresAt     string     `json:"expires_at"`
	// EmailSentAt 空串表示邮件从未投递成功——此时不算「已邀请」，允许重试投递。
	EmailSentAt   string     `json:"email_sent_at"`
	SentBy        int        `json:"sent_by"`
	AcceptedAt    string     `json:"accepted_at"`
	CreatedAt     string     `json:"created_at"`
}

// formatTime 邀请时间戳的统一 RFC3339 序列化；零值/nil 返回空串。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// InvitationListResp 管理端邀请列表（含按申请过滤）。
type InvitationListResp struct {
	Items    []InvitationResp `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// MyInvitation 用户端邀请条目：本人申请关联的邀请视图。
//
// Token 直接携带，用于前端展示「接受邀请」动作；列表按本人 user_id 过滤，
// 不会出现他人邀请。
type MyInvitation struct {
	ApplicationID int    `json:"application_id"`
	Direction     string `json:"direction"`
	Status        string `json:"status"`
	// Expired 由服务端按查询时刻判定，前端不必自行比对过期时间。
	Expired    bool   `json:"expired"`
	Token      string `json:"token"`
	ExpiresAt  string `json:"expires_at"`
	AcceptedAt string `json:"accepted_at"`
	CreatedAt  string `json:"created_at"`
}

// MyInvitationListResp 用户端本人邀请列表。
type MyInvitationListResp struct {
	Items []MyInvitation `json:"items"`
	Total int            `json:"total"`
}

// CandidateResp 候选人（管理端列表/详情视图）。
type CandidateResp struct {
	ID            int    `json:"id"`
	ApplicationID int    `json:"application_id"`
	UserID        int    `json:"user_id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Direction     string `json:"direction"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// CandidateListResp 候选人分页列表。
type CandidateListResp struct {
	Items    []CandidateResp `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// CandidateTimelineResp 时间线一行。
type CandidateTimelineResp struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Note       string `json:"note"`
	OperatorID int    `json:"operator_id"`
	CreatedAt  string `json:"created_at"`
}

// CandidateDetailResp 候选人详情：基础信息 + 时间线。
type CandidateDetailResp struct {
	Candidate CandidateResp            `json:"candidate"`
	Timeline  []CandidateTimelineResp `json:"timeline"`
}

// MyCandidate 用户端本人候选人记录。
type MyCandidate struct {
	ApplicationID int    `json:"application_id"`
	Direction     string `json:"direction"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// AcceptInvitationResp 接受邀请成功：返回本人候选人视图，前端直接展示新状态。
type AcceptInvitationResp struct {
	Candidate MyCandidate `json:"candidate"`
}

// MyCandidateListResp 用户端本人候选人记录列表。
type MyCandidateListResp struct {
	Items []MyCandidate `json:"items"`
	Total int           `json:"total"`
}
