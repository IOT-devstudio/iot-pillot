package service

import (
	"context"
	"errors"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 桩
 * ------------------------------------------------------------------ */

// stubKnowledgeRepo 在内存里复刻「保存 = 升版 + 写快照」的语义。
//
// 刻意自己实现版本闸门与快照追加，而不是简单存一行：
// 本票的核心验收就是「保存升版与历史读取」，桩必须能表现出版本推进，
// 否则测的只是我自己的 if。
type stubKnowledgeRepo struct {
	repository.StudioKnowledgeRepo

	entries   map[int]*domain.StudioKnowledgeEntry
	revisions map[int][]*domain.StudioKnowledgeRevision
	nextID    int
	nextRevID int

	createErr error
	updateErr error
	deleteErr error
}

func newStubKnowledgeRepo() *stubKnowledgeRepo {
	return &stubKnowledgeRepo{
		entries:   make(map[int]*domain.StudioKnowledgeEntry),
		revisions: make(map[int][]*domain.StudioKnowledgeRevision),
		nextID:    1,
		nextRevID: 100,
	}
}

func (s *stubKnowledgeRepo) CreateEntry(_ context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision) error {
	if s.createErr != nil {
		return s.createErr
	}
	if entry.ParentID != nil {
		if _, ok := s.entries[*entry.ParentID]; !ok {
			return repository.ErrKnowledgeParentNotFound
		}
	}
	entry.ID = s.nextID
	s.nextID++
	entry.Version = 1
	stored := *entry
	s.entries[entry.ID] = &stored

	revision.ID = s.nextRevID
	s.nextRevID++
	revision.EntryID = entry.ID
	revision.Version = 1
	rev := *revision
	s.revisions[entry.ID] = []*domain.StudioKnowledgeRevision{&rev}
	return nil
}

func (s *stubKnowledgeRepo) GetEntry(_ context.Context, id int) (*domain.StudioKnowledgeEntry, error) {
	if e, ok := s.entries[id]; ok {
		return e, nil
	}
	return nil, repository.ErrKnowledgeEntryNotFound
}

func (s *stubKnowledgeRepo) ListEntries(_ context.Context, _ repository.StudioKnowledgeListFilter, _ int, _ int) ([]*domain.StudioKnowledgeEntry, int64, error) {
	out := make([]*domain.StudioKnowledgeEntry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	return out, int64(len(out)), nil
}

func (s *stubKnowledgeRepo) UpdateEntry(_ context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision, expectedVersion int) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	current, ok := s.entries[entry.ID]
	if !ok {
		return repository.ErrKnowledgeEntryNotFound
	}
	// 复刻仓储的条件更新：版本对不上就是并发冲突。
	if current.Version != expectedVersion {
		return repository.ErrKnowledgeVersionConflict
	}
	if entry.ParentID != nil {
		if _, ok := s.entries[*entry.ParentID]; !ok {
			return repository.ErrKnowledgeParentNotFound
		}
	}
	current.Title = entry.Title
	current.Body = entry.Body
	current.Permission = entry.Permission
	current.IsPublic = entry.IsPublic
	current.ParentID = entry.ParentID
	current.Version = expectedVersion + 1

	revision.ID = s.nextRevID
	s.nextRevID++
	revision.EntryID = entry.ID
	revision.Version = current.Version
	rev := *revision
	s.revisions[entry.ID] = append(s.revisions[entry.ID], &rev)
	return nil
}

func (s *stubKnowledgeRepo) DeleteEntry(_ context.Context, id int) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.entries[id]; !ok {
		return repository.ErrKnowledgeEntryNotFound
	}
	delete(s.entries, id)
	delete(s.revisions, id)
	return nil
}

func (s *stubKnowledgeRepo) ListRevisions(_ context.Context, entryID int) ([]*domain.StudioKnowledgeRevision, error) {
	stored := s.revisions[entryID]
	out := make([]*domain.StudioKnowledgeRevision, 0, len(stored))
	// 与真实仓储一致：版本倒序。
	for i := len(stored) - 1; i >= 0; i-- {
		out = append(out, stored[i])
	}
	return out, nil
}

func (s *stubKnowledgeRepo) GetRevision(_ context.Context, entryID int, version int) (*domain.StudioKnowledgeRevision, error) {
	for _, rev := range s.revisions[entryID] {
		if rev.Version == version {
			return rev, nil
		}
	}
	return nil, repository.ErrKnowledgeRevisionNotFound
}

