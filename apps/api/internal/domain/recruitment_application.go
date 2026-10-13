package domain

import "time"

// Application 报名申请（BE-04 锚点模型）。
//
// 真正的申请模型归属 #88（BE-03）：字段、状态机与唯一约束以 #88 的交付为准。
// 本模型只镜像迁移 0008 的锚点表形状，供邀请流读取申请与核验归属；
// #88 合入时用它的模型与仓储替换本文件与 repository 的锚点实现。
type Application struct {
	ID            int       `json:"id" gorm:"primaryKey"`
	UserID        int       `json:"user_id" gorm:"column:user_id;index"`
	RoundID       int       `json:"round_id" gorm:"column:round_id"`
	DirectionCode string    `json:"direction_code" gorm:"column:direction_code"`
	Status        string    `json:"status" gorm:"column:status"`
	Name          string    `json:"name" gorm:"column:name"`
	Email         string    `json:"email" gorm:"column:email"`
	Class         string    `json:"class" gorm:"column:class"`
	StudentID     int64     `json:"student_id" gorm:"column:student_id"`
	QQ            string    `json:"qq" gorm:"column:qq"`
	Note          string    `json:"note" gorm:"column:note"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}
