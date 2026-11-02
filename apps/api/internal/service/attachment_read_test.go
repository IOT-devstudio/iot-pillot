package service

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/storage"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// memRepoSvc 服务层测试用的内存仓储（仅覆盖读取/绑定/清理）。
type memRepoSvc struct {
	byID   map[int]*domain.Attachment
	nextID int
}

func newMemRepoSvc() *memRepoSvc {
	return &memRepoSvc{byID: make(map[int]*domain.Attachment), nextID: 1}
}

func (s *memRepoSvc) CreateOrphan(_ context.Context, att *domain.Attachment) error {
	att.ID = s.nextID
	s.nextID++
	s.byID[att.ID] = att
	return nil
}

func (s *memRepoSvc) Get(_ context.Context, id int) (*domain.Attachment, error) {
	att, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrAttachmentNotFound
	}
	return att, nil
}

func (s *memRepoSvc) BindForBody(_ context.Context, col string, ownerID int64, authorID int, ids []int) error {
	for _, id := range ids {
		att, ok := s.byID[id]
		if !ok || att.UploaderID != authorID {
			return domain.ErrAttachmentInvalid
		}
	}
	for _, att := range s.byID {
		att.PostID, att.ReplyID, att.RoadmapID = nil, nil, nil
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

func (s *memRepoSvc) ListOrphans(_ context.Context, olderThan time.Time, limit int) ([]*domain.Attachment, error) {
	out := make([]*domain.Attachment, 0, limit)
	for _, att := range s.byID {
		bound := att.PostID != nil || att.ReplyID != nil || att.RoadmapID != nil
		if bound || !att.CreatedAt.Before(olderThan) {
			continue
		}
		out = append(out, att)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *memRepoSvc) DeleteAttachment(_ context.Context, id int) error {
	delete(s.byID, id)
	return nil
}

// memStoSvc 服务层测试用的内存对象存储（实现 storage.Storage 的全部方法）。
type memStoSvc struct {
	storage.Storage
	objects map[string][]byte
}

func newMemStoSvc() *memStoSvc { return &memStoSvc{objects: make(map[string][]byte)} }

func (s *memStoSvc) PutObject(_ context.Context, objectKey string, reader io.Reader, _ int64, _ string) error {
	b, _ := io.ReadAll(reader)
	s.objects[objectKey] = b
	return nil
}

func (s *memStoSvc) GetObject(_ context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, storage.ErrStorageUnavailable
}

func (s *memStoSvc) DeleteObject(_ context.Context, objectKey string) error {
	delete(s.objects, objectKey)
	return nil
}

func TestRecomputeBindings_ExtractsFromBody(t *testing.T) {
	repo := newMemRepoSvc()
	sto := newMemStoSvc()
	uc := NewAttachmentUseCase(repo, sto, utils.NewLimiter(newMemCounterSvc()))

	repo.CreateOrphan(context.Background(), &domain.Attachment{ObjectKey: "a", UploaderID: 1, ContentType: "image/png", SizeBytes: 1})
	repo.CreateOrphan(context.Background(), &domain.Attachment{ObjectKey: "b", UploaderID: 1, ContentType: "image/png", SizeBytes: 1})
	body := "![x](/api/v1/attachments/1) 外链不计入 https://y.z/p.png ![y](/api/v1/attachments/2) 重复 /api/v1/attachments/1"
	if err := uc.RecomputeBindings(context.Background(), "post_id", 10, 1, body); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	if repo.byID[1].PostID == nil || *repo.byID[1].PostID != 10 {
		t.Errorf("附件1应绑定到 post 10")
	}
	if repo.byID[2].PostID == nil || *repo.byID[2].PostID != 10 {
		t.Errorf("附件2应绑定到 post 10")
	}
}

func TestGetAttachment_ScopePaths(t *testing.T) {
	repo := newMemRepoSvc()
	sto := newMemStoSvc()
	uc := NewAttachmentUseCase(repo, sto, utils.NewLimiter(newMemCounterSvc()))

	repo.CreateOrphan(context.Background(), &domain.Attachment{ID: 1, ObjectKey: "a", UploaderID: 1, ContentType: "image/png", SizeBytes: 1})
	// 孤儿：仅上传者可看。
	if _, err := uc.GetAttachment(context.Background(), 2, false, 1); err != domain.ErrAttachmentForbidden {
		t.Errorf("非上传者读孤儿应 403/404，实际 %v", err)
	}
	if _, err := uc.GetAttachment(context.Background(), 1, false, 1); err != nil {
		t.Errorf("上传者读孤儿应成功，实际 %v", err)
	}
	// 不存在。
	if _, err := uc.GetAttachment(context.Background(), 1, false, 99); err != domain.ErrAttachmentNotFound {
		t.Errorf("不存在应 404，实际 %v", err)
	}
	// 路线绑定：登录即可。
	repo.byID[1].RoadmapID = int64Ptr(5)
	if _, err := uc.GetAttachment(context.Background(), 9, false, 1); err != nil {
		t.Errorf("路线绑定应登录可读，实际 %v", err)
	}
	// post 绑定（论坛未建）：默认 resolver 未判定 → 404。
	repo.byID[1].RoadmapID = nil
	repo.byID[1].PostID = int64Ptr(5)
	if _, err := uc.GetAttachment(context.Background(), 9, false, 1); err != domain.ErrAttachmentNotFound {
		t.Errorf("论坛未建时 post 绑定应 404，实际 %v", err)
	}
}

func int64Ptr(v int64) *int64 { return &v }

type memCounterSvc struct{ utils.Counter }

func newMemCounterSvc() *memCounterSvc { return &memCounterSvc{} }
