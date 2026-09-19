package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// UserRepo 用户持久化。
type UserRepo interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
	// GetByName 按用户名查用户。
	// 用户名**不保证唯一**（唯一性改为落在邮箱上，见迁移 0005），
	// 因此匹配到多行时返回 ErrAmbiguousUsername 而不是随便取一行 ——
	// 取任意一行会让"谁能登进去"取决于数据库返回顺序。
	GetByName(ctx context.Context, name string) (*domain.User, error)
	// GetByEmail 按邮箱查用户（邮箱在 detail_email 列）。
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	SelectUserByNameAndPassword(ctx context.Context, name string, password string) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	GetAll(ctx context.Context, page int, pageSize int) ([]*domain.User, int64, error)
}

// ErrAmbiguousUsername 同一个用户名对应多个账号。
var ErrAmbiguousUsername = errors.New("用户名对应多个账号，请改用邮箱登录")

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("user not found")

// IsDuplicateKey 判断是否为唯一约束冲突。
//
// 没有开 gorm 的 TranslateError，只能看驱动返回的文本：PostgreSQL 唯一约束冲突的
// SQLSTATE 是 23505，驱动同时会带上 "duplicate key"。
// 只认这两个特征，其他错误原样上抛，不吞掉真正的问题。
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "23505") || strings.Contains(message, "duplicate key")
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

// GetByName 按用户名查用户。
//
// 只取两行就能判断唯一性：0 行 = 不存在，1 行 = 命中，2 行 = 重名。
// 重名时返回 ErrAmbiguousUsername，而不是像 First() 那样返回任意一行 ——
// 用户名唯一约束已移除（唯一性改到邮箱上），重名是可能出现的正常状态。
func (r *gormUserRepo) GetByName(ctx context.Context, name string) (*domain.User, error) {
	var users []*domain.User
	err := r.db.WithContext(ctx).Where("name = ?", name).Limit(2).Find(&users).Error
	if err != nil {
		return nil, err
	}

	switch len(users) {
	case 0:
		return nil, ErrUserNotFound
	case 1:
		return users[0], nil
	default:
		return nil, ErrAmbiguousUsername
	}
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
