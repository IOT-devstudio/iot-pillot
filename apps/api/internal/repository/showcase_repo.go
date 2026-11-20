package repository

import (
	"context"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// ShowcaseRepo 作品提交的持久化。
type ShowcaseRepo interface {
	// Create 新建一份提交。
	Create(ctx context.Context, submission *domain.ShowcaseSubmission) error
	// ListByUser 本人作品，按提交时间倒序。
	ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]*domain.ShowcaseSubmission, int64, error)
	// GetByID 按 id 取；不存在返回 nil, nil（与既有仓库口径一致，不抛哨兵）。
	GetByID(ctx context.Context, id int64) (*domain.ShowcaseSubmission, error)
	// DeleteOwn 删除本人的作品；不是本人的返回 domain.ErrShowcaseNotFound。
	//
	// 条件更新而不是「先查再删」：中间有窗口，别人并发改动时会删错东西。
	DeleteOwn(ctx context.Context, id, userID int64) error
	// ListAll 管理端全量列表，可按状态过滤（空串表示全部）。
	ListAll(ctx context.Context, status string, page, pageSize int) ([]*domain.ShowcaseSubmission, int64, error)
	// Review 推进审核状态；返回受影响行数为 0 时说明记录不存在。
	Review(ctx context.Context, id int64, status, note string, reviewerID int64) error
}

// showcaseRepo GORM 实现。
type showcaseRepo struct {
	db *gorm.DB
}

// NewShowcaseRepo 构造函数。
func NewShowcaseRepo(db *gorm.DB) ShowcaseRepo {
	return &showcaseRepo{db: db}
}

// Create 写入一份提交。
func (r *showcaseRepo) Create(ctx context.Context, submission *domain.ShowcaseSubmission) error {
	now := time.Now()
	submission.CreatedAt = now
	submission.UpdatedAt = now
	if submission.Status == "" {
		submission.Status = domain.ShowcaseStatusSubmitted
	}
	return r.db.WithContext(ctx).Create(submission).Error
}

// ListByUser 本人作品列表。
func (r *showcaseRepo) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]*domain.ShowcaseSubmission, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.ShowcaseSubmission{}).Where("user_id = ?", userID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]*domain.ShowcaseSubmission, 0, pageSize)
	err := q.Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetByID 按 id 取一条。
func (r *showcaseRepo) GetByID(ctx context.Context, id int64) (*domain.ShowcaseSubmission, error) {
	var row domain.ShowcaseSubmission
	err := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

// DeleteOwn 删除本人的作品。
//
// 用 user_id 一起做条件而不是先查后删：后者存在「查到之后被别人改掉」的窗口，
// 而且把「不是我的」与「不存在」两类错混在一起时容易漏掉归属判断。
func (r *showcaseRepo) DeleteOwn(ctx context.Context, id, userID int64) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&domain.ShowcaseSubmission{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrShowcaseNotFound
	}
	return nil
}

// ListAll 管理端全量列表。
//
// 排序刻意把待审阅放前面：管理员打开这一页最想做的是「处理待审」，
// 让他先看到已完成的历史只是在增加翻页成本。
func (r *showcaseRepo) ListAll(ctx context.Context, status string, page, pageSize int) ([]*domain.ShowcaseSubmission, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.ShowcaseSubmission{})
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]*domain.ShowcaseSubmission, 0, pageSize)
	err := q.Order("CASE WHEN status = 'submitted' THEN 0 ELSE 1 END, created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Review 推进审核状态。
func (r *showcaseRepo) Review(ctx context.Context, id int64, status, note string, reviewerID int64) error {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&domain.ShowcaseSubmission{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      status,
			"review_note": note,
			"reviewed_at": now,
			"reviewed_by": reviewerID,
			"updated_at":  now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrShowcaseNotFound
	}
	return nil
}

// 编译期确认接口实现完整。
var _ ShowcaseRepo = (*showcaseRepo)(nil)
