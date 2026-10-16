package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// foundingCheckerStub 注入创始判定结果（issue #142）。
type foundingCheckerStub struct {
	founding bool
	err      error
}

func (s foundingCheckerStub) IsFounding(_ context.Context, _ int) (bool, error) {
	return s.founding, s.err
}

// runRequireFounding 跑一遍 RequireFounding，返回 (是否放行, HTTP 状态码)。
//
// gin.HandlerFunc 签名是 func(*gin.Context)：放行 = 中间件没调 c.Abort()，
// 即 !c.IsAborted()。
func runRequireFounding(t *testing.T, checker FoundingChecker, withIdentity bool) (bool, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/admin/users/7", nil)
	if withIdentity {
		c.Set(ContextUserID, 7)
		c.Set(ContextUsername, "founder")
		c.Set(ContextRole, "admin")
	}

	RequireFounding(checker)(c)

	return !c.IsAborted(), recorder.Code
}

// 缺少身份（忘记挂 AuthRequired）→ 401 拒绝，不放行。
func TestRequireFounding_Unauthenticated(t *testing.T) {
	passed, status := runRequireFounding(t, foundingCheckerStub{founding: true}, false)
	if passed {
		t.Error("缺少身份不应放行")
	}
	if status != 401 {
		t.Errorf("期望 401，实际 %d", status)
	}
}

// 创始管理员 → 放行。
func TestRequireFounding_PassesForFounding(t *testing.T) {
	passed, status := runRequireFounding(t, foundingCheckerStub{founding: true}, true)
	if !passed {
		t.Errorf("创始管理员应放行，status=%d", status)
	}
}

// 晋升管理员 / 普通成员（IsFounding=false）→ 403，不放行。
// 403 文案与 RequireAdmin 一致，不向普通管理员暴露「创始」概念的存在。
func TestRequireFounding_ForbiddenForNonFounding(t *testing.T) {
	passed, status := runRequireFounding(t, foundingCheckerStub{founding: false}, true)
	if passed {
		t.Error("非创始管理员不应放行")
	}
	if status != 403 {
		t.Errorf("期望 403，实际 %d", status)
	}
}

// 创始名单读取失败（Redis 故障）→ 5xx 拒绝，绝不变提权窗口
//（与 RequireAdmin「读失败拒绝而不是放行」同款取舍）。
func TestRequireFounding_ServerErrorOnCheckerFailure(t *testing.T) {
	passed, status := runRequireFounding(
		t,
		foundingCheckerStub{err: errors.New("redis down")},
		true,
	)
	if passed {
		t.Error("判定失败时不应放行")
	}
	if status < 500 {
		t.Errorf("判定失败期望 5xx，实际 %d", status)
	}
}
