package repository

import (
	"context"
	"errors"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// LearningRoadmapRepo 学习路线的持久化（#223）。
type LearningRoadmapRepo interface {
	List(ctx context.Context, filter LearningRoadmapListFilter, page, pageSize int) ([]*domain.LearningRoadmap, int64, error)
	GetByID(ctx context.Context, id int) (*domain.LearningRoadmap, error)
	// Directions 返回**只含路线**的方向与条数（ADR 0006 决策 8）。
	Directions(ctx context.Context) ([]LearningRoadmapDirection, error)
	Create(ctx context.Context, roadmap *domain.LearningRoadmap) error
	// Update 带乐观锁：仅当库里 version 等于 expectedVersion 时才写入。
	// 返回 domain.ErrRoadmapVersionConflict 表示有人先保存过。
	Update(ctx context.Context, roadmap *domain.LearningRoadmap, expectedVersion int) error
	SoftDelete(ctx context.Context, id, operatorID int) error
}

// LearningRoadmapListFilter 列表筛选。
type LearningRoadmapListFilter struct {
	// DirectionCode 空串表示全部方向。
	DirectionCode string
}

// LearningRoadmapDirection 列表页的方向筛选项（含条数）。
type LearningRoadmapDirection struct {
	Code  string
	Name  string
	Count int
}

// learningRoadmapRepo GORM 实现。
type learningRoadmapRepo struct {
	db *gorm.DB
}

// NewLearningRoadmapRepo 构造函数。
func NewLearningRoadmapRepo(db *gorm.DB) LearningRoadmapRepo {
	return &learningRoadmapRepo{db: db}
}

// List 分页列出未软删的路线。
func (r *learningRoadmapRepo) List(ctx context.Context, filter LearningRoadmapListFilter, page, pageSize int) ([]*domain.LearningRoadmap, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.LearningRoadmap{}).Where("deleted_at IS NULL")
	if filter.DirectionCode != "" {
		q = q.Where("direction_code = ?", filter.DirectionCode)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []*domain.LearningRoadmap
	err := q.Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetByID 取单条；软删的当作不存在（route 层回 404 而不是 403）。
func (r *learningRoadmapRepo) GetByID(ctx context.Context, id int) (*domain.LearningRoadmap, error) {
	var row domain.LearningRoadmap
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRoadmapNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Directions 返回只含路线的方向与条数。
//
// 关键点（ADR 0006 决策 8）：**不查招新方向接口**。公开的招新方向接口只返回
// open = true 的方向，招新结束后方向全关，路线页会一个方向都不剩——
// 而路线的生命周期跨越多轮招新，不该被招新的开关状态绑架。
//
// LEFT JOIN 方向表取名，方向已被清理时回落显示 code（可见的降级）。
func (r *learningRoadmapRepo) Directions(ctx context.Context) ([]LearningRoadmapDirection, error) {
	var rows []struct {
		Code  string
		Name  *string
		Count int
	}
	err := r.db.WithContext(ctx).
		Table("learning_roadmaps AS r").
		Select("r.direction_code AS code, d.name AS name, count(*) AS count").
		// 方向表名与列名按招新模块既有定义。
		Joins("LEFT JOIN recruitment_directions AS d ON d.code = r.direction_code").
		Where("r.deleted_at IS NULL").
		Group("r.direction_code, d.name").
		Order("count(*) DESC, r.direction_code ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]LearningRoadmapDirection, 0, len(rows))
	for _, row := range rows {
		name := row.Code
		if row.Name != nil && *row.Name != "" {
			name = *row.Name
		}
		out = append(out, LearningRoadmapDirection{Code: row.Code, Name: name, Count: row.Count})
	}
	return out, nil
}

// Create 创建路线（version 固定从 1 起）。
func (r *learningRoadmapRepo) Create(ctx context.Context, roadmap *domain.LearningRoadmap) error {
	if roadmap.Version == 0 {
		roadmap.Version = 1
	}
	now := time.Now()
	roadmap.CreatedAt = now
	roadmap.UpdatedAt = now
	return r.db.WithContext(ctx).Create(roadmap).Error
}

// Update 带乐观锁的整条覆盖。
//
// 用 `WHERE version = ?` 的条件更新而不是「先读再写」：
// 后者在两个管理员同时保存时存在检查与写入之间的窗口，仍会互相覆盖。
// RowsAffected == 0 即冲突——包括「记录已被别人删掉」这种情况，
// 对调用方都表现为「你看到的版本已经不是最新的」。
func (r *learningRoadmapRepo) Update(ctx context.Context, roadmap *domain.LearningRoadmap, expectedVersion int) error {
	res := r.db.WithContext(ctx).
		Model(&domain.LearningRoadmap{}).
		Where("id = ? AND version = ? AND deleted_at IS NULL", roadmap.ID, expectedVersion).
		Updates(map[string]any{
			"title":          roadmap.Title,
			"body":           roadmap.Body,
			"direction_code": roadmap.DirectionCode,
			"stages":         roadmap.Stages,
			"version":        gorm.Expr("version + 1"),
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRoadmapVersionConflict
	}
	roadmap.Version = expectedVersion + 1
	return nil
}

// SoftDelete 软删并记录操作人。
func (r *learningRoadmapRepo) SoftDelete(ctx context.Context, id, operatorID int) error {
	res := r.db.WithContext(ctx).
		Model(&domain.LearningRoadmap{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"deleted_by": operatorID,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrRoadmapNotFound
	}
	return nil
}
