package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
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
	// now 可注入，便于测试固定时钟；生产装配用 time.Now。
	now func() time.Time
}

// NewFreezeUseCase 构造冻结用例。
func NewFreezeUseCase(
	users repository.UserRepo,
	mailer FreezeMailer,
	sessions SessionRevoker,
) *FreezeUseCase {
	return &FreezeUseCase{users: users, mailer: mailer, sessions: sessions, now: time.Now}
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
}

// RunOnce 执行一次扫描并冻结。
//
// 幂等：所有写操作都带条件（Freeze 要求 frozen_at IS NULL），
// 因此重复执行、多实例并发执行都不会重复冻结或重复发信。
// 单个账号的失败只记日志、继续处理其余账号——一个坏邮箱不该让整批停摆。
func (uc *FreezeUseCase) RunOnce(ctx context.Context) (FreezeRunResult, error) {
	var result FreezeRunResult

	now := uc.now()
	deadline := now.Add(-FreezeGracePeriod)

	candidates, err := uc.users.ListFreezeCandidates(ctx, deadline)
	if err != nil {
		return result, err
	}
	result.Candidates = len(candidates)

	for _, user := range candidates {
		if user == nil {
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

// ErrNotFrozen 对未冻结账号执行解冻。
var ErrNotFrozen = errors.New("该账号当前未被冻结")

// Unfreeze 管理员解冻账号。
//
// 只清冻结标记，不动资料本身：解冻是「允许这个人重新登录去改资料」，
// 不是「替他把资料改对」。资料若仍不合规，下一轮扫描会再次冻结，
// 因此管理端应在解冻前确认对方已把资料补好。
func (uc *FreezeUseCase) Unfreeze(ctx context.Context, userID int) (*domain.User, error) {
	if err := uc.users.Unfreeze(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrUserNotFrozen) {
			return nil, ErrNotFrozen
		}
		return nil, err
	}

	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}
