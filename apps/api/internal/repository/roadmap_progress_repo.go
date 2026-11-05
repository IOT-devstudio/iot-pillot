package repository

import (
	"context"
	"errors"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RoadmapProgressRepo 学习进度的持久化（#224）。
//
// **没有**「查某个用户在某条路线的进度」以外的读法：进度只属于本人，
// 接口层拿不到别人的 user_id（从会话取），因此这里也不提供按路线列全部人的方法。
type RoadmapProgressRepo interface {
	// Get 取某人在某条路线上的进度；没有记录时返回 (nil, nil) —— 不是错误。
	ListByUser(ctx context.Context, userID int) ([]*domain.UserRoadmapProgress, error)
	Get(ctx context.Context, userID, roadmapID int) (*domain.UserRoadmapProgress, error)
	// Upsert 标记当前阶段（一人一路线一行，冲突时更新）。
	Upsert(ctx context.Context, userID, roadmapID int, stageID string) error
	// Clear 撤销进度；没有记录时不算错误（再点一次本来就是幂等的撤销）。
	Clear(ctx context.Context, userID, roadmapID int) error
}

type roadmapProgressRepo struct {
	db *gorm.DB
}

// NewRoadmapProgressRepo 构造函数。
func NewRoadmapProgressRepo(db *gorm.DB) RoadmapProgressRepo {
	return &roadmapProgressRepo{db: db}
}

// ListByUser 列出某人的全部进度（列表页用来标「我已开始」，详情页用来定位当前阶段）。
func (r *roadmapProgressRepo) ListByUser(ctx context.Context, userID int) ([]*domain.UserRoadmapProgress, error) {
	var rows []*domain.UserRoadmapProgress
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Get 取某人在某条路线上的进度。
func (r *roadmapProgressRepo) Get(ctx context.Context, userID, roadmapID int) (*domain.UserRoadmapProgress, error) {
	var row domain.UserRoadmapProgress
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有进度是**合法状态**，不是错误：新用户还没开始学任何路线。
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Upsert 标记当前阶段。
//
// 用 `ON CONFLICT (user_id, roadmap_id) DO UPDATE` 而不是「先查再决定插入还是更新」：
// 后者在并发两次点击时可能插入两行（或撞唯一索引报错），
// 而 UPSERT 让这个动作天然幂等。
func (r *roadmapProgressRepo) Upsert(ctx context.Context, userID, roadmapID int, stageID string) error {
	row := &domain.UserRoadmapProgress{
		UserID:    userID,
		RoadmapID: roadmapID,
		StageID:   stageID,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "roadmap_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"stage_id", "updated_at"}),
		}).
		Create(row).Error
}

// Clear 撤销进度。
//
// 没有记录时不报错：用户在两个标签页各点一次「撤销」是正常的，
// 第二次拿不到行不该变成一次失败。
func (r *roadmapProgressRepo) Clear(ctx context.Context, userID, roadmapID int) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).
		Delete(&domain.UserRoadmapProgress{}).Error
}
