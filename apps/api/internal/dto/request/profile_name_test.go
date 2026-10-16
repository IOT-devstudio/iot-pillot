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
	req, err := ParseUpdateMe([]byte(`{"class":"新班级"}`))
	if err != nil || req.Name.Present {
		t.Fatalf("omitted name must stay unchanged: %v", err)
	}
}
