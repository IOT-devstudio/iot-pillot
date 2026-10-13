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

// ErrRoundNotFound 指定的招新轮次不存在。
var ErrRoundNotFound = errors.New("招新轮次不存在")

// ErrDirectionNotFound 指定的招新方向不存在或已不在报名范围内。
var ErrDirectionNotFound = errors.New("招新方向不存在")

// ErrApplicationNotFound 指定的报名申请不存在（或不属于当前查询方）。
var ErrApplicationNotFound = errors.New("申请不存在")

// ErrDuplicateApplication 同一轮次内同一用户对同一方向重复报名。
//
// 触发重复报名规则的直接信号是唯一约束冲突：发送方可借此给出
// "你已经报过这个方向了" 的可理解提示，而不是暴露驱动层的英文报错。
var ErrDuplicateApplication = errors.New("你已在本轮报过这个方向，不能重复报名")

/* ------------------------------------------------------------------ *
 * 招新领域仓储接口
 * ------------------------------------------------------------------ */

// RecruitmentRepo 招新模块的持久化。方向 / 轮次 / 申请 / 时间线属同一域，
// 放一个接口下减少装配与测试桩的开销，且它们共享同一事务封闭的语义。
type RecruitmentRepo interface {
	// ── 招新方向 ──
	ListDirections(ctx context.Context) ([]*domain.RecruitmentDirection, error)
	// GetDirectionByCode 按 code 取方向；不存在返回 ErrDirectionNotFound。
	GetDirectionByCode(ctx context.Context, code string) (*domain.RecruitmentDirection, error)
	// SetDirectionOpen 开启 / 关闭某方向的报名开关。方向不存在返回 ErrDirectionNotFound。
	SetDirectionOpen(ctx context.Context, code string, open bool) error

	// ── 招新轮次 ──
	ListRounds(ctx context.Context) ([]*domain.RecruitmentRound, error)
	// SetRoundAccepting 开启 / 关闭某轮的报名窗口。轮次不存在返回 ErrRoundNotFound。
	SetRoundAccepting(ctx context.Context, roundID int, accepting bool) error
	// GetActiveRound 返回当前处于报名开放状态的轮次。
	// 没有开放轮次时返回 ErrRoundNotFound（上层据此拒绝报名）。
	GetActiveRound(ctx context.Context) (*domain.RecruitmentRound, error)
	// GetRoundByID 按 id 取轮次；不存在返回 ErrRoundNotFound。
	//
	// 表单模块（#92）需要按"表单绑定的那一轮"核验与报名：它不能借
	// GetActiveRound，那是"当前开放的那一轮"的语义 —— 表单可能在开放前建好。
	GetRoundByID(ctx context.Context, id int) (*domain.RecruitmentRound, error)

	// ── 报名申请 ──
	// CreateApplication 写入一条初始状态（applied）的申请。
	// 撞上 (user_id, round_id, direction_code) 唯一约束时返回 ErrDuplicateApplication。
	CreateApplication(ctx context.Context, app *domain.Application) error
	// ListApplicationsByUser 分页列出某用户的全部申请（时间倒序）。
	ListApplicationsByUser(ctx context.Context, userID int, page int, pageSize int) ([]*domain.Application, int64, error)
	// GetApplicationByID 按 id 取申请；不存在返回 ErrApplicationNotFound。
	GetApplicationByID(ctx context.Context, id int) (*domain.Application, error)
	// ListApplications 分页列出全部申请，可按轮次/方向/状态过滤。
	ListApplications(ctx context.Context, filter ApplicationListFilter, page int, pageSize int) ([]*domain.Application, int64, error)
	// UpdateApplicationStatus 更新申请状态并返回更新的行数。
	// 状态已到终态或其他不该再流转的情况（行数 0）不视为错误，
	// 由上层结合领域规则判断是否有非法流转。
	UpdateApplicationStatus(ctx context.Context, id int, status domain.ApplicationStatus) (int64, error)

	// ── 时间线 ──
	AppendTimeline(ctx context.Context, appID int, status domain.ApplicationStatus) error
	// ListTimeline 按时间升序返回某申请的全部时间线。
	ListTimeline(ctx context.Context, applicationID int) ([]*domain.ApplicationTimeline, error)
}

// ApplicationListFilter 报名申请列表的查询条件，零值字段表示不限制。
type ApplicationListFilter struct {
	RoundID       int
	DirectionCode string
	Status        string
	// From/To 时间范围（申请创建时间），零值 = 不限该端；语义为 [From, To]
	From, To time.Time
}

var _ RecruitmentRepo = (*gormRecruitmentRepo)(nil)

type gormRecruitmentRepo struct {
	db *gorm.DB
}

func NewRecruitmentRepo(db *gorm.DB) RecruitmentRepo {
	return &gormRecruitmentRepo{db: db}
}

/* ------------------------------------------------------------------ *
 * 招新方向
 * ------------------------------------------------------------------ */

func (r *gormRecruitmentRepo) ListDirections(ctx context.Context) ([]*domain.RecruitmentDirection, error) {
	var dirs []*domain.RecruitmentDirection
	// 按人工排序字段稳定展示
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&dirs).Error
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

func (r *gormRecruitmentRepo) GetDirectionByCode(ctx context.Context, code string) (*domain.RecruitmentDirection, error) {
	var dir domain.RecruitmentDirection
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&dir).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDirectionNotFound
		}
		return nil, err
	}
	return &dir, nil
}

