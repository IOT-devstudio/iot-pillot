package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 桩：表单仓储（FormUseCase 只依赖接口，不需要 DB）
 * ------------------------------------------------------------------ */

type stubFormRepo struct {
	forms       map[int]*domain.Form
	fields      map[int][]*domain.FormField
	submissions map[int]*domain.FormSubmission
	nextFormID  int
	nextFieldID int
	nextSubID   int

	// 模拟约束冲突
	duplicateOnCreateSubmission bool
	duplicateOnCreateForm       bool
	tokenAttempts               int
}

func newStubFormRepo() *stubFormRepo {
	return &stubFormRepo{
		forms:       make(map[int]*domain.Form),
		fields:      make(map[int][]*domain.FormField),
		submissions: make(map[int]*domain.FormSubmission),
		nextFormID:  10,
		nextFieldID: 100,
		nextSubID:   500,
	}
}

func (s *stubFormRepo) CreateForm(_ context.Context, form *domain.Form, fields []*domain.FormField) error {
	s.tokenAttempts++
	if s.duplicateOnCreateForm && s.tokenAttempts == 1 {
		return repository.ErrDuplicateShareToken
	}
	form.ID = s.nextFormID
	s.nextFormID++
	s.forms[form.ID] = form

	stored := make([]*domain.FormField, 0, len(fields))
	for i, f := range fields {
		f.ID = s.nextFieldID
		s.nextFieldID++
		f.FormID = form.ID
		f.SortOrder = i
		stored = append(stored, f)
	}
	s.fields[form.ID] = stored
	return nil
}

func (s *stubFormRepo) GetFormByID(_ context.Context, id int) (*domain.Form, error) {
	if f, ok := s.forms[id]; ok {
		return f, nil
	}
	return nil, repository.ErrFormNotFound
}

func (s *stubFormRepo) GetFormByShareToken(_ context.Context, token string) (*domain.Form, error) {
	for _, f := range s.forms {
		if f.ShareToken == token {
			return f, nil
		}
	}
	return nil, repository.ErrFormNotFound
}

