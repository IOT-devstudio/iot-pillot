package handler

import (
	"context"
	"net/http"
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

// httpProjectRepo 只实现被用到的方法。
type httpProjectRepo struct {
	repository.StudioProjectRepo

	projects  map[int]*domain.StudioProject
	members   map[int][]*domain.StudioProjectMember
	nextPID   int
	nextMID   int
	createErr error
	addErr    error
}

func newHTTPProjectRepo() *httpProjectRepo {
	return &httpProjectRepo{
		projects: make(map[int]*domain.StudioProject),
		members:  make(map[int][]*domain.StudioProjectMember),
		nextPID:  1,
		nextMID:  100,
	}
}

func (s *httpProjectRepo) Create(_ context.Context, p *domain.StudioProject) error {
	if s.createErr != nil {
		return s.createErr
	}
	p.ID = s.nextPID
	s.nextPID++
	stored := *p
	s.projects[p.ID] = &stored
	return nil
}

func (s *httpProjectRepo) GetByID(_ context.Context, id int) (*domain.StudioProject, error) {
	if p, ok := s.projects[id]; ok {
		return p, nil
	}
	return nil, repository.ErrStudioProjectNotFound
}

func (s *httpProjectRepo) List(_ context.Context, _ repository.StudioProjectListFilter, _ int, _ int) ([]*domain.StudioProject, int64, error) {
	out := make([]*domain.StudioProject, 0, len(s.projects))
	for _, p := range s.projects {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (s *httpProjectRepo) Update(_ context.Context, p *domain.StudioProject) error {
	if _, ok := s.projects[p.ID]; !ok {
		return repository.ErrStudioProjectNotFound
	}
	stored := *p
	s.projects[p.ID] = &stored
	return nil
}

func (s *httpProjectRepo) Delete(_ context.Context, id int) error {
	if _, ok := s.projects[id]; !ok {
		return repository.ErrStudioProjectNotFound
	}
	delete(s.projects, id)
	delete(s.members, id)
	return nil
}

func (s *httpProjectRepo) ListMembers(_ context.Context, projectID int) ([]*domain.StudioProjectMember, error) {
	stored := s.members[projectID]
	out := make([]*domain.StudioProjectMember, 0, len(stored))
	out = append(out, stored...)
	return out, nil
}

func (s *httpProjectRepo) AddMember(_ context.Context, member *domain.StudioProjectMember) error {
	if s.addErr != nil {
		return s.addErr
	}
	if _, ok := s.projects[member.ProjectID]; !ok {
		return repository.ErrStudioProjectNotFound
	}
	for _, m := range s.members[member.ProjectID] {
		if m.Name == member.Name {
			return repository.ErrStudioProjectMemberDuplicate
		}
	}
	member.ID = s.nextMID
	s.nextMID++
	stored := *member
	s.members[member.ProjectID] = append(s.members[member.ProjectID], &stored)
	return nil
}

func (s *httpProjectRepo) UpdateMember(_ context.Context, member *domain.StudioProjectMember) error {
	stored := s.members[member.ProjectID]
	for i, m := range stored {
		if m.ID == member.ID {
			next := *member
			stored[i] = &next
			return nil
		}
	}
	return repository.ErrStudioProjectMemberNotFound
}

func (s *httpProjectRepo) RemoveMember(_ context.Context, projectID int, memberID int) error {
	stored := s.members[projectID]
	for i, m := range stored {
		if m.ID == memberID {
			s.members[projectID] = append(stored[:i], stored[i+1:]...)
			return nil
		}
	}
	return repository.ErrStudioProjectMemberNotFound
}

func (s *httpProjectRepo) StudioMemberExists(_ context.Context, memberID int) (bool, error) {
	return memberID == 7, nil
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

// newProjectTestEngine 组装与生产同一套中间件的项目路由。
func newProjectTestEngine(t *testing.T, admin bool) (*gin.Engine, *httpProjectRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newHTTPProjectRepo()
	uc := service.NewStudioProjectUseCase(repo)
	h := NewStudioProjectHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	requireAdmin := middleware.RequireAdmin(stubAdminChecker{admin: admin})

	group := r.Group("/api/v1/admin", authRequired, requireAdmin)
	studio := group.Group("/studio")
	studio.GET("/projects", h.ListStudioProjects)
	studio.POST("/projects", h.CreateStudioProject)
	studio.GET("/projects/:id", h.GetStudioProject)
	studio.PUT("/projects/:id", h.UpdateStudioProject)
	studio.DELETE("/projects/:id", h.DeleteStudioProject)
	studio.GET("/projects/:id/members", h.ListStudioProjectMembers)
	studio.POST("/projects/:id/members", h.AddStudioProjectMember)
	studio.PUT("/projects/:id/members/:member_id", h.UpdateStudioProjectMember)
	studio.DELETE("/projects/:id/members/:member_id", h.DeleteStudioProjectMember)
	return r, repo
}

const validProjectBody = `{"name":"校园导览小程序","year":"2024","summary":"新生报到用的校园导览。"}`

const validProjectMemberBody = `{"member_id":7,"name":"张三","role":"lead","started_at":"2024-03-01T00:00:00+08:00"}`

/* ------------------------------------------------------------------ *
 * 验收项：非 admin 调用返回 403
 * ------------------------------------------------------------------ */

func TestStudioProject_NonAdminForbidden(t *testing.T) {
	r, repo := newProjectTestEngine(t, false)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/studio/projects", ""},
		{http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody},
		{http.MethodGet, "/api/v1/admin/studio/projects/1", ""},
		{http.MethodPut, "/api/v1/admin/studio/projects/1", `{"name":"新名字"}`},
		{http.MethodDelete, "/api/v1/admin/studio/projects/1", ""},
		{http.MethodGet, "/api/v1/admin/studio/projects/1/members", ""},
		{http.MethodPost, "/api/v1/admin/studio/projects/1/members", validProjectMemberBody},
		{http.MethodPut, "/api/v1/admin/studio/projects/1/members/100", `{"role":"contributor"}`},
		{http.MethodDelete, "/api/v1/admin/studio/projects/1/members/100", ""},
	}

	for _, tc := range cases {
		w := doJSON(r, tc.method, tc.path, tc.body, "good-token")
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s 非管理员应 403，实际 %d，body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if len(repo.projects) != 0 {
		t.Error("非管理员请求不应写库")
	}
}

func TestStudioProject_UnauthenticatedRejected(t *testing.T) {
	r, _ := newProjectTestEngine(t, true)
	if w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 项目 CRUD
 * ------------------------------------------------------------------ */

func TestStudioProjectLifecycle(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	// 创建
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建失败 %d: %s", w.Code, w.Body.String())
	}

	// 列表
	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects", "", "good-token")
	if w.Code != http.StatusOK || !containsAll(w.Body.String(), `"total":1`, "校园导览小程序") {
		t.Fatalf("列表应含刚创建的项目，实际 %d %s", w.Code, w.Body.String())
	}

	// 详情
	w = doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects/1", "", "good-token")
	if w.Code != http.StatusOK || !containsAll(w.Body.String(), "2024", "新生报到用的校园导览。", `"members":[]`) {
		t.Fatalf("详情应返回元数据与空参与名单，实际 %d %s", w.Code, w.Body.String())
	}

	// 更新
	w = doJSON(r, http.MethodPut, "/api/v1/admin/studio/projects/1", `{"summary":"更新后的简介。"}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("更新失败 %d: %s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "更新后的简介。", "校园导览小程序") {
		t.Errorf("更新应只改传入字段，实际 %s", w.Body.String())
	}

	// 删除
	w = doJSON(r, http.MethodDelete, "/api/v1/admin/studio/projects/1", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("删除应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.projects) != 0 {
		t.Error("删除后项目应不存在")
	}

	// 重复删除：明确 400，不是 500
	if w := doJSON(r, http.MethodDelete, "/api/v1/admin/studio/projects/1", "", "good-token"); w.Code != http.StatusBadRequest {
		t.Fatalf("重复删除应 400，实际 %d", w.Code)
	}
}

func TestCreateStudioProject_DuplicateNameRejected(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("首次创建失败 %d", w.Code)
	}
	repo.createErr = repository.ErrStudioProjectDuplicateName

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("同名项目应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "同名") {
		t.Errorf("应说明同名冲突，实际 %s", w.Body.String())
	}
}

func TestCreateStudioProject_InvalidStatusRejected(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects",
		`{"name":"项目","year":"2024","status":"in_progress"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法状态应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "状态") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
	if len(repo.projects) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestStudioProject_NotFound(t *testing.T) {
	r, _ := newProjectTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects/999", "", "good-token")
	if w.Code != http.StatusBadRequest || !containsAll(w.Body.String(), "不存在") {
		t.Fatalf("不存在的项目应 400 且给可读原因，实际 %d %s", w.Code, w.Body.String())
	}
}

func TestStudioProject_InvalidID(t *testing.T) {
	r, _ := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects/abc", "", "good-token"); w.Code != http.StatusBadRequest {
		t.Errorf("非数字 id 应 400，实际 %d", w.Code)
	}
	// 该路径只有 PUT/DELETE（GET 是名单本身），用 PUT 验证 member_id 解析。
	if w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/projects/1/members/abc", `{"role":"lead"}`, "good-token"); w.Code != http.StatusBadRequest {
		t.Errorf("非数字 member_id 应 400，实际 %d", w.Code)
	}
}

func TestListStudioProjects_EmptyReturnsEmptyArray(t *testing.T) {
	r, _ := newProjectTestEngine(t, true)

	w := doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("空列表应 200，实际 %d", w.Code)
	}
	// 空列表必须是 []，不能是 null——前端对 null 做 .map 会直接抛错。
	if !containsAll(w.Body.String(), `"items":[]`) {
		t.Errorf("空列表应序列化成 []，实际 %s", w.Body.String())
	}
}

/* ------------------------------------------------------------------ *
 * 参与
 * ------------------------------------------------------------------ */

func TestStudioProject_MemberLifecycle(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建项目失败 %d", w.Code)
	}

	// 添加参与
	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects/1/members", validProjectMemberBody, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("添加参与失败 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"role":"lead"`, "张三", `"member_id":7`) {
		t.Errorf("参与行应回显角色/姓名/成员关联，实际 %s", w.Body.String())
	}

	// 名单
	w = doJSON(r, http.MethodGet, "/api/v1/admin/studio/projects/1/members", "", "good-token")
	if w.Code != http.StatusOK || !containsAll(w.Body.String(), `"total":1`, "张三") {
		t.Fatalf("名单应含一条参与，实际 %d %s", w.Code, w.Body.String())
	}

	// 更新：改成 contributor
	w = doJSON(r, http.MethodPut, "/api/v1/admin/studio/projects/1/members/100", `{"role":"contributor"}`, "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("更新参与失败 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"role":"contributor"`, "张三") {
		t.Errorf("更新应只改传入字段，实际 %s", w.Body.String())
	}

	// 移除
	w = doJSON(r, http.MethodDelete, "/api/v1/admin/studio/projects/1/members/100", "", "good-token")
	if w.Code != http.StatusOK {
		t.Fatalf("移除应成功，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.members[1]) != 0 {
		t.Error("移除后参与行应不存在")
	}
}

func TestAddStudioProjectMember_DuplicateNameRejected(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建项目失败 %d", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects/1/members", validProjectMemberBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("首次添加失败 %d", w.Code)
	}

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects/1/members", validProjectMemberBody, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("同项目同名应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "同名") {
		t.Errorf("应说明同名冲突，实际 %s", w.Body.String())
	}
	if len(repo.members[1]) != 1 {
		t.Errorf("重复参与不应新增，实际 %d", len(repo.members[1]))
	}
}

