package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

// ------------------------------------------------------------------ *
// 内存桩：覆盖 BE-04 用例依赖的全部仓储接口。
// 桩刻意复刻数据库约束的两个行为：
//   - 同一申请至多一条 pending 邀请（部分唯一索引）；
//   - 一个申请至多一条候选人（application_id 唯一索引）。
// 这两条是幂等兜底的最后一道防线，必须在桩里保留，否则测不到并发分支。
// ------------------------------------------------------------------ *

type applicationRepoStub struct {
	apps     map[int]*domain.Application
	statuses map[int]domain.ApplicationStatus // 记录 UpdateApplicationStatus 调用
}

func (s *applicationRepoStub) GetApplicationByID(ctx context.Context, id int) (*domain.Application, error) {
	app, ok := s.apps[id]
	if !ok {
		return nil, repository.ErrApplicationNotFound
	}
	return app, nil
}

func (s *applicationRepoStub) UpdateApplicationStatus(ctx context.Context, id int, status domain.ApplicationStatus) (int64, error) {
	s.statuses[id] = status
	return 1, nil
}

type invitationRepoStub struct {
	rows map[int]*domain.Invitation
	next int
	// apps 用于 ListByUserID 复刻「邀请经申请关联归属人」的 JOIN 过滤。
	apps *applicationRepoStub
}

func (s *invitationRepoStub) byID(id int) (*domain.Invitation, error) {
	row, ok := s.rows[id]
	if !ok {
		return nil, repository.ErrInvitationNotFound
	}
	return row, nil
}

func (s *invitationRepoStub) GetByID(ctx context.Context, id int) (*domain.Invitation, error) {
	return s.byID(id)
}

func (s *invitationRepoStub) GetByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	for _, row := range s.rows {
		if row.Token == token {
			return row, nil
		}
	}
	return nil, repository.ErrInvitationNotFound
}

func (s *invitationRepoStub) GetPendingByApplication(ctx context.Context, applicationID int) (*domain.Invitation, error) {
	for _, row := range s.rows {
		if row.ApplicationID == applicationID && row.Status == domain.InvitationPending {
			return row, nil
		}
	}
	return nil, repository.ErrInvitationNotFound
}

func (s *invitationRepoStub) List(ctx context.Context, page, pageSize, applicationID int) ([]*domain.Invitation, int64, error) {
	var items []*domain.Invitation
	for _, row := range s.rows {
		if applicationID > 0 && row.ApplicationID != applicationID {
			continue
		}
		items = append(items, row)
	}
	return items, int64(len(items)), nil
}

func (s *invitationRepoStub) ListByUserID(ctx context.Context, userID int) ([]*domain.Invitation, error) {
	var items []*domain.Invitation
	for _, row := range s.rows {
		if app, ok := s.apps.apps[row.ApplicationID]; ok && app.UserID == userID {
			items = append(items, row)
		}
	}
	return items, nil
}

func (s *invitationRepoStub) Create(ctx context.Context, invitation *domain.Invitation) error {
	// 复刻 idx_recruitment_invitations_pending_app：同申请第二条 pending 撞约束。
	for _, row := range s.rows {
		if row.ApplicationID == invitation.ApplicationID && row.Status == domain.InvitationPending {
			return repository.ErrInvitationConflict
		}
	}
	s.next++
	invitation.ID = s.next
	s.rows[invitation.ID] = invitation
	return nil
}

func (s *invitationRepoStub) Update(ctx context.Context, invitation *domain.Invitation) error {
	s.rows[invitation.ID] = invitation
	return nil
}

func (s *invitationRepoStub) count() int { return len(s.rows) }

type candidateRepoStub struct {
	rows     map[int]*domain.Candidate
	timeline map[int][]*domain.CandidateTimeline
	next     int
}

func (s *candidateRepoStub) GetByID(ctx context.Context, id int) (*domain.Candidate, error) {
	row, ok := s.rows[id]
	if !ok {
		return nil, repository.ErrCandidateNotFound
	}
	return row, nil
}

