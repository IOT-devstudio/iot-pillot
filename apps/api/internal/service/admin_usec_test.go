package service

import (
	"context"
	"errors"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// ----------------------------------------------------------------------
// 桩实现（issue #120 创始管理员标记）
// ----------------------------------------------------------------------

// foundingStubAdminDirectory 内存版 AdminDirectory，记录写入轨迹。
//
// 只覆盖创始标记相关测试需要的路径：两个 SET 的成员记录 + 调用计数，
// 用来断言「种入写两个集合」「晋升不写创始集合」「撤销清理创始标记」。
type foundingStubAdminDirectory struct {
	admins              map[int]bool
	founding            map[int]bool
	addFoundingCalls    []int
	removeFoundingCalls []int
}

func newFoundingStubAdminDirectory() *foundingStubAdminDirectory {
	return &foundingStubAdminDirectory{
		admins:   make(map[int]bool),
		founding: make(map[int]bool),
	}
}

func (s *foundingStubAdminDirectory) IsAdmin(_ context.Context, userID int) (bool, error) {
	return s.admins[userID], nil
}

func (s *foundingStubAdminDirectory) AddAdmin(_ context.Context, userID int) error {
	s.admins[userID] = true
	return nil
}

func (s *foundingStubAdminDirectory) RemoveAdmin(_ context.Context, userID int) error {
	delete(s.admins, userID)
	return nil
}

func (s *foundingStubAdminDirectory) ListAdmins(_ context.Context) ([]int, error) {
	ids := make([]int, 0, len(s.admins))
	for id := range s.admins {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *foundingStubAdminDirectory) IsFounding(_ context.Context, userID int) (bool, error) {
	return s.founding[userID], nil
}

func (s *foundingStubAdminDirectory) AddFounding(_ context.Context, userID int) error {
	s.founding[userID] = true
	s.addFoundingCalls = append(s.addFoundingCalls, userID)
	return nil
}

func (s *foundingStubAdminDirectory) RemoveFounding(_ context.Context, userID int) error {
	delete(s.founding, userID)
	s.removeFoundingCalls = append(s.removeFoundingCalls, userID)
	return nil
}

// foundingStubUserRepo 按名字/ID 返回固定用户，供 SeedAdmins 解析引导项。
type foundingStubUserRepo struct {
	issue86UserRepo
	users map[int]*domain.User
}

func (r *foundingStubUserRepo) GetByID(_ context.Context, id int) (*domain.User, error) {
	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return nil, nil
}

func (r *foundingStubUserRepo) GetByName(_ context.Context, name string) (*domain.User, error) {
	for _, user := range r.users {
		if user.Name == name {
			return user, nil
		}
	}
	return nil, nil
}

// noRevoker 空实现 SessionRevoker（创始标记测试不关心踢会话）。
type noRevoker struct{}

func (noRevoker) RevokeUserSessions(_ context.Context, _ int) error { return nil }

var _ utils.AdminDirectory = (*foundingStubAdminDirectory)(nil)

// ----------------------------------------------------------------------
// 种入（issue #120 验收：种入写两个集合）
// ----------------------------------------------------------------------

// 种入把名单同时写进 auth:admins 与创始集合，重启补种幂等。
func TestSeedAdmins_WritesBothSets(t *testing.T) {
	repo := &foundingStubUserRepo{users: map[int]*domain.User{
		1: {ID: 1, Name: "root"},
		3: {ID: 3, Name: "alice"},
	}}
	directory := newFoundingStubAdminDirectory()
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: noRevoker{}}

	if err := useCase.SeedAdmins(context.Background(), []string{"root", "3"}); err != nil {
		t.Fatalf("SeedAdmins() error = %v", err)
	}

	for _, id := range []int{1, 3} {
		if !directory.admins[id] {
			t.Errorf("userID=%d 未写入管理员名单", id)
		}
		if !directory.founding[id] {
			t.Errorf("userID=%d 未写入创始集合", id)
		}
	}
}

// 引导名单里解析不到的账号只跳过，不阻断其余补种（与现状一致）。
func TestSeedAdmins_SkipsUnresolvableEntries(t *testing.T) {
	repo := &foundingStubUserRepo{users: map[int]*domain.User{
		1: {ID: 1, Name: "root"},
	}}
	directory := newFoundingStubAdminDirectory()
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: noRevoker{}}

	if err := useCase.SeedAdmins(context.Background(), []string{"root", "ghost"}); err != nil {
		t.Fatalf("SeedAdmins() error = %v", err)
	}

	if !directory.admins[1] || !directory.founding[1] {
		t.Error("可解析的引导项 root 应被补种进两个集合")
	}
	if len(directory.admins) != 1 || len(directory.founding) != 1 {
		t.Errorf("不应有额外成员被写入：admins=%v founding=%v", directory.admins, directory.founding)
	}
}

