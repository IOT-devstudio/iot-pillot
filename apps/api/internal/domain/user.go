package domain

import "time"

type User struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"column:name;uniqueIndex"`
	Password  string    `json:"password" gorm:"column:password"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}
