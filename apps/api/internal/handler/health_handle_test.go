package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// stubPinger 可控失败的探针替身。
type stubPinger struct {
	err error
}

func (s stubPinger) Ping(ctx context.Context) error { return s.err }

// newHealthTestRouter 造一个只挂两个健康路由的最小路由表。
func newHealthTestRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", h.Health)
	r.GET("/health/ready", h.Ready)
	return r
}

// TestReady_StorageUnavailableTurnsRed 存储不可用时就绪探针必须变红。
//
// 这是 #215 的核心行为：图片上传依赖对象存储，存储挂了却还报 ready，
// 用户会填完整篇帖子、上传图片时才看到失败。
func TestReady_StorageUnavailableTurnsRed(t *testing.T) {
	h := NewHealthHandler(stubPinger{}, stubPinger{}, stubPinger{err: errors.New("存储连不上")})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	newHealthTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("存储不可用时应返回 503，实际 %d", w.Code)
	}
	var body struct {
		Data struct {
			Status   string            `json:"status"`
			Failures map[string]string `json:"failures"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if _, ok := body.Data.Failures["storage"]; !ok {
		t.Fatalf("失败原因里应包含 storage，实际 %v", body.Data.Failures)
	}
}

// TestReady_StorageRecoversTurnsGreen 存储恢复后就绪探针必须变绿。
func TestReady_StorageRecoversTurnsGreen(t *testing.T) {
	h := NewHealthHandler(stubPinger{}, stubPinger{}, stubPinger{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	newHealthTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("依赖全可用时应返回 200，实际 %d（响应体 %s）", w.Code, w.Body.String())
	}
}

// TestHealth_LivenessIgnoresDependencies 存活探针不查任何依赖。
//
// 依赖全挂时存活探针仍要返回 ok：否则数据库一抖，容器就被判定为需要重启，
// 把一次局部故障放大成全站重启。
func TestHealth_LivenessIgnoresDependencies(t *testing.T) {
	h := NewHealthHandler(stubPinger{err: errors.New("db 挂了")}, stubPinger{err: errors.New("redis 挂了")}, stubPinger{err: errors.New("存储挂了")})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	newHealthTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("存活探针应恒返回 200，实际 %d", w.Code)
	}
}
