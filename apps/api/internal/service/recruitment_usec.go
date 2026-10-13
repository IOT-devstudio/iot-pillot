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

// ErrDirectionClosed 目标方向未开放报名或被本期开放方向集排除。
var ErrDirectionClosed = errors.New("该方向当前不接受报名")

// ErrInvalidTransition 申请状态机不允许这次流转（例如已终态申请再改状态）。
var ErrInvalidTransition = errors.New("当前申请状态不允许此操作")

// ErrNotOwned 尝试读取/操作不属于自己的申请。
var ErrNotOwned = errors.New("无权访问该申请")

// allowedDirections 本期开放报名的方向 code 白名单。
//
// 与 issue #88 拍板一致：front-end / back-end / agent 可报名，
// all / game / other 保留为用户资料倾向（users.detail_direction），不开放。
//
// 白名单校验放在服务层（业务规则），方向表里的 open 字段控制运营开关：
// 即使种子数据被管理员误设为前端，白名单也会挡住 all/game/other。
var allowedDirections = map[string]struct{}{
	string(domain.FrontEnd): {},
	string(domain.BackEnd):  {},
	string(domain.Agent):    {},
}

// directionAllowed 报告该方向 code 是否在本期开放报名集合内。
func directionAllowed(code string) bool {
	_, ok := allowedDirections[strings.TrimSpace(code)]
	return ok
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

// ListActiveDirections 只返回当前开放且在本期白名单内的方向。
// 报名页按这个渲染可选方向，保证不会让用户选到关停的方向。
func (u *RecruitmentUseCase) ListActiveDirections(ctx context.Context) ([]*domain.RecruitmentDirection, error) {
	dirs, err := u.repo.ListDirections(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]*domain.RecruitmentDirection, 0, len(dirs))
	for _, d := range dirs {
		if d.Open && directionAllowed(d.Code) {
			out = append(out, d)
		}
	}
	return out, nil
}

// SetDirectionOpen 管理端开启 / 关闭某方向的报名开关。
// 不在本期开放白名单内（all/game/other）的方向拒绝配置，返回 ErrDirectionClosed。
func (u *RecruitmentUseCase) SetDirectionOpen(ctx context.Context, code string, open bool) error {
	code = strings.TrimSpace(code)
	if !directionAllowed(code) {
		return ErrDirectionClosed
	}
	return u.repo.SetDirectionOpen(ctx, code, open)
}

/* ------------------------------------------------------------------ *
 * 用户侧：登录报名
 * ------------------------------------------------------------------ */

// Submit 登录用户报名某个方向。
//
// 前置校验（任一不满足返回相应域错误，不落库）：
//   1. 当前必须有开放报名的轮次
//   2. 目标方向必须在开放白名单内，且运营开关为开
// 然后从令牌（参数里的 userID 由 handler 从鉴权中间件取得）对应的
// 用户资料做快照写入申请，并追加一条初始时间线。
//
// 归属与方向一律由后端决定：body 里不能指定 user_id，防止越权代报。
func (u *RecruitmentUseCase) Submit(ctx context.Context, userID int, directionCode string) (*domain.Application, error) {
	directionCode = strings.TrimSpace(directionCode)
	if !directionAllowed(directionCode) {
		return nil, ErrDirectionClosed
	}

	round, err := u.repo.GetActiveRound(ctx)
	if err != nil {
		// ErrRoundNotFound 已覆盖"没有开放轮次"，转成可读的业务错误
		if errors.Is(err, repository.ErrRoundNotFound) {
			return nil, ErrNoActiveRound
		}
		return nil, err
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
		StudentID: int64(profile.Detail.StudentID),
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