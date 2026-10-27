package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

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

// httpStudioMemberRepo 只实现被用到的仓储方法。
type httpStudioMemberRepo struct {
	repository.StudioMemberRepo

	members map[int]*domain.StudioMember
	nextID  int
}

func newHTTPStudioMemberRepo() *httpStudioMemberRepo {
	return &httpStudioMemberRepo{members: make(map[int]*domain.StudioMember), nextID: 1}
}

func (s *httpStudioMemberRepo) Create(_ context.Context, member *domain.StudioMember) error {
	member.ID = s.nextID
	s.nextID++
	stored := *member
	s.members[member.ID] = &stored
	return nil
}

func (s *httpStudioMemberRepo) GetByID(_ context.Context, id int) (*domain.StudioMember, error) {
	if m, ok := s.members[id]; ok {
		return m, nil
	}
	return nil, repository.ErrStudioMemberNotFound
}

func (s *httpStudioMemberRepo) List(_ context.Context, _ repository.StudioMemberListFilter, _ int, _ int) ([]*domain.StudioMember, int64, error) {
	out := make([]*domain.StudioMember, 0, len(s.members))
	for _, m := range s.members {
		out = append(out, m)
	}
	return out, int64(len(out)), nil
}

func (s *httpStudioMemberRepo) Update(_ context.Context, member *domain.StudioMember) error {
	if _, ok := s.members[member.ID]; !ok {
		return repository.ErrStudioMemberNotFound
	}
	stored := *member
	s.members[member.ID] = &stored
	return nil
}

func (s *httpStudioMemberRepo) Archive(_ context.Context, id int, _ time.Time) error {
	m, ok := s.members[id]
	if !ok {
		return repository.ErrStudioMemberNotFound
	}
	if m.ArchivedAt != nil {
		return repository.ErrStudioMemberArchived
	}
	stamp := time.Unix(0, 0)
	m.ArchivedAt = &stamp
	m.IsActive = false
	return nil
}

// httpStudioAccounts 满足 StudioMemberAccountDirectory：只有 7 号账号存在。
type httpStudioAccounts struct{}

