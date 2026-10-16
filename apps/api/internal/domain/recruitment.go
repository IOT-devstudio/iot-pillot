package domain

import "time"

// RecruitmentRound 招新轮次。
//
// 每一轮招新是一次独立的报名批次。round 自身的编号（RoundNum）供管理与展示，
// 报名归属以 ID 为准；开放报名指"该轮是否处于报名开放窗口"（Accepting）。
// 轮的开启/关闭不自动影响方向与已有申请，只影响"还能不能新报这一轮"。
type RecruitmentRound struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	RoundNum  int       `json:"round_num" gorm:"column:round_num;uniqueIndex"`
	Name      string    `json:"name" gorm:"column:name"`
	Accepting bool      `json:"accepting" gorm:"column:accepting"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// RecruitmentDirection 招新方向。
//
// 注意与 domain.User.Detail.Direction 区分：那是用户资料的**倾向**，包含
// front-end/back-end/agent/all/game/other 六值；这里是"可报名"的招新方向，
// 本期只开放 front-end/back-end/agent 三种（维护者拍板，见 issue #88）。
// all/game/other 保留为用户倾向语义，不在此建模为可报名方向。
//
// Open 表示该方向是否接受报名；关闭时提交报名直接返回业务错误。
type RecruitmentDirection struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	Code      string    `json:"code" gorm:"column:code;uniqueIndex"`
	Name      string    `json:"name" gorm:"column:name"`
	Open      bool      `json:"open" gorm:"column:open"`
	SortOrder int       `json:"sort_order" gorm:"column:sort_order"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// ApplicationStatus 报名申请状态机。
//
// Issue #88 拍板：只实现精简三态
//
//	applied → reviewing → accepted | rejected
//
// 每次流转追加一条时间线记录；不搞 prospect/candidate 语义（留给下游 #90）。
// 本期不做改报/撤报，状态一经流转不可回退。
type ApplicationStatus string

const (
	ApplicationApplied   ApplicationStatus = "applied"
	ApplicationReviewing ApplicationStatus = "reviewing"
	ApplicationAccepted  ApplicationStatus = "accepted"
	ApplicationRejected  ApplicationStatus = "rejected"
)

// Valid 报告是否为合法状态值。
func (s ApplicationStatus) Valid() bool {
	switch s {
	case ApplicationApplied, ApplicationReviewing, ApplicationAccepted, ApplicationRejected:
		return true
	}
	return false
}

// AllowsInterviewSupplement 报告该状态的申请还能否补交面试材料（issue #135）。
//
// 与 CanTransitionTo 是两回事：那个问"审核动作能不能推进状态"，这个问
// "申请还活着、值得再收一份面试材料吗"。applied / reviewing 尚未出结果，
// 补交有意义；accepted / rejected 已经定论，再交材料没有业务含义。
//
// 表单提交走这个判断，而不是复用 CanTransitionTo —— 后者对 applied 只认
// reviewing，会把"先报名、后补面试"这种正常路径误判成非法流转。
func (s ApplicationStatus) AllowsInterviewSupplement() bool {
	switch s {
	case ApplicationApplied, ApplicationReviewing:
		return true
	default:
		return false
	}
}

// CanTransitionTo 报告能否从当前状态流转到 target（顺序状态机）。
//
// 只允许沿 applied → reviewing → accepted/rejected 前进：
//   - applied 不能原地重复、不能直接到 terminal（必须经过 reviewing）
//   - reviewing 可到 accepted / rejected
//   - accepted / rejected 为终态，不可再流转
//
// 审核动作（reviewing → accepted/rejected）之外的非法跳转在此返回 false。
func (s ApplicationStatus) CanTransitionTo(target ApplicationStatus) bool {
	switch s {
	case ApplicationApplied:
		return target == ApplicationReviewing
	case ApplicationReviewing:
		return target == ApplicationAccepted || target == ApplicationRejected
	default:
		return false
	}
}

// Application 一条报名申请。
//
// 归属与方向都由后端从令牌与请求校验确定：UserID 来自鉴权结果，
// Direction 必须是某个已开放且属于本轮的招新方向。
//
// 唯一约束 (user_id, round_id, direction_code) 由迁移 DDL 强制：
// 每轮同一用户对同一方向只能报名一次，重复提交返回业务错误、不覆盖、不追加。
type Application struct {
	ID           int               `json:"id" gorm:"primaryKey"`
	UserID       int               `json:"user_id" gorm:"column:user_id;index"`
	RoundID      int               `json:"round_id" gorm:"column:round_id;index"`
	DirectionCode string           `json:"direction_code" gorm:"column:direction_code"`
	Status       ApplicationStatus `json:"status" gorm:"column:status"`
	Name         string            `json:"name" gorm:"column:name"`
	Email        string            `json:"email" gorm:"column:email"`
	Class        string            `json:"class" gorm:"column:class"`
	StudentID    int64             `json:"student_id" gorm:"column:student_id"`
	QQ           string            `json:"qq" gorm:"column:qq"`
	Note        string            `json:"note" gorm:"column:note"`
	CreatedAt    time.Time         `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time         `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// ApplicationTimeline 报名申请时间线一行。
//
// 每次状态流转（含初始创建）追加一行，记录流转发生在何时、到哪个状态。
// 管理端与用户端读取的"进度"都来自这里，保证两端看到一致的时间戳。
type ApplicationTimeline struct {
	ID            int       `json:"id" gorm:"primaryKey"`
	ApplicationID int       `json:"application_id" gorm:"column:application_id;index"`
	ToStatus      string    `json:"to_status" gorm:"column:to_status"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}