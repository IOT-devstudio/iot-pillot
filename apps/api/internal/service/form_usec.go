package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 对外错误
 * ------------------------------------------------------------------ */

// ErrFormClosed 表单已关闭，不接受提交（管理端「关闭」的语义）。
var ErrFormClosed = errors.New("该表单已关闭，无法提交")

// ErrFormConfig 表单或字段配置不合法（消息里带具体原因，直接给管理端看）。
var ErrFormConfig = errors.New("表单配置不合法")

// ErrAnswerInvalid 提交内容不合法（消息里带具体原因，指出是哪个字段）。
var ErrAnswerInvalid = errors.New("提交内容不合法")

/* ------------------------------------------------------------------ *
 * 上限（防滥用，不是产品规则）
 * ------------------------------------------------------------------ */

const (
	maxFormTitleLen        = 200
	maxFormDescriptionLen  = 2000
	maxFieldLabelLen       = 200
	maxFieldPlaceholderLen = 200
	maxFormFields          = 50
	maxSelectOptions       = 100
	maxAnswerRunes         = 2000
	maxCheckboxAnswerItems = 50
	shareTokenBytes        = 16 // 128bit 随机，hex 后 32 字符
	shareTokenMaxAttempts  = 3
)

// emailAnswerRe 邮箱字段的答案校验。
//
// 只要求"形状像邮箱"：这是收集报名信息的表单答案，不是账号凭据 ——
// 答案永不会写回 users.detail_email（issue #92 验收：答案不改账号邮箱）。
var emailAnswerRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

/* ------------------------------------------------------------------ *
 * 消费者声明的窄接口
 * ------------------------------------------------------------------ */

// ApplicationCreator 表单提交对统一申请模型（#88）的全部依赖：按**指定轮次**建申请。
//
// 实现方是 *RecruitmentUseCase。接口由消费者声明、装配层按"返回值就是接口"
// 连线 —— 与 ApplicationStore / BusinessMailer 同一模式：
// 表单模块不重复实现报名规则（方向白名单、方向开关、轮次开放、资料快照、初始时间线）。
type ApplicationCreator interface {
	SubmitToRound(ctx context.Context, userID int, roundID int, directionCode string) (*domain.Application, error)
}

// NewApplicationCreator 把招新用例作为 ApplicationCreator 提供给装配层。
func NewApplicationCreator(uc *RecruitmentUseCase) ApplicationCreator {
	return uc
}

// RecruitmentDirectory 表单建表时需要核对的招新目录：轮次存在、方向存在。
//
// 只读两个方法：建表要绑定 (round_id, direction_code)，绑定错的两项
// 会让整张表永远提交不进申请，必须在建表时就挡住。
type RecruitmentDirectory interface {
	GetRoundByID(ctx context.Context, id int) (*domain.RecruitmentRound, error)
	GetDirectionByCode(ctx context.Context, code string) (*domain.RecruitmentDirection, error)
}

// NewRecruitmentDirectory 把 #88 招新仓储作为目录提供给装配层。
func NewRecruitmentDirectory(rec repository.RecruitmentRepo) RecruitmentDirectory {
	return rec
}

/* ------------------------------------------------------------------ *
 * 入参
 * ------------------------------------------------------------------ */

// FormFieldInput 建表 / 改表时的字段入参。
//
// ID > 0 表示编辑已有字段（原地更新，历史答案仍对得上）；ID == 0 表示新增。
type FormFieldInput struct {
	ID          int
	Type        string
	Label       string
	Required    bool
	Placeholder string
	Options     []string
}

// CreateFormInput 建表入参。
//
// 轮次与方向只在建表时确定：表单一旦建立，绑定就不再可变
// （要换轮次/方向请新建表单），否则历史提交里的申请归属会与表单定义矛盾。
type CreateFormInput struct {
	Title         string
	Description   string
	RoundID       int
	DirectionCode string
	Fields        []FormFieldInput
}