func (s *stubKnowledgeRepo) IsDescendant(_ context.Context, ancestorID int, candidateID int) (bool, error) {
	// 沿 parent 链往上走，看能不能走到 ancestor。
	current, ok := s.entries[candidateID]
	for ok && current.ParentID != nil {
		if *current.ParentID == ancestorID {
			return true, nil
		}
		current, ok = s.entries[*current.ParentID]
	}
	return ancestorID == candidateID, nil
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

func newKnowledgeUseCase(t *testing.T) (*StudioKnowledgeUseCase, *stubKnowledgeRepo) {
	t.Helper()
	repo := newStubKnowledgeRepo()
	return NewStudioKnowledgeUseCase(repo), repo
}

// parentPatch 构造「要改 parent_id 为这个值」的两层指针入参。
//
// UpdateStudioKnowledgeInput.ParentID 是 **int：外层非 nil = 要改，
// 内层值 = 改到哪个父条目（nil = 移到顶层）。测试里手写 &x 只拿到一层。
func parentPatch(value *int) **int {
	return &value
}

func knowledgeFields() domain.StudioKnowledgeFields {
	return domain.StudioKnowledgeFields{
		Title: "部署手册",
		Body:  "第一版正文",
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：创建条目同时产生 v1 修订
 * ------------------------------------------------------------------ */

func TestCreateKnowledgeEntry_WritesVersionOneRevision(t *testing.T) {
	uc, repo := newKnowledgeUseCase(t)

	entry, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{
		Fields:   knowledgeFields(),
		AuthorID: 7,
	})
	if err != nil {
		t.Fatalf("创建应成功，实际 %v", err)
	}
	if entry.Version != 1 {
		t.Errorf("新条目版本应为 1，实际 %d", entry.Version)
	}
	if entry.AuthorID != 7 {
		t.Errorf("作者应取自入参，实际 %d", entry.AuthorID)
	}

	// v1 快照必须存在：否则历史里缺第一版，管理员看到版本号从 1 起却没有 1 的内容。
	revisions := repo.revisions[entry.ID]
	if len(revisions) != 1 {
		t.Fatalf("创建应写入 1 条修订，实际 %d", len(revisions))
	}
	if revisions[0].Version != 1 || revisions[0].Title != "部署手册" || revisions[0].Body != "第一版正文" {
		t.Errorf("v1 快照内容不对: %+v", revisions[0])
	}
	if revisions[0].EditorID != 7 {
		t.Errorf("v1 编辑者应为创建人，实际 %d", revisions[0].EditorID)
	}
}

func TestCreateKnowledgeEntry_DefaultsPermissionToAdmin(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)

	entry, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	// 不传 permission 与传 admin 必须写出同一行，否则同一份数据会有两种表示。
	if entry.Permission != domain.KnowledgePermissionAdmin {
		t.Errorf("缺省权限应为 admin，实际 %q", entry.Permission)
	}
	if entry.IsPublic {
		t.Error("admin 权限的条目 is_public 应为 false")
	}
}

func TestCreateKnowledgeEntry_DerivesIsPublicFromPermission(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)

	fields := knowledgeFields()
	fields.Permission = domain.KnowledgePermissionPublic
	entry, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: fields, AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	// is_public 是派生值：两处各写一份必然会漂移。
	if !entry.IsPublic {
		t.Error("public 权限的条目 is_public 应为 true")
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：字段校验
 * ------------------------------------------------------------------ */

func TestCreateKnowledgeEntry_Validation(t *testing.T) {
	cases := []struct {
		name  string
		patch func(*domain.StudioKnowledgeFields)
	}{
		{"空标题", func(f *domain.StudioKnowledgeFields) { f.Title = "   " }},
		{"标题过长", func(f *domain.StudioKnowledgeFields) { f.Title = string(make([]rune, 201)) }},
		{"非法权限", func(f *domain.StudioKnowledgeFields) { f.Permission = "everyone" }},
		{"父 id 非正", func(f *domain.StudioKnowledgeFields) { zero := 0; f.ParentID = &zero }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, repo := newKnowledgeUseCase(t)
			fields := knowledgeFields()
			tc.patch(&fields)

			if _, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: fields, AuthorID: 1}); !errors.Is(err, ErrStudioKnowledgeInvalid) {
				t.Fatalf("应返回 ErrStudioKnowledgeInvalid，实际 %v", err)
			}
			if len(repo.entries) != 0 {
				t.Error("校验失败不应写库")
			}
		})
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：更新 = version+1 + 新修订
 * ------------------------------------------------------------------ */

func TestUpdateKnowledgeEntry_BumpsVersionAndAppendsRevision(t *testing.T) {
	uc, repo := newKnowledgeUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	body := "第二版正文"
	updated, err := uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{
		Body:     &body,
		EditorID: 9,
	}, created.Version)
	if err != nil {
		t.Fatalf("保存应成功，实际 %v", err)
	}

	if updated.Version != 2 {
		t.Fatalf("保存后版本应为 2，实际 %d", updated.Version)
	}
	if updated.Body != "第二版正文" {
		t.Errorf("正文应更新，实际 %q", updated.Body)
	}
	// 未传的标题必须保持原值，否则每次保存都会把别的字段清空。
	if updated.Title != "部署手册" {
		t.Errorf("未传的标题应保持原值，实际 %q", updated.Title)
	}

	revisions := repo.revisions[created.ID]
	if len(revisions) != 2 {
		t.Fatalf("两次保存应有 2 条修订，实际 %d", len(revisions))
	}
	if revisions[1].Version != 2 || revisions[1].Body != "第二版正文" {
		t.Errorf("v2 快照内容不对: %+v", revisions[1])
	}
	if revisions[1].EditorID != 9 {
		t.Errorf("v2 编辑者应为 9，实际 %d", revisions[1].EditorID)
	}
}

