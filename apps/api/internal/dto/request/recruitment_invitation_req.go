package request

// SendInvitationReq 管理端针对一个报名申请发送邀请。
//
// 同一申请已有 pending 邀请时不再新建（部分唯一索引兜底），复用该行重试投递：
// 这是「上次邮件没发出去」的安全重试路径；投递成功才会标记 email_sent_at。
type SendInvitationReq struct {
	ApplicationID int `json:"application_id" binding:"required,min=1"`
	// TemplateID invitation 类型模板的 id；不存在或类型不符由服务层校验并报业务错误。
	TemplateID int `json:"template_id" binding:"required,min=1"`
	// ExpiresInDays 邀请有效天数；缺省 7，上限 90（再长就失去"限期响应"的语义）。
	ExpiresInDays int `json:"expires_in_days" binding:"omitempty,min=1,max=90"`
	// Vars 模板变量覆盖。服务端会预填 invitation_token 等邀请上下文，
	// 调用方传同键会覆盖预填值。
	Vars MailVars `json:"vars"`
}

// AcceptInvitationReq 用户端接受邀请：凭邀请邮件 / 本人邀请列表里的 token。
type AcceptInvitationReq struct {
	Token string `json:"token" binding:"required,min=10,max=200"`
}

// 候选人状态推进动作（AdvanceCandidateReq.Action）。
const (
	// AdvanceActionSendOffer 发送 offer 邮件并把候选人推进到 offered（需模板）。
	AdvanceActionSendOffer = "send_offer"
	// AdvanceActionHire 录用：仅 offered 可执行，不发信。
	AdvanceActionHire = "hire"
	// AdvanceActionReject 拒绝并发送感谢信（需模板）；interviewing / offered 可执行。
	AdvanceActionReject = "reject"
)

// AdvanceCandidateReq 管理端推进候选人状态。
//
// 每次请求只做一个动作；send_offer / reject 必须带邮件模板，且**投递成功**
// 状态才会流转——邮件失败保持原状态，可安全重试。
type AdvanceCandidateReq struct {
	Action string `json:"action" binding:"required,oneof=send_offer hire reject"`
	// TemplateID offer / 感谢信模板 id；send_offer 与 reject 时必填。
	TemplateID int `json:"template_id" binding:"omitempty,min=1"`
	// Note 随本次流转写入时间线的备注（如面试记录）。
	Note string `json:"note" binding:"max=500"`
	// Vars 模板变量覆盖（offer / reject 发信时生效）。
	Vars MailVars `json:"vars"`
}