func (s *candidateRepoStub) GetByApplicationID(ctx context.Context, applicationID int) (*domain.Candidate, error) {
	for _, row := range s.rows {
		if row.ApplicationID == applicationID {
			return row, nil
		}
	}
	return nil, repository.ErrCandidateNotFound
}

func (s *candidateRepoStub) GetByUserID(ctx context.Context, userID int) ([]*domain.Candidate, error) {
	var items []*domain.Candidate
	for _, row := range s.rows {
		if row.UserID == userID {
			items = append(items, row)
		}
	}
	return items, nil
}

func (s *candidateRepoStub) List(ctx context.Context, page, pageSize int, status string) ([]*domain.Candidate, int64, error) {
	var items []*domain.Candidate
	for _, row := range s.rows {
		if status != "" && string(row.Status) != status {
			continue
		}
		items = append(items, row)
	}
	return items, int64(len(items)), nil
}

func (s *candidateRepoStub) Create(ctx context.Context, candidate *domain.Candidate) error {
	// 复刻 application_id 唯一索引：并发接受的数据库兜底。
	for _, row := range s.rows {
		if row.ApplicationID == candidate.ApplicationID {
			return repository.ErrCandidateConflict
		}
	}
	s.next++
	candidate.ID = s.next
	s.rows[candidate.ID] = candidate
	return nil
}

func (s *candidateRepoStub) Update(ctx context.Context, candidate *domain.Candidate) error {
	s.rows[candidate.ID] = candidate
	return nil
}

func (s *candidateRepoStub) Timeline(ctx context.Context, candidateID int) ([]*domain.CandidateTimeline, error) {
	return s.timeline[candidateID], nil
}

func (s *candidateRepoStub) AppendTimeline(ctx context.Context, timeline *domain.CandidateTimeline) {
	s.timeline[timeline.CandidateID] = append(s.timeline[timeline.CandidateID], timeline)
}

func (s *candidateRepoStub) count() int { return len(s.rows) }

type userRepoStub struct {
	users map[int]*domain.User
}

func (s *userRepoStub) GetByID(ctx context.Context, id int) (*domain.User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (s *userRepoStub) GetByName(ctx context.Context, name string) (*domain.User, error) {
	return nil, repository.ErrUserNotFound
}

func (s *userRepoStub) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, repository.ErrUserNotFound
}

func (s *userRepoStub) Update(ctx context.Context, user *domain.User) error { return nil }

func (s *userRepoStub) SelectUserByNameAndPassword(ctx context.Context, name, password string) (*domain.User, error) {
	return nil, repository.ErrUserNotFound
}

func (s *userRepoStub) Save(ctx context.Context, user *domain.User) error { return nil }

func (s *userRepoStub) GetAll(ctx context.Context, page, pageSize int) ([]*domain.User, int64, error) {
	return nil, 0, nil
}

// SoftDelete 空实现（issue #142 加接口方法）：邀请测试不涉及注销。
func (s *userRepoStub) SoftDelete(ctx context.Context, id int) error { return nil }

type mailModelRepoStub struct {
	models map[int]*domain.MailModel
}

func (s *mailModelRepoStub) List(ctx context.Context) ([]*domain.MailModel, error) {
	var items []*domain.MailModel
	for _, m := range s.models {
		items = append(items, m)
	}
	return items, nil
}

func (s *mailModelRepoStub) GetByID(ctx context.Context, id int) (*domain.MailModel, error) {
	model, ok := s.models[id]
	if !ok {
		return nil, errors.New("模板不存在")
	}
	return model, nil
}

func (s *mailModelRepoStub) Create(ctx context.Context, model *domain.MailModel) error { return nil }
func (s *mailModelRepoStub) Update(ctx context.Context, model *domain.MailModel) error { return nil }
func (s *mailModelRepoStub) Delete(ctx context.Context, id int) error                  { return nil }

// stubMailer 的 fail 开关模拟 SMTP 投递结果；calls 记录实际外发次数。
type stubMailer struct {
	fail  bool
	calls int
}