func (httpStudioAccounts) GetByID(_ context.Context, id int) (*domain.User, error) {
	if id == 7 {
		return &domain.User{ID: 7, Name: "老成员"}, nil
	}
	return nil, repository.ErrUserNotFound
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

// newStudioMemberTestEngine 组装与生产同一套中间件的成员路由。
//
// 刻意复用 middleware.AuthRequired / RequireAdmin 而不是手写桩中间件：
// 「非 admin 调用返回 403」必须验证真实鉴权链，否则测的是我自己的桩。
func newStudioMemberTestEngine(t *testing.T, admin bool) (*gin.Engine, *httpStudioMemberRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newHTTPStudioMemberRepo()
	uc := service.NewStudioMemberUseCase(repo, httpStudioAccounts{})
	h := NewStudioMemberHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	requireAdmin := middleware.RequireAdmin(stubAdminChecker{admin: admin})

	adminGroup := r.Group("/api/v1/admin", authRequired, requireAdmin)
	studio := adminGroup.Group("/studio")
	studio.GET("/members", h.ListStudioMembers)
	studio.POST("/members", h.CreateStudioMember)
	studio.GET("/members/:id", h.GetStudioMember)
	studio.PUT("/members/:id", h.UpdateStudioMember)
	studio.POST("/members/:id/archive", h.ArchiveStudioMember)
	return r, repo
}

const validMemberBody = `{"name":"张三","birth_year":2003,"cohort":"2024 级","direction":"front-end","role_in_studio":"前端负责人"}`

/* ------------------------------------------------------------------ *
 * 验收项：非 admin 调用返回 403
 * ------------------------------------------------------------------ */

func TestStudioMembers_NonAdminForbidden(t *testing.T) {
	r, repo := newStudioMemberTestEngine(t, false)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/studio/members", ""},
		{http.MethodPost, "/api/v1/admin/studio/members", validMemberBody},
		{http.MethodGet, "/api/v1/admin/studio/members/1", ""},
		{http.MethodPut, "/api/v1/admin/studio/members/1", `{"name":"李四"}`},
		{http.MethodPost, "/api/v1/admin/studio/members/1/archive", ""},
	}

	for _, tc := range cases {
		w := doJSON(r, tc.method, tc.path, tc.body, "good-token")
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s 非管理员应 403，实际 %d，body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if len(repo.members) != 0 {
		t.Error("非管理员请求不应写库")
	}
}

func TestStudioMembers_UnauthenticatedRejected(t *testing.T) {
	r, _ := newStudioMemberTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/members", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：创建（含无账号成员）
 * ------------------------------------------------------------------ */

func TestCreateStudioMember_WithoutAccount(t *testing.T) {
	r, _ := newStudioMemberTestEngine(t, true)

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/members", validMemberBody, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("创建应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Code int                       `json:"code"`
		Data response.StudioMemberResp `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 {
		t.Fatalf("业务码应为 0，实际 %d", body.Code)
	}
	// user_id 缺省必须是 null：前端靠它区分「有账号」与「校友/外部人员」。
	if body.Data.UserID != nil {
		t.Errorf("未传 user_id 时应为 null，实际 %v", *body.Data.UserID)
	}
	if !body.Data.IsActive {
		t.Error("新建成员应是在读状态")
	}
}

func TestCreateStudioMember_InvalidBody(t *testing.T) {
	r, repo := newStudioMemberTestEngine(t, true)

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/members", `{"name":"张三"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺必填字段应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.members) != 0 {
		t.Error("校验失败不应写库")
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：列表 / 详情 / 更新 / 归档主路径
 * ------------------------------------------------------------------ */

func TestStudioMemberLifecycle(t *testing.T) {
	r, _ := newStudioMemberTestEngine(t, true)

	// 创建
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/members", validMemberBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d: %s", w.Code, w.Body.String())
	}

	// 列表
	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/members", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("列表失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"total":1`, "张三") {
		t.Errorf("列表应含刚创建的成员，实际 %s", w.Body.String())
	}

	// 详情
	w = doJSON(r, http.MethodGet, "/api/v1/admin/studio/members/1", "", "good-token")
	if w.Code != http.StatusOK || !containsAll(w.Body.String(), "2024 级", "前端负责人") {
		t.Fatalf("详情应返回完整档案，实际 %d %s", w.Code, w.Body.String())
	}

	// 更新（补丁）
	w = doJSON(r, http.MethodPut, "/api/v1/admin/studio/members/1", `{"role_in_studio":"前端负责人 / 招新"}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("更新失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "前端负责人 / 招新", "张三") {
		t.Errorf("更新应只改传入字段，实际 %s", w.Body.String())
	}

	// 归档
	w = doJSON(r, http.MethodPost, "/api/v1/admin/studio/members/1/archive", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("归档失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"is_active":false`) {
		t.Errorf("归档后 is_active 应为 false，实际 %s", w.Body.String())
	}

	// 重复归档：明确业务错误（400），不是 500
	w = doJSON(r, http.MethodPost, "/api/v1/admin/studio/members/1/archive", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("重复归档应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "已归档") {
		t.Errorf("重复归档应给出可读原因，实际 %s", w.Body.String())
	}
}

// TestUpdateStudioMember_UnbindUserWithExplicitNull 锁住「传 null = 解除绑定」。
//
// 这是最容易被静默丢掉的一条语义：用 **int 绑定会把 null 与「键没出现」
// 解成同一个 nil，界面上点了解除、刷新后绑定还在。这里从 HTTP 层锁住它。
func TestUpdateStudioMember_UnbindUserWithExplicitNull(t *testing.T) {
	r, repo := newStudioMemberTestEngine(t, true)

	createBody := `{"name":"张三","birth_year":2003,"cohort":"2024 级","direction":"front-end","role_in_studio":"前端负责人","user_id":7}`
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/members", createBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d: %s", w.Code, w.Body.String())
	}
	if repo.members[1].UserID == nil || *repo.members[1].UserID != 7 {
		t.Fatalf("应已绑定 7 号账号，实际 %v", repo.members[1].UserID)
	}

	// 显式 null：解除绑定。
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/members/1", `{"user_id":null}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("解除绑定应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if repo.members[1].UserID != nil {
		t.Fatalf("传 null 应解除绑定，实际仍绑定 %v", *repo.members[1].UserID)
	}
}

// TestUpdateStudioMember_AbsentUserIDKeepsBinding 锁住「键不出现 = 不改绑定」。
func TestUpdateStudioMember_AbsentUserIDKeepsBinding(t *testing.T) {
	r, repo := newStudioMemberTestEngine(t, true)

	createBody := `{"name":"张三","birth_year":2003,"cohort":"2024 级","direction":"front-end","role_in_studio":"前端负责人","user_id":7}`
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/members", createBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d: %s", w.Code, w.Body.String())
	}

	// 只改职能：user_id 键不出现，绑定必须保留。
	w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/members/1", `{"role_in_studio":"成员"}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("更新失败 %d: %s", w.Code, w.Body.String())
	}
	if repo.members[1].UserID == nil || *repo.members[1].UserID != 7 {
		t.Fatalf("键不出现时不应动绑定，实际 %v", repo.members[1].UserID)
	}
}

func TestStudioMember_NotFound(t *testing.T) {
	r, _ := newStudioMemberTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/members/999", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在的成员应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "不存在") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
}

func TestStudioMember_InvalidID(t *testing.T) {
	r, _ := newStudioMemberTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/members/abc", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非数字 id 应 400，实际 %d", w.Code)
	}
}

func TestStudioMember_InvalidUserBinding(t *testing.T) {
	r, repo := newStudioMemberTestEngine(t, true)

	// 404 号账号不存在：必须被拒绝，不能建出一条指向幽灵账号的档案。
	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/members",
		`{"name":"张三","birth_year":2003,"cohort":"2024 级","direction":"front-end","role_in_studio":"前端负责人","user_id":404}`,
		"good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在的账号应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.members) != 0 {
		t.Error("账号校验失败不应写库")
	}
}

// containsAll 断言响应体同时包含全部片段。
func containsAll(body string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			return false
		}
	}
	return true
}
