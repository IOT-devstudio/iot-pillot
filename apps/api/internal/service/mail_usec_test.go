package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * 桩实现：邮件服务只依赖接口，因此不需要 DB、Redis 与 SMTP
 * ------------------------------------------------------------------ */

type stubMailModelRepo struct {
	models    map[int]*domain.MailModel
	createErr error
	updateErr error
	deleteErr error
	getCalls  int
}

func newStubMailModelRepo(models ...*domain.MailModel) *stubMailModelRepo {
	repo := &stubMailModelRepo{models: map[int]*domain.MailModel{}}
	for _, model := range models {
		repo.models[model.ID] = model
	}
	return repo
}

func (s *stubMailModelRepo) List(_ context.Context) ([]*domain.MailModel, error) {
	out := make([]*domain.MailModel, 0, len(s.models))
	for _, model := range s.models {
		out = append(out, model)
	}
	return out, nil
}

func (s *stubMailModelRepo) GetByID(_ context.Context, id int) (*domain.MailModel, error) {
	s.getCalls++
	model, ok := s.models[id]
	if !ok {
		return nil, errors.New("模板不存在")
	}
	return model, nil
}

func (s *stubMailModelRepo) Create(_ context.Context, model *domain.MailModel) error {
	if s.createErr != nil {
		return s.createErr
	}
	model.ID = len(s.models) + 100
	s.models[model.ID] = model
	return nil
}

func (s *stubMailModelRepo) Update(_ context.Context, model *domain.MailModel) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.models[model.ID] = model
	return nil
}

func (s *stubMailModelRepo) Delete(_ context.Context, id int) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.models[id]; !ok {
		return errors.New("模板不存在")
	}
	delete(s.models, id)
	return nil
}

type stubMailRepo struct {
	created []*domain.Mail
	err     error
}

func (s *stubMailRepo) Create(_ context.Context, mail *domain.Mail) error {
	if s.err != nil {
		return s.err
	}
	s.created = append(s.created, mail)
	return nil
}

func (s *stubMailRepo) List(_ context.Context, _ int, _ int) ([]*domain.Mail, int64, error) {
	return s.created, int64(len(s.created)), nil
}

type sentEmail struct {
	to      string
	subject string
	body    string
}

type stubMailSender struct {
	sent []sentEmail
	err  error
}

func (s *stubMailSender) SendEmail(email utils.Email) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, sentEmail{to: email.Receiver, subject: email.Subject, body: email.Body})
	return nil
}

/* ------------------------------------------------------------------ *
 * 夹具
 * ------------------------------------------------------------------ */

func mailTestUser(id int, name string, email string) *domain.User {
	return &domain.User{
		ID:     id,
		Name:   name,
		Detail: domain.Detail{Email: email, Class: "软工 2 班", Direction: domain.BackEnd, StudentID: 20260001},
		Parse:  1,
	}
}

func mailFixture() (*MailUseCase, *stubMailModelRepo, *stubMailRepo, *stubMailSender) {
	models := newStubMailModelRepo(&domain.MailModel{
		ID:        1,
		Name:      "invitation",
		Type:      "invitation",
		Title:     "{{name}}，邀请你加入 iot 全栈工作室",
		MailModel: "<p>{{class}} 的 {{name}} 同学你好，第 {{parse}} 轮招新面试定在 {{time}}。</p>",
	})
	mails := &stubMailRepo{}
	sender := &stubMailSender{}

	users := &stubUserRepo{
		byID: map[int]*domain.User{
			1: mailTestUser(1, "张三", "zhangsan@example.com"),
			2: mailTestUser(2, "李四", "lisi@example.com"),
			3: {ID: 3, Name: "无邮箱同学"},
		},
		byName: map[string]*domain.User{},
		all: []*domain.User{
			mailTestUser(1, "张三", "zhangsan@example.com"),
			mailTestUser(2, "李四", "lisi@example.com"),
		},
	}
	users.byName["张三"] = users.byID[1]

	return NewMailUseCase(models, mails, users, sender), models, mails, sender
}

