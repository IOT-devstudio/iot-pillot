package service

import (
	"context"
	"fmt"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 对外错误
 * ------------------------------------------------------------------ */

// ErrStudioProjectInvalid 项目字段不合法（直接复用领域层哨兵）。
//
// 不另起一个同名变量：两处各自 errors.New 出来的是两个不同的值，
// handler 用 errors.Is 判断时会漏掉其中一个，校验失败就变成 500。
var ErrStudioProjectInvalid = domain.ErrStudioProjectInvalid

// ErrStudioProjectMemberInvalidOwner member_id 指向的成员档案不存在。
var ErrStudioProjectMemberInvalidOwner = repository.ErrStudioProjectMemberOwnerNotFound

/* ------------------------------------------------------------------ *
 * 入参
 * ------------------------------------------------------------------ */

// CreateStudioProjectInput 创建项目入参。
type CreateStudioProjectInput struct {
	Fields domain.StudioProjectFields
}

// UpdateStudioProjectInput 更新项目入参：nil 表示「不改这一项」。
type UpdateStudioProjectInput struct {
	Name    *string
	Year    *string
	Summary *string
	Status  *domain.StudioProjectStatus
}

// AddProjectMemberInput 添加参与入参。
type AddProjectMemberInput struct {
	Fields domain.StudioProjectMemberFields
}

// UpdateProjectMemberInput 更新参与入参：nil 表示「不改这一项」。
type UpdateProjectMemberInput struct {
	Name *string
	Role *domain.StudioProjectMemberRole
	// MemberID 三态：nil = 不改；非 nil 且值为 nil = 解除成员关联；
	// 非 nil 且值非 nil = 关联该成员。
	MemberID  **int
	StartedAt *time.Time
	// EndedAt 三态：nil = 不改；非 nil 且值为 nil = 仍在参与；
	// 非 nil 且值非 nil = 结束于该时刻。
	EndedAt **time.Time
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// StudioProjectUseCase 工作室项目与参与（#198）。
//
// 只做项目域：知识库（#196）与成员名录（#195）都已各自独立，
// 这里只读成员档案做存在性校验，不写成员域。
type StudioProjectUseCase struct {
	projects repository.StudioProjectRepo
	// now 注入时钟：参与的默认开始时间在测试里必须可断言。
	now func() time.Time
}

// NewStudioProjectUseCase 构造函数。
func NewStudioProjectUseCase(projects repository.StudioProjectRepo) *StudioProjectUseCase {
	return &StudioProjectUseCase{projects: projects, now: time.Now}
}

// Create 创建项目。
func (uc *StudioProjectUseCase) Create(ctx context.Context, in CreateStudioProjectInput) (*domain.StudioProject, error) {
	fields := in.Fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}

	project := &domain.StudioProject{}
	fields.ApplyTo(project)
	if err := uc.projects.Create(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

// List 分页列出项目。
func (uc *StudioProjectUseCase) List(ctx context.Context, filter repository.StudioProjectListFilter, page int, pageSize int) ([]*domain.StudioProject, int64, error) {
	return uc.projects.List(ctx, filter, page, pageSize)
}

// Get 取项目详情（含参与名单）。
func (uc *StudioProjectUseCase) Get(ctx context.Context, id int) (*domain.StudioProject, []*domain.StudioProjectMember, error) {
	project, err := uc.projects.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	members, err := uc.projects.ListMembers(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return project, members, nil
}

// Update 更新项目元数据（补丁语义）。
func (uc *StudioProjectUseCase) Update(ctx context.Context, id int, in UpdateStudioProjectInput) (*domain.StudioProject, error) {
	current, err := uc.projects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fields := domain.StudioProjectFields{
		Name:    current.Name,
		Year:    current.Year,
		Summary: current.Summary,
		Status:  current.Status,
	}
	if in.Name != nil {
		fields.Name = *in.Name
	}
	if in.Year != nil {
		fields.Year = *in.Year
	}
	if in.Summary != nil {
		fields.Summary = *in.Summary
	}
	if in.Status != nil {
		fields.Status = *in.Status
	}

	fields = fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}

	fields.ApplyTo(current)
	if err := uc.projects.Update(ctx, current); err != nil {
		return nil, err
	}
	// 回读一次：updated_at 由数据库决定，直接返回内存对象会过时。
	return uc.projects.GetByID(ctx, id)
}

// Delete 删除项目（参与行级联删除）。
func (uc *StudioProjectUseCase) Delete(ctx context.Context, id int) error {
	return uc.projects.Delete(ctx, id)
}

/* ------------------------------------------------------------------ *
 * 参与
 * ------------------------------------------------------------------ */

// AddMember 为项目添加参与。
//
// 姓名为**快照**：它不从成员档案自动取，而是由调用方给出。
// 这样成员以后改名，历史参与仍显示当时的名字；同时外部参与者
// （没有成员档案）也能记录。
func (uc *StudioProjectUseCase) AddMember(ctx context.Context, projectID int, in AddProjectMemberInput) (*domain.StudioProjectMember, error) {
	// 项目必须存在：否则外键会拦住，但调用方拿到的会是驱动层错误。
	if _, err := uc.projects.GetByID(ctx, projectID); err != nil {
		return nil, err
	}

	fields := in.Fields.Normalize()
	// 开始时间缺省为现在：多数参与是「今天加入」，让调用方每次都填一遍
	// 反而容易填错。显式给了就用给的。
	if fields.StartedAt.IsZero() {
		fields.StartedAt = uc.now()
	}
	if err := fields.Validate(); err != nil {
		return nil, err
	}
	if err := uc.checkOwner(ctx, fields.MemberID); err != nil {
		return nil, err
	}

	member := &domain.StudioProjectMember{ProjectID: projectID}
	fields.ApplyTo(member)
	if err := uc.projects.AddMember(ctx, member); err != nil {
		return nil, err
	}
	return member, nil
}

// UpdateMember 更新参与行（补丁语义）。
//
// 先取回已存行再合并：ended_at 的三态（不改 / 清空 / 设值）只有在
// 「知道当前值」的前提下才有意义。
func (uc *StudioProjectUseCase) UpdateMember(ctx context.Context, projectID int, memberID int, in UpdateProjectMemberInput) (*domain.StudioProjectMember, error) {
	current, err := uc.findMember(ctx, projectID, memberID)
	if err != nil {
		return nil, err
	}

	fields := domain.StudioProjectMemberFields{
		MemberID:  current.MemberID,
		Name:      current.Name,
		Role:      current.Role,
		StartedAt: current.StartedAt,
		EndedAt:   current.EndedAt,
	}
	if in.Name != nil {
		fields.Name = *in.Name
	}
	if in.Role != nil {
		fields.Role = *in.Role
	}
	if in.MemberID != nil {
		fields.MemberID = *in.MemberID
	}
	if in.StartedAt != nil {
		fields.StartedAt = *in.StartedAt
	}
	if in.EndedAt != nil {
		fields.EndedAt = *in.EndedAt
	}

	fields = fields.Normalize()
	if err := fields.Validate(); err != nil {
		return nil, err
	}
	if err := uc.checkOwner(ctx, fields.MemberID); err != nil {
		return nil, err
	}

	fields.ApplyTo(current)
	if err := uc.projects.UpdateMember(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

// RemoveMember 移除参与行。
func (uc *StudioProjectUseCase) RemoveMember(ctx context.Context, projectID int, memberID int) error {
	return uc.projects.RemoveMember(ctx, projectID, memberID)
}

// ListMembers 列出项目参与。
func (uc *StudioProjectUseCase) ListMembers(ctx context.Context, projectID int) ([]*domain.StudioProjectMember, error) {
	if _, err := uc.projects.GetByID(ctx, projectID); err != nil {
		return nil, err
	}
	return uc.projects.ListMembers(ctx, projectID)
}

// findMember 在项目范围内找参与行。
//
// 用「列出再筛」而不是新增一个仓储方法：一个项目的参与行是个位数，
// 而「参与行必须属于该项目」这条约束在服务层表达一次就够，
// 不必为此再开一个几乎相同的查询。
func (uc *StudioProjectUseCase) findMember(ctx context.Context, projectID int, memberID int) (*domain.StudioProjectMember, error) {
	members, err := uc.projects.ListMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.ID == memberID {
			return m, nil
		}
	}
	return nil, repository.ErrStudioProjectMemberNotFound
}

// checkOwner 校验可选的 member_id 指向一个存在的成员档案。
func (uc *StudioProjectUseCase) checkOwner(ctx context.Context, memberID *int) error {
	if memberID == nil {
		return nil
	}
	if *memberID <= 0 {
		return fmt.Errorf("%w: member_id 必须是正整数", ErrStudioProjectInvalid)
	}
	exists, err := uc.projects.StudioMemberExists(ctx, *memberID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrStudioProjectMemberInvalidOwner
	}
	return nil
}
