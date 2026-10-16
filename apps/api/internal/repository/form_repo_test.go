package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 仓储层用例跑真实 PostgreSQL（#135 §7 要求仓储测试覆盖命中 / 未命中）。
//
// 为什么不用桩：本 issue 的取表规则靠 SQL 的 ORDER BY 表达（is_open 过滤 +
// updated_at DESC, id DESC），桩只能复刻我自己的排序函数，验证不到真正的 SQL。
// 未配置 DSN 时明确 skip，与 internal/migration 的既有约定一致。
func repoTestDB(t *testing.T) *gorm.DB {
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

	// 只建本用例用到的两张表：够验证 SQL，不必跑整套迁移。
	ddl := []string{
		`CREATE TABLE forms (
			id bigserial PRIMARY KEY,
			title text NOT NULL,
			description text,
			round_id bigint NOT NULL,
			direction_code text NOT NULL,
			share_token text NOT NULL,
			is_open boolean NOT NULL DEFAULT false,
			created_at timestamptz,
			updated_at timestamptz
		)`,
		`CREATE UNIQUE INDEX idx_forms_share_token ON forms (share_token)`,
		// 与迁移 0010 一致：同轮同方向至多一张开放表单（#137）。
		//
		// 必须在这里复刻：否则仓储测试跑在一个比生产宽松的 schema 上，
		// 「多张开放表单」这种生产里不可能出现的状态会被测成合法，
		// 而 #135 的排序用例恰好需要构造它来验证取表规则。
		`CREATE UNIQUE INDEX idx_forms_open_uniq ON forms (round_id, direction_code) WHERE is_open`,
		`CREATE TABLE recruitment_applications (
			id bigserial PRIMARY KEY,
			user_id bigint NOT NULL,
			round_id bigint NOT NULL,
			direction_code text NOT NULL,
			status text NOT NULL DEFAULT 'applied',
			name text, email text, class text, student_id bigint, qq text, note text,
			created_at timestamptz,
			updated_at timestamptz
		)`,
		// 与迁移 0007 一致：三元组唯一约束（#135 复用路径的前提）
		`CREATE UNIQUE INDEX idx_recruitment_apps_user_round_direction
			ON recruitment_applications (user_id, round_id, direction_code)`,
	}
	for _, stmt := range ddl {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}
	return db
}

/* ------------------------------------------------------------------ *
 * GetApplicationByUserRoundDirection（#135 变更二的查询）
 * ------------------------------------------------------------------ */

