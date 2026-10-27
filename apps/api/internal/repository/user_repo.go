package repository

import (
	"context"
	"errors"
	"strings"
	"time"

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
	// ListForAdmin 管理端分页列表，按 filter 过滤。
	//
	// 与 GetAll 并存而不是替换：GetAll 的「不排软删行」语义已被既有调用与
	// 测试依赖，新增过滤维度（冻结）时改签名会波及无关代码。
	ListForAdmin(ctx context.Context, page int, pageSize int, filter UserListFilter) ([]*domain.User, int64, error)
	// SoftDelete 软删：置 deleted_at = now（幂等：已注销行返回 ErrUserAlreadyDeleted，
	// 不重复写）。邮箱唯一索引（迁移 0011 partial）随之释放该邮箱。
	SoftDelete(ctx context.Context, id int) error
	// ListFreezeCandidates 列出「注册满 before 之前、未冻结、未注销」的账号，
	// 供定时冻结任务逐个按资料规则判定（0012）。
	ListFreezeCandidates(ctx context.Context, before time.Time) ([]*domain.User, error)
	// ListFrozen 列出当前所有已冻结、未注销的账号。
	//
	// 用途只有一个：定时任务把「管理员被误冻结」这种自锁状态捞回来。
	// 管理员的冻结是事故而不是处置——唯一最高权限账号一旦被冻，
	// 谁也调不了解冻接口。因此扫描每轮都要能看见冻结行做自救。
	ListFrozen(ctx context.Context) ([]*domain.User, error)
	// Freeze 条件冻结：仅当该行仍未冻结时写入，返回是否真的写入。
	//
	// 条件更新（WHERE frozen_at IS NULL）是**多实例幂等闸门**：同一批候选被
	// 两个进程同时扫到，只有一个能把 RowsAffected 拿到 1，另一个拿到 false，
	// 据此跳过重复发信与重复记日志。
	Freeze(ctx context.Context, id int, reason string, at time.Time) (bool, error)
	// MarkFreezeNotified 记录冻结通知邮件的发出时间（幂等：只写未通知过的行）。
	MarkFreezeNotified(ctx context.Context, id int, at time.Time) error
	// Unfreeze 解冻：清空 frozen_at / freeze_reason / freeze_notified_at。
	// 未冻结的行返回 ErrUserNotFrozen，调用方翻译成 400。
	Unfreeze(ctx context.Context, id int) error
}

// ErrUserAlreadyDeleted 对已注销账号重复注销。
var ErrUserAlreadyDeleted = errors.New("该账号已注销")

// ErrUserNotFrozen 对未冻结账号执行解冻（0012）。
var ErrUserNotFrozen = errors.New("该账号当前未被冻结")

// UserListFilter 管理端用户列表的过滤条件，零值表示不额外限制。
type UserListFilter struct {
	// IncludeFrozen 是否把已冻结账号纳入列表。
	//
	// 默认 false：冻结是系统判定「资料不合规」的处置状态，管理员日常名单里
	// 混进这批账号会淹没真正在推进招新的人。需要处置时显式打开。
	// 注意它**只影响冻结**，已注销行（deleted_at）仍按 #142 Q3 保留展示。
	IncludeFrozen bool
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

/* ------------------------------------------------------------------ *
 * 管理端列表过滤与冻结（0012）
 * ------------------------------------------------------------------ */

// ListForAdmin 管理端分页列表。
//
// 过滤条件与计数必须在**同一条查询条件**上：先 Count 再 Find 时若条件不一致，
// total 会比实际能翻到的行数多，前端分页出现永远翻不到的尾页。
// 因此这里把 filter 组装成可复用的 scope 函数，两次查询共用。
func (r *gormUserRepo) ListForAdmin(
	ctx context.Context,
	page int,
	pageSize int,
	filter UserListFilter,
) ([]*domain.User, int64, error) {
	var users []*domain.User
	var total int64

	scope := func(db *gorm.DB) *gorm.DB {
		// 已注销行照旧展示（#142 Q3）：历史与审计要能看到谁被注销了。
		if !filter.IncludeFrozen {
			db = db.Where("frozen_at IS NULL")
		}
		return db
	}

	if err := scope(r.db.WithContext(ctx).Model(&domain.User{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := scope(r.db.WithContext(ctx)).
		Order("id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ListFreezeCandidates 列出待判定的冻结候选：注册时间早于 before、未冻结、未注销。
//
// 不在 SQL 里表达「资料不合规」：规则是 Go 里的正则（domain.ProfileIssues），
// 在 SQL 里重写一遍必然与 Go 版本漂移。这里只按时间与状态粗筛，
// 逐条判定交给调用方；注册满 7 天的账号是有限集合，全量取出可接受。
func (r *gormUserRepo) ListFreezeCandidates(ctx context.Context, before time.Time) ([]*domain.User, error) {
	var users []*domain.User
	err := r.db.WithContext(ctx).
		Where("frozen_at IS NULL AND deleted_at IS NULL AND created_at <= ?", before).
		Order("id ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// ListFrozen 列出已冻结、未注销的账号（供管理员自救使用）。
//
// 为什么单独一条查询：ListFreezeCandidates 按定义排除已冻结行，
// 而「管理员已经被冻结」恰恰只能在已冻结行里发现。冻结集合是人工处置
// 的少量数据，全量取出没有性能问题。
func (r *gormUserRepo) ListFrozen(ctx context.Context) ([]*domain.User, error) {
	var users []*domain.User
	err := r.db.WithContext(ctx).
		Where("frozen_at IS NOT NULL AND deleted_at IS NULL").
		Order("id ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// Freeze 条件冻结。
//
// WHERE frozen_at IS NULL 承担两件事：多实例并发下只有一个进程能写成功，
// 以及「已冻结的账号不会被第二次改写原因」。返回 false 表示这一行已经被
// 别人（或上一轮扫描）冻结过，调用方应跳过发信。
func (r *gormUserRepo) Freeze(ctx context.Context, id int, reason string, at time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ? AND frozen_at IS NULL AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"frozen_at":     at,
			"freeze_reason": reason,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// MarkFreezeNotified 记录通知邮件的发出时间。
//
// 条件里**不要求** frozen_at IS NOT NULL：调用顺序是「先发通知、再写冻结」，
// 若要求已冻结，这条更新永远命中不了（冻结还没发生），于是每轮扫描都会
// 重发一封通知。这里只要求「尚未记录过通知时间」，重复调用是无害空操作。
func (r *gormUserRepo) MarkFreezeNotified(ctx context.Context, id int, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ? AND freeze_notified_at IS NULL", id).
		Update("freeze_notified_at", at).Error
}

// Unfreeze 解冻：三列一起清空。
//
// 用 WHERE frozen_at IS NOT NULL 区分「本来就没冻结」：
// 对未冻结账号点解冻是明确的误操作，要回 400 而不是静默成功。
func (r *gormUserRepo) Unfreeze(ctx context.Context, id int) error {
	result := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ? AND frozen_at IS NOT NULL", id).
		Updates(map[string]any{
			"frozen_at":          nil,
			"freeze_reason":      "",
			"freeze_notified_at": nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 区分「账号不存在」与「本来就没冻结」，两者对操作者的指引不同
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
		return ErrUserNotFrozen
	}
	return nil
}
