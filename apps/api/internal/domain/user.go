package domain

import "time"

type User struct {
	ID       int    `json:"id" gorm:"primaryKey"`
	Name     string `json:"name" gorm:"column:name"`
	Detail   Detail `json:"detail" gorm:"embedded;embeddedPrefix:detail_"`
	Parse    int    `json:"parse" gorm:"column:parse"`
	Password string `json:"password" gorm:"column:password"`
	// DeletedAt 软删标记（issue #142，迁移 0011）：NULL = 未注销，
	// 非 NULL = 已注销。不用 gorm.DeletedAt——那会开启全局查询自动过滤，
	// 管理端用户列表必须能看到已注销行（标 is_deleted），语义要显式可控。
	// 登录面（GetByName/GetByEmail/SelectUserByNameAndPassword）手动排除。
	DeletedAt *time.Time `json:"deleted_at" gorm:"column:deleted_at"`
	// FrozenAt 冻结标记（迁移 0012）：NULL = 正常，非 NULL = 冻结时刻。
	//
	// 与 DeletedAt 正交，刻意不复用：注销是用户主动且不可逆（释放邮箱、
	// 发注销通知、永久拒绝登录），冻结是系统按资料规则判定、可解冻、
	// 邮箱仍归本人。混用会让上面每条既有语义产生歧义。
	FrozenAt *time.Time `json:"frozen_at" gorm:"column:frozen_at"`
	// FreezeReason 冻结原因快照（例如「班级须为「汉字 + 6 位数字」」）。
	// 存快照而不是每次按当前规则重算：规则调整后，已冻结账号的原因不该被改写。
	FreezeReason string `json:"freeze_reason" gorm:"column:freeze_reason"`
	// FreezeNotifiedAt 冻结通知邮件发出时间，供扫描任务幂等（避免重复发信）。
	FreezeNotifiedAt *time.Time `json:"freeze_notified_at" gorm:"column:freeze_notified_at"`
	CreatedAt        time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// IsDeleted 账号是否已注销（软删行判定）。
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// IsFrozen 账号是否已冻结（资料不合规，迁移 0012）。
func (u *User) IsFrozen() bool {
	return u.FrozenAt != nil
}

type Detail struct {
	StudentID int    `json:"student_id"`
	Class     string `json:"class"`
	// QQ issue #57 新增（迁移 0006 加 detail_qq 列）：选填，空串 = 未填。
	QQ        string    `json:"qq"`
	Direction Direction `json:"direction"`
	Email     string    `json:"email"`
}

type Direction string

const (
	FrontEnd Direction = "front-end" // 前端
	BackEnd  Direction = "back-end"  // 后端
	Agent    Direction = "agent"     // 智能体
	All      Direction = "all"       // 全栈
	Game     Direction = "game"      // 游戏
	Other    Direction = "other"     // 其他
)