func (m *stubMailer) SendByTemplate(
	ctx context.Context,
	operatorID int,
	templateID int,
	user *domain.User,
	vars request.MailVars,
) (response.MailSendResp, error) {
	m.calls++
	if m.fail {
		return response.MailSendResp{}, errors.New("smtp 投递失败")
	}
	return response.MailSendResp{}, nil
}

// ------------------------------------------------------------------ *
// 组装与种子数据
// ------------------------------------------------------------------ *

type harness struct {
	apps   *applicationRepoStub
	invs   *invitationRepoStub
	cands  *candidateRepoStub
	users  *userRepoStub
	mails  *mailModelRepoStub
	mailer *stubMailer
	uc     *RecruitmentInvitationUseCase
}

func newHarness() *harness {
	h := &harness{
		apps:   &applicationRepoStub{apps: map[int]*domain.Application{}, statuses: map[int]domain.ApplicationStatus{}},
		invs:   &invitationRepoStub{rows: map[int]*domain.Invitation{}},
		cands:  &candidateRepoStub{rows: map[int]*domain.Candidate{}, timeline: map[int][]*domain.CandidateTimeline{}},
		users:  &userRepoStub{users: map[int]*domain.User{}},
		mails:  &mailModelRepoStub{models: map[int]*domain.MailModel{}},
		mailer: &stubMailer{},
	}
	h.invs.apps = h.apps
	// 模板：1 = invitation，2 = offer，3 = rejection。
	h.mails.models[1] = &domain.MailModel{ID: 1, Name: "invite", Type: "invitation"}
	h.mails.models[2] = &domain.MailModel{ID: 2, Name: "offer", Type: "offer"}
	h.mails.models[3] = &domain.MailModel{ID: 3, Name: "thanks", Type: "rejection"}
	// 用户：7 = 申请人本人，8 = 他人。
	h.users.users[7] = &domain.User{ID: 7, Name: "owner"}
	h.users.users[7].Detail.Email = "owner@example.com"
	h.users.users[8] = &domain.User{ID: 8, Name: "other"}
	// 申请：1 归属用户 7。
	h.apps.apps[1] = &domain.Application{ID: 1, UserID: 7, DirectionCode: "back-end", Status: "applied", Email: "owner@example.com"}
	h.uc = NewRecruitmentInvitationUseCase(h.apps, h.invs, h.cands, h.users, h.mailer, h.mails)
	return h
}

// seedAcceptedInvitation 走真实发送路径拿一条 pending 邀请（含 token）。
func (h *harness) seedInvitation(t *testing.T) *response.InvitationResp {
	t.Helper()
	resp, err := h.uc.SendInvitation(context.Background(), 99, &request.SendInvitationReq{
		ApplicationID: 1,
		TemplateID:    1,
	})
	if err != nil {
		t.Fatalf("SendInvitation 预置失败: %v", err)
	}
	return resp
}

// tokenOf 取当前邀请行的 token（响应体出于保密不直接带 token，从仓储读）。
func (h *harness) tokenOf(t *testing.T, invitationID int) string {
	t.Helper()
	row, err := h.invs.GetByID(context.Background(), invitationID)
	if err != nil {
		t.Fatalf("读取邀请失败: %v", err)
	}
	return row.Token
}

// ------------------------------------------------------------------ *
// 验收场景 1：只有归属申请人能接受
// ------------------------------------------------------------------ *

func TestAcceptInvitationRejectsNonOwner(t *testing.T) {
	h := newHarness()
	inv := h.seedInvitation(t)
	token := h.tokenOf(t, inv.ID)

	_, err := h.uc.AcceptInvitation(context.Background(), 8, &request.AcceptInvitationReq{Token: token})
	if !errors.Is(err, ErrNotApplicationOwner) {
		t.Fatalf("他人接受应报 ErrNotApplicationOwner，实际: %v", err)
	}
	if h.cands.count() != 0 {
		t.Fatalf("他人接受不应产生候选人，当前 %d 条", h.cands.count())
	}
	row, _ := h.invs.GetByID(context.Background(), inv.ID)
	if row.Status != domain.InvitationPending {
		t.Fatalf("被拒绝的接受不应改变邀请状态: %s", row.Status)
	}

	// 本人接受应成功（对照路径）。
	resp, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: token})
	if err != nil {
		t.Fatalf("本人接受失败: %v", err)
	}
	if resp.Candidate.Status != string(domain.CandidateInterviewing) {
		t.Fatalf("本人接受后应为 interviewing，实际: %s", resp.Candidate.Status)
	}
}

