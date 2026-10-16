package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// repoTestDBUsers 一次性库 + users 表（issue #142 登录面排除与重注册测试）。
//
// 模式与 form_repo_test.repoTestDB 一致：DSN 未设则 skip（本地无库时），
// CI 的 ci.yml 提供 IOT_MIGRATION_TEST_DSN。DDL 必须**复刻生产约束**
// ——特别是迁移 0011 的邮箱部分唯一索引，否则「同邮箱重注册」
// 这种生产里由索引决定的行为会在宽松 schema 上测成假绿。
func repoTestDBUsers(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("IOT_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("IOT_MIGRATION_TEST_DSN is unset")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("repo_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
		if _, err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})

	ddl := []string{
		`CREATE TABLE users (
			id bigserial PRIMARY KEY,
			name text NOT NULL,
			detail_student_id bigint,
			detail_class text,
			detail_qq text,
			detail_direction text,
			detail_email text,
			parse int NOT NULL DEFAULT 0,
			password text NOT NULL,
			deleted_at timestamptz,
			created_at timestamptz,
			updated_at timestamptz
		)`,
		// 与迁移 0001 一致：name 无唯一约束（重名合法）
		`CREATE INDEX idx_users_name ON users (name)`,
		// 与迁移 0011 一致（#142 Q5）：仅未注销行参与邮箱唯一性
		`CREATE UNIQUE INDEX uniq_users_detail_email
			ON users (detail_email)
			WHERE detail_email IS NOT NULL AND detail_email <> '' AND deleted_at IS NULL`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}
	return db
}

func seedUser(t *testing.T, db *gorm.DB, name, email string) *domain.User {
	t.Helper()
	user := &domain.User{
		Name:     name,
		Password: "hashed",
		Detail:   domain.Detail{Email: email},
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed 用户失败: %v", err)
	}
	return user
}

/* ------------------------------------------------------------------ *
 * 登录面排除（issue #142：注销后不能登录、不能撞库、不能走重置）
 * ------------------------------------------------------------------ */

// 注销后 GetByName / GetByEmail / SelectUserByNameAndPassword 全部查不到；
// GetByID / GetAll 仍能看到（管理端标 is_deleted、历史按 ID 可查）。
func TestSoftDelete_ExcludesFromLoginFace(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	user := seedUser(t, db, "victim", "victim@example.com")

	if err := repo.SoftDelete(ctx, user.ID); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}

	// 登录面三处入口：全部拒绝（ErrUserNotFound 由调用方翻成凭据错误）
	if _, err := repo.GetByName(ctx, "victim"); err != ErrUserNotFound {
		t.Errorf("GetByName 注销后应 ErrUserNotFound，实际: %v", err)
	}
	if _, err := repo.GetByEmail(ctx, "victim@example.com"); err != ErrUserNotFound {
		t.Errorf("GetByEmail 注销后应 ErrUserNotFound，实际: %v", err)
	}
	if _, err := repo.SelectUserByNameAndPassword(ctx, "victim", "hashed"); err == nil {
		t.Error("SelectUserByNameAndPassword 注销后应查不到")
	}

	// 管理面：GetByID 仍可见且带软删标记；GetAll 不排行
	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID 注销后应仍可见: %v", err)
	}
	if !got.IsDeleted() {
		t.Error("GetByID 返回的行应带 deleted_at 标记")
	}
	all, total, err := repo.GetAll(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if total != 1 || len(all) != 1 || !all[0].IsDeleted() {
		t.Errorf("GetAll 应包含已注销行（Q3 历史保留），total=%d len=%d", total, len(all))
	}
}

// 重复注销 → ErrUserAlreadyDeleted，不重复写；不存在的 ID → ErrUserNotFound。
func TestSoftDelete_IdempotencyGuards(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	user := seedUser(t, db, "once", "once@example.com")

	if err := repo.SoftDelete(ctx, user.ID); err != nil {
		t.Fatalf("首次 SoftDelete() error = %v", err)
	}
	if err := repo.SoftDelete(ctx, user.ID); err != ErrUserAlreadyDeleted {
		t.Errorf("重复注销应 ErrUserAlreadyDeleted，实际: %v", err)
	}
	if err := repo.SoftDelete(ctx, 99999); err != ErrUserNotFound {
		t.Errorf("不存在的 ID 应 ErrUserNotFound，实际: %v", err)
	}
}

/* ------------------------------------------------------------------ *
 * 同邮箱重注册（issue #142 Q5：部分唯一索引释放已注销行的邮箱）
 * ------------------------------------------------------------------ */

// 注销后同邮箱可再次 INSERT（索引不冲突）；未注销行仍互相冲突。
func TestEmailUnique_ReleasedBySoftDelete(t *testing.T) {
	db := repoTestDBUsers(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	old := seedUser(t, db, "old", "reuse@example.com")
	if err := repo.SoftDelete(ctx, old.ID); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}

	// 同邮箱新账号：应成功（0011 索引谓词 deleted_at IS NULL）
	newUser := &domain.User{
		Name:     "newcomer",
		Password: "hashed",
		Detail:   domain.Detail{Email: "reuse@example.com"},
	}
	if err := db.Create(newUser).Error; err != nil {
		t.Errorf("注销后同邮箱重注册应成功（Q5），实际失败: %v", err)
	}

	// 未注销行之间：唯一性仍生效
	dup := &domain.User{
		Name:     "dup",
		Password: "hashed",
		Detail:   domain.Detail{Email: "reuse@example.com"},
	}
	err := db.Create(dup).Error
	if err == nil {
		t.Error("两个未注销账号同邮箱应被唯一索引拒绝")
	} else if !IsDuplicateKey(err) {
		t.Errorf("应是唯一索引冲突，实际: %v", err)
	}

	// 登录面查邮箱：命中的是新账号，不是已注销的老账号
	got, err := repo.GetByEmail(ctx, "reuse@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if got.ID == old.ID || got.Name != "newcomer" {
		t.Errorf("应命中新账号，实际 name=%q id=%d", got.Name, got.ID)
	}
}
