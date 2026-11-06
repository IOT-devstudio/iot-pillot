package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * 登录 / 刷新拦截（0012）
 * ------------------------------------------------------------------ */

// fakeCounter 内存版 Counter：登录路径只用 Get / Incr / Del。
type fakeCounter struct {
	values map[string]int64
}

func newFakeCounter() *fakeCounter {
	return &fakeCounter{values: map[string]int64{}}
}

func (c *fakeCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	c.values[key]++
	return c.values[key], nil
}

func (c *fakeCounter) SetIfAbsent(_ context.Context, key string, _ time.Duration) (bool, error) {
	if _, ok := c.values[key]; ok {
		return false, nil
	}
	c.values[key] = 1
	return true, nil
}

func (c *fakeCounter) Get(_ context.Context, key string) (int64, error) {
	return c.values[key], nil
}

func (c *fakeCounter) Del(_ context.Context, keys ...string) error {
	for _, key := range keys {
		delete(c.values, key)
	}
	return nil
}

// newLoginUseCase 组装一个可跑登录路径的 AuthUseCase（限流走内存计数器）。
func newLoginUseCase(repo repository.UserRepo) *AuthUseCase {
	return NewAuthUseCase(
		repo,
		issue86TokenManager{},
		issue86CodeManager{},
		issue86AdminDirectory{},
		utils.NewLimiter(newFakeCounter()),
	)
}

// frozenUser 构造一个密码已知、已冻结的账号。
func frozenUser() *domain.User {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	return &domain.User{
		ID:       7,
		Name:     "张三",
		Password: mustHash("secret123"),
		Detail:   domain.Detail{Email: "u@example.com", Class: "软工230301", StudentID: 202311451},
		FrozenAt: &now,
	}
}

func mustHash(password string) string {
	hashed, err := utils.HashPassword(password)
	if err != nil {
		panic(err)
	}
	return hashed
}

// TestLogin_FrozenButCompliantSelfHeals 冻结标记 + 资料其实已合规 → 自愈为正常登录。
//
// 这是 0012 自助解冻引入的行为：资料既然合规，让用户卡在一个「实际没问题」的
// 冻结态上毫无意义。frozenUser() 的班级 软工230301 正好合规，因此走这条分支。
func TestLogin_FrozenButCompliantSelfHeals(t *testing.T) {
	user := frozenUser()
	repo := issue86UserRepoWith(user)
	uc := newLoginUseCase(repo)

	resp, err := uc.Login(context.Background(), &request.LoginReq{Username: user.Name, Password: "secret123"})
	if err != nil {
		t.Fatalf("资料已合规的冻结账号应自愈登录成功，实际 %v", err)
	}
	if resp.Scope != utils.ScopeFull {
		t.Fatalf("自愈后应拿到完整权限令牌，实际 scope=%q", resp.Scope)
	}
	if resp.Frozen {
		t.Fatal("自愈后不应仍标记为冻结")
	}
}

// TestLogin_FrozenIncompleteGetsRestrictedToken 冻结且资料不全 → 发受限令牌而不是拒绝。
//
// 这是本次改动的核心：拒绝会形成死锁（拿不到令牌 → 永远调不了补资料的 PUT /me）。
func TestLogin_FrozenIncompleteGetsRestrictedToken(t *testing.T) {
	user := frozenUser()
	user.Detail.Class = "" // 班级缺失 → 资料不合规
	repo := issue86UserRepoWith(user)
	uc := newLoginUseCase(repo)

	resp, err := uc.Login(context.Background(), &request.LoginReq{Username: user.Name, Password: "secret123"})
	if err != nil {
		t.Fatalf("冻结但资料不全应仍能登录（受限），实际 %v", err)
	}
	if resp.Scope != utils.ScopeProfileOnly {
		t.Fatalf("应拿到仅补资料的受限令牌，实际 scope=%q", resp.Scope)
	}
	if !resp.Frozen {
		t.Fatal("受限登录应标记 frozen")
	}
	if len(resp.FreezeIssues) == 0 {
		t.Fatal("受限登录应带上待补的资料项")
	}
	// 至少要有班级这一项，且字段名可被前端定位。
	var hasClass bool
	for _, issue := range resp.FreezeIssues {
		if issue.Field == string(domain.ProfileFieldClass) {
			hasClass = true
		}
	}
	if !hasClass {
		t.Fatalf("待补项应包含 class，实际 %+v", resp.FreezeIssues)
	}
}

