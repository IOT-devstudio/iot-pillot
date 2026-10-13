package response

import (
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

// FormFieldResp 表单字段。
type FormFieldResp struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Required    bool     `json:"required"`
	Placeholder string   `json:"placeholder"`
	Options     []string `json:"options,omitempty"`
	SortOrder   int      `json:"sort_order"`
}

// FormResp 表单定义。管理端列表/详情与用户端填写页共用同一形状。
//
// ShareToken 在填写页也返回：前端正是靠它拼分享链接与提交地址，
// 它不是凭据（读取与提交都要求登录），泄露它不等于泄露数据。
type FormResp struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	RoundID         int    `json:"round_id"`
	DirectionCode   string `json:"direction_code"`
	ShareToken      string `json:"share_token"`
	IsOpen          bool   `json:"is_open"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	SubmissionCount int64  `json:"submission_count"`
}

// FormDetailResp 表单详情 = 定义 + 字段列表。
type FormDetailResp struct {
	Form   FormResp        `json:"form"`
	Fields []FormFieldResp `json:"fields"`
}

// FormListResp 表单分页列表。
type FormListResp struct {
	Items    []FormResp `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// FormSubmissionResp 一条提交记录。
//
// Values 的键是字段 id；ApplicationID 指回统一申请模型里那条申请 ——
// 提交人是谁、申请的审核状态都从申请侧读，避免两处各存一份而漂移。
type FormSubmissionResp struct {
	ID            int            `json:"id"`
	FormID        int            `json:"form_id"`
	SubmitterID   int            `json:"submitter_id"`
	ApplicationID int            `json:"application_id"`
	Values        map[string]any `json:"values"`
	SubmittedAt   string         `json:"submitted_at"`
}

// FormSubmissionListResp 提交分页列表。
type FormSubmissionListResp struct {
	Items    []FormSubmissionResp `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// ShareFormResp 分享链接读取结果：定义 + 字段 + 当前用户的提交情况。
//
// Submitted 为 true 时填写页应直接展示"已提交"，
// MySubmissionID 供详情跳转用。
type ShareFormResp struct {
	Form           FormResp        `json:"form"`
	Fields         []FormFieldResp `json:"fields"`
	Submitted      bool            `json:"submitted"`
	MySubmissionID int             `json:"my_submission_id"`
}

// SubmitFormResp 提交成功结果：提交记录 + 关联到的统一申请。
type SubmitFormResp struct {
	Submission        FormSubmissionResp `json:"submission"`
	ApplicationID     int                `json:"application_id"`
	ApplicationStatus string             `json:"application_status"`
}

// NewFormResp 领域表单 → 响应 DTO。
func NewFormResp(f *domain.Form, submissionCount int64) FormResp {
	return FormResp{
		ID:              f.ID,
		Title:           f.Title,
		Description:     f.Description,
		RoundID:         f.RoundID,
		DirectionCode:   f.DirectionCode,
		ShareToken:      f.ShareToken,
		IsOpen:          f.IsOpen,
		CreatedAt:       f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		SubmissionCount: submissionCount,
	}
}

// NewFormFieldResp 领域字段 → 响应 DTO。
func NewFormFieldResp(f *domain.FormField) FormFieldResp {
	return FormFieldResp{
		ID:          f.ID,
		Type:        string(f.Type),
		Label:       f.Label,
		Required:    f.Required,
		Placeholder: f.Placeholder,
		Options:     f.Options,
		SortOrder:   f.SortOrder,
	}
}

// NewFormFieldResps 批量转换字段（保证空列表序列化成 []，不是 null）。
func NewFormFieldResps(fields []*domain.FormField) []FormFieldResp {
	out := make([]FormFieldResp, 0, len(fields))
	for _, f := range fields {
		out = append(out, NewFormFieldResp(f))
	}
	return out
}

// NewFormSubmissionResp 领域提交 → 响应 DTO。
func NewFormSubmissionResp(s *domain.FormSubmission) FormSubmissionResp {
	values := s.Answers
	if values == nil {
		// nil map 会序列化成 null，前端读 values[key] 直接抛错；
		// 空对象才是"没有答案"的正确表示。
		values = domain.FormAnswers{}
	}
	return FormSubmissionResp{
		ID:            s.ID,
		FormID:        s.FormID,
		SubmitterID:   s.SubmitterID,
		ApplicationID: s.ApplicationID,
		Values:        values,
		SubmittedAt:   s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
