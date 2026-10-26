package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/config"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// SessionRevoker 管理员服务对"踢下线"的全部依赖。
//
// 用接口而不是 *utils.TokenManager：角色变更的核心逻辑（谁被提升/撤销、
// 名单怎么写）不应该被 Redis 会话细节缠住，测试也就不必起 Redis。
type SessionRevoker interface {
	RevokeUserSessions(ctx context.Context, userID int) error
}

// AccountMailer 注销通知邮件的发送能力（issue #159）。
//
// 窄接口收窄到「给某用户发注销通知」一个方法：AdminUseCase 不该知道
// 模板中心、发信记录这些 MailModule 内部细节（仓库约定：邮件统一走 MailModule）。
type AccountMailer interface {
	SendAccountDeletionNotice(ctx context.Context, operatorID int, user *domain.User) error
}

// NewSessionRevoker 把 *utils.TokenManager 作为 SessionRevoker 提供给装配层。
//
// 与 NewTokenIssuer 同理：wire 按类型连线，隐式接口满足对它无效；
// 接口声明在本包，转换函数就写在本包（utils 反向 import service 会成环）。
func NewSessionRevoker(manager *utils.TokenManager) SessionRevoker {
	return manager
}

// NewAccountMailer 把 *MailUseCase 作为 AccountMailer 提供给装配层（issue #159）。
// 同款接口转换，理由见 NewSessionRevoker。
func NewAccountMailer(mailUseCase *MailUseCase) AccountMailer {
	return mailUseCase
}

// AdminUseCase 管理端用例：用户列表、管理员名单、角色变更。
//
// 从 AuthUseCase 里拆出来的原因：认证（谁能登录）与管理（谁能管理别人）
// 是两件事，混在一个 service 里会让 AuthUseCase 同时依赖用户表、
// 管理员名单、发信等多个方向，任何一处改动都要重新审一遍登录逻辑。
type AdminUseCase struct {
	userRepo repository.UserRepo
	admins   utils.AdminDirectory
	sessions SessionRevoker
	mailer   AccountMailer
}

// NewAdminUseCase 构造管理员用例，并执行一次启动期引导：把配置里的管理员补种进 Redis。
//
// 引导为什么放在构造函数里：它需要 DB（把用户名解析成 userID）与 Redis，而两者都由 wire
// 装配。放在这里就不必把依赖一路透传到 main，也不需要在 cmd 里留一个非 NewXxx 的胶水 provider。
//
// 引导是**尽力而为**的：名单里的用户名找不到账号时只记日志（新环境里管理员往往还没注册），
// 因此配置里写错一个名字不会让服务起不来；但 Redis 报错会返回 error —— 那种情况下
// 管理员名单不可信，管理端行为无法预期，宁可拒绝启动。只增不删的理由见 SeedAdmins。
func NewAdminUseCase(
	userRepo repository.UserRepo,
	admins utils.AdminDirectory,
	sessions SessionRevoker,
	mailer AccountMailer,
	cfg *config.Config,
) (*AdminUseCase, error) {
	useCase := &AdminUseCase{
		userRepo: userRepo,
		admins:   admins,
		sessions: sessions,
		mailer:   mailer,
	}

	if err := useCase.SeedAdmins(context.Background(), cfg.AUTH.AdminUsers); err != nil {
		return nil, err
	}

	return useCase, nil
}

/* ------------------------------------------------------------------ *
 * 启动引导
 * ------------------------------------------------------------------ */

