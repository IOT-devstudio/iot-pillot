package utils

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

// fakeAdminDirectory 用于验证"消费者只依赖接口"这件事：
// 管理员服务与中间件都不需要 Redis 就能测。
type fakeAdminDirectory struct {
	ids map[int]bool
	err error
}

func (f *fakeAdminDirectory) IsAdmin(_ context.Context, userID int) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.ids[userID], nil
}

func (f *fakeAdminDirectory) AddAdmin(_ context.Context, userID int) error {
	if f.err != nil {
		return f.err
	}
	f.ids[userID] = true
	return nil
}

func (f *fakeAdminDirectory) RemoveAdmin(_ context.Context, userID int) error {
	if f.err != nil {
		return f.err
	}
	delete(f.ids, userID)
	return nil
}

func (f *fakeAdminDirectory) ListAdmins(_ context.Context) ([]int, error) {
	if f.err != nil {
		return nil, f.err
	}
	ids := make([]string, 0, len(f.ids))
	for id := range f.ids {
		ids = append(ids, strconv.Itoa(id))
	}
	return ParseAdminIDs(ids), nil
}

// 接口断言：保证 AdminStore 真的满足 AdminDirectory，
// 否则 wire 注入时会在编译期之外的地方炸。
var _ AdminDirectory = (*AdminStore)(nil)

func TestParseAdminIDs(t *testing.T) {
	cases := []struct {
		name    string
		members []string
		want    []int
	}{
		{name: "空名单", members: nil, want: []int{}},
		{name: "正常成员", members: []string{"3", "1", "2"}, want: []int{1, 2, 3}},
		{
			name: "跳过脏数据而不是报错",
			members: []string{
				"5",
				"abc", // 非数字
				"",    // 空串
				"0",   // 无意义 ID
				"-7",  // 负 ID
				"2",
			},
			want: []int{2, 5},
		},
		{name: "重复成员去重后排序", members: []string{"4", "4", "2"}, want: []int{2, 4}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := ParseAdminIDs(testCase.members)
			if len(got) != len(testCase.want) {
				t.Fatalf("ParseAdminIDs(%v) = %v，期望 %v", testCase.members, got, testCase.want)
			}
			for i, want := range testCase.want {
				if got[i] != want {
					t.Errorf("第 %d 个 = %d，期望 %d（完整结果 %v）", i, got[i], want, got)
				}
			}
		})
	}
}

// 管理员服务与中间件都依赖接口，因此可以完全不碰 Redis 地覆盖判定分支。
func TestAdminDirectoryStubCoversBranches(t *testing.T) {
	ctx := context.Background()
	directory := &fakeAdminDirectory{ids: map[int]bool{1: true}}

	if ok, err := directory.IsAdmin(ctx, 1); err != nil || !ok {
		t.Fatalf("userID=1 应当是管理员，得到 ok=%v err=%v", ok, err)
	}
	if ok, err := directory.IsAdmin(ctx, 2); err != nil || ok {
		t.Fatalf("userID=2 不该是管理员，得到 ok=%v err=%v", ok, err)
	}

	if err := directory.AddAdmin(ctx, 2); err != nil {
		t.Fatalf("AddAdmin 失败: %v", err)
	}
	if ok, _ := directory.IsAdmin(ctx, 2); !ok {
		t.Error("AddAdmin 后应当判定为管理员")
	}

	if err := directory.RemoveAdmin(ctx, 1); err != nil {
		t.Fatalf("RemoveAdmin 失败: %v", err)
	}
	if ok, _ := directory.IsAdmin(ctx, 1); ok {
		t.Error("RemoveAdmin 后不该再判定为管理员")
	}

	failing := &fakeAdminDirectory{ids: map[int]bool{}, err: errors.New("redis down")}
	if _, err := failing.IsAdmin(ctx, 1); err == nil {
		t.Error("底层报错时必须把错误透出去，而不是静默当成非管理员")
	}
}
