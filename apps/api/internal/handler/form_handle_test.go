package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 桩
 * ------------------------------------------------------------------ */

// httpFormRepo 只实现被用到的仓储方法。
//
// 其余方法由嵌入的接口提供、调用即 panic —— 测试一旦越界会立刻炸，
// 而不是悄悄走到一个"返回零值"的假实现上。
type httpFormRepo struct {
	repository.FormRepo

	forms       map[int]*domain.Form
	fields      map[int][]*domain.FormField
	submissions map[int]*domain.FormSubmission
	nextSubID   int
}

func newHTTPFormRepo() *httpFormRepo {
	return &httpFormRepo{
		forms:       make(map[int]*domain.Form),
		fields:      make(map[int][]*domain.FormField),
		submissions: make(map[int]*domain.FormSubmission),
		nextSubID:   1,
	}
}

func (s *httpFormRepo) GetFormByShareToken(_ context.Context, token string) (*domain.Form, error) {
	for _, f := range s.forms {
		if f.ShareToken == token {
			return f, nil
		}
	}
	return nil, repository.ErrFormNotFound
}

func (s *httpFormRepo) ListFields(_ context.Context, formID int) ([]*domain.FormField, error) {
	if _, ok := s.forms[formID]; !ok {
		return nil, repository.ErrFormNotFound
	}
	return s.fields[formID], nil
}

func (s *httpFormRepo) GetSubmissionBySubmitter(_ context.Context, formID int, submitterID int) (*domain.FormSubmission, error) {
	for _, sub := range s.submissions {
		if sub.FormID == formID && sub.SubmitterID == submitterID {
			return sub, nil
		}
	}
	return nil, repository.ErrSubmissionNotFound
}

func (s *httpFormRepo) CreateSubmission(_ context.Context, submission *domain.FormSubmission) error {
	submission.ID = s.nextSubID
	s.nextSubID++
	s.submissions[submission.ID] = submission
	return nil
}

type httpDirectory struct{ service.RecruitmentDirectory }

func (h httpDirectory) GetRoundByID(_ context.Context, id int) (*domain.RecruitmentRound, error) {
	if id == 1 {
		return &domain.RecruitmentRound{ID: 1, RoundNum: 1, Name: "2026 秋季招新", Accepting: true}, nil
	}
	return nil, repository.ErrRoundNotFound
}

type httpCreator struct{ service.ApplicationCreator }

func (h httpCreator) SubmitToRound(_ context.Context, userID int, roundID int, directionCode string) (*domain.Application, error) {
	return &domain.Application{
		ID:            900,
		UserID:        userID,
		RoundID:       roundID,
		DirectionCode: directionCode,
		Status:        domain.ApplicationApplied,
	}, nil
}

// GetApplicationByUserRoundDirection 默认「没有既有申请」→ 走新建路径。
// 验证复用与终态分支的用例改用 httpCreatorWithApp。
func (h httpCreator) GetApplicationByUserRoundDirection(_ context.Context, _ int, _ int, _ string) (*domain.Application, error) {
	return nil, repository.ErrApplicationNotFound
}

// httpCreatorWithApp 预置一条既有申请，用来验证 #135 的复用与终态拒绝分支。
type httpCreatorWithApp struct {
	service.ApplicationCreator
	existing *domain.Application
}

func (h httpCreatorWithApp) SubmitToRound(_ context.Context, userID int, roundID int, directionCode string) (*domain.Application, error) {
	return &domain.Application{
		ID:            900,
		UserID:        userID,
		RoundID:       roundID,
		DirectionCode: directionCode,
		Status:        domain.ApplicationApplied,
	}, nil
}

func (h httpCreatorWithApp) GetApplicationByUserRoundDirection(_ context.Context, _ int, _ int, _ string) (*domain.Application, error) {
	return h.existing, nil
}

// stubValidator 满足 middleware.TokenValidator。
type stubValidator struct {
	accept string
	userID int
	err    error
}

func (s stubValidator) ValidateAccessToken(_ context.Context, token string) (int, string, string, error) {
	if s.err != nil {
		return 0, "", "", s.err
	}
	if token != s.accept {
		return 0, "", "", errors.New("invalid token")
	}
	return s.userID, "tester", middleware.RoleMember, nil
}

// stubAdminChecker 满足 middleware.AdminChecker。
type stubAdminChecker struct{ admin bool }

func (s stubAdminChecker) IsAdmin(_ context.Context, _ int) (bool, error) { return s.admin, nil }

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

