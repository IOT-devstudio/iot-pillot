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

// FormHandler 表单模块（#92 / BE-05）。
//
// 读接口（列表 / 详情）对 admin 与 member 都开放，写接口在路由上逐条挂
// requireAdmin；分享链接的读取与提交都要求登录 —— share_token 只是定位符，
// 不构成鉴权绕过（#92 裁决：不存在匿名提交路径）。
type FormHandler struct {
	formService *service.FormUseCase
}

// NewFormHandler 构造函数。
func NewFormHandler(formService *service.FormUseCase) *FormHandler {
	return &FormHandler{formService: formService}
}

// failForm 把表单域错误映射成 HTTP 响应。
//
// 用 errors.Is 而不是 switch err == ：表单的配置/答案错误是**带上下文的包装错误**
// （"必填项「姓名」未填写"），必须按哨兵值识别，同时把可读原因原样给到调用方。
// 未找到类错误一律 400 而不是 404，与招新模块的既有约定一致。
func failForm(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrFormNotFound):
		response.FailInvalidParam(c, "表单不存在或分享链接无效")
	case errors.Is(err, repository.ErrFormFieldNotFound):
		response.FailInvalidParam(c, "表单字段不存在或不属于该表单")
	case errors.Is(err, repository.ErrSubmissionNotFound):
		response.FailInvalidParam(c, "提交记录不存在")
	case errors.Is(err, repository.ErrDuplicateSubmission):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrDuplicateApplication):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, repository.ErrRoundNotFound):
		response.FailInvalidParam(c, "招新轮次不存在")
	case errors.Is(err, repository.ErrDirectionNotFound):
		response.FailInvalidParam(c, "招新方向不存在")
	case errors.Is(err, service.ErrFormClosed):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrFormConfig):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrAnswerInvalid):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrRoundClosed):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrDirectionClosed):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrNoActiveRound):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, service.ErrApplicationFinal):
		// 申请已到终态（accepted / rejected）不能补交面试材料（#135 §3.3 / D3）。
		// 沿用"业务错误一律 400"的口径，不新增业务码 —— 前端对终态的置灰是纯前端
		// 判断，正常路径打不到这里，这条只是服务端兜底，调用方不需要区分错误码。
		response.FailInvalidParam(c, err.Error())
	default:
		failInternal(c, err)
	}
}

// parseFormID 解析路径里的表单 id。
func parseFormID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.FailInvalidParam(c, "id 必须是正整数")
		return 0, false
	}
	return id, true
}

/* ------------------------------------------------------------------ *
 * 管理端：表单定义
 * ------------------------------------------------------------------ */

