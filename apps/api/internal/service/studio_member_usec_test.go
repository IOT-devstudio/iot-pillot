package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 桩
 * ------------------------------------------------------------------ */

// stubStudioMemberRepo 只实现被用到的方法。
//
// 其余方法由嵌入的接口提供、调用即 panic —— 测试一旦越界会立刻炸，
// 而不是悄悄走到一个「返回零值」的假实现上（与 form_usec_test 同一做法）。
type stubStudioMemberRepo struct {
	repository.StudioMemberRepo

	members     map[int]*domain.StudioMember
	nextID      int
	createErr   error
	updateErr   error
	archiveErr  error
	lastFilter  repository.StudioMemberListFilter
	archiveCall int
}

func newStubStudioMemberRepo() *stubStudioMemberRepo {
	return &stubStudioMemberRepo{members: make(map[int]*domain.StudioMember), nextID: 1}
}

func (s *stubStudioMemberRepo) Create(_ context.Context, member *domain.StudioMember) error {
	if s.createErr != nil {
		return s.createErr
	}
	member.ID = s.nextID
	s.nextID++
	member.CreatedAt = time.Unix(0, 0)
	member.UpdatedAt = time.Unix(0, 0)
	stored := *member
	s.members[member.ID] = &stored
	return nil
}

func (s *stubStudioMemberRepo) GetByID(_ context.Context, id int) (*domain.StudioMember, error) {
	if m, ok := s.members[id]; ok {
		return m, nil
	}
	return nil, repository.ErrStudioMemberNotFound
}

func (s *stubStudioMemberRepo) List(_ context.Context, filter repository.StudioMemberListFilter, _ int, _ int) ([]*domain.StudioMember, int64, error) {
	s.lastFilter = filter
	out := make([]*domain.StudioMember, 0, len(s.members))
	for _, m := range s.members {
		if filter.IsActive != nil && m.IsActive != *filter.IsActive {
			continue
		}
		out = append(out, m)
	}
	return out, int64(len(out)), nil
}

func (s *stubStudioMemberRepo) Update(_ context.Context, member *domain.StudioMember) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if _, ok := s.members[member.ID]; !ok {
		return repository.ErrStudioMemberNotFound
	}
	stored := *member
	s.members[member.ID] = &stored
	return nil
}

func (s *stubStudioMemberRepo) Archive(_ context.Context, id int, at time.Time) error {
	s.archiveCall++
	if s.archiveErr != nil {
		return s.archiveErr
	}
	m, ok := s.members[id]
	if !ok {
		return repository.ErrStudioMemberNotFound
	}
	if m.ArchivedAt != nil {
		return repository.ErrStudioMemberArchived
	}
	stamp := at
	m.ArchivedAt = &stamp
	m.IsActive = false
	return nil
}

// stubAccountDirectory 满足 StudioMemberAccountDirectory。
type stubAccountDirectory struct {
	users map[int]*domain.User
}

func (s stubAccountDirectory) GetByID(_ context.Context, id int) (*domain.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

func newStudioMemberUseCase(t *testing.T) (*StudioMemberUseCase, *stubStudioMemberRepo) {
	t.Helper()
	repo := newStubStudioMemberRepo()
	deletedAt := time.Unix(0, 0)
	accounts := stubAccountDirectory{users: map[int]*domain.User{
		7: {ID: 7, Name: "老成员"},
		// 9 是已注销账号：软删行不能挂到成员档案上。
		9: {ID: 9, Name: "已注销", DeletedAt: &deletedAt},
	}}
	uc := NewStudioMemberUseCase(repo, accounts)
	// 固定时钟：归档时间必须可断言，不能依赖真实时间。
	uc.now = func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }
	return uc, repo
}

