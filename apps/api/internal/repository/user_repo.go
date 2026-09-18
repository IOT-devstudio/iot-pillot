package repository

import (
	"context"
	"errors"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// UserRepo 用户持久化。
//
// 注意：gormUserRepo 上还有 UpdateRole / UpdateStatus 两个方法，它们**不在本接口里**，
// 而且更新的是 domain.User 上并不存在的 role / status 列（直接调用会报列不存在）。
// 保留是因为它们可能对应尚未落地的字段规划；在补上那两列之前不要使用。
type UserRepo interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
	GetByName(ctx context.Context, name string) (*domain.User, error)
	// GetByEmail 按邮箱查用户（邮箱在 detail_email 列）。
	// 邮件模块用：按邮箱直发时先看对方是不是已注册用户，是的话记录里带上 userID。
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	SelectUserByNameAndPassword(ctx context.Context, name string, password string) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	GetAll(ctx context.Context, page int, pageSize int) ([]*domain.User, int64, error)
}

// gormUserRepo 包含 GORM 的 DB 实例
type gormUserRepo struct {
	db *gorm.DB
}

func (r *gormUserRepo) Update(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// NewUserRepo NewGORMUserRepo 构造函数，需要从外部（如 main.go）将 db 实例注入进来
func NewUserRepo(db *gorm.DB) UserRepo {
	return &gormUserRepo{
		db: db,
	}
}

func (r *gormUserRepo) GetByID(ctx context.Context, id int) (*domain.User, error) {
	var user domain.User
	// First 默认主键查询
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err // 返回其他数据库级别的异常
	}
	return &user, nil
}

func (r *gormUserRepo) GetByName(ctx context.Context, name string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail 按邮箱查用户。
//
// 列名必须是 detail_email：domain.User 里的 Detail 是 `embedded;embeddedPrefix:detail_`
// 嵌入结构体，邮箱落在 detail_email 上。原来这里写的是 "email"，
// 每次查询都会报 SQLSTATE 42703（字段不存在），
// 而调用方（按邮箱发信）把查询失败当成"对方未注册"，于是
// **即使邮箱属于已注册用户，记录里也会丢掉 userID** —— 静默的错误行为。
func (r *gormUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("detail_email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepo) SelectUserByNameAndPassword(ctx context.Context, name string, password string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("name = ? AND password = ?", name, password).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("login failed") // 模糊错误信息，防止暴力撞库
		}
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepo) Save(ctx context.Context, user *domain.User) error {
	// GORM 的 Save 方法是 Upsert：
	// 如果 user.ID 为空或 0，执行 INSERT；如果有值，执行 UPDATE 全字段。
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *gormUserRepo) GetAll(ctx context.Context, page int, pageSize int) ([]*domain.User, int64, error) {
	var users []*domain.User
	var total int64

	r.db.WithContext(ctx).Model(&domain.User{}).Count(&total)

	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *gormUserRepo) UpdateRole(ctx context.Context, userID int, role string) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("role", role).Error
}

func (r *gormUserRepo) UpdateStatus(ctx context.Context, userID int, status int) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("status", status).Error
}
