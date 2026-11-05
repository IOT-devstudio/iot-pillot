package repository

import "testing"

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
