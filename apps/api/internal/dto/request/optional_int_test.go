package request

import (
	"encoding/json"
	"testing"
)

// TestOptionalIntDistinguishesNullFromAbsent 锁住三态语义（#195）。
//
// 这是本类型存在的唯一理由：**int 会把 null 与「键没出现」解成同一个 nil，
// 「解除账号绑定」于是被静默当成「不改」。谁把这里换回 **int，这个用例会红。
func TestOptionalIntDistinguishesNullFromAbsent(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantSet bool
		wantNil bool
		want    int
	}{
		{name: "键没出现", body: `{"name":"a"}`, wantSet: false},
		{name: "显式 null", body: `{"name":"a","user_id":null}`, wantSet: true, wantNil: true},
		{name: "传了数字", body: `{"name":"a","user_id":7}`, wantSet: true, want: 7},
		{name: "传了 0", body: `{"name":"a","user_id":0}`, wantSet: true, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req StudioMemberUpdateReq
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatal(err)
			}
			if req.UserID.Set != tc.wantSet {
				t.Fatalf("Set 应为 %v，实际 %v", tc.wantSet, req.UserID.Set)
			}
			if !tc.wantSet {
				if req.UserID.Value != nil {
					t.Fatalf("键没出现时 Value 应为 nil，实际 %v", *req.UserID.Value)
				}
				return
			}
			if tc.wantNil {
				if req.UserID.Value != nil {
					t.Fatalf("显式 null 时 Value 应为 nil，实际 %v", *req.UserID.Value)
				}
				return
			}
			if req.UserID.Value == nil {
				t.Fatal("传了数字时 Value 不应为 nil")
			}
			if *req.UserID.Value != tc.want {
				t.Fatalf("Value 应为 %d，实际 %d", tc.want, *req.UserID.Value)
			}
		})
	}
}

// TestOptionalIntRejectsNonInteger 非法输入必须报错，而不是静默当成 0。
func TestOptionalIntRejectsNonInteger(t *testing.T) {
	var req StudioMemberUpdateReq
	if err := json.Unmarshal([]byte(`{"user_id":"abc"}`), &req); err == nil {
		t.Fatal("字符串 user_id 应报错")
	}
}
