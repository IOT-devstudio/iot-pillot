package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

// ----------------------------------------------------------------------
// 以下桩实现专供 issue #86 的 UpdateProfile / 改名测试使用，命名以 issue86
// 前缀避免与同包其他 _test.go（admin_usec_test.go 等）里的 stubUserRepo
// 等桩冲突——测试文件虽在同包但各自独立编译，重名会报重复声明。
// ----------------------------------------------------------------------

// issue86UserRepo 内存版 UserRepo，只覆盖 UpdateProfile 测试用到的路径。
type issue86UserRepo struct {
	byID    map[int]*domain.User
	byName  map[string]*domain.User
	all     []*domain.User
	updated []*domain.User
}

func issue86UserRepoWith(users ...*domain.User) *issue86UserRepo {
	r := &issue86UserRepo{
		byID:   make(map[int]*domain.User),
		byName: make(map[string]*domain.User),
	}
	for _, u := range users {
		r.byID[u.ID] = u
		r.byName[u.Name] = u
		r.all = append(r.all, u)
	}
	return r
}

func (r *issue86UserRepo) GetByID(_ context.Context, id int) (*domain.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (r *issue86UserRepo) GetByName(_ context.Context, name string) (*domain.User, error) {
	if u, ok := r.byName[name]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (r *issue86UserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, repository.ErrUserNotFound
}

func (r *issue86UserRepo) Update(_ context.Context, user *domain.User) error {
	r.updated = append(r.updated, user)
	if _, ok := r.byID[user.ID]; ok {
		r.byID[user.ID] = user
	}
	return nil
}

func (r *issue86UserRepo) SelectUserByNameAndPassword(_ context.Context, _, _ string) (*domain.User, error) {
	return nil, errors.New("unsupported in test")
}

func (r *issue86UserRepo) Save(_ context.Context, _ *domain.User) error {
	return errors.New("unsupported in test")
}

func (r *issue86UserRepo) GetAll(_ context.Context, _, _ int) ([]*domain.User, int64, error) {
	return r.all, int64(len(r.all)), nil
}

// SoftDelete 内存版软删（issue #142）：标记 DeletedAt，重复注销返回
// ErrUserAlreadyDeleted（与 gorm 实现同契约，供注销测试断言）。
func (r *issue86UserRepo) SoftDelete(_ context.Context, id int) error {
	user, ok := r.byID[id]
	if !ok {
		return repository.ErrUserNotFound
	}
	if user.DeletedAt != nil {
		return repository.ErrUserAlreadyDeleted
	}
	now := time.Now()
	user.DeletedAt = &now
	return nil
}

// issue86TokenManager 空实现 TokenIssuer，改名测试不需要签发令牌。
type issue86TokenManager struct{}

func (issue86TokenManager) GenerateTokens(_ context.Context, _ int, _, _ string) (string, string, error) {
	return "", "", nil
}

func (issue86TokenManager) ValidateRefreshToken(_ context.Context, _ string) (int, string, string, error) {
	return 0, "", "", nil
}

func (issue86TokenManager) RevokeSession(_ context.Context, _, _ string) error { return nil }
func (issue86TokenManager) RevokeUserSessions(_ context.Context, _ int) error  { return nil }

// issue86CodeManager 空实现 VerifyCodeChecker。
type issue86CodeManager struct{}

func (issue86CodeManager) VerifyCode(_ context.Context, _ string, _ utils.VerifyCodePurpose, _ string) error {
	return nil
}
func (issue86CodeManager) SendVerifyCode(_ context.Context, _ string, _ utils.VerifyCodePurpose, _ utils.VerifierType) error {
	return nil
}

// issue86AdminDirectory 空实现 AdminDirectory。
type issue86AdminDirectory struct{}

func (issue86AdminDirectory) IsAdmin(_ context.Context, _ int) (bool, error)    { return false, nil }
func (issue86AdminDirectory) AddAdmin(_ context.Context, _ int) error           { return nil }
func (issue86AdminDirectory) RemoveAdmin(_ context.Context, _ int) error        { return nil }
func (issue86AdminDirectory) ListAdmins(_ context.Context) ([]int, error)       { return nil, nil }
func (issue86AdminDirectory) IsFounding(_ context.Context, _ int) (bool, error) { return false, nil }
func (issue86AdminDirectory) AddFounding(_ context.Context, _ int) error        { return nil }
func (issue86AdminDirectory) RemoveFounding(_ context.Context, _ int) error     { return nil }

// newTestAuthUseCase 组装一个不依赖 Redis / session 的 AuthUseCase，
// limiter 传 nil（改名路径不触发限流）。
func newTestAuthUseCase(repo repository.UserRepo, tm TokenIssuer, cm VerifyCodeChecker, admins utils.AdminDirectory) *AuthUseCase {
	return NewAuthUseCase(repo, tm, cm, admins, nil)
}

// ----------------------------------------------------------------------
// 测试
// ----------------------------------------------------------------------

// 改名成功：返回的 user.Name 与持久化副本都更新，且不触碰登录名/邮箱/密码。
func TestUpdateProfile_NameEdit(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 1, Name: "旧名字", Password: "secret"})
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{})

	patch := &request.UpdateMeReq{Name: request.PatchField[string]{Present: true, Value: "新的名字"}}
	got, err := auc.UpdateProfile(context.Background(), 1, patch)
	if err != nil {
		t.Fatalf("UpdateProfile 失败: %v", err)
	}
	if got.Name != "新的名字" {
		t.Fatalf("返回的用户姓名未更新，got=%q", got.Name)
	}
	if got.Password != "secret" {
		t.Fatalf("改名不应触碰密码，got=%q", got.Password)
	}
	// 持久化副本也要变（Update 写回 byID）
	if repo.byID[1].Name != "新的名字" {
		t.Fatalf("持久化层姓名未更新，got=%q", repo.byID[1].Name)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("期望调用一次 Update，实际 %d 次", len(repo.updated))
	}
}

