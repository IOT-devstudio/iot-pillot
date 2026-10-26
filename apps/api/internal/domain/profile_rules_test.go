package domain

import "testing"

// 资料规则是冻结判定与写入校验的唯一口径，用例覆盖口径边界：
// 姓名长度/非汉字、班级「汉字+恰好6位数字」、学号恰好 9 位且首位非 0。

func TestValidateName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		ok    bool
	}{
		{"正常两字", "张三", true},
		{"正常三字", "欧阳锋", true},
		{"二十字边界", "一二三四五六七八九十一二三四五六七八九十", true},
		{"前后空白不算合规（不静默裁剪）", "  张三  ", false},
		{"单字过短", "张", false},
		{"空串", "", false},
		{"纯空白", "   ", false},
		{"含数字", "张3", false},
		{"含字母", "张三abc", false},
		{"中英混合", "张San", false},
		{"含标点", "张·三", false},
		{"超长", "一二三四五六七八九十一二三四五六七八九十一", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ValidateName(tc.input)
			if tc.ok && msg != "" {
				t.Fatalf("期望合规，实际: %s", msg)
			}
			if !tc.ok && msg == "" {
				t.Fatalf("期望不合规，实际通过了校验")
			}
		})
	}
}

func TestValidateClass(t *testing.T) {
	cases := []struct {
		name  string
		input string
		ok    bool
	}{
		{"标准写法", "软件工程230301", true},
		{"短专业名", "软工230301", true},
		{"前后空白不算合规", " 软工230301 ", false},
		{"数字不足六位", "软工23030", false},
		{"数字超过六位", "软工2303011", false},
		{"没有数字", "软工", false},
		{"没有汉字", "230301", false},
		{"汉字在数字后", "230301软工", false},
		{"中间夹空格", "软工 230301", false},
		{"含字母", "soft230301", false},
		{"汉字与数字混合分隔", "软工-230301", false},
		{"空串", "", false},
		{"历史写法 软工 2 班", "软工 2 班", false},
		{"历史写法 物联网工程 2301", "物联网工程 2301", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ValidateClass(tc.input)
			if tc.ok && msg != "" {
				t.Fatalf("期望合规，实际: %s", msg)
			}
			if !tc.ok && msg == "" {
				t.Fatalf("期望不合规，实际通过了校验")
			}
		})
	}
}

func TestValidateStudentID(t *testing.T) {
	cases := []struct {
		name  string
		input int
		ok    bool
	}{
		{"九位标准学号", 202311451, true},
		{"九位最大", 999999999, true},
		{"零表示未填", 0, false},
		{"八位", 20231145, false},
		{"十位", 2023114514, false},
		{"首位为零（解析后位数不足）", 2311451, false},
		{"负数", -202311451, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ValidateStudentID(tc.input)
			if tc.ok && msg != "" {
				t.Fatalf("期望合规，实际: %s", msg)
			}
			if !tc.ok && msg == "" {
				t.Fatalf("期望不合规，实际通过了校验")
			}
		})
	}
}

func TestProfileIssues(t *testing.T) {
	t.Run("完全合规返回 nil", func(t *testing.T) {
		user := &User{Name: "张三", Detail: Detail{Class: "软件工程230301", StudentID: 202311451}}
		if issues := ProfileIssues(user); len(issues) != 0 {
			t.Fatalf("期望无问题，实际: %+v", issues)
		}
	})

	t.Run("一次返回全部不合规项且顺序固定", func(t *testing.T) {
		user := &User{Name: "张3", Detail: Detail{Class: "软工 2 班", StudentID: 20231145}}
		issues := ProfileIssues(user)
		if len(issues) != 3 {
			t.Fatalf("期望 3 条，实际 %d: %+v", len(issues), issues)
		}
		wantFields := []string{ProfileFieldName, ProfileFieldClass, ProfileFieldStudentID}
		for i, want := range wantFields {
			if issues[i].Field != want {
				t.Fatalf("第 %d 条字段期望 %s，实际 %s", i, want, issues[i].Field)
			}
		}
	})

	t.Run("nil 用户不 panic", func(t *testing.T) {
		if issues := ProfileIssues(nil); issues != nil {
			t.Fatalf("期望 nil，实际 %+v", issues)
		}
	})

	t.Run("空资料三条都要报", func(t *testing.T) {
		issues := ProfileIssues(&User{})
		if len(issues) != 3 {
			t.Fatalf("期望 3 条，实际 %d: %+v", len(issues), issues)
		}
	})
}

func TestProfileIssuesSummary(t *testing.T) {
	if got := ProfileIssuesSummary(nil); got != "" {
		t.Fatalf("期望空串，实际 %q", got)
	}

	summary := ProfileIssuesSummary([]ProfileIssue{
		{Field: ProfileFieldName, Message: "姓名未填写"},
		{Field: ProfileFieldStudentID, Message: "学号未填写"},
	})
	if summary != "姓名未填写；学号未填写" {
		t.Fatalf("摘要拼接不符: %q", summary)
	}
}
