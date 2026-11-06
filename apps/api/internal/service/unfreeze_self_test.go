package service

import (
	"context"
	"errors"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * 0012 自助解冻：冻结账号补齐资料才能解冻
 *
 * 这一组的核心不变式：**用户能拿到令牌去补资料，但补不完就解不了冻**。
 * 两条都必须成立——只满足前一条等于冻结失效，只满足后一条等于死锁。
 * ------------------------------------------------------------------ */

// frozenIncompleteUser 冻结且资料不合规（班级缺失）。
func frozenIncompleteUser() *domain.User {
	user := frozenUser()
	user.Detail.Class = ""
	return user
}

// TestLogin_FrozenIncompleteCanReachProfile 受限令牌必须是**可用**的令牌。
//
// 只断言 scope 不够：如果令牌根本没签出来，用户照样补不了资料。
func TestLogin_FrozenIncompleteCanReachProfile(t *testing.T) {
	user := frozenIncompleteUser()
	uc := newLoginUseCase(issue86UserRepoWith(user))

	resp, err := uc.Login(context.Background(), &request.LoginReq{Username: user.Name, Password: "secret123"})
	if err != nil {
		t.Fatalf("冻结但资料不全应仍能拿到受限令牌，实际 %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("受限登录也必须签出可用的双令牌，否则用户无法补资料")
	}
}

// TestLogin_FrozenIssuesListAllMissing 待补项要一次给全。
//
// 只给首条会让用户「改一项 → 再被拦一次 → 再改一项」，
// 每次都要重新登录，体验极差。
func TestLogin_FrozenIssuesListAllMissing(t *testing.T) {
	user := frozenUser()
	user.Detail.Class = ""
	user.Detail.StudentID = 0
	uc := newLoginUseCase(issue86UserRepoWith(user))

	resp, err := uc.Login(context.Background(), &request.LoginReq{Username: user.Name, Password: "secret123"})
	if err != nil {
		t.Fatalf("应能登录，实际 %v", err)
	}
	if len(resp.FreezeIssues) != 2 {
		t.Fatalf("班级与学号都缺，应给 2 项，实际 %d 项：%+v", len(resp.FreezeIssues), resp.FreezeIssues)
	}
}

// TestSubmitUnfreeze_RejectsWhenProfileStillIncomplete 差一项就不解冻。
//
// 这是最关键的一条：解冻判定必须真的看资料，而不是「提交了就放行」。
func TestSubmitUnfreeze_RejectsWhenProfileStillIncomplete(t *testing.T) {
	user := frozenIncompleteUser()
	repo := issue86UserRepoWith(user)
	uc := NewAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{}, nil)

	resp, err := uc.SubmitUnfreeze(context.Background(), user.ID)
	if !errors.Is(err, ErrProfileIncomplete) {
		t.Fatalf("资料不全时提交解冻应返回 ErrProfileIncomplete，实际 %v", err)
	}
	if resp == nil || len(resp.FreezeIssues) == 0 {
		t.Fatal("拒绝时应回传剩余待补项，让用户知道改什么")
	}
	// 库里的冻结标记必须原样保留。
	if !user.IsFrozen() {
		t.Fatal("资料不全时不应解除冻结")
	}
}

// namedTokenManager 返回可辨识的具名令牌，用于验证「确实换发了新令牌」。
//
// issue86TokenManager 的 GenerateTokens 返回空串（既有测试不关心令牌内容），
// 拿它断言「换了令牌」会始终失败——那是桩的局限，不是实现的缺陷。
type namedTokenManager struct {
	issue86TokenManager
	fullCalls int
}

func (m *namedTokenManager) GenerateTokens(_ context.Context, _ int, _, _ string) (string, string, error) {
	m.fullCalls++
	return "full-access", "full-refresh", nil
}

// TestSubmitUnfreeze_SucceedsWhenProfileCompleted 补齐后解冻并换发完整令牌。
func TestSubmitUnfreeze_SucceedsWhenProfileCompleted(t *testing.T) {
	user := frozenIncompleteUser()
	repo := issue86UserRepoWith(user)
	tokens := &namedTokenManager{}
	uc := NewAuthUseCase(repo, tokens, issue86CodeManager{}, issue86AdminDirectory{}, nil)

	// 模拟用户已经把班级补齐。
	user.Detail.Class = "软工230301"

	resp, err := uc.SubmitUnfreeze(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("资料已补齐应解冻成功，实际 %v", err)
	}
	if resp.Frozen {
		t.Fatal("解冻后不应仍标记冻结")
	}
	if resp.Scope != utils.ScopeFull {
		t.Fatalf("解冻后应换发完整权限令牌，实际 scope=%q", resp.Scope)
	}
	if resp.AccessToken != "full-access" || resp.RefreshToken != "full-refresh" {
		t.Fatalf("解冻后应换发完整令牌，实际 access=%q refresh=%q", resp.AccessToken, resp.RefreshToken)
	}
	if tokens.fullCalls != 1 {
		t.Fatalf("应恰好签发一次完整令牌，实际 %d 次", tokens.fullCalls)
	}
	if user.IsFrozen() {
		t.Fatal("解冻后库里的冻结标记应已清除")
	}
}

// TestSubmitUnfreeze_NotFrozenIsError 没冻结就提交解冻 → 明确错误而不是静默成功。
func TestSubmitUnfreeze_NotFrozenIsError(t *testing.T) {
	user := frozenUser()
	user.FrozenAt = nil // 未冻结
	repo := issue86UserRepoWith(user)
	uc := NewAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{}, nil)

	if _, err := uc.SubmitUnfreeze(context.Background(), user.ID); !errors.Is(err, ErrNotFrozenForSelf) {
		t.Fatalf("未冻结账号提交解冻应返回 ErrNotFrozenForSelf，实际 %v", err)
	}
}

