package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// repoTestDBKnowledge 一次性库 + 复刻 0015 的知识库两张表。
//
// 与 repoTestDB 同一模式：DSN 未设则 skip。DDL **必须复刻生产约束**
// ——(entry_id, version) 唯一索引与 ON DELETE CASCADE 是本票两条核心语义的
// 唯一执行者，漏掉它们会让「并发升版」和「删除级联」被测成假绿。
func repoTestDBKnowledge(t *testing.T) *gorm.DB {
	t.Helper()

	db := repoTestDB(t)
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS studio_knowledge_entries (
			id bigserial PRIMARY KEY,
			title text NOT NULL,
			body text NOT NULL DEFAULT '',
			author_id bigint NOT NULL,
			is_public boolean NOT NULL DEFAULT false,
			permission text NOT NULL DEFAULT 'admin',
			parent_id bigint REFERENCES studio_knowledge_entries (id) ON DELETE SET NULL,
			version integer NOT NULL DEFAULT 1,
			created_at timestamptz NOT NULL DEFAULT NOW(),
			updated_at timestamptz NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_studio_knowledge_permission
				CHECK (permission IN ('public', 'member', 'admin')),
			CONSTRAINT chk_studio_knowledge_version CHECK (version >= 1)
		)`,
		`CREATE TABLE IF NOT EXISTS studio_knowledge_revisions (
			id bigserial PRIMARY KEY,
			entry_id bigint NOT NULL REFERENCES studio_knowledge_entries (id) ON DELETE CASCADE,
			version integer NOT NULL,
			title text NOT NULL,
			body text NOT NULL,
			editor_id bigint NOT NULL,
			created_at timestamptz NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_studio_knowledge_rev_version CHECK (version >= 1)
		)`,
		// 与迁移 0015 一致：同一版本不可能有两份快照。
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_knowledge_revisions_entry_version
			ON studio_knowledge_revisions (entry_id, version)`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}
	return db
}

func newKnowledgeEntry(title, body string) *domain.StudioKnowledgeEntry {
	return &domain.StudioKnowledgeEntry{
		Title:      title,
		Body:       body,
		AuthorID:   1,
		Permission: domain.KnowledgePermissionAdmin,
	}
}

func newKnowledgeRevision(entry *domain.StudioKnowledgeEntry, editorID int) *domain.StudioKnowledgeRevision {
	return &domain.StudioKnowledgeRevision{
		Title:    entry.Title,
		Body:     entry.Body,
		EditorID: editorID,
	}
}

/* ------------------------------------------------------------------ *
 * 创建：v1 快照 + 父条目校验
 * ------------------------------------------------------------------ */

