package service

import (
	"context"
	"errors"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 桩实现：RecruitmentUseCase 只依赖接口，不需要 DB
 * ------------------------------------------------------------------ */

type stubRecruitmentRepo struct {
	directions        []*domain.RecruitmentDirection
	rounds            []*domain.RecruitmentRound
	applications      map[int]*domain.Application
	timelines         map[int][]*domain.ApplicationTimeline
	profileErr        error
	nextAppID         int
	duplicateOnCreate bool
	// 可模拟错误
	errRoundNotFound     error
	errDirectionNotFound error
	errAppNotFound       error
	errDuplicate         error
}

func newStubRecruitmentRepo() *stubRecruitmentRepo {
	return &stubRecruitmentRepo{
		directions: []*domain.RecruitmentDirection{
			{ID: 1, Code: "front-end", Name: "前端开发", Open: true, SortOrder: 10},
			{ID: 2, Code: "back-end", Name: "后端开发", Open: true, SortOrder: 20},
			{ID: 3, Code: "agent", Name: "智能体", Open: true, SortOrder: 30},
			{ID: 4, Code: "all", Name: "全栈", Open: true, SortOrder: 40},
		},
		rounds: []*domain.RecruitmentRound{
			{ID: 1, RoundNum: 1, Name: "2026 秋季招新", Accepting: true},
		},
		applications: make(map[int]*domain.Application),
		timelines:    make(map[int][]*domain.ApplicationTimeline),
		nextAppID:    100,
	}
}

func (s *stubRecruitmentRepo) ListDirections(_ context.Context) ([]*domain.RecruitmentDirection, error) {
	return s.directions, nil
}

func (s *stubRecruitmentRepo) GetDirectionByCode(_ context.Context, code string) (*domain.RecruitmentDirection, error) {
	for _, d := range s.directions {
		if d.Code == code {
			return d, nil
		}
	}
	// 与 gorm 实现同契约：查不到返回哨兵错误（可用 errDirectionNotFound 覆盖注入其他错误）
	if s.errDirectionNotFound != nil {
		return nil, s.errDirectionNotFound
	}
	return nil, repository.ErrDirectionNotFound
}

func (s *stubRecruitmentRepo) SetDirectionOpen(_ context.Context, code string, open bool) error {
	for _, d := range s.directions {
		if d.Code == code {
			d.Open = open
			return nil
		}
	}
	if s.errDirectionNotFound != nil {
		return s.errDirectionNotFound
	}
	return repository.ErrDirectionNotFound
}

func (s *stubRecruitmentRepo) CreateDirection(_ context.Context, dir *domain.RecruitmentDirection) error {
	for _, d := range s.directions {
		if d.Code == dir.Code {
			return repository.ErrDirectionCodeExists
		}
	}
	dir.ID = len(s.directions) + 1
	s.directions = append(s.directions, dir)
	return nil
}

func (s *stubRecruitmentRepo) ListRounds(_ context.Context) ([]*domain.RecruitmentRound, error) {
	return s.rounds, nil
}

func (s *stubRecruitmentRepo) SetRoundAccepting(_ context.Context, roundID int, accepting bool) error {
	for _, r := range s.rounds {
		if r.ID == roundID {
			r.Accepting = accepting
			return nil
		}
	}
	return s.errRoundNotFound
}

func (s *stubRecruitmentRepo) GetActiveRound(_ context.Context) (*domain.RecruitmentRound, error) {
	for _, r := range s.rounds {
		if r.Accepting {
			return r, nil
		}
	}
	return nil, repository.ErrRoundNotFound
}

// GetRoundByID 供 SubmitToRound（表单模块 #92 的入口）使用。
func (s *stubRecruitmentRepo) GetRoundByID(_ context.Context, id int) (*domain.RecruitmentRound, error) {
	for _, r := range s.rounds {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, repository.ErrRoundNotFound
}

func (s *stubRecruitmentRepo) CreateApplication(_ context.Context, app *domain.Application) error {
	if s.duplicateOnCreate {
		return repository.ErrDuplicateApplication
	}
	app.ID = s.nextAppID
	s.nextAppID++
	s.applications[app.ID] = app
	return nil
}

func (s *stubRecruitmentRepo) ListApplicationsByUser(_ context.Context, userID int, page int, pageSize int) ([]*domain.Application, int64, error) {
	var apps []*domain.Application
	for _, a := range s.applications {
		if a.UserID == userID {
			apps = append(apps, a)
		}
	}
	// 按 id 倒序
	for i := 0; i < len(apps); i++ {
		for j := i + 1; j < len(apps); j++ {
			if apps[j].ID > apps[i].ID {
				apps[i], apps[j] = apps[j], apps[i]
			}
		}
	}
	total := int64(len(apps))
	// 简单分页
	start := (page - 1) * pageSize
	if start >= len(apps) {
		return []*domain.Application{}, total, nil
	}
	end := start + pageSize
	if end > len(apps) {
		end = len(apps)
	}
	return apps[start:end], total, nil
}

func (s *stubRecruitmentRepo) GetApplicationByID(_ context.Context, id int) (*domain.Application, error) {
	if a, ok := s.applications[id]; ok {
		return a, nil
	}
	return nil, s.errAppNotFound
}

// GetApplicationByUserRoundDirection 复刻三元组查询（#135）：
// 三元组是唯一约束的完整键，命中最多一条；不存在返回 ErrApplicationNotFound。
func (s *stubRecruitmentRepo) GetApplicationByUserRoundDirection(_ context.Context, userID int, roundID int, directionCode string) (*domain.Application, error) {
	for _, a := range s.applications {
		if a.UserID == userID && a.RoundID == roundID && a.DirectionCode == directionCode {
			return a, nil
		}
	}
	return nil, repository.ErrApplicationNotFound
}

func (s *stubRecruitmentRepo) ListApplications(_ context.Context, _ repository.ApplicationListFilter, page int, pageSize int) ([]*domain.Application, int64, error) {
	var apps []*domain.Application
	for _, a := range s.applications {
		apps = append(apps, a)
	}
	for i := 0; i < len(apps); i++ {
		for j := i + 1; j < len(apps); j++ {
			if apps[j].ID > apps[i].ID {
				apps[i], apps[j] = apps[j], apps[i]
			}
		}
	}
	total := int64(len(apps))
	start := (page - 1) * pageSize
	if start >= len(apps) {
		return []*domain.Application{}, total, nil
	}
	end := start + pageSize
	if end > len(apps) {
		end = len(apps)
	}
	return apps[start:end], total, nil
}

func (s *stubRecruitmentRepo) UpdateApplicationStatus(_ context.Context, id int, status domain.ApplicationStatus) (int64, error) {
	if a, ok := s.applications[id]; ok {
		a.Status = status
		return 1, nil
	}
	return 0, nil
}

func (s *stubRecruitmentRepo) AppendTimeline(_ context.Context, appID int, status domain.ApplicationStatus) error {
	s.timelines[appID] = append(s.timelines[appID], &domain.ApplicationTimeline{
		ApplicationID: appID,
		ToStatus:      string(status),
	})
	return nil
}

func (s *stubRecruitmentRepo) ListTimeline(_ context.Context, applicationID int) ([]*domain.ApplicationTimeline, error) {
	return s.timelines[applicationID], nil
}

/* ------------------------------------------------------------------ *
 * ProfileSource 桩
 * ------------------------------------------------------------------ */

type stubProfileSource struct {
	user *domain.User
	err  error
}

func (s *stubProfileSource) GetByID(_ context.Context, _ int) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

func recruitmentTestUser(id int) *domain.User {
	return &domain.User{
		ID:     id,
		Name:   "测试用户",
		Detail: domain.Detail{Email: "test@example.com", Class: "软工 2 班", Direction: domain.FrontEnd, StudentID: 20260001, QQ: "1044696157"},
		Parse:  1,
	}
}

func recruitmentFixture() (*RecruitmentUseCase, *stubRecruitmentRepo, *stubProfileSource) {
	repo := newStubRecruitmentRepo()
	profile := &stubProfileSource{user: recruitmentTestUser(1)}
	uc := &RecruitmentUseCase{repo: repo, profile: profile}
	return uc, repo, profile
}

/* ------------------------------------------------------------------ *
 * 测试用例
 * ------------------------------------------------------------------ */

// 验收项：未登录提交 → 由中间件拦截（authRequired），handler 层验 CurrentUser。
// 此处测服务层：提供不存在的用户时返回错误。
func TestSubmit_ProfileNotFound(t *testing.T) {
	uc, repo, profile := recruitmentFixture()
	profile.err = errors.New("user not found")

	_, err := uc.Submit(context.Background(), 999, "front-end")
	if err == nil {
		t.Error("用户不存在时应当报错")
	}

	if _, ok := repo.applications[999]; ok {
		t.Error("不应产生任何申请记录")
	}
	_ = repo
}

// 验收项：重复报名规则 —— 同轮同用户同方向只允许一条。
func TestSubmit_Duplicate(t *testing.T) {
	uc, repo, profile := recruitmentFixture()
	profile.err = nil
	profile.user = recruitmentTestUser(1)

	// 第一次成功
	app1, err := uc.Submit(context.Background(), 1, "front-end")
	if err != nil {
		t.Fatalf("首次报名应当成功: %v", err)
	}

	// 第二次触发唯一约束
	repo.duplicateOnCreate = true
	_, err = uc.Submit(context.Background(), 1, "front-end")
	if !errors.Is(err, repository.ErrDuplicateApplication) {
		t.Errorf("重复报名应当返回 ErrDuplicateApplication，实际: %v", err)
	}

	// 确保没有追加第二条
	count := 0
	for _, a := range repo.applications {
		if a.UserID == 1 && a.DirectionCode == "front-end" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("同方向同轮只应有 1 条记录，实际 %d 条", count)
	}
	_ = app1
}

// 验收项：关闭方向 / 表中不存在的方向拒绝报名。
func TestSubmit_DirectionClosed(t *testing.T) {
	uc, _, _ := recruitmentFixture()
	ctx := context.Background()

	_, err := uc.Submit(ctx, 1, "front-end")
	if err != nil {
		t.Fatalf("默认应开放，不应报错: %v", err)
	}

	// 关闭方向后拒绝（issue #128：准入判据是表内存在且 open=true，无白名单）
	if err := uc.SetDirectionOpen(ctx, "back-end", false); err != nil {
		t.Fatalf("关闭 back-end 失败: %v", err)
	}
	_, err = uc.Submit(ctx, 2, "back-end")
	if !errors.Is(err, ErrDirectionClosed) {
		t.Errorf("已关闭的方向应返回 ErrDirectionClosed，实际: %v", err)
	}

	// 表中不存在的方向拒绝（原白名单用例的等价语义）
	_, err = uc.Submit(ctx, 3, "not-in-table")
	if !errors.Is(err, ErrDirectionClosed) {
		t.Errorf("表中不存在的方向应返回 ErrDirectionClosed，实际: %v", err)
	}
}

// 验收项：无开放轮次时拒绝报名。
func TestSubmit_NoActiveRound(t *testing.T) {
	uc, repo, _ := recruitmentFixture()

	// 关闭所有轮次
	for _, r := range repo.rounds {
		r.Accepting = false
	}

	_, err := uc.Submit(context.Background(), 1, "front-end")
	if !errors.Is(err, ErrNoActiveRound) {
		t.Errorf("没有开放轮次应返回 ErrNoActiveRound，实际: %v", err)
	}

	if len(repo.applications) != 0 {
		t.Error("不应产生任何记录")
	}
}

// 验收项：用户只能读自己的申请（含时间线）。
func TestGetMyApplication_Ownership(t *testing.T) {
	uc, _, _ := recruitmentFixture()

	// 用户 1 报名
	_, err := uc.Submit(context.Background(), 1, "front-end")
	if err != nil {
		t.Fatalf("报名失败: %v", err)
	}

	// 用户 1 读自己的申请（id=100）
	app, tl, err := uc.GetMyApplication(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("用户读自己的申请应当成功: %v", err)
	}
	if app.UserID != 1 {
		t.Errorf("用户 ID 应为 1，实际 %d", app.UserID)
	}
	if len(tl) < 1 {
		t.Error("至少有一条初始时间线")
	}

	// 用户 2 试图读用户 1 的申请 → ErrNotOwned
	_, _, err = uc.GetMyApplication(context.Background(), 2, 100)
	if !errors.Is(err, ErrNotOwned) {
		t.Errorf("非本人申请应返回 ErrNotOwned，实际: %v", err)
	}
}

// 验收项：状态机 applied → reviewing → accepted | rejected。
func TestReview_StateMachine(t *testing.T) {
	uc, repo, _ := recruitmentFixture()

	// 创建一条申请
	_, err := uc.Submit(context.Background(), 1, "front-end")
	if err != nil {
		t.Fatalf("报名失败: %v", err)
	}

	// applied → reviewing：合法
	err = uc.Review(context.Background(), 100, domain.ApplicationReviewing)
	if err != nil {
		t.Errorf("applied → reviewing 应当合法: %v", err)
	}
	if repo.applications[100].Status != domain.ApplicationReviewing {
		t.Errorf("状态应为 reviewing，实际 %s", repo.applications[100].Status)
	}

	// reviewing → accepted：合法
	err = uc.Review(context.Background(), 100, domain.ApplicationAccepted)
	if err != nil {
		t.Errorf("reviewing → accepted 应当合法: %v", err)
	}

	// accepted → rejected：终态不可流转
	err = uc.Review(context.Background(), 100, domain.ApplicationRejected)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("终态不可再流转，应返回 ErrInvalidTransition，实际: %v", err)
	}

	// applied → accepted：跳过 reviewing，非法
	_, err = uc.Submit(context.Background(), 2, "back-end")
	if err != nil {
		t.Fatalf("报名失败: %v", err)
	}
	err = uc.Review(context.Background(), 101, domain.ApplicationAccepted)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("applied 不可直接到 accepted，应返回 ErrInvalidTransition，实际: %v", err)
	}
}

