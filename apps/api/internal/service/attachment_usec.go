package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/storage"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * 对外错误（直接复用领域层哨兵，避免 errors.Is 漏判）
 * ------------------------------------------------------------------ */

var (
	// ErrAttachmentTooLarge 单张图片超过 10MiB（领域哨兵透传）。
	ErrAttachmentTooLarge = domain.ErrAttachmentTooLarge
	// ErrAttachmentUnsupportedType 不是 jpg / png / webp（领域哨兵透传）。
	ErrAttachmentUnsupportedType = domain.ErrAttachmentUnsupportedType
	// ErrAttachmentQuotaExceeded 窗口内上传次数达上限（领域哨兵透传）。
	ErrAttachmentQuotaExceeded = domain.ErrAttachmentQuotaExceeded
	// ErrRequestTooLarge 请求体超过单请求上限（领域哨兵透传）。
	ErrRequestTooLarge = domain.ErrRequestTooLarge
)

/* ------------------------------------------------------------------ *
 * 上限与配额（与 ADR 0004 三层闸门、nginx 12m 对齐）
 * ------------------------------------------------------------------ */

const (
	// attachmentSingleRequestLimit 应用层单请求上限（12MiB）。
	//
	// 必须 > 单文件上限且不超过 nginx 的 client_max_body_size 12m：
	// nginx 12m > 应用 12MiB(12582912) > 单文件 10MiB(10485760)。
	// 超过这一层就由业务错误拦截，绝不会让 nginx 甩一个裸 413 HTML 回来。
	attachmentSingleRequestLimit = 12 << 20 // 12582912 字节
	// attachmentSingleFileLimit 单张图片硬上限（10MiB）。
	attachmentSingleFileLimit = 10 << 20 // 10485760 字节
	// attachmentSniffLen 嗅探所需的前导字节数（http.DetectContentType 约定上限）。
	attachmentSniffLen = 512
	// attachmentDailyQuota 同一账号每窗口最多上传次数。
	attachmentDailyQuota = 50
	// attachmentQuotaWindow 配额窗口（固定 24h，非自然日）。
	attachmentQuotaWindow = 24 * time.Hour
	// attachmentObjectKeyBytes 随机对象键的熵字节数。
	attachmentObjectKeyBytes = 16
)

// 嗅探出的合法类型 → 对象键扩展名。扩展名只从真实字节推导，用户文件名永不进对象键。
var attachmentExtByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

/* ------------------------------------------------------------------ *
 * 入参
 * ------------------------------------------------------------------ */

// UploadAttachmentInput 上传附件入参。
//
// UploaderID 来自鉴权身份（AuthRequired 解出的 userID），不允许请求体指定，
// 否则会出现「替别人上传、占用别人配额」的越权。
// Part 是 multipart 的单个 Part（由 handler 用 c.MultipartReader 取出），
// 这里只当 io.Reader 消费，handler 永不把整段请求落临时文件。
// FileName 仅用于日志审计，不写入对象键、不持久化。
type UploadAttachmentInput struct {
	UploaderID int
	Part       io.Reader
	FileName   string
}

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// AttachmentUseCase 内容附件（#216 上传）用例。
type AttachmentUseCase struct {
	repo     repository.AttachmentRepo
	storage  storage.Storage
	limiter  *utils.Limiter
}

// NewAttachmentUseCase 构造附件用例。
func NewAttachmentUseCase(
	repo repository.AttachmentRepo,
	storage storage.Storage,
	limiter *utils.Limiter,
) *AttachmentUseCase {
	return &AttachmentUseCase{repo: repo, storage: storage, limiter: limiter}
}