func TestLogin_FrozenAccountWithWrongPasswordStaysCredentialError(t *testing.T) {
	// 冻结判定放在密码校验之后：密码错时仍按凭据错误返回，
	// 否则「猜对用户名就能确认该账号存在且被冻结」。
	user := frozenUser()
	repo := issue86UserRepoWith(user)
	uc := newLoginUseCase(repo)

	_, err := uc.Login(context.Background(), &request.LoginReq{Username: user.Name, Password: "wrong-pass"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("密码错误应返回 ErrInvalidCredentials，实际 %v", err)
	}
}

func TestLogin_NormalAccountStillWorks(t *testing.T) {
	user := frozenUser()
	user.FrozenAt = nil
	repo := issue86UserRepoWith(user)
	uc := newLoginUseCase(repo)

	if _, err := uc.Login(context.Background(), &request.LoginReq{Username: user.Name, Password: "secret123"}); err != nil {
		t.Fatalf("正常账号应能登录，实际 %v", err)
	}
}

// refreshTokenManager 让 ValidateRefreshToken 固定返回某个 userID。
type refreshTokenManager struct {
	issue86TokenManager
	userID int
	// scope 原令牌的范围；空串按 full（冻结前签发的完整令牌）。
	scope string
}

func (m refreshTokenManager) ValidateRefreshToken(_ context.Context, _ string) (int, string, string, error) {
	return m.userID, "张三", "member", nil
}

func (m refreshTokenManager) ValidateRefreshTokenWithScope(_ context.Context, _ string) (int, string, string, string, error) {
	scope := m.scope
	if scope == "" {
		scope = utils.ScopeFull
	}
	return m.userID, "张三", "member", scope, nil
}

// TestRefresh_FrozenAccountStaysRestricted 冻结账号刷新后仍只能是受限令牌。
//
// 旧行为是直接拒绝刷新；新行为不拒绝（用户还需要令牌去补资料），
// 但**必须把令牌钉在受限范围内**——否则冻结前签发的完整令牌
// 刷新一次就绕过了冻结，那冻结就形同虚设。
func TestRefresh_FrozenAccountStaysRestricted(t *testing.T) {
	user := frozenUser()
	repo := issue86UserRepoWith(user)
	uc := NewAuthUseCase(repo, refreshTokenManager{userID: user.ID}, issue86CodeManager{}, issue86AdminDirectory{}, nil)

	resp, err := uc.Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("冻结账号刷新不应被拒（需要令牌去补资料），实际 %v", err)
	}
	if resp.Scope != utils.ScopeProfileOnly {
		t.Fatalf("冻结账号刷新后应保持受限，实际 scope=%q", resp.Scope)
	}
}

/* ------------------------------------------------------------------ *
 * 管理端：列表过滤与解冻（0012）
 * ------------------------------------------------------------------ */

