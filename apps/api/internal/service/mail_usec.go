package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// MailSender 邮件服务对"怎么把信发出去"的全部依赖。
//
// 用接口而不是 *utils.MailManager：发信逻辑（选模板、渲染、落记录、逐条汇报）
// 不该被 SMTP 细节缠住，测试也就不必真的连 SMTP。
type MailSender interface {
	SendEmail(email utils.Email) error
}

// NewMailSender 把 *utils.MailManager 作为 MailSender 提供给装配层。
//
// 位置理由同 auth_usec.go 的 NewTokenIssuer：接口由消费者（本包）声明，
// utils 不能反向依赖 service，转换函数只能写在声明接口的这一侧。
func NewMailSender(manager *utils.MailManager) MailSender {
	return manager
}

// MaxBulkRecipients 单次群发的收件人上限。
//
// 目的是防手滑：一次点错就对着几百个真实邮箱连发，是收不回来的操作。
// 超过就报错让调用方分批，而不是悄悄截断。
const MaxBulkRecipients = 200

// ErrMailDelivery 邮件投递失败（SMTP/网络问题），而不是调用方输入有误。
//
// 单独区分出来是为了让接口层给出正确状态码：模板名重复、缺变量属于 400，
// 而"SMTP 连不上"是服务端依赖故障，返回 400 会让管理端以为是自己的参数写错了。
var ErrMailDelivery = errors.New("邮件投递失败")

// MailUseCase 邮件模板管理与发信。
type MailUseCase struct {
	templates repository.MailModelRepo
	mails     repository.MailRepo
	users     repository.UserRepo
	sender    MailSender
}

func NewMailUseCase(
	templates repository.MailModelRepo,
	mails repository.MailRepo,
	users repository.UserRepo,
	sender MailSender,
) *MailUseCase {
	return &MailUseCase{
		templates: templates,
		mails:     mails,
		users:     users,
		sender:    sender,
	}
}

/* ------------------------------------------------------------------ *
 * 模板管理
 * ------------------------------------------------------------------ */

func (a *MailUseCase) ListTemplates(ctx context.Context) (*response.MailModelListResp, error) {
	models, err := a.templates.List(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]response.MailModelResp, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		items = append(items, toMailModelResp(model))
	}

	return &response.MailModelListResp{Items: items, Total: len(items)}, nil
}

func (a *MailUseCase) CreateTemplate(ctx context.Context, req *request.MailTemplateReq) (*response.MailModelResp, error) {
	if err := validateTemplateReq(req); err != nil {
		return nil, err
	}

	model := &domain.MailModel{
		Name:        req.Name,
		Type:        req.Type,
		Title:       req.Title,
		MailExample: req.MailExample,
		MailModel:   req.MailModel,
	}
	if err := a.templates.Create(ctx, model); err != nil {
		return nil, err
	}

	result := toMailModelResp(model)
	return &result, nil
}

func (a *MailUseCase) UpdateTemplate(ctx context.Context, id int, req *request.MailTemplateReq) (*response.MailModelResp, error) {
	if err := validateTemplateReq(req); err != nil {
		return nil, err
	}

	// 先取一次，既确认模板存在，也避免 Save 一个 ID 不存在的空壳（Save 会变成 INSERT）
	model, err := a.templates.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	model.Name = req.Name
	model.Type = req.Type
	model.Title = req.Title
	model.MailExample = req.MailExample
	model.MailModel = req.MailModel

	if err := a.templates.Update(ctx, model); err != nil {
		return nil, err
	}

	result := toMailModelResp(model)
	return &result, nil
}

func (a *MailUseCase) DeleteTemplate(ctx context.Context, id int) error {
	return a.templates.Delete(ctx, id)
}

/* ------------------------------------------------------------------ *
 * 发信
 * ------------------------------------------------------------------ */

