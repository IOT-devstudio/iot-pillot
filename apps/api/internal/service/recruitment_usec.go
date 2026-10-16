package service

import (
	"context"
	"errors"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 对外错误
 * ------------------------------------------------------------------ */

// ErrNoActiveRound 当前没有处于报名开放状态的轮次。
var ErrNoActiveRound = errors.New("当前没有开放报名的招新轮次")

// ErrRoundClosed 指定的轮次不在报名开放窗口内。
//
// 与 ErrNoActiveRound 区分：那个是"一轮都没开"，这个是"你指的这一轮没开"。
// 表单入口（#92）报的是后者 —— 表单绑定了具体某一轮，说"系统里没有开放轮次"
// 会让填写人以为是全局关闭。
var ErrRoundClosed = errors.New("该轮招新当前未开放报名")

// ErrDirectionClosed 目标方向未开放报名或被本期开放方向集排除。
var ErrDirectionClosed = errors.New("该方向当前不接受报名")

// ErrInvalidTransition 申请状态机不允许这次流转（例如已终态申请再改状态）。
var ErrInvalidTransition = errors.New("当前申请状态不允许此操作")

// ErrNotOwned 尝试读取/操作不属于自己的申请。
var ErrNotOwned = errors.New("无权访问该申请")

// ErrInvalidDirectionCode 创建方向时 code 格式非法。
var ErrInvalidDirectionCode = errors.New("方向 code 只能包含小写字母、数字和连字符，长度 1~32")

// ErrInvalidDirectionName 创建方向时 name 缺失或超长。
var ErrInvalidDirectionName = errors.New("方向名称不能为空且不能超过 64 个字符")

// maxDirectionCodeLen / maxDirectionNameLen 创建方向时的长度上限。
const (
	maxDirectionCodeLen = 32
	maxDirectionNameLen = 64
)

// validDirectionCode 判断 code 是否符合创建格式：小写字母 / 数字 / 连字符，1~32 字符。
//
// 独立格式校验，刻意不复用 domain.Direction 六值枚举（issue #128 决策）：
// domain.Direction 只服务 users.detail_direction（用户资料倾向），
// 招新方向 code 是运营自助数据，复用枚举会把可报名方向重新耦合回编译期。
// all / game / other 从不在方向表里（迁移 0007 只种三行），删白名单不会放开它们。
func validDirectionCode(code string) bool {
	if code == "" || len(code) > maxDirectionCodeLen {
		return false
	}
	for _, r := range code {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '-' {
			return false
		}
	}
	return true
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// RecruitmentUseCase 招新模块用例：方向读取、登录报名、本人申请查询、
// 管理端方向配置与申请审核。参赛双方（用户 / 管理端）共用底层状态机。
type RecruitmentUseCase struct {
	repo    repository.RecruitmentRepo
	profile ProfileSource
}

// ProfileSource 报名时从哪拿用户资料快照。收窄成接口便于单测注入桩。
type ProfileSource interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

// NewRecruitmentUseCase 构造函数。userRepo 满足 ProfileSource。
func NewRecruitmentUseCase(repo repository.RecruitmentRepo, users repository.UserRepo) *RecruitmentUseCase {
	return &RecruitmentUseCase{repo: repo, profile: users}
}

/* ------------------------------------------------------------------ *
 * 用户侧：方向读取
 * ------------------------------------------------------------------ */

// ListDirections 返回全部招新方向（含开放状态），供用户报名页展示。
func (u *RecruitmentUseCase) ListDirections(ctx context.Context) ([]*domain.RecruitmentDirection, error) {
	return u.repo.ListDirections(ctx)
}

// ListActiveDirections 只返回当前开放的方向。
// 报名页按这个渲染可选方向，保证不会让用户选到关停的方向。
// 准入判据是「方向表内存在且 open=true」（issue #128：方向表为唯一事实源）。
func (u *RecruitmentUseCase) ListActiveDirections(ctx context.Context) ([]*domain.RecruitmentDirection, error) {
	dirs, err := u.repo.ListDirections(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]*domain.RecruitmentDirection, 0, len(dirs))
	for _, d := range dirs {
		if d.Open {
			out = append(out, d)
		}
	}
	return out, nil
}

// SetDirectionOpen 管理端开启 / 关闭某方向的报名开关。
// 表中不存在的 code 由 repo 返回 ErrDirectionNotFound（400「招新方向不存在」），
// 不再依赖编译期白名单判断（issue #128）。
func (u *RecruitmentUseCase) SetDirectionOpen(ctx context.Context, code string, open bool) error {
	return u.repo.SetDirectionOpen(ctx, strings.TrimSpace(code), open)
}

// CreateDirection 管理端创建招新方向（issue #128）。
//
// open 固定为 false（与方向表列默认一致）：创建本身不开放报名，
// 需管理员显式 PATCH 开启，避免误开放。code 撞已有行时 repo 返回
// ErrDirectionCodeExists，已有行不被修改。
func (u *RecruitmentUseCase) CreateDirection(ctx context.Context, code, name string, sortOrder int) (*domain.RecruitmentDirection, error) {
	code = strings.TrimSpace(code)
	if !validDirectionCode(code) {
		return nil, ErrInvalidDirectionCode
	}

	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxDirectionNameLen {
		return nil, ErrInvalidDirectionName
	}

	dir := &domain.RecruitmentDirection{
		Code:      code,
		Name:      name,
		Open:      false,
		SortOrder: sortOrder,
	}
	if err := u.repo.CreateDirection(ctx, dir); err != nil {
		return nil, err
	}
	return dir, nil
}

/* ------------------------------------------------------------------ *
 * 用户侧：登录报名
 * ------------------------------------------------------------------ */

// Submit 登录用户报名某个方向：取当前开放轮次后交给 SubmitToRound。
//
// 保留这个入口给 /recruitment/applications：报名页报的就是"当前开放的那一轮"，
// 调用方不需要（也不该）指定轮次。
func (u *RecruitmentUseCase) Submit(ctx context.Context, userID int, directionCode string) (*domain.Application, error) {
	round, err := u.repo.GetActiveRound(ctx)
	if err != nil {
		// ErrRoundNotFound 已覆盖"没有开放轮次"，转成可读的业务错误
		if errors.Is(err, repository.ErrRoundNotFound) {
			return nil, ErrNoActiveRound
		}
		return nil, err
	}
	return u.SubmitToRound(ctx, userID, round.ID, directionCode)
}

// SubmitToRound 按**指定轮次**创建申请（表单模块 #92 的入口）。
//
// 表单在创建时就绑定了轮次与方向，提交时不能改成"当前开放的那一轮"。
// 校验：轮次开放 → 方向表内存在且开放 → 用户资料快照 → 申请 + 初始时间线。
// 方向准入不再过编译期白名单（issue #128）：GetDirectionByCode 不存在 →
// ErrDirectionClosed，dir.Open 两道检查兜底。两个入口共享同一条唯一约束
// (user_id, round_id, direction_code)，因此不会因为多了表单入口而多出第二条申请。
func (u *RecruitmentUseCase) SubmitToRound(ctx context.Context, userID int, roundID int, directionCode string) (*domain.Application, error) {
	directionCode = strings.TrimSpace(directionCode)

	round, err := u.repo.GetRoundByID(ctx, roundID)
	if err != nil {
		return nil, err
	}
	if !round.Accepting {
		return nil, ErrRoundClosed
	}

	dir, err := u.repo.GetDirectionByCode(ctx, directionCode)
	if err != nil {
		if errors.Is(err, repository.ErrDirectionNotFound) {
			return nil, ErrDirectionClosed
		}
		return nil, err
	}
	if !dir.Open {
		return nil, ErrDirectionClosed
	}

	profile, err := u.profile.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	app := &domain.Application{
		UserID:        userID,
		RoundID:       round.ID,
		DirectionCode: dir.Code,
		Status:        domain.ApplicationApplied,
		Name:          profile.Name,
		Email:         profile.Detail.Email,
		Class:         profile.Detail.Class,
		StudentID:     int64(profile.Detail.StudentID),
		QQ:            profile.Detail.QQ,
	}

	if err := u.repo.CreateApplication(ctx, app); err != nil {
		return nil, err
	}

	if err := u.repo.AppendTimeline(ctx, app.ID, domain.ApplicationApplied); err != nil {
		return nil, err
	}

	return app, nil
}

/* ------------------------------------------------------------------ *
 * 用户侧：本人申请查询
 * ------------------------------------------------------------------ */

// ListMyApplications 分页列出当前用户本人的申请。
// userID 来自令牌，天然只读本人数据。
func (u *RecruitmentUseCase) ListMyApplications(ctx context.Context, userID int, page int, pageSize int) ([]*domain.Application, int64, error) {
	return u.repo.ListApplicationsByUser(ctx, userID, page, pageSize)
}

// GetMyApplication 取当前用户本人的一条申请（含时间线）。
// 申请不属于该用户时返回 ErrNotOwned，不泄露他人数据。
func (u *RecruitmentUseCase) GetMyApplication(ctx context.Context, userID int, appID int) (*domain.Application, []*domain.ApplicationTimeline, error) {
	app, err := u.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	if app.UserID != userID {
		return nil, nil, ErrNotOwned
	}

	timeline, err := u.repo.ListTimeline(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	return app, timeline, nil
}

/* ------------------------------------------------------------------ *
 * 管理端：轮次与方向配置
 * ------------------------------------------------------------------ */

// ListRounds 罗列全部招新轮次。
func (u *RecruitmentUseCase) ListRounds(ctx context.Context) ([]*domain.RecruitmentRound, error) {
	return u.repo.ListRounds(ctx)
}

// SetRoundAccepting 开启 / 关闭某轮的报名窗口。轮次不存在返回 ErrRoundNotFound。
func (u *RecruitmentUseCase) SetRoundAccepting(ctx context.Context, roundID int, accepting bool) error {
	return u.repo.SetRoundAccepting(ctx, roundID, accepting)
}

/* ------------------------------------------------------------------ *
 * 管理端：申请审核
 * ------------------------------------------------------------------ */

// AdminListApplications 分页列出全部申请（可筛选），供管理端查询。
func (u *RecruitmentUseCase) AdminListApplications(ctx context.Context, filter repository.ApplicationListFilter, page int, pageSize int) ([]*domain.Application, int64, error) {
	return u.repo.ListApplications(ctx, filter, page, pageSize)
}

// AdminGetApplication 取一条申请详情（含时间线）。
func (u *RecruitmentUseCase) AdminGetApplication(ctx context.Context, appID int) (*domain.Application, []*domain.ApplicationTimeline, error) {
	app, err := u.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	timeline, err := u.repo.ListTimeline(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	return app, timeline, nil
}

// Review 推进某条申请到下一状态（applied → reviewing → accepted/rejected）。
//
// 审核动作分两步：
//   - 从 applied 到 reviewing：标记"进入审核"，记录时间线；
//   - 从 reviewing 到 accepted / rejected：给出结果，记录时间线。
//
// 只允许沿状态机合法流转；非法目标（终态再改、reviewing 之前的提前 accept）
// 返回 ErrInvalidTransition 且不做任何写操作。
func (u *RecruitmentUseCase) Review(ctx context.Context, appID int, target domain.ApplicationStatus) error {
	if !target.Valid() || target == domain.ApplicationApplied {
		return ErrInvalidTransition
	}

	app, err := u.repo.GetApplicationByID(ctx, appID)
	if err != nil {
		return err
	}

	if !app.Status.CanTransitionTo(target) {
		return ErrInvalidTransition
	}

	rows, err := u.repo.UpdateApplicationStatus(ctx, appID, target)
	if err != nil {
		return err
	}
	// 并发下别人刚改过：行数 0 说明状态已离开可流转区间，按非法流转处理
	if rows == 0 {
		return ErrInvalidTransition
	}

	if err := u.repo.AppendTimeline(ctx, appID, target); err != nil {
		return err
	}
	return nil
}
