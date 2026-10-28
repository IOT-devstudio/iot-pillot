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

// stubProjectRepo 只实现被用到的方法。
//
// 成员存在性用一个 set 表达：参与行的 member_id 校验靠它。
type stubProjectRepo struct {
	repository.StudioProjectRepo

	projects        map[int]*domain.StudioProject
	members         map[int][]*domain.StudioProjectMember
	existingMembers map[int]bool
	nextProjectID   int
	nextMemberID    int

	createErr error
	addErr    error
}

func newStubProjectRepo() *stubProjectRepo {
	return &stubProjectRepo{
		projects:        make(map[int]*domain.StudioProject),
		members:         make(map[int][]*domain.StudioProjectMember),
		existingMembers: map[int]bool{7: true, 8: true},
		nextProjectID:   1,
		nextMemberID:    100,
	}
}

func (s *stubProjectRepo) Create(_ context.Context, project *domain.StudioProject) error {
	if s.createErr != nil {
		return s.createErr
	}
	project.ID = s.nextProjectID
	s.nextProjectID++
	stored := *project
	s.projects[project.ID] = &stored
	return nil
}

func (s *stubProjectRepo) GetByID(_ context.Context, id int) (*domain.StudioProject, error) {
	if p, ok := s.projects[id]; ok {
		return p, nil
	}
	return nil, repository.ErrStudioProjectNotFound
}

func (s *stubProjectRepo) List(_ context.Context, _ repository.StudioProjectListFilter, _ int, _ int) ([]*domain.StudioProject, int64, error) {
	out := make([]*domain.StudioProject, 0, len(s.projects))
	for _, p := range s.projects {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (s *stubProjectRepo) Update(_ context.Context, project *domain.StudioProject) error {
	if _, ok := s.projects[project.ID]; !ok {
		return repository.ErrStudioProjectNotFound
	}
	stored := *project
	s.projects[project.ID] = &stored
	return nil
}

func (s *stubProjectRepo) Delete(_ context.Context, id int) error {
	if _, ok := s.projects[id]; !ok {
		return repository.ErrStudioProjectNotFound
	}
	delete(s.projects, id)
	// 参与行随之级联删除（真实仓储由迁移的 ON DELETE CASCADE 保证）。
	delete(s.members, id)
	return nil
}

func (s *stubProjectRepo) ListMembers(_ context.Context, projectID int) ([]*domain.StudioProjectMember, error) {
	stored := s.members[projectID]
	out := make([]*domain.StudioProjectMember, 0, len(stored))
	out = append(out, stored...)
	return out, nil
}

func (s *stubProjectRepo) AddMember(_ context.Context, member *domain.StudioProjectMember) error {
	if s.addErr != nil {
		return s.addErr
	}
	if _, ok := s.projects[member.ProjectID]; !ok {
		return repository.ErrStudioProjectNotFound
	}
	for _, m := range s.members[member.ProjectID] {
		if m.Name == member.Name {
			return repository.ErrStudioProjectMemberDuplicate
		}
	}
	member.ID = s.nextMemberID
	s.nextMemberID++
	stored := *member
	s.members[member.ProjectID] = append(s.members[member.ProjectID], &stored)
	return nil
}

func (s *stubProjectRepo) UpdateMember(_ context.Context, member *domain.StudioProjectMember) error {
	stored := s.members[member.ProjectID]
	for i, m := range stored {
		if m.ID == member.ID {
			next := *member
			stored[i] = &next
			return nil
		}
	}
	return repository.ErrStudioProjectMemberNotFound
}

func (s *stubProjectRepo) RemoveMember(_ context.Context, projectID int, memberID int) error {
	stored := s.members[projectID]
	for i, m := range stored {
		if m.ID == memberID {
			s.members[projectID] = append(stored[:i], stored[i+1:]...)
			return nil
		}
	}
	return repository.ErrStudioProjectMemberNotFound
}

func (s *stubProjectRepo) StudioMemberExists(_ context.Context, memberID int) (bool, error) {
	return s.existingMembers[memberID], nil
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

func newProjectUseCase(t *testing.T) (*StudioProjectUseCase, *stubProjectRepo) {
	t.Helper()
	repo := newStubProjectRepo()
	uc := NewStudioProjectUseCase(repo)
	uc.now = func() time.Time { return time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC) }
	return uc, repo
}

func projectFields() domain.StudioProjectFields {
	return domain.StudioProjectFields{
		Name:    "校园导览小程序",
		Year:    "2024",
		Summary: "新生报到用的校园导览。",
	}
}

func memberFields(name string) domain.StudioProjectMemberFields {
	memberID := 7
	return domain.StudioProjectMemberFields{
		MemberID:  &memberID,
		Name:      name,
		Role:      domain.ProjectMemberLead,
		StartedAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
	}
}

/* ------------------------------------------------------------------ *
 * 项目 CRUD
 * ------------------------------------------------------------------ */

func TestCreateStudioProject_DefaultsStatusToActive(t *testing.T) {
	uc, repo := newProjectUseCase(t)

	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatalf("创建应成功，实际 %v", err)
	}
	// 不传 status 与传 active 必须写出同一行。
	if project.Status != domain.ProjectActive {
		t.Errorf("缺省状态应为 active，实际 %q", project.Status)
	}
	if len(repo.projects) != 1 {
		t.Error("应写入 1 个项目")
	}
}

func TestCreateStudioProject_Validation(t *testing.T) {
	cases := []struct {
		name  string
		patch func(*domain.StudioProjectFields)
	}{
		{"空项目名", func(f *domain.StudioProjectFields) { f.Name = "   " }},
		{"项目名过长", func(f *domain.StudioProjectFields) { f.Name = string(make([]rune, 101)) }},
		{"空年份", func(f *domain.StudioProjectFields) { f.Year = "" }},
		{"非法状态", func(f *domain.StudioProjectFields) { f.Status = "in_progress" }},
		{"简介过长", func(f *domain.StudioProjectFields) { f.Summary = string(make([]rune, 2001)) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, repo := newProjectUseCase(t)
			fields := projectFields()
			tc.patch(&fields)

			if _, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: fields}); !errors.Is(err, ErrStudioProjectInvalid) {
				t.Fatalf("应返回 ErrStudioProjectInvalid，实际 %v", err)
			}
			if len(repo.projects) != 0 {
				t.Error("校验失败不应写库")
			}
		})
	}
}

