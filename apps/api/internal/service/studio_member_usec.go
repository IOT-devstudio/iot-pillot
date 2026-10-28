package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 对外错误
 * ------------------------------------------------------------------ */

// ErrStudioMemberInvalid 成员档案字段不合法（直接复用领域层哨兵）。
//
// 不另起一个同名变量：两处各自 errors.New 出来的是两个不同的值，
// handler 用 errors.Is 判断时会漏掉其中一个，校验失败就变成 500。
var ErrStudioMemberInvalid = domain.ErrStudioMemberInvalid

// ErrStudioMemberInvalidUser 关联的 user_id 不存在或已注销。
var ErrStudioMemberInvalidUser = repository.ErrStudioMemberUserNotFound

/* ------------------------------------------------------------------ *
 * 消费者声明的窄接口
 * ------------------------------------------------------------------ */

// StudioMemberAccountDirectory 成员档案对「关联系统账号」的全部依赖。
//
// 只读一个方法：录入成员时若要挂 user_id，必须确认那个账号真实存在且未注销——
// 否则档案会指向一个登不进来的人，后续「转为成员」与项目参与都拿不到身份。
//
// 由消费者声明、装配层按「返回值就是接口」连线（与 ApplicationStore 同一模式），
// 这样服务层单测不必起数据库。
type StudioMemberAccountDirectory interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

// NewStudioMemberAccountDirectory 把用户仓储作为账号目录提供给装配层。
func NewStudioMemberAccountDirectory(repo repository.UserRepo) StudioMemberAccountDirectory {
	return repo
}

/* ------------------------------------------------------------------ *
 * 入参
 * ------------------------------------------------------------------ */

// CreateStudioMemberInput 录入成员入参。
type CreateStudioMemberInput struct {
	Fields domain.StudioMemberFields
	// UserID 可选：nil = 无系统账号（校友 / 外部人员）。
	UserID *int
}

// UpdateStudioMemberInput 更新成员入参：nil 表示「不改这一项」。
//
// 与 Create 分开而不是复用同一个结构体：更新的语义是补丁，
// 「把姓名改成空」必须是显式的非法输入，而不是「没传」的副作用。
type UpdateStudioMemberInput struct {
	Name         *string
	BirthYear    *int
	Cohort       *string
	Direction    *string
	RoleInStudio *string
	Internship   *string
	Employment   *string
	// UserID 三态：nil = 不改；非 nil 且值为 nil = 解除绑定；
	// 非 nil 且值非 nil = 绑定该账号。
	UserID **int
	// IsActive 三态：nil = 不改。归档只走 Archive，
	// 但管理员可能想「手动恢复为在读」（例如归档点错了）。
	IsActive *bool
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// StudioMemberUseCase 工作室成员名录（#195）。
//
// 只做成员域：项目参与（#198）、知识库（#196）、hired 转成员（#197）
// 都是后续票，不在这里预留方法。
type StudioMemberUseCase struct {
	members  repository.StudioMemberRepo
	accounts StudioMemberAccountDirectory
	// now 注入时钟：归档时间在测试里必须可断言，不依赖真实时钟。
	now func() time.Time
}

// NewStudioMemberUseCase 构造函数。
func NewStudioMemberUseCase(members repository.StudioMemberRepo, accounts StudioMemberAccountDirectory) *StudioMemberUseCase {
	return &StudioMemberUseCase{members: members, accounts: accounts, now: time.Now}
}

// Create 录入成员档案。
//
// 无账号成员（UserID == nil）是一等公民，不要求先建用户：
// 校友与外部人员本来就不在系统里。
func (uc *StudioMemberUseCase) Create(ctx context.Context, in CreateStudioMemberInput) (*domain.StudioMember, error) {
	fields := in.Fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}
	if err := uc.checkUser(ctx, in.UserID); err != nil {
		return nil, err
	}

	member := &domain.StudioMember{
		UserID:       in.UserID,
		Name:         fields.Name,
		BirthYear:    fields.BirthYear,
		Cohort:       fields.Cohort,
		Direction:    fields.Direction,
		RoleInStudio: fields.RoleInStudio,
		Internship:   fields.Internship,
		Employment:   fields.Employment,
		IsActive:     true,
	}
	if err := uc.members.Create(ctx, member); err != nil {
		return nil, err
	}
	return member, nil
}