// UpdateFormInput 改表入参：nil 表示"不改这一项"。
type UpdateFormInput struct {
	Title       *string
	Description *string
	IsOpen      *bool
	// Fields 非 nil 时整份替换字段列表（带 id 的原地更新、无 id 的新增、缺失的删除）。
	Fields *[]FormFieldInput
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// FormUseCase 表单模块用例：管理端建/改/开关/查提交，用户端按分享令牌读取并提交。
//
// 提交路径刻意不自己建申请：轮次与方向绑定在表单上，申请一律走
// ApplicationCreator（#88 的规则与唯一约束），因此"报名"只有一个所有者。
type FormUseCase struct {
	forms        repository.FormRepo
	recruitment  RecruitmentDirectory
	applications ApplicationCreator
}

// NewFormUseCase 构造函数。
func NewFormUseCase(
	forms repository.FormRepo,
	recruitment RecruitmentDirectory,
	applications ApplicationCreator,
) *FormUseCase {
	return &FormUseCase{forms: forms, recruitment: recruitment, applications: applications}
}

/* ------------------------------------------------------------------ *
 * 管理端：表单定义
 * ------------------------------------------------------------------ */

// CreateForm 建表：校验标题与字段配置、核对绑定轮次/方向、生成分享令牌。
//
// 幂等冲突只可能来自 share_token（forms 上唯一的唯一约束），
// 撞上就换一个令牌重试 —— 128bit 随机串连续三次冲突已经是天方夜谭。
func (u *FormUseCase) CreateForm(ctx context.Context, in CreateFormInput) (*domain.Form, []*domain.FormField, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, nil, fmt.Errorf("%w: 表单标题不能为空", ErrFormConfig)
	}
	if len([]rune(title)) > maxFormTitleLen {
		return nil, nil, fmt.Errorf("%w: 表单标题不能超过 %d 字", ErrFormConfig, maxFormTitleLen)
	}

	description := strings.TrimSpace(in.Description)
	if len([]rune(description)) > maxFormDescriptionLen {
		return nil, nil, fmt.Errorf("%w: 表单说明不能超过 %d 字", ErrFormConfig, maxFormDescriptionLen)
	}

	round, err := u.recruitment.GetRoundByID(ctx, in.RoundID)
	if err != nil {
		return nil, nil, err
	}

	directionCode := strings.TrimSpace(in.DirectionCode)
	direction, err := u.recruitment.GetDirectionByCode(ctx, directionCode)
	if err != nil {
		return nil, nil, err
	}

	fields, err := validateFieldInputs(in.Fields)
	if err != nil {
		return nil, nil, err
	}

	for attempt := 0; attempt < shareTokenMaxAttempts; attempt++ {
		token, err := newShareToken()
		if err != nil {
			return nil, nil, err
		}

		form := &domain.Form{
			Title:         title,
			Description:   description,
			RoundID:       round.ID,
			DirectionCode: direction.Code,
			ShareToken:    token,
			// 新建默认关闭：先建好再显式开放，避免建表当场收进一堆测试提交。
			IsOpen: false,
		}

		err = u.forms.CreateForm(ctx, form, fields)
		if errors.Is(err, repository.ErrDuplicateShareToken) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		return form, fields, nil
	}

	return nil, nil, errors.New("生成分享令牌失败，请重试")
}

