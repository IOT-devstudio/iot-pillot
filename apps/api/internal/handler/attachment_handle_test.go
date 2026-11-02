package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"encoding/json"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/middleware"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/service"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/storage"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"

	"github.com/gin-gonic/gin"
)

/* ------------------------------------------------------------------ *
 * 内存桩：仓储（只覆盖 CreateOrphan，其余交嵌入接口）
 * ------------------------------------------------------------------ */

type memAttachmentRepo struct {
	repository.AttachmentRepo

	orphans   map[int]*domain.Attachment
	nextID    int
	createErr error
}

func newMemAttachmentRepo() *memAttachmentRepo {
	return &memAttachmentRepo{orphans: make(map[int]*domain.Attachment), nextID: 1}
}

func (s *memAttachmentRepo) CreateOrphan(_ context.Context, att *domain.Attachment) error {
	if s.createErr != nil {
		return s.createErr
	}
	att.ID = s.nextID
	s.nextID++
	s.orphans[att.ID] = att
	return nil
}

/* ------------------------------------------------------------------ *
 * 内存桩：对象存储（map 存字节）
 * ------------------------------------------------------------------ */

type memStorage struct {
	storage.Storage

	objects map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{objects: make(map[string][]byte)}
}

func (s *memStorage) PutObject(_ context.Context, objectKey string, reader io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[objectKey] = data
	return nil
}

func (s *memStorage) GetObject(_ context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	data, ok := s.objects[objectKey]
	if !ok {
		return nil, "", 0, errors.New("对象不存在")
	}
	return io.NopCloser(bytes.NewReader(data)), "image/png", int64(len(data)), nil
}

/* ------------------------------------------------------------------ *
 * 内存桩：限流计数器（固定窗口，计数不自动过期也能被测试直接驱动）
 * ------------------------------------------------------------------ */

type memCounter struct {
	utils.Counter

	counts map[string]int64
}

func newMemCounter() *memCounter {
	return &memCounter{counts: make(map[string]int64)}
}

func (c *memCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	c.counts[key]++
	return c.counts[key], nil
}

func (c *memCounter) SetIfAbsent(_ context.Context, key string, _ time.Duration) (bool, error) {
	if _, ok := c.counts[key]; ok {
		return false, nil
	}
	c.counts[key] = 1
	return true, nil
}

func (c *memCounter) Get(_ context.Context, key string) (int64, error) {
	return c.counts[key], nil
}

func (c *memCounter) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(c.counts, k)
	}
	return nil
}

/* ------------------------------------------------------------------ *
 * 引擎
 * ------------------------------------------------------------------ */

func newAttachmentTestEngine(t *testing.T) (*gin.Engine, *memAttachmentRepo, *memStorage, *memCounter) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMemAttachmentRepo()
	sto := newMemStorage()
	counter := newMemCounter()
	uc := service.NewAttachmentUseCase(repo, sto, utils.NewLimiter(counter))
	h := NewAttachmentHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	r.POST("/api/v1/attachments", authRequired, h.UploadAttachment)
	return r, repo, sto, counter
}

// uploadReq 构造一个 multipart 上传请求。
func uploadReq(t *testing.T, path, token, fileName string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("写 multipart 失败: %v", err)
	}
	w.Close()

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func doReq(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

/* ------------------------------------------------------------------ *
 * 验收项：合法图片放行 + 孤儿行落库 + 返回结构
 * ------------------------------------------------------------------ */

// jpeg/png/webp 的最小合法字节头（足够让 http.DetectContentType / 魔数判定通过）。
var (
	jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01}
	pngBytes  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	webpBytes = []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 0x00, 0x00}
	// 故意不是图片：纯文本。
	textBytes = []byte("this is definitely not an image, just some plain text content")
)

