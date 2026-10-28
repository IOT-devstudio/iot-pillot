package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// repoTestDBStudioMembers 一次性库 + 复刻 0013 的成员表。
//
// 与 repoTestDB 同一模式：DSN 未设则 skip。DDL **必须复刻生产约束**
// ——特别是 (name, birth_year) 唯一索引与 user_id 可空，否则
// 「同名同出生年被拒」与「无账号成员可建」这两条验收会被测成假绿。
func repoTestDBStudioMembers(t *testing.T) *gorm.DB {
	t.Helper()

	db := repoTestDB(t)
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS studio_members (
			id bigserial PRIMARY KEY,
			user_id bigint,
			name text NOT NULL,
			birth_year integer NOT NULL,
			cohort text NOT NULL,
			direction text NOT NULL,
			role_in_studio text NOT NULL DEFAULT '',
			internship text NOT NULL DEFAULT '',
			employment text NOT NULL DEFAULT '',
			is_active boolean NOT NULL DEFAULT true,
			archived_at timestamptz,
			source_candidate_id bigint,
			created_at timestamptz NOT NULL DEFAULT NOW(),
			updated_at timestamptz NOT NULL DEFAULT NOW()
		)`,
		// 与迁移 0013 一致：同名不同届可共存。
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_members_name_birth_year
			ON studio_members (name, birth_year)`,
		// 与迁移 0013 一致：同一候选人只能转入一次（#197 的数据库防线）。
		// 不复刻它的话「重复转入」会被测成假绿——服务层的前置检查只是让
		// 请求早点被拒，真正的唯一性保证在这一行。
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_members_source_candidate
			ON studio_members (source_candidate_id)
			WHERE source_candidate_id IS NOT NULL`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}
	return db
}

func newRepoStudioMember(name string, birthYear int) *domain.StudioMember {
	return &domain.StudioMember{
		Name:         name,
		BirthYear:    birthYear,
		Cohort:       "2024 级",
		Direction:    "front-end",
		RoleInStudio: "前端负责人",
		IsActive:     true,
	}
}

/* ------------------------------------------------------------------ *
 * 创建：无账号成员 + 唯一键翻译
 * ------------------------------------------------------------------ */

func TestStudioMemberRepo_CreateWithoutUser(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)

	member := newRepoStudioMember("张三", 2003)
	if err := repo.Create(context.Background(), member); err != nil {
		t.Fatalf("无账号成员应可创建，实际 %v", err)
	}
	if member.ID == 0 {
		t.Fatal("创建后应回填 id")
	}

	got, err := repo.GetByID(context.Background(), member.ID)
	if err != nil {
		t.Fatal(err)
	}
	// user_id 可空是 ADR 0001 的硬约束：NULL 必须能读回 NULL，不能变成 0。
	if got.UserID != nil {
		t.Errorf("user_id 应保持 NULL，实际 %v", *got.UserID)
	}
}

func TestStudioMemberRepo_DuplicateNameAndBirthYear(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)

	if err := repo.Create(context.Background(), newRepoStudioMember("张三", 2003)); err != nil {
		t.Fatal(err)
	}
	// 同名同出生年 → 驱动层 23505，必须被翻译成哨兵错误。
	err := repo.Create(context.Background(), newRepoStudioMember("张三", 2003))
	if !errors.Is(err, ErrStudioMemberDuplicate) {
		t.Fatalf("同名同出生年应返回 ErrStudioMemberDuplicate，实际 %v", err)
	}

	// 同名不同届（出生年不同）必须放行：这是 (name, birth_year) 唯一键的意义。
	if err := repo.Create(context.Background(), newRepoStudioMember("张三", 2004)); err != nil {
		t.Fatalf("同名不同出生年应可共存，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 列表筛选
 * ------------------------------------------------------------------ */

func TestStudioMemberRepo_ListFilters(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)
	ctx := context.Background()

	first := newRepoStudioMember("张三", 2003)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newRepoStudioMember("李四", 2003)
	second.Direction = "back-end"
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := repo.Archive(ctx, first.ID, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	// nil = 全部
	all, total, err := repo.List(ctx, StudioMemberListFilter{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || total != 2 {
		t.Fatalf("不筛选应返回 2 条，实际 %d/%d", len(all), total)
	}

	// 方向精确匹配
	backEnd, _, err := repo.List(ctx, StudioMemberListFilter{Direction: "back-end"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(backEnd) != 1 || backEnd[0].Name != "李四" {
		t.Fatalf("方向筛选应只剩李四，实际 %+v", backEnd)
	}

	// 关键词命中职能（ILIKE 覆盖四个字段）
	byRole, _, err := repo.List(ctx, StudioMemberListFilter{Query: "负责人"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(byRole) != 2 {
		t.Fatalf("关键词应命中两条，实际 %d", len(byRole))
	}

	// 关键词里的 LIKE 通配符必须被转义：% 不能变成「匹配所有」。
	literal, _, err := repo.List(ctx, StudioMemberListFilter{Query: "%"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(literal) != 0 {
		t.Fatalf("关键词里的 %% 应被当成普通字符，实际命中 %d 条", len(literal))
	}

	// 三态在读筛选
	active := true
	onlyActive, _, err := repo.List(ctx, StudioMemberListFilter{IsActive: &active}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyActive) != 1 || onlyActive[0].Name != "李四" {
		t.Fatalf("仅在读应只剩李四，实际 %+v", onlyActive)
	}
}

/* ------------------------------------------------------------------ *
 * 来源候选人（#197）
 * ------------------------------------------------------------------ */

func TestStudioMemberRepo_GetBySourceCandidateID(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)
	ctx := context.Background()

	member := newRepoStudioMember("张三", 2003)
	candidateID := 55
	member.SourceCandidateID = &candidateID
	if err := repo.Create(ctx, member); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetBySourceCandidateID(ctx, candidateID)
	if err != nil {
		t.Fatalf("应按来源候选人查到档案，实际 %v", err)
	}
	if got.ID != member.ID {
		t.Errorf("应返回刚建的档案 %d，实际 %d", member.ID, got.ID)
	}

	// 没转过：必须返回 NotFound，服务层据此判定「可以转入」。
	if _, err := repo.GetBySourceCandidateID(ctx, 999); !errors.Is(err, ErrStudioMemberNotFound) {
		t.Fatalf("未转过应返回 ErrStudioMemberNotFound，实际 %v", err)
	}
}

// TestStudioMemberRepo_DuplicateSourceCandidate 锁住数据库层的「一个候选人只能转一次」。
func TestStudioMemberRepo_DuplicateSourceCandidate(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)
	ctx := context.Background()

	candidateID := 55
	first := newRepoStudioMember("张三", 2003)
	first.SourceCandidateID = &candidateID
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}

	// 换个姓名与出生年，绕过 (name, birth_year) 唯一索引，
	// 让这次冲突只可能来自 source_candidate_id。
	second := newRepoStudioMember("李四", 2004)
	second.SourceCandidateID = &candidateID
	if err := repo.Create(ctx, second); !errors.Is(err, ErrStudioMemberDuplicate) {
		t.Fatalf("同一候选人重复转入应撞唯一索引，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 归档：条件更新 + 幂等
 * ------------------------------------------------------------------ */

func TestStudioMemberRepo_ArchiveIsIdempotentGate(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)
	ctx := context.Background()

	member := newRepoStudioMember("张三", 2003)
	if err := repo.Create(ctx, member); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if err := repo.Archive(ctx, member.ID, at); err != nil {
		t.Fatalf("首次归档应成功，实际 %v", err)
	}

	got, err := repo.GetByID(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsActive {
		t.Error("归档后 is_active 应为 false")
	}
	if got.ArchivedAt == nil || !got.ArchivedAt.Equal(at) {
		t.Fatalf("归档时刻应写入 %v，实际 %v", at, got.ArchivedAt)
	}

	// 第二次归档：条件更新拿不到行，按「已归档」拒绝，
	// 而不是把归档时间刷成第二次点击的时刻。
	later := at.Add(time.Hour)
	if err := repo.Archive(ctx, member.ID, later); !errors.Is(err, ErrStudioMemberArchived) {
		t.Fatalf("重复归档应返回 ErrStudioMemberArchived，实际 %v", err)
	}
	got, err = repo.GetByID(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ArchivedAt.Equal(at) {
		t.Errorf("重复归档不应改写归档时刻，实际 %v", got.ArchivedAt)
	}
}

func TestStudioMemberRepo_ArchiveNotFound(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)

	if err := repo.Archive(context.Background(), 999, time.Unix(0, 0)); !errors.Is(err, ErrStudioMemberNotFound) {
		t.Fatalf("不存在的成员应返回 ErrStudioMemberNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 更新：只写可编辑列，且撞唯一键要翻译
 * ------------------------------------------------------------------ */

func TestStudioMemberRepo_UpdateDoesNotClobberArchiveState(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)
	ctx := context.Background()

	member := newRepoStudioMember("张三", 2003)
	if err := repo.Create(ctx, member); err != nil {
		t.Fatal(err)
	}
	if err := repo.Archive(ctx, member.ID, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}

	// 取回归档后的行再改资料：archived_at 必须原样保留。
	current, err := repo.GetByID(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.RoleInStudio = "前端负责人 / 招新"
	if err := repo.Update(ctx, current); err != nil {
		t.Fatalf("更新应成功，实际 %v", err)
	}

	got, err := repo.GetByID(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RoleInStudio != "前端负责人 / 招新" {
		t.Errorf("职能应更新，实际 %q", got.RoleInStudio)
	}
	if got.ArchivedAt == nil {
		t.Error("更新资料不应清掉归档时刻")
	}
}

func TestStudioMemberRepo_UpdateDuplicateTranslation(t *testing.T) {
	db := repoTestDBStudioMembers(t)
	repo := NewStudioMemberRepo(db)
	ctx := context.Background()

	first := newRepoStudioMember("张三", 2003)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := newRepoStudioMember("李四", 2003)
	if err := repo.Create(ctx, second); err != nil {
		t.Fatal(err)
	}

	// 把李四改成与张三同名同出生年 → 撞唯一索引，必须翻译成哨兵。
	second.Name = "张三"
	if err := repo.Update(ctx, second); !errors.Is(err, ErrStudioMemberDuplicate) {
		t.Fatalf("撞唯一索引应返回 ErrStudioMemberDuplicate，实际 %v", err)
	}
}
