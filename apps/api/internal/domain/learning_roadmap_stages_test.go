package domain

import (
	"errors"
	"strings"
	"testing"
)

/* ------------------------------------------------------------------ *
 * #223：阶段 id 对齐（AC：有 id 则保留、无 id 则生成、旧有新无则丢弃）
 * ------------------------------------------------------------------ */

func stage(id, title string) RoadmapStage {
	return RoadmapStage{ID: id, Title: title}
}

func TestNewRoadmapStageID_Shape(t *testing.T) {
	id := NewRoadmapStageID()
	if !strings.HasPrefix(id, "s_") {
		t.Fatalf("id 应以 s_ 开头，得到 %q", id)
	}
	if len(id) != 10 {
		t.Fatalf("id 应为 s_ + 8 位，得到 %q（长度 %d）", id, len(id))
	}
	if !IsValidRoadmapStageID(id) {
		t.Fatalf("生成的 id 应通过形状校验：%q", id)
	}
}

func TestNewRoadmapStageID_Unique(t *testing.T) {
	// 随机 id 的价值就在不重复：两次生成撞车说明实现有问题。
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewRoadmapStageID()
		if seen[id] {
			t.Fatalf("1000 次生成出现重复 id: %q", id)
		}
		seen[id] = true
	}
}

func TestIsValidRoadmapStageID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"s_abc12xyz", true},
		{"s_", false},          // 太短
		{"s_abc12xy", false},   // 少一位
		{"s_abc12xyzz", false}, // 多一位
		{"x_abc12xyz", false},  // 前缀错
		{"s_abc12xyZ", false},  // 大写不在 base36 小写表里
		{"", false},
	}
	for _, c := range cases {
		if got := IsValidRoadmapStageID(c.id); got != c.want {
			t.Errorf("IsValidRoadmapStageID(%q) = %v，期望 %v", c.id, got, c.want)
		}
	}
}

func TestAlignStageIDs_KeepsKnownIDs(t *testing.T) {
	prev := RoadmapStages{stage("s_aaaa1111", "旧一"), stage("s_bbbb2222", "旧二")}
	next := RoadmapStages{stage("s_aaaa1111", "旧一"), stage("s_bbbb2222", "旧二改")}

	got := AlignStageIDs(next, prev)
	if got[0].ID != "s_aaaa1111" || got[1].ID != "s_bbbb2222" {
		t.Fatalf("已有 id 应保留，得到 %+v", got)
	}
	if got[1].Title != "旧二改" {
		t.Fatalf("标题应更新，得到 %q", got[1].Title)
	}
}

func TestAlignStageIDs_GeneratesForNewStage(t *testing.T) {
	prev := RoadmapStages{stage("s_aaaa1111", "旧一")}
	next := RoadmapStages{stage("s_aaaa1111", "旧一"), stage("", "新增阶段")}

	got := AlignStageIDs(next, prev)
	if got[0].ID != "s_aaaa1111" {
		t.Fatalf("原阶段 id 应保留，得到 %q", got[0].ID)
	}
	if got[1].ID == "" || !IsValidRoadmapStageID(got[1].ID) {
		t.Fatalf("新阶段应拿到合法 id，得到 %q", got[1].ID)
	}
	if got[1].ID == got[0].ID {
		t.Fatal("新阶段 id 不能与原阶段相同")
	}
}

func TestAlignStageIDs_RejectsForgedID(t *testing.T) {
	// 前端伪造一个形状合法但库里没有的 id：不能保留，否则会造出
	// 「指向不存在阶段」的进度锚点。按新阶段处理是安全的。
	prev := RoadmapStages{stage("s_aaaa1111", "旧一")}
	next := RoadmapStages{stage("s_9999zzzz", "伪造")}

	got := AlignStageIDs(next, prev)
	if got[0].ID == "s_9999zzzz" {
		t.Fatal("库里不存在的 id 不应被保留")
	}
	if !IsValidRoadmapStageID(got[0].ID) {
		t.Fatalf("应改发合法新 id，得到 %q", got[0].ID)
	}
}

func TestAlignStageIDs_DropsRemovedStages(t *testing.T) {
	// 「旧有新无则丢弃」：被删掉的阶段不再出现在结果里。
	prev := RoadmapStages{stage("s_aaaa1111", "一"), stage("s_bbbb2222", "二")}
	next := RoadmapStages{stage("s_bbbb2222", "二")}

	got := AlignStageIDs(next, prev)
	if len(got) != 1 || got[0].ID != "s_bbbb2222" {
		t.Fatalf("应只剩保留的那一个，得到 %+v", got)
	}
}