func TestUploadAttachment_JPEGAccepted(t *testing.T) {
	r, repo, sto, _ := newAttachmentTestEngine(t)
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "pic.jpg", jpegBytes))

	if w.Code != http.StatusOK {
		t.Fatalf("jpg 应上传成功(200)，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(repo.orphans) != 1 {
		t.Fatalf("应落 1 条孤儿行，实际 %d", len(repo.orphans))
	}
	att := repo.orphans[1]
	if att.ContentType != "image/jpeg" {
		t.Errorf("嗅探类型应为 image/jpeg，实际 %q", att.ContentType)
	}
	if att.UploaderID != 1 {
		t.Errorf("上传者应为令牌身份 1，实际 %d", att.UploaderID)
	}
	if _, ok := sto.objects[att.ObjectKey]; !ok {
		t.Errorf("对象存储应存在键 %q", att.ObjectKey)
	}
	// 对象键扩展名从嗅探推导，不含用户文件名。
	if !strings.HasSuffix(att.ObjectKey, ".jpg") {
		t.Errorf("对象键应以 .jpg 结尾，实际 %q", att.ObjectKey)
	}
	if strings.Contains(att.ObjectKey, "pic.jpg") {
		t.Errorf("对象键不应含用户文件名，实际 %q", att.ObjectKey)
	}
	// 响应结构 {id,url,content_type,size_bytes}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			ID          int    `json:"id"`
			URL         string `json:"url"`
			ContentType string `json:"content_type"`
			SizeBytes   int64  `json:"size_bytes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v，body=%s", err, w.Body.String())
	}
	if resp.Code != 0 {
		t.Errorf("业务码应为 0，实际 %d", resp.Code)
	}
	if resp.Data.ID != 1 {
		t.Errorf("响应 id 应为 1，实际 %d", resp.Data.ID)
	}
	if resp.Data.URL != "/api/v1/attachments/1" {
		t.Errorf("响应 url 应为 /api/v1/attachments/1，实际 %q", resp.Data.URL)
	}
	if resp.Data.ContentType != "image/jpeg" {
		t.Errorf("响应 content_type 应为 image/jpeg，实际 %q", resp.Data.ContentType)
	}
	if resp.Data.SizeBytes != int64(len(jpegBytes)) {
		t.Errorf("响应 size_bytes 应为 %d，实际 %d", len(jpegBytes), resp.Data.SizeBytes)
	}
}

func TestUploadAttachment_PNGAccepted(t *testing.T) {
	r, repo, _, _ := newAttachmentTestEngine(t)
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "a.png", pngBytes))
	if w.Code != http.StatusOK {
		t.Fatalf("png 应上传成功(200)，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if repo.orphans[1].ContentType != "image/png" {
		t.Errorf("嗅探类型应为 image/png，实际 %q", repo.orphans[1].ContentType)
	}
}

func TestUploadAttachment_WebPAccepted(t *testing.T) {
	r, repo, _, _ := newAttachmentTestEngine(t)
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "b.webp", webpBytes))
	if w.Code != http.StatusOK {
		t.Fatalf("webp 应上传成功(200)，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if repo.orphans[1].ContentType != "image/webp" {
		t.Errorf("嗅探类型应为 image/webp，实际 %q", repo.orphans[1].ContentType)
	}
}

func TestUploadAttachment_NonImageRejected(t *testing.T) {
	r, repo, sto, _ := newAttachmentTestEngine(t)
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "note.txt", textBytes))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非图片应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "jpg") && !strings.Contains(w.Body.String(), "webp") {
		t.Errorf("应给出支持类型提示，实际 %s", w.Body.String())
	}
	// 零副作用：不落库、不写对象。
	if len(repo.orphans) != 0 {
		t.Errorf("拒绝上传不应落库，实际 %d 行", len(repo.orphans))
	}
	if len(sto.objects) != 0 {
		t.Errorf("拒绝上传不应写对象，实际 %d", len(sto.objects))
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：未登录 401 / 缺文件 400
 * ------------------------------------------------------------------ */

func TestUploadAttachment_Unauthenticated(t *testing.T) {
	r, _, _, _ := newAttachmentTestEngine(t)
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "", "x.jpg", jpegBytes))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestUploadAttachment_MissingFile(t *testing.T) {
	r, _, _, _ := newAttachmentTestEngine(t)
	// 空 multipart（没有文件 part）。
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer good-token")
	w := doReq(r, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺文件应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "文件") {
		t.Errorf("应提示缺少文件，实际 %s", w.Body.String())
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：单文件超 10MiB → 400
 * ------------------------------------------------------------------ */

func TestUploadAttachment_SingleFileTooLarge(t *testing.T) {
	r, _, _, _ := newAttachmentTestEngine(t)
	big := make([]byte, 10<<20+1024) // 比 10MiB 大 1KB，且是合法 png 头 + 填充
	copy(big, pngBytes)
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "big.png", big))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("单文件超 10MiB 应 400，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "10MiB") {
		t.Errorf("应给出 10MiB 限制提示，实际 %s", w.Body.String())
	}
}

/* ------------------------------------------------------------------ *
 * 验收项：配额计数（同一账号累计，超限 429）
 * ------------------------------------------------------------------ */

func TestUploadAttachment_DailyQuotaCountedAndExceeded(t *testing.T) {
	r, repo, _, counter := newAttachmentTestEngine(t)

	// 第 1~50 次：成功。
	for i := 1; i <= 50; i++ {
		w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", fmt.Sprintf("p%d.png", i), pngBytes))
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次上传应成功，实际 %d，body=%s", i, w.Code, w.Body.String())
		}
	}
	// 配额 key 应累计到 50。
	key := "limit:upload:daily:1"
	if got := counter.counts[key]; got != 50 {
		t.Errorf("配额计数应为 50，实际 %d", got)
	}
	if len(repo.orphans) != 50 {
		t.Errorf("应落 50 条孤儿行，实际 %d", len(repo.orphans))
	}

	// 第 51 次：超限 429。
	w := doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "over.png", pngBytes))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("超配额应 429，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "上限") {
		t.Errorf("应给出配额提示，实际 %s", w.Body.String())
	}
	// 超限后不落库。
	if len(repo.orphans) != 50 {
		t.Errorf("超限后孤儿行不应增加，实际 %d", len(repo.orphans))
	}
}
