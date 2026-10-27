package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/gorm"
)

// 冻结仓储用例跑真实 PostgreSQL（复用 repoTestDBUsers 的临时库）。
//
// 为什么必须跑真库：本功能的正确性几乎全在 SQL 谓词上——
//   - 管理端默认过滤 frozen_at IS NULL，且 total 与分页同条件；
//   - 条件冻结 WHERE frozen_at IS NULL 是多实例幂等闸门；
//   - 通知时间要在「冻结之前」就能写入（流程是先发信、后冻结）。
// 桩只能复刻我自己写的 Go 判断，验证不到这些。

// seedFreezeUser 插入一个账号；CreatedAt 由调用方指定，用于构造「注册满 7 天」。
func seedFreezeUser(t *testing.T, db *gorm.DB, name string, createdAt time.Time) *domain.User {
	t.Helper()
	user := &domain.User{
		Name:      name,
		Password:  "hashed",
		Detail:    domain.Detail{Email: name + "@example.com"},
		CreatedAt: createdAt,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("预置用户失败: %v", err)
	}
	return user
}

func TestListForAdmin_DefaultHidesFrozen(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	normal := seedFreezeUser(t, db, "normal", time.Now())
	frozen := seedFreezeUser(t, db, "frozen", time.Now())
	if err := db.Model(&domain.User{}).Where("id = ?", frozen.ID).
		Updates(map[string]any{"frozen_at": time.Now(), "freeze_reason": "班级不合规"}).Error; err != nil {
		t.Fatal(err)
	}

	// 默认过滤：只剩未冻结行，且 total 与 items 一致
	users, total, err := repo.ListForAdmin(ctx, 1, 20, UserListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(users) != 1 || users[0].ID != normal.ID {
		t.Fatalf("默认应只返回未冻结账号，total=%d users=%d", total, len(users))
	}

	// include_frozen：两行都返回，冻结行带原因
	users, total, err = repo.ListForAdmin(ctx, 1, 20, UserListFilter{IncludeFrozen: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(users) != 2 {
		t.Fatalf("include_frozen 应返回 2 行，total=%d users=%d", total, len(users))
	}
	var seen bool
	for _, user := range users {
		if user.ID != frozen.ID {
			continue
		}
		seen = true
		if !user.IsFrozen() || user.FreezeReason != "班级不合规" {
			t.Fatalf("冻结行字段未正确读出: %+v", user)
		}
	}
	if !seen {
		t.Fatal("冻结行未出现在 include_frozen 结果里")
	}

	// 分页：total 是过滤后的总数，不是全表数
	users, total, err = repo.ListForAdmin(ctx, 2, 1, UserListFilter{IncludeFrozen: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(users) != 1 {
		t.Fatalf("分页结果不符: total=%d users=%d", total, len(users))
	}
}

func TestListFreezeCandidates_ExcludesFrozenDeletedAndFresh(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	old := time.Now().AddDate(0, 0, -10)
	fresh := time.Now().AddDate(0, 0, -1)

	candidate := seedFreezeUser(t, db, "candidate", old)
	seedFreezeUser(t, db, "fresh", fresh)
	frozen := seedFreezeUser(t, db, "frozen", old)
	deleted := seedFreezeUser(t, db, "deleted", old)
	if err := db.Model(&domain.User{}).Where("id = ?", frozen.ID).Update("frozen_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domain.User{}).Where("id = ?", deleted.ID).Update("deleted_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}

	users, err := repo.ListFreezeCandidates(ctx, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != candidate.ID {
		ids := make([]int, 0, len(users))
		for _, user := range users {
			ids = append(ids, user.ID)
		}
		t.Fatalf("候选应只有注册满 7 天的未冻结未注销账号，实际 %v", ids)
	}
}

func TestFreezeAndMarkNotifiedAndUnfreeze(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	user := seedFreezeUser(t, db, "zhangsan", time.Now().AddDate(0, 0, -8))
	now := time.Now()

	// 通知时间可以在冻结之前写入（流程是先发信、后冻结）。
	// 若这里要求已冻结，那一轮的通知时间永远记不上，下一轮会重复发信。
	if err := repo.MarkFreezeNotified(ctx, user.ID, now); err != nil {
		t.Fatal(err)
	}
	var afterNotify domain.User
	if err := db.First(&afterNotify, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterNotify.FreezeNotifiedAt == nil {
		t.Fatal("冻结前也应能记录通知时间")
	}

	// 首次冻结成功
	frozen, err := repo.Freeze(ctx, user.ID, "班级不合规", now)
	if err != nil {
		t.Fatal(err)
	}
	if !frozen {
		t.Fatal("首次冻结应命中")
	}

	// 二次冻结不命中（条件更新是幂等闸门，且不改写原因）
	frozen, err = repo.Freeze(ctx, user.ID, "别的理由", now)
	if err != nil {
		t.Fatal(err)
	}
	if frozen {
		t.Fatal("已冻结行不应被再次冻结")
	}

	var afterFreeze domain.User
	if err := db.First(&afterFreeze, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterFreeze.FreezeReason != "班级不合规" {
		t.Fatalf("冻结原因被改写了: %q", afterFreeze.FreezeReason)
	}

	// 解冻清三列
	if err := repo.Unfreeze(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	var afterUnfreeze domain.User
	if err := db.First(&afterUnfreeze, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterUnfreeze.IsFrozen() || afterUnfreeze.FreezeReason != "" || afterUnfreeze.FreezeNotifiedAt != nil {
		t.Fatalf("解冻后三列都应为空: %+v", afterUnfreeze)
	}

	// 未冻结账号再解冻：明确业务错误
	if err := repo.Unfreeze(ctx, user.ID); !errors.Is(err, ErrUserNotFrozen) {
		t.Fatalf("重复解冻应返回 ErrUserNotFrozen，实际 %v", err)
	}
	// 不存在的账号
	if err := repo.Unfreeze(ctx, 999999); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("不存在账号应返回 ErrUserNotFound，实际 %v", err)
	}
}

func TestFreeze_RefusesDeletedUser(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	user := seedFreezeUser(t, db, "deleted-user", time.Now().AddDate(0, 0, -8))
	if err := db.Model(&domain.User{}).Where("id = ?", user.ID).Update("deleted_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}

	frozen, err := repo.Freeze(ctx, user.ID, "x", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if frozen {
		t.Fatal("已注销账号不应被冻结")
	}
}
