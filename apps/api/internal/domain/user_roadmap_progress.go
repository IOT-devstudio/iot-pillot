package domain

import "time"

// UserRoadmapProgress 用户在某条路线上的学习进度（#224，ADR 0006 决策 4）。
//
// 一人一路线一行：UNIQUE (user_id, roadmap_id)。「标记学到这」= UPSERT；
// 再点同一个阶段 = 删掉这一行（撤销）。
//
// **进度只属于本人**：接口只返回当前登录用户的进度，没有「查看某人进度」的端点。
// 管理员也看不到——少一套权限判定，也少一个隐私面。
type UserRoadmapProgress struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	UserID    int       `json:"user_id" gorm:"column:user_id"`
	RoadmapID int       `json:"roadmap_id" gorm:"column:roadmap_id"`
	StageID   string    `json:"stage_id" gorm:"column:stage_id"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 固定表名。
func (UserRoadmapProgress) TableName() string { return "user_roadmap_progress" }