func TestCreateStudioProject_DuplicateNameRejected(t *testing.T) {
	uc, repo := newProjectUseCase(t)
	if _, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()}); err != nil {
		t.Fatal(err)
	}

	repo.createErr = repository.ErrStudioProjectDuplicateName
	_, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if !errors.Is(err, repository.ErrStudioProjectDuplicateName) {
		t.Fatalf("同名项目应被拒绝，实际 %v", err)
	}
}

func TestUpdateStudioProject_PatchMergesWithStoredValues(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}

	year := "2025"
	updated, err := uc.Update(context.Background(), created.ID, UpdateStudioProjectInput{Year: &year})
	if err != nil {
		t.Fatalf("更新应成功，实际 %v", err)
	}
	if updated.Year != "2025" {
		t.Errorf("年份应更新为 2025，实际 %q", updated.Year)
	}
	// 未传的字段必须保持原值。
	if updated.Name != "校园导览小程序" {
		t.Errorf("未传的项目名应保持原值，实际 %q", updated.Name)
	}
	if updated.Status != domain.ProjectActive {
		t.Errorf("未传的状态应保持原值，实际 %q", updated.Status)
	}
}

func TestUpdateStudioProject_NotFound(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	name := "新名字"
	if _, err := uc.Update(context.Background(), 999, UpdateStudioProjectInput{Name: &name}); !errors.Is(err, repository.ErrStudioProjectNotFound) {
		t.Fatalf("不存在的项目应返回 ErrStudioProjectNotFound，实际 %v", err)
	}
}

