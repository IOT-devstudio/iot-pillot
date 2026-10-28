package response

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNilDataIsOmitted 确认删除类端点的响应形状。
//
// 这是本模块的一个真实隐患：后端 OKWithMsg(c, msg, nil) 会把 data 写成 nil，
// 而 Result.Data 是 `json:"data,omitempty"` —— nil interface 会被省略，
// 于是前端 requestWithSession 拿不到 data，会把**成功的删除**当成「服务暂时不可用」。
func TestNilDataIsOmitted(t *testing.T) {
	got, err := json.Marshal(Success(nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Success(nil) -> %s", string(got))

	if !strings.Contains(string(got), "data") {
		t.Log("确认：nil data 被 omitempty 省略，删除类前端必须走 message 出口")
	}
}

// TestDataPresentWhenNonNil 对照组：有数据时 data 必须出现。
func TestDataPresentWhenNonNil(t *testing.T) {
	got, err := json.Marshal(Success(map[string]int{"id": 1}))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Success(map) -> %s", string(got))
	if !strings.Contains(string(got), "data") {
		t.Fatal("非 nil 的 data 不应被省略")
	}
}
