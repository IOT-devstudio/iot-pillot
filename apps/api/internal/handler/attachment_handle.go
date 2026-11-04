package handler

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"

	"github.com/gin-gonic/gin"
)

// attachmentSingleRequestLimit 应用层单请求上限（12MiB），与 service 同名常量一致。
// 必须大于单文件上限（10MiB）且小于 nginx client_max_body_size 12m。
const attachmentSingleRequestLimit = 12 << 20

// AttachmentHandler 内容附件（#216 上传 / #217 读取）。
//
// 两端点都只挂 authRequired：上传不分 scope、无 requireAdmin（任何登录用户都能发图，
// 管理员身份不赋予上传特权）；读取的归属判定在 service，handler 只解析 id 并一律 404
// （防枚举，详见 ADR 0004）。
type AttachmentHandler struct {
	uc *service.AttachmentUseCase
}

// NewAttachmentHandler 构造函数。
func NewAttachmentHandler(uc *service.AttachmentUseCase) *AttachmentHandler {
	return &AttachmentHandler{uc: uc}
}

// failAttachment 把附件域错误映射成 HTTP 响应。
//
// 业务错误一律 400 + 可读中文（与知识库/成员名录模块约定一致）；
// 配额超限走 429；请求体过大走 400 + 专属文案（不让 nginx 裸 413 HTML 漏给前端）；
// 其余交 failInternal。
func failAttachment(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrAttachmentInvalid):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, domain.ErrAttachmentTooLarge):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, domain.ErrAttachmentUnsupportedType):
		response.FailInvalidParam(c, err.Error())
	case errors.Is(err, domain.ErrAttachmentQuotaExceeded):
		// 配额超限：语义是「稍后再试」，与登录限流同一档 429。
		response.FailTooManyRequests(c, err.Error())
	case errors.Is(err, domain.ErrRequestTooLarge):
		// 请求体过大：统一业务错误，不让 nginx 甩裸 413 HTML。
		response.FailInvalidParam(c, err.Error())
	// 读取端点：NotFound / Forbidden 都压成 404（防枚举，见 ADR 0004 §1）。
	case errors.Is(err, domain.ErrAttachmentNotFound):
		response.FailNotFound(c, "附件不存在")
	case errors.Is(err, domain.ErrAttachmentForbidden):
		response.FailNotFound(c, "附件不存在")
	default:
		failInternal(c, err)
	}
}

// UploadAttachment 上传一张图片（#216）。
// @Summary 上传内容附件（图片）
// @Description 登录用户上传一张 jpg/png/webp 图片。请求体为 multipart/form-data，单字段文件。返回 {id,url,content_type,size_bytes}。读取与校验全部在流式下进行：不落临时文件、不整块进内存。
// @Tags 内容附件
// @Accept multipart/form-data
// @Produce json
// @Success 200 {object} response.Result{data=response.AttachmentUploadResp} "上传成功"
// @Failure 400 {object} response.Result "参数错误（非图片 / 单张过大 / 请求体过大）"
// @Failure 401 {object} response.Result "未认证"
// @Failure 429 {object} response.Result "上传次数达上限"
// @Security BearerAuth
// @Router /api/v1/attachments [post]
func (h *AttachmentHandler) UploadAttachment(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}

	// 单请求闸门：在读 multipart 之前给请求体套上限（12MiB+1）。
	// 超过这一层由 MaxBytesReader 在读取时直接截断，不会把大请求整块读进内存。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(attachmentSingleRequestLimit)+1)

	reader, err := c.Request.MultipartReader()
	if err != nil {
		// 不是合法的 multipart 请求体 → 参数错误。
		response.FailInvalidParam(c, "请求格式应为 multipart/form-data")
		return
	}

	part, err := reader.NextPart()
	if err != nil {
		if errors.Is(err, io.EOF) {
			response.FailInvalidParam(c, "缺少上传文件")
			return
		}
		if isRequestTooLarge(err) {
			failAttachment(c, domain.ErrRequestTooLarge)
			return
		}
		failInternal(c, err)
		return
	}
	defer part.Close()

	// 用户文件名只进日志，永不进对象键、不持久化。
	fileName := part.FileName()
	if fileName != "" {
		log.Printf("[attachment] 上传文件 username-less user_id=%d filename=%q", userID, fileName)
	}

	att, err := h.uc.UploadAttachment(c.Request.Context(), service.UploadAttachmentInput{
		UploaderID: userID,
		Part:       part,
		FileName:   fileName,
	})
	if err != nil {
		failAttachment(c, err)
		return
	}

	response.OK(c, response.NewAttachmentUploadResp(att))
}

// GetAttachment 读取一张图（#217）。
// @Summary 读取内容附件（图片）
// @Description 登录用户按 id 读取图片；归属判定在 service（孤儿仅上传者 / studio 需管理员 / 路线与招新问答登录即可），无权或不存在一律 404 防枚举。响应经 Go 代理转发字节流并带 Cache-Control: private, max-age=31536000, immutable（见 ADR 0004 §8）。
// @Tags 内容附件
// @Produce image/jpeg,image/png,image/webp
// @Success 200 {file} file "图片字节流"
// @Failure 401 {object} response.Result "未认证"
// @Failure 404 {object} response.Result "附件不存在或无权限"
// @Security BearerAuth
// @Router /api/v1/attachments/{id} [get]
func (h *AttachmentHandler) GetAttachment(c *gin.Context) {
	userID, _, _, ok := middleware.CurrentUser(c)
	if !ok {
		response.FailUnauthorized(c, "请先登录")
		return
	}

	// 解析 id：非法 id（非数字 / <=0）按 404，不暴露「解析失败」与「不存在」的差异。
	id, ok := parseAttachmentID(c.Param("id"))
	if !ok {
		response.FailNotFound(c, "附件不存在")
		return
	}

	// 管理员身份来自鉴权链写入的 role（中间件只认 Redis 名单，撤销管理员立即生效）。
	isAdmin := middleware.RoleAdmin == roleOf(c)

	att, err := h.uc.GetAttachment(c.Request.Context(), userID, isAdmin, id)
	if err != nil {
		failAttachment(c, err)
		return
	}

	rc, contentType, size, err := h.uc.ReadObject(c.Request.Context(), att.ObjectKey)
	if err != nil {
		failInternal(c, err)
		return
	}
	defer rc.Close()

	// ADR 0004 §8：图片内容不可变，长期私有缓存。
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.DataFromReader(http.StatusOK, size, contentType, rc, nil)
}

// parseAttachmentID 解析路由里的附件 id：非正整数一律返回 (0, false)。
func parseAttachmentID(raw string) (int, bool) {
	id := 0
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch < '0' || ch > '9' {
			return 0, false
		}
		id = id*10 + int(ch-'0')
		if id > 1<<30 {
			return 0, false
		}
	}
	if id <= 0 {
		return 0, false
	}
	return id, true
}

// roleOf 读出 AuthRequired 写入的角色（"admin" / "member"）。
func roleOf(c *gin.Context) string {
	if v, ok := c.Get(middleware.ContextRole); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// isRequestTooLarge 判断底层 reader 是否因超过 MaxBytesReader 上限而失败。
func isRequestTooLarge(err error) bool {
	if err == nil {
		return false
	}
	// net/http 在超过 MaxBytesReader 上限时返回此哨兵（Go 1.22+）。
	return containsSubstr(err.Error(), "http: request body too large")
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
