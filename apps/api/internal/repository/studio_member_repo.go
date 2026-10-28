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

// ErrStudioMemberNotFound 成员档案不存在。
var ErrStudioMemberNotFound = errors.New("工作室成员不存在")

// ErrStudioMemberDuplicate 同名同出生年的成员已存在（迁移 0013 的唯一索引）。
//
// 由 idx_studio_members_name_birth_year 强制；仓储把驱动层的 23505 翻译成
// 本哨兵，让管理端拿到「已有同名同出生年成员」而不是英文报错。
var ErrStudioMemberDuplicate = errors.New("已存在同名同出生年的成员，请核对后修改")

// ErrStudioMemberArchived 对已归档成员重复归档。
var ErrStudioMemberArchived = errors.New("该成员已归档")

// ErrStudioMemberUserNotFound user_id 指向的账号不存在或已注销。
var ErrStudioMemberUserNotFound = errors.New("关联的系统账号不存在或已注销")

/* ------------------------------------------------------------------ *
 * 过滤条件
 * ------------------------------------------------------------------ */

// StudioMemberListFilter 成员列表的筛选条件，零值表示不额外限制。
//
// IsActive 用 *bool 而不是 bool：三态必须可区分——
//   - nil   = 全部（在读 + 已归档）
//   - true  = 仅在读
//   - false = 仅归档
//
// 用 bool 会让「不筛选」和「只看已归档」撞成同一个值。
type StudioMemberListFilter struct {
	// Query 关键词：姓名 / 届别 / 方向 / 职能的模糊匹配。
	Query string
	// Direction 方向精确匹配（与招新方向 code 对齐时可直接复用）。
	Direction string
	// IsActive 在读状态三态筛选。
	IsActive *bool
}

/* ------------------------------------------------------------------ *
 * 仓储接口
 * ------------------------------------------------------------------ */

// StudioMemberRepo 工作室成员档案的持久化（#195）。
//
// 只有成员域：项目参与与知识库是后续票（#198/#196），
// 现在不为它们预留方法——用不到的抽象等真实需求出现再写。
type StudioMemberRepo interface {
	// Create 写入成员档案；撞 (name, birth_year) 唯一索引返回 ErrStudioMemberDuplicate。
	Create(ctx context.Context, member *domain.StudioMember) error
	// GetByID 按 id 取档案；不存在返回 ErrStudioMemberNotFound。
	GetByID(ctx context.Context, id int) (*domain.StudioMember, error)
	// GetBySourceCandidateID 按来源候选人取档案；没转过返回 ErrStudioMemberNotFound。
	//
	// 「hired 转成员」用它做写库前的前置检查（#197）：数据库的
	// source_candidate_id 部分唯一索引仍是最后一道防线，但只靠它的话，
	// 重复点击会先写一条再撞约束回滚，日志里留下无意义的失败写入。
	GetBySourceCandidateID(ctx context.Context, candidateID int) (*domain.StudioMember, error)
	// List 分页列出成员（新建在前）。
	List(ctx context.Context, filter StudioMemberListFilter, page int, pageSize int) ([]*domain.StudioMember, int64, error)
	// Update 更新成员的可写列；不存在返回 ErrStudioMemberNotFound，
	// 撞唯一索引返回 ErrStudioMemberDuplicate。
	Update(ctx context.Context, member *domain.StudioMember) error
	// Archive 归档：置 archived_at = at、is_active = false（幂等：已归档返回 ErrStudioMemberArchived）。
	Archive(ctx context.Context, id int, at time.Time) error
}

var _ StudioMemberRepo = (*gormStudioMemberRepo)(nil)

type gormStudioMemberRepo struct {
	db *gorm.DB
}

// NewStudioMemberRepo 构造函数。
func NewStudioMemberRepo(db *gorm.DB) StudioMemberRepo {
	return &gormStudioMemberRepo{db: db}
}

/* ------------------------------------------------------------------ *
 * 实现
 * ------------------------------------------------------------------ */

func (r *gormStudioMemberRepo) Create(ctx context.Context, member *domain.StudioMember) error {
	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		// studio_members 上只有 (name, birth_year) 与 source_candidate_id 两个
		// 唯一索引；本方法不写 source_candidate_id（那是 #197 的转入口），
		// 所以撞上的只可能是姓名+出生年。
		if IsDuplicateKey(err) {
			return ErrStudioMemberDuplicate
		}
		return err
	}
	return nil
}

