package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 桩
 * ------------------------------------------------------------------ */

// httpKnowledgeRepo 在内存里复刻「保存 = 升版 + 写快照」的语义。
type httpKnowledgeRepo struct {
	repository.StudioKnowledgeRepo

	entries   map[int]*domain.StudioKnowledgeEntry
	revisions map[int][]*domain.StudioKnowledgeRevision
	nextID    int
	nextRevID int
}

func newHTTPKnowledgeRepo() *httpKnowledgeRepo {
	return &httpKnowledgeRepo{
		entries:   make(map[int]*domain.StudioKnowledgeEntry),
		revisions: make(map[int][]*domain.StudioKnowledgeRevision),
		nextID:    1,
		nextRevID: 100,
	}
}

func (s *httpKnowledgeRepo) CreateEntry(_ context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision) error {
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

func (s *httpKnowledgeRepo) GetEntry(_ context.Context, id int) (*domain.StudioKnowledgeEntry, error) {
	if e, ok := s.entries[id]; ok {
		return e, nil
	}
	return nil, repository.ErrKnowledgeEntryNotFound
}

func (s *httpKnowledgeRepo) ListEntries(_ context.Context, _ repository.StudioKnowledgeListFilter, _ int, _ int) ([]*domain.StudioKnowledgeEntry, int64, error) {
	out := make([]*domain.StudioKnowledgeEntry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	return out, int64(len(out)), nil
}

func (s *httpKnowledgeRepo) UpdateEntry(_ context.Context, entry *domain.StudioKnowledgeEntry, revision *domain.StudioKnowledgeRevision, expectedVersion int) error {
	current, ok := s.entries[entry.ID]
	if !ok {
		return repository.ErrKnowledgeEntryNotFound
	}
	if current.Version != expectedVersion {
		return repository.ErrKnowledgeVersionConflict
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

func (s *httpKnowledgeRepo) DeleteEntry(_ context.Context, id int) error {
	if _, ok := s.entries[id]; !ok {
		return repository.ErrKnowledgeEntryNotFound
	}
	delete(s.entries, id)
	delete(s.revisions, id)
	return nil
}

func (s *httpKnowledgeRepo) ListRevisions(_ context.Context, entryID int) ([]*domain.StudioKnowledgeRevision, error) {
	stored := s.revisions[entryID]
	out := make([]*domain.StudioKnowledgeRevision, 0, len(stored))
	for i := len(stored) - 1; i >= 0; i-- {
		out = append(out, stored[i])
	}
	return out, nil
}

func (s *httpKnowledgeRepo) GetRevision(_ context.Context, entryID int, version int) (*domain.StudioKnowledgeRevision, error) {
	for _, rev := range s.revisions[entryID] {
		if rev.Version == version {
			return rev, nil
		}
	}
	return nil, repository.ErrKnowledgeRevisionNotFound
}

func (s *httpKnowledgeRepo) IsDescendant(_ context.Context, ancestorID int, candidateID int) (bool, error) {
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

// newKnowledgeTestEngine 组装与生产同一套中间件的知识库路由。
//
// 刻意复用 middleware.AuthRequired / RequireAdmin 而不是手写桩中间件：
// 「非 admin 调用返回 403」必须验证真实鉴权链，否则测的是我自己的桩。
func newKnowledgeTestEngine(t *testing.T, admin bool) (*gin.Engine, *httpKnowledgeRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newHTTPKnowledgeRepo()
	uc := service.NewStudioKnowledgeUseCase(repo)
	h := NewStudioKnowledgeHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	requireAdmin := middleware.RequireAdmin(stubAdminChecker{admin: admin})

	group := r.Group("/api/v1/admin", authRequired, requireAdmin)
	studio := group.Group("/studio")
	studio.GET("/knowledge", h.ListStudioKnowledge)
	studio.POST("/knowledge", h.CreateStudioKnowledge)
	studio.GET("/knowledge/:id", h.GetStudioKnowledge)
	studio.PUT("/knowledge/:id", h.UpdateStudioKnowledge)
	studio.DELETE("/knowledge/:id", h.DeleteStudioKnowledge)
	studio.GET("/knowledge/:id/revisions", h.ListStudioKnowledgeRevisions)
	studio.GET("/knowledge/:id/revisions/:version", h.GetStudioKnowledgeRevision)
	return r, repo
}

const validKnowledgeBody = `{"title":"部署手册","body":"第一版正文"}`

/* ------------------------------------------------------------------ *
 * 验收项：非 admin 调用返回 403
 * ------------------------------------------------------------------ */

func TestStudioKnowledge_NonAdminForbidden(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, false)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/studio/knowledge", ""},
		{http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody},
		{http.MethodGet, "/api/v1/admin/studio/knowledge/1", ""},
		{http.MethodPut, "/api/v1/admin/studio/knowledge/1", `{"title":"新标题"}`},
		{http.MethodDelete, "/api/v1/admin/studio/knowledge/1", ""},
		{http.MethodGet, "/api/v1/admin/studio/knowledge/1/revisions", ""},
		{http.MethodGet, "/api/v1/admin/studio/knowledge/1/revisions/1", ""},
	}

	for _, tc := range cases {
		w := doJSON(r, tc.method, tc.path, tc.body, "good-token")
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s 非管理员应 403，实际 %d，body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if len(repo.entries) != 0 {
		t.Error("非管理员请求不应写库")
	}
}

func TestStudioKnowledge_UnauthenticatedRejected(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：创建条目（同时产生 v1 修订）
 * ------------------------------------------------------------------ */

func TestCreateStudioKnowledge_WritesVersionOne(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, true)

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("创建应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Code int                               `json:"code"`
		Data response.StudioKnowledgeEntryResp `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 {
		t.Fatalf("业务码应为 0，实际 %d", body.Code)
	}
	if body.Data.Version != 1 {
		t.Errorf("新条目版本应为 1，实际 %d", body.Data.Version)
	}
	// 作者取自登录身份（stubValidator 固定 userID = 1），不接受请求体指定。
	if body.Data.AuthorID != 1 {
		t.Errorf("作者应取自登录身份，实际 %d", body.Data.AuthorID)
	}
	// 缺省权限 admin（与迁移 0015 的列默认值一致）。
	if body.Data.Permission != "admin" {
		t.Errorf("缺省权限应为 admin，实际 %q", body.Data.Permission)
	}
	// v1 快照必须同时写入。
	if len(repo.revisions[1]) != 1 {
		t.Errorf("创建应写入 1 条修订，实际 %d", len(repo.revisions[1]))
	}
}

func TestCreateStudioKnowledge_InvalidBody(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, true)

	// 缺标题：binding:required 直接挡掉。
	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", `{"body":"只有正文"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺标题应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.entries) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestCreateStudioKnowledge_InvalidPermission(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, true)

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge",
		`{"title":"标题","body":"正文","permission":"everyone"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法权限应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "权限") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
	if len(repo.entries) != 0 {
		t.Error("校验失败不应写库")
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：保存升版 + 历史读取（HTTP 层闭环）
 * ------------------------------------------------------------------ */

func TestStudioKnowledge_SaveBumpsVersionAndHistory(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	// 创建
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d: %s", w.Code, w.Body.String())
	}

	// 保存（带 version=1 做乐观锁）
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/knowledge/1",
		`{"body":"第二版正文","version":1}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("保存失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"version":2`, "第二版正文") {
		t.Errorf("保存后版本应为 2，实际 %s", w.Body.String())
	}
	// 未传的标题必须保持原值。
	if !containsAll(w.Body.String(), "部署手册") {
		t.Errorf("未传的标题应保持原值，实际 %s", w.Body.String())
	}

	// 修订列表：两条，版本倒序
	w = doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge/1/revisions", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("修订列表失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"total":2`, "第一版正文", "第二版正文") {
		t.Errorf("修订列表应含两版内容，实际 %s", w.Body.String())
	}

	// 按版本取旧快照
	w = doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge/1/revisions/1", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("取 v1 失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "第一版正文") {
		t.Errorf("v1 应为初始内容，实际 %s", w.Body.String())
	}
}

func TestStudioKnowledge_VersionConflictReturns400(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d", w.Code)
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/knowledge/1", `{"body":"a","version":1}`, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("首次保存失败 %d: %s", w.Code, w.Body.String())
	}

	// 另一个人还拿着 version=1：必须冲突，不能静默覆盖。
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/knowledge/1", `{"body":"b","version":1}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("旧版本号保存应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "刷新") {
		t.Errorf("冲突应提示刷新后重试，实际 %s", w.Body.String())
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：可选 parent 树，非法 parent 有明确错误
 * ------------------------------------------------------------------ */

func TestStudioKnowledge_ParentTree(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建父条目失败 %d", w.Code)
	}

	// 挂到存在的父条目下
	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge",
		`{"title":"子条目","body":"子正文","parent_id":1}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("挂到父条目应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"parent_id":1`) {
		t.Errorf("parent_id 应回显 1，实际 %s", w.Body.String())
	}

	// 不存在的父条目：明确 400，不是 500
	w = doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge",
		`{"title":"孤儿","body":"x","parent_id":999}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在的父条目应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "父条目") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
}

// TestStudioKnowledge_ParentIdNullMovesToTopLevel 锁住「传 null = 移到顶层」。
//
// 这是最容易被静默丢掉的一条语义：用 **int 绑定会把 null 与「键没出现」
// 解成同一个 nil，「移到顶层」于是被当成「不改」。
func TestStudioKnowledge_ParentIdNullMovesToTopLevel(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建父条目失败 %d", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge",
		`{"title":"子条目","body":"子正文","parent_id":1}`, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建子条目失败 %d", w.Code)
	}
	if repo.entries[2].ParentID == nil {
		t.Fatal("子条目应有 parent_id")
	}

	// 显式 null：移到顶层。
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/knowledge/2", `{"parent_id":null,"version":1}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("移到顶层应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if repo.entries[2].ParentID != nil {
		t.Errorf("传 null 应移到顶层，实际 parent_id=%v", *repo.entries[2].ParentID)
	}
}

// TestStudioKnowledge_AbsentParentIdKeepsParent 锁住「键不出现 = 不改父条目」。
func TestStudioKnowledge_AbsentParentIdKeepsParent(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建父条目失败 %d", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge",
		`{"title":"子条目","body":"子正文","parent_id":1}`, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建子条目失败 %d", w.Code)
	}

	// 只改正文：parent_id 键不出现，父子关系必须保留。
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/knowledge/2", `{"body":"新正文","version":1}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("保存失败 %d，body=%s", w.Code, w.Body.String())
	}
	if repo.entries[2].ParentID == nil || *repo.entries[2].ParentID != 1 {
		t.Errorf("键不出现时不应动父子关系，实际 %v", repo.entries[2].ParentID)
	}
}

func TestStudioKnowledge_ParentCycleRejected(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建根条目失败 %d", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge",
		`{"title":"子条目","body":"子正文","parent_id":1}`, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建子条目失败 %d", w.Code)
	}

	// 把根挂到自己的子条目下 → 成环，必须拒绝。
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/knowledge/1", `{"parent_id":2,"version":1}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("成环应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "父条目") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
}

/* ------------------------------------------------------------------ *
 * 删除
 * ------------------------------------------------------------------ */

func TestStudioKnowledge_Delete(t *testing.T) {
	r, repo := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d", w.Code)
	}

	w := doJSON(r, http.MethodDelete, "/api/v1/admin/studio/knowledge/1", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("删除应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.entries) != 0 {
		t.Error("删除后条目应不存在")
	}
	// 修订随条目级联删除。
	if len(repo.revisions[1]) != 0 {
		t.Error("删除条目应一并清理修订")
	}

	// 重复删除：明确 400，不是 500
	w = doJSON(r, http.MethodDelete, "/api/v1/admin/studio/knowledge/1", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("重复删除应 400，实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 详情 / 错误分支
 * ------------------------------------------------------------------ */

func TestStudioKnowledge_NotFound(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge/999", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在的条目应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "不存在") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
}

func TestStudioKnowledge_InvalidID(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge/abc", "", "good-token"); w.Code != http.StatusBadRequest {
		t.Errorf("非数字 id 应 400，实际 %d", w.Code)
	}
	if w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge/1/revisions/abc", "", "good-token"); w.Code != http.StatusBadRequest {
		t.Errorf("非数字 version 应 400，实际 %d", w.Code)
	}
}

func TestStudioKnowledge_RevisionNotFound(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/knowledge", validKnowledgeBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d", w.Code)
	}

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge/1/revisions/99", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在的版本应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestStudioKnowledge_ListReturnsEmptyArray(t *testing.T) {
	r, _ := newKnowledgeTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/knowledge", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("空列表应 200，实际 %d", w.Code)
	}
	// 空列表必须是 []，不能是 null——前端对 null 做 .map 会直接抛错。
	if !containsAll(w.Body.String(), `"items":[]`) {
		t.Errorf("空列表应序列化成 []，实际 %s", w.Body.String())
	}
}