// UpdateForm 改表基本字段（标题 / 说明 / 开放状态），可选整份替换字段列表。
//
// 绑定（轮次 / 方向）不在可改范围内：入参里根本没有这两个字段。
// 返回更新后的字段与提交数，让调用方不必再查一次（管理端详情要展示提交数）。
func (u *FormUseCase) UpdateForm(ctx context.Context, formID int, in UpdateFormInput) (*domain.Form, []*domain.FormField, int64, error) {
	form, err := u.forms.GetFormByID(ctx, formID)
	if err != nil {
		return nil, nil, 0, err
	}

	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return nil, nil, 0, fmt.Errorf("%w: 表单标题不能为空", ErrFormConfig)
		}
		if len([]rune(title)) > maxFormTitleLen {
			return nil, nil, 0, fmt.Errorf("%w: 表单标题不能超过 %d 字", ErrFormConfig, maxFormTitleLen)
		}
		form.Title = title
	}

	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		if len([]rune(description)) > maxFormDescriptionLen {
			return nil, nil, 0, fmt.Errorf("%w: 表单说明不能超过 %d 字", ErrFormConfig, maxFormDescriptionLen)
		}
		form.Description = description
	}

	if in.IsOpen != nil {
		form.IsOpen = *in.IsOpen
	}

	var fields []*domain.FormField
	if in.Fields != nil {
		fields, err = validateFieldInputs(*in.Fields)
		if err != nil {
			return nil, nil, 0, err
		}
	}

	if err := u.forms.UpdateForm(ctx, form, fields); err != nil {
		return nil, nil, 0, err
	}

	current, err := u.forms.ListFields(ctx, form.ID)
	if err != nil {
		return nil, nil, 0, err
	}

	counts, err := u.forms.CountSubmissions(ctx, []int{form.ID})
	if err != nil {
		return nil, nil, 0, err
	}

	return form, current, counts[form.ID], nil
}

// ListForms 分页列出表单，附带每张表的提交数（管理端列表要显示）。
func (u *FormUseCase) ListForms(ctx context.Context, page int, pageSize int) ([]*domain.Form, map[int]int64, int64, error) {
	forms, total, err := u.forms.ListForms(ctx, page, pageSize)
	if err != nil {
		return nil, nil, 0, err
	}

	ids := make([]int, 0, len(forms))
	for _, f := range forms {
		ids = append(ids, f.ID)
	}
	counts, err := u.forms.CountSubmissions(ctx, ids)
	if err != nil {
		return nil, nil, 0, err
	}
	return forms, counts, total, nil
}

// GetFormDetail 管理端表单详情：定义 + 字段 + 提交数。
func (u *FormUseCase) GetFormDetail(ctx context.Context, formID int) (*domain.Form, []*domain.FormField, int64, error) {
	form, err := u.forms.GetFormByID(ctx, formID)
	if err != nil {
		return nil, nil, 0, err
	}

	fields, err := u.forms.ListFields(ctx, form.ID)
	if err != nil {
		return nil, nil, 0, err
	}

	counts, err := u.forms.CountSubmissions(ctx, []int{form.ID})
	if err != nil {
		return nil, nil, 0, err
	}
	return form, fields, counts[form.ID], nil
}

/* ------------------------------------------------------------------ *
 * 用户端：按分享令牌读取与提交
 * ------------------------------------------------------------------ */

// ReadShareForm 按分享令牌读取表单定义（调用方已通过 AuthRequired）。
//
// 关闭的表单照样返回定义（is_open=false），让填写页能给出"已关闭"的明确反馈，
// 而不是一个像链接失效一样的 404。mySubmission 是当前用户已提交的记录，
// 用于页面提示"你已经提交过"（提交防重最终仍由数据库唯一约束兜底）。
func (u *FormUseCase) ReadShareForm(ctx context.Context, token string, userID int) (*domain.Form, []*domain.FormField, *domain.FormSubmission, error) {
	form, err := u.forms.GetFormByShareToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, nil, nil, err
	}

	fields, err := u.forms.ListFields(ctx, form.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	submission, err := u.forms.GetSubmissionBySubmitter(ctx, form.ID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrSubmissionNotFound) {
			submission = nil
		} else {
			return nil, nil, nil, err
		}
	}

	return form, fields, submission, nil
}

