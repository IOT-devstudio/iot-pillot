package request

// FormFieldReq 建表 / 改表时的字段入参。
//
// ID 缺省（0）= 新增字段；带 id = 原地更新——保留 id 意味着历史提交里的答案
// 仍然对得上字段，改个标题不会让旧答案变成孤儿。
type FormFieldReq struct {
	ID          int      `json:"id"`
	Type        string   `json:"type" binding:"required"`
	Label       string   `json:"label" binding:"required"`
	Required    bool     `json:"required"`
	Placeholder string   `json:"placeholder"`
	Options     []string `json:"options"`
}

// FormCreateReq 建表请求。
//
// 不接受 is_open：新建一律是关闭状态，开放走 PATCH —— 免得"建表时顺手打开"
// 变成收测试数据的口子。轮次与方向只在建表时确定（绑定创建后不可变）。
type FormCreateReq struct {
	Title         string         `json:"title" binding:"required"`
	Description   string         `json:"description"`
	RoundID       int            `json:"round_id" binding:"required"`
	DirectionCode string         `json:"direction_code" binding:"required"`
	Fields        []FormFieldReq `json:"fields" binding:"required,min=1"`
}

// FormUpdateReq 改表请求：只传要改的项。
//
// round_id / direction_code 故意不在入参里：表单的招新轮次与方向创建后不可变，
// 要换就新建表单（否则历史提交指向的申请归属会与新定义矛盾）。
type FormUpdateReq struct {
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	IsOpen      *bool           `json:"is_open"`
	Fields      *[]FormFieldReq `json:"fields"`
}

// FormSubmitReq 提交请求。
//
// 只有 values，没有 user_id：提交人一律由令牌决定（防越权代交）。
// values 的键是字段 id 的十进制字符串，值是字符串（text/email/textarea/select）
// 或字符串数组（checkbox），与共享类型 FormSubmission.values 一致。
type FormSubmitReq struct {
	Values map[string]any `json:"values"`
}
