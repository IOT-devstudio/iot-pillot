package request

import (
	"strings"
	"testing"
)

// TestParseUpdateMe 三态语义与校验（issue #57 验收项的纯函数部分）。
func TestParseUpdateMe(t *testing.T) {
	t.Run("完整补丁：四个字段都有值", func(t *testing.T) {
		patch, err := ParseUpdateMe([]byte(`{
			"class": "物联网工程 2301",
			"student_id": 2023114514,
			"qq": "1044696157",
			"direction": "front-end"
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !patch.Class.Present || patch.Class.Clear || patch.Class.Value != "物联网工程 2301" {
			t.Errorf("class = %+v", patch.Class)
		}
		if !patch.StudentID.Present || patch.StudentID.Clear || patch.StudentID.Value != 2023114514 {
			t.Errorf("student_id = %+v", patch.StudentID)
		}
		if !patch.QQ.Present || patch.QQ.Value != "1044696157" {
			t.Errorf("qq = %+v", patch.QQ)
		}
		if !patch.Direction.Present || patch.Direction.Value != "front-end" {
			t.Errorf("direction = %+v", patch.Direction)
		}
	})

	t.Run("缺省的键不动（Present=false）", func(t *testing.T) {
		patch, err := ParseUpdateMe([]byte(`{"qq": "12345"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if patch.Class.Present || patch.StudentID.Present || patch.Direction.Present {
			t.Errorf("absent keys must not be present: %+v", patch)
		}
		if !patch.QQ.Present || patch.QQ.Value != "12345" {
			t.Errorf("qq = %+v", patch.QQ)
		}
	})

	t.Run("null 清除（Clear=true）", func(t *testing.T) {
		patch, err := ParseUpdateMe([]byte(`{
			"class": null,
			"student_id": null,
			"qq": null,
			"direction": null
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for name, field := range map[string]struct {
			Present bool
			Clear   bool
		}{
			"class":      {patch.Class.Present, patch.Class.Clear},
			"student_id": {patch.StudentID.Present, patch.StudentID.Clear},
			"qq":         {patch.QQ.Present, patch.QQ.Clear},
			"direction":  {patch.Direction.Present, patch.Direction.Clear},
		} {
			if !field.Present || !field.Clear {
				t.Errorf("%s: want present+clear, got %+v", name, field)
			}
		}
	})

	t.Run("方向空串与 null 同义（清除）", func(t *testing.T) {
		patch, err := ParseUpdateMe([]byte(`{"direction": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !patch.Direction.Present || !patch.Direction.Clear {
			t.Errorf("direction = %+v, want present+clear", patch.Direction)
		}
	})

	t.Run("越权键 user_id 被忽略（issue 验收：塞别人 ID 不生效）", func(t *testing.T) {
		patch, err := ParseUpdateMe([]byte(`{"user_id": 999, "qq": "1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 解析结果里没有任何 user_id 的落点——更新目标只可能来自令牌
		if !patch.QQ.Present || patch.QQ.Value != "1" {
			t.Errorf("qq = %+v", patch.QQ)
		}
	})

	t.Run("非法 direction 返回中文错误", func(t *testing.T) {
		_, err := ParseUpdateMe([]byte(`{"direction": "blockchain"}`))
		if err == nil {
			t.Fatal("want error, got nil")
		}
		if !strings.Contains(err.Error(), "direction") || !strings.Contains(err.Error(), "之一") {
			t.Errorf("error = %q, want chinese message about direction", err)
		}
	})

	t.Run("student_id 拒绝负数 / 小数 / 字符串", func(t *testing.T) {
		for _, body := range []string{
			`{"student_id": -1}`,
			`{"student_id": 3.7}`,
			`{"student_id": "123"}`,
		} {
			if _, err := ParseUpdateMe([]byte(body)); err == nil {
				t.Errorf("%s: want error, got nil", body)
			} else if !strings.Contains(err.Error(), "student_id") {
				t.Errorf("%s: error = %q", body, err)
			}
		}
	})

	t.Run("长度上限与前端 maxlength 对齐", func(t *testing.T) {
		longClass := `{"class": "` + strings.Repeat("班", 65) + `"}`
		if _, err := ParseUpdateMe([]byte(longClass)); err == nil {
			t.Error("class 65 chars: want error, got nil")
		}
		longQQ := `{"qq": "` + strings.Repeat("9", 21) + `"}`
		if _, err := ParseUpdateMe([]byte(longQQ)); err == nil {
			t.Error("qq 21 chars: want error, got nil")
		}

		okClass := `{"class": "` + strings.Repeat("班", 64) + `"}`
		if _, err := ParseUpdateMe([]byte(okClass)); err != nil {
			t.Errorf("class 64 chars should pass: %v", err)
		}
	})

	t.Run("非对象 body 拒绝", func(t *testing.T) {
		for _, body := range []string{`[1,2]`, `"str"`, `42`, `not-json`} {
			if _, err := ParseUpdateMe([]byte(body)); err == nil {
				t.Errorf("%s: want error, got nil", body)
			}
		}
	})

	t.Run("字面量 null body 等价空补丁", func(t *testing.T) {
		patch, err := ParseUpdateMe([]byte(`null`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if patch.Class.Present || patch.StudentID.Present || patch.QQ.Present || patch.Direction.Present {
			t.Errorf("null body should be an empty patch: %+v", patch)
		}
	})
}