func (s *stubFormRepo) ListForms(_ context.Context, page int, pageSize int) ([]*domain.Form, int64, error) {
	var out []*domain.Form
	for _, f := range s.forms {
		out = append(out, f)
	}
	total := int64(len(out))
	start := (page - 1) * pageSize
	if start >= len(out) {
		return []*domain.Form{}, total, nil
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (s *stubFormRepo) UpdateForm(_ context.Context, form *domain.Form, fields []*domain.FormField) error {
	current, ok := s.forms[form.ID]
	if !ok {
		return repository.ErrFormNotFound
	}
	current.Title = form.Title
	current.Description = form.Description
	current.IsOpen = form.IsOpen

	if fields != nil {
		existing := s.fields[form.ID]
		owned := make(map[int]bool, len(existing))
		for _, e := range existing {
			owned[e.ID] = true
		}
		next := make([]*domain.FormField, 0, len(fields))
		for i, f := range fields {
			if f.ID > 0 && !owned[f.ID] {
				return repository.ErrFormFieldNotFound
			}
			if f.ID == 0 {
				f.ID = s.nextFieldID
				s.nextFieldID++
			}
			f.FormID = form.ID
			f.SortOrder = i
			next = append(next, f)
		}
		s.fields[form.ID] = next
	}
	return nil
}

func (s *stubFormRepo) SetFormOpen(_ context.Context, id int, open bool) error {
	f, ok := s.forms[id]
	if !ok {
		return repository.ErrFormNotFound
	}
	f.IsOpen = open
	return nil
}

func (s *stubFormRepo) ListFields(_ context.Context, formID int) ([]*domain.FormField, error) {
	if _, ok := s.forms[formID]; !ok {
		return nil, repository.ErrFormNotFound
	}
	return s.fields[formID], nil
}

func (s *stubFormRepo) CountSubmissions(_ context.Context, formIDs []int) (map[int]int64, error) {
	counts := make(map[int]int64, len(formIDs))
	for _, sub := range s.submissions {
		for _, id := range formIDs {
			if sub.FormID == id {
				counts[id]++
			}
		}
	}
	return counts, nil
}

// ListOpenFormsByRound 复刻仓储的取表规则（#135）：
// 只取该轮 is_open 的表，按 (direction_code, updated_at DESC, id DESC) 排序。
// 排序在此实现而不是让调用方兜底，测试才能验证"同方向取最新一张"这条验收标准。
func (s *stubFormRepo) ListOpenFormsByRound(_ context.Context, roundID int) ([]*domain.Form, error) {
	out := make([]*domain.Form, 0)
	for _, f := range s.forms {
		if f.RoundID == roundID && f.IsOpen {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DirectionCode != out[j].DirectionCode {
			return out[i].DirectionCode < out[j].DirectionCode
		}
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

func (s *stubFormRepo) CreateSubmission(_ context.Context, submission *domain.FormSubmission) error {
	if s.duplicateOnCreateSubmission {
		return repository.ErrDuplicateSubmission
	}
	for _, existing := range s.submissions {
		// 数据库唯一索引 (form_id, submitter_id) 的桩等价物
		if existing.FormID == submission.FormID && existing.SubmitterID == submission.SubmitterID {
			return repository.ErrDuplicateSubmission
		}
	}
	submission.ID = s.nextSubID
	s.nextSubID++
	s.submissions[submission.ID] = submission
	return nil
}

func (s *stubFormRepo) GetSubmissionBySubmitter(_ context.Context, formID int, submitterID int) (*domain.FormSubmission, error) {
	for _, sub := range s.submissions {
		if sub.FormID == formID && sub.SubmitterID == submitterID {
			return sub, nil
		}
	}
	return nil, repository.ErrSubmissionNotFound
}

func (s *stubFormRepo) ListSubmissions(_ context.Context, formID int, _ int, _ int) ([]*domain.FormSubmission, int64, error) {
	var out []*domain.FormSubmission
	for _, sub := range s.submissions {
		if sub.FormID == formID {
			out = append(out, sub)
		}
	}
	return out, int64(len(out)), nil
}

func (s *stubFormRepo) GetSubmission(_ context.Context, formID int, submissionID int) (*domain.FormSubmission, error) {
	if sub, ok := s.submissions[submissionID]; ok && sub.FormID == formID {
		return sub, nil
	}
	return nil, repository.ErrSubmissionNotFound
}

/* ------------------------------------------------------------------ *
 * 桩：招新目录（轮次 / 方向）
 * ------------------------------------------------------------------ */

type stubRecruitmentDirectory struct {
	rounds     map[int]*domain.RecruitmentRound
	directions map[string]*domain.RecruitmentDirection
}

func newStubRecruitmentDirectory() *stubRecruitmentDirectory {
	return &stubRecruitmentDirectory{
		rounds: map[int]*domain.RecruitmentRound{
			1: {ID: 1, RoundNum: 1, Name: "2026 秋季招新", Accepting: true},
			2: {ID: 2, RoundNum: 2, Name: "2027 春季招新", Accepting: false},
		},
		directions: map[string]*domain.RecruitmentDirection{
			"front-end": {ID: 1, Code: "front-end", Name: "前端开发", Open: true, SortOrder: 10},
			"all":       {ID: 4, Code: "all", Name: "全栈", Open: true, SortOrder: 40},
		},
	}
}

func (s *stubRecruitmentDirectory) GetRoundByID(_ context.Context, id int) (*domain.RecruitmentRound, error) {
	if r, ok := s.rounds[id]; ok {
		return r, nil
	}
	return nil, repository.ErrRoundNotFound
}

func (s *stubRecruitmentDirectory) GetDirectionByCode(_ context.Context, code string) (*domain.RecruitmentDirection, error) {
	if d, ok := s.directions[code]; ok {
		return d, nil
	}
	return nil, repository.ErrDirectionNotFound
}

/* ------------------------------------------------------------------ *
 * 桩：申请创建者（模拟 #88 SubmitToRound 的语义）
 * ------------------------------------------------------------------ */

type stubApplicationCreator struct {
	profile *domain.User
	apps    map[int]*domain.Application
	nextID  int
	// seen 是最新申请的键集合：等价于 (user_id, round_id, direction_code) 唯一约束
	seen map[string]bool
	// byKey 是同一键 → 申请对象，供复用路径（#135）查询。
	// 与 seen 同步维护：唯一约束既挡住重复创建，也让"已有申请"可被查回来。
	byKey map[string]*domain.Application
	// errRoundClosed 模拟表单绑定轮次未开放
	roundClosed map[int]bool
	calls       int
}

func newStubApplicationCreator(profile *domain.User) *stubApplicationCreator {
	return &stubApplicationCreator{
		profile:     profile,
		apps:        make(map[int]*domain.Application),
		nextID:      900,
		seen:        make(map[string]bool),
		byKey:       make(map[string]*domain.Application),
		roundClosed: make(map[int]bool),
	}
}

func applicationKey(userID int, roundID int, directionCode string) string {
	return fmt.Sprintf("%d|%d|%s", userID, roundID, directionCode)
}

func (s *stubApplicationCreator) SubmitToRound(_ context.Context, userID int, roundID int, directionCode string) (*domain.Application, error) {
	s.calls++
	if s.roundClosed[roundID] {
		return nil, ErrRoundClosed
	}
	key := applicationKey(userID, roundID, directionCode)
	if s.seen[key] {
		return nil, repository.ErrDuplicateApplication
	}
	s.seen[key] = true

	// 与 #88 一致：资料取自用户快照，不看请求体
	app := &domain.Application{
		ID:            s.nextID,
		UserID:        userID,
		RoundID:       roundID,
		DirectionCode: directionCode,
		Status:        domain.ApplicationApplied,
		Name:          s.profile.Name,
		Email:         s.profile.Detail.Email,
		Class:         s.profile.Detail.Class,
		StudentID:     int64(s.profile.Detail.StudentID),
		QQ:            s.profile.Detail.QQ,
	}
	s.nextID++
	s.apps[app.ID] = app
	s.byKey[key] = app
	return app, nil
}

// GetApplicationByUserRoundDirection 复刻三元组查询（#135 的复用路径）。
// 不存在返回 repository.ErrApplicationNotFound，与真实仓储一致。
func (s *stubApplicationCreator) GetApplicationByUserRoundDirection(_ context.Context, userID int, roundID int, directionCode string) (*domain.Application, error) {
	if app, ok := s.byKey[applicationKey(userID, roundID, directionCode)]; ok {
		return app, nil
	}
	return nil, repository.ErrApplicationNotFound
}

// seedApplication 预置一条既有申请（模拟"用户先走报名、之后来补面试"）。
// 同时登记唯一约束键，保证后续直接创建同键仍会拿到重复报名错误。
func (s *stubApplicationCreator) seedApplication(userID int, roundID int, directionCode string, status domain.ApplicationStatus) *domain.Application {
	app := &domain.Application{
		ID:            s.nextID,
		UserID:        userID,
		RoundID:       roundID,
		DirectionCode: directionCode,
		Status:        status,
		Name:          s.profile.Name,
		Email:         s.profile.Detail.Email,
		Class:         s.profile.Detail.Class,
		StudentID:     int64(s.profile.Detail.StudentID),
		QQ:            s.profile.Detail.QQ,
	}
	s.nextID++
	s.apps[app.ID] = app
	key := applicationKey(userID, roundID, directionCode)
	s.seen[key] = true
	s.byKey[key] = app
	return app
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

func formTestUser() *domain.User {
	return &domain.User{
		ID:     1,
		Name:   "测试用户",
		Detail: domain.Detail{Email: "account@example.com", Class: "软工 2 班", StudentID: 20260001, QQ: "1044696157"},
		Parse:  1,
	}
}

func formFixture() (*FormUseCase, *stubFormRepo, *stubApplicationCreator) {
	repo := newStubFormRepo()
	creator := newStubApplicationCreator(formTestUser())
	uc := NewFormUseCase(repo, newStubRecruitmentDirectory(), creator)
	return uc, repo, creator
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// fiveFieldForm 五种字段齐全的建表入参。
func fiveFieldForm() CreateFormInput {
	return CreateFormInput{
		Title:         "2026 秋季招新报名表",
		Description:   "请如实填写",
		RoundID:       1,
		DirectionCode: "front-end",
		Fields: []FormFieldInput{
			{Type: "text", Label: "姓名", Required: true},
			{Type: "email", Label: "联系邮箱", Required: true},
			{Type: "textarea", Label: "自我介绍", Placeholder: "200 字以内"},
			{Type: "select", Label: "投入时长", Required: true, Options: []string{"每周 5 小时", "每周 10 小时"}},
			{Type: "checkbox", Label: "感兴趣的活动"},
		},
	}
}

// openForm 建表并开放，返回表单与字段 id（按 label → id）。
func openForm(t *testing.T, uc *FormUseCase) (*domain.Form, map[string]string) {
	t.Helper()

	form, fields, err := uc.CreateForm(context.Background(), fiveFieldForm())
	if err != nil {
		t.Fatalf("建表应当成功: %v", err)
	}
	if form.IsOpen {
		t.Error("新建表单必须是关闭状态（由管理端显式开放）")
	}
	if len(fields) != 5 {
		t.Fatalf("应有 5 个字段，实际 %d", len(fields))
	}

	if _, _, _, err := uc.UpdateForm(context.Background(), form.ID, UpdateFormInput{IsOpen: boolPtr(true)}); err != nil {
		t.Fatalf("开放表单应当成功: %v", err)
	}
	if !form.IsOpen {
		t.Error("开放后 IsOpen 应为 true")
	}

	ids := make(map[string]string, len(fields))
	for _, f := range fields {
		ids[f.Label] = fmt.Sprintf("%d", f.ID)
	}
	return form, ids
}

/* ------------------------------------------------------------------ *
 * 建表校验
 * ------------------------------------------------------------------ */

func TestCreateForm_ShareTokenAndBinding(t *testing.T) {
	uc, _, _ := formFixture()

	form, fields, err := uc.CreateForm(context.Background(), fiveFieldForm())
	if err != nil {
		t.Fatalf("建表应当成功: %v", err)
	}

	if len(form.ShareToken) != shareTokenBytes*2 {
		t.Errorf("分享令牌应为 %d 位 hex，实际 %d 位: %q", shareTokenBytes*2, len(form.ShareToken), form.ShareToken)
	}
	if form.RoundID != 1 || form.DirectionCode != "front-end" {
		t.Errorf("绑定应为 (1, front-end)，实际 (%d, %s)", form.RoundID, form.DirectionCode)
	}
	for i, f := range fields {
		if f.SortOrder != i {
			t.Errorf("字段排序应等于入参顺序：第 %d 个字段 SortOrder=%d", i, f.SortOrder)
		}
		if f.ID == 0 {
			t.Errorf("落库字段应当带上生成的 id：%+v", f)
		}
	}
}

func TestCreateForm_ShareTokenRetry(t *testing.T) {
	uc, repo, _ := formFixture()
	repo.duplicateOnCreateForm = true

	form, _, err := uc.CreateForm(context.Background(), fiveFieldForm())
	if err != nil {
		t.Fatalf("令牌冲突应当换一个再试并成功: %v", err)
	}
	if repo.tokenAttempts != 2 {
		t.Errorf("应当尝试两次（一次冲突 + 一次成功），实际 %d 次", repo.tokenAttempts)
	}
	if form.ID == 0 {
		t.Error("重试后应当拿到表单 id")
	}
}

func TestCreateForm_RejectsBadConfig(t *testing.T) {
	cases := []struct {
		name string
		in   CreateFormInput
		want string
	}{
		{"空标题", CreateFormInput{Title: "  ", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "text", Label: "姓名"}}}, "标题不能为空"},
		{"无字段", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end"}, "至少需要一个字段"},
		{"未知类型", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "date", Label: "日期"}}}, "不在支持范围"},
		{"select 无选项", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "select", Label: "时长"}}}, "至少要有一个候选项"},
		{"select 空选项", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "select", Label: "时长", Options: []string{"  "}}}}, "空候选项"},
		{"select 重复选项", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "select", Label: "时长", Options: []string{"A", "A"}}}}, "重复"},
		{"非 select 带选项", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "text", Label: "姓名", Options: []string{"A"}}}}, "只有 select"},
		{"字段无标题", CreateFormInput{Title: "表", RoundID: 1, DirectionCode: "front-end", Fields: []FormFieldInput{{Type: "text", Label: " "}}}, "缺少标题"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, _, _ := formFixture()
			_, _, err := uc.CreateForm(context.Background(), tc.in)
			if !errors.Is(err, ErrFormConfig) {
				t.Fatalf("应返回 ErrFormConfig，实际: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("错误信息应包含 %q，实际: %v", tc.want, err)
			}
		})
	}
}