// 清除姓名：null / clear 语义落到空串，且其他字段不受影响。
func TestUpdateProfile_ClearName(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 1, Name: "张三", Detail: domain.Detail{Class: "c1"}})
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{})

	patch := &request.UpdateMeReq{Name: request.PatchField[string]{Present: true, Clear: true}}
	got, err := auc.UpdateProfile(context.Background(), 1, patch)
	if err != nil {
		t.Fatalf("UpdateProfile 失败: %v", err)
	}
	if got.Name != "" {
		t.Fatalf("清除姓名期望空串，got=%q", got.Name)
	}
	if got.Detail.Class != "c1" {
		t.Fatalf("清除姓名不应影响班级，got=%q", got.Detail.Class)
	}
}

// 缺少姓名键：Present=false，改名分支整体跳过，姓名保持原值。
func TestUpdateProfile_NameAbsentNoOp(t *testing.T) {
	repo := issue86UserRepoWith(&domain.User{ID: 1, Name: "原值"})
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{})

	patch := &request.UpdateMeReq{} // Name 零值：Present=false
	got, err := auc.UpdateProfile(context.Background(), 1, patch)
	if err != nil {
		t.Fatalf("UpdateProfile 失败: %v", err)
	}
	if got.Name != "原值" {
		t.Fatalf("缺省姓名不应改动，got=%q", got.Name)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("缺省也要回写一次完整实体（GORM Save 全字段），期望 1 次 Update，实际 %d", len(repo.updated))
	}
}

