package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

/* ------------------------------------------------------------------ *
 * 桩实现：冻结用例只依赖 UserRepo + FreezeMailer
 * ------------------------------------------------------------------ */

// freezeRecorder 记录通知邮件调用，可注入失败。
type freezeRecorder struct {
	sent    []*domain.User
	reasons []string
	err     error
}

func (m *freezeRecorder) SendAccountFreezeNotice(_ context.Context, user *domain.User, reason string) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, user)
	m.reasons = append(m.reasons, reason)
	return nil
}

// 会话撤销复用 admin_usec_test.go 的 recordingRevoker（同包、同契约）。

// stubAdminDirectory 可配置的管理员名单桩（hotfix：排除/自救管理员）。
type stubAdminDirectory struct {
	ids []int
	err error
}

func (s *stubAdminDirectory) IsAdmin(_ context.Context, userID int) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	for _, id := range s.ids {
		if id == userID {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubAdminDirectory) AddAdmin(_ context.Context, _ int) error    { return nil }
func (s *stubAdminDirectory) RemoveAdmin(_ context.Context, _ int) error { return nil }

func (s *stubAdminDirectory) ListAdmins(_ context.Context) ([]int, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.ids, nil
}

func (s *stubAdminDirectory) IsFounding(_ context.Context, _ int) (bool, error) { return false, nil }
func (s *stubAdminDirectory) AddFounding(_ context.Context, _ int) error        { return nil }
func (s *stubAdminDirectory) RemoveFounding(_ context.Context, _ int) error     { return nil }

// noAdmins 空管理员名单：普通冻结用例的默认值。
func noAdmins() *stubAdminDirectory { return &stubAdminDirectory{} }

// freezeUserRepo 只实现冻结用例用到的方法，其余按需 panic/空实现。
//
// 用真实的 issue86UserRepo 作为底座（它在 auth_usec_test.go 里维护了
// 内存版冻结语义），这样两个测试文件共用同一份内存实现，
// 不会出现「一个文件里的桩说能冻结、另一个说不能」。
type freezeFixture struct {
	repo  *issue86UserRepo
	now   time.Time
	users []*domain.User
}

func newFreezeFixture(users ...*domain.User) *freezeFixture {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	repo := &issue86UserRepo{
		byID: map[int]*domain.User{},
		all:  users,
	}
	for _, user := range users {
		repo.byID[user.ID] = user
	}
	return &freezeFixture{repo: repo, now: now, users: users}
}

// user 构造一个注册于 daysAgo 天前的账号。
func user(id int, daysAgo int, name, class string, studentID int) *domain.User {
	return &domain.User{
		ID:        id,
		Name:      name,
		Detail:    domain.Detail{Class: class, StudentID: studentID, Email: "u@example.com"},
		CreatedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC).AddDate(0, 0, -daysAgo),
	}
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

func TestFreezeRunOnce_FreezesIncompleteAfterGracePeriod(t *testing.T) {
	incomplete := user(2, 8, "张3", "软工 2 班", 20231145)
	compliant := user(3, 8, "张三", "软工230301", 202311451)
	// 注册不足 7 天：即使不合规也不能冻结（还在宽限期）
	within := user(4, 3, "", "", 0)
	// 已注销：不参与
	deleted := user(5, 8, "张3", "", 0)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	deleted.DeletedAt = &now

	fixture := newFreezeFixture(incomplete, compliant, within, deleted)
	mailer := &freezeRecorder{}
	revoker := &recordingRevoker{}
	uc := NewFreezeUseCase(fixture.repo, mailer, revoker, noAdmins())
	uc.now = func() time.Time { return fixture.now }

	result, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}

	if result.Candidates != 2 {
		t.Fatalf("候选应为 2（incomplete + compliant），实际 %d", result.Candidates)
	}
	if result.Compliant != 1 {
		t.Fatalf("合规应为 1，实际 %d", result.Compliant)
	}
	if result.Frozen != 1 || result.MailFailed != 0 {
		t.Fatalf("应冻结 1 个且邮件全成功，实际 %+v", result)
	}
	if !incomplete.IsFrozen() {
		t.Fatal("资料不合规账号应被冻结")
	}
	if incomplete.FreezeReason == "" {
		t.Fatal("冻结原因不应为空")
	}
	if compliant.IsFrozen() {
		t.Fatal("资料合规账号不应被冻结")
	}
	if within.IsFrozen() {
		t.Fatal("宽限期内账号不应被冻结")
	}
	if deleted.IsFrozen() {
		t.Fatal("已注销账号不应被冻结")
	}
	if len(mailer.sent) != 1 || mailer.sent[0].ID != incomplete.ID {
		t.Fatalf("应只给不合规账号发一封通知，实际 %d 封", len(mailer.sent))
	}
	if incomplete.FreezeNotifiedAt == nil {
		t.Fatal("通知时间应被记录")
	}
	// 冻结后必须撤销会话：否则冻结前签发的 access token 在过期前仍能调接口。
	if len(revoker.revoked) != 1 || revoker.revoked[0] != incomplete.ID {
		t.Fatalf("应只撤销被冻结账号的会话，实际 %v", revoker.revoked)
	}
	// 合规账号与宽限期内账号不该被踢
	for _, id := range revoker.revoked {
		if id == compliant.ID || id == within.ID {
			t.Fatalf("不该撤销 userID=%d 的会话", id)
		}
	}
}

