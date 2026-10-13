package handler

import (
	"errors"
	"strconv"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// RecruitmentInvitationHandler 邀请与候选人状态流转接口。
//
// 管理端动作的操作者（SentBy / OperatorID）一律取自 AuthRequired 写入的当前
// 用户，不接受请求体传入（同 MailHandler 的追责理由）；用户端归属核验在
// 服务层做（application.user_id == 当前登录用户），handler 只负责透传身份。
type RecruitmentInvitationHandler struct {
	service *service.RecruitmentInvitationUseCase
}

// NewRecruitmentInvitationHandler 构造 handler。
func NewRecruitmentInvitationHandler(service *service.RecruitmentInvitationUseCase) *RecruitmentInvitationHandler {
	return &RecruitmentInvitationHandler{service: service}
}

/* ------------------------------------------------------------------ *
 * 管理端
 * ------------------------------------------------------------------ */

// SendInvitation 发送/重试邀请
// @Summary 发送邀请（含投递失败重试）
// @Description 针对一个报名申请发送邀请邮件。已有 pending 邀请时复用该行重试投递；邮件投递失败不产生"已邀请"状态。
// @Tags 管理端-招新
// @Accept json
// @Produce json
// @Param request body request.SendInvitationReq true "邀请内容"
// @Success 200 {object} response.Result{data=response.InvitationResp} "成功"
// @Failure 400 {object} response.Result "申请不存在/状态不允许/无邮箱/模板类型不符/已有 pending 邀请"
// @Failure 500 {object} response.Result "邮件投递失败"
// @Security BearerAuth
// @Router /api/v1/admin/invitations/send [post]
func (h *RecruitmentInvitationHandler) SendInvitation(c *gin.Context) {
	var req request.SendInvitationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.service.SendInvitation(c.Request.Context(), currentUserID(c), &req)
	if err != nil {
		failInvitation(c, err)
		return
	}
	response.OK(c, result)
}

// ListInvitations 邀请列表
// @Summary 邀请列表（可按申请过滤）
// @Tags 管理端-招新
// @Produce json
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认 20"
// @Param application_id query int false "按申请过滤"
// @Success 200 {object} response.Result{data=response.InvitationListResp} "成功"
// @Security BearerAuth
// @Router /api/v1/admin/invitations [get]
func (h *RecruitmentInvitationHandler) ListInvitations(c *gin.Context) {
	page := parsePage(c)
	pageSize := parsePageSize(c)
	applicationID, _ := strconv.Atoi(c.Query("application_id"))

	result, err := h.service.ListInvitations(c.Request.Context(), page, pageSize, applicationID)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

// RevokeInvitation 撤销邀请
// @Summary 撤销邀请
// @Description pending（含已过期未处理的）邀请落为 revoked；已接受/已撤销的返回业务错误。
// @Tags 管理端-招新
// @Produce json
// @Param id path int true "邀请 ID"
// @Success 200 {object} response.Result{data=response.InvitationResp} "成功"
// @Failure 400 {object} response.Result "邀请不存在或状态不允许撤销"
// @Security BearerAuth
// @Router /api/v1/admin/invitations/{id}/revoke [post]
func (h *RecruitmentInvitationHandler) RevokeInvitation(c *gin.Context) {
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.service.RevokeInvitation(c.Request.Context(), id)
	if err != nil {
		failInvitation(c, err)
		return
	}
	response.OK(c, result)
}

// ListCandidates 候选人列表
// @Summary 候选人列表（可按状态过滤）
// @Tags 管理端-招新
// @Produce json
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认 20"
// @Param status query string false "interviewing|offered|hired|rejected"
// @Success 200 {object} response.Result{data=response.CandidateListResp} "成功"
// @Security BearerAuth
// @Router /api/v1/admin/candidates [get]
func (h *RecruitmentInvitationHandler) ListCandidates(c *gin.Context) {
	page := parsePage(c)
	pageSize := parsePageSize(c)

	result, err := h.service.ListCandidates(c.Request.Context(), page, pageSize, c.Query("status"))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

// CandidateDetail 候选人详情
// @Summary 候选人详情（含时间线）
// @Tags 管理端-招新
// @Produce json
// @Param id path int true "候选人 ID"
// @Success 200 {object} response.Result{data=response.CandidateDetailResp} "成功"
// @Failure 400 {object} response.Result "候选人不存在"
// @Security BearerAuth
// @Router /api/v1/admin/candidates/{id} [get]
func (h *RecruitmentInvitationHandler) CandidateDetail(c *gin.Context) {
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.service.CandidateDetail(c.Request.Context(), id)
	if err != nil {
		failInvitation(c, err)
		return
	}
	response.OK(c, result)
}

// AdvanceCandidate 推进候选人状态
// @Summary 推进候选人状态（send_offer / hire / reject）
// @Description send_offer 与 reject 在邮件投递成功后才流转状态；失败保持原状态可安全重试。
// @Tags 管理端-招新
// @Accept json
// @Produce json
// @Param id path int true "候选人 ID"
// @Param request body request.AdvanceCandidateReq true "动作与模板"
// @Success 200 {object} response.Result{data=response.CandidateResp} "成功"
// @Failure 400 {object} response.Result "非法流转/缺模板/候选人不存在"
// @Failure 500 {object} response.Result "邮件投递失败"
// @Security BearerAuth
// @Router /api/v1/admin/candidates/{id}/advance [post]
func (h *RecruitmentInvitationHandler) AdvanceCandidate(c *gin.Context) {
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	var req request.AdvanceCandidateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.service.AdvanceCandidate(c.Request.Context(), currentUserID(c), id, &req)
	if err != nil {
		failInvitation(c, err)
		return
	}
	response.OK(c, result)
}

/* ------------------------------------------------------------------ *
 * 用户端
 * ------------------------------------------------------------------ */

// ListMyInvitations 我的邀请
// @Summary 本人邀请列表
// @Description 只含归属本人申请的邀请；expired 由服务端按当前时刻判定。
// @Tags 用户端-招新
// @Produce json
// @Success 200 {object} response.Result{data=response.MyInvitationListResp} "成功"
// @Security BearerAuth
// @Router /api/v1/me/invitations [get]
func (h *RecruitmentInvitationHandler) ListMyInvitations(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "缺少认证信息")
		return
	}

	result, err := h.service.ListMyInvitations(c.Request.Context(), userID)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

// AcceptInvitation 接受邀请
// @Summary 接受邀请
// @Description 必须登录且当前账号是邀请对应申请的归属人；重复接受幂等返回当前候选人视图。
// @Tags 用户端-招新
// @Accept json
// @Produce json
// @Param request body request.AcceptInvitationReq true "邀请 token"
// @Success 200 {object} response.Result{data=response.AcceptInvitationResp} "成功（含幂等重复接受）"
// @Failure 400 {object} response.Result "token 无效/已过期/已撤销/非归属人"
// @Security BearerAuth
// @Router /api/v1/me/invitations/accept [post]
func (h *RecruitmentInvitationHandler) AcceptInvitation(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "缺少认证信息")
		return
	}

	var req request.AcceptInvitationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.service.AcceptInvitation(c.Request.Context(), userID, &req)
	if err != nil {
		failInvitation(c, err)
		return
	}
	response.OK(c, result)
}

