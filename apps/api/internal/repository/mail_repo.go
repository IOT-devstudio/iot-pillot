package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// ErrDuplicateMailModel 模板名或类型撞上了唯一约束。
//
// 单独定义一个哨兵错误，让上层能给出「模板名或类型已存在」这种能看懂的提示，
// 而不是把驱动层的英文报错直接甩给管理端。
var ErrDuplicateMailModel = errors.New("模板名或模板类型已存在")

// MailModelRepo 邮件模板的持久化。
type MailModelRepo interface {
	List(ctx context.Context) ([]*domain.MailModel, error)
	GetByID(ctx context.Context, id int) (*domain.MailModel, error)
	Create(ctx context.Context, model *domain.MailModel) error
	Update(ctx context.Context, model *domain.MailModel) error
	Delete(ctx context.Context, id int) error
}

// MailRepo 发信记录的持久化。
type MailRepo interface {
	Create(ctx context.Context, mail *domain.Mail) error
	List(ctx context.Context, page int, pageSize int) ([]*domain.Mail, int64, error)
	// DeleteByIDs 按 id 批量硬删除，返回实际删除的行数。
	// id 不存在不算错误（并发场景下别人可能刚删过），行数会如实反映。
	DeleteByIDs(ctx context.Context, ids []int) (int64, error)
}

type gormMailModelRepo struct {
	db *gorm.DB
}

func NewMailModelRepo(db *gorm.DB) MailModelRepo {
	return &gormMailModelRepo{db: db}
}

func (r *gormMailModelRepo) List(ctx context.Context) ([]*domain.MailModel, error) {
	var models []*domain.MailModel
	// 按 id 升序：模板是低频改动的小表，顺序稳定比花哨的排序更重要
	err := r.db.WithContext(ctx).Order("id ASC").Find(&models).Error
	if err != nil {
		return nil, err
	}
	return models, nil
}

func (r *gormMailModelRepo) GetByID(ctx context.Context, id int) (*domain.MailModel, error) {
	var model domain.MailModel
	err := r.db.WithContext(ctx).First(&model, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("模板不存在")
		}
		return nil, err
	}
	return &model, nil
}

func (r *gormMailModelRepo) Create(ctx context.Context, model *domain.MailModel) error {
	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return translateMailModelError(err)
	}
	return nil
}

func (r *gormMailModelRepo) Update(ctx context.Context, model *domain.MailModel) error {
	if err := r.db.WithContext(ctx).Save(model).Error; err != nil {
		return translateMailModelError(err)
	}
	return nil
}

func (r *gormMailModelRepo) Delete(ctx context.Context, id int) error {
	result := r.db.WithContext(ctx).Delete(&domain.MailModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("模板不存在")
	}
	return nil
}

// translateMailModelError 把唯一约束冲突翻译成哨兵错误。
//
// 没有开 gorm 的 TranslateError，所以只能看驱动返回的文本：
// PostgreSQL 唯一约束冲突的 SQLSTATE 是 23505，驱动会把 "duplicate key" 也带上。
// 只认这两个特征，其他错误原样上抛，不吞掉真正的问题。
func translateMailModelError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if strings.Contains(message, "23505") || strings.Contains(message, "duplicate key") {
		return ErrDuplicateMailModel
	}
	return err
}

type gormMailRepo struct {
	db *gorm.DB
}

func NewMailRepo(db *gorm.DB) MailRepo {
	return &gormMailRepo{db: db}
}

func (r *gormMailRepo) Create(ctx context.Context, mail *domain.Mail) error {
	return r.db.WithContext(ctx).Create(mail).Error
}

func (r *gormMailRepo) List(ctx context.Context, page int, pageSize int) ([]*domain.Mail, int64, error) {
	var mails []*domain.Mail
	var total int64

	if err := r.db.WithContext(ctx).Model(&domain.Mail{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	// 按时间倒序；created_at 相同时用 id 兜底，保证分页不会漏行或重复
	err := r.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&mails).Error
	if err != nil {
		return nil, 0, err
	}

	return mails, total, nil
}

func (r *gormMailRepo) DeleteByIDs(ctx context.Context, ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&domain.Mail{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
