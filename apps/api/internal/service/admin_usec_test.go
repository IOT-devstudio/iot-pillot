package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * 桩实现：AdminUseCase 只依赖接口，因此不需要 DB 与 Redis
 * ------------------------------------------------------------------ */

type stubUserRepo struct {
	byID   map[int]*domain.User
	byName map[string]*domain.User
	all    []*domain.User
	err    error
}

func (s *stubUserRepo) GetByID(_ context.Context, id int) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	user, ok := s.byID[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *stubUserRepo) GetByName(_ context.Context, name string) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	user, ok := s.byName[name]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *stubUserRepo) Update(_ context.Context, _ *domain.User) error { return nil }

func (s *stubUserRepo) SelectUserByNameAndPassword(_ context.Context, _ string, _ string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (s *stubUserRepo) Save(_ context.Context, _ *domain.User) error { return nil }

func (s *stubUserRepo) GetAll(_ context.Context, _ int, _ int) ([]*domain.User, int64, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.all, int64(len(s.all)), nil
}

type stubAdminDirectory struct {
	ids map[int]bool
	err error
}

func (s *stubAdminDirectory) IsAdmin(_ context.Context, userID int) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.ids[userID], nil
}

func (s *stubAdminDirectory) AddAdmin(_ context.Context, userID int) error {
	if s.err != nil {
		return s.err
	}
	s.ids[userID] = true
	return nil
}

func (s *stubAdminDirectory) RemoveAdmin(_ context.Context, userID int) error {
	if s.err != nil {
		return s.err
	}
	delete(s.ids, userID)
	return nil
}

func (s *stubAdminDirectory) ListAdmins(_ context.Context) ([]int, error) {
	if s.err != nil {
		return nil, s.err
	}
	ids := make([]int, 0, len(s.ids))
	for id, isAdmin := range s.ids {
		if isAdmin {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

type stubSessionRevoker struct {
	revoked []int
	err     error
}

func (s *stubSessionRevoker) RevokeUserSessions(_ context.Context, userID int) error {
	if s.err != nil {
		return s.err
	}
	s.revoked = append(s.revoked, userID)
	return nil
}

func sampleUsers() *stubUserRepo {
	created := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	alice := &domain.User{ID: 1, Name: "alice", CreatedAt: created}
	bob := &domain.User{ID: 2, Name: "bob", CreatedAt: created}
	return &stubUserRepo{
		byID:   map[int]*domain.User{1: alice, 2: bob},
		byName: map[string]*domain.User{"alice": alice, "bob": bob},
		all:    []*domain.User{alice, bob},
	}
}

func newTestAdminUseCase(repo *stubUserRepo, admins *stubAdminDirectory, sessions *stubSessionRevoker) *AdminUseCase {
	return NewAdminUseCase(repo, admins, sessions)
}

/* ------------------------------------------------------------------ *
 * 用户列表
 * ------------------------------------------------------------------ */

func TestListUsers_MarksAdminsFromDirectory(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{1: true}}
	useCase := newTestAdminUseCase(repo, directory, &stubSessionRevoker{})

	result, err := useCase.ListUsers(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("ListUsers 失败: %v", err)
	}

	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("期望 2 个用户，实际 total=%d items=%d", result.Total, len(result.Items))
	}
	if !result.Items[0].IsAdmin {
		t.Error("alice 在名单里，IsAdmin 应为 true")
	}
	if result.Items[1].IsAdmin {
		t.Error("bob 不在名单里，IsAdmin 应为 false")
	}
	if result.Items[0].CreatedAt == "" {
		t.Error("CreatedAt 不应为空")
	}
}

// 名单读不到时不能退化成"都不是管理员"了事：那会让管理端把真实管理员
// 显示成普通成员，撤销按钮也就点错了对象。必须把错误透出去。
func TestListUsers_PropagatesDirectoryError(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{}, err: errors.New("redis down")}
	useCase := newTestAdminUseCase(repo, directory, &stubSessionRevoker{})

	if _, err := useCase.ListUsers(context.Background(), 1, 20); err == nil {
		t.Fatal("名单读失败时 ListUsers 应当报错，而不是返回一份 is_admin 全 false 的列表")
	}
}

/* ------------------------------------------------------------------ *
 * 管理员名单
 * ------------------------------------------------------------------ */

func TestListAdmins_SkipsDeletedUsers(t *testing.T) {
	repo := sampleUsers()
	// 99 号用户已经被删除，但名单里还留着
	directory := &stubAdminDirectory{ids: map[int]bool{1: true, 99: true}}
	useCase := newTestAdminUseCase(repo, directory, &stubSessionRevoker{})

	result, err := useCase.ListAdmins(context.Background())
	if err != nil {
		t.Fatalf("ListAdmins 失败: %v", err)
	}

	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("期望只列出 1 个仍然存在的管理员，实际 %d 个", len(result.Items))
	}
	if result.Items[0].Name != "alice" {
		t.Errorf("列出的应当是 alice，实际 %q", result.Items[0].Name)
	}
	if !result.Items[0].IsAdmin {
		t.Error("名单来源本身，IsAdmin 应为 true")
	}
}

