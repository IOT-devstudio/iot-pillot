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

// ErrCandidateNotHired 只有 hired 状态的候选人才能转为成员。
//
// 文案直接说清「现在是哪个状态」，管理员据此知道该先做什么动作
// （先发 offer、先确认录用），而不是只收到一句「不能转入」。
var ErrCandidateNotHired = errors.New("只有已录用（hired）的候选人才能转为工作室成员")

// ErrCandidateAlreadyConverted 该候选人已经转过一次成员档案。
//
// 与 ErrStudioMemberDuplicate 分开：那个哨兵同时覆盖「同名同出生年」，
// 管理员看到「已存在同名同出生年的成员」时无法判断是自己填重了、
// 还是这个候选人早就转过。转入路径必须能区分这两件事。
var ErrCandidateAlreadyConverted = errors.New("该候选人已转为工作室成员，无需重复转入")

// 候选人不存在直接复用招新域已有的 ErrCandidateMissing（recruitment_invitation_usec.go）：
// 两处各自 errors.New 出来的是两个不同的值，errors.Is 会漏掉一个。

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

// StudioMemberCandidateDirectory 「hired 候选人转成员」对招新域的全部依赖。
//
// 只读两个方法：转入动作要读候选人（判断状态、拿 application_id）与申请
// （拿姓名、方向、账号）。写操作一律落在成员仓储，**不改招新状态机**
// （#197 验收明确要求）。
//
// 由消费者声明、装配层按「返回值就是接口」连线（与 ApplicationStore 同一模式），
// 这样服务层单测不必起数据库。
type StudioMemberCandidateDirectory interface {
	GetByID(ctx context.Context, id int) (*domain.Candidate, error)
	GetApplicationByID(ctx context.Context, id int) (*domain.Application, error)
}

// studioMemberCandidateDirectory 把两个招新仓储合成一个只读目录。
//
// 为什么不直接让某个仓储实现这个接口：候选人在 CandidateRepo、申请在
// RecruitmentRepo，是两个独立的仓储。服务层只需要「按 id 读这两个东西」，
// 因此由装配层把两者拼起来，而不是让服务层知道仓储是怎么划分的。
type studioMemberCandidateDirectory struct {
	candidates   repository.CandidateRepo
	applications repository.RecruitmentRepo
}

func (d studioMemberCandidateDirectory) GetByID(ctx context.Context, id int) (*domain.Candidate, error) {
	return d.candidates.GetByID(ctx, id)
}

func (d studioMemberCandidateDirectory) GetApplicationByID(ctx context.Context, id int) (*domain.Application, error) {
	return d.applications.GetApplicationByID(ctx, id)
}

