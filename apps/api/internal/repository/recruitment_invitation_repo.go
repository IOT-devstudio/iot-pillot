package repository

import (
	"context"
	"errors"
	"log"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// ErrApplicationNotFound 报名申请不存在。
var ErrApplicationNotFound = errors.New("报名申请不存在")

// ErrInvitationNotFound 邀请不存在（含 token 无效）。
var ErrInvitationNotFound = errors.New("邀请不存在")

// ErrCandidateNotFound 候选人不存在。
var ErrCandidateNotFound = errors.New("候选人不存在")

// ApplicationRepo 报名申请的持久化（BE-04 只需读路径与状态推进；完整 CRUD 归属 #88）。
//
// BE-04 把申请当作上游交付物消费：发送邀请校验存在与归属，接受邀请校验归属。
// 当 #88 交付真实实现时，本接口的实现应替换为 #88 的仓储，接口形状按需演化。
type ApplicationRepo interface {
	GetByID(ctx context.Context, id int) (*domain.Application, error)
	// UpdateStatus 推进申请状态（接受邀请时 applied → accepted）。
	// 失败由调用方降级为日志：邀请与候选人已落库，回滚只会制造不一致。
	UpdateStatus(ctx context.Context, id int, status string) error
}

// gormApplicationRepo BE-04 锚点实现：直接读锚点表 recruitment_applications。
type gormApplicationRepo struct {
	db *gorm.DB
}

// NewApplicationRepo 构造申请锚点仓储。#88 合入后此构造函数替换为其实现。
func NewApplicationRepo(db *gorm.DB) ApplicationRepo {
	return &gormApplicationRepo{db: db}
}

func (r *gormApplicationRepo) GetByID(ctx context.Context, id int) (*domain.Application, error) {
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

func (r *gormApplicationRepo) UpdateStatus(ctx context.Context, id int, status string) error {
	return r.db.WithContext(ctx).Model(&domain.Application{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// InvitationRepo 邀请的持久化。
type InvitationRepo interface {
	// GetByID 按行 id 取邀请。
	GetByID(ctx context.Context, id int) (*domain.Invitation, error)
	// GetByToken 按接受凭据取邀请；token 无效返回 ErrInvitationNotFound。
	GetByToken(ctx context.Context, token string) (*domain.Invitation, error)
	// GetPendingByApplication 取申请当前的 pending 邀请；没有返回 ErrInvitationNotFound。
	GetPendingByApplication(ctx context.Context, applicationID int) (*domain.Invitation, error)
	// List 管理端分页列表；applicationID > 0 时按申请过滤。
	List(ctx context.Context, page int, pageSize int, applicationID int) ([]*domain.Invitation, int64, error)
	// ListByUserID 本人邀请列表：邀请经申请表关联到归属用户。
	ListByUserID(ctx context.Context, userID int) ([]*domain.Invitation, error)
	// Create 新建邀请。撞 pending 部分唯一索引时返回 ErrInvitationConflict。
	Create(ctx context.Context, invitation *domain.Invitation) error
	// Update 保存邀请的状态/投递/接受字段变更。
	Update(ctx context.Context, invitation *domain.Invitation) error
}

// ErrInvitationConflict 同一申请已有一条 pending 邀请（部分唯一索引兜底）。
var ErrInvitationConflict = errors.New("该申请已有一条待接受的邀请")

// gormInvitationRepo GORM 实现。
type gormInvitationRepo struct {
	db *gorm.DB
}

// NewInvitationRepo 构造邀请仓储。
func NewInvitationRepo(db *gorm.DB) InvitationRepo {
	return &gormInvitationRepo{db: db}
}

func (r *gormInvitationRepo) GetByID(ctx context.Context, id int) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.db.WithContext(ctx).First(&invitation, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *gormInvitationRepo) GetByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&invitation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *gormInvitationRepo) GetPendingByApplication(ctx context.Context, applicationID int) (*domain.Invitation, error) {
	var invitation domain.Invitation
	err := r.db.WithContext(ctx).
		Where("application_id = ? AND status = ?", applicationID, domain.InvitationPending).
		First(&invitation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	return &invitation, nil
}

func (r *gormInvitationRepo) List(ctx context.Context, page int, pageSize int, applicationID int) ([]*domain.Invitation, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Invitation{})
	if applicationID > 0 {
		query = query.Where("application_id = ?", applicationID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var invitations []*domain.Invitation
	err := query.
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&invitations).Error
	if err != nil {
		return nil, 0, err
	}
	return invitations, total, nil
}

// ListByUserID 邀请经申请表关联用户：JOIN recruitment_applications 过滤归属人。
func (r *gormInvitationRepo) ListByUserID(ctx context.Context, userID int) ([]*domain.Invitation, error) {
	var invitations []*domain.Invitation
	err := r.db.WithContext(ctx).
		Joins("JOIN recruitment_applications ON recruitment_applications.id = recruitment_invitations.application_id").
		Where("recruitment_applications.user_id = ?", userID).
		Order("recruitment_invitations.id DESC").
		Find(&invitations).Error
	if err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *gormInvitationRepo) Create(ctx context.Context, invitation *domain.Invitation) error {
	err := r.db.WithContext(ctx).Create(invitation).Error
	if IsDuplicateKey(err) {
		return ErrInvitationConflict
	}
	return err
}

func (r *gormInvitationRepo) Update(ctx context.Context, invitation *domain.Invitation) error {
	return r.db.WithContext(ctx).Save(invitation).Error
}

// CandidateRepo 候选人与时间线的持久化。
type CandidateRepo interface {
	// GetByID 按行 id 取候选人。
	GetByID(ctx context.Context, id int) (*domain.Candidate, error)
	// GetByApplicationID 按申请取候选人；还不是候选人时返回 ErrCandidateNotFound。
	GetByApplicationID(ctx context.Context, applicationID int) (*domain.Candidate, error)
	// GetByUserID 按所属用户取候选人列表（用户端「我的候选人记录」）。
	GetByUserID(ctx context.Context, userID int) ([]*domain.Candidate, error)
	// List 分页列出候选人；status 为空串时不过滤。
	List(ctx context.Context, page int, pageSize int, status string) ([]*domain.Candidate, int64, error)
	// Create 新建候选人（接受邀请时）。application_id 撞唯一索引返回 ErrCandidateConflict。
	Create(ctx context.Context, candidate *domain.Candidate) error
	// Update 保存候选人状态变更。
	Update(ctx context.Context, candidate *domain.Candidate) error
	// Timeline 按候选人取完整时间线，按写入顺序升序。
	Timeline(ctx context.Context, candidateID int) ([]*domain.CandidateTimeline, error)
	// AppendTimeline 追加一条时间线。timeline 创建失败不能让状态流转整体失败，
	// 因此实现内部吞错只记日志（同 mail record 的取舍）。
	AppendTimeline(ctx context.Context, timeline *domain.CandidateTimeline)
}

// ErrCandidateConflict 该申请已是候选人（并发接受/重复接受的数据库防线）。
var ErrCandidateConflict = errors.New("该申请已是候选人")

// gormCandidateRepo GORM 实现。
type gormCandidateRepo struct {
	db *gorm.DB
}

// NewCandidateRepo 构造候选人仓储。
func NewCandidateRepo(db *gorm.DB) CandidateRepo {
	return &gormCandidateRepo{db: db}
}

func (r *gormCandidateRepo) GetByID(ctx context.Context, id int) (*domain.Candidate, error) {
	var candidate domain.Candidate
	err := r.db.WithContext(ctx).First(&candidate, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCandidateNotFound
		}
		return nil, err
	}
	return &candidate, nil
}

func (r *gormCandidateRepo) GetByApplicationID(ctx context.Context, applicationID int) (*domain.Candidate, error) {
	var candidate domain.Candidate
	err := r.db.WithContext(ctx).Where("application_id = ?", applicationID).First(&candidate).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCandidateNotFound
		}
		return nil, err
	}
	return &candidate, nil
}

func (r *gormCandidateRepo) GetByUserID(ctx context.Context, userID int) ([]*domain.Candidate, error) {
	var candidates []*domain.Candidate
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

func (r *gormCandidateRepo) List(ctx context.Context, page int, pageSize int, status string) ([]*domain.Candidate, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Candidate{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var candidates []*domain.Candidate
	err := query.
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&candidates).Error
	if err != nil {
		return nil, 0, err
	}
	return candidates, total, nil
}

func (r *gormCandidateRepo) Create(ctx context.Context, candidate *domain.Candidate) error {
	err := r.db.WithContext(ctx).Create(candidate).Error
	if IsDuplicateKey(err) {
		return ErrCandidateConflict
	}
	return err
}

func (r *gormCandidateRepo) Update(ctx context.Context, candidate *domain.Candidate) error {
	return r.db.WithContext(ctx).Save(candidate).Error
}

func (r *gormCandidateRepo) Timeline(ctx context.Context, candidateID int) ([]*domain.CandidateTimeline, error) {
	var timeline []*domain.CandidateTimeline
	err := r.db.WithContext(ctx).
		Where("candidate_id = ?", candidateID).
		Order("id ASC").
		Find(&timeline).Error
	if err != nil {
		return nil, err
	}
	return timeline, nil
}

// AppendTimeline 追加时间线。时间线写失败不回滚状态流转：状态已经推进，
// 抛错只会诱导调用方重试并触发非法流转拒绝；留日志暴露存储问题。
func (r *gormCandidateRepo) AppendTimeline(ctx context.Context, timeline *domain.CandidateTimeline) {
	if err := r.db.WithContext(ctx).Create(timeline).Error; err != nil {
		logTimelineFailure(timeline, err)
	}
}

// logTimelineFailure 时间线写失败的日志出口（放在 repo 层避免 service 依赖 log 细节）。
func logTimelineFailure(timeline *domain.CandidateTimeline, err error) {
	log.Printf("[recruitment] 候选人时间线写入失败（candidate %d）: %v", timeline.CandidateID, err)
}
