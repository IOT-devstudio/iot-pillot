package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 桩：招新仓储 / 表单仓储（只实现被用到的方法，越界即 panic）
 * ------------------------------------------------------------------ */

type httpRecruitmentRepo struct {
	repository.RecruitmentRepo
	directions []*domain.RecruitmentDirection
	rounds     []*domain.RecruitmentRound
}

func (s *httpRecruitmentRepo) ListDirections(_ context.Context) ([]*domain.RecruitmentDirection, error) {
	return s.directions, nil
}

func (s *httpRecruitmentRepo) GetActiveRound(_ context.Context) (*domain.RecruitmentRound, error) {
	for _, r := range s.rounds {
		if r.Accepting {
			return r, nil
		}
	}
	return nil, repository.ErrRoundNotFound
}

type httpUserRepo struct{ repository.UserRepo }

// httpFormRepoForDirections 只实现方向列表组装要用的查询。
type httpFormRepoForDirections struct {
	repository.FormRepo
	forms []*domain.Form
}

func (s *httpFormRepoForDirections) ListOpenFormsByRound(_ context.Context, roundID int) ([]*domain.Form, error) {
	out := make([]*domain.Form, 0)
	for _, f := range s.forms {
		if f.RoundID == roundID && f.IsOpen {
			out = append(out, f)
		}
	}
	return out, nil
}

// newDirectionsEngine 组装方向列表路由（#135 变更一的验收面）。
func newDirectionsEngine(dirs []*domain.RecruitmentDirection, rounds []*domain.RecruitmentRound, forms []*domain.Form) *gin.Engine {
	gin.SetMode(gin.TestMode)

	recRepo := &httpRecruitmentRepo{directions: dirs, rounds: rounds}
	recUC := service.NewRecruitmentUseCase(recRepo, httpUserRepo{})

	formRepo := &httpFormRepoForDirections{forms: forms}
	formUC := service.NewFormUseCase(formRepo, httpDirectory{}, httpCreator{})

	h := NewRecruitmentHandler(recUC, formUC)

	r := gin.New()
	r.GET("/api/v1/recruitment/directions",
		middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1}),
		h.ListActiveDirections)
	return r
}

type directionItem struct {
	Code string `json:"code"`
	Open bool   `json:"open"`
	Form *struct {
		ID         int    `json:"id"`
		ShareToken string `json:"share_token"`
		IsOpen     bool   `json:"is_open"`
	} `json:"form"`
}

func decodeDirections(t *testing.T, body []byte) []directionItem {
	t.Helper()
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []directionItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v，body=%s", err, string(body))
	}
	if resp.Code != 0 {
		t.Errorf("业务码应为 0，实际 %d", resp.Code)
	}
	return resp.Data.Items
}

/* ------------------------------------------------------------------ *
 * 验收项：每个方向带 form；无开放表单时为 null
 * ------------------------------------------------------------------ */

