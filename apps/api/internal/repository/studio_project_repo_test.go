package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// repoTestDBProjects 一次性库 + 复刻 0014 的项目与参与两张表。
//
// 与 repoTestDB 同一模式：DSN 未设则 skip。DDL **必须复刻生产约束**
// ——尤其是 studio_projects 的 name 唯一索引与参与行的
// (project_id, name) 唯一索引，否则「项目名重复」与「同项目同名参与者」
// 两条验收会被测成假绿。
//
// 先建 studio_members 是为了让参与行的 member_id 外键真实生效：
// 外键缺失会让「关联不存在的成员」被测成合法。
func repoTestDBProjects(t *testing.T) *gorm.DB {
	t.Helper()

	db := repoTestDBStudioMembers(t)
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS studio_projects (
			id bigserial PRIMARY KEY,
			name text NOT NULL,
			year text NOT NULL,
			summary text NOT NULL DEFAULT '',
			status text NOT NULL DEFAULT 'active',
			created_at timestamptz NOT NULL DEFAULT NOW(),
			updated_at timestamptz NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_studio_projects_status
				CHECK (status IN ('active', 'completed', 'archived'))
		)`,
		// 与迁移 0014 一致：项目名全局唯一。
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_projects_name
			ON studio_projects (name)`,
		`CREATE TABLE IF NOT EXISTS studio_project_members (
			id bigserial PRIMARY KEY,
			project_id bigint NOT NULL REFERENCES studio_projects (id) ON DELETE CASCADE,
			member_id bigint REFERENCES studio_members (id) ON DELETE SET NULL,
			name text NOT NULL,
			role text NOT NULL DEFAULT 'contributor',
			started_at timestamptz NOT NULL DEFAULT NOW(),
			ended_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_studio_project_members_role
				CHECK (role IN ('lead', 'contributor'))
		)`,
		// 与迁移 0014 一致：同项目下同名参与者唯一。
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_project_members_project_name
			ON studio_project_members (project_id, name)`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}
	// 种子两个成员档案：参与行的 member_id 外键真实生效，
	// 这样「关联不存在的成员」才会被 FK 拦住而不是静默通过。
	if err := db.Exec(`INSERT INTO studio_members (id, name, birth_year, cohort, direction, role_in_studio)
		VALUES
		  (1, '张三', 2003, '2024 级', 'front-end', '前端成员'),
		  (2, '李四', 2004, '2024 级', 'back-end', '后端成员')`).Error; err != nil {
		t.Fatalf("种子成员失败: %v", err)
	}
	return db
}

func newProject(name, year string) *domain.StudioProject {
	return &domain.StudioProject{
		Name:    name,
		Year:    year,
		Summary: "简介",
		Status:  domain.ProjectActive,
	}
}

func newProjectMember(projectID int, memberID *int, name string) *domain.StudioProjectMember {
	return &domain.StudioProjectMember{
		ProjectID: projectID,
		MemberID:  memberID,
		Name:      name,
		Role:      domain.ProjectMemberLead,
		StartedAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
	}
}

// memberRef 返回指向该整数的指针（便于构造 *int 参数）。
func memberRef(v int) *int { return &v }

/* ------------------------------------------------------------------ *
 * 项目
 * ------------------------------------------------------------------ */

func TestStudioProjectRepo_CreateAndGet(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatalf("创建应成功，实际 %v", err)
	}
	if project.ID == 0 {
		t.Fatal("创建后应回填 id")
	}

	got, err := repo.GetByID(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "校园导览小程序" || got.Year != "2024" || got.Status != domain.ProjectActive {
		t.Errorf("读回的项目不对: %+v", got)
	}
}

func TestStudioProjectRepo_DuplicateNameRejected(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	if err := repo.Create(ctx, newProject("校园导览小程序", "2024")); err != nil {
		t.Fatal(err)
	}

	err := repo.Create(ctx, newProject("校园导览小程序", "2025"))
	if !errors.Is(err, ErrStudioProjectDuplicateName) {
		t.Fatalf("同名项目应返回 ErrStudioProjectDuplicateName，实际 %v", err)
	}
}

func TestStudioProjectRepo_Update(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}

	project.Year = "2025"
	project.Status = domain.ProjectCompleted
	if err := repo.Update(ctx, project); err != nil {
		t.Fatalf("更新应成功，实际 %v", err)
	}

	got, err := repo.GetByID(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year != "2025" || got.Status != domain.ProjectCompleted {
		t.Errorf("更新未生效: %+v", got)
	}
}

func TestStudioProjectRepo_UpdateDuplicateNameTranslation(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	first := newProject("项目甲", "2024")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newProject("项目乙", "2024")
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}

	// 把乙改成与甲同名 → 撞唯一索引，必须翻译成哨兵。
	second.Name = "项目甲"
	if err := repo.Update(ctx, second); !errors.Is(err, ErrStudioProjectDuplicateName) {
		t.Fatalf("撞唯一索引应返回 ErrStudioProjectDuplicateName，实际 %v", err)
	}
}

func TestStudioProjectRepo_DeleteCascadesMembers(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	member := newProjectMember(project.ID, memberRef(1), "张三")
	if err := repo.AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, project.ID); err != nil {
		t.Fatalf("删除应成功，实际 %v", err)
	}

	if _, err := repo.GetByID(ctx, project.ID); !errors.Is(err, ErrStudioProjectNotFound) {
		t.Error("删除后项目应不存在")
	}
	// 参与行由 ON DELETE CASCADE 清理（不是服务层手动删）。
	if _, err := repo.ListMembers(ctx, project.ID); err != nil {
		t.Errorf("级联删除后查询应返回空列表而不是报错，实际 %v", err)
	}
}

func TestStudioProjectRepo_DeleteMemberKeepsProject(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	member := newProjectMember(project.ID, memberRef(1), "张三")
	if err := repo.AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	if err := repo.RemoveMember(ctx, project.ID, member.ID); err != nil {
		t.Fatalf("移除应成功，实际 %v", err)
	}

	// 移除参与行只影响这一行，项目本身还在。
	if _, err := repo.GetByID(ctx, project.ID); err != nil {
		t.Error("移除参与行不应删除项目")
	}
	if _, err := repo.ListMembers(ctx, project.ID); err != nil {
		t.Error("移除后查询应返回空列表")
	}
}

/* ------------------------------------------------------------------ *
 * 参与
 * ------------------------------------------------------------------ */

func TestStudioProjectRepo_AddMember(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}

	// 关联成员档案
	member := newProjectMember(project.ID, memberRef(1), "张三")
	if err := repo.AddMember(ctx, member); err != nil {
		t.Fatalf("添加参与应成功，实际 %v", err)
	}

	// 纯外部参与者：member_id 为 NULL
	external := newProjectMember(project.ID, nil, "校友")
	if err := repo.AddMember(ctx, external); err != nil {
		t.Fatalf("外部参与者应可添加，实际 %v", err)
	}

	members, err := repo.ListMembers(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("应有 2 条参与，实际 %d", len(members))
	}
	// 按开始时间升序，同时间用 id 兜底。
	if members[0].StartedAt.After(members[1].StartedAt) {
		t.Error("参与名单应按开始时间升序")
	}

	// NULL 的 member_id 必须读回 NULL，不能变成零值 ——
	// 否则「外部参与者」与「成员档案 id=0」无法区分。
	var externalRow *domain.StudioProjectMember
	for _, m := range members {
		if m.Name == "校友" {
			externalRow = m
		}
	}
	if externalRow == nil {
		t.Fatal("外部参与者应出现在名单里")
	}
	if externalRow.MemberID != nil {
		t.Errorf("外部参与者的 member_id 应为 NULL，实际 %v", *externalRow.MemberID)
	}
}

func TestStudioProjectRepo_MemberDuplicateNameRejected(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddMember(ctx, newProjectMember(project.ID, memberRef(1), "张三")); err != nil {
		t.Fatal(err)
	}

	// 换一个成员档案，同名第二次出现：必须被 (project_id, name) 唯一索引拦住。
	err := repo.AddMember(ctx, newProjectMember(project.ID, memberRef(2), "张三"))
	if !errors.Is(err, ErrStudioProjectMemberDuplicate) {
		t.Fatalf("同项目同名应返回 ErrStudioProjectMemberDuplicate，实际 %v", err)
	}
}

func TestStudioProjectRepo_MemberDuplicateIsScopedPerProject(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	first := newProject("项目甲", "2024")
	second := newProject("项目乙", "2024")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}

	if err := repo.AddMember(ctx, newProjectMember(first.ID, memberRef(1), "张三")); err != nil {
		t.Fatal(err)
	}
	// 唯一索引是 (project_id, name)，所以另一个项目下同名是合法的。
	if err := repo.AddMember(ctx, newProjectMember(second.ID, memberRef(2), "张三")); err != nil {
		t.Fatalf("不同项目下同名应可共存，实际 %v", err)
	}
}

func TestStudioProjectRepo_UpdateMember(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	project := newProject("校园导览小程序", "2024")
	if err := repo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	member := newProjectMember(project.ID, memberRef(1), "张三")
	if err := repo.AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	ended := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	member.Role = domain.ProjectMemberContributor
	member.EndedAt = &ended
	if err := repo.UpdateMember(ctx, member); err != nil {
		t.Fatalf("更新参与应成功，实际 %v", err)
	}

	got, err := repo.ListMembers(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("应有 1 条参与，实际 %d", len(got))
	}
	if got[0].Role != domain.ProjectMemberContributor {
		t.Errorf("角色应更新，实际 %q", got[0].Role)
	}
	if got[0].EndedAt == nil || !got[0].EndedAt.Equal(ended) {
		t.Errorf("结束时间应更新，实际 %v", got[0].EndedAt)
	}
}

func TestStudioProjectRepo_UpdateMemberScopedToProject(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	first := newProject("项目甲", "2024")
	second := newProject("项目乙", "2024")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	member := newProjectMember(first.ID, memberRef(1), "张三")
	if err := repo.AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	// 用项目乙的 id 更新属于项目甲的参与行：必须找不到。
	member.ProjectID = second.ID
	err := repo.UpdateMember(ctx, member)
	if !errors.Is(err, ErrStudioProjectMemberNotFound) {
		t.Fatalf("跨项目更新应返回 ErrStudioProjectMemberNotFound，实际 %v", err)
	}
}

func TestStudioProjectRepo_RemoveMemberNotFound(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	if err := repo.RemoveMember(ctx, 999, 1); !errors.Is(err, ErrStudioProjectMemberNotFound) {
		t.Fatalf("不存在的参与行应返回 ErrStudioProjectMemberNotFound，实际 %v", err)
	}
}

func TestStudioProjectRepo_StudioMemberExists(t *testing.T) {
	db := repoTestDBProjects(t)
	repo := NewStudioProjectRepo(db)
	ctx := context.Background()

	// 直接验证种子成员：repoTestDBProjects 固定种子了 id=1 / id=2 两条成员档案，
	// 这样既不依赖新插入（避免与 (name, birth_year) 唯一索引纠缠），
	// 也能同时验证「存在」与「不存在」两个分支。
	exists, err := repo.StudioMemberExists(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("种子成员 #1 应报告为存在")
	}

	exists, err = repo.StudioMemberExists(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("种子成员 #2 应报告为存在")
	}

	missing, err := repo.StudioMemberExists(ctx, 999)
	if err != nil {
		t.Fatal(err)
	}
	if missing {
		t.Error("不存在的成员档案应报告为不存在")
	}
}