// ----------------------------------------------------------------------
// 晋升 / 撤销（issue #120 Q1 裁决：晋升不写创始集合，撤销同步清理）
// ----------------------------------------------------------------------

// 晋升只写主名单，不写创始集合。
func TestGrantAdmin_DoesNotTouchFounding(t *testing.T) {
	repo := &foundingStubUserRepo{users: map[int]*domain.User{
		5: {ID: 5, Name: "bob"},
	}}
	directory := newFoundingStubAdminDirectory()
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: noRevoker{}}

	if _, err := useCase.GrantAdmin(context.Background(), 5); err != nil {
		t.Fatalf("GrantAdmin() error = %v", err)
	}

	if !directory.admins[5] {
		t.Error("晋升后应进入管理员名单")
	}
	if directory.founding[5] {
		t.Error("晋升不得写创始集合")
	}
	if len(directory.addFoundingCalls) != 0 {
		t.Errorf("晋升不应调用 AddFounding，实际调用 %v", directory.addFoundingCalls)
	}
}

// 撤销同步移出创始集合；重新晋升后 is_founding 回到 false（Q1 裁决验收）。
func TestRevokeAdmin_CleansFounding(t *testing.T) {
	repo := &foundingStubUserRepo{users: map[int]*domain.User{
		1: {ID: 1, Name: "root"},
	}}
	directory := newFoundingStubAdminDirectory()
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: noRevoker{}}

	// 模拟种入后的状态
	if err := useCase.SeedAdmins(context.Background(), []string{"root"}); err != nil {
		t.Fatalf("SeedAdmins() error = %v", err)
	}

	if _, err := useCase.RevokeAdmin(context.Background(), 1); err != nil {
		t.Fatalf("RevokeAdmin() error = %v", err)
	}

	if directory.admins[1] {
		t.Error("撤销后应移出管理员名单")
	}
	if directory.founding[1] {
		t.Error("撤销后应移出创始集合")
	}
	if len(directory.removeFoundingCalls) == 0 || directory.removeFoundingCalls[0] != 1 {
		t.Errorf("撤销应调用 RemoveFounding(1)，实际 %v", directory.removeFoundingCalls)
	}

	// 重新晋升：应是普通管理员，不再带创始标记
	if _, err := useCase.GrantAdmin(context.Background(), 1); err != nil {
		t.Fatalf("GrantAdmin() error = %v", err)
	}
	if directory.admins[1] != true {
		t.Error("重新晋升后应回到管理员名单")
	}
	if directory.founding[1] {
		t.Error("重新晋升后 is_founding 必须回到 false（不残留创始特权）")
	}
}

// ----------------------------------------------------------------------
// FoundingOf（/me 三种身份取值）
// ----------------------------------------------------------------------

// FoundingOf：创始 = true；普通管理员与成员 = false。
func TestFoundingOf_ThreeIdentities(t *testing.T) {
	directory := newFoundingStubAdminDirectory()
	directory.admins[1] = true // 晋升管理员（无创始标记）
	directory.admins[2] = true
	directory.founding[2] = true // 创始管理员
	// userID 3：普通成员，两个集合都不在

	auc := newTestAuthUseCase(&foundingStubUserRepo{}, issue86TokenManager{}, issue86CodeManager{}, directory)

	cases := []struct {
		userID int
		want   bool
	}{
		{1, false}, // 晋升管理员
		{2, true},  // 创始管理员
		{3, false}, // 普通成员
	}

	for _, tc := range cases {
		got, err := auc.FoundingOf(context.Background(), tc.userID)
		if err != nil {
			t.Fatalf("FoundingOf(%d) error = %v", tc.userID, err)
		}
		if got != tc.want {
			t.Errorf("FoundingOf(%d) = %v, want %v", tc.userID, got, tc.want)
		}
	}
}

/* ------------------------------------------------------------------ *
 * 账号注销（issue #142）
 * ------------------------------------------------------------------ */

// recordingRevoker 记录被踢会话的 userID，供注销测试断言。
type recordingRevoker struct {
	revoked []int
	err     error
}

func (r *recordingRevoker) RevokeUserSessions(_ context.Context, userID int) error {
	if r.err != nil {
		return r.err
	}
	r.revoked = append(r.revoked, userID)
	return nil
}

