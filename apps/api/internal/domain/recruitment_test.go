package domain

import "testing"

// AllowsInterviewSupplement 是 #135 的补交判据：只有还没出结果的申请能再收材料。
//
// 与 CanTransitionTo 分开测，因为两者回答不同问题：applied 不能流转到 accepted
// （CanTransitionTo=false），但完全允许补交面试材料（AllowsInterviewSupplement=true）。
// 表单提交一旦误用 CanTransitionTo，"先报名、后补面试"这条正常路径就会被拒。
func TestApplicationStatus_AllowsInterviewSupplement(t *testing.T) {
	cases := []struct {
		status ApplicationStatus
		want   bool
	}{
		{ApplicationApplied, true},
		{ApplicationReviewing, true},
		{ApplicationAccepted, false},
		{ApplicationRejected, false},
		// 未知状态按"不可补交"处理：宁可拒绝也不给出模糊的写入许可
		{ApplicationStatus("bogus"), false},
	}

	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := tc.status.AllowsInterviewSupplement(); got != tc.want {
				t.Errorf("AllowsInterviewSupplement(%q) = %v，期望 %v", tc.status, got, tc.want)
			}
		})
	}
}

// 守住两个判据的差异：applied 不可流转但可补交，这正是 #135 不能复用
// CanTransitionTo 做补交判断的原因。
func TestApplicationStatus_SupplementDiffersFromTransition(t *testing.T) {
	applied := ApplicationApplied
	if applied.CanTransitionTo(ApplicationAccepted) {
		t.Fatal("前提变了：applied 不应能直接流转到 accepted")
	}
	if !applied.AllowsInterviewSupplement() {
		t.Error("applied 必须允许补交面试材料（先报名、后补面试）")
	}
}
