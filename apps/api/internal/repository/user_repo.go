package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// UserRepo 用户持久化。
//
// 软删分工（issue #142）：**登录面入口**（GetByName/GetByEmail/
// SelectUserByNameAndPassword）排除已注销行——注销后不能登录、不能
// 撞库注册占坑、不能走重置密码；**按 ID 与全量查询**（GetByID/GetAll）
// 不排除——管理端列表要展示已注销行并标 is_deleted，历史查询要能按
// user_id 找到人。排除条件统一写在列上（deleted_at IS NULL），
// 调用方不感知。
type UserRepo interface {
	// GetByID 按 ID 取用户，**含已注销行**（管理端与历史查询用）。
	GetByID(ctx context.Context, id int) (*domain.User, error)
	// GetByName 按用户名查用户（登录面，排除已注销行）。
	// 用户名**不保证唯一**（唯一性改为落在邮箱上，见迁移 0005），
	// 因此匹配到多行时返回 ErrAmbiguousUsername 而不是随便取一行 ——
	// 取任意一行会让"谁能登进去"取决于数据库返回顺序。
	GetByName(ctx context.Context, name string) (*domain.User, error)
	// GetByEmail 按邮箱查用户（登录面，排除已注销行；邮箱在 detail_email 列）。
	// 注销后同邮箱可重新注册（#142 Q5）：查重命中不到已删行，新注册放行。
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	// SelectUserByNameAndPassword 登录口令比对（排除已注销行）。
	SelectUserByNameAndPassword(ctx context.Context, name string, password string) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	// GetAll 全量分页（**含已注销行**，管理端列表标 is_deleted）。
	GetAll(ctx context.Context, page int, pageSize int) ([]*domain.User, int64, error)
	// SoftDelete 软删：置 deleted_at = now（幂等：已注销行返回 ErrUserAlreadyDeleted，
	// 不重复写）。邮箱唯一索引（迁移 0011 partial）随之释放该邮箱。
	SoftDelete(ctx context.Context, id int) error
}

// ErrUserAlreadyDeleted 对已注销账号重复注销。
var ErrUserAlreadyDeleted = errors.New("该账号已注销")

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
			// 必须返回 ErrUserNotFound 这个哨兵：调用方用 errors.Is 判断"没有这个用户"，
			// 就地 errors.New 出来的是另一个值（消息一样也判不中），
			// 于是"查不到"被当成真故障，注册对任何新邮箱都会 500。
			return nil, ErrUserNotFound
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
	// deleted_at IS NULL：已注销账号不参与登录（issue #142）。
	// 不排除的话注销形同无效——原用户名还能直接登进来。
	err := r.db.WithContext(ctx).
		Where("name = ? AND deleted_at IS NULL", name).
		Limit(2).
		Find(&users).Error
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
	// deleted_at IS NULL：登录面排除已注销行（#142）。副作用正是 Q5 想要的——
	// 注销释放邮箱后，注册查重（Register 走本方法）命中不到已删行，同邮箱可重注册。
	err := r.db.WithContext(ctx).
		Where("detail_email = ? AND deleted_at IS NULL", email).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 同 GetByID：返回哨兵而不是就地新建错误，否则注册的
			// "邮箱是否已被占用" 判断会走成 500。
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepo) SelectUserByNameAndPassword(ctx context.Context, name string, password string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Where("name = ? AND password = ? AND deleted_at IS NULL", name, password).
		First(&user).Error
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

	// 刻意不排软删行（issue #142 Q3）：历史保留，管理端列表标「账号已注销」
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

// SoftDelete 软删用户（issue #142）：置 deleted_at = now。
//
// 用 WHERE deleted_at IS NULL 做幂等闸门：已注销行 RowsAffected=0，
// 返回 ErrUserAlreadyDeleted 而不是重复写——重复注销是明确业务错误，
// 调用方要把它翻译成 400 而不是静默成功。
func (r *gormUserRepo) SoftDelete(ctx context.Context, id int) error {
	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 区分「账号不存在」与「已注销」：先查一次存在性
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&domain.User{}).
			Where("id = ?", id).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrUserNotFound
		}
		return ErrUserAlreadyDeleted
	}
	return nil
}
