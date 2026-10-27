package migration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/migrations"
	_ "github.com/lib/pq"
)

// These tests require a disposable PostgreSQL server with CREATEDB permission.
// Each case creates its own database; never point this at production.
func testDB(t *testing.T) *sql.DB {
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
	name := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE "` + name + `"`); err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return db
}

func execSQL(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func manual(t *testing.T, db *sql.DB, versions []uint) {
	t.Helper()
	for _, version := range versions {
		query, err := readUp(version)
		if err != nil {
			t.Fatal(err)
		}
		execSQL(t, db, string(query))
	}
}

func TestEmptyDatabaseAndRepeat(t *testing.T) {
	db := testDB(t)
	for range 2 {
		if err := Run(context.Background(), db, "up", nil); err != nil {
			t.Fatal(err)
		}
	}
	var version int
	var dirty bool
	if err := db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 12 || dirty {
		t.Fatalf("version=%d dirty=%v", version, dirty)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM recruitment_directions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("seed count=%d", count)
	}
	// 0010 的部分唯一索引必须真的落地，否则 #137 的约束形同虚设。
	var indexdef string
	if err := db.QueryRow(
		"SELECT indexdef FROM pg_indexes WHERE tablename='forms' AND indexname='idx_forms_open_uniq'",
	).Scan(&indexdef); err != nil {
		t.Fatalf("0010 的唯一索引未创建: %v", err)
	}
	if !strings.Contains(indexdef, "UNIQUE") || !strings.Contains(indexdef, "is_open") {
		t.Fatalf("索引应为 is_open 谓词上的部分唯一索引，实际 %q", indexdef)
	}
	// 0011 的邮箱唯一索引谓词必须带 deleted_at IS NULL（issue #142 Q5），
	// 否则注销释放邮箱的语义形同虚设——重注册会撞老行的唯一约束。
	var emailIndexDef string
	if err := db.QueryRow(
		"SELECT indexdef FROM pg_indexes WHERE tablename='users' AND indexname='uniq_users_detail_email'",
	).Scan(&emailIndexDef); err != nil {
		t.Fatalf("0011 的邮箱唯一索引未创建: %v", err)
	}
	if !strings.Contains(emailIndexDef, "deleted_at") {
		t.Fatalf("邮箱唯一索引谓词应含 deleted_at IS NULL（#142 Q5），实际 %q", emailIndexDef)
	}
	// 0012 的冻结三列必须真的落地：扫描任务与登录拦截都按列名读写，
	// 缺列会让「定时冻结」在运行时直接报 SQLSTATE 42703。
	for _, column := range []string{"frozen_at", "freeze_reason", "freeze_notified_at"} {
		var dataType string
		if err := db.QueryRow(
			`SELECT data_type FROM information_schema.columns
			 WHERE table_name='users' AND column_name=$1`, column,
		).Scan(&dataType); err != nil {
			t.Fatalf("0012 的 %s 列未创建: %v", column, err)
		}
	}
	// 扫描主查询「未冻结 + 未注销 + 注册早于 N 天」需要部分索引支撑，
	// 否则用户表增长后会退化成全表扫描。
	var freezeIndexDef string
	if err := db.QueryRow(
		"SELECT indexdef FROM pg_indexes WHERE tablename='users' AND indexname='idx_users_active_created_at'",
	).Scan(&freezeIndexDef); err != nil {
		t.Fatalf("0012 的部分索引未创建: %v", err)
	}
	if !strings.Contains(freezeIndexDef, "frozen_at IS NULL") || !strings.Contains(freezeIndexDef, "deleted_at IS NULL") {
		t.Fatalf("索引谓词应同时含 frozen_at IS NULL 与 deleted_at IS NULL，实际 %q", freezeIndexDef)
	}
}

func TestAdoptNonContiguousPreservesDataAndDDL(t *testing.T) {
	db := testDB(t)
	applied := []uint{1, 2, 3, 4, 5, 7, 9}
	manual(t, db, applied)
	execSQL(t, db, "INSERT INTO users (name, detail_email) VALUES ('existing','existing@example.test'); UPDATE recruitment_rounds SET name='custom' WHERE round_num=1")
	var before uint32
	if err := db.QueryRow("SELECT 'idx_mail_models_type'::regclass::oid").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db, "up", nil); err == nil {
		t.Fatal("unmanaged database accepted")
	}
	if err := Run(context.Background(), db, "check", applied); err != nil {
		t.Fatal(err)
	}
	var ledger bool
	if err := db.QueryRow("SELECT to_regclass('public.migration_adoptions') IS NOT NULL").Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger {
		t.Fatal("check wrote adoption ledger")
	}
	if err := Run(context.Background(), db, "adopt", applied); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Run(context.Background(), db, "up", nil); err != nil {
			t.Fatal(err)
		}
	}
	var after uint32
	if err := db.QueryRow("SELECT 'idx_mail_models_type'::regclass::oid").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("adopted DDL was replayed")
	}
	var name, qq string
	if err := db.QueryRow("SELECT name, coalesce(detail_qq,'') FROM users WHERE detail_email='existing@example.test'").Scan(&name, &qq); err != nil {
		t.Fatal(err)
	}
	if name != "existing" || qq != "" {
		t.Fatal("existing data changed")
	}
	if err := db.QueryRow("SELECT name FROM recruitment_rounds WHERE round_num=1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "custom" {
		t.Fatal("seed data was replayed")
	}
}

func TestAdoptRejectsWrongManifestAndDrift(t *testing.T) {
	for _, scenario := range []string{"missing6", "wrongtype", "wrongindex", "partial3"} {
		t.Run(scenario, func(t *testing.T) {
			db := testDB(t)
			actual := []uint{1, 2, 3, 4, 5, 7, 8, 9}
			manual(t, db, actual)
			claimed := append([]uint{}, actual...)
			switch scenario {
			case "missing6":
				claimed = []uint{1, 2, 3, 4, 5, 6, 7, 8, 9}
			case "wrongtype":
				execSQL(t, db, "ALTER TABLE users ALTER COLUMN detail_student_id TYPE text")
			case "wrongindex":
				execSQL(t, db, "DROP INDEX idx_forms_share_token; CREATE INDEX idx_forms_share_token ON forms(share_token)")
			case "partial3":
				execSQL(t, db, `ALTER TABLE mails RENAME COLUMN to_user_id TO "to"`)
			}
			if err := Run(context.Background(), db, "adopt", claimed); err == nil {
				t.Fatal("unsafe baseline accepted")
			}
			var exists bool
			if err := db.QueryRow("SELECT to_regclass('public.migration_adoptions') IS NOT NULL").Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists {
				t.Fatal("failed adoption left metadata")
			}
		})
	}
}

func TestMigrationFailureAndDirtyRetry(t *testing.T) {
	db := testDB(t)
	applied := []uint{1, 2, 3, 4}
	manual(t, db, applied)
	execSQL(t, db, "INSERT INTO users(name,detail_email) VALUES ('a','duplicate@example.test'),('b','duplicate@example.test')")
	if err := Run(context.Background(), db, "adopt", applied); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db, "up", nil); err == nil {
		t.Fatal("duplicate email migration succeeded")
	}
	var dirty bool
	if err := db.QueryRow("SELECT dirty FROM schema_migrations").Scan(&dirty); err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatal("failure not marked dirty")
	}
	if err := Run(context.Background(), db, "up", nil); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("retry=%v", err)
	}
}

func TestManifestValidation(t *testing.T) {
	for _, input := range []string{"", "1,1", "0", "10", "1,x", "1,3", "6"} {
		if _, err := ParseApplied(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	versions, err := ParseApplied("1,2,3,4,5,7,9")
	if err != nil || len(versions) != 7 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
}

func TestAdoptOnlyRecruitmentAndForms(t *testing.T) {
	db := testDB(t)
	applied := []uint{7, 9}
	manual(t, db, applied)
	if err := Run(context.Background(), db, "adopt", applied); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db, "up", nil); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentOperationRejected(t *testing.T) {
	db := testDB(t)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock(107, 20261014)"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock(107, 20261014)")
	if err := Run(context.Background(), db, "up", nil); err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("concurrent run: %v", err)
	}
}

func TestAdoptionChecksumChangeRejected(t *testing.T) {
	db := testDB(t)
	manual(t, db, []uint{1})
	if err := Run(context.Background(), db, "adopt", []uint{1}); err != nil {
		t.Fatal(err)
	}
	execSQL(t, db, "UPDATE migration_adoptions SET checksum='changed'")
	if err := Run(context.Background(), db, "up", nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum guard: %v", err)
	}
}

func TestAdoptRejectsIncompleteRecruitmentSeeds(t *testing.T) {
	for _, scenario := range []string{"no_seeds", "missing_direction"} {
		t.Run(scenario, func(t *testing.T) {
			db := testDB(t)
			if scenario == "no_seeds" {
				query, err := readUp(7)
				if err != nil {
					t.Fatal(err)
				}
				// Reproduce manually running the DDL but omitting the seed INSERTs.
				prefix := strings.SplitN(string(query), "INSERT INTO recruitment_rounds", 2)[0]
				execSQL(t, db, prefix+"COMMIT;")
			} else {
				manual(t, db, []uint{7})
				execSQL(t, db, "DELETE FROM recruitment_directions WHERE code='agent'")
			}
			if err := Run(context.Background(), db, "adopt", []uint{7}); err == nil {
				t.Fatal("incomplete seed state accepted")
			}
		})
	}
}

/* ------------------------------------------------------------------ *
 * 0010：同轮同方向至多一张开放表单（issue #137）
 * ------------------------------------------------------------------ */

// 空库 up 到 0010 后，约束必须真的挡住"同轮同方向第二张开放表单"，
// 同时不误伤历史（关闭）表单 —— 后者是 0010 选部分索引而不是普通唯一索引的理由。
func TestFormsOpenUniqueConstraint(t *testing.T) {
	db := testDB(t)
	if err := Run(context.Background(), db, "up", nil); err != nil {
		t.Fatal(err)
	}

	execSQL(t, db, `INSERT INTO forms (title, round_id, direction_code, share_token, is_open)
		VALUES ('开放表', 1, 'front-end', 'tok-open', true)`)

	// 同轮同方向第二张开放表单：必须被约束拒绝
	_, err := db.Exec(`INSERT INTO forms (title, round_id, direction_code, share_token, is_open)
		VALUES ('第二张开放表', 1, 'front-end', 'tok-second', true)`)
	if err == nil {
		t.Fatal("同轮同方向第二张开放表单未被约束拒绝")
	}

	// 同轮同方向多张关闭表单可以共存（历史表单不被堵死）
	for _, token := range []string{"tok-closed-1", "tok-closed-2"} {
		if _, err := db.Exec(`INSERT INTO forms (title, round_id, direction_code, share_token, is_open)
			VALUES ('历史表', 1, 'front-end', $1, false)`, token); err != nil {
			t.Fatalf("关闭的历史表单应可共存，实际: %v", err)
		}
	}

	// 别的方向、别的轮次各自可以有一张开放表（约束是 per 方向 per 轮次）
	execSQL(t, db, `INSERT INTO forms (title, round_id, direction_code, share_token, is_open)
		VALUES ('后端开放表', 1, 'back-end', 'tok-be', true)`)
	execSQL(t, db, `INSERT INTO forms (title, round_id, direction_code, share_token, is_open)
		VALUES ('下一轮开放表', 2, 'front-end', 'tok-round2', true)`)

	// 把已开放的那张关掉，同方向就能再开放另一张（关闭永不受约束影响）
	execSQL(t, db, `UPDATE forms SET is_open = false WHERE share_token = 'tok-open'`)
	execSQL(t, db, `UPDATE forms SET is_open = true WHERE share_token = 'tok-closed-1'`)
}

// D1 的核心：库里已有重复开放行时，up 必须报错退出，且**不改数据、不建索引**。
// 人工清理后重跑才成功 —— 脚本绝不替业务决定保留哪张。
func TestFormsOpenUniqueRejectsExistingDuplicates(t *testing.T) {
	db := testDB(t)
	// 手工应用到 0009（0010 之前），此时还没有约束
	manual(t, db, []uint{1, 2, 3, 4, 5, 6, 7, 8, 9})

	// 制造坏数据：同轮同方向两张开放表单（在 0010 之前这是允许的）
	execSQL(t, db, `INSERT INTO forms (title, round_id, direction_code, share_token, is_open)
		VALUES ('坏表 A', 1, 'front-end', 'tok-a', true),
		       ('坏表 B', 1, 'front-end', 'tok-b', true)`)

	// 0009 已被手工执行，必须走 adopt 才能让 up 接手后续版本
	if err := Run(context.Background(), db, "adopt", []uint{1, 2, 3, 4, 5, 6, 7, 8, 9}); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), db, "up", nil); err == nil {
		t.Fatal("存在重复开放表单时 up 应当失败")
	}

	// 零副作用：两张坏数据都还在，索引没建出来
	var count int
	if err := db.QueryRow("SELECT count(*) FROM forms WHERE is_open").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("脚本不应修改存量数据，开放表单应仍为 2 张，实际 %d", count)
	}
	var exists bool
	if err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename='forms' AND indexname='idx_forms_open_uniq')",
	).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("失败时不应留下索引")
	}

	// 人工清理（保留一张、关闭另一张）后重跑应当成功
	execSQL(t, db, `UPDATE forms SET is_open = false WHERE share_token = 'tok-b'`)
	// 失败会留下 dirty，先按维护者手册修复元数据（等价于人工确认最后成功版本）
	execSQL(t, db, `UPDATE schema_migrations SET version = 9, dirty = false`)
	if err := Run(context.Background(), db, "up", nil); err != nil {
		t.Fatalf("清理后重跑应当成功: %v", err)
	}
	if err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename='forms' AND indexname='idx_forms_open_uniq')",
	).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("清理后重跑应当建出索引")
	}
}

// TestMigrationVersionsUnique 不需要数据库：只读嵌入的 SQL 文件名。
//
// golang-migrate 在源初始化阶段就以「duplicate migration file」直接失败，
// 表现是服务起不来、迁移工具跑不动、镜像冒烟测试红。这类错误必须在
// 提交前就被挡住——曾经有一次把另一个分支未跟踪的 0011_*.up.sql 带进
// 主干，直到部署阶段才暴露（阻断发布）。
func TestMigrationVersionsUnique(t *testing.T) {
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
			continue // README / embed.go 等
		}
		version := strings.SplitN(name, "_", 2)[0]
		// 只对「同版本同方向」判重：up/down 成对是正常的。
		direction := "up"
		if strings.HasSuffix(name, ".down.sql") {
			direction = "down"
		}
		key := version + "." + direction
		if previous, ok := seen[key]; ok {
			t.Fatalf("迁移版本重复：%s 与 %s（同版本同方向只能有一个文件）", previous, name)
		}
		seen[key] = name
	}

	if len(seen) == 0 {
		t.Fatal("没有发现任何迁移文件，嵌入目录可能配置错了")
	}
}
