package service

import (
	"context"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
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
