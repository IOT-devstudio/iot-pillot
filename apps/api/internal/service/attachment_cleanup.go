package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/storage"
)

// 孤儿清理常量（与 ADR 0004 §7 对齐）。
const (
	// attachmentCleanupInterval 清理周期：每小时一次。
	attachmentCleanupInterval = time.Hour
	// attachmentCleanupBatch 单批最多清理条数。
	attachmentCleanupBatch = 500
	// attachmentOrphanTTL 孤儿存活上限：超过即清理（24h）。
	attachmentOrphanTTL = 24 * time.Hour
	// attachmentCleanupRunTimeout 单轮清理超时（保护，避免卡死）。
	attachmentCleanupRunTimeout = 5 * time.Minute
)

// AttachmentCleanupScheduler 孤儿附件清理（#217）。
//
// 范式同 FreezeScheduler：进程内 ticker，挂在 Router 生命周期上（Start/Stop），
// RunOnce 是纯编排、可独立测。孤儿定义见 ADR 0004 §7：三列全空且
// created_at < now()-24h，单批最多 500 条。
type AttachmentCleanupScheduler struct {
	repo    repository.AttachmentRepo
	storage storage.Storage

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewAttachmentCleanupScheduler 构造调度器（不启动）。
func NewAttachmentCleanupScheduler(repo repository.AttachmentRepo, sto storage.Storage) *AttachmentCleanupScheduler {
	return &AttachmentCleanupScheduler{
		repo:    repo,
		storage: sto,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start 启动后台循环：立即扫一次，之后每 interval 扫一次，直到 Stop。
func (s *AttachmentCleanupScheduler) Start() {
	go func() {
		defer close(s.done)
		s.runOnce("启动")
		ticker := time.NewTicker(attachmentCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runOnce("定时")
			case <-s.stop:
				return
			}
		}
	}()
}

// Stop 停止并等待当前一轮结束（同 FreezeScheduler 的取舍）。
func (s *AttachmentCleanupScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		<-s.done
	})
}

// runOnce 执行一轮并只记日志（后台任务无调用方接收错误）。
func (s *AttachmentCleanupScheduler) runOnce(trigger string) {
	ctx, cancel := context.WithTimeout(context.Background(), attachmentCleanupRunTimeout)
	defer cancel()

	count, err := s.RunOnce(ctx)
	if err != nil {
		log.Printf("[attachment-cleanup] %s清理失败（下一轮重试）: %v", trigger, err)
		return
	}
	log.Printf("[attachment-cleanup] %s清理完成：删除孤儿 %d", trigger, count)
}

// RunOnce 删除一批孤儿附件（含对象存储），返回删除条数。
//
// 纯编排：遍历 ListOrphans 的结果，先删对象再删库行（对象删失败不删库，
// 下次还能重试；库行删失败也不动对象，避免悬空对象）。幂等靠对象 key 的
// 幂等删除与库行主键唯一。
func (s *AttachmentCleanupScheduler) RunOnce(ctx context.Context) (int, error) {
	olderThan := time.Now().Add(-attachmentOrphanTTL)
	orphans, err := s.repo.ListOrphans(ctx, olderThan, attachmentCleanupBatch)
	if err != nil {
		return 0, err
	}
	if len(orphans) == 0 {
		return 0, nil
	}

	var deleted int
	for _, att := range orphans {
		// 先删对象（幂等：对象不存在也返回 nil）。
		if err := s.storage.DeleteObject(ctx, att.ObjectKey); err != nil {
			log.Printf("[attachment-cleanup] 删除对象 %q 失败，跳过本行: %v", att.ObjectKey, err)
			continue
		}
		if err := s.repo.DeleteAttachment(ctx, att.ID); err != nil {
			log.Printf("[attachment-cleanup] 删除附件行 #%d 失败（对象已删，将留悬空对象）: %v", att.ID, err)
			continue
		}
		deleted++
	}
	return deleted, nil
}