// ListForms 表单列表。
// @Summary 表单列表
// @Description 分页列出全部表单，含开放状态与提交数。管理员与成员都可读。
// @Tags 表单模块
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.FormListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/forms [get]
func (h *FormHandler) ListForms(c *gin.Context) {
	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	forms, counts, total, err := h.formService.ListForms(c.Request.Context(), page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}

	items := make([]response.FormResp, 0, len(forms))
	for _, f := range forms {
		items = append(items, response.NewFormResp(f, counts[f.ID]))
	}

	response.OK(c, response.FormListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetFormDetail 表单详情。
// @Summary 表单详情
// @Description 取表单定义、字段列表与提交数。管理员与成员都可读。
// @Tags 表单模块
// @Produce json
// @Param id path int true "表单 ID"
// @Success 200 {object} response.Result{data=response.FormDetailResp} "成功"
// @Failure 400 {object} response.Result "表单不存在"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/forms/{id} [get]
func (h *FormHandler) GetFormDetail(c *gin.Context) {
	id, ok := parseFormID(c)
	if !ok {
		return
	}

	form, fields, count, err := h.formService.GetFormDetail(c.Request.Context(), id)
	if err != nil {
		failForm(c, err)
		return
	}

	response.OK(c, response.FormDetailResp{
		Form:   response.NewFormResp(form, count),
		Fields: response.NewFormFieldResps(fields),
	})
}

// CreateForm 创建表单。
// @Summary 创建表单
// @Description 管理员建表：绑定一轮招新与一个方向，生成分享令牌；新建默认关闭。
// @Tags 管理端-表单
// @Accept json
// @Produce json
// @Param request body request.FormCreateReq true "建表请求"
// @Success 200 {object} response.Result{data=response.FormDetailResp} "成功"
// @Failure 400 {object} response.Result "配置不合法或轮次/方向不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/forms [post]
func (h *FormHandler) CreateForm(c *gin.Context) {
	var req request.FormCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	form, fields, err := h.formService.CreateForm(c.Request.Context(), service.CreateFormInput{
		Title:         req.Title,
		Description:   req.Description,
		RoundID:       req.RoundID,
		DirectionCode: req.DirectionCode,
		Fields:        toFieldInputs(req.Fields),
	})
	if err != nil {
		failForm(c, err)
		return
	}

	response.OKWithMsg(c, "表单已创建", response.FormDetailResp{
		Form:   response.NewFormResp(form, 0),
		Fields: response.NewFormFieldResps(fields),
	})
}

// UpdateForm 修改表单。
// @Summary 修改表单
// @Description 管理员改表：标题 / 说明 / 开放状态；带 fields 时整份替换字段列表。
// @Description 轮次与方向绑定不可修改（要换请新建表单）。
// @Tags 管理端-表单
// @Accept json
// @Produce json
// @Param id path int true "表单 ID"
// @Param request body request.FormUpdateReq true "修改请求"
// @Success 200 {object} response.Result{data=response.FormDetailResp} "成功"
// @Failure 400 {object} response.Result "配置不合法或表单不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/forms/{id} [patch]
func (h *FormHandler) UpdateForm(c *gin.Context) {
	id, ok := parseFormID(c)
	if !ok {
		return
	}

	var req request.FormUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	in := service.UpdateFormInput{
		Title:       req.Title,
		Description: req.Description,
		IsOpen:      req.IsOpen,
	}
	if req.Fields != nil {
		fields := toFieldInputs(*req.Fields)
		in.Fields = &fields
	}

	form, fields, count, err := h.formService.UpdateForm(c.Request.Context(), id, in)
	if err != nil {
		failForm(c, err)
		return
	}

	response.OKWithMsg(c, "表单已更新", response.FormDetailResp{
		Form:   response.NewFormResp(form, count),
		Fields: response.NewFormFieldResps(fields),
	})
}

/* ------------------------------------------------------------------ *
 * 用户端：分享链接读取与提交
 * ------------------------------------------------------------------ */

// ReadShareForm 按分享令牌读取表单定义。
// @Summary 读取分享表单
// @Description 登录用户按分享令牌读取表单定义与字段；关闭的表单照样返回定义（is_open=false），
// @Description 并返回当前用户在该表单上的提交情况，供填写页提示"已提交"。
// @Tags 表单模块
// @Produce json
// @Param shareToken path string true "分享令牌"
// @Success 200 {object} response.Result{data=response.ShareFormResp} "成功"
// @Failure 400 {object} response.Result "分享链接无效"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/forms/public/{shareToken} [get]
func (h *FormHandler) ReadShareForm(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	form, fields, mySubmission, err := h.formService.ReadShareForm(c.Request.Context(), c.Param("shareToken"), userID)
	if err != nil {
		failForm(c, err)
		return
	}

	resp := response.ShareFormResp{
		Form:   response.NewFormResp(form, 0),
		Fields: response.NewFormFieldResps(fields),
	}
	if mySubmission != nil {
		resp.Submitted = true
		resp.MySubmissionID = mySubmission.ID
	}

	response.OK(c, resp)
}

// SubmitForm 提交表单。
// @Summary 提交表单
// @Description 登录用户经分享链接提交：答案由服务端校验（必填 / 类型 / select 选项合法性），
// @Description 提交人取自令牌；提交会按统一申请模型创建申请，不产生重复申请记录。
// @Tags 表单模块
// @Accept json
// @Produce json
// @Param shareToken path string true "分享令牌"
// @Param request body request.FormSubmitReq true "提交内容"
// @Success 200 {object} response.Result{data=response.SubmitFormResp} "成功"
// @Failure 400 {object} response.Result "表单关闭 / 校验失败 / 重复提交"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/forms/public/{shareToken}/submit [post]
func (h *FormHandler) SubmitForm(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "未认证")
		return
	}

	var req request.FormSubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	submission, application, err := h.formService.SubmitByShareToken(
		c.Request.Context(), c.Param("shareToken"), userID, req.Values,
	)
	if err != nil {
		failForm(c, err)
		return
	}

	response.OKWithMsg(c, "提交成功", response.SubmitFormResp{
		Submission:        response.NewFormSubmissionResp(submission),
		ApplicationID:     application.ID,
		ApplicationStatus: string(application.Status),
	})
}