func TestCreateForm_MissingRoundOrDirection(t *testing.T) {
	uc, _, _ := formFixture()

	in := fiveFieldForm()
	in.RoundID = 999
	if _, _, err := uc.CreateForm(context.Background(), in); !errors.Is(err, repository.ErrRoundNotFound) {
		t.Errorf("轮次不存在应返回 ErrRoundNotFound，实际: %v", err)
	}

	in = fiveFieldForm()
	in.DirectionCode = "sales"
	if _, _, err := uc.CreateForm(context.Background(), in); !errors.Is(err, repository.ErrDirectionNotFound) {
		t.Errorf("方向不存在应返回 ErrDirectionNotFound，实际: %v", err)
	}
}

func TestUpdateForm_FieldsIncremental(t *testing.T) {
	uc, _, _ := formFixture()
	form, ids := openForm(t, uc)

	// 改标题 + 原地改第二个字段的标题 + 新增一个字段 + 删掉一个字段
	newFields := []FormFieldInput{
		{ID: mustAtoi(t, ids["姓名"]), Type: "text", Label: "真实姓名", Required: true},
		{ID: mustAtoi(t, ids["联系邮箱"]), Type: "email", Label: "联系邮箱", Required: true},
		{Type: "textarea", Label: "补充说明"},
	}
	_, fields, _, err := uc.UpdateForm(context.Background(), form.ID, UpdateFormInput{
		Title:  strPtr("改名后的表"),
		Fields: &newFields,
	})
	if err != nil {
		t.Fatalf("改表应当成功: %v", err)
	}

	if form.Title != "改名后的表" {
		t.Errorf("标题应已更新，实际 %q", form.Title)
	}
	if len(fields) != 3 {
		t.Fatalf("字段应为 3 个（改 2 + 新增 1，删掉 2），实际 %d", len(fields))
	}
	// 带 id 的字段必须保留 id：历史答案靠它对得上
	if fields[0].ID != mustAtoi(t, ids["姓名"]) || fields[0].Label != "真实姓名" {
		t.Errorf("带 id 的字段应原地更新并保留 id，实际 %+v", fields[0])
	}
	if fields[2].ID == 0 {
		t.Error("新增字段应当拿到新 id")
	}
	for i, f := range fields {
		if f.SortOrder != i {
			t.Errorf("字段顺序应跟随入参：第 %d 个 SortOrder=%d", i, f.SortOrder)
		}
	}
}

