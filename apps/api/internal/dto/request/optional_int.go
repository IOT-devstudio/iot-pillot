package request

import "encoding/json"

// OptionalInt JSON 三态整数：区分「没传这个键」「传了 null」「传了数字」。
//
// 为什么不能用 **int：encoding/json 对 null 的规则是「把指针置为 nil」，
// 于是 {"user_id":null} 与「键完全没出现」解出来**一模一样**（都是 nil）。
// 而这两个语义在这里必须分开：
//
//	键没出现 = 不改账号绑定；
//	传 null  = 解除账号绑定。
//
// 用 **int 会让「解除绑定」被静默当成「不改」，界面上点了解除、刷新后绑定还在。
//
// 自定义 UnmarshalJSON 是唯一能真正区分它们的办法（Set 标记键是否出现过）。
type OptionalInt struct {
	// Set 表示请求体里出现过这个键（无论值是不是 null）。
	Set bool
	// Value 是解析出的值；Set 为 true 且 Value 为 nil 表示显式传了 null。
	Value *int
}

// UnmarshalJSON 实现 json.Unmarshaler。
func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value int
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}
