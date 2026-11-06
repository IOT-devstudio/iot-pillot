package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 写入桩：在既有 stubForumRepo 上补写入方法
 * ------------------------------------------------------------------ */

// stubForumWriteRepo 复用 stubForumRepo 的读语义，只额外记录写入。
type stubForumWriteRepo struct {
	stubForumRepo
	createdPost  *domain.ForumPost
	updatedPost  *domain.ForumPost
	deletedPost  [2]int
	reply        *domain.ForumReply
	replyErr     error
	createdReply *domain.ForumReply
	updatedReply *domain.ForumReply
	deletedReply [2]int
	resolved     [3]any // [id, byUserID, resolved]
}

func (s *stubForumWriteRepo) CreatePost(_ context.Context, p *domain.ForumPost) error {
	p.ID = 100
	s.createdPost = p
	return nil
}

func (s *stubForumWriteRepo) UpdatePost(_ context.Context, p *domain.ForumPost) error {
	s.updatedPost = p
	return nil
}

func (s *stubForumWriteRepo) SoftDeletePost(_ context.Context, id int, by int) error {
	s.deletedPost = [2]int{id, by}
	return nil
}

func (s *stubForumWriteRepo) GetReply(_ context.Context, _ int) (*domain.ForumReply, error) {
	if s.replyErr != nil {
		return nil, s.replyErr
	}
	return s.reply, nil
}

func (s *stubForumWriteRepo) CreateReply(_ context.Context, r *domain.ForumReply) error {
	r.ID = 200
	s.createdReply = r
	return nil
}

func (s *stubForumWriteRepo) UpdateReply(_ context.Context, r *domain.ForumReply) error {
	s.updatedReply = r
	return nil
}

func (s *stubForumWriteRepo) SoftDeleteReply(_ context.Context, id int, by int) error {
	s.deletedReply = [2]int{id, by}
	return nil
}

// SetPostResolved 记录解决标记。内嵌的 ForumRepo 是零值接口，
// 不补这个方法会在调用时 panic（nil 接口调用）。
func (s *stubForumWriteRepo) SetPostResolved(_ context.Context, id int, by int, resolved bool) error {
	s.resolved = [3]any{id, by, resolved}
	return nil
}

// newForumWriteEngine 组装带写入端点的招新问答路由（当前用户 userID=1）。
//
// 回复端点与生产同形：挂管理员关（#241）。adminDir 同时喂给中间件与 service，
// 与 wire 组装一致——生产上两者读的是同一份 Redis 名单，测试里若不一致，
// 会出现「中间件放行、service 又拒」的假绿。
func newForumWriteEngine(repo *stubForumWriteRepo, adminDir stubAdminDir) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := service.NewForumUseCase(repo, adminDir)
	h := NewForumHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	requireReplyAdmin := middleware.RequireAdminWithMessage(adminDir, "只有管理员可以回答招新问答")
	forum := r.Group("/api/v1/forum", authRequired)
	forum.POST("/posts", func(c *gin.Context) { h.CreateForumPost(c, domain.ForumScopeRecruitment) })
	forum.PATCH("/posts/:id", func(c *gin.Context) { h.UpdateForumPost(c, domain.ForumScopeRecruitment) })
	forum.DELETE("/posts/:id", func(c *gin.Context) { h.DeleteForumPost(c, domain.ForumScopeRecruitment) })
	// 只有这一条挂管理员关，其余写入端点保持原样。
	forum.POST("/posts/:id/replies", requireReplyAdmin, func(c *gin.Context) { h.CreateForumReply(c, domain.ForumScopeRecruitment) })
	forum.PATCH("/replies/:id", func(c *gin.Context) { h.UpdateForumReply(c, domain.ForumScopeRecruitment) })
	forum.DELETE("/replies/:id", func(c *gin.Context) { h.DeleteForumReply(c, domain.ForumScopeRecruitment) })
	forum.POST("/posts/:id/resolve", func(c *gin.Context) { h.ResolveForumPost(c, domain.ForumScopeRecruitment) })
	return r
}

// doForumJSON 发带 JSON body 的请求。
func doForumJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

/* ------------------------------------------------------------------ *
 * 权限与认证
 * ------------------------------------------------------------------ */