func TestAddStudioProjectMember_EndedBeforeStartRejected(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建项目失败 %d", w.Code)
	}

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects/1/members",
		`{"member_id":7,"name":"张三","role":"lead","started_at":"2024-03-01T00:00:00+08:00","ended_at":"2024-02-01T00:00:00+08:00"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("结束早于开始应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "结束时间不能早于开始时间") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
	if len(repo.members[1]) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestAddStudioProjectMember_UnknownMemberRejected(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建项目失败 %d", w.Code)
	}

	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects/1/members",
		`{"member_id":999,"name":"张三","role":"lead","started_at":"2024-03-01T00:00:00+08:00"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("不存在的成员档案应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), "成员不存在") {
		t.Errorf("应给出可读原因，实际 %s", w.Body.String())
	}
	if len(repo.members[1]) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestAddStudioProjectMember_InvalidTimeRejected(t *testing.T) {
	r, repo := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建项目失败 %d", w.Code)
	}

	// 格式非法必须报错：把它当成「还没解析」会被静默当成「仍在参与」。
	w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects/1/members",
		`{"member_id":7,"name":"张三","role":"lead","started_at":"2024/03/01"}`, "good-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法时间格式应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.members[1]) != 0 {
		t.Error("校验失败不应写库")
	}
}