func TestAlignStageIDs_DuplicateIDsGetNewOne(t *testing.T) {
	// 同一次提交出现重复 id：后一个改发新 id，避免同一路线内两个同号阶段。
	prev := RoadmapStages{stage("s_aaaa1111", "一")}
	next := RoadmapStages{stage("s_aaaa1111", "一"), stage("s_aaaa1111", "复制的一")}

	got := AlignStageIDs(next, prev)
	if got[0].ID != got[1].ID {
		return // 已分开，符合预期
	}
	t.Fatalf("重复 id 应被拆开，得到 %+v", got)
}

/* ------------------------------------------------------------------ *
 * 边界校验（AC：阶段最多 20、每阶段资源最多 20，超限给可读错误）
 * ------------------------------------------------------------------ */

func makeStages(n int) RoadmapStages {
	out := make(RoadmapStages, n)
	for i := range out {
		out[i] = stage("", "阶段")
	}
	return out
}

func TestValidateRoadmapStages_StageLimit(t *testing.T) {
	if err := ValidateRoadmapStages(makeStages(20)); err != nil {
		t.Fatalf("正好 20 个阶段应通过，得到 %v", err)
	}

	err := ValidateRoadmapStages(makeStages(21))
	if err == nil {
		t.Fatal("21 个阶段应被拒绝")
	}
	if !errors.Is(err, ErrRoadmapInvalid) {
		t.Fatalf("应能匹配 ErrRoadmapInvalid，得到 %v", err)
	}
	if !strings.Contains(err.Error(), "最多 20 个") {
		t.Fatalf("错误应可读地说明上限，得到 %q", err.Error())
	}
}

func TestValidateRoadmapStages_ResourceLimit(t *testing.T) {
	withResources := func(n int) RoadmapStages {
		res := make([]RoadmapResource, n)
		for i := range res {
			res[i] = RoadmapResource{Title: "r", URL: "https://example.com"}
		}
		return RoadmapStages{{Title: "阶段", Resources: res}}
	}
	if err := ValidateRoadmapStages(withResources(20)); err != nil {
		t.Fatalf("正好 20 个资源应通过，得到 %v", err)
	}

	err := ValidateRoadmapStages(withResources(21))
	if err == nil || !strings.Contains(err.Error(), "最多 20 个资源") {
		t.Fatalf("21 个资源应被拒绝且说明上限，得到 %v", err)
	}
}

func TestValidateRoadmapStages_MissingTitle(t *testing.T) {
	err := ValidateRoadmapStages(RoadmapStages{{Title: "  "}})
	if err == nil || !strings.Contains(err.Error(), "缺少标题") {
		t.Fatalf("空标题应被拒绝，得到 %v", err)
	}
	// 报错定位用「第 N 个阶段」而不是 id：id 对管理员是不可见的技术细节。
	if !strings.Contains(err.Error(), "第 1 个阶段") {
		t.Fatalf("应指明位置，得到 %q", err.Error())
	}
}

/* ------------------------------------------------------------------ *
 * 资源链接协议白名单（AC：不合规拒绝保存）
 * ------------------------------------------------------------------ */

func TestIsAllowedResourceURL(t *testing.T) {
	// 与前端 protocolAllowlist.ts 同口径。
	cases := []struct {
		url  string
		want bool
	}{
		{"https://example.com/a", true},
		{"http://example.com", true},
		{"/user/qa", true},
		{"https://example.com/a?b=1#c", true},
		{"javascript:alert(1)", false},
		{"data:text/html,<script>", false},
		{"file:///etc/passwd", false},
		{"//evil.com/x", false}, // 协议相对：会跳到外部站点
		{"mailto:a@b.com", false},
		{"", false},
		{"   ", false},
		{"not a url", false},
	}
	for _, c := range cases {
		if got := IsAllowedResourceURL(c.url); got != c.want {
			t.Errorf("IsAllowedResourceURL(%q) = %v，期望 %v", c.url, got, c.want)
		}
	}
}

func TestValidateRoadmapStages_RejectsBadURL(t *testing.T) {
	stages := RoadmapStages{{
		Title:     "阶段",
		Resources: []RoadmapResource{{Title: "危险链接", URL: "javascript:alert(1)"}},
	}}
	err := ValidateRoadmapStages(stages)
	if err == nil {
		t.Fatal("javascript: 链接应被拒绝")
	}
	if !strings.Contains(err.Error(), "不合规") {
		t.Fatalf("应说明链接不合规，得到 %q", err.Error())
	}
}