// SubmitByShareToken 登录用户经分享链接提交。
//
// 顺序刻意如此：
//  1. 令牌 → 表单；表单关闭直接拒绝（不落任何记录）
//  2. 服务端校验答案（必填 / 类型 / select 选项合法性 / 未知字段）
//  3. 查该用户是否已提交过 → 直接返回 ErrDuplicateSubmission，此时零副作用
//  4. 走统一申请模型建申请（同轮同方向已报名会拿到重复报名错误）
//  5. 落提交记录，application_id 指回第 4 步的申请
//
// 第 4 步失败时不会有提交记录，第 5 步失败时数据库唯一约束仍挡住重复 ——
// 两道唯一索引（(form_id, submitter_id) 与 (user_id, round_id, direction_code)）
// 是防重的最终依据，第 3 步只是为了给出干净的错误而不是让用户看到约束冲突。
func (u *FormUseCase) SubmitByShareToken(ctx context.Context, token string, userID int, values map[string]any) (*domain.FormSubmission, *domain.Application, error) {
	form, err := u.forms.GetFormByShareToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, nil, err
	}
	if !form.IsOpen {
		return nil, nil, ErrFormClosed
	}

	fields, err := u.forms.ListFields(ctx, form.ID)
	if err != nil {
		return nil, nil, err
	}

	answers, err := validateAnswers(fields, values)
	if err != nil {
		return nil, nil, err
	}

	if _, err := u.forms.GetSubmissionBySubmitter(ctx, form.ID, userID); err == nil {
		return nil, nil, repository.ErrDuplicateSubmission
	} else if !errors.Is(err, repository.ErrSubmissionNotFound) {
		return nil, nil, err
	}

	application, err := u.applications.SubmitToRound(ctx, userID, form.RoundID, form.DirectionCode)
	if err != nil {
		return nil, nil, err
	}

	submission := &domain.FormSubmission{
		FormID:        form.ID,
		SubmitterID:   userID,
		ApplicationID: application.ID,
		Answers:       answers,
	}
	if err := u.forms.CreateSubmission(ctx, submission); err != nil {
		return nil, nil, err
	}

	return submission, application, nil
}

/* ------------------------------------------------------------------ *
 * 管理端：提交记录
 * ------------------------------------------------------------------ */

// ListSubmissions 分页列出某表单的提交记录；表单不存在返回 ErrFormNotFound。
func (u *FormUseCase) ListSubmissions(ctx context.Context, formID int, page int, pageSize int) ([]*domain.FormSubmission, int64, error) {
	if _, err := u.forms.GetFormByID(ctx, formID); err != nil {
		return nil, 0, err
	}
	return u.forms.ListSubmissions(ctx, formID, page, pageSize)
}

// GetSubmission 取某表单下一条提交详情。
func (u *FormUseCase) GetSubmission(ctx context.Context, formID int, submissionID int) (*domain.FormSubmission, error) {
	if _, err := u.forms.GetFormByID(ctx, formID); err != nil {
		return nil, err
	}
	return u.forms.GetSubmission(ctx, formID, submissionID)
}

/* ------------------------------------------------------------------ *
 * 校验
 * ------------------------------------------------------------------ */

