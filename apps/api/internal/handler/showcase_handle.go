package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// showcaseRequestLimit 作品提交单请求上限。
//
// 比图片上传大得多：要装下一个 50MiB 的源码包外加表单字段。
// 多留 1MiB 给 multipart 边界与文本字段，避免恰好卡在边界上被截断。
const showcaseRequestLimit = domain.ShowcaseArchiveMaxBytes + (1 << 20)

// ShowcaseHandler 作品提交。
type ShowcaseHandler struct {
	uc *service.ShowcaseUseCase
	// admins 判断调用方是否管理员（决定能否看全量与审阅）。
	admins middleware.AdminChecker
}

// NewShowcaseHandler 构造函数。
func NewShowcaseHandler(uc *service.ShowcaseUseCase, admins middleware.AdminChecker) *ShowcaseHandler {
	return &ShowcaseHandler{uc: uc, admins: admins}
}

// failShowcase 把作品域错误映射成 HTTP 响应。
//
// 404 而不是 403：作品不存在与不是本人的作品回同一个响应，
// 否则可以用 403 探测出别人作品的存在性（与附件同口径）。
func failShowcase(c *gin.Context, err error) {
	var validation *domain.ShowcaseValidationError
	switch {
	case errors.As(err, &validation):
		response.FailInvalidParam(c, validation.Reason)
	case errors.Is(err, domain.ErrShowcaseNotFound):
		response.FailNotFound(c, domain.ErrShowcaseNotFound.Error())
	case errors.Is(err, domain.ErrShowcaseArchiveUnsupportedType),
		errors.Is(err, domain.ErrShowcaseArchiveTooLarge):
		response.FailInvalidParam(c, err.Error())
	default:
		failInternal(c, err)
	}
}

// showcaseResp 一份作品的响应形态。
//
// 刻意不返回 archive_object_key：对象键是内部实现细节，
// 前端要走带鉴权的下载端点，而不是直接拼存储地址。
type showcaseResp struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	RoadmapID     *int64 `json:"roadmap_id"`
	RoadmapTitle  string `json:"roadmap_title"`
	RepoURL       string `json:"repo_url"`
	HasArchive    bool   `json:"has_archive"`
	ArchiveName   string `json:"archive_name"`
	ArchiveSize   int64  `json:"archive_size"`
	Status        string `json:"status"`
	ReviewNote    string `json:"review_note"`
	ReviewedAt    string `json:"reviewed_at"`
	CreatedAt     string `json:"created_at"`
	// IsMine 管理端列表里用来区分「自己的」与他人作品。
	IsMine bool `json:"is_mine,omitempty"`
}