// List 分页列出成员。
func (uc *StudioMemberUseCase) List(ctx context.Context, filter repository.StudioMemberListFilter, page int, pageSize int) ([]*domain.StudioMember, int64, error) {
	return uc.members.List(ctx, filter, page, pageSize)
}

// Get 取成员详情。
func (uc *StudioMemberUseCase) Get(ctx context.Context, id int) (*domain.StudioMember, error) {
	return uc.members.GetByID(ctx, id)
}

// Update 更新成员资料。
//
// 先取当前行再合并补丁：唯一键是 (name, birth_year) 的组合，
// 只改其中一项时另一项必须来自已存的值，否则会把唯一键算错。
func (uc *StudioMemberUseCase) Update(ctx context.Context, id int, in UpdateStudioMemberInput) (*domain.StudioMember, error) {
	current, err := uc.members.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fields := domain.StudioMemberFields{
		Name:         current.Name,
		BirthYear:    current.BirthYear,
		Cohort:       current.Cohort,
		Direction:    current.Direction,
		RoleInStudio: current.RoleInStudio,
		Internship:   current.Internship,
		Employment:   current.Employment,
	}
	if in.Name != nil {
		fields.Name = *in.Name
	}
	if in.BirthYear != nil {
		fields.BirthYear = *in.BirthYear
	}
	if in.Cohort != nil {
		fields.Cohort = *in.Cohort
	}
	if in.Direction != nil {
		fields.Direction = *in.Direction
	}
	if in.RoleInStudio != nil {
		fields.RoleInStudio = *in.RoleInStudio
	}
	if in.Internship != nil {
		fields.Internship = *in.Internship
	}
	if in.Employment != nil {
		fields.Employment = *in.Employment
	}

	fields = fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}

	if in.UserID != nil {
		if err := uc.checkUser(ctx, *in.UserID); err != nil {
			return nil, err
		}
		current.UserID = *in.UserID
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
		// 恢复在读时清掉归档时刻，避免出现「is_active=true 但 archived_at 非空」
		// 这种自相矛盾的行（列表按 is_active 筛，详情却显示归档时间）。
		if *in.IsActive {
			current.ArchivedAt = nil
		}
	}

	current.Name = fields.Name
	current.BirthYear = fields.BirthYear
	current.Cohort = fields.Cohort
	current.Direction = fields.Direction
	current.RoleInStudio = fields.RoleInStudio
	current.Internship = fields.Internship
	current.Employment = fields.Employment

	if err := uc.members.Update(ctx, current); err != nil {
		return nil, err
	}
	// 回读一次：更新列里有 updated_at 等由数据库决定的值，
	// 直接把内存对象返回会让调用方看到过时的 updated_at。
	return uc.members.GetByID(ctx, id)
}

// Archive 归档成员。
//
// 不物理删除：毕业/退出的人仍要留在名录里，后续项目参与（#198）
// 会按 member_id 指回这条档案，删行会让历史参与变成孤儿。
func (uc *StudioMemberUseCase) Archive(ctx context.Context, id int) (*domain.StudioMember, error) {
	if err := uc.members.Archive(ctx, id, uc.now()); err != nil {
		return nil, err
	}
	return uc.members.GetByID(ctx, id)
}

// checkUser 校验可选的 user_id：必须存在且未注销。
func (uc *StudioMemberUseCase) checkUser(ctx context.Context, userID *int) error {
	if userID == nil {
		return nil
	}
	if *userID <= 0 {
		return fmt.Errorf("%w: user_id 必须是正整数", ErrStudioMemberInvalid)
	}
	user, err := uc.accounts.GetByID(ctx, *userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrStudioMemberInvalidUser
		}
		return err
	}
	// 已注销账号不能再挂到成员档案上：它是软删行，语义上「这个人已经离开系统」，
	// 挂上去会让后续按 user_id 反查身份的路径拿到一个登不进来的账号。
	if user.IsDeleted() {
		return ErrStudioMemberInvalidUser
	}
	return nil
}