// ------------------------------------------------------------------ *
// 验收场景 2：重复接受幂等
// ------------------------------------------------------------------ *

func TestAcceptInvitationIsIdempotent(t *testing.T) {
	h := newHarness()
	inv := h.seedInvitation(t)
	token := h.tokenOf(t, inv.ID)

	first, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: token})
	if err != nil {
		t.Fatalf("首次接受失败: %v", err)
	}
	second, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: token})
	if err != nil {
		t.Fatalf("重复接受应幂等成功，实际报错: %v", err)
	}
	if second.Candidate.ApplicationID != first.Candidate.ApplicationID {
		t.Fatalf("重复接受返回了不同候选人: %v vs %v", first, second)
	}
	if h.cands.count() != 1 {
		t.Fatalf("重复接受产生了 %d 条候选人，应为 1", h.cands.count())
	}
	events, _ := h.cands.Timeline(context.Background(), 1)
	if len(events) != 1 {
		t.Fatalf("重复接受写了 %d 条时间线，应为 1（初始行）", len(events))
	}
	if h.invs.count() != 1 {
		t.Fatalf("接受不应新建邀请行，当前 %d 条", h.invs.count())
	}
}

// ------------------------------------------------------------------ *
// 验收场景 3：过期 / 撤销不可接受
// ------------------------------------------------------------------ *

func TestAcceptInvitationExpiredRejected(t *testing.T) {
	h := newHarness()
	inv := h.seedInvitation(t)
	row, _ := h.invs.GetByID(context.Background(), inv.ID)
	row.ExpiresAt = time.Now().Add(-time.Hour) // 查询期判过期，无需改状态

	_, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: row.Token})
	if !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("过期邀请应报 ErrInvitationExpired，实际: %v", err)
	}
	if h.cands.count() != 0 {
		t.Fatalf("过期接受不应产生候选人")
	}
}

func TestAcceptInvitationRevokedRejected(t *testing.T) {
	h := newHarness()
	inv := h.seedInvitation(t)
	if _, err := h.uc.RevokeInvitation(context.Background(), inv.ID); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}
	token := h.tokenOf(t, inv.ID)

	_, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: token})
	if !errors.Is(err, ErrInvitationRevoked) {
		t.Fatalf("已撤销邀请应报 ErrInvitationRevoked，实际: %v", err)
	}
	if h.cands.count() != 0 {
		t.Fatalf("撤销后接受不应产生候选人")
	}
}

// ------------------------------------------------------------------ *
// 验收场景 4：邮件失败不误记成功
// ------------------------------------------------------------------ *

func TestSendInvitationMailFailureNotMarkedSent(t *testing.T) {
	h := newHarness()
	h.mailer.fail = true

	_, err := h.uc.SendInvitation(context.Background(), 99, &request.SendInvitationReq{
		ApplicationID: 1,
		TemplateID:    1,
	})
	if err == nil {
		t.Fatal("邮件失败时 SendInvitation 应返回错误")
	}
	if h.invs.count() != 1 {
		t.Fatalf("应保留 pending 邀请行供重试，当前 %d 条", h.invs.count())
	}
	row, _ := h.invs.GetByID(context.Background(), 1)
	if row.EmailSentAt != nil {
		t.Fatal("邮件失败不应落 email_sent_at（误记已邀请）")
	}
	if row.Status != domain.InvitationPending {
		t.Fatalf("邮件失败邀请状态应保持 pending，实际: %s", row.Status)
	}
}