// SendToUser 按 userID 单发。
func (a *MailUseCase) SendToUser(ctx context.Context, operatorID int, req *request.SendMailToUserReq) (*response.MailSendResp, error) {
	user, err := a.users.GetByID(ctx, req.ToUserID)
	if err != nil || user == nil {
		return nil, errors.New("收件人不存在")
	}

	result, err := a.sendToUser(ctx, operatorID, user, req.TemplateID, req.Vars)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SendToEmail 按邮箱单发。
//
// 邮箱能对上已注册用户时按用户记录（这样记录里带 userID）；对不上就只记邮箱，
// 那是"发给还没注册的报名者"的正常情况，不是错误。
func (a *MailUseCase) SendToEmail(ctx context.Context, operatorID int, req *request.SendMailToEmailReq) (*response.MailSendResp, error) {
	model, err := a.templates.GetByID(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}

	// 收件人可能是注册用户，也可能不是：取到就预填资料，取不到就当匿名收件人
	var user *domain.User
	if found, err := a.users.GetByEmail(ctx, req.Email); err == nil {
		user = found
	}

	title, content, err := a.render(model, user, req.Vars)
	if err != nil {
		return nil, err
	}

	if err := a.sender.SendEmail(utils.Email{Receiver: req.Email, Subject: title, Body: content}); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMailDelivery, err)
	}

	a.record(ctx, operatorID, userIDOf(user), req.Email, title, content)

	return &response.MailSendResp{
		ToUserID: userIDOf(user),
		ToEmail:  req.Email,
		Title:    title,
	}, nil
}

// SendBulk 群发：同一模板，逐个收件人各自的替换内容。
//
// 逐条独立处理，单个失败不影响其他收件人，最终把成败一起返回。
// 若在第一个失败处整体返回，前面已经投递出去的邮件就会变成"没发过"，
// 管理端既看不到记录、也可能会重发一遍。
func (a *MailUseCase) SendBulk(ctx context.Context, operatorID int, req *request.SendMailBulkReq) (*response.MailBulkSendResp, error) {
	if len(req.Recipients) > MaxBulkRecipients {
		return nil, fmt.Errorf("单次群发最多 %d 个收件人，本次 %d 个，请分批发送",
			MaxBulkRecipients, len(req.Recipients))
	}

	model, err := a.templates.GetByID(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}

	result := &response.MailBulkSendResp{
		Sent:   make([]response.MailSendResp, 0, len(req.Recipients)),
		Failed: make([]response.MailSendFailure, 0),
		Total:  len(req.Recipients),
	}

	for _, recipient := range req.Recipients {
		user, err := a.users.GetByID(ctx, recipient.ToUserID)
		if err != nil || user == nil {
			result.Failed = append(result.Failed, response.MailSendFailure{
				ToUserID: recipient.ToUserID,
				Reason:   "收件人不存在",
			})
			continue
		}

		// 复用循环外取到的模板，避免每个收件人再查一次库
		sent, err := a.sendWithTemplate(ctx, operatorID, model, user, recipient.Vars)
		if err != nil {
			result.Failed = append(result.Failed, response.MailSendFailure{
				ToUserID: recipient.ToUserID,
				ToEmail:  user.Detail.Email,
				Reason:   err.Error(),
			})
			continue
		}
		result.Sent = append(result.Sent, sent)
	}

	return result, nil
}

/* ------------------------------------------------------------------ *
 * 发信记录
 * ------------------------------------------------------------------ */

func (a *MailUseCase) ListMails(ctx context.Context, page int, pageSize int) (*response.MailRecordListResp, error) {
	mails, total, err := a.mails.List(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]response.MailRecordResp, 0, len(mails))
	for _, mail := range mails {
		if mail == nil {
			continue
		}
		items = append(items, response.MailRecordResp{
			ID:         mail.ID,
			Title:      mail.Title,
			FromUserID: mail.FromUserID,
			ToUserID:   mail.ToUserID,
			ToEmail:    mail.ToEmail,
			CreatedAt:  mail.CreatedAt.Format(time.RFC3339),
		})
	}

	return &response.MailRecordListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

/* ------------------------------------------------------------------ *
 * 内部
 * ------------------------------------------------------------------ */

// sendToUser 按 userID 单发：取模板后交给 sendWithTemplate。
func (a *MailUseCase) sendToUser(
	ctx context.Context,
	operatorID int,
	user *domain.User,
	templateID int,
	vars request.MailVars,
) (response.MailSendResp, error) {
	model, err := a.templates.GetByID(ctx, templateID)
	if err != nil {
		return response.MailSendResp{}, err
	}
	return a.sendWithTemplate(ctx, operatorID, model, user, vars)
}