func (r *gormStudioMemberRepo) GetByID(ctx context.Context, id int) (*domain.StudioMember, error) {
	var member domain.StudioMember
	if err := r.db.WithContext(ctx).First(&member, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudioMemberNotFound
		}
		return nil, err
	}
	return &member, nil
}

func (r *gormStudioMemberRepo) GetBySourceCandidateID(ctx context.Context, candidateID int) (*domain.StudioMember, error) {
	var member domain.StudioMember
	err := r.db.WithContext(ctx).
		Where("source_candidate_id = ?", candidateID).
		First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudioMemberNotFound
		}
		return nil, err
	}
	return &member, nil
}

func (r *gormStudioMemberRepo) List(ctx context.Context, filter StudioMemberListFilter, page int, pageSize int) ([]*domain.StudioMember, int64, error) {
	var members []*domain.StudioMember
	var total int64

	// Count 与 Find 必须带同一组条件，否则 total 是全表数、分页却是过滤结果。
	applyFilter := func(q *gorm.DB) *gorm.DB {
		if keyword := strings.TrimSpace(filter.Query); keyword != "" {
			// ILIKE + 转义：% 与 _ 是 LIKE 的通配符，不转义会让用户输入的
			// 下划线变成「任意一个字符」，关键词越搜越宽。
			pattern := "%" + escapeLike(keyword) + "%"
			q = q.Where(
				"name ILIKE ? OR cohort ILIKE ? OR direction ILIKE ? OR role_in_studio ILIKE ?",
				pattern, pattern, pattern, pattern,
			)
		}
		if direction := strings.TrimSpace(filter.Direction); direction != "" {
			q = q.Where("direction = ?", direction)
		}
		if filter.IsActive != nil {
			q = q.Where("is_active = ?", *filter.IsActive)
		}
		return q
	}

	if err := applyFilter(r.db.WithContext(ctx).Model(&domain.StudioMember{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := applyFilter(r.db.WithContext(ctx)).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&members).Error
	if err != nil {
		return nil, 0, err
	}
	return members, total, nil
}

// Update 只写可编辑列。
//
// 刻意不用 Save（那会把零值/未加载字段一并写回）：归档态与 source_candidate_id
// 由各自的动作独占，更新资料不该顺手把它们抹掉。
func (r *gormStudioMemberRepo) Update(ctx context.Context, member *domain.StudioMember) error {
	result := r.db.WithContext(ctx).
		Model(&domain.StudioMember{}).
		Where("id = ?", member.ID).
		Updates(map[string]any{
			"user_id":        member.UserID,
			"name":           member.Name,
			"birth_year":     member.BirthYear,
			"cohort":         member.Cohort,
			"direction":      member.Direction,
			"role_in_studio": member.RoleInStudio,
			"internship":     member.Internship,
			"employment":     member.Employment,
			"is_active":      member.IsActive,
			// archived_at 必须一起写：服务层「恢复在读」会把内存里的归档时刻
			// 清成 nil，若这里不落库，数据库就会留下
			// 「is_active = true 但 archived_at 非空」的自相矛盾行——
			// 列表按 is_active 筛出在读，详情却显示归档时间。
			"archived_at": member.ArchivedAt,
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		if IsDuplicateKey(result.Error) {
			return ErrStudioMemberDuplicate
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 行数 0 也可能是「内容完全没变」，用一次存在性查询区分，
		// 免得把无变化的保存报成不存在。
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&domain.StudioMember{}).
			Where("id = ?", member.ID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrStudioMemberNotFound
		}
	}
	return nil
}

// Archive 归档成员（不物理删除）。
//
// 条件更新（WHERE archived_at IS NULL）是幂等闸门：并发或重复点击时
// 只有第一次能拿到 RowsAffected = 1，其余按「已归档」拒绝，
// 不会把归档时间刷成第二次点击的时刻。
func (r *gormStudioMemberRepo) Archive(ctx context.Context, id int, at time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&domain.StudioMember{}).
		Where("id = ? AND archived_at IS NULL", id).
		Updates(map[string]any{
			"archived_at": at,
			"is_active":   false,
			"updated_at":  at,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&domain.StudioMember{}).
			Where("id = ?", id).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrStudioMemberNotFound
		}
		return ErrStudioMemberArchived
	}
	return nil
}

// escapeLike 转义 LIKE 模式里的通配符（配合 PostgreSQL 默认的 ESCAPE '\'）。
func escapeLike(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"%", "\\%",
		"_", "\\_",
	)
	return replacer.Replace(value)
}