func TestAdvanceCandidateMailFailureKeepsStatus(t *testing.T) {
	h := newHarness()
	// 预置一位 interviewing 候选人。
	h.cands.rows[10] = &domain.Candidate{ID: 10, ApplicationID: 1, UserID: 7, Status: domain.CandidateInterviewing}

	h.mailer.fail = true
	_, err := h.uc.AdvanceCandidate(context.Background(), 99, 10, &request.AdvanceCandidateReq{
		Action:     request.AdvanceActionSendOffer,
		TemplateID: 2,
	})
	if err == nil {
		t.Fatal("offer 邮件失败应返回错误")
	}
	if h.cands.rows[10].Status != domain.CandidateInterviewing {
		t.Fatalf("邮件失败状态不应流转，实际: %s", h.cands.rows[10].Status)
	}
	if events := h.cands.timeline[10]; len(events) != 0 {
		t.Fatalf("邮件失败不应写时间线，实际 %d 条", len(events))
	}

	// 重试：成功流转，且只产生一次流转。
	h.mailer.fail = false
	resp, err := h.uc.AdvanceCandidate(context.Background(), 99, 10, &request.AdvanceCandidateReq{
		Action:     request.AdvanceActionSendOffer,
		TemplateID: 2,
	})
	if err != nil {
		t.Fatalf("重试 offer 失败: %v", err)
	}
	if resp.Status != string(domain.CandidateOffered) {
		t.Fatalf("重试后应为 offered，实际: %s", resp.Status)
	}
	events := h.cands.timeline[10]
	if len(events) != 1 {
		t.Fatalf("重试只应产生 1 条流转记录，实际 %d 条", len(events))
	}
	if events[0].FromStatus != string(domain.CandidateInterviewing) || events[0].ToStatus != domain.CandidateOffered {
		t.Fatalf("流转记录方向错误: %+v", events[0])
	}
}

// ------------------------------------------------------------------ *
// 验收场景 5：安全重试不产生重复流转
// ------------------------------------------------------------------ *

func TestSendInvitationRetryReusesPendingRow(t *testing.T) {
	h := newHarness()
	h.mailer.fail = true
	_, err := h.uc.SendInvitation(context.Background(), 99, &request.SendInvitationReq{
		ApplicationID: 1,
		TemplateID:    1,
	})
	if err == nil {
		t.Fatal("首次发送应因邮件失败报错")
	}
	// 失败路径不返回响应体；保留的 pending 行直接从仓储读取。
	failed, _ := h.invs.GetPendingByApplication(context.Background(), 1)
	if failed == nil {
		t.Fatal("邮件失败后应保留 pending 邀请行供重试")
	}

	h.mailer.fail = false
	second, err := h.uc.SendInvitation(context.Background(), 99, &request.SendInvitationReq{
		ApplicationID: 1,
		TemplateID:    1,
	})
	if err != nil {
		t.Fatalf("重试发送失败: %v", err)
	}
	if second.ID != failed.ID {
		t.Fatalf("重试应复用同一条 pending 邀请，实际 %d vs %d", second.ID, failed.ID)
	}
	if h.invs.count() != 1 {
		t.Fatalf("重试产生了 %d 条邀请，应为 1", h.invs.count())
	}
	row, _ := h.invs.GetByID(context.Background(), failed.ID)
	if row.EmailSentAt == nil {
		t.Fatal("重试投递成功应落 email_sent_at")
	}
	// 一次成功发送 + 一次失败后的重试，共两次外发；token 在两次投递里保持同一行。
	if h.mailer.calls != 2 {
		t.Fatalf("外发次数应为 2，实际 %d", h.mailer.calls)
	}
}

// ------------------------------------------------------------------ *
// 状态机守卫：非法流转拒绝（hired 不能从 interviewing 直达）
// ------------------------------------------------------------------ *

