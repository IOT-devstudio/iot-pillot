package request

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

// 资料字段长度上限：与前端个人设置表单的 maxlength 一一对应
// （apps/web/src/modules/user-settings/components/UserSettingsForm.vue）。
const (
	maxClassLen = 64
	maxQQLen    = 20
)

// UpdateMePatch PUT /me 的 body 文档形状（swag 注解引用）。
//
// 实际解析走 ParseUpdateMe —— 这里的指针字段只为在 OpenAPI 里表达
// 「可选且可为 null」；请求体的三态语义见 ParseUpdateMe。
type UpdateMePatch struct {
	Class     *string `json:"class" example:"物联网工程 2301"`                                                        // 班级；null 清除，缺省不动
	StudentID *int    `json:"student_id" example:2023114514`                                                     // 学号；null 清除（0 视为未填），缺省不动
	QQ        *string `json:"qq" example:"1044696157"`                                                           // QQ；null 清除，缺省不动
	Direction *string `json:"direction" example:"front-end" Enums(front-end, back-end, agent, all, game, other)` // 方向；null/空串清除，缺省不动
}

// PatchField 白名单字段的三态值。
//
// Go 的指针区分不了「JSON 里没这个键」和「键的值是 null」，而本接口两者
// 语义不同（缺省 = 不动该字段，null = 清除该字段），所以解析时显式记录
// Present / Clear，不靠指针本身承载语义。
type PatchField[T any] struct {
	Present bool // body 里出现了这个键
	Clear   bool // 出现且值为 null（direction 另允许空串，同义）
	Value   T    // Clear == false 时有效
}

// UpdateMeReq PUT /me 的字段补丁。
//
// 只有白名单四个键会进到这里：body 里的其他键（如 user_id）在解析阶段
// 直接丢弃 —— 目标永远是令牌本人，越权在结构上不可能（issue #57 验收项）。
type UpdateMeReq struct {
	Class     PatchField[string]
	StudentID PatchField[int]
	QQ        PatchField[string]
	Direction PatchField[string]
}

// ParseUpdateMe 解析并校验 PUT /me 的请求体。
//
// 三态语义：
//   - 键缺失      → Present=false，更新时不动该字段
//   - 值为 null   → Present=true, Clear=true，清除该字段
//   - 有值        → Present=true，覆盖（校验失败返回中文错误）
//
// 校验规则与前端表单一致：class ≤ 64 字符、qq ≤ 20 字符、student_id
// 为非负整数、direction 必须是 domain.Direction 枚举之一。
// 返回的 error 文案直接外发给调用方，措辞必须是用户能看懂的中文。
func ParseUpdateMe(body []byte) (*UpdateMeReq, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("请求体必须是 JSON 对象")
	}
	if raw == nil {
		// body 是字面量 null：等价于空补丁，什么都不改
		return &UpdateMeReq{}, nil
	}

	req := &UpdateMeReq{}

	class, err := parsePatchString(raw, "class", maxClassLen)
	if err != nil {
		return nil, err
	}
	req.Class = class

	studentID, err := parsePatchStudentID(raw)
	if err != nil {
		return nil, err
	}
	req.StudentID = studentID

	qq, err := parsePatchString(raw, "qq", maxQQLen)
	if err != nil {
		return nil, err
	}
	req.QQ = qq

	direction, err := parsePatchString(raw, "direction", maxQQLen)
	if err != nil {
		return nil, err
	}
	// 空串与 null 同义（清除）：前端表单已把空串转成 null，
	// 这里兼容直接发空串的客户端，不把它当非法枚举拒掉。
	if direction.Present && !direction.Clear && direction.Value == "" {
		direction.Clear = true
	}
	if direction.Present && !direction.Clear && !isDirection(direction.Value) {
		return nil, fmt.Errorf("direction 必须是 front-end / back-end / agent / all / game / other 之一")
	}
	req.Direction = direction

	return req, nil
}

// parsePatchString 解析一个可 null 的字符串字段。key 只会是白名单常量。
func parsePatchString(raw map[string]json.RawMessage, key string, maxLen int) (PatchField[string], error) {
	var field PatchField[string]
	b, ok := raw[key]
	if !ok {
		return field, nil // 缺省 = 不动
	}
	field.Present = true

	var v *string
	if err := json.Unmarshal(b, &v); err != nil {
		return field, fmt.Errorf("%s 必须是字符串", key)
	}
	if v == nil {
		field.Clear = true
		return field, nil
	}
	if utf8.RuneCountInString(*v) > maxLen {
		return field, fmt.Errorf("%s 不能超过 %d 个字符", key, maxLen)
	}
	field.Value = *v
	return field, nil
}

// parsePatchStudentID 解析可 null 的学号。小数 / 字符串 / 负数都拒绝。
func parsePatchStudentID(raw map[string]json.RawMessage) (PatchField[int], error) {
	var field PatchField[int]
	b, ok := raw["student_id"]
	if !ok {
		return field, nil
	}
	field.Present = true

	var v *int
	// json 无法把 3.7 / "123" 解进 int，会返回 UnmarshalTypeError ——
	// 统一翻译成同一句中文，不把 encoding/json 的英文错误透出去。
	if err := json.Unmarshal(b, &v); err != nil {
		return field, fmt.Errorf("student_id 必须是非负整数")
	}
	if v == nil {
		field.Clear = true
		return field, nil
	}
	if *v < 0 {
		return field, fmt.Errorf("student_id 必须是非负整数")
	}
	field.Value = *v
	return field, nil
}

// isDirection 校验枚举值，列表与 domain.Direction 常量同源。
func isDirection(v string) bool {
	switch domain.Direction(v) {
	case domain.FrontEnd, domain.BackEnd, domain.Agent, domain.All, domain.Game, domain.Other:
		return true
	}
	return false
}