func TestFreezeRunOnce_MailFailureSkipsFreeze(t *testing.T) {
	target := user(2, 8, "张3", "", 0)
	fixture := newFreezeFixture(target)
	mailer := &freezeRecorder{err: errors.New("smtp down")}
	uc := NewFreezeUseCase(fixture.repo, mailer, &recordingRevoker{}, noAdmins())
	uc.now = func() time.Time { return fixture.now }

	result, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}

	// 先发通知后冻结：发信失败时本轮必须跳过冻结，留给下一轮重试。
	if result.MailFailed != 1 || result.Frozen != 0 {
		t.Fatalf("邮件失败时应跳过冻结，实际 %+v", result)
	}
	if target.IsFrozen() {
		t.Fatal("邮件失败的账号不应被冻结")
	}
	if target.FreezeNotifiedAt != nil {
		t.Fatal("邮件失败不应记录通知时间")
	}
}

func TestFreezeRunOnce_IdempotentAcrossRuns(t *testing.T) {
	target := user(2, 8, "张3", "", 0)
	fixture := newFreezeFixture(target)
	mailer := &freezeRecorder{}
	uc := NewFreezeUseCase(fixture.repo, mailer, &recordingRevoker{}, noAdmins())
	uc.now = func() time.Time { return fixture.now }

	first, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("第一轮失败: %v", err)
	}
	if first.Frozen != 1 {
		t.Fatalf("第一轮应冻结 1 个，实际 %+v", first)
	}

	second, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("第二轮失败: %v", err)
	}
	if second.Frozen != 0 {
		t.Fatalf("第二轮不应再冻结，实际 %+v", second)
	}
	if second.Candidates != 0 {
		t.Fatalf("第二轮候选应为 0（已冻结行被排除），实际 %d", second.Candidates)
	}
	// 关键：整个流程只发一封通知，第二次扫描不能重复发信。
	if len(mailer.sent) != 1 {
		t.Fatalf("重复扫描不应重复发信，实际发了 %d 封", len(mailer.sent))
	}
}

func TestFreezeRunOnce_DoesNotResendWhenAlreadyNotified(t *testing.T) {
	// 模拟「上一轮邮件已发、冻结写库失败」：通知时间已记录但未冻结。
	target := user(2, 8, "张3", "", 0)
	notified := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	target.FreezeNotifiedAt = &notified

	fixture := newFreezeFixture(target)
	mailer := &freezeRecorder{}
	uc := NewFreezeUseCase(fixture.repo, mailer, &recordingRevoker{}, noAdmins())
	uc.now = func() time.Time { return fixture.now }

	result, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}

	if len(mailer.sent) != 0 {
		t.Fatalf("已通知过的不应重发，实际发了 %d 封", len(mailer.sent))
	}
	if result.Frozen != 1 {
		t.Fatalf("已通知但未冻结的账号本轮应补冻结，实际 %+v", result)
	}
	if !target.IsFrozen() {
		t.Fatal("应补上冻结")
	}
}

func TestFreezeRunOnce_RepoErrorPropagates(t *testing.T) {
	repo := &failingFreezeRepo{err: errors.New("db down")}
	uc := NewFreezeUseCase(repo, &freezeRecorder{}, &recordingRevoker{}, noAdmins())
	if _, err := uc.RunOnce(context.Background()); err == nil {
		t.Fatal("仓储错误应外传，由调度器记日志")
	}
}

// failingFreezeRepo 只实现 ListFreezeCandidates 的失败路径。
type failingFreezeRepo struct {
	issue86UserRepo
	err error
}

func (r *failingFreezeRepo) ListFreezeCandidates(_ context.Context, _ time.Time) ([]*domain.User, error) {
	return nil, r.err
}