func TestListActiveDirections_IncludesForm(t *testing.T) {
	dirs := []*domain.RecruitmentDirection{
		{ID: 1, Code: "front-end", Name: "前端开发", Open: true, SortOrder: 10},
		{ID: 2, Code: "back-end", Name: "后端开发", Open: true, SortOrder: 20},
	}
	rounds := []*domain.RecruitmentRound{{ID: 1, RoundNum: 1, Accepting: true}}
	forms := []*domain.Form{
		{ID: 7, RoundID: 1, DirectionCode: "front-end", ShareToken: "front-token", IsOpen: true},
	}

	r := newDirectionsEngine(dirs, rounds, forms)
	w := doJSON(r, http.MethodGet, "/api/v1/recruitment/directions", "", "good-token")

	if w.Code != http.StatusOK {
		t.Fatalf("方向列表应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}

	items := decodeDirections(t, w.Body.Bytes())
	if len(items) != 2 {
		t.Fatalf("应有 2 个方向，实际 %d", len(items))
	}

	byCode := make(map[string]directionItem, len(items))
	for _, it := range items {
		byCode[it.Code] = it
	}

	// 有开放表单的方向：form 带 id / share_token / is_open
	fe, ok := byCode["front-end"]
	if !ok {
		t.Fatal("应包含 front-end")
	}
	if fe.Form == nil {
		t.Fatal("front-end 有开放表单，form 不应为 null")
	}
	if fe.Form.ID != 7 || fe.Form.ShareToken != "front-token" || !fe.Form.IsOpen {
		t.Errorf("form 内容不符：%+v", *fe.Form)
	}

	// 无开放表单的方向：form 必须是 null，不是空对象
	be, ok := byCode["back-end"]
	if !ok {
		t.Fatal("应包含 back-end")
	}
	if be.Form != nil {
		t.Errorf("back-end 无开放表单，form 应为 null，实际 %+v", *be.Form)
	}
}

// form 为 null 必须序列化成 JSON null —— 前端靠 form === null 降级，
// 若序列化成 {"id":0,...} 假对象，前端的 null 判断会失效。
func TestListActiveDirections_NullFormIsJSONNull(t *testing.T) {
	dirs := []*domain.RecruitmentDirection{
		{ID: 1, Code: "front-end", Name: "前端开发", Open: true, SortOrder: 10},
	}
	rounds := []*domain.RecruitmentRound{{ID: 1, RoundNum: 1, Accepting: true}}

	r := newDirectionsEngine(dirs, rounds, nil)
	w := doJSON(r, http.MethodGet, "/api/v1/recruitment/directions", "", "good-token")

	var raw struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if len(raw.Data.Items) != 1 {
		t.Fatalf("应有 1 个方向，实际 %d", len(raw.Data.Items))
	}

	form, present := raw.Data.Items[0]["form"]
	if !present {
		t.Fatal("form 字段必须存在（值可为 null）")
	}
	if form != nil {
		t.Errorf("无表单时 form 必须是 JSON null，实际 %#v", form)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：无开放轮次时所有 form 为 null，接口不报错
 * ------------------------------------------------------------------ */

func TestListActiveDirections_NoActiveRound(t *testing.T) {
	dirs := []*domain.RecruitmentDirection{
		{ID: 1, Code: "front-end", Name: "前端开发", Open: true, SortOrder: 10},
	}
	// 没有任何 accepting 的轮次
	rounds := []*domain.RecruitmentRound{{ID: 1, RoundNum: 1, Accepting: false}}
	forms := []*domain.Form{
		{ID: 7, RoundID: 1, DirectionCode: "front-end", ShareToken: "t", IsOpen: true},
	}

	r := newDirectionsEngine(dirs, rounds, forms)
	w := doJSON(r, http.MethodGet, "/api/v1/recruitment/directions", "", "good-token")

	if w.Code != http.StatusOK {
		t.Fatalf("无开放轮次时方向列表仍应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}

	items := decodeDirections(t, w.Body.Bytes())
	if len(items) != 1 {
		t.Fatalf("应有 1 个方向，实际 %d", len(items))
	}
	if items[0].Form != nil {
		t.Errorf("无开放轮次时 form 应为 null，实际 %+v", *items[0].Form)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：方向列表需要登录
 * ------------------------------------------------------------------ */

func TestListActiveDirections_RequiresLogin(t *testing.T) {
	dirs := []*domain.RecruitmentDirection{
		{ID: 1, Code: "front-end", Name: "前端开发", Open: true, SortOrder: 10},
	}
	rounds := []*domain.RecruitmentRound{{ID: 1, RoundNum: 1, Accepting: true}}

	r := newDirectionsEngine(dirs, rounds, nil)
	w := doJSON(r, http.MethodGet, "/api/v1/recruitment/directions", "", "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录读方向列表应返回 401，实际 %d，body=%s", w.Code, w.Body.String())
	}
}