func TestGetApplicationByUserRoundDirection_Hit(t *testing.T) {
	db := repoTestDB(t)
	repo := NewRecruitmentRepo(db)
	ctx := context.Background()

	want := &domain.Application{
		UserID: 7, RoundID: 1, DirectionCode: "front-end",
		Status: domain.ApplicationApplied, Name: "张三",
	}
	if err := repo.CreateApplication(ctx, want); err != nil {
		t.Fatalf("预置申请失败: %v", err)
	}

	got, err := repo.GetApplicationByUserRoundDirection(ctx, 7, 1, "front-end")
	if err != nil {
		t.Fatalf("应命中申请，实际: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("应取回同一条申请：want=%d got=%d", want.ID, got.ID)
	}
	if got.Status != domain.ApplicationApplied {
		t.Errorf("状态应为 applied，实际 %q", got.Status)
	}
}

// 三元组必须精确匹配：任一维度不同都不能命中，否则复用逻辑会认错申请。
func TestGetApplicationByUserRoundDirection_Miss(t *testing.T) {
	db := repoTestDB(t)
	repo := NewRecruitmentRepo(db)
	ctx := context.Background()

	if err := repo.CreateApplication(ctx, &domain.Application{
		UserID: 7, RoundID: 1, DirectionCode: "front-end",
		Status: domain.ApplicationApplied,
	}); err != nil {
		t.Fatalf("预置申请失败: %v", err)
	}

	cases := []struct {
		name          string
		userID        int
		roundID       int
		directionCode string
	}{
		{"别的用户", 8, 1, "front-end"},
		{"别的轮次", 7, 2, "front-end"},
		{"别的方向", 7, 1, "back-end"},
		{"带空格的方向 code", 7, 1, "front-end "}, // 归一化后应命中，见下一个用例
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.GetApplicationByUserRoundDirection(ctx, tc.userID, tc.roundID, tc.directionCode)
			if tc.name == "带空格的方向 code" {
				// 实现会 TrimSpace，因此这一条应当命中（防"同方向两种写法"导致漏查）
				if err != nil {
					t.Errorf("TrimSpace 后应命中，实际: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrApplicationNotFound) {
				t.Errorf("不匹配的三元组应返回 ErrApplicationNotFound，实际: %v", err)
			}
		})
	}
}

/* ------------------------------------------------------------------ *
 * ListOpenFormsByRound（#135 变更一的取表规则）
 * ------------------------------------------------------------------ */

// 取表规则在 #137 之后的形态：只返回本轮的开放表，且排序确定。
//
// ⚠️ 与 #135 的差异（#137 引入）：#135 原本用「同方向两张开放表」验证
// updated_at DESC 取最新。0010 的部分唯一索引让那个状态在生产里**不可能出现**
// （同轮同方向至多一张开放表），所以这里只验证仍然可达的行为 —— 过滤与排序。
// 排序里的 updated_at DESC / id DESC 兜底保留在实现中（跨方向排序仍需要它，
// 且它是 #135 D1 的既定决策），但同方向的那一段已是防御性代码：
// 见 TestOpenFormsPerDirectionNowUnique。
func TestListOpenFormsByRound_FiltersAndOrders(t *testing.T) {
	db := repoTestDB(t)
	repo := NewFormRepo(db)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	seed := []*domain.Form{
		{Title: "前端开放", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-fe", IsOpen: true, UpdatedAt: base},
		{Title: "后端开放", RoundID: 1, DirectionCode: "back-end", ShareToken: "t-be", IsOpen: true, UpdatedAt: base.Add(time.Minute)},
		{Title: "关闭的表", RoundID: 1, DirectionCode: "agent", ShareToken: "t-closed", IsOpen: false, UpdatedAt: base.Add(2 * time.Minute)},
		{Title: "下一轮的表", RoundID: 2, DirectionCode: "front-end", ShareToken: "t-round2", IsOpen: true, UpdatedAt: base.Add(3 * time.Minute)},
	}
	for _, f := range seed {
		if err := db.Create(f).Error; err != nil {
			t.Fatalf("预置表单失败: %v", err)
		}
	}

	forms, err := repo.ListOpenFormsByRound(ctx, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	// 本轮两张开放表（前端、后端各一）；agent 关闭、round 2 不属于本轮。
	if len(forms) != 2 {
		t.Fatalf("本轮应有 2 张开放表，实际 %d 张", len(forms))
	}
	for _, f := range forms {
		if !f.IsOpen {
			t.Error("结果不应包含关闭的表")
		}
		if f.RoundID != 1 {
			t.Errorf("结果不应包含其他轮次的表，实际 round_id=%d", f.RoundID)
		}
	}
	// 主排序是 direction_code ASC：调用方按方向取第一张，顺序必须确定。
	if forms[0].DirectionCode != "back-end" || forms[1].DirectionCode != "front-end" {
		t.Errorf("应按 direction_code ASC 排序，实际 %q, %q",
			forms[0].DirectionCode, forms[1].DirectionCode)
	}

	// 重复查询结果稳定（排序不随查询计划漂移）
	for i := 0; i < 3; i++ {
		again, err := repo.ListOpenFormsByRound(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(again) != 2 || again[0].ShareToken != forms[0].ShareToken {
			t.Fatalf("第 %d 次查询结果漂移：%q", i+2, again[0].ShareToken)
		}
	}
}

// #137 的核心：同一轮同一方向至多一张开放表单。
//
// 本测试同时守住两件事：
//  1. 仓储测试的 schema 与生产一致（复刻了 0010 的部分唯一索引）——
//     否则「多张开放表单」这种生产不可能的状态会被测成合法；
//  2. #135 那个「同方向比 updated_at」的场景确实已不可构造。
func TestOpenFormsPerDirectionNowUnique(t *testing.T) {
	db := repoTestDB(t)
	repo := NewFormRepo(db)
	ctx := context.Background()

	first := &domain.Form{Title: "先开放", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-1", IsOpen: true}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("第一张开放表应当写入成功: %v", err)
	}

	// 同轮同方向第二张开放表：必须被 0010 的约束拒绝
	second := &domain.Form{Title: "后开放", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-2", IsOpen: true}
	if err := db.Create(second).Error; err == nil {
		t.Fatal("同轮同方向第二张开放表未被约束拒绝 —— 仓储 schema 与迁移 0010 不一致")
	}

	// 关闭的历史表单仍可共存（部分索引只覆盖 is_open = true）
	for _, token := range []string{"t-c1", "t-c2"} {
		if err := db.Create(&domain.Form{
			Title: "历史表", RoundID: 1, DirectionCode: "front-end", ShareToken: token, IsOpen: false,
		}).Error; err != nil {
			t.Fatalf("关闭的历史表单应可共存，实际: %v", err)
		}
	}

	// 因此该方向的开放表恰好只有一张 —— 取表规则在此方向上无歧义。
	forms, err := repo.ListOpenFormsByRound(ctx, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(forms) != 1 || forms[0].ShareToken != "t-1" {
		t.Fatalf("该方向应恰有一张开放表 t-1，实际 %d 张", len(forms))
	}
}

// 仓储层把约束冲突翻译成 ErrFormAlreadyOpen（#137）：
// 并发下服务层的前置检查会双双通过，最终靠这条翻译给出可读错误。
func TestUpdateForm_TranslatesOpenConflict(t *testing.T) {
	db := repoTestDB(t)
	repo := NewFormRepo(db)
	ctx := context.Background()

	open := &domain.Form{Title: "已开放", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-open", IsOpen: true}
	if err := db.Create(open).Error; err != nil {
		t.Fatal(err)
	}
	other := &domain.Form{Title: "待开放", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-other", IsOpen: false}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}

	// 直接走仓储（绕过服务层前置检查）模拟并发窗口里输掉的那个事务
	other.IsOpen = true
	err := repo.UpdateForm(ctx, other, nil)
	if !errors.Is(err, ErrFormAlreadyOpen) {
		t.Fatalf("应翻译为 ErrFormAlreadyOpen，实际: %v", err)
	}

	// 零副作用：另一张仍是关闭状态
	var reloaded domain.Form
	if err := db.First(&reloaded, other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.IsOpen {
		t.Error("失败时不应写入 is_open")
	}

	// 关闭操作永不冲突
	open.IsOpen = false
	if err := repo.UpdateForm(ctx, open, nil); err != nil {
		t.Fatalf("关闭表单不应受约束影响: %v", err)
	}
}

// 无开放表单 → 空结果且不报错（handler 据此把 form 置 null）。
func TestListOpenFormsByRound_NoOpenForms(t *testing.T) {
	db := repoTestDB(t)
	repo := NewFormRepo(db)
	ctx := context.Background()

	if err := db.Create(&domain.Form{
		Title: "关掉的", RoundID: 1, DirectionCode: "front-end", ShareToken: "t", IsOpen: false,
	}).Error; err != nil {
		t.Fatalf("预置表单失败: %v", err)
	}

	forms, err := repo.ListOpenFormsByRound(ctx, 1)
	if err != nil {
		t.Fatalf("无开放表单不应报错，实际: %v", err)
	}
	if len(forms) != 0 {
		t.Errorf("应返回空结果，实际 %d 张", len(forms))
	}
}
