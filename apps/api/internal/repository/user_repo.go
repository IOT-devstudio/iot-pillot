package repository

import (
	"context"
	"errors"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// UserRepo 接口保持不变
type UserRepo interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
	GetByName(ctx context.Context, name string) (*domain.User, error)
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

func (r *gormUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
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