// validateFieldInputs 校验并把入参转成领域字段。
//
// 建表与改表共用：改表时带 id 的字段会被原地更新，因此这里也要认 id。
func validateFieldInputs(inputs []FormFieldInput) ([]*domain.FormField, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: 表单至少需要一个字段", ErrFormConfig)
	}
	if len(inputs) > maxFormFields {
		return nil, fmt.Errorf("%w: 字段数量不能超过 %d 个", ErrFormConfig, maxFormFields)
	}

	out := make([]*domain.FormField, 0, len(inputs))
	seenIDs := make(map[int]bool, len(inputs))

	for i, in := range inputs {
		fieldType := domain.FormFieldType(strings.TrimSpace(in.Type))
		if !fieldType.Valid() {
			return nil, fmt.Errorf("%w: 第 %d 个字段的类型 %q 不在支持范围（%s）",
				ErrFormConfig, i+1, in.Type, strings.Join(domain.FormFieldTypes(), " / "))
		}

		label := strings.TrimSpace(in.Label)
		if label == "" {
			return nil, fmt.Errorf("%w: 第 %d 个字段缺少标题", ErrFormConfig, i+1)
		}
		if len([]rune(label)) > maxFieldLabelLen {
			return nil, fmt.Errorf("%w: 字段「%s」的标题不能超过 %d 字", ErrFormConfig, label, maxFieldLabelLen)
		}

		placeholder := strings.TrimSpace(in.Placeholder)
		if len([]rune(placeholder)) > maxFieldPlaceholderLen {
			return nil, fmt.Errorf("%w: 字段「%s」的占位提示不能超过 %d 字", ErrFormConfig, label, maxFieldPlaceholderLen)
		}

		var options domain.StringList
		if fieldType == domain.FieldSelect {
			if len(in.Options) == 0 {
				return nil, fmt.Errorf("%w: 字段「%s」是 select，至少要有一个候选项", ErrFormConfig, label)
			}
			if len(in.Options) > maxSelectOptions {
				return nil, fmt.Errorf("%w: 字段「%s」的候选项不能超过 %d 个", ErrFormConfig, label, maxSelectOptions)
			}
			cleaned := make([]string, 0, len(in.Options))
			dup := make(map[string]bool, len(in.Options))
			for _, opt := range in.Options {
				opt = strings.TrimSpace(opt)
				if opt == "" {
					return nil, fmt.Errorf("%w: 字段「%s」存在空候选项", ErrFormConfig, label)
				}
				if dup[opt] {
					return nil, fmt.Errorf("%w: 字段「%s」的候选项 %q 重复", ErrFormConfig, label, opt)
				}
				dup[opt] = true
				cleaned = append(cleaned, opt)
			}
			options = cleaned
		} else if len(in.Options) > 0 {
			// 共享类型里 options 只属于 select；其他类型带候选项是明确的配置错误，
			// 静默忽略会让管理端以为"选项配上了"。
			return nil, fmt.Errorf("%w: 只有 select 字段可以配置候选项（字段「%s」是 %s）", ErrFormConfig, label, fieldType)
		}

		if in.ID > 0 {
			if seenIDs[in.ID] {
				return nil, fmt.Errorf("%w: 字段 id %d 重复出现", ErrFormConfig, in.ID)
			}
			seenIDs[in.ID] = true
		}

		out = append(out, &domain.FormField{
			ID:          in.ID,
			Type:        fieldType,
			Label:       label,
			Required:    in.Required,
			Placeholder: placeholder,
			Options:     options,
			SortOrder:   i,
		})
	}

	return out, nil
}

// validateAnswers 校验提交答案并规范化成落库形态。
//
// 键是字段 id 的十进制字符串（与共享类型 FormSubmission.values 一致）。
// 未填写且非必填的字段不落键 —— 存一堆空串会让"填了空 vs 没填"分不清。
func validateAnswers(fields []*domain.FormField, values map[string]any) (domain.FormAnswers, error) {
	known := make(map[string]*domain.FormField, len(fields))
	for _, f := range fields {
		known[strconv.Itoa(f.ID)] = f
	}

	for key := range values {
		if _, ok := known[strings.TrimSpace(key)]; !ok {
			return nil, fmt.Errorf("%w: 提交里含不属于该表单的字段 %q", ErrAnswerInvalid, key)
		}
	}

	answers := make(domain.FormAnswers, len(fields))
	for key, field := range known {
		raw, present := values[key]

		switch field.Type {
		case domain.FieldText, domain.FieldEmail, domain.FieldTextarea:
			text, err := answerText(field, raw, present)
			if err != nil {
				return nil, err
			}
			if text == "" {
				continue
			}
			if field.Type == domain.FieldEmail && !emailAnswerRe.MatchString(text) {
				return nil, fmt.Errorf("%w: 字段「%s」不是合法的邮箱地址", ErrAnswerInvalid, field.Label)
			}
			answers[key] = text

		case domain.FieldSelect:
			text, err := answerText(field, raw, present)
			if err != nil {
				return nil, err
			}
			if text == "" {
				continue
			}
			if !containsString(field.Options, text) {
				return nil, fmt.Errorf("%w: 字段「%s」的答案不在候选项内", ErrAnswerInvalid, field.Label)
			}
			answers[key] = text

		case domain.FieldCheckbox:
			chosen, err := answerCheckbox(field, raw, present)
			if err != nil {
				return nil, err
			}
			if len(chosen) == 0 {
				continue
			}
			answers[key] = chosen

		default:
			// 类型在落库前已校验过；走到这里说明库里存了脏类型，
			// 宁可报错也不猜它该怎么解析。
			return nil, fmt.Errorf("%w: 字段「%s」的类型 %q 无法处理", ErrFormConfig, field.Label, field.Type)
		}
	}

	return answers, nil
}