func TestUpdateKnowledgeEntry_AllowsEmptyBody(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	// 正文可以是空串（先建骨架、回头再写）。指针语义让「传空串」与「没传」分开。
	empty := ""
	updated, err := uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{Body: &empty, EditorID: 1}, created.Version)
	if err != nil {
		t.Fatalf("正文置空应被允许，实际 %v", err)
	}
	if updated.Body != "" {
		t.Errorf("正文应被清空，实际 %q", updated.Body)
	}
}

// TestUpdateKnowledgeEntry_RequiresVersion 锁住「version 必填」。
//
// 缺 version 时若默默回退到 current.Version，乐观锁就退化成「总是覆盖」：
// 两个管理员同时编辑时，后保存的会静默抹掉前一个人的改动，
// 而历史里看不出发生过覆盖。宁可报参数错误，也不给一个「成功」的假象。
func TestUpdateKnowledgeEntry_RequiresVersion(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	body := "没有带版本号的保存"
	_, err = uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{Body: &body, EditorID: 1}, 0)
	if !errors.Is(err, ErrStudioKnowledgeInvalid) {
		t.Fatalf("缺 version 应返回 ErrStudioKnowledgeInvalid，实际 %v", err)
	}
}

func TestUpdateKnowledgeEntry_VersionConflict(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	body := "第一次保存"
	if _, err := uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{Body: &body, EditorID: 1}, 1); err != nil {
		t.Fatal(err)
	}

	// 另一个人还拿着 version=1 在编辑：必须冲突，不能静默覆盖。
	other := "另一个人的改动"
	_, err = uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{Body: &other, EditorID: 2}, 1)
	if !errors.Is(err, ErrStudioKnowledgeVersionConflict) {
		t.Fatalf("旧版本号保存应冲突，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：历史读取
 * ------------------------------------------------------------------ */

func TestListKnowledgeRevisions_NewestFirst(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	second := "第二版"
	if _, err := uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{Body: &second, EditorID: 1}, 1); err != nil {
		t.Fatal(err)
	}

	revisions, err := uc.Revisions(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 {
		t.Fatalf("应有 2 条修订，实际 %d", len(revisions))
	}
	// 版本倒序：最新在前，管理端列表直接展示即可。
	if revisions[0].Version != 2 || revisions[1].Version != 1 {
		t.Errorf("修订应按版本倒序，实际 %d, %d", revisions[0].Version, revisions[1].Version)
	}
	if revisions[0].Body != "第二版" || revisions[1].Body != "第一版正文" {
		t.Errorf("快照内容不对: %q, %q", revisions[0].Body, revisions[1].Body)
	}
}

func TestListKnowledgeRevisions_MissingEntryIsNotFound(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)

	// 不存在的条目要报「不存在」，而不是返回空列表——
	// 空列表会让调用方以为「这条目没有历史」。
	if _, err := uc.Revisions(context.Background(), 999); !errors.Is(err, repository.ErrKnowledgeEntryNotFound) {
		t.Fatalf("不存在的条目应返回 ErrKnowledgeEntryNotFound，实际 %v", err)
	}
}

func TestGetKnowledgeRevision(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	second := "第二版"
	if _, err := uc.Update(context.Background(), created.ID, UpdateStudioKnowledgeInput{Body: &second, EditorID: 1}, 1); err != nil {
		t.Fatal(err)
	}

	// 取旧版：这是「恢复」的实现方式——读回 v1 的内容，再保存成 v3。
	old, err := uc.Revision(context.Background(), created.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if old.Body != "第一版正文" {
		t.Errorf("v1 正文应为初始内容，实际 %q", old.Body)
	}

	if _, err := uc.Revision(context.Background(), created.ID, 99); !errors.Is(err, repository.ErrKnowledgeRevisionNotFound) {
		t.Fatalf("不存在的版本应返回 ErrKnowledgeRevisionNotFound，实际 %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：可选 parent 树，非法 parent 有明确错误
 * ------------------------------------------------------------------ */

func TestCreateKnowledgeEntry_WithParent(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	parent, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	child := knowledgeFields()
	child.Title = "子条目"
	child.ParentID = &parent.ID
	created, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: child, AuthorID: 1})
	if err != nil {
		t.Fatalf("挂到存在的父条目下应成功，实际 %v", err)
	}
	if created.ParentID == nil || *created.ParentID != parent.ID {
		t.Errorf("父条目应为 %d，实际 %v", parent.ID, created.ParentID)
	}
}

func TestCreateKnowledgeEntry_MissingParentIsRejected(t *testing.T) {
	uc, repo := newKnowledgeUseCase(t)

	missing := 999
	fields := knowledgeFields()
	fields.ParentID = &missing

	_, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: fields, AuthorID: 1})
	if !errors.Is(err, ErrStudioKnowledgeParentInvalid) {
		t.Fatalf("不存在的父条目应返回 ErrStudioKnowledgeParentInvalid，实际 %v", err)
	}
	if len(repo.entries) != 0 {
		t.Error("父条目校验失败不应写库")
	}
}

func TestUpdateKnowledgeEntry_RejectsParentCycle(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	root, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	childFields := knowledgeFields()
	childFields.Title = "子条目"
	childFields.ParentID = &root.ID
	child, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: childFields, AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	// 把根挂到自己的子条目下 → 树变成环，递归读取会无限循环。
	cycle := child.ID
	_, err = uc.Update(context.Background(), root.ID, UpdateStudioKnowledgeInput{ParentID: parentPatch(&cycle), EditorID: 1}, root.Version)
	if !errors.Is(err, ErrStudioKnowledgeParentInvalid) {
		t.Fatalf("成环的父条目应被拒绝，实际 %v", err)
	}
}

func TestUpdateKnowledgeEntry_RejectsSelfParent(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	entry, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	self := entry.ID
	_, err = uc.Update(context.Background(), entry.ID, UpdateStudioKnowledgeInput{ParentID: parentPatch(&self), EditorID: 1}, entry.Version)
	if !errors.Is(err, ErrStudioKnowledgeParentInvalid) {
		t.Fatalf("自己指自己应被拒绝，实际 %v", err)
	}
}

func TestUpdateKnowledgeEntry_MoveToTopLevel(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	root, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}
	childFields := knowledgeFields()
	childFields.ParentID = &root.ID
	child, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: childFields, AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	// 三态：传了 parent_id 但值是 null = 移到顶层。
	var topLevel *int
	updated, err := uc.Update(context.Background(), child.ID, UpdateStudioKnowledgeInput{ParentID: parentPatch(topLevel), EditorID: 1}, child.Version)
	if err != nil {
		t.Fatalf("移到顶层应成功，实际 %v", err)
	}
	if updated.ParentID != nil {
		t.Errorf("应移到顶层，实际 parent_id=%v", *updated.ParentID)
	}
}

