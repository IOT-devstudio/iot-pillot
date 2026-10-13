package domain

import "time"

// InvitationStatus 邀请状态机。
//
//	pending → accepted | revoked
//
// 「过期」不是独立状态：pending 且 expires_at 已过即视为过期（查询期判定），
// 不需要后台任务把过期邀请改成某个状态。撤销允许从 pending 直接发生；
// 过期邀请也允许撤销（状态落 revoked，语义 = 这条邀请不再有效）。
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationRevoked  InvitationStatus = "revoked"
)

// Valid 报告是否为合法状态值。
func (s InvitationStatus) Valid() bool {
	switch s {
	case InvitationPending, InvitationAccepted, InvitationRevoked:
		return true
	}
	return false
}

// Expired 报告这条邀请在 at 时刻是否已过期。
// 只有 pending 才谈过期；accepted/revoked 不因时间流逝改变语义。
func (s InvitationStatus) Expired(expiresAt time.Time, at time.Time) bool {
	return s == InvitationPending && at.After(expiresAt)
}

// Acceptable 报告这条邀请在 at 时刻能否被接受。
func (s InvitationStatus) Acceptable(expiresAt time.Time, at time.Time) bool {
	return s == InvitationPending && !s.Expired(expiresAt, at)
}

// Invitation 一条招新邀请：把一个报名申请推进到候选人流程的凭证。
//
// Token 是接受动作的凭据（用户端不需要知道邀请行 id）；一个申请同一时刻至多一条
// pending 邀请（部分唯一索引保证，见迁移 0008）。EmailSentAt 为空表示邀请邮件
// 从未投递成功——此时业务上不算「已邀请」，允许重试投递；投递成功才落时间。
type Invitation struct {
	ID            int              `json:"id" gorm:"primaryKey"`
	ApplicationID int              `json:"application_id" gorm:"column:application_id;index"`
	Token         string           `json:"-" gorm:"column:token;uniqueIndex"`
	Status        InvitationStatus `json:"status" gorm:"column:status"`
	ExpiresAt     time.Time        `json:"expires_at" gorm:"column:expires_at"`
	EmailSentAt   *time.Time       `json:"email_sent_at" gorm:"column:email_sent_at"`
	SentBy        int              `json:"sent_by" gorm:"column:sent_by"`
	AcceptedAt    *time.Time       `json:"accepted_at" gorm:"column:accepted_at"`
	CreatedAt     time.Time        `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time        `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// Expired 报告这条邀请在 at 时刻是否已过期。
func (i *Invitation) Expired(at time.Time) bool {
	return i.Status.Expired(i.ExpiresAt, at)
}

// Acceptable 报告这条邀请在 at 时刻能否被接受。
func (i *Invitation) Acceptable(at time.Time) bool {
	return i.Status.Acceptable(i.ExpiresAt, at)
}

// CandidateStatus 候选人状态机。
//
//	interviewing → offered → hired；interviewing | offered → rejected
//
// hired 与 rejected 是终态。所有流转要求邮件动作成功的见各动作说明：
// offered 必须在 offer 邮件投递成功后落地；rejected 在感谢信投递成功后落地。
type CandidateStatus string

const (
	CandidateInterviewing CandidateStatus = "interviewing"
	CandidateOffered      CandidateStatus = "offered"
	CandidateHired        CandidateStatus = "hired"
	CandidateRejected     CandidateStatus = "rejected"
)

// Valid 报告是否为合法状态值。
func (s CandidateStatus) Valid() bool {
	switch s {
	case CandidateInterviewing, CandidateOffered, CandidateHired, CandidateRejected:
		return true
	}
	return false
}

// CanTransitionTo 报告能否从当前状态流转到 target。
func (s CandidateStatus) CanTransitionTo(target CandidateStatus) bool {
	switch s {
	case CandidateInterviewing:
		return target == CandidateOffered || target == CandidateRejected
	case CandidateOffered:
		return target == CandidateHired || target == CandidateRejected
	default:
		// interviewing/offered 之外都是终态，不可再流转
		return false
	}
}

// Candidate 一位候选人：接受邀请后的申请（含所属用户）进入面试流程。
//
// ApplicationID 唯一（迁移 0008 部分约束）——并发接受同一申请的邀请时，
// 第二个事务在这一行上撞唯一约束，上层翻译成业务错误，实现幂等兜底。
type Candidate struct {
	ID            int             `json:"id" gorm:"primaryKey"`
	ApplicationID int             `json:"application_id" gorm:"column:application_id;uniqueIndex"`
	UserID        int             `json:"user_id" gorm:"column:user_id;index"`
	Status        CandidateStatus `json:"status" gorm:"column:status"`
	CreatedAt     time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// CandidateTimeline 候选人时间线一行：每次状态流转追加一条。
//
// 管理端与用户端读取同一张表，保证两端看到一致的状态与时间。
// 初始行（applied 之外的起点）记 to_status = interviewing、from_status 为空。
type CandidateTimeline struct {
	ID          int             `json:"id" gorm:"primaryKey"`
	CandidateID int             `json:"candidate_id" gorm:"column:candidate_id;index"`
	FromStatus  string          `json:"from_status" gorm:"column:from_status"`
	ToStatus    CandidateStatus `json:"to_status" gorm:"column:to_status"`
	Note        string          `json:"note" gorm:"column:note"`
	OperatorID  int             `json:"operator_id" gorm:"column:operator_id"`
	CreatedAt   time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}