// answerText 取文本类答案：必填校验 + 长度上限。
func answerText(field *domain.FormField, raw any, present bool) (string, error) {
	if !present || raw == nil {
		if field.Required {
			return "", fmt.Errorf("%w: 必填项「%s」未填写", ErrAnswerInvalid, field.Label)
		}
		return "", nil
	}

	text, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%w: 字段「%s」需要文本答案", ErrAnswerInvalid, field.Label)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		if field.Required {
			return "", fmt.Errorf("%w: 必填项「%s」未填写", ErrAnswerInvalid, field.Label)
		}
		return "", nil
	}
	if len([]rune(text)) > maxAnswerRunes {
		return "", fmt.Errorf("%w: 字段「%s」的答案不能超过 %d 字", ErrAnswerInvalid, field.Label, maxAnswerRunes)
	}
	return text, nil
}

// answerCheckbox 取多选答案。
//
// 共享类型里 options 只属于 select（checkboxes 没有候选项约束），
// 因此这里只校验"是字符串数组 + 逐项非空 + 条数上限"，
// 不拿候选项比对 —— 候选约束要生效必须先改共享类型契约。
func answerCheckbox(field *domain.FormField, raw any, present bool) ([]string, error) {
	if !present || raw == nil {
		if field.Required {
			return nil, fmt.Errorf("%w: 必填项「%s」未勾选", ErrAnswerInvalid, field.Label)
		}
		return nil, nil
	}

	var items []any
	switch v := raw.(type) {
	case []any:
		items = v
	case []string:
		items = make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
	default:
		return nil, fmt.Errorf("%w: 字段「%s」需要多选答案（字符串数组）", ErrAnswerInvalid, field.Label)
	}

	if len(items) > maxCheckboxAnswerItems {
		return nil, fmt.Errorf("%w: 字段「%s」最多选择 %d 项", ErrAnswerInvalid, field.Label, maxCheckboxAnswerItems)
	}

	chosen := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%w: 字段「%s」的多选答案里含非字符串项", ErrAnswerInvalid, field.Label)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if len([]rune(text)) > maxAnswerRunes {
			return nil, fmt.Errorf("%w: 字段「%s」的某项答案过长", ErrAnswerInvalid, field.Label)
		}
		if seen[text] {
			continue
		}
		seen[text] = true
		chosen = append(chosen, text)
	}

	if len(chosen) == 0 && field.Required {
		return nil, fmt.Errorf("%w: 必填项「%s」未勾选", ErrAnswerInvalid, field.Label)
	}
	return chosen, nil
}

// containsString 候选项比对。
func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// newShareToken 生成分享令牌：128bit 随机数的 hex 表示。
//
// 用 crypto/rand 而不是可预测的递增 id：令牌本身就是分享入口的"地址"，
// 可猜的令牌等于把别人的报名表暴露给全网。
func newShareToken() (string, error) {
	buf := make([]byte, shareTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成分享令牌失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