// ListMyCandidates 我的候选人记录
// @Summary 本人候选人记录
// @Tags 用户端-招新
// @Produce json
// @Success 200 {object} response.Result{data=response.MyCandidateListResp} "成功"
// @Security BearerAuth
// @Router /api/v1/me/candidates [get]
func (h *RecruitmentInvitationHandler) ListMyCandidates(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "缺少认证信息")
		return
	}

	result, err := h.service.ListMyCandidates(c.Request.Context(), userID)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

/* ------------------------------------------------------------------ *
 * 辅助
 * ------------------------------------------------------------------ */

// failInvitation 邀请/候选人服务错误 → HTTP 映射：
// 归属不匹配是 403（越权），请求/状态问题是 400，投递失败是 500（同 failMailRequest 理由）。
// 命名避开 #88 申请域的 failRecruitment：同包两套映射，各自可读。
func failInvitation(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotApplicationOwner):
		response.Fail(c, 403, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrMailDelivery):
		failInternal(c, err)
	case errors.Is(err, service.ErrInvitationToken),
		errors.Is(err, service.ErrInvitationExpired),
		errors.Is(err, service.ErrInvitationRevoked),
		errors.Is(err, service.ErrInvitationAccepted),
		errors.Is(err, service.ErrInvitationPending),
		errors.Is(err, service.ErrInvitationNotPending),
		errors.Is(err, service.ErrApplicationMissing),
		errors.Is(err, service.ErrApplicationNotInvitable),
		errors.Is(err, service.ErrTemplateNotInvitation),
		errors.Is(err, service.ErrNoEmail),
		errors.Is(err, service.ErrCandidateMissing),
		errors.Is(err, service.ErrCandidateTransitionInvalid),
		errors.Is(err, service.ErrActionTemplateRequired),
		errors.Is(err, repository.ErrInvitationConflict),
		errors.Is(err, repository.ErrCandidateConflict):
		response.FailInvalidParam(c, err.Error())
	default:
		failInternal(c, err)
	}
}

// parsePage / parsePageSize 分页参数兜底，与邮件记录列表一致。
func parsePage(c *gin.Context) int {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func parsePageSize(c *gin.Context) int {
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || pageSize < 1 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}