func TestUpdateForm_RejectsForeignFieldID(t *testing.T) {
	uc, _, _ := formFixture()
	form, _ := openForm(t, uc)

	fields := []FormFieldInput{{ID: 123456, Type: "text", Label: "不属于该表的字段"}}
	_, _, _, err := uc.UpdateForm(context.Background(), form.ID, UpdateFormInput{Fields: &fields})
	if !errors.Is(err, repository.ErrFormFieldNotFound) {
		t.Errorf("外部字段 id 应返回 ErrFormFieldNotFound，实际: %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 提交：拒绝路径
 * ------------------------------------------------------------------ */

func TestSubmit_InvalidShareToken(t *testing.T) {
	uc, repo, _ := formFixture()

	_, _, err := uc.SubmitByShareToken(context.Background(), "not-a-real-token", 1, map[string]any{})
	if !errors.Is(err, repository.ErrFormNotFound) {
		t.Fatalf("无效分享令牌应返回 ErrFormNotFound，实际: %v", err)
	}
	if len(repo.submissions) != 0 {
		t.Error("无效令牌不应产生任何提交记录")
	}
}

func TestSubmit_ClosedForm(t *testing.T) {
	uc, repo, creator := formFixture()
	form, _ := openForm(t, uc)

	// 关闭表单
	if _, _, _, err := uc.UpdateForm(context.Background(), form.ID, UpdateFormInput{IsOpen: boolPtr(false)}); err != nil {
		t.Fatalf("关闭表单应当成功: %v", err)
	}

	_, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{})
	if !errors.Is(err, ErrFormClosed) {
		t.Fatalf("关闭的表单应返回 ErrFormClosed，实际: %v", err)
	}
	if len(repo.submissions) != 0 {
		t.Error("关闭表单不应产生提交记录")
	}
	if creator.calls != 0 {
		t.Error("关闭表单不应走到建申请这一步")
	}
}

func TestSubmit_ValidationFailures(t *testing.T) {
	cases := []struct {
		name   string
		values func(ids map[string]string) map[string]any
		want   string
	}{
		{
			"缺必填",
			func(ids map[string]string) map[string]any {
				return map[string]any{ids["联系邮箱"]: "a@b.com", ids["投入时长"]: "每周 5 小时"}
			},
			"必填项「姓名」未填写",
		},
		{
			"非法邮箱",
			func(ids map[string]string) map[string]any {
				return map[string]any{ids["姓名"]: "张三", ids["联系邮箱"]: "not-an-email", ids["投入时长"]: "每周 5 小时"}
			},
			"不是合法的邮箱地址",
		},
		{
			"select 非法选项",
			func(ids map[string]string) map[string]any {
				return map[string]any{ids["姓名"]: "张三", ids["联系邮箱"]: "a@b.com", ids["投入时长"]: "每周 50 小时"}
			},
			"不在候选项内",
		},
		{
			"未知字段",
			func(ids map[string]string) map[string]any {
				return map[string]any{"99999": "野字段"}
			},
			"不属于该表单的字段",
		},
		{
			"text 字段给了数组",
			func(ids map[string]string) map[string]any {
				// 必填项都给出，让错误只可能来自"类型不对"这一个原因
				return map[string]any{
					ids["姓名"]:   []any{"张三"},
					ids["联系邮箱"]: "a@b.com",
					ids["投入时长"]: "每周 5 小时",
				}
			},
			"需要文本答案",
		},
		{
			"checkbox 给了字符串",
			func(ids map[string]string) map[string]any {
				return map[string]any{
					ids["姓名"]:     "张三",
					ids["联系邮箱"]:   "a@b.com",
					ids["投入时长"]:   "每周 5 小时",
					ids["感兴趣的活动"]: "不应该是字符串",
				}
			},
			"需要多选答案",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, repo, creator := formFixture()
			form, ids := openForm(t, uc)

			_, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, tc.values(ids))
			if !errors.Is(err, ErrAnswerInvalid) {
				t.Fatalf("应返回 ErrAnswerInvalid，实际: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("错误信息应包含 %q，实际: %v", tc.want, err)
			}
			if len(repo.submissions) != 0 {
				t.Error("校验失败不应产生提交记录")
			}
			if creator.calls != 0 {
				t.Error("校验失败不应建申请（零副作用）")
			}
		})
	}
}

/* ------------------------------------------------------------------ *
 * 提交：成功与防重
 * ------------------------------------------------------------------ */

func TestSubmit_SuccessLinksApplication(t *testing.T) {
	uc, repo, creator := formFixture()
	form, ids := openForm(t, uc)

	submission, application, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 7, map[string]any{
		ids["姓名"]:     "张三",
		ids["联系邮箱"]:   "zhangsan@example.com",
		ids["投入时长"]:   "每周 10 小时",
		ids["感兴趣的活动"]: []any{"技术分享", "线下hackathon", "技术分享"},
	})
	if err != nil {
		t.Fatalf("提交应当成功: %v", err)
	}

	if submission.ID == 0 {
		t.Error("提交应当落库并带上 id")
	}
	if submission.SubmitterID != 7 {
		t.Errorf("提交人应取自令牌（7），实际 %d", submission.SubmitterID)
	}
	if submission.ApplicationID != application.ID {
		t.Errorf("提交应关联到刚创建的申请：submission.application_id=%d application.id=%d",
			submission.ApplicationID, application.ID)
	}
	if application.UserID != 7 || application.DirectionCode != "front-end" {
		t.Errorf("申请归属/方向应来自令牌与表单绑定，实际 %+v", application)
	}

	// 未填的非必填字段不落键
	if _, ok := submission.Answers[ids["自我介绍"]]; ok {
		t.Error("未填写的非必填字段不应出现在答案里")
	}
	if got := submission.Answers[ids["姓名"]]; got != "张三" {
		t.Errorf("姓名答案应为 张三，实际 %v", got)
	}
	// checkbox 去重
	chosen, ok := submission.Answers[ids["感兴趣的活动"]].([]string)
	if !ok {
		t.Fatalf("checkbox 答案应为字符串数组，实际 %T", submission.Answers[ids["感兴趣的活动"]])
	}
	if len(chosen) != 2 {
		t.Errorf("checkbox 答案应去重成 2 项，实际 %v", chosen)
	}
	if len(repo.submissions) != 1 || len(creator.apps) != 1 {
		t.Errorf("应恰好 1 条提交与 1 条申请，实际 提交=%d 申请=%d", len(repo.submissions), len(creator.apps))
	}
}

