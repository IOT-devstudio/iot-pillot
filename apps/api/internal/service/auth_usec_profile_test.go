package service

import (
	"context"
	"errors"
	"testing"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// stubUserRepo 只实现 UpdateProfile 路径用到的 GetByID / Update，
// 其余方法与本测试无关，直接 panic —— 若被调用说明测试假设错了。
type stubUserRepo struct {
	user    *domain.User
	getErr  error
	saved   *domain.User
	paniced string
}

func (s *stubUserRepo) GetByID(ctx context.Context, id int) (*domain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.user, nil
}

func (s *stubUserRepo) Update(ctx context.Context, user *domain.User) error {
	s.saved = user
	return nil
}

func (s *stubUserRepo) GetByName(ctx context.Context, name string) (*domain.User, error) {
	s.paniced = "GetByName"
	panic("GetByName not expected in this test")
}

func (s *stubUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	s.paniced = "GetByEmail"
	panic("GetByEmail not expected in this test")
}

func (s *stubUserRepo) SelectUserByNameAndPassword(ctx context.Context, name string, password string) (*domain.User, error) {
	panic("SelectUserByNameAndPassword not expected in this test")
}

func (s *stubUserRepo) Save(ctx context.Context, user *domain.User) error {
	panic("Save not expected in this test")
}

func (s *stubUserRepo) GetAll(ctx context.Context, page int, pageSize int) ([]*domain.User, int64, error) {
	panic("GetAll not expected in this test")
}

func profileTestUser() *domain.User {
	return &domain.User{
		ID:       7,
		Name:     "ada",
		Password: "hash-of-password",
		Detail: domain.Detail{
			StudentID: 2023114514,
			Class:     "物联网工程 2301",
			QQ:        "1044696157",
			Direction: domain.FrontEnd,
			Email:     "ada@example.edu.cn",
		},
	}
}

// newProfileUseCase 只为 UpdateProfile 造依赖：其余依赖本路径不会触碰，传 nil。
func newProfileUseCase(repo *stubUserRepo) *AuthUseCase {
	return NewAuthUseCase(repo, nil, nil, nil, nil)
}

func TestUpdateProfile(t *testing.T) {
	t.Run("有值覆盖，缺省不动", func(t *testing.T) {
		repo := &stubUserRepo{user: profileTestUser()}
		auc := newProfileUseCase(repo)

		patch := &request.UpdateMeReq{
			QQ: request.PatchField[string]{Present: true, Value: "999888777"},
		}

		got, err := auc.UpdateProfile(context.Background(), 7, patch)
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if got.Detail.QQ != "999888777" {
			t.Errorf("qq = %q, want 999888777", got.Detail.QQ)
		}
		// 缺省字段必须原样保留——尤其是 email/password，Save 全字段覆盖
		if got.Detail.Class != "物联网工程 2301" || got.Detail.StudentID != 2023114514 ||
			got.Detail.Direction != domain.FrontEnd || got.Detail.Email != "ada@example.edu.cn" {
			t.Errorf("absent fields changed: %+v", got.Detail)
		}
		if repo.saved == nil {
			t.Fatal("Update was not called")
		}
		if repo.saved.Password != "hash-of-password" {
			t.Errorf("password must be preserved through Save, got %q", repo.saved.Password)
		}
	})

	t.Run("null 清除为零值", func(t *testing.T) {
		repo := &stubUserRepo{user: profileTestUser()}
		auc := newProfileUseCase(repo)

		patch := &request.UpdateMeReq{
			Class:     request.PatchField[string]{Present: true, Clear: true},
			StudentID: request.PatchField[int]{Present: true, Clear: true},
			Direction: request.PatchField[string]{Present: true, Clear: true},
		}

		got, err := auc.UpdateProfile(context.Background(), 7, patch)
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if got.Detail.Class != "" || got.Detail.StudentID != 0 || got.Detail.Direction != "" {
			t.Errorf("clear failed: %+v", got.Detail)
		}
		// 未触碰的 qq/email 保留
		if got.Detail.QQ != "1044696157" || got.Detail.Email != "ada@example.edu.cn" {
			t.Errorf("untouched fields changed: %+v", got.Detail)
		}
	})

	t.Run("账号不存在时返回 ErrUserNotFound 且不落库", func(t *testing.T) {
		repo := &stubUserRepo{getErr: repository.ErrUserNotFound}
		auc := newProfileUseCase(repo)

		_, err := auc.UpdateProfile(context.Background(), 7, &request.UpdateMeReq{})
		if !errors.Is(err, repository.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
		if repo.saved != nil {
			t.Error("Update must not be called when user is missing")
		}
	})
}

func TestGetProfile(t *testing.T) {
	t.Run("返回完整实体", func(t *testing.T) {
		repo := &stubUserRepo{user: profileTestUser()}
		auc := newProfileUseCase(repo)

		got, err := auc.GetProfile(context.Background(), 7)
		if err != nil {
			t.Fatalf("GetProfile: %v", err)
		}
		if got.Name != "ada" || got.Detail.QQ != "1044696157" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("查不到时哨兵错误原样透出", func(t *testing.T) {
		repo := &stubUserRepo{getErr: repository.ErrUserNotFound}
		auc := newProfileUseCase(repo)

		if _, err := auc.GetProfile(context.Background(), 7); !errors.Is(err, repository.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})
}
