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

func (s *memStorage) DeleteObject(_ context.Context, objectKey string) error {
	delete(s.objects, objectKey)
	return nil
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

// withBearer 把 Bearer 头写进请求（middleware 从 Authorization 头读令牌）。
func withBearer(token string) context.Context {
	// 这里返回 context 只是占位；真正注入在 req 上做。测试统一用 reqWithToken。
	return context.Background()
}

func reqWithToken(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
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


/* ------------------------------------------------------------------ *
 * 内存桩扩展：支撑 #217 读取 / 绑定 / 清理测试
 * ------------------------------------------------------------------ */

// memAttachmentRepoFull 在 CreateOrphan 基础上补齐 Get / BindForBody /
// ListOrphans / DeleteAttachment，全部走内存 map，模拟真实事务语义。
type memAttachmentRepoFull struct {
	*memAttachmentRepo

	// bound 记录每个 owner 列当前绑定的 id 集合（用于 BindForBody 校验占用）。
	byID map[int]*domain.Attachment
}

func newMemAttachmentRepoFull() *memAttachmentRepoFull {
	return &memAttachmentRepoFull{
		memAttachmentRepo: newMemAttachmentRepo(),
		byID:              make(map[int]*domain.Attachment),
	}
}

func (s *memAttachmentRepoFull) Get(_ context.Context, id int) (*domain.Attachment, error) {
	att, ok := s.byID[id]
	if !ok {
		// 孤儿行也存在 byID 中；这里统一从 byID 读。
		return nil, repository.ErrAttachmentNotFound
	}
	return att, nil
}

func (s *memAttachmentRepoFull) BindForBody(_ context.Context, col string, ownerID int64, authorID int, ids []int) error {
	// 校验：存在 / UploaderID==authorID / 未被其他 owner 占用。
	for _, id := range ids {
		att, ok := s.byID[id]
		if !ok {
			return domain.ErrAttachmentInvalid
		}
		if att.UploaderID != authorID {
			return domain.ErrAttachmentInvalid
		}
		// 占用检查：该 id 已绑定到别的 owner（同列不同 owner，或任意绑定列非空）。
		if occupiedByOther(att, col, ownerID) {
			return domain.ErrAttachmentInvalid
		}
	}
	// 简单实现：把 ids 设到对应列，旧绑定（同列）清掉。
	for _, att := range s.byID {
		clearCol(att, col)
	}
	for _, id := range ids {
		att := s.byID[id]
		switch col {
		case "post_id":
			att.PostID = &ownerID
		case "reply_id":
			att.ReplyID = &ownerID
		case "roadmap_id":
			att.RoadmapID = &ownerID
		}
	}
	return nil
}

func occupiedByOther(att *domain.Attachment, col string, ownerID int64) bool {
	bound := att.PostID != nil || att.ReplyID != nil || att.RoadmapID != nil
	if !bound {
		return false
	}
	switch col {
	case "post_id":
		return att.PostID == nil || *att.PostID != ownerID
	case "reply_id":
		return att.ReplyID == nil || *att.ReplyID != ownerID
	case "roadmap_id":
		return att.RoadmapID == nil || *att.RoadmapID != ownerID
	}
	return false
}

func clearCol(att *domain.Attachment, col string) {
	switch col {
	case "post_id":
		att.PostID = nil
	case "reply_id":
		att.ReplyID = nil
	case "roadmap_id":
		att.RoadmapID = nil
	}
}

func (s *memAttachmentRepoFull) ListOrphans(_ context.Context, olderThan time.Time, limit int) ([]*domain.Attachment, error) {
	out := make([]*domain.Attachment, 0, limit)
	for _, att := range s.byID {
		bound := att.PostID != nil || att.ReplyID != nil || att.RoadmapID != nil
		if bound {
			continue
		}
		if !att.CreatedAt.Before(olderThan) {
			continue
		}
		out = append(out, att)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *memAttachmentRepoFull) DeleteAttachment(_ context.Context, id int) error {
	delete(s.byID, id)
	delete(s.orphans, id)
	return nil
}

// 补齐 CreateOrphan：同步写 byID。
func (s *memAttachmentRepoFull) CreateOrphan(ctx context.Context, att *domain.Attachment) error {
	if err := s.memAttachmentRepo.CreateOrphan(ctx, att); err != nil {
		return err
	}
	att.CreatedAt = time.Now()
	cp := *att
	s.byID[att.ID] = &cp
	return nil
}


/* ------------------------------------------------------------------ *
 * 验收项（#217）：正文正则提取、读取归属、绑定、清理
 * ------------------------------------------------------------------ */

func TestExtractAttachmentIDs_DedupAndOrder(t *testing.T) {
	body := "看图 ![a](/api/v1/attachments/3) 和 ![b](/api/v1/attachments/1) 还有 /api/v1/attachments/3 重复；外链 https://x.com/i/9.png 不应计入"
	got := service.ExtractAttachmentIDs(body)
	want := []int{3, 1}
	if len(got) != len(want) {
		t.Fatalf("应去重保序为 [3,1]，实际 %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个应为 %d，实际 %d", i, want[i], got[i])
		}
	}
}

// newGetEngine 构造带 GET 端点的引擎，使用完整内存桩。
func newGetEngine(t *testing.T) (*gin.Engine, *memAttachmentRepoFull, *memStorage, *memCounter) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMemAttachmentRepoFull()
	sto := newMemStorage()
	counter := newMemCounter()
	uc := service.NewAttachmentUseCase(repo, sto, utils.NewLimiter(counter))
	h := NewAttachmentHandler(uc)

	r := gin.New()
	authRequired := middleware.AuthRequired(stubValidator{accept: "good-token", userID: 1})
	r.POST("/api/v1/attachments", authRequired, h.UploadAttachment)
	r.GET("/api/v1/attachments/:id", authRequired, h.GetAttachment)
	return r, repo, sto, counter
}

func TestGetAttachment_OrphanOnlyUploader(t *testing.T) {
	r, repo, sto, _ := newGetEngine(t)

	// 上传一张图（userID=1），得到孤儿行 id=1。
	_ = doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "a.png", pngBytes))
	att := repo.orphans[1]
	sto.objects[att.ObjectKey] = pngBytes

	// 上传者本人读：200。
	w := doReq(r, reqWithToken(http.MethodGet, "/api/v1/attachments/1", "good-token"))
	if w.Code != http.StatusOK {
		t.Fatalf("上传者应能读(200)，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" {
		t.Errorf("缓存头缺失或错误: %q", w.Header().Get("Cache-Control"))
	}
	if !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Errorf("返回字节应与上传一致")
	}

	// 非上传者（userID=2）读：404（防枚举）。
	r2 := gin.New()
	uc := service.NewAttachmentUseCase(repo, sto, utils.NewLimiter(newMemCounter()))
	h := NewAttachmentHandler(uc)
	r2.GET("/api/v1/attachments/:id", middleware.AuthRequired(stubValidator{accept: "other", userID: 2}), h.GetAttachment)
	w2 := doReq(r2, reqWithToken(http.MethodGet, "/api/v1/attachments/1", "other"))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("他人读孤儿应 404，实际 %d，body=%s", w2.Code, w2.Body.String())
	}
}

