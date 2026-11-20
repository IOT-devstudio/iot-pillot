package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/storage"
)

// ShowcaseUseCase 作品提交（学员交作业 + 管理端审阅）。
//
// 职责边界：
//   - 学员只能看到并删除自己的作品，归属判定在仓库层用 user_id 做条件；
//   - 管理员可看全量、可推进审核状态；
//   - zip 落在对象存储，数据库只存 key。
type ShowcaseUseCase struct {
	repo repository.ShowcaseRepo
	// storage 可为 nil：为 nil 时 zip 上传与下载不可用，
	// 纯列表/审阅路径不受影响（与 LearningRoadmap 的 attachments 同口径）。
	storage storage.Storage
	// roadmaps 用于取路线标题快照。可为 nil（提交时就不带标题快照）。
	roadmaps repository.LearningRoadmapRepo
}

// NewShowcaseUseCase 构造函数。
func NewShowcaseUseCase(repo repository.ShowcaseRepo) *ShowcaseUseCase {
	return &ShowcaseUseCase{repo: repo}
}

// WithStorage 注入对象存储。
func (uc *ShowcaseUseCase) WithStorage(s storage.Storage) *ShowcaseUseCase {
	uc.storage = s
	return uc
}

// WithRoadmaps 注入学习路线仓库（用于标题快照）。
func (uc *ShowcaseUseCase) WithRoadmaps(r repository.LearningRoadmapRepo) *ShowcaseUseCase {
	uc.roadmaps = r
	return uc
}

// CreateShowcaseInput 一次提交的入参。
//
// Archive 可选：给了就先把 zip 落对象存储再写库；
// 落库失败时会把对象删掉，避免留下孤儿文件。
type CreateShowcaseInput struct {
	RoadmapID *int64
	Title     string
	Summary   string
	RepoURL   string

	ArchiveName string
	ArchiveType string
	ArchiveSize int64
	// ArchiveData 源码包内容。为空表示这次不交 zip（走 GitHub 链接）。
	ArchiveData []byte
}

// Create 创建一份作品提交。
//
// 顺序刻意是「先校验 → 再传文件 → 最后落库」：
// 内容不合法时不该先把文件传上去再回滚。
func (uc *ShowcaseUseCase) Create(ctx context.Context, userID int64, in CreateShowcaseInput) (*domain.ShowcaseSubmission, error) {
	hasArchive := len(in.ArchiveData) > 0
	// 先做与存储无关的校验；此时还不知道 archiveKey，先按「有没有文件」占位。
	if err := domain.ValidateShowcaseInput(in.Title, in.Summary, in.RepoURL, boolToKey(hasArchive)); err != nil {
		return nil, err
	}

	submission := &domain.ShowcaseSubmission{
		UserID:  userID,
		Title:   strings.TrimSpace(in.Title),
		Summary: in.Summary,
		RepoURL: strings.TrimSpace(in.RepoURL),
	}

	// 路线标题快照：路线可能之后被改名或删除，学员要能看出当时交的是哪条。
	if in.RoadmapID != nil && uc.roadmaps != nil {
		submission.RoadmapID = in.RoadmapID
		if rm, err := uc.roadmaps.GetByID(ctx, int(*in.RoadmapID)); err == nil && rm != nil {
			submission.RoadmapTitle = rm.Title
		}
	}

	if hasArchive {
		key, err := uc.putArchive(ctx, in)
		if err != nil {
			return nil, err
		}
		submission.ArchiveObjectKey = key
		submission.ArchiveName = in.ArchiveName
		submission.ArchiveSize = int64(len(in.ArchiveData))
	}

	if err := uc.repo.Create(ctx, submission); err != nil {
		// 落库失败要把刚传上去的对象删掉，否则每次失败都留一个孤儿文件。
		uc.discardArchive(ctx, submission.ArchiveObjectKey)
		return nil, err
	}
	return submission, nil
}

// putArchive 校验并上传源码包，返回对象 key。
//
// 校验顺序刻意是「先看文件本身合不合规，再看存储能不能用」：
// 用户传了 rar 却被告知「存储不可用」，会去重试一个根本不会成功的操作。
func (uc *ShowcaseUseCase) putArchive(ctx context.Context, in CreateShowcaseInput) (string, error) {
	if !domain.IsZipArchive(in.ArchiveName, in.ArchiveType) {
		return "", domain.ErrShowcaseArchiveUnsupportedType
	}
	if int64(len(in.ArchiveData)) > domain.ShowcaseArchiveMaxBytes {
		return "", domain.ErrShowcaseArchiveTooLarge
	}
	if uc.storage == nil {
		return "", errors.New("对象存储不可用，暂时无法接收源码包")
	}

	key, err := newShowcaseArchiveKey()
	if err != nil {
		return "", err
	}
	if err := uc.storage.PutObject(ctx, key, bytes.NewReader(in.ArchiveData), int64(len(in.ArchiveData)), "application/zip"); err != nil {
		return "", err
	}
	return key, nil
}

