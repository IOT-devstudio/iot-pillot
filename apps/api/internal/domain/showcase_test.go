package domain

import (
	"errors"
	"strings"
	"testing"
)

/**
 * 作品提交的校验规则。
 *
 * 盯住三件事：载体至少一个（空提交对审阅者毫无意义）、
 * 仓库链接只放行 http/https（挡 javascript: 这类会执行脚本的协议）、
 * 源码包只收 zip。
 */

func TestValidateShowcaseInput_RequiresAtLeastOnePayload(t *testing.T) {
	err := ValidateShowcaseInput("我的作品", "", "", "")
	if !errors.Is(err, ErrShowcaseInvalid) {
		t.Fatalf("无载体应返回 ErrShowcaseInvalid，实际 %v", err)
	}
	var ve *ShowcaseValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Reason, "GitHub") {
		t.Fatalf("应给出可读原因，实际 %v", err)
	}
}

func TestValidateShowcaseInput_AcceptsEitherPayload(t *testing.T) {
	cases := []struct{ name, repo, key string }{
		{"只有仓库链接", "https://github.com/a/b", ""},
		{"只有源码包", "", "showcase/abc.zip"},
		{"两者都有", "https://github.com/a/b", "showcase/abc.zip"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateShowcaseInput("作品", "", tc.repo, tc.key); err != nil {
				t.Fatalf("应通过，实际 %v", err)
			}
		})
	}
}

func TestValidateShowcaseInput_TitleRules(t *testing.T) {
	if err := ValidateShowcaseInput("   ", "", "https://github.com/a/b", ""); err == nil {
		t.Fatal("空标题应被拒")
	}
	// 超过 100 个汉字应被拒（按 rune 计，不是字节）。
	tooLong := strings.Repeat("字", ShowcaseTitleMaxRunes+1)
	if err := ValidateShowcaseInput(tooLong, "", "https://github.com/a/b", ""); err == nil {
		t.Fatal("超长标题应被拒")
	}
	if err := ValidateShowcaseInput(strings.Repeat("字", ShowcaseTitleMaxRunes), "", "https://github.com/a/b", ""); err != nil {
		t.Fatalf("恰好 100 字应通过，实际 %v", err)
	}
}

func TestValidateShowcaseInput_RejectsDangerousSchemes(t *testing.T) {
	// javascript: 与 data: 在渲染成 <a href> 后可能执行脚本，必须挡掉。
	for _, bad := range []string{"javascript:alert(1)", "data:text/html,<script>", "ftp://x/y", "  "} {
		if err := ValidateShowcaseInput("作品", "", bad, ""); err == nil {
			t.Fatalf("%q 应被拒", bad)
		}
	}
}

func TestIsAllowedShowcaseRepoURL(t *testing.T) {
	ok := []string{"https://github.com/a/b", "http://example.com/x", "  https://gitee.com/a/b  "}
	for _, u := range ok {
		if !IsAllowedShowcaseRepoURL(u) {
			t.Fatalf("%q 应放行", u)
		}
	}
	bad := []string{"", "javascript:alert(1)", "data:text/html,x", "file:///etc/passwd", "//evil.com"}
	for _, u := range bad {
		if IsAllowedShowcaseRepoURL(u) {
			t.Fatalf("%q 应拒绝", u)
		}
	}
}

func TestIsZipArchive(t *testing.T) {
	// 扩展名命中
	if !IsZipArchive("demo.zip", "") {
		t.Fatal("demo.zip 应识别为 zip")
	}
	if !IsZipArchive("DEMO.ZIP", "application/octet-stream") {
		t.Fatal("大写扩展名也应识别")
	}
	// MIME 命中（部分浏览器给的 MIME 不稳定，所以两者任一即可）
	if !IsZipArchive("download", "application/x-zip-compressed") {
		t.Fatal("MIME 是 zip 时应识别")
	}
	if IsZipArchive("demo.rar", "application/x-rar-compressed") {
		t.Fatal("rar 不应识别为 zip")
	}
	if IsZipArchive("demo.txt", "") {
		t.Fatal("txt 不应识别为 zip")
	}
}

func TestShowcaseStatusKnown(t *testing.T) {
	for _, s := range []string{ShowcaseStatusSubmitted, ShowcaseStatusApproved, ShowcaseStatusRejected} {
		if !ShowcaseStatusKnown(s) {
			t.Fatalf("%q 应为合法状态", s)
		}
	}
	for _, s := range []string{"", "pending", "APPROVED", "done"} {
		if ShowcaseStatusKnown(s) {
			t.Fatalf("%q 不应为合法状态", s)
		}
	}
}
