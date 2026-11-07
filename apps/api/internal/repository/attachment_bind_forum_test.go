package repository

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

	"gorm.io/gorm"
)

// 本文件锁住 #241 的附件绑定语义：
//
//  1. 保存后正文里引用的附件行被回填到 post_id / reply_id；
//  2. 编辑后正文里消失的图**解绑回孤儿态**（三列全空，等待 24h 清理）；
//  3. 校验失败（不存在 / 别人上传的 / 已被别的内容占用）整次回滚，不留半成品。
//
// 必须跑真实迁移建出的 schema：`num_nonnulls(post_id, reply_id, roadmap_id) <= 1`
// 这条 CHECK 与三列可空是「孤儿态」这个概念的真源，手写 DDL 会让测试与生产脱节
// （见 repoTestDBForum 的注释）。

// insertOrphan 插入一条孤儿附件，返回 id。
func insertOrphan(t *testing.T, db *gorm.DB, key string, uploader int) int {
	t.Helper()
	var id int
	if err := db.Raw(
		`INSERT INTO content_attachments (object_key, uploader_id, content_type, size_bytes)
		 VALUES (?, ?, 'image/png', 1) RETURNING id`,
		key, uploader,
	).Scan(&id).Error; err != nil {
		t.Fatalf("预置附件失败: %v", err)
	}
	return id
}

// attachmentBinding 读出一行的三列绑定值。
type attachmentBinding struct {
	PostID    *int64
	ReplyID   *int64
	RoadmapID *int64
}

func (b attachmentBinding) orphan() bool {
	return b.PostID == nil && b.ReplyID == nil && b.RoadmapID == nil
}

func (b attachmentBinding) String() string {
	return "post=" + fmtPtr(b.PostID) + " reply=" + fmtPtr(b.ReplyID) + " roadmap=" + fmtPtr(b.RoadmapID)
}

func fmtPtr(p *int64) string {
	if p == nil {
		return "NULL"
	}
	return strconv.FormatInt(*p, 10)
}

// readBinding 读回某条附件的三列。
func readBinding(t *testing.T, db *gorm.DB, id int) attachmentBinding {
	t.Helper()
	var b attachmentBinding
	if err := db.Raw(
		`SELECT post_id, reply_id, roadmap_id FROM content_attachments WHERE id = ?`, id,
	).Row().Scan(&b.PostID, &b.ReplyID, &b.RoadmapID); err != nil {
		t.Fatalf("读取附件 %d 绑定失败: %v", id, err)
	}
	return b
}

/* ------------------------------ 用例 ------------------------------ */

// TestBindForBody_BindsAndUnbinds 绑定 → 解绑回孤儿态的完整回路。
func TestBindForBody_BindsAndUnbinds(t *testing.T) {
	db := repoTestDBForum(t)
	repo := NewAttachmentRepo(db)
	ctx := context.Background()

	const uploader = 101
	a1 := insertOrphan(t, db, "k-241-a1", uploader)
	a2 := insertOrphan(t, db, "k-241-a2", uploader)
	a3 := insertOrphan(t, db, "k-241-a3", uploader)

	// 第一轮：正文引用 a1 + a2。
	if err := repo.BindForBody(ctx, "post_id", 900, uploader, []int{a1, a2}); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	for _, id := range []int{a1, a2} {
		if b := readBinding(t, db, id); b.PostID == nil || *b.PostID != 900 {
			t.Fatalf("附件 %d 应绑 post_id=900，实际 %s", id, b)
		}
	}
	if b := readBinding(t, db, a3); !b.orphan() {
		t.Fatalf("未被引用的附件 %d 应保持孤儿态，实际 %s", a3, b)
	}

	// 第二轮：正文改成只剩 a2 —— AC 要求「移除某张图后，该附件解绑回孤儿态」。
	if err := repo.BindForBody(ctx, "post_id", 900, uploader, []int{a2}); err != nil {
		t.Fatalf("重算失败: %v", err)
	}
	if b := readBinding(t, db, a1); !b.orphan() {
		t.Fatalf("被移除的附件 %d 应解绑回孤儿态，实际 %s", a1, b)
	}
	if b := readBinding(t, db, a2); b.PostID == nil || *b.PostID != 900 {
		t.Fatalf("留下的附件 %d 应继续绑 post_id=900，实际 %s", a2, b)
	}

	// 第三轮：正文清空（ids 为空），同列同 owner 的全部解绑。
	if err := repo.BindForBody(ctx, "post_id", 900, uploader, nil); err != nil {
		t.Fatalf("清空绑定失败: %v", err)
	}
	if b := readBinding(t, db, a2); !b.orphan() {
		t.Fatalf("清空正文后附件 %d 应回到孤儿态，实际 %s", a2, b)
	}
}