/* ------------------------------------------------------------------ *
 * 角色变更
 * ------------------------------------------------------------------ */

func TestGrantAdmin_AddsAndRevokesSessions(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{}}
	sessions := &stubSessionRevoker{}
	useCase := newTestAdminUseCase(repo, directory, sessions)

	result, err := useCase.GrantAdmin(context.Background(), 2)
	if err != nil {
		t.Fatalf("GrantAdmin 失败: %v", err)
	}

	if !directory.ids[2] {
		t.Error("提升后名单里应当有 userID=2")
	}
	if !result.IsAdmin || !result.SessionRevoked {
		t.Errorf("返回结果有误: %+v", result)
	}
	// 旧令牌的 role claim 已经过时，必须踢下线强制重新登录
	if len(sessions.revoked) != 1 || sessions.revoked[0] != 2 {
		t.Errorf("应当撤销 userID=2 的会话，实际 %v", sessions.revoked)
	}
}

func TestGrantAdmin_RejectsUnknownUser(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{}}
	useCase := newTestAdminUseCase(repo, directory, &stubSessionRevoker{})

	if _, err := useCase.GrantAdmin(context.Background(), 404); err == nil {
		t.Fatal("提升不存在的用户应当报错")
	}
	if len(directory.ids) != 0 {
		t.Error("提升失败时不该往名单里写任何东西")
	}
}

func TestRevokeAdmin_RemovesAndRevokesSessions(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{1: true}}
	sessions := &stubSessionRevoker{}
	useCase := newTestAdminUseCase(repo, directory, sessions)

	result, err := useCase.RevokeAdmin(context.Background(), 1)
	if err != nil {
		t.Fatalf("RevokeAdmin 失败: %v", err)
	}

	if directory.ids[1] {
		t.Error("撤销后名单里不该还有 userID=1")
	}
	if result.IsAdmin {
		t.Error("撤销后 IsAdmin 应为 false")
	}
	if len(sessions.revoked) != 1 {
		t.Errorf("应当撤销会话，实际 %v", sessions.revoked)
	}
}

// 撤销时踢下线失败不该让整个操作失败：名单已经写成功了，
// 真实权限已经收回，只是旧令牌要等到过期才失效。
func TestRevokeAdmin_ToleratesSessionRevokeFailure(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{1: true}}
	sessions := &stubSessionRevoker{err: errors.New("redis timeout")}
	useCase := newTestAdminUseCase(repo, directory, sessions)

	result, err := useCase.RevokeAdmin(context.Background(), 1)
	if err != nil {
		t.Fatalf("名单写成功时不该因为踢下线失败而报错: %v", err)
	}

	if directory.ids[1] {
		t.Error("撤销必须生效")
	}
	if result.SessionRevoked {
		t.Error("踢下线失败时 SessionRevoked 应为 false，便于调用方提示")
	}
}

/* ------------------------------------------------------------------ *
 * 启动引导
 * ------------------------------------------------------------------ */

func TestSeedAdmins_AddsExistingUsersAndSkipsUnknown(t *testing.T) {
	repo := sampleUsers()
	directory := &stubAdminDirectory{ids: map[int]bool{}}
	useCase := newTestAdminUseCase(repo, directory, &stubSessionRevoker{})

	// 名单里混了一个还没注册的名字：只跳过，不该让启动失败
	err := useCase.SeedAdmins(context.Background(), []string{"alice", "not-registered-yet"})
	if err != nil {
		t.Fatalf("引导不该因为名字不存在而失败: %v", err)
	}

	if !directory.ids[1] {
		t.Error("alice 应当被补种为管理员")
	}
	if len(directory.ids) != 1 {
		t.Errorf("只应补种 1 个账号，实际 %d 个", len(directory.ids))
	}
}

func TestSeedAdmins_EmptyListIsNoop(t *testing.T) {
	directory := &stubAdminDirectory{ids: map[int]bool{}}
	useCase := newTestAdminUseCase(sampleUsers(), directory, &stubSessionRevoker{})

	if err := useCase.SeedAdmins(context.Background(), nil); err != nil {
		t.Fatalf("空名单不该报错: %v", err)
	}
	if len(directory.ids) != 0 {
		t.Error("空名单不该写任何东西")
	}
}

// 引导失败（Redis 报错）必须透出去：这时名单不可信，管理端行为无法预期。
func TestSeedAdmins_PropagatesRedisFailure(t *testing.T) {
	directory := &stubAdminDirectory{ids: map[int]bool{}, err: errors.New("redis down")}
	useCase := newTestAdminUseCase(sampleUsers(), directory, &stubSessionRevoker{})

	if err := useCase.SeedAdmins(context.Background(), []string{"alice"}); err == nil {
		t.Fatal("Redis 报错时引导应当返回错误")
	}
}

// 接口断言：AdminStore 必须满足 AdminDirectory（wire 注入时靠它）。
var _ utils.AdminDirectory = (*utils.AdminStore)(nil)