// 验收项：时间线记录完整（创建 + 每次流转）。
func TestReview_Timeline(t *testing.T) {
	uc, repo, _ := recruitmentFixture()

	_, err := uc.Submit(context.Background(), 1, "front-end")
	if err != nil {
		t.Fatalf("报名失败: %v", err)
	}

	// 时间线应有一条 applied
	tl, _ := repo.ListTimeline(context.Background(), 100)
	if len(tl) != 1 || tl[0].ToStatus != "applied" {
		t.Fatalf("初始时间线应为 [applied]，实际 %+v", tl)
	}

	// 流转到 reviewing
	err = uc.Review(context.Background(), 100, domain.ApplicationReviewing)
	if err != nil {
		t.Fatalf("流转失败: %v", err)
	}

	tl, _ = repo.ListTimeline(context.Background(), 100)
	if len(tl) != 2 || tl[1].ToStatus != "reviewing" {
		t.Errorf("时间线应为 [applied, reviewing]，实际 %+v", tl)
	}

	// 流转到 accepted
	err = uc.Review(context.Background(), 100, domain.ApplicationAccepted)
	if err != nil {
		t.Fatalf("流转失败: %v", err)
	}

	tl, _ = repo.ListTimeline(context.Background(), 100)
	if len(tl) != 3 || tl[2].ToStatus != "accepted" {
		t.Errorf("时间线应为 [applied, reviewing, accepted]，实际 %+v", tl)
	}
}