// NewStudioMemberCandidateDirectory 把两个招新仓储合成为候选人目录提供给装配层。
func NewStudioMemberCandidateDirectory(candidates repository.CandidateRepo, applications repository.RecruitmentRepo) StudioMemberCandidateDirectory {
	return studioMemberCandidateDirectory{candidates: candidates, applications: applications}
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

// StudioMemberUseCase 工作室成员名录（#195）与 hired 转成员（#197）。
//
// 只做成员域：项目参与是 #198 的后续票，不在这里预留方法。
type StudioMemberUseCase struct {
	members    repository.StudioMemberRepo
	accounts   StudioMemberAccountDirectory
	candidates StudioMemberCandidateDirectory
	// now 注入时钟：归档时间在测试里必须可断言，不依赖真实时钟。
	now func() time.Time
}

// NewStudioMemberUseCase 构造函数。
func NewStudioMemberUseCase(
	members repository.StudioMemberRepo,
	accounts StudioMemberAccountDirectory,
	candidates StudioMemberCandidateDirectory,
) *StudioMemberUseCase {
	return &StudioMemberUseCase{members: members, accounts: accounts, candidates: candidates, now: time.Now}
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

// ConvertCandidateInput 「hired 候选人转成员」入参。
//
// 只收候选人 id 与需要**人工补全**的字段：
// 姓名与方向从申请带过来（不让管理员重填，重填就有填错的可能），
// 而出生年 / 届别 / 职能在招新数据里根本不存在，必须由管理员给出。
type ConvertCandidateInput struct {
	CandidateID int
	Fields      domain.StudioMemberFields
}

// ConvertFromCandidate 把 hired 候选人转为工作室成员。
//
// 三条业务约束（#197 验收）：
//  1. 只有 hired 可转入 —— 其他状态一律拒绝，避免把还在面试的人提前入库；
//  2. 同一候选人只能转入一次 —— 靠 studio_members.source_candidate_id 的
//     部分唯一索引兜底（迁移 0013），仓储把 23505 翻译成 ErrStudioMemberDuplicate；
//  3. 不部分写入 —— 校验在写库前完成，写库是一次 Create，失败即整体不生效。
//
// **刻意不改招新状态机**：候选人仍是 hired，转入只是「另建一份成员档案」，
// 不是状态流转。这样将来若需要撤销转入，删成员档案即可，招新侧无副作用。
func (uc *StudioMemberUseCase) ConvertFromCandidate(ctx context.Context, in ConvertCandidateInput) (*domain.StudioMember, error) {
	candidate, err := uc.candidates.GetByID(ctx, in.CandidateID)
	if err != nil {
		if errors.Is(err, repository.ErrCandidateNotFound) {
			return nil, ErrCandidateMissing
		}
		return nil, err
	}
	if candidate.Status != domain.CandidateHired {
		return nil, fmt.Errorf("%w（当前状态：%s）", ErrCandidateNotHired, candidate.Status)
	}

	app, err := uc.candidates.GetApplicationByID(ctx, candidate.ApplicationID)
	if err != nil {
		return nil, err
	}

	// 姓名与方向以申请为准：它们已经在招新流程里确认过，
	// 让管理员再填一遍只会制造「两个地方不一致」的机会。
	fields := in.Fields
	fields.Name = app.Name
	fields.Direction = app.DirectionCode
	fields = fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}

	// 转入前先查一遍是否已转过：数据库的 source_candidate_id 部分唯一索引
	// 是最后一道防线，但只靠它的话，重复点击会先写进一条再撞约束回滚，
	// 日志里留下一次无意义的失败写入。前置检查让重复请求在**写库前**被拒绝，
	// 提示也更准确（「已转过」而不是「同名同出生年冲突」）。
	if err := uc.ensureNotConverted(ctx, candidate.ID); err != nil {
		return nil, err
	}

	// 候选人有系统账号就一并绑定；账号校验走与录入相同的路径。
	userID := &candidate.UserID
	if err := uc.checkUser(ctx, userID); err != nil {
		return nil, err
	}

	member := &domain.StudioMember{
		UserID:            userID,
		Name:              fields.Name,
		BirthYear:         fields.BirthYear,
		Cohort:            fields.Cohort,
		Direction:         fields.Direction,
		RoleInStudio:      fields.RoleInStudio,
		Internship:        fields.Internship,
		Employment:        fields.Employment,
		IsActive:          true,
		SourceCandidateID: &candidate.ID,
	}
	if err := uc.members.Create(ctx, member); err != nil {
		// 撞 source_candidate_id 唯一索引 = 这个候选人已经转过一次了。
		// 仓储对两个唯一索引都返回 ErrStudioMemberDuplicate，文案已覆盖两种情况。
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

// ensureNotConverted 确认该候选人还没转过成员档案。
//
// 已经转过时返回 ErrCandidateAlreadyConverted（而不是通用的「同名同出生年」），
// 这样管理端能给出准确提示，也能据此把按钮置灰。
func (uc *StudioMemberUseCase) ensureNotConverted(ctx context.Context, candidateID int) error {
	existing, err := uc.members.GetBySourceCandidateID(ctx, candidateID)
	if err != nil {
		if errors.Is(err, repository.ErrStudioMemberNotFound) {
			return nil
		}
		return err
	}
	return fmt.Errorf("%w（成员 #%d）", ErrCandidateAlreadyConverted, existing.ID)
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
