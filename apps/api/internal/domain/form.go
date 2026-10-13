package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// FormFieldType 表单字段类型。
//
// 首期五种，与 packages/shared-types/src/form.ts 的 FormFieldType 一一对应。
// 新增类型必须同时改这里、共享类型与前端渲染 —— 少改一处就是"类型漂移"。
type FormFieldType string

const (
	FieldText     FormFieldType = "text"
	FieldEmail    FormFieldType = "email"
	FieldTextarea FormFieldType = "textarea"
	FieldSelect   FormFieldType = "select"
	FieldCheckbox FormFieldType = "checkbox"
)

// Valid 报告该类型是否在本期支持范围内。
//
// 服务层建表与提交校验都走它：未知类型在这里就被挡住，
// 不会等到前端渲染时才发现。
func (t FormFieldType) Valid() bool {
	switch t {
	case FieldText, FieldEmail, FieldTextarea, FieldSelect, FieldCheckbox:
		return true
	}
	return false
}

// FormFieldTypes 全部可用类型（校验错误提示里列举用）。
func FormFieldTypes() []string {
	return []string{
		string(FieldText),
		string(FieldEmail),
		string(FieldTextarea),
		string(FieldSelect),
		string(FieldCheckbox),
	}
}

// Form 表单定义。
//
// 绑定 (RoundID, DirectionCode)：提交时按 #88 的统一申请模型建申请，
// 表单自身不重复建模申请。绑定在创建后不可修改（要换轮次或方向就新建表单）——
// 否则历史提交记录里的 application_id 会与表单定义不一致。
type Form struct {
	ID            int       `json:"id" gorm:"primaryKey"`
	Title         string    `json:"title" gorm:"column:title"`
	Description   string    `json:"description" gorm:"column:description"`
	RoundID       int       `json:"round_id" gorm:"column:round_id;index"`
	DirectionCode string    `json:"direction_code" gorm:"column:direction_code"`
	ShareToken    string    `json:"share_token" gorm:"column:share_token;uniqueIndex"`
	IsOpen        bool      `json:"is_open" gorm:"column:is_open"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// FormField 表单的一个字段。
//
// ID 由客户端在编辑时回传即可原地更新：保留 id 意味着历史提交里的答案
// 仍然对得上字段，仅改标题/占位不会让旧答案变成孤儿。
type FormField struct {
	ID          int           `json:"id" gorm:"primaryKey"`
	FormID      int           `json:"form_id" gorm:"column:form_id;index"`
	Type        FormFieldType `json:"type" gorm:"column:field_type"`
	Label       string        `json:"label" gorm:"column:label"`
	Required    bool          `json:"required" gorm:"column:required"`
	Placeholder string        `json:"placeholder" gorm:"column:placeholder"`
	// Options 仅 select 使用；其他类型为 nil（迁移里对应 jsonb 的 NULL）。
	Options   StringList `json:"options,omitempty" gorm:"column:options;type:jsonb"`
	SortOrder int        `json:"sort_order" gorm:"column:sort_order"`
	CreatedAt time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// FormSubmission 一条表单提交。
//
// 归属由令牌决定（SubmitterID 来自鉴权结果，请求体不接受任何用户标识）；
// ApplicationID 指向统一申请模型的申请，重复提交不会新增第二条申请。
type FormSubmission struct {
	ID            int         `json:"id" gorm:"primaryKey"`
	FormID        int         `json:"form_id" gorm:"column:form_id;index"`
	SubmitterID   int         `json:"submitter_id" gorm:"column:submitter_id;index"`
	ApplicationID int         `json:"application_id" gorm:"column:application_id;index"`
	Answers       FormAnswers `json:"answers" gorm:"column:answers;type:jsonb"`
	CreatedAt     time.Time   `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time   `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// FormAnswers 提交答案：字段 id（十进制字符串）→ 答案值。
//
// 落库是 jsonb。显式实现 Scanner/Valuer 而不是依赖 ORM 的 JSON 支持：
// 服务层要在写库前用它做键校验（键必须对应真实字段 id），
// 序列化边界只允许出现在这一个类型里。
type FormAnswers map[string]any

// Value 实现 driver.Valuer：序列化成 JSON 文本，交给 pg 写进 jsonb 列。
func (a FormAnswers) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("序列化表单答案失败: %w", err)
	}
	return string(raw), nil
}

// Scan 实现 sql.Scanner：把 jsonb 列读回 map。
func (a *FormAnswers) Scan(src any) error {
	out := FormAnswers{}

	switch v := src.(type) {
	case nil:
		*a = out
		return nil
	case []byte:
		if len(v) == 0 {
			*a = out
			return nil
		}
		if err := json.Unmarshal(v, &out); err != nil {
			return fmt.Errorf("解析表单答案列失败: %w", err)
		}
	case string:
		if v == "" {
			*a = out
			return nil
		}
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return fmt.Errorf("解析表单答案列失败: %w", err)
		}
	default:
		return fmt.Errorf("无法解析表单答案列，类型 %T", src)
	}

	*a = out
	return nil
}

// StringList select 的候选项列表（jsonb 数组）。
//
// 与 FormAnswers 同理：单点实现 Scanner/Valuer，别处不再处理 JSON。
type StringList []string

// Value 实现 driver.Valuer。
func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	raw, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("序列化候选项失败: %w", err)
	}
	return string(raw), nil
}

// Scan 实现 sql.Scanner。
func (l *StringList) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*l = nil
		return nil
	case []byte:
		if len(v) == 0 {
			*l = nil
			return nil
		}
		var out []string
		if err := json.Unmarshal(v, &out); err != nil {
			return fmt.Errorf("解析候选项列失败: %w", err)
		}
		*l = out
		return nil
	case string:
		if v == "" {
			*l = nil
			return nil
		}
		var out []string
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return fmt.Errorf("解析候选项列失败: %w", err)
		}
		*l = out
		return nil
	default:
		return fmt.Errorf("无法解析候选项列，类型 %T", src)
	}
}