func TestSubmit_DuplicateRejectedWithoutSecondApplication(t *testing.T) {
	uc, repo, creator := formFixture()
	form, ids := openForm(t, uc)
	values := map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	}

	if _, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, values); err != nil {
		t.Fatalf("首次提交应当成功: %v", err)
	}

	_, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, values)
	if !errors.Is(err, repository.ErrDuplicateSubmission) {
		t.Fatalf("重复提交应返回 ErrDuplicateSubmission，实际: %v", err)
	}

	if len(repo.submissions) != 1 {
		t.Errorf("重复提交不应产生第二条提交，实际 %d 条", len(repo.submissions))
	}
	if len(creator.apps) != 1 {
		t.Errorf("重复提交不应产生第二条申请，实际 %d 条", len(creator.apps))
	}
}

// 并发/边界：唯一索引在最后一道拦下时也不该多出提交记录。
func TestSubmit_DuplicateFromUniqueIndex(t *testing.T) {
	uc, repo, _ := formFixture()
	form, ids := openForm(t, uc)

	repo.duplicateOnCreateSubmission = true
	_, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	})
	if !errors.Is(err, repository.ErrDuplicateSubmission) {
		t.Fatalf("唯一索引冲突应翻译成 ErrDuplicateSubmission，实际: %v", err)
	}
	if len(repo.submissions) != 0 {
		t.Error("冲突时不应留下提交记录")
	}
}