func TestKnowledgeRepo_CreateWritesVersionOneRevision(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	entry := newKnowledgeEntry("部署手册", "第一版正文")
	if err := repo.CreateEntry(ctx, entry, newKnowledgeRevision(entry, 1)); err != nil {
		t.Fatalf("创建应成功，实际 %v", err)
	}
	if entry.Version != 1 {
		t.Fatalf("新条目版本应为 1，实际 %d", entry.Version)
	}

	got, err := repo.GetEntry(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Title != "部署手册" {
		t.Errorf("读回的条目不对: %+v", got)
	}

	// v1 快照必须落库，否则历史里缺第一版。
	revision, err := repo.GetRevision(ctx, entry.ID, 1)
	if err != nil {
		t.Fatalf("v1 修订应存在，实际 %v", err)
	}
	if revision.Body != "第一版正文" || revision.EditorID != 1 {
		t.Errorf("v1 快照内容不对: %+v", revision)
	}
}

func TestKnowledgeRepo_CreateRejectsMissingParent(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)

	missing := 999
	entry := newKnowledgeEntry("孤儿", "x")
	entry.ParentID = &missing

	err := repo.CreateEntry(context.Background(), entry, newKnowledgeRevision(entry, 1))
	if !errors.Is(err, ErrKnowledgeParentNotFound) {
		t.Fatalf("不存在的父条目应返回 ErrKnowledgeParentNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 保存：版本 +1、新快照、并发闸门
 * ------------------------------------------------------------------ */

func TestKnowledgeRepo_UpdateBumpsVersionAndAppendsRevision(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	entry := newKnowledgeEntry("部署手册", "第一版正文")
	if err := repo.CreateEntry(ctx, entry, newKnowledgeRevision(entry, 1)); err != nil {
		t.Fatal(err)
	}

	update := &domain.StudioKnowledgeEntry{
		ID:         entry.ID,
		Title:      "部署手册",
		Body:       "第二版正文",
		Permission: domain.KnowledgePermissionAdmin,
	}
	if err := repo.UpdateEntry(ctx, update, newKnowledgeRevision(update, 9), 1); err != nil {
		t.Fatalf("保存应成功，实际 %v", err)
	}

	got, err := repo.GetEntry(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Fatalf("保存后版本应为 2，实际 %d", got.Version)
	}
	if got.Body != "第二版正文" {
		t.Errorf("正文应更新，实际 %q", got.Body)
	}

	revisions, err := repo.ListRevisions(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 {
		t.Fatalf("应有 2 条修订，实际 %d", len(revisions))
	}
	// 版本倒序：最新在前。
	if revisions[0].Version != 2 || revisions[1].Version != 1 {
		t.Errorf("修订应按版本倒序，实际 %d, %d", revisions[0].Version, revisions[1].Version)
	}
	if revisions[0].EditorID != 9 {
		t.Errorf("v2 编辑者应为 9，实际 %d", revisions[0].EditorID)
	}
}

// TestKnowledgeRepo_UpdateRejectsStaleVersion 锁住乐观锁（条件更新）。
//
// 两个人同时打开同一版编辑时，先保存的把版本推到 2；后保存的必须拿到冲突，
// 而不是静默覆盖——那样前一个人的改动就消失了，且没有任何痕迹。
func TestKnowledgeRepo_UpdateRejectsStaleVersion(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	entry := newKnowledgeEntry("部署手册", "初始正文")
	if err := repo.CreateEntry(ctx, entry, newKnowledgeRevision(entry, 1)); err != nil {
		t.Fatal(err)
	}

	first := &domain.StudioKnowledgeEntry{ID: entry.ID, Title: "部署手册", Body: "甲的改动", Permission: domain.KnowledgePermissionAdmin}
	if err := repo.UpdateEntry(ctx, first, newKnowledgeRevision(first, 1), 1); err != nil {
		t.Fatalf("首次保存应成功，实际 %v", err)
	}

	second := &domain.StudioKnowledgeEntry{ID: entry.ID, Title: "部署手册", Body: "乙的改动", Permission: domain.KnowledgePermissionAdmin}
	err := repo.UpdateEntry(ctx, second, newKnowledgeRevision(second, 2), 1)
	if !errors.Is(err, ErrKnowledgeVersionConflict) {
		t.Fatalf("旧版本号应返回 ErrKnowledgeVersionConflict，实际 %v", err)
	}

	// 甲的改动必须还在。
	got, err := repo.GetEntry(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "甲的改动" {
		t.Errorf("冲突不应覆盖已保存的改动，实际 %q", got.Body)
	}
	if got.Version != 2 {
		t.Errorf("冲突不应推进版本，实际 %d", got.Version)
	}
}

func TestKnowledgeRepo_UpdateNotFound(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)

	missing := &domain.StudioKnowledgeEntry{ID: 999, Title: "x", Body: "y", Permission: domain.KnowledgePermissionAdmin}
	err := repo.UpdateEntry(context.Background(), missing, newKnowledgeRevision(missing, 1), 1)
	if !errors.Is(err, ErrKnowledgeEntryNotFound) {
		t.Fatalf("不存在的条目应返回 ErrKnowledgeEntryNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 删除：级联修订 + 子条目 parent 置空
 * ------------------------------------------------------------------ */

func TestKnowledgeRepo_DeleteCascadesRevisions(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	entry := newKnowledgeEntry("部署手册", "正文")
	if err := repo.CreateEntry(ctx, entry, newKnowledgeRevision(entry, 1)); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatalf("删除应成功，实际 %v", err)
	}

	if _, err := repo.GetEntry(ctx, entry.ID); !errors.Is(err, ErrKnowledgeEntryNotFound) {
		t.Error("删除后条目应不存在")
	}
	// 修订由 ON DELETE CASCADE 清理（不是服务层手动删）。
	if _, err := repo.GetRevision(ctx, entry.ID, 1); !errors.Is(err, ErrKnowledgeRevisionNotFound) {
		t.Error("删除条目应级联清理修订")
	}
}

func TestKnowledgeRepo_DeleteDetachesChildren(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	root := newKnowledgeEntry("根", "根正文")
	if err := repo.CreateEntry(ctx, root, newKnowledgeRevision(root, 1)); err != nil {
		t.Fatal(err)
	}
	child := newKnowledgeEntry("子", "子正文")
	child.ParentID = &root.ID
	if err := repo.CreateEntry(ctx, child, newKnowledgeRevision(child, 1)); err != nil {
		t.Fatal(err)
	}

	// 删父条目：子条目**不**连带删除，只把 parent_id 置空（迁移的 ON DELETE SET NULL）。
	if err := repo.DeleteEntry(ctx, root.ID); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetEntry(ctx, child.ID)
	if err != nil {
		t.Fatalf("子条目应保留，实际 %v", err)
	}
	if got.ParentID != nil {
		t.Errorf("父条目删除后子条目 parent_id 应置空，实际 %v", *got.ParentID)
	}
}

func TestKnowledgeRepo_DeleteNotFound(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)

	if err := repo.DeleteEntry(context.Background(), 999); !errors.Is(err, ErrKnowledgeEntryNotFound) {
		t.Fatalf("不存在的条目应返回 ErrKnowledgeEntryNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 列表筛选
 * ------------------------------------------------------------------ */

func TestKnowledgeRepo_ListFilters(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	first := newKnowledgeEntry("部署手册", "第一版正文")
	if err := repo.CreateEntry(ctx, first, newKnowledgeRevision(first, 1)); err != nil {
		t.Fatal(err)
	}
	second := newKnowledgeEntry("架构笔记", "关于分层的思考")
	second.Permission = domain.KnowledgePermissionMember
	if err := repo.CreateEntry(ctx, second, newKnowledgeRevision(second, 1)); err != nil {
		t.Fatal(err)
	}

	// 关键词命中正文
	byBody, _, err := repo.ListEntries(ctx, StudioKnowledgeListFilter{Query: "分层"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(byBody) != 1 || byBody[0].Title != "架构笔记" {
		t.Fatalf("正文关键词应命中架构笔记，实际 %+v", byBody)
	}

	// 关键词里的 LIKE 通配符必须被转义：% 不能变成「匹配所有」。
	literal, _, err := repo.ListEntries(ctx, StudioKnowledgeListFilter{Query: "%"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(literal) != 0 {
		t.Fatalf("关键词里的 %% 应被当成普通字符，实际命中 %d 条", len(literal))
	}

	// 权限精确匹配
	memberOnly, _, err := repo.ListEntries(ctx, StudioKnowledgeListFilter{Permission: "member"}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberOnly) != 1 || memberOnly[0].Title != "架构笔记" {
		t.Fatalf("权限筛选应只剩架构笔记，实际 %+v", memberOnly)
	}
}

func TestKnowledgeRepo_ListParentFilterThreeState(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	root := newKnowledgeEntry("根", "根正文")
	if err := repo.CreateEntry(ctx, root, newKnowledgeRevision(root, 1)); err != nil {
		t.Fatal(err)
	}
	child := newKnowledgeEntry("子", "子正文")
	child.ParentID = &root.ID
	if err := repo.CreateEntry(ctx, child, newKnowledgeRevision(child, 1)); err != nil {
		t.Fatal(err)
	}

	// nil = 不筛选（含子条目）
	all, _, err := repo.ListEntries(ctx, StudioKnowledgeListFilter{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("不筛选应返回 2 条，实际 %d", len(all))
	}

	// 指向 nil = 只看顶层
	var topLevel *int
	roots, _, err := repo.ListEntries(ctx, StudioKnowledgeListFilter{ParentID: &topLevel}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Title != "根" {
		t.Fatalf("只看顶层应只剩根，实际 %+v", roots)
	}

	// 指向数字 = 只看该条目的直接子条目
	// 两层指针：外层非 nil = 要按父条目筛选，内层非 nil = 筛这一条。
	parentID := root.ID
	inner := &parentID
	children, _, err := repo.ListEntries(ctx, StudioKnowledgeListFilter{ParentID: &inner}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Title != "子" {
		t.Fatalf("看子条目应只剩子，实际 %+v", children)
	}
}

/* ------------------------------------------------------------------ *
 * 环检测
 * ------------------------------------------------------------------ */

func TestKnowledgeRepo_IsDescendant(t *testing.T) {
	db := repoTestDBKnowledge(t)
	repo := NewStudioKnowledgeRepo(db)
	ctx := context.Background()

	root := newKnowledgeEntry("根", "根正文")
	if err := repo.CreateEntry(ctx, root, newKnowledgeRevision(root, 1)); err != nil {
		t.Fatal(err)
	}
	child := newKnowledgeEntry("子", "子正文")
	child.ParentID = &root.ID
	if err := repo.CreateEntry(ctx, child, newKnowledgeRevision(child, 1)); err != nil {
		t.Fatal(err)
	}
	grandchild := newKnowledgeEntry("孙", "孙正文")
	grandchild.ParentID = &child.ID
	if err := repo.CreateEntry(ctx, grandchild, newKnowledgeRevision(grandchild, 1)); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		ancestor  int
		candidate int
		want      bool
	}{
		{"自身算子孙（挡住自己指自己）", root.ID, root.ID, true},
		{"直接子条目", root.ID, child.ID, true},
		{"隔代孙条目", root.ID, grandchild.ID, true},
		{"反向不是子孙", grandchild.ID, root.ID, false},
		{"无关条目", child.ID, child.ID, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.IsDescendant(ctx, tc.ancestor, tc.candidate)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("IsDescendant(%d,%d) 应为 %v，实际 %v", tc.ancestor, tc.candidate, tc.want, got)
			}
		})
	}
}
