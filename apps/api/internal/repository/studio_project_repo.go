package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

/* ------------------------------------------------------------------ *
 * 哨兵错误
 * ------------------------------------------------------------------ */

// ErrStudioProjectNotFound 项目不存在。
var ErrStudioProjectNotFound = errors.New("工作室项目不存在")

// ErrStudioProjectDuplicateName 项目名已被占用（迁移 0014 的唯一索引）。
var ErrStudioProjectDuplicateName = errors.New("已存在同名项目，请换一个名字")

// ErrStudioProjectMemberNotFound 参与行不存在，或不属于该项目。
var ErrStudioProjectMemberNotFound = errors.New("项目参与记录不存在")

// ErrStudioProjectMemberDuplicate 同一项目下已有同名参与者。
//
// 由 idx_studio_project_members_project_name 强制。同一个人在项目里出现
// 两行会让「谁是 lead」「参与了多久」变成两个答案。
var ErrStudioProjectMemberDuplicate = errors.New("该项目下已有同名参与者")

// ErrStudioProjectMemberOwnerNotFound member_id 指向的成员档案不存在。
var ErrStudioProjectMemberOwnerNotFound = errors.New("关联的工作室成员不存在")

/* ------------------------------------------------------------------ *
 * 过滤条件
 * ------------------------------------------------------------------ */

// StudioProjectListFilter 项目列表的筛选条件，零值表示不额外限制。
type StudioProjectListFilter struct {
	// Query 关键词：项目名 / 简介的模糊匹配。
	Query string
	// Status 状态精确匹配（空串 = 不筛选）。
	Status string
	// Year 年份精确匹配（空串 = 不筛选）。
	Year string
}

/* ------------------------------------------------------------------ *
 * 仓储接口
 * ------------------------------------------------------------------ */

// StudioProjectRepo 工作室项目与参与的持久化（#198）。
//
// 项目与参与放在同一个接口下：它们的写操作共享「项目必须存在」这条前提，
// 拆成两个仓储会让「给不存在的项目加参与」变成调用方的责任。
type StudioProjectRepo interface {
	// Create 写入项目；撞 name 唯一索引返回 ErrStudioProjectDuplicateName。
	Create(ctx context.Context, project *domain.StudioProject) error
	// GetByID 按 id 取项目；不存在返回 ErrStudioProjectNotFound。
	GetByID(ctx context.Context, id int) (*domain.StudioProject, error)
	// List 分页列出项目（新建在前）。
	List(ctx context.Context, filter StudioProjectListFilter, page int, pageSize int) ([]*domain.StudioProject, int64, error)
	// Update 更新项目元数据；不存在返回 ErrStudioProjectNotFound，
	// 撞 name 唯一索引返回 ErrStudioProjectDuplicateName。
	Update(ctx context.Context, project *domain.StudioProject) error
	// Delete 删除项目；参与行由迁移的 ON DELETE CASCADE 一并删除。
	Delete(ctx context.Context, id int) error

	// ListMembers 列出某项目的参与行（按开始时间升序，同时间按 id）。
	ListMembers(ctx context.Context, projectID int) ([]*domain.StudioProjectMember, error)
	// AddMember 为项目添加一条参与；项目不存在返回 ErrStudioProjectNotFound，
	// 同项目同名返回 ErrStudioProjectMemberDuplicate。
	AddMember(ctx context.Context, member *domain.StudioProjectMember) error
	// UpdateMember 更新参与行；不匹配（id 不属于该项目）返回
	// ErrStudioProjectMemberNotFound，同名冲突返回 ErrStudioProjectMemberDuplicate。
	UpdateMember(ctx context.Context, member *domain.StudioProjectMember) error
	// RemoveMember 移除参与行；不匹配返回 ErrStudioProjectMemberNotFound。
	RemoveMember(ctx context.Context, projectID int, memberID int) error
	// StudioMemberExists 报告 member_id 是否指向一个存在的成员档案。
	StudioMemberExists(ctx context.Context, memberID int) (bool, error)
}

var _ StudioProjectRepo = (*gormStudioProjectRepo)(nil)

type gormStudioProjectRepo struct {
	db *gorm.DB
}

// NewStudioProjectRepo 构造函数。
func NewStudioProjectRepo(db *gorm.DB) StudioProjectRepo {
	return &gormStudioProjectRepo{db: db}
}

/* ------------------------------------------------------------------ *
 * 项目
 * ------------------------------------------------------------------ */

func (r *gormStudioProjectRepo) Create(ctx context.Context, project *domain.StudioProject) error {
	if err := r.db.WithContext(ctx).Create(project).Error; err != nil {
		// studio_projects 上只有 name 一个唯一索引。
		if IsDuplicateKey(err) {
			return ErrStudioProjectDuplicateName
		}
		return err
	}
	return nil
}

