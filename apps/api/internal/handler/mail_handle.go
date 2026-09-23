package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// MailHandler 邮件模板与发信接口（全部要求管理员）。
//
// 发信人（FromUserID）取自 AuthRequired 写入的当前用户，不接受请求体传入：
// 否则操作者可以伪造成别人发的信，而发信记录是要用于追责的。
type MailHandler struct {
	mailService *service.MailUseCase
}

func NewMailHandler(mailService *service.MailUseCase) *MailHandler {
	return &MailHandler{mailService: mailService}
}

// ListMailTemplates 模板列表
// @Summary 邮件模板列表
// @Description 列出全部邮件模板；每个模板附带它用到的变量名，便于管理端提示。
// @Tags 管理端-邮件
// @Produce json
// @Success 200 {object} response.Result{data=response.MailModelListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/mail-templates [get]
func (h *MailHandler) ListMailTemplates(c *gin.Context) {
	result, err := h.mailService.ListTemplates(c.Request.Context())
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

// CreateMailTemplate 新建模板
// @Summary 新建邮件模板
// @Description 主题与正文都支持 {{变量}} 占位符；括号不成对会被拒绝（那是会静默发出错信的错误）。
// @Tags 管理端-邮件
// @Accept json
// @Produce json
// @Param request body request.MailTemplateReq true "模板内容"
// @Success 200 {object} response.Result{data=response.MailModelResp} "成功"
// @Failure 400 {object} response.Result "参数错误或模板名/类型重复"
// @Security BearerAuth
// @Router /api/v1/admin/mail-templates [post]
func (h *MailHandler) CreateMailTemplate(c *gin.Context) {
	var req request.MailTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.mailService.CreateTemplate(c.Request.Context(), &req)
	if err != nil {
		failMailRequest(c, err)
		return
	}
	response.OK(c, result)
}

// UpdateMailTemplate 更新模板
// @Summary 更新邮件模板
// @Tags 管理端-邮件
// @Accept json
// @Produce json
// @Param id path int true "模板 ID"
// @Param request body request.MailTemplateReq true "模板内容"
// @Success 200 {object} response.Result{data=response.MailModelResp} "成功"
// @Failure 400 {object} response.Result "参数错误、模板不存在或名称重复"
// @Security BearerAuth
// @Router /api/v1/admin/mail-templates/{id} [put]
func (h *MailHandler) UpdateMailTemplate(c *gin.Context) {
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	var req request.MailTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.mailService.UpdateTemplate(c.Request.Context(), id, &req)
	if err != nil {
		failMailRequest(c, err)
		return
	}
	response.OK(c, result)
}

// DeleteMailTemplate 删除模板
// @Summary 删除邮件模板
// @Description 只删模板，不动已发出的邮件记录——记录是审计凭据，不该因为模板被删就消失。
// @Tags 管理端-邮件
// @Produce json
// @Param id path int true "模板 ID"
// @Success 200 {object} response.Result "成功"
// @Failure 400 {object} response.Result "模板不存在"
// @Security BearerAuth
// @Router /api/v1/admin/mail-templates/{id} [delete]
func (h *MailHandler) DeleteMailTemplate(c *gin.Context) {
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	if err := h.mailService.DeleteTemplate(c.Request.Context(), id); err != nil {
		failMailRequest(c, err)
		return
	}
	response.OKWithMsg(c, "模板已删除", nil)
}

// SendMailToUser 按用户发送
// @Summary 按 userID 发送邮件
// @Description 变量不传也行：服务端会用收件人资料（姓名/邮箱/班级/方向/学号/轮次）预填，请求里的同键覆盖。
// @Tags 管理端-邮件
// @Accept json
// @Produce json
// @Param request body request.SendMailToUserReq true "发送参数"
// @Success 200 {object} response.Result{data=response.MailSendResp} "成功"
// @Failure 400 {object} response.Result "参数错误、模板不存在、缺少变量或用户没有邮箱"
// @Security BearerAuth
// @Router /api/v1/admin/mails/send [post]
func (h *MailHandler) SendMailToUser(c *gin.Context) {
	var req request.SendMailToUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	operatorID := currentUserID(c)
	result, err := h.mailService.SendToUser(c.Request.Context(), operatorID, &req)
	if err != nil {
		failMailRequest(c, err)
		return
	}
	response.OK(c, result)
}

// SendMailToEmail 按邮箱发送
// @Summary 按邮箱地址发送邮件
// @Description 用于发给还没注册的报名者；邮箱能对上已注册用户时会按用户记录。
// @Tags 管理端-邮件
// @Accept json
// @Produce json
// @Param request body request.SendMailToEmailReq true "发送参数"
// @Success 200 {object} response.Result{data=response.MailSendResp} "成功"
// @Failure 400 {object} response.Result "参数错误、模板不存在或缺少变量"
// @Security BearerAuth
// @Router /api/v1/admin/mails/send-by-email [post]
func (h *MailHandler) SendMailToEmail(c *gin.Context) {
	var req request.SendMailToEmailReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	operatorID := currentUserID(c)
	result, err := h.mailService.SendToEmail(c.Request.Context(), operatorID, &req)
	if err != nil {
		failMailRequest(c, err)
		return
	}
	response.OK(c, result)
}

// SendMailBulk 群发
// @Summary 群发邮件（同一模板，逐个替换内容）
// @Description 逐条独立投递，单个失败不影响其他收件人；响应里分别给出成功与失败清单。
// @Tags 管理端-邮件
// @Accept json
// @Produce json
// @Param request body request.SendMailBulkReq true "群发参数"
// @Success 200 {object} response.Result{data=response.MailBulkSendResp} "成功（含逐条结果）"
// @Failure 400 {object} response.Result "参数错误、模板不存在或收件人超过上限"
// @Security BearerAuth
// @Router /api/v1/admin/mails/send-bulk [post]
func (h *MailHandler) SendMailBulk(c *gin.Context) {
	var req request.SendMailBulkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	operatorID := currentUserID(c)
	result, err := h.mailService.SendBulk(c.Request.Context(), operatorID, &req)
	if err != nil {
		failMailRequest(c, err)
		return
	}
	response.OK(c, result)
}

// ListMails 发信记录
// @Summary 发信记录
// @Description 按发送时间倒序分页。可选 keyword（主题/收件人模糊匹配）与 from/to（日期，含端点）过滤；total 为过滤后的总数。只返回记录元信息，不返回正文。
// @Tags 管理端-邮件
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Param keyword query string false "模糊搜索主题与收件人邮箱，最长 100"
// @Param from query string false "起始日期 YYYY-MM-DD（含当天）"
// @Param to query string false "结束日期 YYYY-MM-DD（含当天）"
// @Success 200 {object} response.Result{data=response.MailRecordListResp} "成功"
// @Failure 400 {object} response.Result "日期格式错误"
// @Security BearerAuth
// @Router /api/v1/admin/mails [get]
func (h *MailHandler) ListMails(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 100 {
		// 超长直接截断而不是报错：搜索框敲快了带进来的内容不该换来一个 400
		keyword = keyword[:100]
	}

	var filter repository.MailListFilter
	filter.Keyword = keyword
	// 日期按「天」粒度收口：from 取当天零点，to 放到次日零点用 < 排除，
	// 避免 "2026-09-23" 这种日期选择器值把当天中午的记录挡在外面
	if raw := c.Query("from"); raw != "" {
		t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			response.FailInvalidParam(c, "from 日期格式应为 YYYY-MM-DD")
			return
		}
		filter.From = t
	}
	if raw := c.Query("to"); raw != "" {
		t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			response.FailInvalidParam(c, "to 日期格式应为 YYYY-MM-DD")
			return
		}
		filter.To = t.AddDate(0, 0, 1)
		// repo 用 <=，这里推到次日零点后减一纳秒 = 当天 23:59:59.999...
		filter.To = filter.To.Add(-time.Nanosecond)
	}

	result, err := h.mailService.ListMails(c.Request.Context(), page, pageSize, filter)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