// recordingMailer 记录注销通知邮件调用（issue #159 Q3/Q4）。
type recordingMailer struct {
	notices []int // 收到通知的 userID
	err     error // 注入发送失败，验证「失败不阻断注销」
}

func (m *recordingMailer) SendAccountDeletionNotice(_ context.Context, _ int, user *domain.User) error {
	if m.err != nil {
		return m.err
	}
	m.notices = append(m.notices, user.ID)
	return nil
}

// 注销成功：软删落库 + 移出两个管理员集合 + 踢掉全部会话。
func TestDeleteUser_Success(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 7, Name: "victim"})
	directory := newFoundingStubAdminDirectory()
	directory.admins[7] = true // 目标是晋升管理员
	revoker := &recordingRevoker{}
	mailer := &recordingMailer{}
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: revoker, mailer: mailer}

	if err := useCase.DeleteUser(context.Background(), 1 /* operator 创始 */, 7); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	if repo.byID[7].DeletedAt == nil {
		t.Error("目标账号应被软删（deleted_at 置位）")
	}
	if directory.admins[7] {
		t.Error("应移出 auth:admins 集合")
	}
	if directory.founding[7] {
		t.Error("应移出创始集合（幂等清理）")
	}
	if len(revoker.revoked) != 1 || revoker.revoked[0] != 7 {
		t.Errorf("应踢掉目标会话一次，实际 %v", revoker.revoked)
	}
}

// 守卫：不能注销自己——任何副作用都不该发生。
func TestDeleteUser_RejectSelfDeletion(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 1, Name: "founder"})
	directory := newFoundingStubAdminDirectory()
	revoker := &recordingRevoker{}
	mailer := &recordingMailer{}
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: revoker, mailer: mailer}

	err := useCase.DeleteUser(context.Background(), 1, 1)
	if !errors.Is(err, ErrSelfDeletion) {
		t.Fatalf("自注销应返回 ErrSelfDeletion，实际: %v", err)
	}
	if repo.byID[1].DeletedAt != nil {
		t.Error("守卫失败不应产生任何副作用")
	}
	if len(revoker.revoked) != 0 {
		t.Error("守卫失败不应踢会话")
	}
}

// 守卫：创始管理员账号受保护，互不注销（Q4 裁决）。
func TestDeleteUser_RejectDeletingFoundingMember(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 2, Name: "other-founder"})
	directory := newFoundingStubAdminDirectory()
	directory.founding[2] = true
	revoker := &recordingRevoker{}
	mailer := &recordingMailer{}
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: revoker, mailer: mailer}

	err := useCase.DeleteUser(context.Background(), 1, 2)
	if !errors.Is(err, ErrFoundingProtected) {
		t.Fatalf("注销其他创始管理员应返回 ErrFoundingProtected，实际: %v", err)
	}
	if repo.byID[2].DeletedAt != nil {
		t.Error("守卫失败不应软删")
	}
}

// 重复注销：明确业务错误，无重复副作用（Q4：不重复写）。
func TestDeleteUser_RejectDuplicateDeletion(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 7, Name: "victim"})
	directory := newFoundingStubAdminDirectory()
	revoker := &recordingRevoker{}
	mailer := &recordingMailer{}
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: revoker, mailer: mailer}

	if err := useCase.DeleteUser(context.Background(), 1, 7); err != nil {
		t.Fatalf("首次注销应成功: %v", err)
	}
	err := useCase.DeleteUser(context.Background(), 1, 7)
	if !errors.Is(err, repository.ErrUserAlreadyDeleted) {
		t.Fatalf("重复注销应返回 ErrUserAlreadyDeleted，实际: %v", err)
	}
	if len(revoker.revoked) != 1 {
		t.Errorf("重复注销不应再踢会话，实际 %d 次", len(revoker.revoked))
	}
}

// 目标不存在：返回可判别的用户不存在错误。
func TestDeleteUser_TargetNotFound(t *testing.T) {
	useCase := &AdminUseCase{
		userRepo: issue86UserRepoWith(),
		admins:   newFoundingStubAdminDirectory(),
		sessions: &recordingRevoker{},
		mailer:   &recordingMailer{},
	}

	if err := useCase.DeleteUser(context.Background(), 1, 999); !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("目标不存在应返回 ErrUserNotFound，实际: %v", err)
	}
}