// UploadAttachment 处理一次图片上传。
//
// 顺序（闸门从外到内）：
//  1. 配额闸门：同一账号 24h 内 ≤50 次，超限返回 429 语义错误（在读取请求体之前判定，
//     不浪费带宽）。
//  2. 单请求闸门：用 io.LimitReader 把请求体截断在 12MiB+1，超过就不让继续读。
//  3. 字节嗅探：前 512 字节真实判定 jpg / png / webp（http.DetectContentType
//     对 webp 在部分 Go 版本不识别，单独补 RIFF/WEBP 魔数）；声明类型一律以真实为准。
//  4. 单文件闸门：剩余字节用 io.LimitReader 截断在 10MiB+1，超过即「单张过大」。
//  5. 落库为孤儿行（#217 负责绑定与清理），返回 {id,url,...}。
//
// 不整块进内存、不写临时文件：请求体被直接流式读入一个有上限的小缓冲，
// 上限即单文件硬上限，不存在无界内存占用；对象键随机生成、扩展名由嗅探推导。
func (uc *AttachmentUseCase) UploadAttachment(ctx context.Context, in UploadAttachmentInput) (*domain.Attachment, error) {
	// 1. 配额闸门（先于读取，省带宽）。
	quotaKey := "limit:upload:daily:" + itoa(in.UploaderID)
	allowed, err := uc.limiter.AllowWithin(ctx, quotaKey, attachmentDailyQuota, attachmentQuotaWindow)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrAttachmentQuotaExceeded
	}

	// 2. 单请求闸门：超过 12MiB 的请求体在这里被截断，不会无限读。
	limited := io.LimitReader(in.Part, int64(attachmentSingleRequestLimit)+1)
	br := bufio.NewReaderSize(limited, 1<<16)

	// 3. 字节嗅探（前 512 字节）。
	sniff := make([]byte, attachmentSniffLen)
	n, readErr := io.ReadFull(br, sniff)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	contentType, _, ok := sniffImageType(sniff[:n])
	if !ok {
		// 真实字节不是图片：直接拒绝（读取不超过 512 字节，不会消耗大请求体）。
		return nil, ErrAttachmentUnsupportedType
	}

	// 4. 单文件闸门：剩余字节截断在 10MiB+1。
	rest, err := io.ReadAll(io.LimitReader(br, int64(attachmentSingleFileLimit)+1))
	if err != nil {
		return nil, err
	}
	full := append(sniff[:n], rest...)
	if len(full) > attachmentSingleFileLimit {
		return nil, ErrAttachmentTooLarge
	}

	// 5. 随机对象键 + 扩展名由嗅探推导；用户文件名只进日志。
	objectKey, err := newAttachmentObjectKey(contentType)
	if err != nil {
		return nil, err
	}

	if err := uc.storage.PutObject(ctx, objectKey, bytes.NewReader(full), int64(len(full)), contentType); err != nil {
		return nil, err
	}

	att := &domain.Attachment{
		ObjectKey:   objectKey,
		UploaderID:  in.UploaderID,
		ContentType: contentType,
		SizeBytes:   int64(len(full)),
	}
	if err := uc.repo.CreateOrphan(ctx, att); err != nil {
		return nil, err
	}
	return att, nil
}

// sniffImageType 由真实字节判定图片类型。
//
// 返回 (contentType, ext, ok)。webp 需要单独补魔数，因为较新 Go 的
// http.DetectContentType 对 webp 可能仍是 application/octet-stream。
func sniffImageType(b []byte) (string, string, bool) {
	ct := http.DetectContentType(b)
	switch {
	case strings.HasPrefix(ct, "image/jpeg"):
		return "image/jpeg", ".jpg", true
	case strings.HasPrefix(ct, "image/png"):
		return "image/png", ".png", true
	case isWebP(b):
		return "image/webp", ".webp", true
	}
	return "", "", false
}

// isWebP RIFF....WEBP 魔数判定。
func isWebP(b []byte) bool {
	return len(b) >= 12 &&
		string(b[0:4]) == "RIFF" &&
		string(b[8:12]) == "WEBP"
}

// newAttachmentObjectKey 生成随机对象键（16 字节熵 + 从类型推导的扩展名）。
func newAttachmentObjectKey(contentType string) (string, error) {
	buf := make([]byte, attachmentObjectKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ext, ok := attachmentExtByContentType[contentType]
	if !ok {
		ext = ""
	}
	return hex.EncodeToString(buf) + ext, nil
}

// itoa 避免引入 strconv 仅为拼 key（保持本文件依赖精简）。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