// SeedAdmins 把配置里的管理员引导项补种进管理员名单（启动时执行一次）。
//
// 每一项既可以是**用户名**，也可以是**纯数字的 userID**：
// 先按用户名查，查不到再按 userID 查。两种都写不出结果时只记日志。
// 之所以两种都收：运维在配置里很容易直接写 "1"（想表达 ID），
// 如果只认用户名就会静默失效——配了却没有任何人是管理员，而且不报错。
//
// 只增不删：配置表达的是"初始有哪些管理员"，而不是"当前只允许这些管理员"。
// 若改成覆盖，运维在管理端新加的管理员会在下次重启时被静默抹掉。
//
// 找不到的引导项只记日志、不中断启动：新环境里管理员往往还没注册，
// 这时让服务起不来会把"引导还没完成"变成"服务不可用"。
// 真正的失败（Redis 报错）才返回错误。
func (a *AdminUseCase) SeedAdmins(ctx context.Context, entries []string) error {
	if len(entries) == 0 {
		return nil
	}

	var ids []int
	for _, entry := range entries {
		id, ok := a.resolveSeedEntry(ctx, entry)
		if !ok {
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		log.Printf("[admin] 引导名单里的 %d 项都没有对应账号，本次没有补种任何管理员"+
			"（账号注册后再重启，或直接把 userID 填进 auth.admin_users）", len(entries))
		return nil
	}

	if err := seedInto(ctx, a.admins, ids); err != nil {
		return err
	}

	// 同步写创始集合（issue #120）：两个集合分工——
	// auth:admins 是鉴权真源（晋升也会写），创始集合只标记"种入来源"（仅此处写）。
	if err := seedFoundingInto(ctx, a.admins, ids); err != nil {
		return err
	}

	log.Printf("[admin] 已把 %d 个账号补种为管理员（含创始标记）", len(ids))
	return nil
}

// resolveSeedEntry 把一条引导项解析成 userID。
//
// 先按用户名查（更直观、也没有歧义），查不到且该项是纯数字时再按 userID 查。
// 这个顺序保证"真的存在一个叫 1 的用户"时不会被当成 ID 抢走。
func (a *AdminUseCase) resolveSeedEntry(ctx context.Context, entry string) (int, bool) {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return 0, false
	}

	if user, err := a.userRepo.GetByName(ctx, trimmed); err == nil && user != nil {
		log.Printf("[admin] 引导：按用户名匹配到 %q（userID=%d）", trimmed, user.ID)
		return user.ID, true
	}

	if id, err := strconv.Atoi(trimmed); err == nil && id > 0 {
		if user, err := a.userRepo.GetByID(ctx, id); err == nil && user != nil {
			log.Printf("[admin] 引导：按 userID 匹配到 %d（%q）", user.ID, user.Name)
			return user.ID, true
		}
		log.Printf("[admin] 引导跳过：既没有名为 %q 的账号，也没有 userID=%d 的账号", trimmed, id)
		return 0, false
	}

	log.Printf("[admin] 引导跳过：找不到名为 %q 的账号（注册后再重启）", trimmed)
	return 0, false
}