/* ------------------------------------------------------------------ *
 * 单发
 * ------------------------------------------------------------------ */

// 核心用例：只要模板用的变量能从收件人资料推导出来，调用方就不用传。
// 这正是「群发同一个模板、逐个替换内容」最常见的用法。
//
// 注意 {{time}} 这类**不属于用户资料**的变量仍必须显式传入：服务端无从推断面试时间，
// 缺了它按 RenderTemplate 的约定直接拒绝发送，而不是渲染成空字符串。
func TestSendToUser_PrefillsVariablesFromRecipient(t *testing.T) {
	useCase, _, mails, sender := mailFixture()

	result, err := useCase.SendToUser(context.Background(), 9, &request.SendMailToUserReq{
		TemplateID: 1,
		ToUserID:   1,
		Vars:       request.MailVars{"time": "周三 14:00"},
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("应当发出 1 封，实际 %d 封", len(sender.sent))
	}
	if sender.sent[0].to != "zhangsan@example.com" {
		t.Errorf("收件人 = %q", sender.sent[0].to)
	}
	// {{name}} 没显式传，应当由收件人资料预填
	if result.Title != "张三，邀请你加入 iot 全栈工作室" {
		t.Errorf("渲染后的主题 = %q", result.Title)
	}
	if !strings.Contains(sender.sent[0].body, "软工 2 班") {
		t.Errorf("正文里应当自动替换了 class，实际: %q", sender.sent[0].body)
	}
	if !strings.Contains(sender.sent[0].body, "第 1 轮") {
		t.Errorf("正文里应当自动替换了 parse，实际: %q", sender.sent[0].body)
	}
	if !strings.Contains(sender.sent[0].body, "周三 14:00") {
		t.Errorf("显式传入的 time 应当被替换，实际: %q", sender.sent[0].body)
	}

	// 记录：操作者是 9，收件人是 1
	if len(mails.created) != 1 {
		t.Fatalf("应当写入 1 条发信记录，实际 %d 条", len(mails.created))
	}
	if mails.created[0].FromUserID != 9 || mails.created[0].ToUserID != 1 {
		t.Errorf("记录的发件/收件人不对: %+v", mails.created[0])
	}
	if mails.created[0].Title != result.Title {
		t.Error("记录里的主题应当是渲染后的主题")
	}
}

// 调用方传的变量覆盖预填值。
func TestSendToUser_OverridesPrefilledVariables(t *testing.T) {
	useCase, _, _, sender := mailFixture()

	_, err := useCase.SendToUser(context.Background(), 9, &request.SendMailToUserReq{
		TemplateID: 1,
		ToUserID:   1,
		Vars:       request.MailVars{"time": "下周一 10:00"},
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	if !strings.Contains(sender.sent[0].body, "下周一 10:00") {
		t.Errorf("覆盖变量没有生效: %q", sender.sent[0].body)
	}
}

// 缺变量必须发送失败：宁可发不出去，也不要发出「你好，{{name}}」这种半成品。
func TestSendToUser_FailsWhenVariableMissing(t *testing.T) {
	useCase, _, mails, sender := mailFixture()

	// 模板用到 {{time}}，而收件人资料里没有这个变量
	_, err := useCase.SendToUser(context.Background(), 9, &request.SendMailToUserReq{
		TemplateID: 1,
		ToUserID:   1,
	})
	if err == nil {
		t.Fatal("缺 time 变量时应当报错")
	}
	if !strings.Contains(err.Error(), "time") {
		t.Errorf("错误信息应当指明缺了哪个变量，实际: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Error("渲染失败时不该发出任何邮件")
	}
	if len(mails.created) != 0 {
		t.Error("渲染失败时不该写发信记录")
	}
}

func TestSendToUser_RejectsUserWithoutEmail(t *testing.T) {
	useCase, _, _, sender := mailFixture()

	_, err := useCase.SendToUser(context.Background(), 9, &request.SendMailToUserReq{
		TemplateID: 1,
		ToUserID:   3,
	})
	if err == nil {
		t.Fatal("收件人没有邮箱时应当报错")
	}
	if len(sender.sent) != 0 {
		t.Error("没有邮箱时不该调用发送")
	}
}

/* ------------------------------------------------------------------ *
 * 按邮箱单发
 * ------------------------------------------------------------------ */

func TestSendToEmail_RecordsUserIDWhenRegistered(t *testing.T) {
	useCase, _, mails, sender := mailFixture()

	_, err := useCase.SendToEmail(context.Background(), 9, &request.SendMailToEmailReq{
		TemplateID: 1,
		Email:      "lisi@example.com",
		Vars:       request.MailVars{"time": "周三"},
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	if sender.sent[0].to != "lisi@example.com" {
		t.Errorf("收件人 = %q", sender.sent[0].to)
	}
	// 邮箱能对上已注册用户，记录里应当带上 userID
	if mails.created[0].ToUserID != 2 {
		t.Errorf("记录里的 ToUserID = %d，期望 2", mails.created[0].ToUserID)
	}
}

func TestSendToEmail_RecordsEmailWhenNotRegistered(t *testing.T) {
	useCase, _, mails, _ := mailFixture()

	_, err := useCase.SendToEmail(context.Background(), 9, &request.SendMailToEmailReq{
		TemplateID: 1,
		Email:      "newcomer@example.com",
		// 对方未注册，没有用户资料可预填，模板里的变量必须全部显式给出
		Vars: request.MailVars{"name": "新同学", "class": "软工 3 班", "parse": "1", "time": "周五"},
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}

	record := mails.created[0]
	// 未注册的收件人没有 userID，必须把邮箱记下来，否则这条记录无法追溯
	if record.ToUserID != 0 || record.ToEmail != "newcomer@example.com" {
		t.Errorf("记录应当保留邮箱且 ToUserID=0，实际: %+v", record)
	}
}

/* ------------------------------------------------------------------ *
 * 群发
 * ------------------------------------------------------------------ */

// 群发的关键性质：单个失败不影响其他收件人。
func TestSendBulk_ContinuesAfterOneFailure(t *testing.T) {
	useCase, models, mails, sender := mailFixture()

	result, err := useCase.SendBulk(context.Background(), 9, &request.SendMailBulkReq{
		TemplateID: 1,
		Recipients: []request.MailRecipientReq{
			{ToUserID: 1, Vars: request.MailVars{"time": "周三"}},
			{ToUserID: 999}, // 不存在
			{ToUserID: 3, Vars: request.MailVars{"time": "周三"}}, // 没有邮箱
			{ToUserID: 2, Vars: request.MailVars{"time": "周三"}},
		},
	})
	if err != nil {
		t.Fatalf("群发不该整体失败: %v", err)
	}

	if len(result.Sent) != 2 {
		t.Errorf("应当成功 2 封，实际 %d 封", len(result.Sent))
	}
	if len(result.Failed) != 2 {
		t.Errorf("应当失败 2 条，实际 %d 条", len(result.Failed))
	}
	if result.Total != 4 {
		t.Errorf("Total = %d，期望 4", result.Total)
	}
	if len(sender.sent) != 2 {
		t.Errorf("实际投递 %d 封，期望 2 封", len(sender.sent))
	}
	if len(mails.created) != 2 {
		t.Errorf("应当只给成功的 2 封写记录，实际 %d 条", len(mails.created))
	}

	// 失败原因要能看懂
	for _, failure := range result.Failed {
		if failure.Reason == "" {
			t.Error("失败项必须带原因")
		}
	}

	// 模板只该查一次：每个收件人再查一遍是典型的 N+1
	if models.getCalls != 1 {
		t.Errorf("模板查询次数 = %d，期望 1（群发复用同一个模板）", models.getCalls)
	}
}

func TestSendBulk_RejectsTooManyRecipients(t *testing.T) {
	useCase, _, _, sender := mailFixture()

	recipients := make([]request.MailRecipientReq, MaxBulkRecipients+1)
	for i := range recipients {
		recipients[i] = request.MailRecipientReq{ToUserID: i + 1}
	}

	_, err := useCase.SendBulk(context.Background(), 9, &request.SendMailBulkReq{
		TemplateID: 1,
		Recipients: recipients,
	})
	if err == nil {
		t.Fatal("超过上限时应当报错，而不是悄悄截断")
	}
	if len(sender.sent) != 0 {
		t.Error("参数被拒绝时不该发出任何邮件")
	}
}

/* ------------------------------------------------------------------ *
 * 模板管理
 * ------------------------------------------------------------------ */

func TestCreateTemplate_RejectsUnbalancedPlaceholder(t *testing.T) {
	useCase, _, _, _ := mailFixture()

	_, err := useCase.CreateTemplate(context.Background(), &request.MailTemplateReq{
		Name:      "bad",
		Type:      "bad",
		Title:     "你好 {{name}",
		MailModel: "正文",
	})
	if err == nil {
		t.Fatal("占位符括号不成对时应当拒绝——这种模板会静默发出写错的原样文本")
	}
}

func TestCreateTemplate_PropagatesDuplicateError(t *testing.T) {
	useCase, models, _, _ := mailFixture()
	models.createErr = repository.ErrDuplicateMailModel

	_, err := useCase.CreateTemplate(context.Background(), &request.MailTemplateReq{
		Name:      "invitation",
		Type:      "invitation",
		Title:     "主题",
		MailModel: "正文",
	})
	if !errors.Is(err, repository.ErrDuplicateMailModel) {
		t.Errorf("应当透出重复错误以便接口层返回 400，实际: %v", err)
	}
}

func TestListTemplates_MergesVariablesFromTitleAndBody(t *testing.T) {
	useCase, _, _, _ := mailFixture()

	result, err := useCase.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("列举失败: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("模板数 = %d，期望 1", len(result.Items))
	}

	variables := result.Items[0].Variables
	// 主题里有 name，正文里有 class / name / parse / time
	want := map[string]bool{"name": true, "class": true, "parse": true, "time": true}
	if len(variables) != len(want) {
		t.Fatalf("变量列表 = %v，期望 %d 个（name 去重）", variables, len(want))
	}
	for _, name := range variables {
		if !want[name] {
			t.Errorf("出现了预期外的变量 %q（完整列表 %v）", name, variables)
		}
	}
}

func TestDeleteTemplate_PropagatesNotFound(t *testing.T) {
	useCase, _, _, _ := mailFixture()

	if err := useCase.DeleteTemplate(context.Background(), 404); err == nil {
		t.Fatal("删除不存在的模板应当报错")
	}
}

/* ------------------------------------------------------------------ *
 * 记录写入失败
 * ------------------------------------------------------------------ */

// 邮件已经投递出去之后，写记录失败不能反过来让发送算失败：
// 调用方会以为没发成功而重发，收件人就收到两封。
func TestSendToUser_SucceedsEvenIfRecordingFails(t *testing.T) {
	useCase, _, mails, sender := mailFixture()
	mails.err = errors.New("db down")

	_, err := useCase.SendToUser(context.Background(), 9, &request.SendMailToUserReq{
		TemplateID: 1,
		ToUserID:   1,
		Vars:       request.MailVars{"time": "周三"},
	})
	if err != nil {
		t.Fatalf("写记录失败不该让发送失败: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Error("邮件应当已经发出")
	}
}

// SMTP 失败要能被上层识别成「投递失败」，以便返回 500 而不是 400。
func TestSendToUser_WrapsDeliveryFailure(t *testing.T) {
	useCase, _, _, sender := mailFixture()
	sender.err = errors.New("dial tcp: connection refused")

	_, err := useCase.SendToUser(context.Background(), 9, &request.SendMailToUserReq{
		TemplateID: 1,
		ToUserID:   1,
		Vars:       request.MailVars{"time": "周三"},
	})
	if !errors.Is(err, ErrMailDelivery) {
		t.Errorf("SMTP 失败应当包装成 ErrMailDelivery，实际: %v", err)
	}
}
