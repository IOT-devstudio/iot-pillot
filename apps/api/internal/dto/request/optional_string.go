package request

import "encoding/json"

// OptionalString JSON 三态字符串：区分「没传这个键」「传了 null」「传了字符串」。
//
// 与 OptionalInt 同一个理由：`*string` 会把 null 与「键没出现」解成同一个 nil，
// 于是「传 null = 仍在参与」被静默当成「不改」。
//
// 请求层只负责解出三态，**不做时间解析**——时间格式属于 handler 的职责
// （它知道领域层的时间布局），放进请求包会让这一层知道不该知道的东西。
type OptionalString struct {
	// Set 表示请求体里出现过这个键（无论值是不是 null）。
	Set bool
	// Value 是解析出的值；Set 为 true 且 Value 为 nil 表示显式传了 null。
	Value *string
}

// UnmarshalJSON 实现 json.Unmarshaler。
func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}