// 可报名列表只含 open=true 的方向（issue #128：方向表为唯一事实源，无白名单）。
// 夹具里 all 也是 open=true，按新语义应出现在列表——断言口径随之改为
// 「open 必须为 true」，而不是按 code 黑名单过滤。
func TestListActiveDirections_OpenOnly(t *testing.T) {
	uc, _, _ := recruitmentFixture()

	// 关掉 all，模拟它不该出现的场景
	if err := uc.SetDirectionOpen(context.Background(), "all", false); err != nil {
		t.Fatalf("关闭 all 失败: %v", err)
	}

	dirs, err := uc.ListActiveDirections(context.Background())
	if err != nil {
		t.Fatalf("读取方向失败: %v", err)
	}

	for _, d := range dirs {
		if !d.Open {
			t.Errorf("可报名列表不应含关闭方向: %s", d.Code)
		}
		if d.Code == "all" {
			t.Error("已关闭的 all 不应出现在可报名列表")
		}
	}

	// 开放的 front-end/back-end/agent 仍在列表
	codes := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		codes[d.Code] = true
	}
	for _, want := range []string{"front-end", "back-end", "agent"} {
		if !codes[want] {
			t.Errorf("开放方向 %s 应在可报名列表", want)
		}
	}
}

// 表中不存在的 code 拒绝配置开关（替代原白名单语义，issue #128）。
func TestSetDirectionOpen_RejectUnknownCode(t *testing.T) {
	uc, repo, _ := recruitmentFixture()
	repo.errDirectionNotFound = repository.ErrDirectionNotFound

	err := uc.SetDirectionOpen(context.Background(), "not-a-direction", false)
	if !errors.Is(err, repository.ErrDirectionNotFound) {
		t.Errorf("表中不存在的 code 应返回 ErrDirectionNotFound，实际: %v", err)
	}

	// 表中存在的 code（含新创建的）可正常配置
	if err := uc.SetDirectionOpen(context.Background(), "front-end", false); err != nil {
		t.Errorf("表中存在的 front-end 应可正常配置: %v", err)
	}
}