func validMemberFields() domain.StudioMemberFields {
	return domain.StudioMemberFields{
		Name:         "张三",
		BirthYear:    2003,
		Cohort:       "2024 级",
		Direction:    "front-end",
		RoleInStudio: "前端负责人",
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：无账号成员可创建
 * ------------------------------------------------------------------ */

func TestCreateStudioMember_WithoutUserAccount(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)

	member, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if err != nil {
		t.Fatalf("无账号成员应可创建，实际 %v", err)
	}
	if member.UserID != nil {
		t.Errorf("未传 user_id 时应保持 nil（无系统账号），实际 %v", *member.UserID)
	}
	if !member.IsActive {
		t.Error("新建成员应是在读状态")
	}
}

func TestCreateStudioMember_WithExistingAccount(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	userID := 7

	member, err := uc.Create(context.Background(), CreateStudioMemberInput{
		Fields: validMemberFields(),
		UserID: &userID,
	})
	if err != nil {
		t.Fatalf("关联存在的账号应成功，实际 %v", err)
	}
	if member.UserID == nil || *member.UserID != 7 {
		t.Errorf("应绑定 user_id=7，实际 %v", member.UserID)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：字段校验
 * ------------------------------------------------------------------ */

func TestCreateStudioMember_Validation(t *testing.T) {
	cases := []struct {
		name  string
		patch func(*domain.StudioMemberFields)
	}{
		{"空姓名", func(f *domain.StudioMemberFields) { f.Name = "   " }},
		{"姓名过长", func(f *domain.StudioMemberFields) { f.Name = string(make([]rune, 41)) }},
		{"出生年过小", func(f *domain.StudioMemberFields) { f.BirthYear = 1899 }},
		{"出生年过大", func(f *domain.StudioMemberFields) { f.BirthYear = 2101 }},
		{"空届别", func(f *domain.StudioMemberFields) { f.Cohort = "" }},
		{"空方向", func(f *domain.StudioMemberFields) { f.Direction = "" }},
		{"空职能", func(f *domain.StudioMemberFields) { f.RoleInStudio = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, repo := newStudioMemberUseCase(t)
			fields := validMemberFields()
			tc.patch(&fields)

			if _, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: fields}); !errors.Is(err, ErrStudioMemberInvalid) {
				t.Fatalf("应返回 ErrStudioMemberInvalid，实际 %v", err)
			}
			if len(repo.members) != 0 {
				t.Error("校验失败不应写库")
			}
		})
	}
}

func TestCreateStudioMember_TrimsWhitespace(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	fields := validMemberFields()
	fields.Name = "  张三  "
	fields.RoleInStudio = " 前端负责人 "

	member, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: fields})
	if err != nil {
		t.Fatalf("应成功，实际 %v", err)
	}
	// 不 Trim 会让「张三」与「张三 」绕过 (name, birth_year) 唯一键。
	if member.Name != "张三" {
		t.Errorf("姓名应去首尾空白，实际 %q", member.Name)
	}
	if member.RoleInStudio != "前端负责人" {
		t.Errorf("职能应去首尾空白，实际 %q", member.RoleInStudio)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：同名+出生年冲突有明确业务错误
 * ------------------------------------------------------------------ */

func TestCreateStudioMember_DuplicateNameAndBirthYear(t *testing.T) {
	uc, repo := newStudioMemberUseCase(t)
	repo.createErr = repository.ErrStudioMemberDuplicate

	_, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if !errors.Is(err, repository.ErrStudioMemberDuplicate) {
		t.Fatalf("撞唯一键应返回 ErrStudioMemberDuplicate，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：user_id 必须存在且未注销
 * ------------------------------------------------------------------ */

func TestCreateStudioMember_InvalidUser(t *testing.T) {
	uc, repo := newStudioMemberUseCase(t)

	missing := 404
	if _, err := uc.Create(context.Background(), CreateStudioMemberInput{
		Fields: validMemberFields(),
		UserID: &missing,
	}); !errors.Is(err, ErrStudioMemberInvalidUser) {
		t.Fatalf("不存在的账号应被拒绝，实际 %v", err)
	}

	deleted := 9
	if _, err := uc.Create(context.Background(), CreateStudioMemberInput{
		Fields: validMemberFields(),
		UserID: &deleted,
	}); !errors.Is(err, ErrStudioMemberInvalidUser) {
		t.Fatalf("已注销账号应被拒绝，实际 %v", err)
	}

	if len(repo.members) != 0 {
		t.Error("账号校验失败不应写库")
	}
}

/* ------------------------------------------------------------------ *
 * 更新：补丁语义与唯一键合并
 * ------------------------------------------------------------------ */

func TestUpdateStudioMember_PatchMergesWithStoredValues(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if err != nil {
		t.Fatal(err)
	}

	// 只改出生年：姓名必须来自已存的值，否则唯一键会被算错。
	birthYear := 2004
	updated, err := uc.Update(context.Background(), created.ID, UpdateStudioMemberInput{BirthYear: &birthYear})
	if err != nil {
		t.Fatalf("应成功，实际 %v", err)
	}
	if updated.BirthYear != 2004 {
		t.Errorf("出生年应更新为 2004，实际 %d", updated.BirthYear)
	}
	if updated.Name != "张三" {
		t.Errorf("未传的姓名应保持原值，实际 %q", updated.Name)
	}
}

func TestUpdateStudioMember_RejectsEmptyRequiredField(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if err != nil {
		t.Fatal(err)
	}

	blank := "   "
	if _, err := uc.Update(context.Background(), created.ID, UpdateStudioMemberInput{Name: &blank}); !errors.Is(err, ErrStudioMemberInvalid) {
		t.Fatalf("把姓名改成空应被拒绝，实际 %v", err)
	}
}

func TestUpdateStudioMember_UnbindUser(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	userID := 7
	created, err := uc.Create(context.Background(), CreateStudioMemberInput{
		Fields: validMemberFields(),
		UserID: &userID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 三态：传了 user_id 但值是 null = 解除绑定。
	var unbind *int
	updated, err := uc.Update(context.Background(), created.ID, UpdateStudioMemberInput{UserID: &unbind})
	if err != nil {
		t.Fatalf("解除绑定应成功，实际 %v", err)
	}
	if updated.UserID != nil {
		t.Errorf("应解除绑定，实际 %v", *updated.UserID)
	}
}

func TestUpdateStudioMember_RestoreClearsArchivedAt(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Archive(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}

	active := true
	restored, err := uc.Update(context.Background(), created.ID, UpdateStudioMemberInput{IsActive: &active})
	if err != nil {
		t.Fatalf("恢复在读应成功，实际 %v", err)
	}
	// 「is_active=true 但 archived_at 非空」是自相矛盾的行，必须清掉。
	if restored.ArchivedAt != nil {
		t.Errorf("恢复在读应清空归档时刻，实际 %v", restored.ArchivedAt)
	}
	if !restored.IsActive {
		t.Error("恢复后应是在读状态")
	}
}

func TestUpdateStudioMember_NotFound(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	name := "李四"
	if _, err := uc.Update(context.Background(), 999, UpdateStudioMemberInput{Name: &name}); !errors.Is(err, repository.ErrStudioMemberNotFound) {
		t.Fatalf("不存在的成员应返回 ErrStudioMemberNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 归档
 * ------------------------------------------------------------------ */

func TestArchiveStudioMember(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if err != nil {
		t.Fatal(err)
	}

	archived, err := uc.Archive(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("归档应成功，实际 %v", err)
	}
	if archived.IsActive {
		t.Error("归档后 is_active 应为 false")
	}
	if archived.ArchivedAt == nil {
		t.Fatal("归档后 archived_at 应非空")
	}
	// 固定时钟断言：归档时间来自注入的 now，不是真实时间。
	want := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if !archived.ArchivedAt.Equal(want) {
		t.Errorf("归档时刻应为 %v，实际 %v", want, archived.ArchivedAt)
	}

	// 重复归档：明确业务错误，不把归档时间刷成第二次点击的时刻。
	if _, err := uc.Archive(context.Background(), created.ID); !errors.Is(err, repository.ErrStudioMemberArchived) {
		t.Fatalf("重复归档应返回 ErrStudioMemberArchived，实际 %v", err)
	}
}

func TestArchiveStudioMember_NotFound(t *testing.T) {
	uc, _ := newStudioMemberUseCase(t)
	if _, err := uc.Archive(context.Background(), 999); !errors.Is(err, repository.ErrStudioMemberNotFound) {
		t.Fatalf("不存在的成员应返回 ErrStudioMemberNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 列表筛选
 * ------------------------------------------------------------------ */

func TestListStudioMembers_ThreeStateActiveFilter(t *testing.T) {
	uc, repo := newStudioMemberUseCase(t)
	first, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: validMemberFields()})
	if err != nil {
		t.Fatal(err)
	}
	other := validMemberFields()
	other.Name = "李四"
	if _, err := uc.Create(context.Background(), CreateStudioMemberInput{Fields: other}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Archive(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}

	// nil = 全部（在读 + 已归档）
	all, total, err := uc.List(context.Background(), repository.StudioMemberListFilter{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || total != 2 {
		t.Fatalf("不筛选应返回全部 2 条，实际 %d/%d", len(all), total)
	}

	// true = 仅在读
	active := true
	onlyActive, _, err := uc.List(context.Background(), repository.StudioMemberListFilter{IsActive: &active}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyActive) != 1 || onlyActive[0].Name != "李四" {
		t.Fatalf("仅在读应只剩李四，实际 %+v", onlyActive)
	}

	// false = 仅已归档
	inactive := false
	onlyArchived, _, err := uc.List(context.Background(), repository.StudioMemberListFilter{IsActive: &inactive}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyArchived) != 1 || onlyArchived[0].Name != "张三" {
		t.Fatalf("仅归档应只剩张三，实际 %+v", onlyArchived)
	}
	// 过滤器要原样传到仓储：三态在服务层不能被压成 bool。
	if repo.lastFilter.IsActive == nil {
		t.Error("筛选条件应原样传给仓储")
	}
}
