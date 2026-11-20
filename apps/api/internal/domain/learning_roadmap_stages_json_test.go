package domain

import (
	"encoding/json"
	"testing"
)

/**
 * stages 的 JSONB 读写（#223 线上事故回归）。
 *
 * 事故现场：管理端新建学习路线一律回 5000，且与内容无关。
 * 真实原因是 GORM 看到 RoadmapStages 是切片，按 PostgreSQL **数组**字面量
 * 写库（'{...}'），而 stages 列是 jsonb：
 *
 *   空切片 → 写 NULL        → SQLSTATE 23502（NOT NULL）
 *   非空   → 写 '{...}'     → SQLSTATE 23514（jsonb_typeof = 'array' 约束）
 *
 * 所以必须自己实现 driver.Valuer / sql.Scanner。这组测试盯的就是这两条：
 * 无论如何都不能让 stages 变成 NULL 或 PG 数组格式。
 */

// TestRoadmapStages_ValueIsAlwaysJSONArray 写库值必须是合法 JSON 数组。
//
// 空切片这条最关键：「新建一条还没加阶段就保存」是最常见的路径，
// 一旦写 NULL 就直接撞 NOT NULL，用户看到的是无从理解的 5000。
func TestRoadmapStages_ValueIsAlwaysJSONArray(t *testing.T) {
	cases := []struct {
		name   string
		stages RoadmapStages
		want   string
	}{
		{"nil 切片", nil, "[]"},
		{"空切片", RoadmapStages{}, "[]"},
		{"有阶段", RoadmapStages{{ID: "s_abc", Title: "阶段一", Resources: []RoadmapResource{}}},
			`[{"id":"s_abc","title":"阶段一","summary":"","resources":[]}]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.stages.Value()
			if err != nil {
				t.Fatalf("Value 不应报错，实际 %v", err)
			}
			s, ok := v.(string)
			if !ok {
				t.Fatalf("应返回 string（jsonb 文本），实际 %T", v)
			}
			if s == "" || s[0] != '[' {
				// '[' 开头才可能是 JSON 数组；'{' 是 PG 数组字面量，正是要避免的。
				t.Fatalf("必须是 JSON 数组文本，实际 %q", s)
			}
			var back any
			if err := json.Unmarshal([]byte(s), &back); err != nil {
				t.Fatalf("结果不是合法 JSON：%q", s)
			}
			if tc.name != "有阶段" {
				if s != tc.want {
					t.Fatalf("空值应编码为 %q，实际 %q", tc.want, s)
				}
			}
		})
	}
}

// TestRoadmapStages_ScanReadsJSON 读库要把 jsonb 文本还原成阶段列表。
func TestRoadmapStages_ScanReadsJSON(t *testing.T) {
	var got RoadmapStages
	raw := []byte(`[{"id":"s_abc","title":"阶段一","summary":"s","resources":[{"title":"r","url":"https://a.b"}]}]`)
	if err := got.Scan(raw); err != nil {
		t.Fatalf("Scan 不应报错，实际 %v", err)
	}
	if len(got) != 1 || got[0].ID != "s_abc" || got[0].Title != "阶段一" {
		t.Fatalf("还原结果不对：%+v", got)
	}
	if len(got[0].Resources) != 1 || got[0].Resources[0].URL != "https://a.b" {
		t.Fatalf("资源还原不对：%+v", got[0].Resources)
	}
}

// TestRoadmapStages_ScanToleratesNullAndEmpty 历史脏数据不该让整条路线打不开。
func TestRoadmapStages_ScanToleratesNullAndEmpty(t *testing.T) {
	for _, src := range []any{nil, []byte(""), ""} {
		var got RoadmapStages
		if err := got.Scan(src); err != nil {
			t.Fatalf("Scan(%#v) 不应报错，实际 %v", src, err)
		}
		if got == nil {
			t.Fatalf("Scan(%#v) 应得到空切片而不是 nil（避免再次写回 NULL）", src)
		}
		if len(got) != 0 {
			t.Fatalf("Scan(%#v) 应为空，实际 %+v", src, got)
		}
	}
}

// TestRoadmapStages_ValueScanRoundTrip 写进去再读回来要一致。
func TestRoadmapStages_ValueScanRoundTrip(t *testing.T) {
	original := RoadmapStages{
		{ID: "s_1", Title: "基础", Summary: "sum", Resources: []RoadmapResource{
			{Title: "教程", URL: "https://spring.io/guides"},
		}},
		{ID: "s_2", Title: "进阶", Resources: []RoadmapResource{}},
	}

	v, err := original.Value()
	if err != nil {
		t.Fatalf("Value 报错：%v", err)
	}
	var back RoadmapStages
	if err := back.Scan([]byte(v.(string))); err != nil {
		t.Fatalf("Scan 报错：%v", err)
	}

	// 逐字段比对，切片用 DeepEqual 会因 nil/空切片差异误报。
	if len(back) != len(original) {
		t.Fatalf("阶段数不一致：%d vs %d", len(back), len(original))
	}
	for i := range original {
		if back[i].ID != original[i].ID || back[i].Title != original[i].Title {
			t.Fatalf("第 %d 个阶段不一致：%+v vs %+v", i, back[i], original[i])
		}
		if len(back[i].Resources) != len(original[i].Resources) {
			t.Fatalf("第 %d 个阶段资源数不一致", i)
		}
	}
}