func (r *gormRecruitmentRepo) SetDirectionOpen(ctx context.Context, code string, open bool) error {
	result := r.db.WithContext(ctx).
		Model(&domain.RecruitmentDirection{}).
		Where("code = ?", code).
		Update("open", open)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDirectionNotFound
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 招新轮次
 * ------------------------------------------------------------------ */

func (r *gormRecruitmentRepo) ListRounds(ctx context.Context) ([]*domain.RecruitmentRound, error) {
	var rounds []*domain.RecruitmentRound
	// 按轮次编号升序；编号是展示标识
	err := r.db.WithContext(ctx).Order("round_num ASC, id ASC").Find(&rounds).Error
	if err != nil {
		return nil, err
	}
	return rounds, nil
}

// GetActiveRound 返回唯一处于报名开放状态的轮次。
//
// 报名只对当前开放轮开放，因此取 where accepting = true 的单行；
// 没有开放轮（多数时候的状态）返回 ErrRoundNotFound，上层据此拒绝报名。
// 多行开放轮是数据异常（本期不提供并行轮）——取第一行并信任排序。
func (r *gormRecruitmentRepo) GetActiveRound(ctx context.Context) (*domain.RecruitmentRound, error) {
	var round domain.RecruitmentRound
	err := r.db.WithContext(ctx).
		Where("accepting = ?", true).
		Order("round_num ASC, id ASC").
		First(&round).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoundNotFound
		}
		return nil, err
	}
	return &round, nil
}

// GetRoundByID 按 id 取轮次；不存在返回 ErrRoundNotFound。
//
// 表单模块（#92）要按"表单绑定的那一轮"核验与报名，不能借 GetActiveRound：
// 那是"当前开放的那一轮"的语义，而表单可能在开放前就建好了。
func (r *gormRecruitmentRepo) GetRoundByID(ctx context.Context, id int) (*domain.RecruitmentRound, error) {
	var round domain.RecruitmentRound
	err := r.db.WithContext(ctx).First(&round, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoundNotFound
		}
		return nil, err
	}
	return &round, nil
}

func (r *gormRecruitmentRepo) SetRoundAccepting(ctx context.Context, roundID int, accepting bool) error {
	result := r.db.WithContext(ctx).
		Model(&domain.RecruitmentRound{}).
		Where("id = ?", roundID).
		Update("accepting", accepting)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRoundNotFound
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 报名申请
 * ------------------------------------------------------------------ */

func (r *gormRecruitmentRepo) CreateApplication(ctx context.Context, app *domain.Application) error {
	if err := r.db.WithContext(ctx).Create(app).Error; err != nil {
		if IsDuplicateKey(err) {
			return ErrDuplicateApplication
		}
		return err
	}
	return nil
}

func (r *gormRecruitmentRepo) ListApplicationsByUser(ctx context.Context, userID int, page int, pageSize int) ([]*domain.Application, int64, error) {
	var apps []*domain.Application
	var total int64

	base := r.db.WithContext(ctx).Model(&domain.Application{}).Where("user_id = ?", userID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&apps).Error
	if err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

func (r *gormRecruitmentRepo) GetApplicationByID(ctx context.Context, id int) (*domain.Application, error) {
	var app domain.Application
	err := r.db.WithContext(ctx).First(&app, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, err
	}
	return &app, nil
}

func (r *gormRecruitmentRepo) ListApplications(ctx context.Context, filter ApplicationListFilter, page int, pageSize int) ([]*domain.Application, int64, error) {
	var apps []*domain.Application
	var total int64

	// Count 与 Find 必须带同一组条件，否则 total 是全表数、分页却是过滤结果。
	applyFilter := func(q *gorm.DB) *gorm.DB {
		if filter.RoundID > 0 {
			q = q.Where("round_id = ?", filter.RoundID)
		}
		if code := strings.TrimSpace(filter.DirectionCode); code != "" {
			q = q.Where("direction_code = ?", code)
		}
		if status := strings.TrimSpace(filter.Status); status != "" {
			q = q.Where("status = ?", status)
		}
		if !filter.From.IsZero() {
			q = q.Where("created_at >= ?", filter.From)
		}
		if !filter.To.IsZero() {
			q = q.Where("created_at <= ?", filter.To)
		}
		return q
	}

	base := r.db.WithContext(ctx).Model(&domain.Application{})
	if err := applyFilter(base).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := applyFilter(r.db.WithContext(ctx).Model(&domain.Application{})).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&apps).Error
	if err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

func (r *gormRecruitmentRepo) UpdateApplicationStatus(ctx context.Context, id int, status domain.ApplicationStatus) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&domain.Application{}).
		Where("id = ?", id).
		Update("status", string(status))
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

/* ------------------------------------------------------------------ *
 * 时间线
 * ------------------------------------------------------------------ */

func (r *gormRecruitmentRepo) AppendTimeline(ctx context.Context, appID int, status domain.ApplicationStatus) error {
	entry := &domain.ApplicationTimeline{
		ApplicationID: appID,
		ToStatus:      string(status),
	}
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *gormRecruitmentRepo) ListTimeline(ctx context.Context, applicationID int) ([]*domain.ApplicationTimeline, error) {
	var entries []*domain.ApplicationTimeline
	// 时间升序 = 状态演进的历史顺序
	err := r.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("created_at ASC, id ASC").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}
	return entries, nil
}