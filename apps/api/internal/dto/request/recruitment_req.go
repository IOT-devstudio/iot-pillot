package request

// RecruitmentSubmitReq 用户报名请求。
//
// 归属一律由令牌决定，body 里故意不设 user_id 字段 ——
// 防止越权代他人报名。
type RecruitmentSubmitReq struct {
	DirectionCode string `json:"direction_code" binding:"required"`
}

// RecruitmentReviewReq 管理端审核申请。
//
// Target 必须是状态机合法目标：applied→reviewing，reviewing→accepted/rejected。
// 服务端会校验流转合法性，非法目标返回业务错误且不产生任何写操作。
type RecruitmentReviewReq struct {
	Target string `json:"target" binding:"required,oneof=reviewing accepted rejected"`
}

// RecruitmentSetRoundReq 管理端配置轮次开放状态。
type RecruitmentSetRoundReq struct {
	Accepting bool `json:"accepting"`
}

// RecruitmentSetDirectionReq 管理端配置方向开关。
type RecruitmentSetDirectionReq struct {
	Open bool `json:"open"`
}