/* ------------------------------------------------------------------ *
 * 管理端：提交记录
 * ------------------------------------------------------------------ */

// ListFormSubmissions 表单提交列表。
// @Summary 表单提交列表
// @Description 分页列出某表单的提交记录（新提交在前）。
// @Tags 管理端-表单
// @Produce json
// @Param id path int true "表单 ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数，最大 100" default(20)
// @Success 200 {object} response.Result{data=response.FormSubmissionListResp} "成功"
// @Failure 400 {object} response.Result "表单不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/forms/{id}/submissions [get]
func (h *FormHandler) ListFormSubmissions(c *gin.Context) {
	id, ok := parseFormID(c)
	if !ok {
		return
	}

	page := parseBoundedInt(c.Query("page"), 1, 1, 100000)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	submissions, total, err := h.formService.ListSubmissions(c.Request.Context(), id, page, pageSize)
	if err != nil {
		failForm(c, err)
		return
	}

	items := make([]response.FormSubmissionResp, 0, len(submissions))
	for _, s := range submissions {
		items = append(items, response.NewFormSubmissionResp(s))
	}

	response.OK(c, response.FormSubmissionListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetFormSubmission 提交详情。
// @Summary 提交详情
// @Description 取某表单下一条提交记录（字段值 + 关联申请 id）。
// @Tags 管理端-表单
// @Produce json
// @Param id path int true "表单 ID"
// @Param submission_id path int true "提交 ID"
// @Success 200 {object} response.Result{data=response.FormSubmissionResp} "成功"
// @Failure 400 {object} response.Result "表单或提交记录不存在"
// @Failure 401 {object} response.Result "未认证"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/forms/{id}/submissions/{submission_id} [get]
func (h *FormHandler) GetFormSubmission(c *gin.Context) {
	id, ok := parseFormID(c)
	if !ok {
		return
	}

	submissionID, err := strconv.Atoi(c.Param("submission_id"))
	if err != nil || submissionID <= 0 {
		response.FailInvalidParam(c, "submission_id 必须是正整数")
		return
	}

	submission, err := h.formService.GetSubmission(c.Request.Context(), id, submissionID)
	if err != nil {
		failForm(c, err)
		return
	}

	response.OK(c, response.NewFormSubmissionResp(submission))
}

// toFieldInputs 请求 DTO → 服务入参。
func toFieldInputs(fields []request.FormFieldReq) []service.FormFieldInput {
	out := make([]service.FormFieldInput, 0, len(fields))
	for _, f := range fields {
		out = append(out, service.FormFieldInput{
			ID:          f.ID,
			Type:        f.Type,
			Label:       f.Label,
			Required:    f.Required,
			Placeholder: f.Placeholder,
			Options:     f.Options,
		})
	}
	return out
}