func TestDeleteStudioProject_CascadesMembers(t *testing.T) {
	uc, repo := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: memberFields("张三")}); err != nil {
		t.Fatal(err)
	}

	if err := uc.Delete(context.Background(), project.ID); err != nil {
		t.Fatalf("删除应成功，实际 %v", err)
	}
	if len(repo.projects) != 0 {
		t.Error("删除后项目应不存在")
	}
	// 参与行随项目级联删除。
	if len(repo.members[project.ID]) != 0 {
		t.Error("删除项目应级联清理参与行")
	}
}

/* ------------------------------------------------------------------ *
 * 参与
 * ------------------------------------------------------------------ */

func TestAddProjectMember_WithoutMemberRecord(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}

	// member_id 不传 = 纯外部参与者：姓名快照仍要存在。
	fields := memberFields("校友")
	fields.MemberID = nil
	member, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: fields})
	if err != nil {
		t.Fatalf("外部参与者应可添加，实际 %v", err)
	}
	if member.MemberID != nil {
		t.Errorf("未关联成员档案时应为 NULL，实际 %v", *member.MemberID)
	}
	if member.Name != "校友" {
		t.Errorf("姓名快照应保留，实际 %q", member.Name)
	}
}

func TestAddProjectMember_UnknownMemberRejected(t *testing.T) {
	uc, repo := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}

	missing := 999
	fields := memberFields("张三")
	fields.MemberID = &missing
	_, err = uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: fields})
	if !errors.Is(err, ErrStudioProjectMemberInvalidOwner) {
		t.Fatalf("不存在的成员档案应被拒绝，实际 %v", err)
	}
	if len(repo.members[project.ID]) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestAddProjectMember_DuplicateNameRejected(t *testing.T) {
	uc, repo := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: memberFields("张三")}); err != nil {
		t.Fatal(err)
	}

	// 同一项目下同名第二次出现：必须被拒绝（否则「谁是 lead」有两份答案）。
	_, err = uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: memberFields("张三")})
	if !errors.Is(err, repository.ErrStudioProjectMemberDuplicate) {
		t.Fatalf("同项目同名应被拒绝，实际 %v", err)
	}
	if len(repo.members[project.ID]) != 1 {
		t.Errorf("重复参与不应新增，实际 %d 条", len(repo.members[project.ID]))
	}
}

func TestAddProjectMember_EndedBeforeStartRejected(t *testing.T) {
	uc, repo := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}

	fields := memberFields("张三")
	ended := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	fields.EndedAt = &ended

	_, err = uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: fields})
	if !errors.Is(err, ErrStudioProjectInvalid) {
		t.Fatalf("结束早于开始应被拒绝，实际 %v", err)
	}
	if len(repo.members[project.ID]) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestAddProjectMember_StartedAtDefaultsToNow(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}

	fields := memberFields("张三")
	fields.StartedAt = time.Time{}
	member, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: fields})
	if err != nil {
		t.Fatalf("开始时间缺省应可用，实际 %v", err)
	}
	if member.StartedAt.Unix() != time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("开始时间缺省应为注入的 now，实际 %v", member.StartedAt)
	}
}

func TestAddProjectMember_ProjectNotFound(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	if _, err := uc.AddMember(context.Background(), 999, AddProjectMemberInput{Fields: memberFields("张三")}); !errors.Is(err, repository.ErrStudioProjectNotFound) {
		t.Fatalf("不存在的项目应返回 ErrStudioProjectNotFound，实际 %v", err)
	}
}

func TestUpdateProjectMember_EndedAtNullRestoresOngoing(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	fields := memberFields("张三")
	ended := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fields.EndedAt = &ended
	member, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: fields})
	if err != nil {
		t.Fatal(err)
	}

	// 三态：传了 ended_at 但值是 nil = 仍在参与。
	var ongoing *time.Time
	updated, err := uc.UpdateMember(context.Background(), project.ID, member.ID, UpdateProjectMemberInput{EndedAt: &ongoing})
	if err != nil {
		t.Fatalf("三态清空应成功，实际 %v", err)
	}
	if updated.EndedAt != nil {
		t.Errorf("传 null 应清空结束时间，实际 %v", updated.EndedAt)
	}
}