// sendWithTemplate 渲染 + 发送 + 落记录。单发与群发共用同一条路径，
// 避免两条路径的校验与记录行为出现分歧；模板由调用方传入，群发时只查一次。
func (a *MailUseCase) sendWithTemplate(
	ctx context.Context,
	operatorID int,
	model *domain.MailModel,
	user *domain.User,
	vars request.MailVars,
) (response.MailSendResp, error) {
	if user.Detail.Email == "" {
		return response.MailSendResp{}, errors.New("该用户没有邮箱，无法发送")
	}

	title, content, err := a.render(model, user, vars)
	if err != nil {
		return response.MailSendResp{}, err
	}

	if err := a.sender.SendEmail(utils.Email{Receiver: user.Detail.Email, Subject: title, Body: content}); err != nil {
		return response.MailSendResp{}, fmt.Errorf("%w: %v", ErrMailDelivery, err)
	}

	a.record(ctx, operatorID, user.ID, user.Detail.Email, title, content)

	return response.MailSendResp{
		ToUserID: user.ID,
		ToEmail:  user.Detail.Email,
		Title:    title,
	}, nil
}

// render 渲染主题与正文。
//
// 变量表先用收件人资料预填，再让调用方传进来的同键覆盖。
// 这样"群发同一个模板、内容逐个不同"在多数场景下不需要调用方传任何变量，
// 而需要覆盖时（例如临时改了面试时间）又能显式指定。
func (a *MailUseCase) render(
	model *domain.MailModel,
	user *domain.User,
	overrides request.MailVars,
) (title string, content string, err error) {
	vars := utils.TemplateVars{}
	if user != nil {
		// 只放非空值：留空会让模板里的 {{class}} 渲染成空字符串，
		// 收件人看到"班级："后面什么都没有。缺值就让它按缺变量报错，暴露数据不全。
		putIfNotEmpty(vars, "name", user.Name)
		putIfNotEmpty(vars, "email", user.Detail.Email)
		putIfNotEmpty(vars, "class", user.Detail.Class)
		putIfNotEmpty(vars, "direction", string(user.Detail.Direction))
		if user.Detail.StudentID != 0 {
			vars["student_id"] = strconv.Itoa(user.Detail.StudentID)
		}
		vars["parse"] = strconv.Itoa(user.Parse)
	}
	for key, value := range overrides {
		vars[key] = value
	}

	title, err = utils.RenderTemplate(model.Title, vars)
	if err != nil {
		return "", "", fmt.Errorf("渲染主题失败：%w", err)
	}

	content, err = utils.RenderTemplate(model.MailModel, vars)
	if err != nil {
		return "", "", fmt.Errorf("渲染正文失败：%w", err)
	}

	return title, content, nil
}

// record 写发信记录。
//
// 落记录失败**不影响发送结果**：邮件已经投出去了，这时把错误抛给调用方
// 只会让人以为没发成功而重发。所以只记日志，由日志暴露存储问题。
func (a *MailUseCase) record(ctx context.Context, operatorID int, toUserID int, toEmail string, title string, content string) {
	mail := &domain.Mail{
		Title:      title,
		Content:    content,
		FromUserID: operatorID,
		ToUserID:   toUserID,
		ToEmail:    toEmail,
	}

	if err := a.mails.Create(ctx, mail); err != nil {
		log.Printf("[mail] 邮件已发出但写入发信记录失败（收件人 %s）: %v", toEmail, err)
	}
}

func validateTemplateReq(req *request.MailTemplateReq) error {
	if err := utils.ValidateTemplateSyntax(req.Title); err != nil {
		return fmt.Errorf("主题：%w", err)
	}
	if err := utils.ValidateTemplateSyntax(req.MailModel); err != nil {
		return fmt.Errorf("正文：%w", err)
	}
	return nil
}

func toMailModelResp(model *domain.MailModel) response.MailModelResp {
	return response.MailModelResp{
		ID:          model.ID,
		Name:        model.Name,
		Type:        model.Type,
		Title:       model.Title,
		MailExample: model.MailExample,
		MailModel:   model.MailModel,
		// 主题与正文的变量合并去重，供管理端提示可用变量
		Variables: mergeVariables(
			utils.ListTemplateVariables(model.Title),
			utils.ListTemplateVariables(model.MailModel),
		),
	}
}

func mergeVariables(lists ...[]string) []string {
	seen := map[string]struct{}{}
	merged := make([]string, 0)

	for _, list := range lists {
		for _, name := range list {
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			merged = append(merged, name)
		}
	}

	return merged
}

func putIfNotEmpty(vars utils.TemplateVars, key string, value string) {
	if value != "" {
		vars[key] = value
	}
}

func userIDOf(user *domain.User) int {
	if user == nil {
		return 0
	}
	return user.ID
}