// discardArchive 尽力删除对象（失败只记日志：主体流程已经失败了）。
func (uc *ShowcaseUseCase) discardArchive(ctx context.Context, key string) {
	if uc.storage == nil || key == "" {
		return
	}
	if err := uc.storage.DeleteObject(ctx, key); err != nil {
		log.Printf("[showcase] 清理孤儿源码包失败 key=%s: %v", key, err)
	}
}

// ListByUser 本人的作品列表。
func (uc *ShowcaseUseCase) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]*domain.ShowcaseSubmission, int64, error) {
	return uc.repo.ListByUser(ctx, userID, page, pageSize)
}

// Delete 删除本人的作品（连带删除 zip 对象）。
//
// 先删库再删对象：库删成功而对象没删掉只是留个孤儿文件（有清理任务兜底）；
// 反过来先删对象再删库失败，会让一条记录指向不存在的文件。
func (uc *ShowcaseUseCase) Delete(ctx context.Context, id, userID int64) error {
	existing, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// 不区分「不存在」与「不是你的」：回 404 而不是 403，
	// 否则可以用 403 探测出别人作品的存在性。
	if existing == nil || existing.UserID != userID {
		return domain.ErrShowcaseNotFound
	}
	if err := uc.repo.DeleteOwn(ctx, id, userID); err != nil {
		return err
	}
	uc.discardArchive(ctx, existing.ArchiveObjectKey)
	return nil
}

// ListAll 管理端全量列表。
func (uc *ShowcaseUseCase) ListAll(ctx context.Context, status string, page, pageSize int) ([]*domain.ShowcaseSubmission, int64, error) {
	return uc.repo.ListAll(ctx, status, page, pageSize)
}

// OpenArchive 读取源码包，供下载。
//
// 返回 (元信息, 内容流, 错误)。内容流由调用方关闭。
// 归属判定在这里做：不是本人的作品按「不存在」处理，避免用 403 探测他人作品。
func (uc *ShowcaseUseCase) OpenArchive(ctx context.Context, id, userID int64, isAdmin bool) (*domain.ShowcaseSubmission, io.ReadCloser, int64, error) {
	existing, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, 0, err
	}
	if existing == nil || (!isAdmin && existing.UserID != userID) {
		return nil, nil, 0, domain.ErrShowcaseNotFound
	}
	if existing.ArchiveObjectKey == "" {
		return nil, nil, 0, domain.ErrShowcaseNotFound
	}
	if uc.storage == nil {
		return nil, nil, 0, errors.New("对象存储不可用，暂时无法下载源码包")
	}
	rc, _, size, err := uc.storage.GetObject(ctx, existing.ArchiveObjectKey)
	if err != nil {
		return nil, nil, 0, err
	}
	return existing, rc, size, nil
}

// Review 推进审核状态。
//
// 驳回**必须**带意见：只说「不通过」而不说改什么，学员下一次提交大概率还是同样的问题。
// 通过时意见可空。
func (uc *ShowcaseUseCase) Review(ctx context.Context, id, reviewerID int64, status, note string) error {
	if !domain.ShowcaseStatusKnown(status) || status == domain.ShowcaseStatusSubmitted {
		return &domain.ShowcaseValidationError{Reason: "审核状态只能是「通过」或「驳回」"}
	}
	trimmedNote := strings.TrimSpace(note)
	if status == domain.ShowcaseStatusRejected && trimmedNote == "" {
		return &domain.ShowcaseValidationError{Reason: "驳回时请填写理由，让学员知道要改什么"}
	}
	if len([]rune(trimmedNote)) > domain.ShowcaseReviewNoteMaxRunes {
		return &domain.ShowcaseValidationError{Reason: "审核意见最多 1000 个字"}
	}
	return uc.repo.Review(ctx, id, status, trimmedNote, reviewerID)
}

// newShowcaseArchiveKey 生成源码包的随机对象键。
//
// 前缀区分目录，便于按前缀统计与清理；随机名避免与图片附件撞键。
func newShowcaseArchiveKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "showcase/" + hex.EncodeToString(buf) + ".zip", nil
}

// boolToKey 把「有没有文件」转成 ValidateShowcaseInput 需要的载体标记。
func boolToKey(has bool) string {
	if has {
		return "archive"
	}
	return ""
}