// TestForumWriteRequiresAuth 六个写入端点未登录全部 401。
func TestForumWriteRequiresAuth(t *testing.T) {
	repo := &stubForumWriteRepo{}
	r := newForumWriteEngine(repo, stubAdminDir{})

	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/forum/posts"},
		{http.MethodPatch, "/api/v1/forum/posts/1"},
		{http.MethodDelete, "/api/v1/forum/posts/1"},
		{http.MethodPost, "/api/v1/forum/posts/1/replies"},
		{http.MethodPatch, "/api/v1/forum/replies/1"},
		{http.MethodDelete, "/api/v1/forum/replies/1"},
	}
	for _, tc := range cases {
		w := doForumJSON(t, r, tc.method, tc.path, "", map[string]any{"title": "t", "body": "b"})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d", tc.method, tc.path, w.Code)
		}
	}
}

// TestForumDeleteOthersPostIs404 非作者且非管理员删除他人帖子 → 404（不是 403）。
func TestForumDeleteOthersPostIs404(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.post = &domain.ForumPost{ID: 1, AuthorID: 999, Scope: domain.ForumScopeRecruitment}
	r := newForumWriteEngine(repo, stubAdminDir{})

	w := doForumJSON(t, r, http.MethodDelete, "/api/v1/forum/posts/1", "good-token", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleting others' post, got %d (body=%s)", w.Code, w.Body.String())
	}
	if repo.deletedPost != [2]int{0, 0} {
		t.Fatal("must not soft delete when unauthorized")
	}
}

/* ------------------------------------------------------------------ *
 * 成功路径与长度边界
 * ------------------------------------------------------------------ */