func TestAdminListUsers_HidesFrozenByDefault(t *testing.T) {
	normal := &domain.User{ID: 1, Name: "张三", Detail: domain.Detail{Email: "a@example.com"}}
	frozen := frozenUser()
	frozen.ID = 2

	repo := &issue86UserRepo{
		byID:   map[int]*domain.User{normal.ID: normal, frozen.ID: frozen},
		byName: map[string]*domain.User{normal.Name: normal, frozen.Name: frozen},
		all:    []*domain.User{frozen, normal},
	}
	uc := &AdminUseCase{userRepo: repo, admins: issue86AdminDirectory{}, sessions: noRevoker{}}

	// 默认：看不到冻结账号
	hidden, err := uc.ListUsers(context.Background(), 1, 20, false)
	if err != nil {
		t.Fatalf("ListUsers 失败: %v", err)
	}
	if hidden.Total != 1 || len(hidden.Items) != 1 {
		t.Fatalf("默认应只返回 1 个未冻结账号，实际 total=%d items=%d", hidden.Total, len(hidden.Items))
	}
	if hidden.Items[0].UserID != normal.ID {
		t.Fatalf("默认列表应只含未冻结账号，实际 %+v", hidden.Items)
	}

	// include_frozen=true：两个都返回，并带冻结标记与原因
	shown, err := uc.ListUsers(context.Background(), 1, 20, true)
	if err != nil {
		t.Fatalf("ListUsers(includeFrozen) 失败: %v", err)
	}
	if shown.Total != 2 || len(shown.Items) != 2 {
		t.Fatalf("include_frozen 应返回 2 个，实际 total=%d items=%d", shown.Total, len(shown.Items))
	}

	var frozenItem *struct {
		IsFrozen     bool
		FrozenAt     string
		FreezeReason string
	}
	for _, item := range shown.Items {
		if item.UserID != frozen.ID {
			continue
		}
		if !item.IsFrozen || item.FrozenAt == "" {
			t.Fatalf("冻结账号应带 is_frozen 与 frozen_at，实际 %+v", item)
		}
		_ = frozenItem
	}
}

func TestAdminUnfreezeUser(t *testing.T) {
	user := frozenUser()
	repo := issue86UserRepoWith(user)
	uc := &AdminUseCase{userRepo: repo, admins: issue86AdminDirectory{}, sessions: noRevoker{}}

	resp, err := uc.UnfreezeUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("解冻失败: %v", err)
	}
	if resp.UserID != user.ID || resp.IsFrozen {
		t.Fatalf("解冻响应不符: %+v", resp)
	}
	if user.IsFrozen() {
		t.Fatal("解冻后不应再是冻结状态")
	}
	// 三列都要清干净：只清 frozen_at 会留下原因/通知时间，
	// 下次冻结时「已通知过」的判断会把该发的信吞掉。
	user.FreezeReason = "班级不合规"
	notified := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	user.FreezeNotifiedAt = &notified
	user.FrozenAt = &notified
	if _, err := uc.UnfreezeUser(context.Background(), user.ID); err != nil {
		t.Fatalf("再次解冻失败: %v", err)
	}
	if user.FreezeReason != "" || user.FreezeNotifiedAt != nil {
		t.Fatalf("解冻应清空原因与通知时间，实际 reason=%q notifiedAt=%v", user.FreezeReason, user.FreezeNotifiedAt)
	}

	// 重复解冻：明确业务错误
	if _, err := uc.UnfreezeUser(context.Background(), user.ID); !errors.Is(err, ErrNotFrozen) {
		t.Fatalf("重复解冻应返回 ErrNotFrozen，实际 %v", err)
	}

	// 不存在的账号
	if _, err := uc.UnfreezeUser(context.Background(), 999); !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("不存在账号应返回 ErrUserNotFound，实际 %v", err)
	}
}

func TestAdminUnfreezeUser_RejectsDeletedAccount(t *testing.T) {
	user := frozenUser()
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	user.DeletedAt = &now

	repo := issue86UserRepoWith(user)
	uc := &AdminUseCase{userRepo: repo, admins: issue86AdminDirectory{}, sessions: noRevoker{}}

	if _, err := uc.UnfreezeUser(context.Background(), user.ID); !errors.Is(err, ErrUnfreezeDeleted) {
		t.Fatalf("已注销账号应拒绝解冻，实际 %v", err)
	}
}

// noopAccountMailer 空实现 AccountMailer。
type noopAccountMailer struct{}

func (noopAccountMailer) SendAccountDeletionNotice(_ context.Context, _ int, _ *domain.User) error {
	return nil
}
