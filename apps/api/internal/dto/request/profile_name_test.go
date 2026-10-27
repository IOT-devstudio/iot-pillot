package request

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProfileChineseName(t *testing.T) {
	for _, name := range []string{"张三", strings.Repeat("张", 20), strings.Repeat("𠮷", 20)} {
		body, _ := json.Marshal(map[string]string{"name": name})
		req, err := ParseUpdateMe(body)
		if err != nil || req.Name.Value != name {
			t.Fatalf("valid name %q: %v", name, err)
		}
	}
	for _, name := range []string{"", "张", strings.Repeat("张", 21), "Ada", "张三A", "张三1", "张 三", " 张三", "张三 ", "张·三", "张三😀"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		if _, err := ParseUpdateMe(body); err == nil || err.Error() != "姓名须为 2–20 个中文汉字" {
			t.Errorf("invalid name %q: %v", name, err)
		}
	}
	for _, body := range []string{`{"name":null}`, `{"name":123}`} {
		if _, err := ParseUpdateMe([]byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	req, err := ParseUpdateMe([]byte(`{"class":"软工230301"}`))
	if err != nil || req.Name.Present {
		t.Fatalf("omitted name must stay unchanged: %v", err)
	}
}

// 班级与学号的写入校验（0012）：与定时冻结共用 domain 规则。
// 不合规输入在写入阶段就被拒，避免「保存成功 → 7 天后被冻结」这种落差。
func TestParseUpdateMe_ClassAndStudentID(t *testing.T) {
	cases := []struct {
		desc    string
		body    string
		wantErr string
	}{
		{"班级合规", `{"class":"软件工程230301"}`, ""},
		{"班级数字不足", `{"class":"软工23030"}`, "班级须为「汉字 + 6 位数字」"},
		{"班级没有数字", `{"class":"软工"}`, "班级须为「汉字 + 6 位数字」"},
		{"班级含空格", `{"class":"软工 230301"}`, "班级须为「汉字 + 6 位数字」"},
		{"班级清除（null）放行", `{"class":null}`, ""},
		{"学号九位合规", `{"student_id":202311451}`, ""},
		{"学号八位", `{"student_id":20231145}`, "学号须为 9 位数字"},
		{"学号十位", `{"student_id":2023114514}`, "学号须为 9 位数字"},
		{"学号清除（null）放行", `{"student_id":null}`, ""},
		{"学号零视为未填放行", `{"student_id":0}`, ""},
		{"班级与学号同时不合规时先报班级", `{"class":"x","student_id":1}`, "班级须为"},
	}
	for _, c := range cases {
		_, err := ParseUpdateMe([]byte(c.body))
		if c.wantErr == "" {
			if err != nil {
				t.Fatalf("用例 %q 应合法，却报错: %v", c.desc, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("用例 %q 应报错，却通过", c.desc)
		}
		if !strings.Contains(err.Error(), c.wantErr) {
			t.Fatalf("用例 %q 文案不符，got=%q want 包含 %q", c.desc, err.Error(), c.wantErr)
		}
	}
}