// 表单绑定轮次未开放报名 → 冒泡招新域错误，且不产生提交记录。
func TestSubmit_RoundClosed(t *testing.T) {
	uc, repo, creator := formFixture()
	form, ids := openForm(t, uc)
	creator.roundClosed[form.RoundID] = true

	_, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	})
	if !errors.Is(err, ErrRoundClosed) {
		t.Fatalf("轮次未开放应返回 ErrRoundClosed，实际: %v", err)
	}
	if len(repo.submissions) != 0 {
		t.Error("轮次未开放不应产生提交记录")
	}
}

// 已有同轮同方向申请（先走 /recruitment/applications 报过）→ #135 起改为复用：
// 不新增申请，只落一条提交，且提交的 application_id 指向那条既有申请。
//
// 这条用例的前身断言的是"冒泡 ErrDuplicateApplication"（旧行为）。行为按 #135
// 刻意改了，断言随之改写，不是删除 —— 旧语义由下面
// TestSubmit_NoApplicationStillCreates 与直接报名入口的测试继续守住。
func TestSubmit_ReusesExistingApplication(t *testing.T) {
	uc, repo, creator := formFixture()
	form, ids := openForm(t, uc)

	// 用户先通过普通报名入口报过同一轮同一方向
	existing := creator.seedApplication(1, form.RoundID, form.DirectionCode, domain.ApplicationApplied)
	before := len(creator.apps)

	submission, application, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	})
	if err != nil {
		t.Fatalf("已有申请时补交面试应当成功，实际: %v", err)
	}

	if application.ID != existing.ID {
		t.Errorf("应复用既有申请 %d，实际拿到 %d", existing.ID, application.ID)
	}
	if submission.ApplicationID != existing.ID {
		t.Errorf("提交记录的 application_id 应指向既有申请 %d，实际 %d", existing.ID, submission.ApplicationID)
	}
	if len(creator.apps) != before {
		t.Errorf("不应新增申请：之前 %d 条，现在 %d 条", before, len(creator.apps))
	}
	if len(repo.submissions) != 1 {
		t.Errorf("应恰有 1 条提交记录，实际 %d 条", len(repo.submissions))
	}
	if creator.calls != 0 {
		t.Errorf("复用路径不应调用 SubmitToRound，实际调用 %d 次", creator.calls)
	}
}

// 已有 reviewing 申请同样允许补交（仅 applied / reviewing 可补交）。
func TestSubmit_ReusesReviewingApplication(t *testing.T) {
	uc, repo, creator := formFixture()
	form, ids := openForm(t, uc)

	existing := creator.seedApplication(1, form.RoundID, form.DirectionCode, domain.ApplicationReviewing)

	_, application, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	})
	if err != nil {
		t.Fatalf("reviewing 申请补交应当成功，实际: %v", err)
	}
	if application.ID != existing.ID {
		t.Errorf("应复用 reviewing 申请 %d，实际 %d", existing.ID, application.ID)
	}
	if len(repo.submissions) != 1 {
		t.Errorf("应恰有 1 条提交记录，实际 %d 条", len(repo.submissions))
	}
}

