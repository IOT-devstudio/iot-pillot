package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// stubForumRepo 记录列表调用参数，便于断言「筛选组合 / 分页边界」落到 repo。
type stubForumRepo struct {
	repository.ForumRepo
	post       *domain.ForumPost
	postErr    error
	replies    []*domain.ForumReply
	listFilter repository.ForumPostListFilter
	listPage   int
	listSize   int
}

func (s *stubForumRepo) GetPost(_ context.Context, _ int) (*domain.ForumPost, error) {
	return s.post, s.postErr
}

func (s *stubForumRepo) ListReplies(_ context.Context, _ int) ([]*domain.ForumReply, error) {
	return s.replies, nil
}

func (s *stubForumRepo) ListPosts(_ context.Context, _ domain.ForumScope, f repository.ForumPostListFilter, page, pageSize int) ([]*repository.ForumPostSummary, int64, error) {
	s.listFilter = f
	s.listPage = page
	s.listSize = pageSize
	// 列表只用元信息，正文无需返回。
	sum := &repository.ForumPostSummary{ID: 1, Scope: "recruitment", Title: "t", AuthorName: "u"}
	return []*repository.ForumPostSummary{sum}, 1, nil
}

// stubAdminDir 满足 utils.AdminDirectory（handler 包内独立桩，避免跨包依赖）。
type stubAdminDir struct {
	ids []int
}

func (s stubAdminDir) IsAdmin(_ context.Context, id int) (bool, error) {
	for _, v := range s.ids {
		if v == id {
			return true, nil
		}
	}
	return false, nil
}
func (s stubAdminDir) AddAdmin(_ context.Context, _ int) error    { return nil }
func (s stubAdminDir) RemoveAdmin(_ context.Context, _ int) error { return nil }
func (s stubAdminDir) ListAdmins(_ context.Context) ([]int, error) { return s.ids, nil }
func (s stubAdminDir) IsFounding(_ context.Context, _ int) (bool, error)  { return false, nil }
func (s stubAdminDir) AddFounding(_ context.Context, _ int) error        { return nil }
func (s stubAdminDir) RemoveFounding(_ context.Context, _ int) error     { return nil }

// newForumEngine 组装招新问答（recruitment）路由，与生产 router.go 的 Group 一致。
func newForumEngine(repo *stubForumRepo, adminDir stubAdminDir) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := service.NewForumUseCase(repo, adminDir)
	h := NewForumHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	forum := r.Group("/api/v1/forum", authRequired)
	forum.GET("/posts", func(c *gin.Context) { h.ListForumPosts(c, domain.ForumScopeRecruitment) })
	forum.GET("/posts/:id", func(c *gin.Context) { h.GetForumPost(c, domain.ForumScopeRecruitment) })
	return r
}

// newStudioForumEngine 组装内部论坛（studio）路由，非 admin 必须拿到 404（而非 403）。
// 与生产 router.go 的 /studio/forum 闭包一致：只挂 authRequired，角色判定在闭包里。
func newStudioForumEngine(repo *stubForumRepo, adminDir stubAdminDir) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := service.NewForumUseCase(repo, adminDir)
	h := NewForumHandler(uc)

	r := gin.New()
	// 第二个 validator 是「成员」角色（userID=2, role=member），用于验证非 admin 访问。
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 2, role: middleware.RoleMember})
	studioForum := r.Group("/api/v1/studio/forum", authRequired)
	studioForum.GET("/posts", func(c *gin.Context) {
		if _, _, role, ok := middleware.CurrentUser(c); !ok || role != middleware.RoleAdmin {
			response.FailNotFound(c, "帖子不存在")
			return
		}
		h.ListForumPosts(c, domain.ForumScopeStudio)
	})
	studioForum.GET("/posts/:id", func(c *gin.Context) {
		if _, _, role, ok := middleware.CurrentUser(c); !ok || role != middleware.RoleAdmin {
			response.FailNotFound(c, "帖子不存在")
			return
		}
		h.GetForumPost(c, domain.ForumScopeStudio)
	})
	return r
}

