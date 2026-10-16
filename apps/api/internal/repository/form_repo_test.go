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

// 验收标准：同轮同方向多张表单时，按「is_open 且 updated_at 最新」稳定取一张。
func TestListOpenFormsByRound_PicksNewestPerDirection(t *testing.T) {
	db := repoTestDB(t)
	repo := NewFormRepo(db)
	ctx := context.Background()

	base := time.Now().Add(-time.Hour)
	seed := []*domain.Form{
		{Title: "旧表", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-old", IsOpen: true, UpdatedAt: base},
		{Title: "新表", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-new", IsOpen: true, UpdatedAt: base.Add(time.Minute)},
		{Title: "关闭的表", RoundID: 1, DirectionCode: "back-end", ShareToken: "t-closed", IsOpen: false, UpdatedAt: base.Add(2 * time.Minute)},
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

	// 仓储返回的是「本轮全部开放表」的有序列表（去重是服务层 OpenFormsByRound 的事）：
	// front-end 两张开放（old/new），back-end 关闭，round 2 不属于本轮。
	if len(forms) != 2 {
		t.Fatalf("本轮应有 2 张开放表，实际 %d 张", len(forms))
	}
	// 排序保证同方向第一张就是该方向该用的那张：updated_at 新的在前
	if forms[0].ShareToken != "t-new" {
		t.Errorf("同方向最新的一张应排在最前（t-new），实际 %q", forms[0].ShareToken)
	}
	for _, f := range forms {
		if f.DirectionCode != "front-end" {
			t.Errorf("结果不应包含其他方向的开放表，实际含 %q", f.DirectionCode)
		}
		if !f.IsOpen {
			t.Error("结果不应包含关闭的表")
		}
	}
}

// D1 的关键：updated_at 完全相同（批量建表同一秒）时靠 id DESC 兜底，结果必须稳定。
func TestListOpenFormsByRound_TieBreaksByIdDesc(t *testing.T) {
	db := repoTestDB(t)
	repo := NewFormRepo(db)
	ctx := context.Background()

	same := time.Now().Add(-time.Minute)
	first := &domain.Form{Title: "先建", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-1", IsOpen: true, UpdatedAt: same}
	second := &domain.Form{Title: "后建", RoundID: 1, DirectionCode: "front-end", ShareToken: "t-2", IsOpen: true, UpdatedAt: same}
	for _, f := range []*domain.Form{first, second} {
		if err := db.Create(f).Error; err != nil {
			t.Fatalf("预置表单失败: %v", err)
		}
	}
	if second.ID <= first.ID {
		t.Fatalf("前提不成立：后建的 id 应更大（%d vs %d）", second.ID, first.ID)
	}

	forms, err := repo.ListOpenFormsByRound(ctx, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(forms) != 2 {
		t.Fatalf("应有 2 张开放表，实际 %d 张", len(forms))
	}
	// id DESC 兜底：同 updated_at 时 id 大的排前面
	if forms[0].ShareToken != "t-2" {
		t.Errorf("updated_at 打平时应按 id DESC 兜底（t-2 在前），实际 %q", forms[0].ShareToken)
	}

	// 重复查询结果稳定
	for i := 0; i < 3; i++ {
		again, err := repo.ListOpenFormsByRound(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if again[0].ShareToken != "t-2" {
			t.Fatalf("第 %d 次查询结果漂移：%q", i+2, again[0].ShareToken)
		}
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