func TestUpdateProjectMember_KeyAbsentKeepsEndedAt(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	fields := memberFields("张三")
	ended := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fields.EndedAt = &ended
	member, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: fields})
	if err != nil {
		t.Fatal(err)
	}

	// 只改角色：ended_at 键不出现，必须保留原值。
	role := domain.ProjectMemberContributor
	updated, err := uc.UpdateMember(context.Background(), project.ID, member.ID, UpdateProjectMemberInput{Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	if updated.EndedAt == nil || !updated.EndedAt.Equal(ended) {
		t.Errorf("键不出现时不应动结束时间，实际 %v", updated.EndedAt)
	}
	if updated.Role != domain.ProjectMemberContributor {
		t.Errorf("角色应更新，实际 %q", updated.Role)
	}
}

func TestUpdateProjectMember_RoleMustBeLeadOrContributor(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	member, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: memberFields("张三")})
	if err != nil {
		t.Fatal(err)
	}

	invalid := "founder"
	_, err = uc.UpdateMember(context.Background(), project.ID, member.ID, UpdateProjectMemberInput{Role: (*domain.StudioProjectMemberRole)(&invalid)})
	if !errors.Is(err, ErrStudioProjectInvalid) {
		t.Fatalf("非法角色应被拒绝，实际 %v", err)
	}
}

func TestUpdateProjectMember_NotFound(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.UpdateMember(context.Background(), project.ID, 999, UpdateProjectMemberInput{}); !errors.Is(err, repository.ErrStudioProjectMemberNotFound) {
		t.Fatalf("不存在的参与行应返回 ErrStudioProjectMemberNotFound，实际 %v", err)
	}
}

func TestUpdateProjectMember_OtherProjectNotMutated(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	first, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	secondFields := projectFields()
	secondFields.Name = "另一个项目"
	second, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: secondFields})
	if err != nil {
		t.Fatal(err)
	}
	member, err := uc.AddMember(context.Background(), first.ID, AddProjectMemberInput{Fields: memberFields("张三")})
	if err != nil {
		t.Fatal(err)
	}

	// 参与行属于项目一，用项目二的 id 更新必须找不到——
	// 否则「参与行属于哪个项目」就取决于查询顺序。
	_, err = uc.UpdateMember(context.Background(), second.ID, member.ID, UpdateProjectMemberInput{})
	if !errors.Is(err, repository.ErrStudioProjectMemberNotFound) {
		t.Fatalf("跨项目更新应被拒绝，实际 %v", err)
	}
}

func TestRemoveProjectMember(t *testing.T) {
	uc, repo := newProjectUseCase(t)
	project, err := uc.Create(context.Background(), CreateStudioProjectInput{Fields: projectFields()})
	if err != nil {
		t.Fatal(err)
	}
	member, err := uc.AddMember(context.Background(), project.ID, AddProjectMemberInput{Fields: memberFields("张三")})
	if err != nil {
		t.Fatal(err)
	}

	if err := uc.RemoveMember(context.Background(), project.ID, member.ID); err != nil {
		t.Fatalf("移除应成功，实际 %v", err)
	}
	if len(repo.members[project.ID]) != 0 {
		t.Error("移除后参与行应不存在")
	}

	// 重复移除：明确业务错误。
	if err := uc.RemoveMember(context.Background(), project.ID, member.ID); !errors.Is(err, repository.ErrStudioProjectMemberNotFound) {
		t.Fatalf("重复移除应返回 ErrStudioProjectMemberNotFound，实际 %v", err)
	}
}

func TestListProjectMembers_ProjectNotFound(t *testing.T) {
	uc, _ := newProjectUseCase(t)
	// 不存在的项目必须报错，而不是返回空列表——
	// 空列表会让调用方以为「这个项目还没有参与人」。
	if _, err := uc.ListMembers(context.Background(), 999); !errors.Is(err, repository.ErrStudioProjectNotFound) {
		t.Fatalf("不存在的项目应返回 ErrStudioProjectNotFound，实际 %v", err)
	}
}