// TestForumCreatePostSuccess 建帖成功并把标题 trim 后落库。
func TestForumCreatePostSuccess(t *testing.T) {
	repo := &stubForumWriteRepo{}
	r := newForumWriteEngine(repo, stubAdminDir{})

	w := doForumJSON(t, r, http.MethodPost, "/api/v1/forum/posts", "good-token",
		map[string]any{"title": "  标题  ", "body": "正文"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if repo.createdPost == nil {
		t.Fatal("expected post created")
	}
	if repo.createdPost.Title != "标题" {
		t.Fatalf("expected trimmed title, got %q", repo.createdPost.Title)
	}
	if repo.createdPost.AuthorID != 1 {
		t.Fatalf("expected author from token (1), got %d", repo.createdPost.AuthorID)
	}
}

// TestForumCreatePostTooLongIs400 标题超长回 400，且 message 可读。
func TestForumCreatePostTooLongIs400(t *testing.T) {
	repo := &stubForumWriteRepo{}
	r := newForumWriteEngine(repo, stubAdminDir{})

	long := make([]rune, 201)
	for i := range long {
		long[i] = '标'
	}
	w := doForumJSON(t, r, http.MethodPost, "/api/v1/forum/posts", "good-token",
		map[string]any{"title": string(long), "body": "正文"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for over-long title, got %d (body=%s)", w.Code, w.Body.String())
	}
	var resp response.Result
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if resp.Message == "" {
			t.Fatal("expected readable message for over-long title")
		}
	}
	if repo.createdPost != nil {
		t.Fatal("must not create when validation fails")
	}
}

// TestForumCreateReplyOnMissingPostIs404 帖子不存在时回复 404。
//
// 用管理员身份：中间件先于「帖子是否存在」判定（权限是路由层的关），
// 非管理员拿到的 403 与帖子在不在无关，测不到这条 404 语义。
func TestForumCreateReplyOnMissingPostIs404(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.stubForumRepo.postErr = service.ErrForumPostNotFound
	r := newForumWriteEngine(repo, stubAdminDir{ids: []int{1}})

	w := doForumJSON(t, r, http.MethodPost, "/api/v1/forum/posts/123/replies", "good-token", map[string]any{"body": "回复"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestForumDeleteOwnPostSuccess 删除自己的帖子成功。
func TestForumDeleteOwnPostSuccess(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.post = &domain.ForumPost{ID: 1, AuthorID: 1, Scope: domain.ForumScopeRecruitment}
	r := newForumWriteEngine(repo, stubAdminDir{})

	w := doForumJSON(t, r, http.MethodDelete, "/api/v1/forum/posts/1", "good-token", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if repo.deletedPost != [2]int{1, 1} {
		t.Fatalf("expected soft delete (id=1, by=1), got %v", repo.deletedPost)
	}
}

// TestForumCrossScopeWriteIs404 拿招新路由操作 studio 帖子 → 404（不暴露跨域存在性）。
func TestForumCrossScopeWriteIs404(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.post = &domain.ForumPost{ID: 1, AuthorID: 1, Scope: domain.ForumScopeStudio}
	r := newForumWriteEngine(repo, stubAdminDir{})

	w := doForumJSON(t, r, http.MethodDelete, "/api/v1/forum/posts/1", "good-token", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-scope write, got %d (body=%s)", w.Code, w.Body.String())
	}
	if repo.deletedPost != [2]int{0, 0} {
		t.Fatal("must not write across scope")
	}
}

// TestForumStudioWriteNonAdminIs404 内部论坛写入对非 admin 一律 404。
func TestForumStudioWriteNonAdminIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &stubForumWriteRepo{}
	uc := service.NewForumUseCase(repo, stubAdminDir{})
	h := NewForumHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 2, role: middleware.RoleMember})
	studioForum := r.Group("/api/v1/studio/forum", authRequired)
	studioWrite := func(action func(*gin.Context, domain.ForumScope)) gin.HandlerFunc {
		return func(c *gin.Context) {
			if _, _, role, ok := middleware.CurrentUser(c); !ok || role != middleware.RoleAdmin {
				response.FailNotFound(c, "帖子不存在")
				return
			}
			action(c, domain.ForumScopeStudio)
		}
	}
	studioForum.POST("/posts", studioWrite(h.CreateForumPost))
	studioForum.DELETE("/posts/:id", studioWrite(h.DeleteForumPost))

	w := doForumJSON(t, r, http.MethodPost, "/api/v1/studio/forum/posts", "good-token", map[string]any{"title": "t", "body": "b"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-admin studio write, got %d", w.Code)
	}
	if repo.createdPost != nil {
		t.Fatal("must not create studio post as non-admin")
	}
}

// TestForumReplyEditSuccess 编辑自己的回复回 200 且带 edited_at。
func TestForumReplyEditSuccess(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.reply = &domain.ForumReply{ID: 1, PostID: 1, AuthorID: 1, Body: "旧"}
	r := newForumWriteEngine(repo, stubAdminDir{})

	w := doForumJSON(t, r, http.MethodPatch, "/api/v1/forum/replies/1", "good-token", map[string]any{"body": "新"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Data response.ForumReplyResp `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Body != "新" {
		t.Fatalf("expected updated body, got %q", resp.Data.Body)
	}
	if resp.Data.EditedAt == nil {
		t.Fatal("expected edited_at in response")
	}
}

// TestForumReplyDeleteOthersIs404 删除他人回复 → 404。
func TestForumReplyDeleteOthersIs404(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.reply = &domain.ForumReply{ID: 1, PostID: 1, AuthorID: 999}
	r := newForumWriteEngine(repo, stubAdminDir{})

	w := doForumJSON(t, r, http.MethodDelete, "/api/v1/forum/replies/1", "good-token", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", w.Code, w.Body.String())
	}
	if repo.deletedReply != [2]int{0, 0} {
		t.Fatal("must not delete others' reply")
	}
}

/* ------------------------------------------------------------------ *
 * 回答权限收口到管理员（#241）
 * ------------------------------------------------------------------ */

// newForumReplyEngine 组装「登录 + 管理员关」两步的真实路由，
// 与生产 router.go 里 `apiV1.POST("/forum/posts/:id/replies", ..., requireAdmin, ...)`
// 的形状一致：角色判定交给中间件，handler 不再 if-else。
//
// adminDir 同时喂给中间件（AdminChecker）与 service（ForumUseCase）：
// 生产上两者读的是同一份 Redis 名单，测试里也必须一致，
// 否则会出现「中间件放行、service 又拒」的假绿。
func newForumReplyEngine(repo *stubForumWriteRepo, userID int, admin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	adminDir := stubAdminDir{}
	if admin {
		adminDir = stubAdminDir{ids: []int{userID}}
	}
	uc := service.NewForumUseCase(repo, adminDir)
	h := NewForumHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: userID})
	// 与生产 router.go 一致：用带专用文案的管理员关。
	requireAdmin := middleware.RequireAdminWithMessage(stubAdminChecker{admin: admin}, "只有管理员可以回答招新问答")
	// 「回答仅限管理员」：只给这一条挂管理员关。
	r.POST("/api/v1/forum/posts/:id/replies", authRequired, requireAdmin,
		func(c *gin.Context) { h.CreateForumReply(c, domain.ForumScopeRecruitment) })
	// 其余端点保持「零变化」：不挂管理员关。
	forum := r.Group("/api/v1/forum", authRequired)
	forum.POST("/posts", func(c *gin.Context) { h.CreateForumPost(c, domain.ForumScopeRecruitment) })
	forum.PATCH("/posts/:id", func(c *gin.Context) { h.UpdateForumPost(c, domain.ForumScopeRecruitment) })
	forum.DELETE("/posts/:id", func(c *gin.Context) { h.DeleteForumPost(c, domain.ForumScopeRecruitment) })
	forum.PATCH("/replies/:id", func(c *gin.Context) { h.UpdateForumReply(c, domain.ForumScopeRecruitment) })
	forum.DELETE("/replies/:id", func(c *gin.Context) { h.DeleteForumReply(c, domain.ForumScopeRecruitment) })
	forum.POST("/posts/:id/resolve", func(c *gin.Context) { h.ResolveForumPost(c, domain.ForumScopeRecruitment) })
	return r
}

// TestForumCreateReplyMemberIs403 普通用户回答招新问答 → 403（不是 404）。
//
// 帖子对他可见，404 只会误导前端给出「帖子不存在」的假话；
// 与「一律 404」的分界见 ADR 0003 决策 8 的修订。
func TestForumCreateReplyMemberIs403(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.post = &domain.ForumPost{ID: 1, AuthorID: 999, Scope: domain.ForumScopeRecruitment}
	r := newForumReplyEngine(repo, 2, false)

	w := doForumJSON(t, r, http.MethodPost, "/api/v1/forum/posts/1/replies", "good-token", map[string]any{"body": "我也来答"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member reply, got %d (body=%s)", w.Code, w.Body.String())
	}
	// message 必须可读中文，且要说清「为什么」——默认的「无权限访问该资源」
	// 对可见的帖子没有信息量，spec 明确要求类似「只有管理员可以回答招新问答」。
	var resp response.Result
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Message != "只有管理员可以回答招新问答" {
		t.Fatalf("403 文案应说明原因，实际 %q", resp.Message)
	}
	if repo.createdReply != nil {
		t.Fatal("must not create reply as non-admin")
	}
}

// TestForumCreateReplyAdminIs200 管理员回答招新问答 → 200 且回复落库。
func TestForumCreateReplyAdminIs200(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.post = &domain.ForumPost{ID: 1, AuthorID: 999, Scope: domain.ForumScopeRecruitment}
	r := newForumReplyEngine(repo, 9, true)

	w := doForumJSON(t, r, http.MethodPost, "/api/v1/forum/posts/1/replies", "good-token", map[string]any{"body": "官方回答"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin reply, got %d (body=%s)", w.Code, w.Body.String())
	}
	if repo.createdReply == nil {
		t.Fatal("expected admin reply persisted")
	}
	if repo.createdReply.AuthorID != 9 || repo.createdReply.Body != "官方回答" {
		t.Fatalf("unexpected reply: %+v", repo.createdReply)
	}
}

// TestForumMemberCanStillWriteElsewhere 收口只针对「新建回复」，
// 其余写入对普通用户行为零变化。
//
// spec 逐项列出了这些端点：发帖、编辑/删除自己的帖子与回复、标记已解决。
// 这里逐条复核，而不是抽查一个——把 requireAdmin 挂错分组正是本次要防的回归。
func TestForumMemberCanStillWriteElsewhere(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.post = &domain.ForumPost{ID: 1, AuthorID: 2, Scope: domain.ForumScopeRecruitment}
	repo.reply = &domain.ForumReply{ID: 1, PostID: 1, AuthorID: 2, Body: "旧"}
	r := newForumReplyEngine(repo, 2, false)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"发帖", http.MethodPost, "/api/v1/forum/posts", map[string]any{"title": "提问", "body": "正文"}},
		{"编辑自己的帖子", http.MethodPatch, "/api/v1/forum/posts/1", map[string]any{"title": "改", "body": "正文"}},
		{"删除自己的帖子", http.MethodDelete, "/api/v1/forum/posts/1", nil},
		{"编辑自己的回复", http.MethodPatch, "/api/v1/forum/replies/1", map[string]any{"body": "新"}},
		{"删除自己的回复", http.MethodDelete, "/api/v1/forum/replies/1", nil},
		{"标记已解决", http.MethodPost, "/api/v1/forum/posts/1/resolve", map[string]any{"resolved": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doForumJSON(t, r, tc.method, tc.path, "good-token", tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("普通用户应仍可%s，实际 %d（body=%s）", tc.name, w.Code, w.Body.String())
			}
		})
	}
}