// seedInto 走 AdminDirectory 的公开方法完成补种。
//
// 不在接口里再加一个 Seed 方法：AdminStore 有批量 SAdd 的实现，
// 但那是 Redis 细节；这里逐个 Add 已经足够（启动时只有个位数成员），
// 换来的是接口更小、桩实现更好写。
func seedInto(ctx context.Context, admins utils.AdminDirectory, ids []int) error {
	for _, id := range ids {
		if err := admins.AddAdmin(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// seedFoundingInto 把引导名单同步写进创始集合（issue #120）。
//
// 与 seedInto 同批次、同语义（只增不删、逐个写）：
// 创始集合标记"这个账号来自 ADMIN_USERS 配置"，供 /me 的 is_founding 做前端分流；
// 它不是第二套鉴权——中间件永远不读它，晋升管理员也不会写它。
func seedFoundingInto(ctx context.Context, admins utils.AdminDirectory, ids []int) error {
	for _, id := range ids {
		if err := admins.AddFounding(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 用户列表
 * ------------------------------------------------------------------ */

// ListUsers 分页读取用户列表（管理端专用）。
//
// 只映射出必要字段：密码哈希绝不能出现在任何响应里，
// 所以这里显式构造 DTO，而不是把 domain.User 直接交给序列化。
//
// 冻结过滤（0012）：默认不显示冻结账号，includeFrozen=true 才纳入。
// 过滤与分页在仓储层同一条查询条件上完成，total 与可翻页数一致。
func (a *AdminUseCase) ListUsers(
	ctx context.Context,
	page int,
	pageSize int,
	includeFrozen bool,
) (*response.AdminUserListResp, error) {
	users, total, err := a.userRepo.ListForAdmin(ctx, page, pageSize, repository.UserListFilter{
		IncludeFrozen: includeFrozen,
	})
	if err != nil {
		return nil, err
	}

	// 管理员名单一次取出，避免在循环里对每个用户查一次 Redis
	adminIDs, err := a.admins.ListAdmins(ctx)
	if err != nil {
		return nil, err
	}
	adminSet := toSet(adminIDs)

	items := make([]response.AdminUserResp, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		items = append(items, toAdminUserResp(user, adminSet))
	}

	return &response.AdminUserListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

/* ------------------------------------------------------------------ *
 * 管理员名单
 * ------------------------------------------------------------------ */

// ListAdmins 列出当前管理员。
//
// 名单里可能存在已被删除的用户 ID，这类直接跳过：否则管理端会显示一行
// 点不进去、也撤销不掉的空条目。
func (a *AdminUseCase) ListAdmins(ctx context.Context) (*response.AdminListResp, error) {
	ids, err := a.admins.ListAdmins(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]response.AdminUserResp, 0, len(ids))
	for _, id := range ids {
		user, err := a.userRepo.GetByID(ctx, id)
		if err != nil || user == nil {
			log.Printf("[admin] 名单里的 userID=%d 已不存在，跳过", id)
			continue
		}
		// 名单来源本身，is_admin 恒为 true
		items = append(items, toAdminUserResp(user, map[int]struct{}{id: {}}))
	}

	return &response.AdminListResp{Items: items, Total: len(items)}, nil
}

// GrantAdmin 把已注册用户提升为管理员。
//
// 提升后**强制该用户重新登录**：JWT 里的 role claim 是签发时写死的，
// 不踢下线的话他手里的旧令牌仍然声称 member，前端守卫会把管理端入口藏起来，
// 表现为"明明提升了却进不去"。
func (a *AdminUseCase) GrantAdmin(ctx context.Context, userID int) (*response.AdminMutationResp, error) {
	user, err := a.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("用户不存在")
	}

	if err := a.admins.AddAdmin(ctx, userID); err != nil {
		return nil, err
	}

	revoked := true
	if err := a.sessions.RevokeUserSessions(ctx, userID); err != nil {
		// 名单已经写入成功，这里失败只影响"旧令牌何时失效"，不该让整个操作回滚
		revoked = false
		log.Printf("[admin] 提升 userID=%d 后撤销会话失败（旧令牌会在过期后失效）: %v", userID, err)
	}

	return &response.AdminMutationResp{UserID: userID, IsAdmin: true, SessionRevoked: revoked}, nil
}

// RevokeAdmin 撤销管理员。
//
// 同样强制重新登录：不踢下线的话，他手里的旧令牌仍带着 admin claim，
// 前端会继续显示管理端入口（服务端因为读 Redis 已经拦住，所以只是体验不一致，
// 但"看起来还能进"本身就会让人怀疑权限没有生效）。
//
// 创始标记一并移除（issue #120 Q1 裁决）：撤销即失去全部管理员身份，
// 被撤销者重新晋升后 is_founding 必须回到 false，否则创始特权会复活残留。
// 清理放在移出主名单**之前**且错误外传：先清创始标记失败时管理员身份还在，
// 操作可安全重试；若先删主名单再清创始标记失败，就会留下"非管理员但带创始标记"
// 的不一致状态。
func (a *AdminUseCase) RevokeAdmin(ctx context.Context, userID int) (*response.AdminMutationResp, error) {
	if err := a.admins.RemoveFounding(ctx, userID); err != nil {
		return nil, err
	}

	if err := a.admins.RemoveAdmin(ctx, userID); err != nil {
		return nil, err
	}

	revoked := true
	if err := a.sessions.RevokeUserSessions(ctx, userID); err != nil {
		revoked = false
		log.Printf("[admin] 撤销 userID=%d 后撤销会话失败: %v", userID, err)
	}

	return &response.AdminMutationResp{UserID: userID, IsAdmin: false, SessionRevoked: revoked}, nil
}

/* ------------------------------------------------------------------ *
 * 账号注销（issue #142）
 * ------------------------------------------------------------------ */

// ErrSelfDeletion 尝试注销自己。
//
// 自注销是纯误操作：操作者自己消失、请求随即失去授权依据，
// 出问题也无法追查是谁点的。前端隐藏按钮只是体验，这里强制。
var ErrSelfDeletion = errors.New("不能注销自己的账号")

// ErrFoundingProtected 尝试注销其他创始管理员。
//
// 创始集合内互不注销（#142 Q4 裁决）：创始账号来自部署配置（ADMIN_USERS），
// 撤销它的唯一出口是改配置重启——给彼此开一个「互相注销」的口子，
// 等于让运营配置可以被页面上的误操作绕过。
var ErrFoundingProtected = errors.New("创始管理员账号不能被注销，请通过部署配置调整")

// DeleteUser 注销账号（创始管理员专属，#142）：软删 + 踢会话 + 清两个管理员集合。
//
// 前置鉴权（仅创始管理员可调）在中间件层，本方法只管业务守卫与编排：
//  1. 守卫：不能注销自己；不能注销其他创始管理员；重复注销返回明确业务错误
//  2. 软删（repo 层幂等闸门，已删行返回 ErrUserAlreadyDeleted）
//  3. 移出 auth:admins / auth:admins:founding（与 RevokeAdmin 同款顺序：
//     先创始后主名单，失败外传不静默——残留创始标记会让 is_founding 失真）
//  4. 踢掉全部会话（复用 RevokeUserSessions，与改密/撤销管理员同款）
//
// 历史记录（招新申请/表单提交/邮件）不删不改，按 user_id 保留（Q3）。
// 错误顺序刻意「守卫在前、副作用在后」：任何守卫失败都不产生半套状态。
func (a *AdminUseCase) DeleteUser(ctx context.Context, operatorID int, targetID int) error {
	if operatorID == targetID {
		return ErrSelfDeletion
	}

	target, err := a.userRepo.GetByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// 保留哨兵可判别性：handler 靠 errors.Is 翻译成 400「用户不存在」，
			// 就地 errors.New 出来的新错误判不中，会掉进 failInternal 变 500。
			return fmt.Errorf("用户不存在: %w", repository.ErrUserNotFound)
		}
		return err
	}

	// 目标是否创始：在软删前判定，避免注销了创始账号后才发现
	if found, err := a.admins.IsFounding(ctx, targetID); err != nil {
		return err
	} else if found {
		return ErrFoundingProtected
	}

	if err := a.userRepo.SoftDelete(ctx, targetID); err != nil {
		// ErrUserAlreadyDeleted 原样上抛，由 handler 翻译成 400
		return err
	}

	if err := a.admins.RemoveFounding(ctx, targetID); err != nil {
		return err
	}
	if err := a.admins.RemoveAdmin(ctx, targetID); err != nil {
		return err
	}

	// 踢会话失败不回滚软删（账号已注销，登录面已封死，残留令牌最多活到过期），
	// 但必须外传——与「注销成功」混在一起静默吞掉会让调用方误判。
	if err := a.sessions.RevokeUserSessions(ctx, targetID); err != nil {
		return fmt.Errorf("账号已注销，但清理会话失败（旧令牌将在过期后失效）: %w", err)
	}

	// 注销通知邮件（issue #159 Q3/Q4）：仅管理员发起时发；发送失败
	// **不阻断注销**——账号已注销是既成事实，回滚没有意义（学踢会话
	// 失败的降级先例），记日志供排查即可。自助注销不走这里。
	if err := a.mailer.SendAccountDeletionNotice(ctx, operatorID, target); err != nil {
		log.Printf("[admin] 注销通知邮件发送失败（userID=%d，注销已生效，不阻断）: %v",
			targetID, err)
	}

	log.Printf("[admin] 账号已注销: userID=%d name=%q（操作者 userID=%d）",
		targetID, target.Name, operatorID)
	return nil
}

/* ------------------------------------------------------------------ *
 * 解冻账号（0012）
 * ------------------------------------------------------------------ */

// UnfreezeUser 解冻因资料不合规被冻结的账号。
//
// 刻意**不踢会话**：冻结期间登录与刷新都被拒绝，该账号手上不会存在有效令牌；
// 解冻的意义正是让本人重新登录去补资料，此时踢会话没有任何对象可踢。
//
// 也不校验资料是否已补齐：解冻是「允许这个人重新登录」，资料是否改对由
// 下一轮定时扫描判定（仍不合规会再次冻结）。若在这里硬性要求资料合规，
// 管理员就无法在用户尚未登录、无法修改资料的情况下做任何处置。
func (a *AdminUseCase) UnfreezeUser(ctx context.Context, targetID int) (*response.AdminUnfreezeResp, error) {
	user, err := a.userRepo.GetByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// 保留哨兵可判别性：handler 靠 errors.Is 翻译成 400
			return nil, fmt.Errorf("用户不存在: %w", repository.ErrUserNotFound)
		}
		return nil, err
	}
	if user.IsDeleted() {
		return nil, ErrUnfreezeDeleted
	}

	if err := a.userRepo.Unfreeze(ctx, targetID); err != nil {
		if errors.Is(err, repository.ErrUserNotFrozen) {
			return nil, ErrNotFrozen
		}
		return nil, err
	}

	log.Printf("[admin] 账号已解冻: userID=%d name=%q", targetID, user.Name)
	return &response.AdminUnfreezeResp{UserID: targetID, IsFrozen: false}, nil
}

// ErrUnfreezeDeleted 尝试解冻一个已注销账号。
//
// 注销不可逆（#142），解冻它没有意义：登录面按 deleted_at 已经封死，
// 清掉 frozen_at 也不会让它重新可用。
var ErrUnfreezeDeleted = errors.New("该账号已注销，无法解冻")

// ErrNotFrozen 对未冻结账号执行解冻（0012）。
//
// 单独给「本来就没冻结」一个明确错误：对未冻结账号点解冻是误操作，
// 静默成功会让操作者以为解冻生效了，而实际上什么都没有发生。
var ErrNotFrozen = errors.New("该账号当前未被冻结")

/* ------------------------------------------------------------------ *
 * 辅助
 * ------------------------------------------------------------------ */

func toSet(ids []int) map[int]struct{} {
	set := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// toAdminUserResp 把领域实体映射成列表项。
//
// 收整个 *domain.User 而不是散参数：issue #57 起列表要带只读 detail，
// 逐字段透传会越拉越长；显式 DTO 映射同时保证 password 不外泄。
func toAdminUserResp(user *domain.User, adminSet map[int]struct{}) response.AdminUserResp {
	_, isAdmin := adminSet[user.ID]

	// 冻结字段（0012）：时刻按 RFC3339 输出，未冻结统一空串，
	// 前端无需为「有 frozen_at 但没 reason」这类半套状态写分支。
	frozenAt := ""
	if user.FrozenAt != nil {
		frozenAt = user.FrozenAt.Format(time.RFC3339)
	}

	return response.AdminUserResp{
		UserID:    user.ID,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		IsAdmin:   isAdmin,
		// 已注销行保留在列表里（#142 Q3），前端按此标「账号已注销」
		IsDeleted:    user.IsDeleted(),
		IsFrozen:     user.IsFrozen(),
		FrozenAt:     frozenAt,
		FreezeReason: user.FreezeReason,
		Detail:       response.NewUserProfileDetail(user.Detail),
	}
}