func TestAdvanceCandidateInvalidTransition(t *testing.T) {
	h := newHarness()
	h.cands.rows[10] = &domain.Candidate{ID: 10, ApplicationID: 1, UserID: 7, Status: domain.CandidateInterviewing}

	_, err := h.uc.AdvanceCandidate(context.Background(), 99, 10, &request.AdvanceCandidateReq{
		Action: request.AdvanceActionHire,
	})
	if !errors.Is(err, ErrCandidateTransitionInvalid) {
		t.Fatalf("interviewing 直接 hire 应报 ErrCandidateTransitionInvalid，实际: %v", err)
	}
	if h.mailer.calls != 0 {
		t.Fatalf("非法 hire 不应触发外发")
	}
	if h.cands.rows[10].Status != domain.CandidateInterviewing {
		t.Fatalf("非法流转不应改变状态")
	}

	// 终态不可再流转。
	h.cands.rows[10].Status = domain.CandidateHired
	_, err = h.uc.AdvanceCandidate(context.Background(), 99, 10, &request.AdvanceCandidateReq{
		Action:     request.AdvanceActionReject,
		TemplateID: 3,
	})
	if !errors.Is(err, ErrCandidateTransitionInvalid) {
		t.Fatalf("hired 为终态，reject 应被拒绝，实际: %v", err)
	}
}

// ------------------------------------------------------------------ *
// 管理端与用户端视图一致（读同一张表）
// ------------------------------------------------------------------ *

func TestAdminAndUserViewsAgree(t *testing.T) {
	h := newHarness()
	inv := h.seedInvitation(t)
	token := h.tokenOf(t, inv.ID)
	if _, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: token}); err != nil {
		t.Fatalf("接受失败: %v", err)
	}

	mine, err := h.uc.ListMyCandidates(context.Background(), 7)
	if err != nil {
		t.Fatalf("用户端列表失败: %v", err)
	}
	if mine.Total != 1 || mine.Items[0].Status != string(domain.CandidateInterviewing) {
		t.Fatalf("用户端视图异常: %+v", mine)
	}

	admin, err := h.uc.ListCandidates(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("管理端列表失败: %v", err)
	}
	if admin.Total != 1 || admin.Items[0].Status != mine.Items[0].Status {
		t.Fatalf("两端状态不一致: admin=%+v mine=%+v", admin.Items, mine.Items)
	}

	// 用户端邀请列表应反映 accepted 状态。
	myInvs, err := h.uc.ListMyInvitations(context.Background(), 7)
	if err != nil {
		t.Fatalf("本人邀请列表失败: %v", err)
	}
	if myInvs.Total != 1 || myInvs.Items[0].Status != string(domain.InvitationAccepted) {
		t.Fatalf("本人邀请列表状态异常: %+v", myInvs.Items)
	}
}

// ------------------------------------------------------------------ *
// token 无效：拒绝且不落任何状态
// ------------------------------------------------------------------ *

func TestAcceptUnknownTokenRejected(t *testing.T) {
	h := newHarness()
	_, err := h.uc.AcceptInvitation(context.Background(), 7, &request.AcceptInvitationReq{Token: "not-a-real-token"})
	if !errors.Is(err, ErrInvitationToken) {
		t.Fatalf("无效 token 应报 ErrInvitationToken，实际: %v", err)
	}
	if h.cands.count() != 0 || h.invs.count() != 0 {
		t.Fatal("无效 token 不应产生任何行")
	}
}

// ------------------------------------------------------------------ *
// 模板类型守卫：非 invitation 模板不能用于发邀请
// ------------------------------------------------------------------ *

func TestSendInvitationRequiresInvitationTemplate(t *testing.T) {
	h := newHarness()
	_, err := h.uc.SendInvitation(context.Background(), 99, &request.SendInvitationReq{
		ApplicationID: 1,
		TemplateID:    2, // offer 模板
	})
	if !errors.Is(err, ErrTemplateNotInvitation) {
		t.Fatalf("用 offer 模板发邀请应报 ErrTemplateNotInvitation，实际: %v", err)
	}
	if h.invs.count() != 0 {
		t.Fatal("模板校验失败不应留下邀请行")
	}
}