// newFormTestEngine 组装与生产同一套中间件的表单路由。
//
// 刻意复用 middleware.AuthRequired / RequireAdmin 而不是手写桩中间件：
// "未登录被拒绝"必须验证真实鉴权链，否则测的是我自己的桩。
func newFormTestEngine(t *testing.T) (*gin.Engine, *httpFormRepo) {
	t.Helper()
	r, repo, _ := newFormTestEngineWithCreator(t, httpCreator{})
	return r, repo
}

// newFormTestEngineWithCreator 允许替换 ApplicationCreator，
// 用于验证 #135 的复用与终态拒绝分支。
func newFormTestEngineWithCreator(t *testing.T, creator service.ApplicationCreator) (*gin.Engine, *httpFormRepo, *service.FormUseCase) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newHTTPFormRepo()
	repo.forms[1] = &domain.Form{
		ID: 1, Title: "报名表", RoundID: 1, DirectionCode: "front-end",
		ShareToken: "valid-token", IsOpen: true,
	}
	repo.fields[1] = []*domain.FormField{
		{ID: 11, FormID: 1, Type: domain.FieldText, Label: "姓名", Required: true},
		{ID: 12, FormID: 1, Type: domain.FieldEmail, Label: "邮箱", Required: true},
	}

	uc := service.NewFormUseCase(repo, httpDirectory{}, creator)
	h := NewFormHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	requireAdmin := middleware.RequireAdmin(stubAdminChecker{admin: false})

	r.POST("/api/v1/forms/public/:shareToken/submit", authRequired, h.SubmitForm)
	r.GET("/api/v1/forms/public/:shareToken", authRequired, h.ReadShareForm)
	r.POST("/api/v1/forms", authRequired, requireAdmin, h.CreateForm)
	return r, repo, uc
}

func doJSON(r *gin.Engine, method string, path string, body string, token string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

/* ------------------------------------------------------------------ *
 * 验收项：未登录提交被拒绝，不产生匿名记录
 * ------------------------------------------------------------------ */

func TestSubmitForm_UnauthenticatedRejected(t *testing.T) {
	r, repo := newFormTestEngine(t)

	w := doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"11":"张三","12":"a@b.com"}}`, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录提交应返回 401，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.submissions) != 0 {
		t.Errorf("未登录提交不应产生提交记录，实际 %d 条", len(repo.submissions))
	}
	if !strings.Contains(w.Body.String(), "Authorization") {
		t.Errorf("应返回缺少认证头的明确错误，实际 %s", w.Body.String())
	}
}

func TestSubmitForm_InvalidTokenRejected(t *testing.T) {
	r, repo := newFormTestEngine(t)

	w := doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"11":"张三","12":"a@b.com"}}`, "bad-token")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无效令牌应返回 401，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.submissions) != 0 {
		t.Error("无效令牌不应产生提交记录")
	}
}

