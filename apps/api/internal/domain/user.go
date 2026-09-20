package domain

import "time"

type User struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"column:name"`
	Detail    Detail    `json:"detail" gorm:"embedded;embeddedPrefix:detail_"`
	Parse     int       `json:"parse" gorm:"column:parse"`
	Password  string    `json:"password" gorm:"column:password"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

type Detail struct {
	StudentID int       `json:"student_id"`
	Class     string    `json:"class"`
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
