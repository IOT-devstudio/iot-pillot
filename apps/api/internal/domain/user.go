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
	CreatedAt time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// IsDeleted 账号是否已注销（软删行判定）。
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
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