func newShowcaseResp(s *domain.ShowcaseSubmission) showcaseResp {
	reviewedAt := ""
	if s.ReviewedAt != nil {
		reviewedAt = s.ReviewedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return showcaseResp{
		ID:           s.ID,
		Title:        s.Title,
		Summary:      s.Summary,
		RoadmapID:    s.RoadmapID,
		RoadmapTitle: s.RoadmapTitle,
		RepoURL:      s.RepoURL,
		HasArchive:   s.ArchiveObjectKey != "",
		ArchiveName:  s.ArchiveName,
		ArchiveSize:  s.ArchiveSize,
		Status:       s.Status,
		ReviewNote:   s.ReviewNote,
		ReviewedAt:   reviewedAt,
		CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// showcaseListResp 分页结果。
type showcaseListResp struct {
	Items    []showcaseResp `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// CreateShowcase 提交作品。
// @Summary 提交作品
// @Description 登录用户提交作品。请求体为 multipart/form-data：
// repo_url 与 archive 至少要有一个；archive 只接受 .zip，最大 50MB。
// @Tags 作品提交
// @Accept multipart/form-data
// @Produce json
// @Success 200 {object} response.Result{data=showcaseResp} "提交成功"
// @Failure 400 {object} response.Result "内容不合法（无载体 / 非 zip / 包过大）"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/me/showcase [post]
func (h *ShowcaseHandler) CreateShowcase(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}

	// 在读 multipart 之前套上限：MaxBytesReader 会在读取时截断，不会整块进内存。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(showcaseRequestLimit))

	// 32MiB 内存缓冲，超出部分落临时文件（Go 标准库行为），
	// 这样大 zip 不会把进程内存顶穿。
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		if isRequestTooLarge(err) {
			response.FailInvalidParam(c, "源码包过大，请压缩后再试")
			return
		}
		response.FailInvalidParam(c, "请求格式应为 multipart/form-data")
		return
	}

	in := service.CreateShowcaseInput{
		Title:   c.PostForm("title"),
		Summary: c.PostForm("summary"),
		RepoURL: c.PostForm("repo_url"),
	}

	// roadmap_id 可选：不带就是与路线无关的自由作品。
	if raw := strings.TrimSpace(c.PostForm("roadmap_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.FailInvalidParam(c, "route 参数不合法")
			return
		}
		in.RoadmapID = &id
	}

	// 源码包可选：只有真的选了文件才读，避免空表单也去碰存储。
	if file, header, err := c.Request.FormFile("archive"); err == nil {
		defer file.Close()
		data, readErr := io.ReadAll(io.LimitReader(file, domain.ShowcaseArchiveMaxBytes+1))
		if readErr != nil {
			failInternal(c, readErr)
			return
		}
		if int64(len(data)) > domain.ShowcaseArchiveMaxBytes {
			response.FailInvalidParam(c, domain.ErrShowcaseArchiveTooLarge.Error())
			return
		}
		in.ArchiveName = header.Filename
		in.ArchiveType = header.Header.Get("Content-Type")
		in.ArchiveData = data
	}

	submission, err := h.uc.Create(c.Request.Context(), int64(userID), in)
	if err != nil {
		failShowcase(c, err)
		return
	}
	response.OKWithMsg(c, "作品已提交", newShowcaseResp(submission))
}

// ListMyShowcase 本人的作品列表。
// @Summary 我的作品
// @Tags 作品提交
// @Produce json
// @Success 200 {object} response.Result{data=showcaseListResp} "成功"
// @Failure 401 {object} response.Result "未认证"
// @Security BearerAuth
// @Router /api/v1/me/showcase [get]
func (h *ShowcaseHandler) ListMyShowcase(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}
	page := parseBoundedInt(c.Query("page"), 1, 1, 1<<20)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)

	rows, total, err := h.uc.ListByUser(c.Request.Context(), int64(userID), page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}
	items := make([]showcaseResp, 0, len(rows))
	for _, s := range rows {
		items = append(items, newShowcaseResp(s))
	}
	response.OK(c, showcaseListResp{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// DeleteShowcase 删除本人的作品。
// @Summary 删除作品
// @Description 只能删自己的；不是本人的按不存在处理（404 防枚举）。
// @Tags 作品提交
// @Produce json
// @Success 200 {object} response.Result "删除成功"
// @Failure 404 {object} response.Result "不存在或不属于本人"
// @Security BearerAuth
// @Router /api/v1/me/showcase/{id} [delete]
func (h *ShowcaseHandler) DeleteShowcase(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	if err := h.uc.Delete(c.Request.Context(), int64(id), int64(userID)); err != nil {
		failShowcase(c, err)
		return
	}
	response.OKWithMsg(c, "已删除", gin.H{"deleted": true})
}

// DownloadShowcaseArchive 下载源码包。
// @Summary 下载源码包
// @Description 本人或管理员可下载；不带对象键给前端，统一从这里取。
// @Tags 作品提交
// @Success 200 {file} file "源码包字节流"
// @Failure 404 {object} response.Result "不存在、不属于本人或没有源码包"
// @Security BearerAuth
// @Router /api/v1/me/showcase/{id}/archive [get]
func (h *ShowcaseHandler) DownloadShowcaseArchive(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}

	isAdmin, adminErr := h.admins.IsAdmin(c.Request.Context(), userID)
	if adminErr != nil {
		// 读不到管理员名单时按「不是管理员」处理：下载权限宁窄勿宽。
		isAdmin = false
	}

	submission, rc, size, err := h.uc.OpenArchive(c.Request.Context(), int64(id), int64(userID), isAdmin)
	if err != nil {
		failShowcase(c, err)
		return
	}
	defer rc.Close()

	// 文件名来自用户输入：走附件下载头时由 Go 做转义，不进 Content-Type。
	name := submission.ArchiveName
	if name == "" {
		name = fmt.Sprintf("showcase-%d.zip", submission.ID)
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	// 私有内容：允许本人浏览器缓存，但禁止共享缓存。
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// ListAllShowcase 管理端全量作品列表。
// @Summary 管理端作品列表
// @Description 待审阅的排在前面；可按 status 过滤。
// @Tags 作品提交
// @Produce json
// @Success 200 {object} response.Result{data=showcaseListResp} "成功"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/showcase [get]
func (h *ShowcaseHandler) ListAllShowcase(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}
	page := parseBoundedInt(c.Query("page"), 1, 1, 1<<20)
	pageSize := parseBoundedInt(c.Query("page_size"), 20, 1, 100)
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && !domain.ShowcaseStatusKnown(status) {
		response.FailInvalidParam(c, "状态参数不合法")
		return
	}

	rows, total, err := h.uc.ListAll(c.Request.Context(), status, page, pageSize)
	if err != nil {
		failInternal(c, err)
		return
	}
	items := make([]showcaseResp, 0, len(rows))
	for _, s := range rows {
		item := newShowcaseResp(s)
		item.IsMine = s.UserID == int64(userID)
		items = append(items, item)
	}
	response.OK(c, showcaseListResp{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// reviewShowcaseReq 审核入参。
type reviewShowcaseReq struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

// ReviewShowcase 审阅作品。
// @Summary 审阅作品
// @Description status 只能是 approved / rejected；驳回必须带理由。
// @Tags 作品提交
// @Accept json
// @Produce json
// @Success 200 {object} response.Result "已更新"
// @Failure 400 {object} response.Result "状态不合法或驳回未填理由"
// @Failure 403 {object} response.Result "非管理员"
// @Security BearerAuth
// @Router /api/v1/admin/showcase/{id}/review [patch]
func (h *ShowcaseHandler) ReviewShowcase(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}
	id, err := parsePositiveParam(c, "id")
	if err != nil {
		response.FailInvalidParam(c, err.Error())
		return
	}
	var req reviewShowcaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailInvalidParam(c, "请求内容格式不正确")
		return
	}
	if err := h.uc.Review(c.Request.Context(), int64(id), int64(userID), req.Status, req.Note); err != nil {
		failShowcase(c, err)
		return
	}
	response.OKWithMsg(c, "已更新审核状态", gin.H{"id": id, "status": req.Status})
}