/* ------------------------------------------------------------------ *
 * 管理员例外（hotfix）：不自动冻结 + 自救已冻结管理员
 * ------------------------------------------------------------------ */

func TestFreezeRunOnce_SkipsAdmins(t *testing.T) {
	// 管理员资料即使明显不合规，也不能被自动冻结——冻结会连解冻入口一起封死。
	admin := user(7, 10, "管理员", "软工 2 班", 20231145)
	normal := user(8, 10, "张3", "软工 2 班", 20231145)

	fixture := newFreezeFixture(admin, normal)
	mailer := &freezeRecorder{}
	uc := NewFreezeUseCase(fixture.repo, mailer, &recordingRevoker{}, &stubAdminDirectory{ids: []int{admin.ID}})
	uc.now = func() time.Time { return fixture.now }

	result, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}

	if admin.IsFrozen() {
		t.Fatal("管理员不应被自动冻结")
	}
	if !normal.IsFrozen() {
		t.Fatal("普通用户仍应正常冻结")
	}
	if result.SkippedAdmins != 1 {
		t.Fatalf("应记录跳过 1 个管理员，实际 %+v", result)
	}
	// 管理员不该收到冻结通知邮件（他并没有被冻结）
	for _, sent := range mailer.sent {
		if sent.ID == admin.ID {
			t.Fatal("管理员不应收到冻结通知")
		}
	}
}

func TestFreezeRunOnce_RescuesFrozenAdmins(t *testing.T) {
	// 复现事故现场：唯一最高权限管理员已被冻结，解冻接口无人可调。
	admin := user(7, 10, "管理员", "软工 2 班", 20231145)
	frozenAt := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	admin.FrozenAt = &frozenAt
	admin.FreezeReason = "班级须为「汉字 + 6 位数字」"
	deletedAdmin := user(9, 10, "已注销管理员", "", 0)
	deletedAdmin.FrozenAt = &frozenAt
	deletedAdmin.FreezeReason = "注销前的旧原因"
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	deletedAdmin.DeletedAt = &now

	fixture := newFreezeFixture(admin, deletedAdmin)
	uc := NewFreezeUseCase(
		fixture.repo,
		&freezeRecorder{},
		&recordingRevoker{},
		&stubAdminDirectory{ids: []int{admin.ID, deletedAdmin.ID}},
	)
	uc.now = func() time.Time { return fixture.now }

	result, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}

	if admin.IsFrozen() {
		t.Fatal("被冻结的管理员应在本轮被解冻")
	}
	if admin.FreezeReason != "" || admin.FreezeNotifiedAt != nil {
		t.Fatalf("自救应清空冻结三列: %+v", admin)
	}
	if result.RescuedAdmins != 1 {
		t.Fatalf("应解冻 1 个管理员，实际 %+v", result)
	}
	// 已注销行不参与自救（登录面已封死，解冻没有意义）
	if deletedAdmin.FreezeReason != "注销前的旧原因" || deletedAdmin.FrozenAt == nil {
		t.Fatal("已注销账号不应被改动")
	}
}

func TestFreezeRunOnce_DoesNotRescueNormalUsers(t *testing.T) {
	// 普通用户的冻结是正常处置，不能被管理员自救逻辑顺手放开。
	normal := user(8, 10, "张3", "软工 2 班", 20231145)
	frozenAt := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	normal.FrozenAt = &frozenAt

	fixture := newFreezeFixture(normal)
	uc := NewFreezeUseCase(fixture.repo, &freezeRecorder{}, &recordingRevoker{}, noAdmins())
	uc.now = func() time.Time { return fixture.now }

	result, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}
	if !normal.IsFrozen() {
		t.Fatal("普通用户的冻结不应被解开放")
	}
	if result.RescuedAdmins != 0 {
		t.Fatalf("没有管理员被解冻，实际 %+v", result)
	}
}

func TestFreezeRunOnce_AdminListErrorAbortsRound(t *testing.T) {
	// 读不到管理员名单时宁可整轮不冻结：在不知道谁是管理员的情况下批量封号，
	// 正是把最高权限账号锁死的路径。
	target := user(8, 10, "张3", "", 0)
	fixture := newFreezeFixture(target)
	uc := NewFreezeUseCase(
		fixture.repo,
		&freezeRecorder{},
		&recordingRevoker{},
		&stubAdminDirectory{err: errors.New("redis down")},
	)
	uc.now = func() time.Time { return fixture.now }

	if _, err := uc.RunOnce(context.Background()); err == nil {
		t.Fatal("管理员名单读取失败应中止本轮")
	}
	if target.IsFrozen() {
		t.Fatal("名单不可用时不应冻结任何账号")
	}
}