// 正常成员（非管理员集合成员）也可被注销，流程与管理员一致。
func TestDeleteUser_PlainMember(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 9, Name: "plain"})
	directory := newFoundingStubAdminDirectory()
	revoker := &recordingRevoker{}
	mailer := &recordingMailer{}
	useCase := &AdminUseCase{userRepo: repo, admins: directory, sessions: revoker, mailer: mailer}

	if err := useCase.DeleteUser(context.Background(), 1, 9); err != nil {
		t.Fatalf("注销普通成员应成功: %v", err)
	}
	if repo.byID[9].DeletedAt == nil {
		t.Error("普通成员应被软删")
	}
	if len(revoker.revoked) != 1 || revoker.revoked[0] != 9 {
		t.Errorf("应踢掉普通成员会话，实际 %v", revoker.revoked)
	}
}

/* ------------------------------------------------------------------ *
 * 注销通知邮件（issue #159 Q3/Q4）
 * ------------------------------------------------------------------ */

// 管理员发起注销成功 → 给目标发通知邮件（收件人是被注销者，不是操作者）。
func TestDeleteUser_SendsDeletionNotice(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 7, Name: "victim"})
	directory := newFoundingStubAdminDirectory()
	mailer := &recordingMailer{}
	useCase := &AdminUseCase{
		userRepo: repo, admins: directory,
		sessions: &recordingRevoker{}, mailer: mailer,
	}

	if err := useCase.DeleteUser(context.Background(), 1, 7); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if len(mailer.notices) != 1 || mailer.notices[0] != 7 {
		t.Errorf("应给被注销者 userID=7 发通知，实际 %v", mailer.notices)
	}
}

// Q4：邮件发送失败不阻断注销——注销已生效，错误仅记日志。
func TestDeleteUser_MailFailureDoesNotBlockDeletion(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 7, Name: "victim"})
	directory := newFoundingStubAdminDirectory()
	mailer := &recordingMailer{err: errors.New("smtp down")}
	useCase := &AdminUseCase{
		userRepo: repo, admins: directory,
		sessions: &recordingRevoker{}, mailer: mailer,
	}

	if err := useCase.DeleteUser(context.Background(), 1, 7); err != nil {
		t.Fatalf("邮件失败不应阻断注销，实际返回错误: %v", err)
	}
	if repo.byID[7].DeletedAt == nil {
		t.Error("注销应已生效（软删落位）")
	}
}

/* ------------------------------------------------------------------ *
 * 自助注销（issue #159）
 * ------------------------------------------------------------------ */

// 自助注销成功：软删 + 清两个集合 + 踢全部会话；不触发通知邮件（Q3）。
func TestDeleteSelf_Success(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 5, Name: "self"})
	directory := newFoundingStubAdminDirectory()
	directory.admins[5] = true // 自助者恰好是晋升管理员，集合要清
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, directory)

	// 踢会话走 tokenManager（issue86 空实现恒成功）；通知邮件只在管理端
	// DeleteUser 路径（Q3），AuthUseCase 无 mailer 字段——此处只断言软删与集合。
	if err := auc.DeleteSelf(context.Background(), 5); err != nil {
		t.Fatalf("DeleteSelf() error = %v", err)
	}
	if repo.byID[5].DeletedAt == nil {
		t.Error("应软删自己")
	}
	if directory.admins[5] {
		t.Error("应移出 auth:admins（自助者可能是晋升管理员）")
	}
	if directory.founding[5] {
		t.Error("应移出创始集合")
	}
}

// Q1：创始管理员禁止自助注销，无任何副作用。
func TestDeleteSelf_RejectFoundingAdmin(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 1, Name: "founder"})
	directory := newFoundingStubAdminDirectory()
	directory.founding[1] = true
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, directory)

	err := auc.DeleteSelf(context.Background(), 1)
	if !errors.Is(err, ErrFoundingProtected) {
		t.Fatalf("创始管理员自助注销应返回 ErrFoundingProtected，实际: %v", err)
	}
	if repo.byID[1].DeletedAt != nil {
		t.Error("守卫失败不应产生任何副作用")
	}
}

// 重复自助注销（对已注销账号再调）→ 明确业务错误。
func TestDeleteSelf_RejectDuplicate(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 5, Name: "self"})
	directory := newFoundingStubAdminDirectory()
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, directory)

	if err := auc.DeleteSelf(context.Background(), 5); err != nil {
		t.Fatalf("首次注销应成功: %v", err)
	}
	err := auc.DeleteSelf(context.Background(), 5)
	if !errors.Is(err, repository.ErrUserAlreadyDeleted) {
		t.Fatalf("重复注销应返回 ErrUserAlreadyDeleted，实际: %v", err)
	}
}