// 创建方向：默认 open=false；成功后可被 PATCH 开启并进入用户端可报名列表。
func TestCreateDirection_DefaultClosedThenOpen(t *testing.T) {
	uc, _, _ := recruitmentFixture()
	ctx := context.Background()

	dir, err := uc.CreateDirection(ctx, "algo", "算法", 50)
	if err != nil {
		t.Fatalf("CreateDirection() error = %v", err)
	}
	if dir.Open {
		t.Error("创建的方向必须默认 open=false（防误开放）")
	}
	if dir.Code != "algo" || dir.Name != "算法" || dir.SortOrder != 50 {
		t.Errorf("创建的方向字段不符: %+v", dir)
	}

	// 未开启前：不出现在用户端可报名列表
	dirs, err := uc.ListActiveDirections(ctx)
	if err != nil {
		t.Fatalf("ListActiveDirections() error = %v", err)
	}
	for _, d := range dirs {
		if d.Code == "algo" {
			t.Error("未开启的新方向不应出现在可报名列表")
		}
	}

	// PATCH 开启后：立即出现
	if err := uc.SetDirectionOpen(ctx, "algo", true); err != nil {
		t.Fatalf("开启新方向失败: %v", err)
	}
	dirs, err = uc.ListActiveDirections(ctx)
	if err != nil {
		t.Fatalf("ListActiveDirections() error = %v", err)
	}
	found := false
	for _, d := range dirs {
		if d.Code == "algo" && d.Open {
			found = true
		}
	}
	if !found {
		t.Error("PATCH 开启后新方向应出现在可报名列表")
	}
}

