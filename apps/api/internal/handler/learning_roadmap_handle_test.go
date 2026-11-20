package handler

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

/**
 * 学习路线错误映射（#223）。
 *
 * 盯住一条线上的事故：管理端新建路线回 {"code":5000,"message":"操作失败，请稍后重试"}，
 * 而服务端一个字都没留，完全无法判断是约束冲突、表缺失还是连接断开。
 *
 * 所以这里锁两件事：
 *   1. 未识别的错误仍然只给客户端一句通用文案（不泄露 SQLSTATE / 主机端口）；
 *   2. 但必须记一行日志 —— 否则下一次再出 5000，还是没人查得动。
 */

// captureRoadmapFail 用最小 gin 上下文调用 h.fail，返回响应体。
//
// 不在这里改 gin.Mode：回包是否脱敏完全取决于是 debug 还是 release，
// 由调用方按要验证的那一面自行设置。
func captureRoadmapFail(t *testing.T, err error) (int, string, string) {
	t.Helper()

	// 截获日志：断言「确实记了」而不是靠肉眼看。
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)

	h := &LearningRoadmapHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/studio/roadmaps", nil)

	h.fail(c, err)
	return w.Code, w.Body.String(), logs.String()
}

// TestRoadmapFail_UnknownErrorIsLogged 未识别错误：脱敏回包 + 落日志。
func TestRoadmapFail_UnknownErrorIsLogged(t *testing.T) {
	// release 模式才是线上形态：failInternal 只在 debug 下回原文（本地联调便利）。
	prevMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(prevMode)

	// 用一个真实会出现、但类型上「不认识」的错误：模拟写库失败。
	dbErr := errors.New(`pq: insert or update on table "learning_roadmaps" violates foreign key constraint`)

	status, body, logs := captureRoadmapFail(t, dbErr)

	if status != http.StatusInternalServerError {
		t.Fatalf("未识别错误应回 500，实际 %d", status)
	}
	// 回包必须脱敏：不能把 SQLSTATE、表名、约束名吐给前端。
	for _, leak := range []string{"pq:", "learning_roadmaps", "constraint"} {
		if strings.Contains(body, leak) {
			t.Fatalf("回包泄露了内部细节 %q：%s", leak, body)
		}
	}
	if !strings.Contains(body, "5000") {
		t.Fatalf("应使用通用失败业务码 5000，实际 %s", body)
	}
	if !strings.Contains(logs, "violates foreign key constraint") {
		t.Fatalf("未识别的错误必须记日志，否则线上无从排查；实际日志：%q", logs)
	}
}

// TestRoadmapFail_ValidationKeepsReadableReason 内容不合法要回可读原因，不能降级成 5000。
func TestRoadmapFail_ValidationKeepsReadableReason(t *testing.T) {
	reason := "第 1 个阶段的资源链接必须是 http/https"
	status, body, _ := captureRoadmapFail(t, &domain.RoadmapValidationError{Reason: reason})

	if status != http.StatusBadRequest {
		t.Fatalf("内容不合法应回 400，实际 %d", status)
	}
	if !strings.Contains(body, reason) {
		t.Fatalf("应回传具体原因，实际 %s", body)
	}
}

// TestRoadmapFail_NotFoundMapsTo404 找不到映射 404 而不是 5000。
func TestRoadmapFail_NotFoundMapsTo404(t *testing.T) {
	status, _, _ := captureRoadmapFail(t, domain.ErrRoadmapNotFound)
	if status != http.StatusNotFound {
		t.Fatalf("路线不存在应回 404，实际 %d", status)
	}
}

// TestRoadmapFail_VersionConflictMapsTo400 版本冲突是可操作错误，不是服务端故障。
func TestRoadmapFail_VersionConflictMapsTo400(t *testing.T) {
	status, body, _ := captureRoadmapFail(t, domain.ErrRoadmapVersionConflict)
	if status != http.StatusBadRequest {
		t.Fatalf("版本冲突应回 400，实际 %d", status)
	}
	if !strings.Contains(body, "刷新") {
		t.Fatalf("版本冲突要告诉用户刷新后重试，实际 %s", body)
	}
}
