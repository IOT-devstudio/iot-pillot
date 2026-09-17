package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// stubValidator 替代 *utils.TokenManager。
//
// 真实实现要连 Redis 做会话唯一码比对，测试里连不上也没必要连：
// 中间件只依赖 TokenValidator 这一个方法，注入桩就能把 401/403/放行三条分支
// 全覆盖。这也是把依赖收窄成接口（而不是具体 TokenManager）的直接收益。
type stubValidator struct {
	userID   int
	username string
	role     string
	err      error
	calls    int
}

func (s *stubValidator) ValidateAccessToken(_ context.Context, _ string) (int, string, string, error) {
	s.calls++
	return s.userID, s.username, s.role, s.err
}

// newTestRouter 复刻 router.go 里管理端那条中间件链：
// AuthRequired 在前提供身份，RequireRole 在后判定角色。
func newTestRouter(validator TokenValidator, role string, reached *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	protected := r.Group("/protected", AuthRequired(validator), RequireRole(role))
	protected.GET("", func(c *gin.Context) {
		*reached = true
		userID, username, currentRole, ok := CurrentUser(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "缺少身份信息"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"role":     currentRole,
		})
	})

	return r
}

func doRequest(r *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	return recorder
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (body=%s)", err, recorder.Body.String())
	}
	return body
}

func TestAuthRequired_RejectsMissingHeader(t *testing.T) {
	reached := false
	validator := &stubValidator{role: RoleAdmin}
	r := newTestRouter(validator, RoleAdmin, &reached)

	recorder := doRequest(r, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("缺少 Authorization 时状态码 = %d，期望 401", recorder.Code)
	}
	if reached {
		t.Error("未认证的请求不应该到达业务 handler")
	}
	if validator.calls != 0 {
		t.Errorf("缺少请求头时不应该调用令牌校验，实际调用了 %d 次", validator.calls)
	}
}

func TestAuthRequired_RejectsMalformedHeader(t *testing.T) {
	reached := false
	r := newTestRouter(&stubValidator{role: RoleAdmin}, RoleAdmin, &reached)

	// 少了 Bearer 前缀：常见的手写请求错误，必须当成未认证
	recorder := doRequest(r, "some-token")

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("Authorization 格式错误时状态码 = %d，期望 401", recorder.Code)
	}
	if reached {
		t.Error("格式错误的令牌不应该到达业务 handler")
	}
}

func TestAuthRequired_RejectsInvalidToken(t *testing.T) {
	reached := false
	validator := &stubValidator{err: errors.New("token 已过期")}
	r := newTestRouter(validator, RoleAdmin, &reached)

	recorder := doRequest(r, "Bearer expired-token")

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("令牌无效时状态码 = %d，期望 401", recorder.Code)
	}
	if reached {
		t.Error("令牌无效的请求不应该到达业务 handler")
	}
}

// 核心用例：普通成员访问管理端必须 403，而不是悄悄放行。
func TestRequireRole_RejectsMember(t *testing.T) {
	reached := false
	validator := &stubValidator{userID: 7, username: "member_user", role: RoleMember}
	r := newTestRouter(validator, RoleAdmin, &reached)

	recorder := doRequest(r, "Bearer member-token")

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("成员访问管理端状态码 = %d，期望 403", recorder.Code)
	}
	if reached {
		t.Error("成员不应该到达管理端 handler")
	}

	body := decodeBody(t, recorder)
	if body["code"] != float64(4003) {
		t.Errorf("业务码 = %v，期望 4003（无权限）", body["code"])
	}
	// 不回显所需角色，避免向普通成员暴露管理端结构
	if message, _ := body["message"].(string); message == "" || message == RoleAdmin {
		t.Errorf("错误信息不应暴露所需角色，实际为 %q", message)
	}
}

func TestRequireRole_AllowsAdmin(t *testing.T) {
	reached := false
	validator := &stubValidator{userID: 1, username: "drayee", role: RoleAdmin}
	r := newTestRouter(validator, RoleAdmin, &reached)

	recorder := doRequest(r, "Bearer admin-token")

	if recorder.Code != http.StatusOK {
		t.Fatalf("管理员访问状态码 = %d，期望 200 (body=%s)", recorder.Code, recorder.Body.String())
	}
	if !reached {
		t.Error("管理员应该到达管理端 handler")
	}

	body := decodeBody(t, recorder)
	if body["username"] != "drayee" || body["role"] != RoleAdmin {
		t.Errorf("身份信息透传有误: %v", body)
	}
	if body["user_id"] != float64(1) {
		t.Errorf("user_id 透传有误: %v", body["user_id"])
	}
}

// RequireRole 忘了搭配 AuthRequired 时必须拒绝访问，而不是因为"没有身份"就放行。
func TestRequireRole_WithoutAuthRequiredFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reached := false
	r := gin.New()
	r.GET("/protected", RequireRole(RoleAdmin), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("缺少 AuthRequired 时状态码 = %d，期望 401（fail closed）", recorder.Code)
	}
	if reached {
		t.Error("缺少认证信息时不应该放行到 handler")
	}
}

func TestExtractBearerToken(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		fails  bool
	}{
		{name: "标准形式", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "多余空格", header: "Bearer   abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "空头", header: "", fails: true},
		{name: "缺少前缀", header: "abc.def.ghi", fails: true},
		{name: "只有前缀", header: "Bearer ", fails: true},
		{name: "小写前缀", header: "bearer abc", fails: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if testCase.header != "" {
				c.Request.Header.Set("Authorization", testCase.header)
			}

			token, err := extractBearerToken(c)

			if testCase.fails {
				if err == nil {
					t.Fatalf("期望失败，实际取到 %q", token)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外失败: %v", err)
			}
			if token != testCase.want {
				t.Errorf("token = %q，期望 %q", token, testCase.want)
			}
		})
	}
}
