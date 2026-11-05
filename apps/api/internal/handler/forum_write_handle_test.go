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

// newForumWriteEngine 组装带写入端点的招新问答路由（当前用户 userID=1）。
func newForumWriteEngine(repo *stubForumWriteRepo, adminDir stubAdminDir) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := service.NewForumUseCase(repo, adminDir)
	h := NewForumHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	forum := r.Group("/api/v1/forum", authRequired)
	forum.POST("/posts", func(c *gin.Context) { h.CreateForumPost(c, domain.ForumScopeRecruitment) })
	forum.PATCH("/posts/:id", func(c *gin.Context) { h.UpdateForumPost(c, domain.ForumScopeRecruitment) })
	forum.DELETE("/posts/:id", func(c *gin.Context) { h.DeleteForumPost(c, domain.ForumScopeRecruitment) })
	forum.POST("/posts/:id/replies", func(c *gin.Context) { h.CreateForumReply(c, domain.ForumScopeRecruitment) })
	forum.PATCH("/replies/:id", func(c *gin.Context) { h.UpdateForumReply(c, domain.ForumScopeRecruitment) })
	forum.DELETE("/replies/:id", func(c *gin.Context) { h.DeleteForumReply(c, domain.ForumScopeRecruitment) })
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
func TestForumCreateReplyOnMissingPostIs404(t *testing.T) {
	repo := &stubForumWriteRepo{}
	repo.stubForumRepo.postErr = service.ErrForumPostNotFound
	r := newForumWriteEngine(repo, stubAdminDir{})

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