// TestBindForBody_ReplyColumn 回复走 reply_id 列，与 post_id 互不串列。
func TestBindForBody_ReplyColumn(t *testing.T) {
	db := repoTestDBForum(t)
	repo := NewAttachmentRepo(db)
	ctx := context.Background()

	const uploader = 102
	id := insertOrphan(t, db, "k-241-reply", uploader)

	if err := repo.BindForBody(ctx, "reply_id", 700, uploader, []int{id}); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	b := readBinding(t, db, id)
	if b.ReplyID == nil || *b.ReplyID != 700 || b.PostID != nil || b.RoadmapID != nil {
		t.Fatalf("附件 %d 应绑 reply_id=700，实际 %s", id, b)
	}
}

// TestBindForBody_RejectsForeignUpload 别人上传的图一律拒绝，且不落任何绑定。
func TestBindForBody_RejectsForeignUpload(t *testing.T) {
	db := repoTestDBForum(t)
	repo := NewAttachmentRepo(db)
	ctx := context.Background()

	id := insertOrphan(t, db, "k-241-foreign", 201)

	// 上传者是 201，却以 202 的身份绑定 → 拒绝。
	err := repo.BindForBody(ctx, "post_id", 800, 202, []int{id})
	if !errors.Is(err, domain.ErrAttachmentForbidden) {
		t.Fatalf("期望 ErrAttachmentForbidden，实际 %v", err)
	}
	if b := readBinding(t, db, id); !b.orphan() {
		t.Fatalf("被拒后附件 %d 应保持孤儿态，实际 %s", id, b)
	}
}

// TestBindForBody_RejectsMissingID 引用不存在的 id 整次失败，同批合法 id 也不绑。
func TestBindForBody_RejectsMissingID(t *testing.T) {
	db := repoTestDBForum(t)
	repo := NewAttachmentRepo(db)
	ctx := context.Background()

	const uploader = 103
	id := insertOrphan(t, db, "k-241-ok", uploader)

	// 合法 id + 不存在 id 混在一起：整次回滚，不留半成品。
	err := repo.BindForBody(ctx, "post_id", 801, uploader, []int{id, 999999})
	if !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("期望 ErrAttachmentNotFound，实际 %v", err)
	}
	if b := readBinding(t, db, id); !b.orphan() {
		t.Fatalf("回滚后附件 %d 应仍是孤儿态，实际 %s", id, b)
	}
}

// TestBindForBody_RejectsAlreadyBoundElsewhere 已被别的内容占用的图不能抢绑。
func TestBindForBody_RejectsAlreadyBoundElsewhere(t *testing.T) {
	db := repoTestDBForum(t)
	repo := NewAttachmentRepo(db)
	ctx := context.Background()

	const uploader = 104
	id := insertOrphan(t, db, "k-241-taken", uploader)

	// 先绑到帖子 555，再试图改绑到 802。
	if err := repo.BindForBody(ctx, "post_id", 555, uploader, []int{id}); err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	err := repo.BindForBody(ctx, "post_id", 802, uploader, []int{id})
	if !errors.Is(err, domain.ErrAttachmentForbidden) {
		t.Fatalf("期望 ErrAttachmentForbidden，实际 %v", err)
	}
	b := readBinding(t, db, id)
	if b.PostID == nil || *b.PostID != 555 {
		t.Fatalf("被拒后原绑定应保持 post_id=555，实际 %s", b)
	}
}