func TestStudioProject_MemberNotFound(t *testing.T) {
	r, _ := newProjectTestEngine(t, true)

	if w := doJSON(r, http.MethodPost, "/api/v1/admin/studio/projects", validProjectBody, "good-token"); w.Code != http.StatusOK {
		t.Fatalf("创建项目失败 %d", w.Code)
	}

	if w := doJSON(r, http.MethodPut, "/api/v1/admin/studio/projects/1/members/999", `{"role":"lead"}`, "good-token"); w.Code != http.StatusBadRequest {
		t.Errorf("不存在的参与行应 400，实际 %d", w.Code)
	}
	if w := doJSON(r, http.MethodDelete, "/api/v1/admin/studio/projects/1/members/999", "", "good-token"); w.Code != http.StatusBadRequest {
		t.Errorf("不存在的参与行应 400，实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 时间解析
 * ------------------------------------------------------------------ */

func TestParseTimeValue_ThreeState(t *testing.T) {
	cases := []struct {
		name      string
		value     *string
		wantOK    bool
		wantEmpty bool
	}{
		{"没传", nil, false, true},
		{"传空串", ptr(""), false, true},
		{"传 null", ptr("null"), false, true},
		{"传合法时间", ptr("2024-03-01T00:00:00+08:00"), true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := parseTimeValue(tc.value, domain.ProjectTimeLayout)
			if err != nil {
				t.Fatalf("不应报错，实际 %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok 应为 %v，实际 %v", tc.wantOK, ok)
			}
			if ok && got.IsZero() {
				t.Error("解析成功时不应返回零值")
			}
		})
	}
}

func TestParseTimeValue_InvalidFormatRejected(t *testing.T) {
	// 把 "2024/03/01" 当成「还没解析」会被静默当成「仍在参与」。
	if _, _, err := parseTimeValue(ptr("2024/03/01"), domain.ProjectTimeLayout); err == nil {
		t.Fatal("非法格式应报错")
	}
}

func ptr(v string) *string { return &v }