// 创建方向：code 格式非法 / name 缺失 → 业务错误，不落库。
func TestCreateDirection_Validation(t *testing.T) {
	uc, repo, _ := recruitmentFixture()
	ctx := context.Background()
	before := len(repo.directions)

	cases := []struct {
		desc    string
		code    string
		name    string
		wantErr error
	}{
		{"空code", "  ", "名称", ErrInvalidDirectionCode},
		{"大写", "Front", "名称", ErrInvalidDirectionCode},
		{"非法字符", "front_end", "名称", ErrInvalidDirectionCode},
		{"含空格", "front end", "名称", ErrInvalidDirectionCode},
		{"超长33位", "a33", "名称", ErrInvalidDirectionCode},
		{"空name", "new-dir", "  ", ErrInvalidDirectionName},
		{"合法", "new-dir", "新方向", nil},
	}
	// 超长用例单独构造（33 个字符）
	longCode := ""
	for i := 0; i < 33; i++ {
		longCode += "a"
	}
	cases[4].code = longCode

	for _, tc := range cases {
		_, err := uc.CreateDirection(ctx, tc.code, tc.name, 0)
		if tc.wantErr == nil {
			if err != nil {
				t.Errorf("用例 %q 应成功，实际: %v", tc.desc, err)
			}
			continue
		}
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("用例 %q 应返回 %v，实际: %v", tc.desc, tc.wantErr, err)
		}
	}

	// 只有「合法」那条落库
	if len(repo.directions) != before+1 {
		t.Errorf("非法创建不应落库，期望方向数 %d，实际 %d", before+1, len(repo.directions))
	}
}

// 创建方向：code 重复 → ErrDirectionCodeExists，已有行不被修改。
func TestCreateDirection_DuplicateCode(t *testing.T) {
	uc, _, _ := recruitmentFixture()
	ctx := context.Background()

	_, err := uc.CreateDirection(ctx, "front-end", "冒名顶替", 999)
	if !errors.Is(err, repository.ErrDirectionCodeExists) {
		t.Fatalf("重复 code 应返回 ErrDirectionCodeExists，实际: %v", err)
	}

	// 已有行不被修改
	dirs, _ := uc.ListDirections(ctx)
	for _, d := range dirs {
		if d.Code == "front-end" {
			if d.Name != "前端开发" || d.SortOrder != 10 {
				t.Errorf("重复创建不得修改已有行，实际 name=%q sort=%d", d.Name, d.SortOrder)
			}
		}
	}
}

// 对表中存在的新 code 提交报名成功（不再被白名单拦截，issue #128 验收）。
func TestSubmit_NewDirectionFromTable(t *testing.T) {
	uc, _, _ := recruitmentFixture()
	ctx := context.Background()

	if _, err := uc.CreateDirection(ctx, "algo", "算法", 50); err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	if err := uc.SetDirectionOpen(ctx, "algo", true); err != nil {
		t.Fatalf("开启方向失败: %v", err)
	}

	app, err := uc.Submit(ctx, 1, "algo")
	if err != nil {
		t.Fatalf("表中存在的新 code 应可报名，实际: %v", err)
	}
	if app.DirectionCode != "algo" {
		t.Errorf("申请方向应为 algo，实际 %q", app.DirectionCode)
	}
}