// TestSubmitUnfreeze_ReadsFreshStateFromRepo 判定读库而不是信任调用方。
//
// 用户刚保存的资料才是判定依据；若拿旧快照判定，会出现
// 「刚补齐却说我还不合规」。
func TestSubmitUnfreeze_ReadsFreshStateFromRepo(t *testing.T) {
	user := frozenIncompleteUser()
	repo := issue86UserRepoWith(user)
	uc := NewAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{}, nil)

	// 先合一次：应被拒。
	if _, err := uc.SubmitUnfreeze(context.Background(), user.ID); !errors.Is(err, ErrProfileIncomplete) {
		t.Fatalf("首次应被拒，实际 %v", err)
	}
	// 库里更新资料后再合：应通过。
	user.Detail.Class = "软工230301"
	if _, err := uc.SubmitUnfreeze(context.Background(), user.ID); err != nil {
		t.Fatalf("补齐后应通过，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 令牌范围（scope）语义
 * ------------------------------------------------------------------ */

func TestTokenScope_NormalizeBackCompat(t *testing.T) {
	// 旧令牌没有 scope claim：必须归一到 full，否则加字段会让所有人掉线。
	if got := utils.NormalizeScope(""); got != utils.ScopeFull {
		t.Fatalf("空 scope 应归一为 full，实际 %q", got)
	}
	if got := utils.NormalizeScope(utils.ScopeProfileOnly); got != utils.ScopeProfileOnly {
		t.Fatalf("受限 scope 应原样保留，实际 %q", got)
	}
}

func TestTokenScope_RejectsUnknownValue(t *testing.T) {
	// 未知 scope 视为非法令牌，而不是猜测它想表达什么。
	if utils.IsValidScope("root") {
		t.Fatal("未知 scope 应判为非法")
	}
	if !utils.IsValidScope("") || !utils.IsValidScope(utils.ScopeFull) || !utils.IsValidScope(utils.ScopeProfileOnly) {
		t.Fatal("合法的三个取值都应通过")
	}
}