func TestGetAttachment_NonexistentIs404(t *testing.T) {
	r, _, _, _ := newGetEngine(t)
	w := doReq(r, reqWithToken(http.MethodGet, "/api/v1/attachments/999", "good-token"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("不存在的附件应 404，实际 %d", w.Code)
	}
}

func TestGetAttachment_InvalidIDIs404(t *testing.T) {
	r, _, _, _ := newGetEngine(t)
	// 非数字 / 非正整数都按 404，不暴露解析失败。
	for _, id := range []string{"abc", "0", "-1", ""} {
		w := doReq(r, reqWithToken(http.MethodGet, "/api/v1/attachments/"+id, "good-token"))
		if w.Code != http.StatusNotFound {
			t.Fatalf("非法 id %q 应 404，实际 %d", id, w.Code)
		}
	}
}

func TestGetAttachment_RoadmapLoginOK(t *testing.T) {
	r, repo, sto, _ := newGetEngine(t)
	_ = doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "a.png", pngBytes))
	att := repo.orphans[1]
	sto.objects[att.ObjectKey] = pngBytes

	// 模拟绑定到某条学习路线（roadmap_id 非空）→ 登录即可读。
	rid := int64(7)
	att.RoadmapID = &rid
	repo.byID[att.ID] = att

	w := doReq(r, reqWithToken(http.MethodGet, "/api/v1/attachments/1", "good-token"))
	if w.Code != http.StatusOK {
		t.Fatalf("路线绑定应登录可读(200)，实际 %d，body=%s", w.Code, w.Body.String())
	}
}

func TestGetAttachment_UnboundPostIs404(t *testing.T) {
	r, repo, sto, _ := newGetEngine(t)
	_ = doReq(r, uploadReq(t, "/api/v1/attachments", "good-token", "a.png", pngBytes))
	att := repo.orphans[1]
	sto.objects[att.ObjectKey] = pngBytes

	// post_id 非空但论坛表未建 → 默认 resolver 返回 "" 未判定 → 404。
	pid := int64(5)
	att.PostID = &pid
	repo.byID[att.ID] = att

	w := doReq(r, reqWithToken(http.MethodGet, "/api/v1/attachments/1", "good-token"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("论坛未建时 post 绑定应 404（未判定），实际 %d", w.Code)
	}
}

/* ------------------------------------------------------------------ *
 * 验收项（#217）：孤儿清理幂等
 * ------------------------------------------------------------------ */

func TestCleanupOrphans_DeletesOnlyOldOrphans(t *testing.T) {
	repo := newMemAttachmentRepoFull()
	sto := newMemStorage()
	uc := service.NewAttachmentUseCase(repo, sto, utils.NewLimiter(newMemCounter()))
	_ = uc  // 清理走调度器，直接构造

	sched := service.NewAttachmentCleanupScheduler(repo, sto)

	// 老孤儿（>24h）和刚上传的孤儿。
	old := &domain.Attachment{ID: 1, ObjectKey: "old.key", UploaderID: 1, ContentType: "image/png", SizeBytes: 10}
	old.CreatedAt = time.Now().Add(-48 * time.Hour)
	repo.byID[1] = old
	repo.orphans[1] = old
	sto.objects["old.key"] = pngBytes

	fresh := &domain.Attachment{ID: 2, ObjectKey: "new.key", UploaderID: 1, ContentType: "image/png", SizeBytes: 10}
	fresh.CreatedAt = time.Now()
	repo.byID[2] = fresh
	repo.orphans[2] = fresh
	sto.objects["new.key"] = pngBytes

	if _, err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce 失败: %v", err)
	}
	if _, ok := sto.objects["old.key"]; ok {
		t.Errorf("老孤儿对象应被删")
	}
	if _, ok := sto.objects["new.key"]; !ok {
		t.Errorf("新孤儿对象应保留")
	}
	if _, ok := repo.byID[1]; ok {
		t.Errorf("老孤儿行应被删")
	}
	if _, ok := repo.byID[2]; !ok {
		t.Errorf("新孤儿行应保留")
	}

	// 幂等：再跑一次不应出错，且无对象遗留。
	if _, err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("第二次 RunOnce 失败: %v", err)
	}
}