func (r *gormStudioProjectRepo) GetByID(ctx context.Context, id int) (*domain.StudioProject, error) {
	var project domain.StudioProject
	if err := r.db.WithContext(ctx).First(&project, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudioProjectNotFound
		}
		return nil, err
	}
	return &project, nil
}

func (r *gormStudioProjectRepo) List(ctx context.Context, filter StudioProjectListFilter, page int, pageSize int) ([]*domain.StudioProject, int64, error) {
	var projects []*domain.StudioProject
	var total int64

	// Count 与 Find 必须带同一组条件，否则 total 是全表数、分页却是过滤结果。
	applyFilter := func(q *gorm.DB) *gorm.DB {
		if keyword := strings.TrimSpace(filter.Query); keyword != "" {
			pattern := "%" + escapeLike(keyword) + "%"
			q = q.Where("name ILIKE ? OR summary ILIKE ?", pattern, pattern)
		}
		if status := strings.TrimSpace(filter.Status); status != "" {
			q = q.Where("status = ?", status)
		}
		if year := strings.TrimSpace(filter.Year); year != "" {
			q = q.Where("year = ?", year)
		}
		return q
	}

	if err := applyFilter(r.db.WithContext(ctx).Model(&domain.StudioProject{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := applyFilter(r.db.WithContext(ctx)).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&projects).Error
	if err != nil {
		return nil, 0, err
	}
	return projects, total, nil
}

func (r *gormStudioProjectRepo) Update(ctx context.Context, project *domain.StudioProject) error {
	result := r.db.WithContext(ctx).
		Model(&domain.StudioProject{}).
		Where("id = ?", project.ID).
		Updates(map[string]any{
			"name":       project.Name,
			"year":       project.Year,
			"summary":    project.Summary,
			"status":     project.Status,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		if IsDuplicateKey(result.Error) {
			return ErrStudioProjectDuplicateName
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 行数 0 也可能是「内容完全没变」，用存在性查询区分。
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&domain.StudioProject{}).
			Where("id = ?", project.ID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrStudioProjectNotFound
		}
	}
	return nil
}

// Delete 删除项目。
//
// 参与行由 studio_project_members 的 ON DELETE CASCADE 一并清理（#198 验收）。
// 这里刻意不先删参与再删项目：两条语句之间失败会留下半个项目，
// 而数据库的外键能在一条语句里保证「要么都在，要么都不在」。
func (r *gormStudioProjectRepo) Delete(ctx context.Context, id int) error {
	result := r.db.WithContext(ctx).Delete(&domain.StudioProject{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrStudioProjectNotFound
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 参与
 * ------------------------------------------------------------------ */

func (r *gormStudioProjectRepo) ListMembers(ctx context.Context, projectID int) ([]*domain.StudioProjectMember, error) {
	var members []*domain.StudioProjectMember
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		// 按开始时间升序：详情页展示「谁先加入」，同时间用 id 兜底保证稳定。
		Order("started_at ASC, id ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *gormStudioProjectRepo) AddMember(ctx context.Context, member *domain.StudioProjectMember) error {
	// 先确认项目存在：外键会拦住不存在的 project_id，但那样拿到的是
	// 驱动层的 23503，调用方无法区分「项目没了」与「数据写坏了」。
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.StudioProject{}).
		Where("id = ?", member.ProjectID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrStudioProjectNotFound
	}

	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		// studio_project_members 上有 (project_id, name) 唯一索引。
		if IsDuplicateKey(err) {
			return ErrStudioProjectMemberDuplicate
		}
		return err
	}
	return nil
}

func (r *gormStudioProjectRepo) UpdateMember(ctx context.Context, member *domain.StudioProjectMember) error {
	result := r.db.WithContext(ctx).
		Model(&domain.StudioProjectMember{}).
		Where("id = ? AND project_id = ?", member.ID, member.ProjectID).
		Updates(map[string]any{
			"member_id":  member.MemberID,
			"name":       member.Name,
			"role":       member.Role,
			"started_at": member.StartedAt,
			"ended_at":   member.EndedAt,
		})
	if result.Error != nil {
		if IsDuplicateKey(result.Error) {
			return ErrStudioProjectMemberDuplicate
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&domain.StudioProjectMember{}).
			Where("id = ? AND project_id = ?", member.ID, member.ProjectID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrStudioProjectMemberNotFound
		}
	}
	return nil
}

func (r *gormStudioProjectRepo) RemoveMember(ctx context.Context, projectID int, memberID int) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND project_id = ?", memberID, projectID).
		Delete(&domain.StudioProjectMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrStudioProjectMemberNotFound
	}
	return nil
}

func (r *gormStudioProjectRepo) StudioMemberExists(ctx context.Context, memberID int) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.StudioMember{}).
		Where("id = ?", memberID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