// 用户**无**申请时提交表单：新建一条申请 + 一条提交记录（行为同现状，不能因 #135 回归）。
func TestSubmit_NoApplicationStillCreates(t *testing.T) {
	uc, repo, creator := formFixture()
	form, ids := openForm(t, uc)

	submission, application, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	})
	if err != nil {
		t.Fatalf("无申请时提交应当成功，实际: %v", err)
	}

	if creator.calls != 1 {
		t.Errorf("无申请时应调用一次 SubmitToRound 新建，实际 %d 次", creator.calls)
	}
	if len(creator.apps) != 1 {
		t.Errorf("应新增 1 条申请，实际 %d 条", len(creator.apps))
	}
	if len(repo.submissions) != 1 {
		t.Errorf("应恰有 1 条提交记录，实际 %d 条", len(repo.submissions))
	}
	if submission.ApplicationID != application.ID {
		t.Errorf("提交应关联到新建的申请：submission=%d application=%d", submission.ApplicationID, application.ID)
	}
}

// 申请已到终态（accepted / rejected）→ 拒绝补交，且零副作用。
func TestSubmit_RejectsFinalApplication(t *testing.T) {
	for _, status := range []domain.ApplicationStatus{domain.ApplicationAccepted, domain.ApplicationRejected} {
		t.Run(string(status), func(t *testing.T) {
			uc, repo, creator := formFixture()
			form, ids := openForm(t, uc)

			creator.seedApplication(1, form.RoundID, form.DirectionCode, status)
			before := len(creator.apps)

			_, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
				ids["姓名"]:   "张三",
				ids["联系邮箱"]: "zhangsan@example.com",
				ids["投入时长"]: "每周 5 小时",
			})
			if !errors.Is(err, ErrApplicationFinal) {
				t.Fatalf("终态申请补交应返回 ErrApplicationFinal，实际: %v", err)
			}
			if len(creator.apps) != before {
				t.Errorf("零副作用：不应新增申请，之前 %d 条现在 %d 条", before, len(creator.apps))
			}
			if len(repo.submissions) != 0 {
				t.Errorf("零副作用：不应留下提交记录，实际 %d 条", len(repo.submissions))
			}
		})
	}
}

/* ------------------------------------------------------------------ *
 * 方向 → 面试表单的组装（#135 变更一）
 * ------------------------------------------------------------------ */

// OpenFormsByRound 只收 is_open 的表，且同方向取"最新一张"。
func TestOpenFormsByRound_PicksNewestPerDirection(t *testing.T) {
	uc, repo, _ := formFixture()

	// 同方向两张开放表：updated_at 较新的必须胜出（id 更大的更新）
	repo.forms[1] = &domain.Form{
		ID: 1, Title: "旧表", RoundID: 1, DirectionCode: "front-end",
		ShareToken: "old-token", IsOpen: true,
	}
	repo.forms[2] = &domain.Form{
		ID: 2, Title: "新表", RoundID: 1, DirectionCode: "front-end",
		ShareToken: "new-token", IsOpen: true,
	}
	// 关闭的表不参与
	repo.forms[3] = &domain.Form{
		ID: 3, Title: "关掉的表", RoundID: 1, DirectionCode: "back-end",
		ShareToken: "closed-token", IsOpen: false,
	}
	// 别的轮次不参与
	repo.forms[4] = &domain.Form{
		ID: 4, Title: "下一轮的表", RoundID: 2, DirectionCode: "front-end",
		ShareToken: "next-round-token", IsOpen: true,
	}

	forms, err := uc.OpenFormsByRound(context.Background(), 1)
	if err != nil {
		t.Fatalf("组装应当成功: %v", err)
	}

	if len(forms) != 1 {
		t.Fatalf("本轮应只有 front-end 一张开放表，实际 %d 个方向", len(forms))
	}
	got, ok := forms["front-end"]
	if !ok {
		t.Fatal("应包含 front-end")
	}
	if got.ShareToken != "new-token" {
		t.Errorf("同方向多张开放表应取最新一张（new-token），实际 %q", got.ShareToken)
	}
	if _, ok := forms["back-end"]; ok {
		t.Error("关闭的表不应出现在结果里")
	}
}

// 没有开放轮次（roundID = 0）→ 空结果且不报错，方向列表照常返回。
func TestOpenFormsByRound_NoActiveRound(t *testing.T) {
	uc, repo, _ := formFixture()
	repo.forms[1] = &domain.Form{
		ID: 1, RoundID: 1, DirectionCode: "front-end", ShareToken: "t", IsOpen: true,
	}

	forms, err := uc.OpenFormsByRound(context.Background(), 0)
	if err != nil {
		t.Fatalf("无开放轮次不应报错，实际: %v", err)
	}
	if len(forms) != 0 {
		t.Errorf("无开放轮次应返回空结果，实际 %d 个方向", len(forms))
	}
}

// 该轮没有任何开放表单 → 空结果且不报错（handler 据此把 form 置 null）。
func TestOpenFormsByRound_NoOpenForm(t *testing.T) {
	uc, repo, _ := formFixture()
	repo.forms[1] = &domain.Form{
		ID: 1, RoundID: 1, DirectionCode: "front-end", ShareToken: "t", IsOpen: false,
	}

	forms, err := uc.OpenFormsByRound(context.Background(), 1)
	if err != nil {
		t.Fatalf("无开放表单不应报错，实际: %v", err)
	}
	if len(forms) != 0 {
		t.Errorf("应返回空结果，实际 %d 个方向", len(forms))
	}
}

