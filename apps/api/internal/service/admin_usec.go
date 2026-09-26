package service

import (
	"context"
	"errors"
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

// NewSessionRevoker 把 *utils.TokenManager 作为 SessionRevoker 提供给装配层。
//
// 与 NewTokenIssuer 同理：wire 按类型连线，隐式接口满足对它无效；
// 接口声明在本包，转换函数就写在本包（utils 反向 import service 会成环）。
func NewSessionRevoker(manager *utils.TokenManager) SessionRevoker {
	return manager
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
	cfg *config.Config,
) (*AdminUseCase, error) {
	useCase := &AdminUseCase{
		userRepo: userRepo,
		admins:   admins,
		sessions: sessions,
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

	log.Printf("[admin] 已把 %d 个账号补种为管理员", len(ids))
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

/* ------------------------------------------------------------------ *
 * 用户列表
 * ------------------------------------------------------------------ */

// ListUsers 分页读取用户列表（管理端专用）。
//
// 只映射出必要字段：密码哈希绝不能出现在任何响应里，
// 所以这里显式构造 DTO，而不是把 domain.User 直接交给序列化。
func (a *AdminUseCase) ListUsers(ctx context.Context, page int, pageSize int) (*response.AdminUserListResp, error) {
	users, total, err := a.userRepo.GetAll(ctx, page, pageSize)
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
func (a *AdminUseCase) RevokeAdmin(ctx context.Context, userID int) (*response.AdminMutationResp, error) {
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

	return response.AdminUserResp{
		UserID:    user.ID,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		IsAdmin:   isAdmin,
		Detail:    response.NewUserProfileDetail(user.Detail),
	}
}
