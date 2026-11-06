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
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/migration"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// escapeLike 是论坛列表 ILIKE 的关键转义点（ADR 0003 决策 7）：
// 用户搜 "%" 或 "_" 时若不当通配符，必须转义，否则会匹配所有/任意单字符。
// #219 验收点「% 转义」落到这里（handler 透传原始串，repo 负责转义）。
func TestEscapeLikeEscapesWildcards(t *testing.T) {
	cases := map[string]string{
		"50%off":    "50\\%off",
		"a_b":        "a\\_b",
		`c\d`:        `c\\d`,
		"plain":      "plain",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Fatalf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

// repoTestDBForum 用**真实迁移**建库，而不是手写 DDL。
//
// 为什么这里必须跑迁移：`ListPosts` 的 SQL 直接写死了列名（`u.name` / `r.deleted_at`），
// 而列名的真源只有 migrations/*.sql。手写一份 CREATE TABLE 等于把这个 bug 的成因
// 再抄一遍——#219 的 `COALESCE(u.username, ...)` 正是引用了 users 表里不存在的列，
// 全套迁移建出来的库才会让这条 SQL 立刻报 `column u.username does not exist`。
// 这类「代码与 schema 漂移」只有拿真实 schema 跑才能锁住。
//
// 与其他 *_repo_test.go 的分工：它们关心索引/排序语义，用最小 DDL 即可；
// 这里关心的是「SQL 能不能在生产的表结构上执行」，必须走迁移。
// 未配置 DSN 时明确 skip，与 form_repo_test.go / user_repo_test.go 的既有约定一致。
func repoTestDBForum(t *testing.T) *gorm.DB {
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
	name := fmt.Sprintf("forum_repo_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatal(err)
	}

	// 清理紧跟建库注册，不能等到函数末尾：中间要跑全部迁移，是这条路径上
	// 最可能失败的一步，失败即 t.Fatalf 退出。注册晚了就会在宿主机上留下一个库
	// 和两条打开的连接（admin 与 sqlDB），而 t.Fatalf 之后没有任何代码再执行。
	// 两个句柄用可空变量接，避免「已建库但句柄还没开」时清理函数自己 panic。
	var sqlDB *sql.DB
	var db *gorm.DB
	t.Cleanup(func() {
		if db != nil {
			if gormSQL, err := db.DB(); err == nil {
				gormSQL.Close()
			}
		}
		if sqlDB != nil {
			sqlDB.Close()
		}
		if _, err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})

	u.Path = "/" + name
	sqlDB, err = sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := migration.Run(context.Background(), sqlDB, "up", nil); err != nil {
		t.Fatalf("跑迁移失败: %v", err)
	}

	db, err = gorm.Open(postgres.Open(u.String()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// TestListPosts_RealSchema 列表查询必须能在迁移建出的表结构上跑通。
//
// 回归 #219：`author_name` 取的列名写成了 `users.username`，而 users 表只有 `name`
// （domain.User.Name 映射到 users.name）。任何一次列表请求都在 SQL 层报错，
// 经 failInternal 转成 HTTP 500 + 业务码 5000，前端只看到「服务暂时不可用」。
func TestListPosts_RealSchema(t *testing.T) {
	db := repoTestDBForum(t)
	repo := NewForumRepo(db)
	ctx := context.Background()

	// 作者名与帖子都要真实存在，才能验证 JOIN 取到的是姓名而不是空串。
	if err := db.Exec(
		`INSERT INTO users (name, created_at, updated_at) VALUES (?, NOW(), NOW())`,
		"张三",
	).Error; err != nil {
		t.Fatalf("预置用户失败: %v", err)
	}
	var authorID int
	if err := db.Raw(`SELECT id FROM users WHERE name = ?`, "张三").Scan(&authorID).Error; err != nil {
		t.Fatalf("读取用户 id 失败: %v", err)
	}

	post := &domain.ForumPost{
		Scope:    domain.ForumScopeRecruitment,
		Title:    "怎么准备面试",
		AuthorID: authorID,
		Body:     "正文",
	}
	if err := repo.CreatePost(ctx, post); err != nil {
		t.Fatalf("预置帖子失败: %v", err)
	}

	items, total, err := repo.ListPosts(ctx, domain.ForumScopeRecruitment, ForumPostListFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("列表查询失败（SQL 与迁移 schema 不一致？）: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("应取回 1 条帖子，实际 total=%d len=%d", total, len(items))
	}
	if items[0].AuthorName != "张三" {
		t.Errorf("作者名应取 users.name，实际 %q", items[0].AuthorName)
	}
}
