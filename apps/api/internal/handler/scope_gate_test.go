package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 0012 自助解冻：受限令牌（profile_only）的权限边界
 *
 * 这组测试盯的是本改动最危险的一点：受限令牌**必须**被业务端点拒绝。
 * 漏一个端点就是一次静默越权——没有报错、没有日志，
 * 一个资料不全的冻结账号能照常用所有功能。
 * ------------------------------------------------------------------ */

// newScopeGateEngine 组装一个只验证「门槛」的最小路由表。
//
// 用真实的 AuthRequired + RequireFullScope，但不挂真实 handler：
// 这里要验的是中间件组合的判定，不是各 handler 的业务逻辑。
func newScopeGateEngine(scope string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	validator := stubValidator{accept: "restricted-token", userID: 7, role: middleware.RoleMember, scope: scope}

	authRequired := middleware.AuthRequired(validator)
	requireFullScope := middleware.RequireFullScope()

	r := gin.New()
	// 白名单：只要求登录，受限令牌也能过。
	r.GET("/api/v1/me", authRequired, func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.PUT("/api/v1/me", authRequired, func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.POST("/api/v1/me/unfreeze", authRequired, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 业务端点：登录 + 完整权限。
	biz := r.Group("/api/v1", authRequired, requireFullScope)
	biz.GET("/forum/posts", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	biz.POST("/forum/posts", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	biz.GET("/learning/roadmaps", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	biz.GET("/attachments/1", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	biz.GET("/admin/users", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	biz.POST("/change-password", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	biz.DELETE("/me", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func doScopeReq(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer restricted-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestRestrictedToken_WhitelistAllowed 补资料闭环的三个端点必须放行。
//
// 它们被拒就等于把用户锁死：拿不到资料、也就永远解不了冻。
func TestRestrictedToken_WhitelistAllowed(t *testing.T) {
	r := newScopeGateEngine(utils.ScopeProfileOnly)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodPut, "/api/v1/me"},
		{http.MethodPost, "/api/v1/me/unfreeze"},
	}
	for _, tc := range cases {
		w := doScopeReq(r, tc.method, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("%s %s 应放行受限令牌，实际 %d", tc.method, tc.path, w.Code)
		}
	}
}

// TestRestrictedToken_BusinessEndpointsForbidden 业务端点必须逐个拒绝。
//
// 逐个列举而不是抽查一个：漏一个就是一次静默越权。
func TestRestrictedToken_BusinessEndpointsForbidden(t *testing.T) {
	r := newScopeGateEngine(utils.ScopeProfileOnly)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/forum/posts"},
		{http.MethodPost, "/api/v1/forum/posts"},
		{http.MethodGet, "/api/v1/learning/roadmaps"},
		{http.MethodGet, "/api/v1/attachments/1"},
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPost, "/api/v1/change-password"},
		{http.MethodDelete, "/api/v1/me"},
	}
	for _, tc := range cases {
		w := doScopeReq(r, tc.method, tc.path)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s 应拒绝受限令牌（403），实际 %d", tc.method, tc.path, w.Code)
		}
	}
}

// TestFullScope_BusinessEndpointsAllowed 完整令牌不受影响。
//
// 防止「加门槛把所有人都拦了」这种过度收紧。
func TestFullScope_BusinessEndpointsAllowed(t *testing.T) {
	r := newScopeGateEngine(utils.ScopeFull)

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/forum/posts"},
		{http.MethodGet, "/api/v1/learning/roadmaps"},
		{http.MethodGet, "/api/v1/attachments/1"},
	}
	for _, tc := range cases {
		w := doScopeReq(r, tc.method, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("完整令牌访问 %s %s 应放行，实际 %d", tc.method, tc.path, w.Code)
		}
	}
}

// TestMissingScope_FailsClosed 上下文里没有 scope 时按拒绝处理。
//
// fail-closed 的意义：忘记给新路由挂门槛时，结果是「被拒」而不是「开门」。
func TestMissingScope_FailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 只挂 AuthRequired 与 RequireFullScope，但用不写 scope 的校验器模拟「缺失」。
	r.GET("/biz", func(c *gin.Context) {
		c.Set(middleware.ContextUserID, 7)
		c.Set(middleware.ContextUsername, "tester")
		c.Set(middleware.ContextRole, middleware.RoleMember)
		// 刻意不设 ContextScope
	}, middleware.RequireFullScope(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/biz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("scope 缺失时应拒绝（fail-closed），实际 %d", w.Code)
	}
}

// TestRequireFullScope_WithoutAuthIs401 没经过 AuthRequired 时按 401 而不是放行。
func TestRequireFullScope_WithoutAuthIs401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/biz", middleware.RequireFullScope(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/biz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("缺少认证信息应 401，实际 %d", w.Code)
	}
}

// scopeErrorValidator 让校验直接失败，验证错误分支不被 scope 逻辑吞掉。
type scopeErrorValidator struct{ err error }

func (v scopeErrorValidator) ValidateAccessToken(_ context.Context, _ string) (int, string, string, error) {
	return 0, "", "", v.err
}

func (v scopeErrorValidator) ValidateAccessTokenWithScope(_ context.Context, _ string) (int, string, string, string, error) {
	return 0, "", "", "", v.err
}

// TestAuthRequired_StillRejectsExpiredWithScopeValidator 加了 scope 之后，
// 过期令牌仍必须走 4010（前端安静刷新），不能被新逻辑改变。
func TestAuthRequired_StillRejectsExpiredWithScopeValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/me", middleware.AuthRequired(scopeErrorValidator{err: utils.ErrTokenExpired}),
		func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer x")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("过期令牌应 401，实际 %d", w.Code)
	}
}
