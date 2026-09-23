package repository

import (
	"context"
	"errors"
	"strings"
	"time"

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

// MailListFilter 发信记录的查询条件，零值字段表示不限制。
type MailListFilter struct {
	// Keyword 模糊匹配主题与收件人邮箱（ILIKE，大小写不敏感）
	Keyword string
	// From/To 时间范围，零值 = 不限该端；语义为 [From, To]
	From, To time.Time
}

// MailRepo 发信记录的持久化。
type MailRepo interface {
	Create(ctx context.Context, mail *domain.Mail) error
	List(ctx context.Context, page int, pageSize int, filter MailListFilter) ([]*domain.Mail, int64, error)
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

func (r *gormMailRepo) List(ctx context.Context, page int, pageSize int, filter MailListFilter) ([]*domain.Mail, int64, error) {
	var mails []*domain.Mail
	var total int64

	// Count 与 Find 必须带同一组条件，否则 total 是全表数、分页却是过滤结果，
	// 页码显示和实际条数会对不上。applyFilter 是纯条件叠加，两边各调一次。
	applyFilter := func(q *gorm.DB) *gorm.DB {
		if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
			// 转义 ILIKE 的通配符，用户搜 "100%" 应当匹配字面量而不是全表；
			// PostgreSQL 的 ILIKE 默认以 \ 为转义符
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)
			pattern := "%" + escaped + "%"
			q = q.Where("title ILIKE ? OR to_email ILIKE ?", pattern, pattern)
		}
		if !filter.From.IsZero() {
			q = q.Where("created_at >= ?", filter.From)
		}
		if !filter.To.IsZero() {
			q = q.Where("created_at <= ?", filter.To)
		}
		return q
	}

	base := r.db.WithContext(ctx).Model(&domain.Mail{})
	if err := applyFilter(base).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	// 按时间倒序；created_at 相同时用 id 兜底，保证分页不会漏行或重复
	err := applyFilter(r.db.WithContext(ctx).Model(&domain.Mail{})).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&mails).Error
	if err != nil {
		return nil, 0, err
	}

	return mails, total, nil
}