/* ------------------------------------------------------------------ *
 * 删除
 * ------------------------------------------------------------------ */

func TestDeleteKnowledgeEntry(t *testing.T) {
	uc, repo := newKnowledgeUseCase(t)
	entry, err := uc.Create(context.Background(), CreateStudioKnowledgeInput{Fields: knowledgeFields(), AuthorID: 1})
	if err != nil {
		t.Fatal(err)
	}

	if err := uc.Delete(context.Background(), entry.ID); err != nil {
		t.Fatalf("删除应成功，实际 %v", err)
	}
	if _, err := repo.GetEntry(context.Background(), entry.ID); !errors.Is(err, repository.ErrKnowledgeEntryNotFound) {
		t.Error("删除后条目应不存在")
	}
	// 修订随条目级联删除（真实仓储由迁移的 ON DELETE CASCADE 保证）。
	if len(repo.revisions[entry.ID]) != 0 {
		t.Error("删除条目应一并清理修订")
	}
}

func TestDeleteKnowledgeEntry_NotFound(t *testing.T) {
	uc, _ := newKnowledgeUseCase(t)
	if err := uc.Delete(context.Background(), 999); !errors.Is(err, repository.ErrKnowledgeEntryNotFound) {
		t.Fatalf("不存在的条目应返回 ErrKnowledgeEntryNotFound，实际 %v", err)
	}
}