// DeleteMails 批量删除发信记录
// @Summary 批量删除发信记录
// @Description 管理员清理记录用；返回实际删除的行数，ids 中已不存在的 id 会被跳过。
// @Tags 管理端-邮件
// @Accept json
// @Produce json
// @Param request body request.MailBatchDeleteReq true "待删除的记录 id 列表"
// @Success 200 {object} response.Result{data=response.MailBatchDeleteResp} "成功"
// @Failure 400 {object} response.Result "参数错误"
// @Security BearerAuth
// @Router /api/v1/admin/mails/batch-delete [delete]
func (h *MailHandler) DeleteMails(c *gin.Context) {
	var req request.MailBatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	result, err := h.mailService.DeleteMailRecords(c.Request.Context(), req.IDs)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, result)
}

/* ------------------------------------------------------------------ *
 * 辅助
 * ------------------------------------------------------------------ */

func currentUserID(c *gin.Context) int {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		return 0
	}
	return userID
}

func parsePositiveParam(c *gin.Context, name string) (int, error) {
	value, err := strconv.Atoi(c.Param(name))
	if err != nil || value <= 0 {
		return 0, errors.New(name + " 必须是正整数")
	}
	return value, nil
}

// failMailRequest 把服务层错误映射成合适的状态码。
//
// 区分两类：调用方输入问题（模板名重复、模板/收件人不存在、缺变量、超上限）→ 400，
// 服务端依赖故障（SMTP 投递失败）→ 500。全塞成 400 会让管理端把"SMTP 挂了"
// 当成自己参数写错了，从而反复重试同样的请求。
func failMailRequest(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrDuplicateMailModel):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrMailDelivery):
		failInternal(c, err)
	default:
		response.FailInvalidParam(c, err.Error())
	}
}