func doForumReq(t *testing.T, r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestForumListRequiresAuth 未登录（无 token）被拦成 401。
func TestForumListRequiresAuth(t *testing.T) {
	r := newForumEngine(&stubForumRepo{}, stubAdminDir{})
	w := doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}

// TestForumDetailSoftDeletedIs404 软删帖对所有人不可见 → 404 而非 403。
func TestForumDetailSoftDeletedIs404(t *testing.T) {
	now := time.Now()
	repo := &stubForumRepo{post: &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment, DeletedAt: &now}}
	r := newForumEngine(repo, stubAdminDir{})
	w := doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts/1", "good-token")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for soft-deleted post, got %d", w.Code)
	}
}

// TestForumDetailCrossScopeIs404 跨域直访（拿 recruitment 路由打 studio 帖）返回 404 而非 403。
func TestForumDetailCrossScopeIs404(t *testing.T) {
	repo := &stubForumRepo{post: &domain.ForumPost{ID: 1, Scope: domain.ForumScopeStudio}}
	r := newForumEngine(repo, stubAdminDir{})
	w := doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts/1", "good-token")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-scope access, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestForumDetailOfficialPinned 详情响应里官方回答（管理员回复）置顶，且 is_official 标记正确。
func TestForumDetailOfficialPinned(t *testing.T) {
	base := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	repo := &stubForumRepo{
		post: &domain.ForumPost{ID: 1, Scope: domain.ForumScopeRecruitment},
		replies: []*domain.ForumReply{
			{ID: 1, AuthorID: 5, CreatedAt: base.Add(time.Minute)},
			{ID: 2, AuthorID: 2, CreatedAt: base},
		},
	}
	r := newForumEngine(repo, stubAdminDir{ids: []int{2}})
	w := doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts/1", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Replies []struct {
				ID         int  `json:"id"`
				IsOfficial bool `json:"is_official"`
			} `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data.Replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(body.Data.Replies))
	}
	if body.Data.Replies[0].ID != 2 || !body.Data.Replies[0].IsOfficial {
		t.Fatalf("expected admin reply id=2 pinned & official, got %+v", body.Data.Replies[0])
	}
	if body.Data.Replies[1].ID != 1 || body.Data.Replies[1].IsOfficial {
		t.Fatalf("expected non-official reply id=1 after, got %+v", body.Data.Replies[1])
	}
}

// TestForumListQueryParsing 列表参数（关键词/方向/分页）正确落到 repo。
func TestForumListQueryParsing(t *testing.T) {
	repo := &stubForumRepo{}
	r := newForumEngine(repo, stubAdminDir{})
	w := doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts?q=hello%25world&direction=front-end&page=2&page_size=10", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.listFilter.Query != "hello%world" {
		t.Fatalf("expected raw query preserved before ILIKE escape, got %q", repo.listFilter.Query)
	}
	if repo.listFilter.DirectionCode == nil || *repo.listFilter.DirectionCode != "front-end" {
		t.Fatalf("expected direction=front-end, got %v", repo.listFilter.DirectionCode)
	}
	if repo.listPage != 2 || repo.listSize != 10 {
		t.Fatalf("expected page=2 size=10, got page=%d size=%d", repo.listPage, repo.listSize)
	}
}

// TestForumListDirectionAllLeavesNull 不传 direction 时按「全部」（不加方向条件）。
func TestForumListDirectionAllLeavesNull(t *testing.T) {
	repo := &stubForumRepo{}
	r := newForumEngine(repo, stubAdminDir{})
	doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts", "good-token")
	if repo.listFilter.DirectionCode != nil {
		t.Fatalf("expected nil direction (all) when param absent, got %v", repo.listFilter.DirectionCode)
	}
}

// TestStudioForumNonAdminIs404 非管理员访问内部论坛 → 404（不是 403），防枚举规模。
func TestStudioForumNonAdminIs404(t *testing.T) {
	repo := &stubForumRepo{}
	r := newStudioForumEngine(repo, stubAdminDir{})
	w := doForumReq(t, r, http.MethodGet, "/api/v1/studio/forum/posts", "good-token")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-admin studio forum, got %d (body=%s)", w.Code, w.Body.String())
	}
	w2 := doForumReq(t, r, http.MethodGet, "/api/v1/studio/forum/posts/1", "good-token")
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-admin studio forum detail, got %d", w2.Code)
	}
}

// TestStudioForumAdminAllowed 管理员访问内部论坛正常进入 handler（scope=studio 落到 repo）。
func TestStudioForumAdminAllowed(t *testing.T) {
	repo := &stubForumRepo{}
	uc := service.NewForumUseCase(repo, stubAdminDir{})
	h := NewForumHandler(uc)
	gin.SetMode(gin.TestMode)
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1, role: middleware.RoleAdmin})
	adminEngine := gin.New()
	// 把 authRequired 挂上去，让 CurrentUser 能读到 admin 角色。
	adminEngine.Use(authRequired)
	adminEngine.GET("/api/v1/studio/forum/posts", func(c *gin.Context) {
		if _, _, role, ok := middleware.CurrentUser(c); !ok || role != middleware.RoleAdmin {
			response.FailNotFound(c, "帖子不存在")
			return
		}
		h.ListForumPosts(c, domain.ForumScopeStudio)
	})
	w := doForumReq(t, adminEngine, http.MethodGet, "/api/v1/studio/forum/posts", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin studio forum, got %d (body=%s)", w.Code, w.Body.String())
	}
}

// TestForumListPageBoundaries 脏分页参数被钳制而非 400：
// page<=0 → 1；page_size<=0 → 1（落到 min 下限）；page_size>100 → 100（上限）。
func TestForumListPageBoundaries(t *testing.T) {
	repo := &stubForumRepo{}
	r := newForumEngine(repo, stubAdminDir{})

	doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts?page=0&page_size=0", "good-token")
	if repo.listPage != 1 || repo.listSize != 1 {
		t.Fatalf("expected page=1 size=1 after zeroes, got page=%d size=%d", repo.listPage, repo.listSize)
	}

	doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts?page=-3&page_size=999", "good-token")
	if repo.listPage != 1 || repo.listSize != 100 {
		t.Fatalf("expected page=1 size=100 after negatives/oversize, got page=%d size=%d", repo.listPage, repo.listSize)
	}
}

// TestForumListPercentIsEscaped 关键词里的 % / _ 原样进 repo（handler 不转义，repo 的 escapeLike 负责）。
// 这里只验证 handler 把原始字符串透传给 repo；LIKE 通配符的转义归 repository 层单测/集成测。
func TestForumListPercentIsEscaped(t *testing.T) {
	repo := &stubForumRepo{}
	r := newForumEngine(repo, stubAdminDir{})
	doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts?q=50%25off", "good-token")
	if repo.listFilter.Query != "50%off" {
		t.Fatalf("expected raw '50%%off' preserved to repo, got %q", repo.listFilter.Query)
	}
}

// TestForumListQueryDirectionCombo 关键词 + 方向同时传入都落到 repo。
func TestForumListQueryDirectionCombo(t *testing.T) {
	repo := &stubForumRepo{}
	r := newForumEngine(repo, stubAdminDir{})
	doForumReq(t, r, http.MethodGet, "/api/v1/forum/posts?q=编译&direction=back-end", "good-token")
	if repo.listFilter.Query != "编译" {
		t.Fatalf("expected query '编译', got %q", repo.listFilter.Query)
	}
	if repo.listFilter.DirectionCode == nil || *repo.listFilter.DirectionCode != "back-end" {
		t.Fatalf("expected direction=back-end, got %v", repo.listFilter.DirectionCode)
	}
}