func TestReadShareForm_RequiresLogin(t *testing.T) {
	r, _ := newFormTestEngine(t)

	w := doJSON(r, http.MethodGet, "/api/v1/forms/public/valid-token", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("读取分享表单也必须登录（401），实际 %d，body=%s", w.Code, w.Body.String())
	}

	// 登录后能拿到定义与字段
	w = doJSON(r, http.MethodGet, "/api/v1/forms/public/valid-token", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("登录后读取应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "姓名") || !strings.Contains(w.Body.String(), "valid-token") {
		t.Errorf("响应应含表单定义与字段，实际 %s", w.Body.String())
	}

	// 无效分享令牌：明确错误，不是 404 也不是 500
	w = doJSON(r, http.MethodGet, "/api/v1/forms/public/nope", "", "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("无效分享令牌应返回 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "分享链接无效") {
		t.Errorf("应给出可读的无效链接提示，实际 %s", w.Body.String())
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：非管理员调用写接口返回 403
 * ------------------------------------------------------------------ */

func TestCreateForm_NonAdminForbidden(t *testing.T) {
	r, _ := newFormTestEngine(t)

	body := `{"title":"表","round_id":1,"direction_code":"front-end","fields":[{"type":"text","label":"姓名"}]}`
	w := doJSON(r, http.MethodPost, "/api/v1/forms", body, "good-token")

	if w.Code != http.StatusForbidden {
		t.Fatalf("非管理员建表应返回 403，实际 %d，body=%s", w.Code, w.Body.String())
	}

	// 未登录同样被拦（401 而不是 403）
	w = doJSON(r, http.MethodPost, "/api/v1/forms", body, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录建表应返回 401，实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：有效令牌 + 登录用户提交后真实落库
 * ------------------------------------------------------------------ */

func TestSubmitForm_SuccessPersists(t *testing.T) {
	r, repo := newFormTestEngine(t)

	w := doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"11":"张三","12":"zhangsan@example.com"}}`, "good-token")

	if w.Code != http.StatusOK {
		t.Fatalf("提交应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Submission struct {
				ID            int            `json:"id"`
				SubmitterID   int            `json:"submitter_id"`
				ApplicationID int            `json:"application_id"`
				Values        map[string]any `json:"values"`
			} `json:"submission"`
			ApplicationID int `json:"application_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v，body=%s", err, w.Body.String())
	}

	if resp.Code != 0 {
		t.Errorf("业务码应为 0，实际 %d", resp.Code)
	}
	if resp.Data.Submission.ID == 0 {
		t.Error("响应里应带回落库的提交 id")
	}
	if resp.Data.Submission.SubmitterID != 1 {
		t.Errorf("提交人应取自令牌（1），实际 %d", resp.Data.Submission.SubmitterID)
	}
	if resp.Data.Submission.ApplicationID != resp.Data.ApplicationID || resp.Data.ApplicationID == 0 {
		t.Errorf("提交应关联到同一份申请，实际 %d vs %d",
			resp.Data.Submission.ApplicationID, resp.Data.ApplicationID)
	}
	if got := resp.Data.Submission.Values["11"]; got != "张三" {
		t.Errorf("答案应回显，实际 %v", got)
	}
	if len(repo.submissions) != 1 {
		t.Errorf("应恰有 1 条提交记录，实际 %d 条", len(repo.submissions))
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：终态申请补交 → 400 + 可读原因，零副作用（#135）
 * ------------------------------------------------------------------ */

func TestSubmitForm_FinalApplicationRejected(t *testing.T) {
	existing := &domain.Application{
		ID: 555, UserID: 1, RoundID: 1, DirectionCode: "front-end",
		Status: domain.ApplicationAccepted,
	}
	r, repo, _ := newFormTestEngineWithCreator(t, httpCreatorWithApp{existing: existing})

	w := doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"11":"张三","12":"a@b.com"}}`, "good-token")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("终态申请补交应返回 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "已结束") {
		t.Errorf("应给出「申请已结束」的可读原因，实际 %s", w.Body.String())
	}
	if len(repo.submissions) != 0 {
		t.Errorf("零副作用：不应留下提交记录，实际 %d 条", len(repo.submissions))
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：已有 applied 申请 → 复用同一条申请，只落一条提交（#135）
 * ------------------------------------------------------------------ */

func TestSubmitForm_ReusesExistingApplication(t *testing.T) {
	existing := &domain.Application{
		ID: 555, UserID: 1, RoundID: 1, DirectionCode: "front-end",
		Status: domain.ApplicationApplied,
	}
	r, repo, _ := newFormTestEngineWithCreator(t, httpCreatorWithApp{existing: existing})

	w := doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"11":"张三","12":"a@b.com"}}`, "good-token")

	if w.Code != http.StatusOK {
		t.Fatalf("已有 applied 申请时补交应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Submission struct {
				ApplicationID int `json:"application_id"`
			} `json:"submission"`
			ApplicationID int `json:"application_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v，body=%s", err, w.Body.String())
	}

	if resp.Data.ApplicationID != existing.ID {
		t.Errorf("应复用既有申请 %d，实际 %d", existing.ID, resp.Data.ApplicationID)
	}
	if resp.Data.Submission.ApplicationID != existing.ID {
		t.Errorf("提交记录应回指既有申请 %d，实际 %d", existing.ID, resp.Data.Submission.ApplicationID)
	}
	if len(repo.submissions) != 1 {
		t.Errorf("应恰有 1 条提交记录，实际 %d 条", len(repo.submissions))
	}
}


/* ------------------------------------------------------------------ *
 * 验收项：服务端拒绝缺必填 / 已关闭表单
 * ------------------------------------------------------------------ */

func TestSubmitForm_ServerSideValidation(t *testing.T) {
	r, repo := newFormTestEngine(t)

	// 缺必填项
	w := doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"12":"a@b.com"}}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺必填应返回 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "必填项") {
		t.Errorf("应指出哪个必填项缺失，实际 %s", w.Body.String())
	}

	// 表单关闭后拒绝
	repo.forms[1].IsOpen = false
	w = doJSON(r, http.MethodPost, "/api/v1/forms/public/valid-token/submit",
		`{"values":{"11":"张三","12":"a@b.com"}}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("关闭表单提交应返回 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "已关闭") {
		t.Errorf("应给出「已关闭」的明确反馈，实际 %s", w.Body.String())
	}
	if len(repo.submissions) != 0 {
		t.Error("校验失败与关闭状态下都不应留下提交记录")
	}
}
