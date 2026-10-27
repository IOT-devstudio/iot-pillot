package service

import (
	"context"
	"log"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * 资料不合规账号的定时冻结（0012）
 * ------------------------------------------------------------------ */

const (
	// FreezeGracePeriod 注册后的资料补全宽限期。
	//
	// 注册只收姓名/邮箱/密码，班级与学号是登录后到个人设置页补的，
	// 因此必须给一段「先让人补」的时间窗。满 7 天仍不合规才冻结。
	FreezeGracePeriod = 7 * 24 * time.Hour

	// FreezeScanInterval 定时扫描间隔。
	//
	// 一天一次足够：冻结不是实时风控，晚几小时冻结不会造成实际损失；
	// 而扫描要遍历注册满 7 天的账号并可能批量发信，频率过高只是浪费。
	FreezeScanInterval = 24 * time.Hour
)

// FreezeMailer 冻结通知邮件的发送能力。
//
// 窄接口：冻结用例不该知道模板中心、发信记录这些 MailModule 内部细节
// （仓库约定：所有 SMTP 外发统一走 MailModule）。*MailUseCase 实现它。
type FreezeMailer interface {
	SendAccountFreezeNotice(ctx context.Context, user *domain.User, reason string) error
}

// NewFreezeMailer 把 *MailUseCase 作为 FreezeMailer 提供给装配层。
//
// 与 NewAccountMailer 同理：wire 按类型连线，隐式接口满足对它无效；
// 接口声明在本包，转换函数就写在本包（utils 反向 import service 会成环）。
func NewFreezeMailer(mailUseCase *MailUseCase) FreezeMailer {
	return mailUseCase
}

// FreezeUseCase 资料不合规账号的定时冻结。
//
// 编排顺序刻意是「先发通知、再冻结」：
//   - 邮件发不出去（SMTP 故障）时本轮不冻结，下一轮重试。冻结是系统自动行为，
//     在用户还没收到任何通知、也不知道要改什么的时候直接封号，是最差的体验。
//   - 反过来先冻结再发信，一旦发信失败就会留下「账号已冻结但用户毫不知情」
//     的状态，且没有任何重试入口。
type FreezeUseCase struct {
	users  repository.UserRepo
	mailer FreezeMailer
	// sessions 冻结成功后撤销该账号的全部会话。
	//
	// 不做这一步，冻结只挡住「新登录」和「刷新令牌」，而冻结前签发的 access
	// token 在过期前仍能调接口——账号看起来被封了，实际还能操作数据。
	sessions SessionRevoker
	// admins 管理员名单（Redis 真源），用于把管理员排除在自动冻结之外。
	//
	// 为什么必须排除：冻结会同时封死登录与解冻接口（解冻本身要管理员身份）。
	// 唯一最高权限账号被冻结后，系统就再没有任何人能通过 API 解冻它——
	// 这是自锁，不是处置。管理员资料不合规要靠人工提醒，不能靠自动封号。
	admins utils.AdminDirectory
	// now 可注入，便于测试固定时钟；生产装配用 time.Now。
	now func() time.Time
}

// NewFreezeUseCase 构造冻结用例。
func NewFreezeUseCase(
	users repository.UserRepo,
	mailer FreezeMailer,
	sessions SessionRevoker,
	admins utils.AdminDirectory,
) *FreezeUseCase {
	return &FreezeUseCase{
		users:    users,
		mailer:   mailer,
		sessions: sessions,
		admins:   admins,
		now:      time.Now,
	}
}

// FreezeRunResult 一次扫描的结果，供日志与测试断言。
type FreezeRunResult struct {
	// Candidates 注册满宽限期、未冻结、未注销的账号数（粗筛结果）
	Candidates int
	// Compliant 其中资料合规、无需处置的数量
	Compliant int
	// Frozen 本轮成功冻结的数量
	Frozen int
	// AlreadyFrozen 已被其他实例/上一轮冻结、本轮跳过的数量
	AlreadyFrozen int
	// MailFailed 邮件发送失败、本轮跳过冻结的数量（下一轮重试）
	MailFailed int
	// SkippedAdmins 本轮因是管理员而跳过冻结的数量
	SkippedAdmins int
	// RescuedAdmins 本轮被解冻的管理员数量（自救：此前被误冻结的管理员）
	RescuedAdmins int
}

// RunOnce 执行一次扫描并冻结。
//
// 幂等：所有写操作都带条件（Freeze 要求 frozen_at IS NULL），
// 因此重复执行、多实例并发执行都不会重复冻结或重复发信。
// 单个账号的失败只记日志、继续处理其余账号——一个坏邮箱不该让整批停摆。
//
// 管理员例外（hotfix）：
//  1. 已是管理员的账号**永不自动冻结**——冻结会同时封死登录与解冻入口，
//     唯一最高权限账号被冻后没有任何 API 能救回来；
//  2. 每轮先把「当前被冻结的管理员」解冻，这是对已经发生的自锁状态的自救。
func (uc *FreezeUseCase) RunOnce(ctx context.Context) (FreezeRunResult, error) {
	var result FreezeRunResult

	now := uc.now()
	deadline := now.Add(-FreezeGracePeriod)

	// 管理员名单读 Redis：读不到就整轮放弃。
	// 宁可这轮不冻结，也不能在不知道谁是管理员的情况下批量封号——
	// 那正是把最高权限账号锁死的路径。
	adminIDs, err := uc.admins.ListAdmins(ctx)
	if err != nil {
		return result, err
	}
	adminSet := make(map[int]struct{}, len(adminIDs))
	for _, id := range adminIDs {
		adminSet[id] = struct{}{}
	}

	result.RescuedAdmins = uc.rescueFrozenAdmins(ctx, adminSet, now)

	candidates, err := uc.users.ListFreezeCandidates(ctx, deadline)
	if err != nil {
		return result, err
	}
	result.Candidates = len(candidates)

	for _, user := range candidates {
		if user == nil {
			continue
		}

		// 管理员一律跳过（见方法注释）：不判断资料、不发冻结邮件、不冻结。
		if _, isAdmin := adminSet[user.ID]; isAdmin {
			result.SkippedAdmins++
			continue
		}

		issues := domain.ProfileIssues(user)
		if len(issues) == 0 {
			result.Compliant++
			continue
		}
		reason := domain.ProfileIssuesSummary(issues)

		// 通知先发：失败则本轮不冻结，留给下一轮。
		// 已通知过的账号（freeze_notified_at 非空）不再重复发信——
		// 那一轮很可能是「邮件已发、冻结写库失败」，用户已经收到过通知。
		if user.FreezeNotifiedAt == nil {
			if err := uc.mailer.SendAccountFreezeNotice(ctx, user, reason); err != nil {
				result.MailFailed++
				log.Printf("[freeze] userID=%d 通知邮件发送失败，本轮跳过冻结（下一轮重试）: %v", user.ID, err)
				continue
			}
			if err := uc.users.MarkFreezeNotified(ctx, user.ID, now); err != nil {
				// 记不上通知时间不影响冻结：邮件确实发出去了，
				// 最坏结果是下一轮再发一封（可接受，且日志留痕）。
				log.Printf("[freeze] userID=%d 通知时间写入失败（下一轮可能重发）: %v", user.ID, err)
			}
		}

		frozen, err := uc.users.Freeze(ctx, user.ID, reason, now)
		if err != nil {
			log.Printf("[freeze] userID=%d 冻结写入失败: %v", user.ID, err)
			continue
		}
		if !frozen {
			// 条件更新没命中：已被其他实例冻结，或账号在此期间被注销
			result.AlreadyFrozen++
			continue
		}

		result.Frozen++
		log.Printf("[freeze] userID=%d 已冻结，原因：%s", user.ID, reason)

		// 冻结生效后撤销全部会话：否则冻结前签发的 access token 在过期前
		// 仍能调接口。失败不回滚冻结（冻结已经是既成事实，且登录面已封死），
		// 但必须记日志——这时旧令牌最多活到自然过期。
		if err := uc.sessions.RevokeUserSessions(ctx, user.ID); err != nil {
			log.Printf("[freeze] userID=%d 已冻结，但撤销会话失败（旧令牌将在过期后失效）: %v", user.ID, err)
		}
	}

	return result, nil
}

// rescueFrozenAdmins 解冻「当前被冻结的管理员」，返回解冻数量。
//
// 这是自锁状态的自救通道：0012 上线首轮扫描按资料规则冻结了管理员，
// 而解冻接口要求管理员身份——锁死。每轮扫描先跑这段，容器重启后
// 管理员即恢复，不依赖人工改库。
//
// 只对管理员生效：普通用户的冻结是正常处置，不能被这里顺手放开。
// 单个失败只记日志，不影响本轮其余账号的处理。
func (uc *FreezeUseCase) rescueFrozenAdmins(
	ctx context.Context,
	adminSet map[int]struct{},
	now time.Time,
) int {
	if len(adminSet) == 0 {
		return 0
	}

	frozenUsers, err := uc.users.ListFrozen(ctx)
	if err != nil {
		log.Printf("[freeze] 读取已冻结账号失败（本轮跳过管理员自救）: %v", err)
		return 0
	}

	rescued := 0
	for _, user := range frozenUsers {
		if user == nil || !user.IsFrozen() {
			continue
		}
		if _, isAdmin := adminSet[user.ID]; !isAdmin {
			continue
		}

		if err := uc.users.Unfreeze(ctx, user.ID); err != nil {
			log.Printf("[freeze] 管理员 userID=%d 已冻结但解冻失败（下一轮重试）: %v", user.ID, err)
			continue
		}
		rescued++
		log.Printf("[freeze] 已解冻误冻结的管理员 userID=%d（原因曾是：%s），解冻时刻 %s",
			user.ID, user.FreezeReason, now.Format(time.RFC3339))
	}
	return rescued
}