/* ------------------------------------------------------------------ *
 * 验收：答案不改账号邮箱
 * ------------------------------------------------------------------ */

func TestSubmit_EmailAnswerDoesNotTouchAccountEmail(t *testing.T) {
	uc, _, creator := formFixture()
	const accountEmail = "account@example.com"
	creator.profile.Detail.Email = accountEmail

	form, ids := openForm(t, uc)

	submission, application, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "answer@example.com",
		ids["投入时长"]: "每周 5 小时",
	})
	if err != nil {
		t.Fatalf("提交应当成功: %v", err)
	}

	if creator.profile.Detail.Email != accountEmail {
		t.Errorf("账号绑定邮箱被改写为 %q", creator.profile.Detail.Email)
	}
	if application.Email != accountEmail {
		t.Errorf("申请快照应来自用户资料（%s），实际 %q", accountEmail, application.Email)
	}
	if got := submission.Answers[ids["联系邮箱"]]; got != "answer@example.com" {
		t.Errorf("表单里的邮箱答案应原样存档，实际 %v", got)
	}
}

/* ------------------------------------------------------------------ *
 * 分享读取：已提交状态与关闭状态
 * ------------------------------------------------------------------ */

func TestReadShareForm_ReturnsMySubmission(t *testing.T) {
	uc, _, _ := formFixture()
	form, ids := openForm(t, uc)

	got, fields, mine, err := uc.ReadShareForm(context.Background(), form.ShareToken, 1)
	if err != nil {
		t.Fatalf("读取分享表单应当成功: %v", err)
	}
	if got.ID != form.ID || len(fields) != 5 {
		t.Errorf("应返回定义与 5 个字段，实际 id=%d fields=%d", got.ID, len(fields))
	}
	if mine != nil {
		t.Error("还没提交时不应返回已提交记录")
	}

	if _, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	}); err != nil {
		t.Fatalf("提交应当成功: %v", err)
	}

	_, _, mine, err = uc.ReadShareForm(context.Background(), form.ShareToken, 1)
	if err != nil {
		t.Fatalf("再次读取应当成功: %v", err)
	}
	if mine == nil {
		t.Fatal("已提交后应返回该用户的提交记录")
	}
	if mine.SubmitterID != 1 {
		t.Errorf("返回的提交应属于当前用户，实际 submitter=%d", mine.SubmitterID)
	}

	// 另一个用户看到的仍是"未提交"
	if _, _, other, err := uc.ReadShareForm(context.Background(), form.ShareToken, 2); err != nil || other != nil {
		t.Errorf("他人不应看到别人的提交：err=%v other=%v", err, other)
	}
}

// 关闭的表单照样能读到定义（填写页据此显示"已关闭"，而不是报链接失效）。
func TestReadShareForm_ClosedStillReadable(t *testing.T) {
	uc, _, _ := formFixture()
	form, _ := openForm(t, uc)

	if _, _, _, err := uc.UpdateForm(context.Background(), form.ID, UpdateFormInput{IsOpen: boolPtr(false)}); err != nil {
		t.Fatalf("关闭表单应当成功: %v", err)
	}

	got, _, _, err := uc.ReadShareForm(context.Background(), form.ShareToken, 1)
	if err != nil {
		t.Fatalf("关闭后读取定义仍应成功: %v", err)
	}
	if got.IsOpen {
		t.Error("关闭后 is_open 应为 false")
	}
}

/* ------------------------------------------------------------------ *
 * 管理端查询
 * ------------------------------------------------------------------ */

func TestListForms_WithSubmissionCounts(t *testing.T) {
	uc, _, _ := formFixture()
	form, ids := openForm(t, uc)

	if _, _, err := uc.SubmitByShareToken(context.Background(), form.ShareToken, 1, map[string]any{
		ids["姓名"]:   "张三",
		ids["联系邮箱"]: "zhangsan@example.com",
		ids["投入时长"]: "每周 5 小时",
	}); err != nil {
		t.Fatalf("提交应当成功: %v", err)
	}

	forms, counts, total, err := uc.ListForms(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("列表应当成功: %v", err)
	}
	if total != 1 || len(forms) != 1 {
		t.Fatalf("应有 1 张表单，实际 total=%d len=%d", total, len(forms))
	}
	if counts[form.ID] != 1 {
		t.Errorf("提交数应为 1，实际 %d", counts[form.ID])
	}
}

func TestListSubmissions_UnknownForm(t *testing.T) {
	uc, _, _ := formFixture()

	if _, _, err := uc.ListSubmissions(context.Background(), 999, 1, 20); !errors.Is(err, repository.ErrFormNotFound) {
		t.Errorf("表单不存在应返回 ErrFormNotFound，实际: %v", err)
	}
	if _, err := uc.GetSubmission(context.Background(), 999, 1); !errors.Is(err, repository.ErrFormNotFound) {
		t.Errorf("表单不存在应返回 ErrFormNotFound，实际: %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 工具
 * ------------------------------------------------------------------ */

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	var out int
	if _, err := fmt.Sscanf(s, "%d", &out); err != nil {
		t.Fatalf("解析字段 id 失败: %q", s)
	}
	return out
}