// 解析阶段姓名校验：中文按 UTF-8 码点计，<3 或 >20 触发可读错误；
// 超过 20 码点的上限检查先于 rune 区间检查触发，文案为"不能超过 20 个字符"。
// 校验实际发生在 ParseUpdateMe（auth_usec.go 的 UpdateProfile 不再重复校验）。
func TestParseUpdateMe_NameValidation(t *testing.T) {
	cases := []struct {
		desc    string
		body    string
		wantErr string // 期望的错误文案；空串表示合法
	}{
		{"缺省_不动", `{"class":"c"}`, ""},
		{"null_清除合法", `{"name":null}`, ""},
		{"边界_正好三中文", `{"name":"张三丰"}`, ""},
		{"边界_正好三ASCII", `{"name":"abc"}`, ""},
		{"正常_四中文", `{"name":"李四王"}`, ""},
		{"边界_正好二十中文", `{"name":"一二三四五六七八九十壹贰叁肆伍陆柒捌玖拾"}`, ""},
		{"太短_两中文", `{"name":"张丰"}`, "name 长度需在 3 到 20 个字符之间"},
		{"太短_单字", `{"name":"张"}`, "name 长度需在 3 到 20 个字符之间"},
		{"超长_二十一ASCII", `{"name":"aaaaaaaaaaaaaaaaaaaaa"}`, "name 不能超过 20 个字符"},
		{"超长_二十一中文", `{"name":"一二三四五六七八九十壹贰叁肆伍陆柒捌玖拾壹"}`, "name 不能超过 20 个字符"},
		{"非法JSON", `{bad`, "请求体必须是 JSON 对象"},
	}
	for _, c := range cases {
		req, err := request.ParseUpdateMe([]byte(c.body))
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
			t.Fatalf("用例 %q 错误文案不符，got=%q want 包含 %q", c.desc, err.Error(), c.wantErr)
		}
		_ = req
	}
}

// 解析阶段：姓名键缺省 / null / 有值 三态语义正确。
func TestParseUpdateMe_NameThreeState(t *testing.T) {
	// 缺省
	req, err := request.ParseUpdateMe([]byte(`{"class":"c"}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if req.Name.Present {
		t.Fatal("缺省姓名应为 Present=false")
	}

	// null = 清除
	req, err = request.ParseUpdateMe([]byte(`{"name":null}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !req.Name.Present || !req.Name.Clear {
		t.Fatalf("null 姓名应为 Present=true,Clear=true，got Present=%v Clear=%v", req.Name.Present, req.Name.Clear)
	}

	// 有值
	req, err = request.ParseUpdateMe([]byte(`{"name":"李小四"}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !req.Name.Present || req.Name.Clear || req.Name.Value != "李小四" {
		t.Fatalf("有值姓名解析错误，got %+v", req.Name)
	}
}

// 越权在结构上不可能：UpdateProfile 只接受 userID 参数，patch 里没有任何
// 目标标识；即使存储里存在 user 2，以 user 1 身份调用也绝不会改到 user 2。
func TestUpdateProfile_SelfOnlyNoPrivilegeEscalation(t *testing.T) {
	repo := issue86UserRepoWith(
		&domain.User{ID: 1, Name: "用户一"},
		&domain.User{ID: 2, Name: "用户二"},
	)
	auc := newTestAuthUseCase(repo, issue86TokenManager{}, issue86CodeManager{}, issue86AdminDirectory{})

	// 以 ID=1 的身份改名
	patch := &request.UpdateMeReq{Name: request.PatchField[string]{Present: true, Value: "改名后的用户一"}}
	if _, err := auc.UpdateProfile(context.Background(), 1, patch); err != nil {
		t.Fatalf("UpdateProfile 失败: %v", err)
	}
	if repo.byID[1].Name != "改名后的用户一" {
		t.Fatalf("user 1 应被改名，got=%q", repo.byID[1].Name)
	}
	if repo.byID[2].Name != "用户二" {
		t.Fatalf("user 2 不应被改动，got=%q", repo.byID[2].Name)
	}
	// 只回写了 user 1
	if len(repo.updated) != 1 || repo.updated[0].ID != 1 {
		t.Fatalf("期望只更新 user 1，实际更新 %d 条", len(repo.updated))
	}
}

// 越权在结构上不可能（续）：body 里的 user_id 等非白名单键在解析阶段被丢弃，
// 不会出现"以他人为目标"的语义。
func TestParseUpdateMe_DropsNonWhitelistKeys(t *testing.T) {
	body := []byte(`{"name":"王小明","user_id":2,"email":"a@b.c","role":"admin"}`)
	req, err := request.ParseUpdateMe(body)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if req.Name.Value != "王小明" {
		t.Fatalf("白名单 name 应被解析，got=%q", req.Name.Value)
	}
	// 非白名单键（user_id / email / role）不应进入任何补丁字段。
	// UpdateMeReq 没有这些字段，这里仅确保 ParseUpdateMe 不报错地丢弃了它们；
	// 真正的"无目标参数"保证由 UpdateProfile(userID int, ...) 的签名承接。
	_ = req
}
