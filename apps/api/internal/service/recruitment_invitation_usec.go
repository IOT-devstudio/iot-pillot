package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/request"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/dto/response"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/repository"
)

/* ------------------------------------------------------------------ *
 * 哨兵错误（handler 据此映射状态码）
 * ------------------------------------------------------------------ */

var (
	// ErrApplicationMissing 申请不存在。
	ErrApplicationMissing = errors.New("报名申请不存在")
	// ErrApplicationNotInvitable 申请状态不允许发邀请（如已 rejected）。
	ErrApplicationNotInvitable = errors.New("该申请当前状态不接受邀请")
	// ErrTemplateNotInvitation 模板不存在或类型不是 invitation。
	ErrTemplateNotInvitation = errors.New("模板不存在或类型不是 invitation")
	// ErrNoEmail 申请人没有邮箱，邀请无处投递。
	ErrNoEmail = errors.New("该申请人没有邮箱，请先补全资料")
	// ErrInvitationPending 已有 pending 邀请（投递失败的重试应走同一条发送接口）。
	ErrInvitationPending = errors.New("该申请已有一条待接受的邀请")
	// ErrInvitationNotPending 邀请不在可撤销状态。
	ErrInvitationNotPending = errors.New("邀请已处理或已过期，无法撤销")
	// ErrInvitationExpired 邀请已过期。
	ErrInvitationExpired = errors.New("邀请已过期，请联系管理员重新发送")
	// ErrInvitationRevoked 邀请已被撤销。
	ErrInvitationRevoked = errors.New("邀请已被撤销")
	// ErrInvitationAccepted 邀请已被接受（重复接受走幂等返回）。
	ErrInvitationAccepted = errors.New("邀请已被接受")
	// ErrNotApplicationOwner 当前账号不是该申请的归属人。
	ErrNotApplicationOwner = errors.New("当前账号与该申请不匹配")
	// ErrCandidateMissing 候选人不存在。
	ErrCandidateMissing = errors.New("候选人不存在")
	// ErrCandidateTransitionInvalid 候选人状态机不允许这次流转（含终态再流转）。
	// 命名避开 #88 申请域的 ErrInvalidTransition：同包两个状态机，各自的消息要能区分。
	ErrCandidateTransitionInvalid = errors.New("当前候选人状态不允许该操作")
	// ErrActionTemplateRequired send_offer / reject 动作缺少邮件模板。
	ErrActionTemplateRequired = errors.New("该操作需要指定邮件模板")
	// ErrInvitationToken 查无此邀请（用户端视角：token 无效）。
	ErrInvitationToken = errors.New("邀请无效或不存在")
)

/* ------------------------------------------------------------------ *
 * 邀请有效期
 * ------------------------------------------------------------------ */

const (
	// DefaultInvitationDays 邀请默认有效天数。
	DefaultInvitationDays = 7
	// MaxInvitationDays 邀请最长有效天数。
	MaxInvitationDays = 90
)

/* ------------------------------------------------------------------ *
 * 用例
 * ------------------------------------------------------------------ */

// ApplicationStore BE-04 对报名申请的的全部依赖：读取 + 接受邀请后的状态推进。
//
// 实现方是 #88 的 repository.RecruitmentRepo（申请域的正式所有者）；接口由
// 消费者声明、wire 按"返回值就是接口"连线 —— 与 BusinessMailer 同一模式。
type ApplicationStore interface {
	GetApplicationByID(ctx context.Context, id int) (*domain.Application, error)
	// UpdateApplicationStatus 推进申请状态并返回更新行数（#88 语义：行数 0 不算错误）。
	UpdateApplicationStatus(ctx context.Context, id int, status domain.ApplicationStatus) (int64, error)
}

// NewApplicationStore 把 #88 仓储作为 ApplicationStore 提供给装配层。
func NewApplicationStore(rec repository.RecruitmentRepo) ApplicationStore {
	return rec
}

// RecruitmentInvitationUseCase 邀请与候选人状态流转。
//
// 依赖的仓储只暴露本用例需要的方法；邮件出口走 MailModule 的
// BusinessMailer（渲染/投递/落记录复用同一路径），业务代码不接触 SMTP。
type RecruitmentInvitationUseCase struct {
	applications ApplicationStore
	invitations  repository.InvitationRepo
	candidates   repository.CandidateRepo
	users        repository.UserRepo
	mailer       BusinessMailer
	templates    repository.MailModelRepo
}

// NewRecruitmentInvitationUseCase 构造招新邀请用例。
func NewRecruitmentInvitationUseCase(
	applications ApplicationStore,
	invitations repository.InvitationRepo,
	candidates repository.CandidateRepo,
	users repository.UserRepo,
	mailer BusinessMailer,
	templates repository.MailModelRepo,
) *RecruitmentInvitationUseCase {
	return &RecruitmentInvitationUseCase{
		applications: applications,
		invitations:  invitations,
		candidates:   candidates,
		users:        users,
		mailer:       mailer,
		templates:    templates,
	}
}

/* ------------------------------------------------------------------ *
 * 管理端：发送 / 撤销邀请
 * ------------------------------------------------------------------ */

// SendInvitation 针对一个报名申请发送（或重试投递）邀请邮件。
//
// 行为规则：
//   - 申请不存在 / 没有邮箱 / 状态不允许 → 业务错误，不产生任何行。
//   - 已有 pending 邀请 → 复用该行重试投递（幂等），不新建、不改过期时间。
//   - 撤销过的邀请（revoked）→ 允许发一条新邀请（新 token、新有效期）。
//   - 邮件投递失败 → 不标记 email_sent_at，状态保持 pending，可再次重试。
//     业务上"已邀请"只在投递成功后成立。
func (uc *RecruitmentInvitationUseCase) SendInvitation(
	ctx context.Context,
	operatorID int,
	req *request.SendInvitationReq,
) (*response.InvitationResp, error) {
	app, err := uc.applications.GetApplicationByID(ctx, req.ApplicationID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return nil, ErrApplicationMissing
		}
		return nil, err
	}

	if app.Status == domain.ApplicationRejected {
		return nil, ErrApplicationNotInvitable
	}

	if strings.TrimSpace(app.Email) == "" {
		// 申请行没带邮箱时回落到账号资料邮箱：报名要求登录且账号邮箱不可改，
		// 两者都为空才是真的无处投递。
		if user, userErr := uc.users.GetByID(ctx, app.UserID); userErr == nil && user != nil {
			app.Email = user.Detail.Email
		}
	}
	if strings.TrimSpace(app.Email) == "" {
		return nil, ErrNoEmail
	}

	// 模板必须存在且类型为 invitation：发错类型（比如把 offer 模板当邀请发）
	// 会把变量渲染错，宁可发不出去让管理端换模板。
	model, err := uc.mailerTemplateCheck(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}

	expiresIn := req.ExpiresInDays
	if expiresIn <= 0 {
		expiresIn = DefaultInvitationDays
	}

	now := time.Now()
	pending, err := uc.invitations.GetPendingByApplication(ctx, app.ID)
	switch {
	case err == nil:
		// 幂等重试：复用 pending 行重投邮件；已成功投递过且未过期也允许重发
		// （收件人说没收到邮件是常态），但不重置过期时间。
		invitation := pending
		sendErr := uc.deliverInvitation(ctx, operatorID, model, invitation, app, req.Vars)
		if sendErr != nil {
			return nil, sendErr
		}
		invitation.EmailSentAt = &now
		if err := uc.invitations.Update(ctx, invitation); err != nil {
			return nil, err
		}
		return toInvitationResp(invitation), nil

	case errors.Is(err, repository.ErrInvitationNotFound):
		// 无 pending：新建一条。并发重复发送由 pending 部分唯一索引兜底，
		// 撞线方拿 ErrInvitationConflict → 映射 ErrInvitationPending。
		token, tokenErr := newInvitationToken()
		if tokenErr != nil {
			return nil, tokenErr
		}
		invitation := &domain.Invitation{
			ApplicationID: app.ID,
			Token:         token,
			Status:        domain.InvitationPending,
			ExpiresAt:     now.AddDate(0, 0, expiresIn),
			SentBy:        operatorID,
		}
		if err := uc.invitations.Create(ctx, invitation); err != nil {
			if errors.Is(err, repository.ErrInvitationConflict) {
				return nil, ErrInvitationPending
			}
			return nil, err
		}
		if sendErr := uc.deliverInvitation(ctx, operatorID, model, invitation, app, req.Vars); sendErr != nil {
			// 新行已建但邮件失败：保留 pending 行（email_sent_at 为空）供重试。
			return nil, sendErr
		}
		invitation.EmailSentAt = &now
		if err := uc.invitations.Update(ctx, invitation); err != nil {
			return nil, err
		}
		return toInvitationResp(invitation), nil

	default:
		return nil, err
	}
}

// RevokeInvitation 撤销一条邀请：pending（含已过期未处理的）落为 revoked。
//
// 撤销后申请回到"未邀请"语义，管理端可再次发送新邀请；已 accepted 的不可撤销。
func (uc *RecruitmentInvitationUseCase) RevokeInvitation(ctx context.Context, invitationID int) (*response.InvitationResp, error) {
	invitation, err := uc.invitations.GetByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, repository.ErrInvitationNotFound) {
			return nil, ErrInvitationToken
		}
		return nil, err
	}

	if invitation.Status != domain.InvitationPending {
		return nil, ErrInvitationNotPending
	}

	invitation.Status = domain.InvitationRevoked
	if err := uc.invitations.Update(ctx, invitation); err != nil {
		return nil, err
	}
	return toInvitationResp(invitation), nil
}

// ListInvitations 管理端邀请列表：可按申请过滤，分页。
func (uc *RecruitmentInvitationUseCase) ListInvitations(
	ctx context.Context,
	page int,
	pageSize int,
	applicationID int,
) (*response.InvitationListResp, error) {
	invitations, total, err := uc.invitations.List(ctx, page, pageSize, applicationID)
	if err != nil {
		return nil, err
	}
	items := make([]response.InvitationResp, 0, len(invitations))
	for _, invitation := range invitations {
		items = append(items, *toInvitationResp(invitation))
	}
	return &response.InvitationListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

/* ------------------------------------------------------------------ *
 * 用户端：本人邀请列表 / 接受
 * ------------------------------------------------------------------ */

// ListMyInvitations 当前登录用户的邀请列表：只含归属本人申请的邀请。
//
// 过期是查询期判定（Expired 字段由服务端按 now 算好），前端无需自行比对。
func (uc *RecruitmentInvitationUseCase) ListMyInvitations(
	ctx context.Context,
	userID int,
) (*response.MyInvitationListResp, error) {
	invitations, err := uc.invitations.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	items := make([]response.MyInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		app, err := uc.applications.GetApplicationByID(ctx, invitation.ApplicationID)
		if err != nil {
			// 申请行缺失是数据不一致（申请被清理等），跳过而不是 500 整个列表。
			log.Printf("[recruitment] 邀请 %d 关联的申请 %d 不存在，跳过", invitation.ID, invitation.ApplicationID)
			continue
		}
		items = append(items, response.MyInvitation{
			ApplicationID: app.ID,
			Direction:     app.DirectionCode,
			Status:        string(invitation.Status),
			Expired:       invitation.Expired(now),
			Token:         invitation.Token,
			ExpiresAt:     formatTime(invitation.ExpiresAt),
			AcceptedAt:    formatTimePtr(invitation.AcceptedAt),
			CreatedAt:     formatTime(invitation.CreatedAt),
		})
	}
	return &response.MyInvitationListResp{Items: items, Total: len(items)}, nil
}

// AcceptInvitation 接受邀请：登录 + 归属核验 + 状态机 + 幂等。
//
// 规则：
//   - token 查无 / 已撤销 / 已过期 → 明确业务错误；未登录在中间件层已 401。
//   - 申请归属人 = 当前登录用户，否则 403 语义错误（ErrNotApplicationOwner）。
//   - 已接受过 → 幂等成功：返回既有候选人视图，不重复流转。
//   - 并发接受 → candidates.application_id 唯一约束兜底，撞线方返回业务错误。
//   - 接受成功：邀请落 accepted，申请状态推进到 accepted，候选人 interviewing，
//     各写一条时间线。
func (uc *RecruitmentInvitationUseCase) AcceptInvitation(
	ctx context.Context,
	userID int,
	req *request.AcceptInvitationReq,
) (*response.AcceptInvitationResp, error) {
	invitation, err := uc.invitations.GetByToken(ctx, req.Token)
	if err != nil {
		if errors.Is(err, repository.ErrInvitationNotFound) {
			return nil, ErrInvitationToken
		}
		return nil, err
	}

	now := time.Now()
	if invitation.Status == domain.InvitationRevoked {
		return nil, ErrInvitationRevoked
	}
	if invitation.Status == domain.InvitationAccepted {
		// 幂等：重复接受返回当前候选人视图（不报错，不重复流转）。
		return uc.existingCandidateView(ctx, userID, invitation.ApplicationID)
	}
	if invitation.Expired(now) {
		return nil, ErrInvitationExpired
	}

	app, err := uc.applications.GetApplicationByID(ctx, invitation.ApplicationID)
	if err != nil {
		if errors.Is(err, repository.ErrApplicationNotFound) {
			return nil, ErrApplicationMissing
		}
		return nil, err
	}
	if app.UserID != userID {
		return nil, ErrNotApplicationOwner
	}

	// 已是候选人（如历史数据重复接受）：幂等返回，不重复建行。
	if existing, candErr := uc.candidates.GetByApplicationID(ctx, app.ID); candErr == nil && existing != nil {
		invitation.Status = domain.InvitationAccepted
		if invitation.AcceptedAt == nil {
			invitation.AcceptedAt = &now
		}
		if err := uc.invitations.Update(ctx, invitation); err != nil {
			return nil, err
		}
		return toAcceptResp(existing, app), nil
	}

	nowAccept := time.Now()
	invitation.Status = domain.InvitationAccepted
	invitation.AcceptedAt = &nowAccept
	if err := uc.invitations.Update(ctx, invitation); err != nil {
		return nil, err
	}

	candidate := &domain.Candidate{
		ApplicationID: app.ID,
		UserID:        app.UserID,
		Status:        domain.CandidateInterviewing,
	}
	if err := uc.candidates.Create(ctx, candidate); err != nil {
		if errors.Is(err, repository.ErrCandidateConflict) {
			// 并发接受：另一请求已建候选人。落回幂等视图。
			return uc.existingCandidateView(ctx, userID, app.ID)
		}
		return nil, err
	}

	uc.candidates.AppendTimeline(ctx, &domain.CandidateTimeline{
		CandidateID: candidate.ID,
		FromStatus:  "",
		ToStatus:    domain.CandidateInterviewing,
		Note:        "接受邀请进入面试流程",
		OperatorID:  userID,
	})

	// 申请状态推进 + 时间线（申请域归 #88；失败不阻断接受）。
	uc.advanceApplication(ctx, app, domain.ApplicationAccepted)

	return toAcceptResp(candidate, app), nil
}

/* ------------------------------------------------------------------ *
 * 管理端：候选人列表 / 详情 / 推进
 * ------------------------------------------------------------------ */

// ListCandidates 管理端候选人分页列表，可按状态过滤。
func (uc *RecruitmentInvitationUseCase) ListCandidates(
	ctx context.Context,
	page int,
	pageSize int,
	status string,
) (*response.CandidateListResp, error) {
	candidates, total, err := uc.candidates.List(ctx, page, pageSize, status)
	if err != nil {
		return nil, err
	}

	items := make([]response.CandidateResp, 0, len(candidates))
	for _, candidate := range candidates {
		item, err := uc.toCandidateResp(ctx, candidate)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return &response.CandidateListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CandidateDetail 管理端候选人详情：基础信息 + 时间线。
func (uc *RecruitmentInvitationUseCase) CandidateDetail(
	ctx context.Context,
	candidateID int,
) (*response.CandidateDetailResp, error) {
	candidate, err := uc.candidates.GetByID(ctx, candidateID)
	if err != nil {
		if errors.Is(err, repository.ErrCandidateNotFound) {
			return nil, ErrCandidateMissing
		}
		return nil, err
	}

	timeline, err := uc.candidates.Timeline(ctx, candidate.ID)
	if err != nil {
		return nil, err
	}

	resp, err := uc.toCandidateResp(ctx, candidate)
	if err != nil {
		return nil, err
	}

	items := make([]response.CandidateTimelineResp, 0, len(timeline))
	for _, event := range timeline {
		items = append(items, response.CandidateTimelineResp{
			FromStatus: event.FromStatus,
			ToStatus:   string(event.ToStatus),
			Note:       event.Note,
			OperatorID: event.OperatorID,
			CreatedAt:  formatTime(event.CreatedAt),
		})
	}
	return &response.CandidateDetailResp{Candidate: *resp, Timeline: items}, nil
}

// AdvanceCandidate 推进候选人状态。三个动作：
//
//	send_offer：interviewing → offered，offer 邮件投递成功才落状态；
//	hire：offered → hired，不发信；
//	reject：interviewing|offered → rejected，感谢信投递成功才落状态。
//
// 邮件失败时状态保持不变（ErrMailDelivery 透传），可安全重试——
// 重试只影响邮件投递，不产生第二次状态流转，因为第一次流转根本没发生。
func (uc *RecruitmentInvitationUseCase) AdvanceCandidate(
	ctx context.Context,
	operatorID int,
	candidateID int,
	req *request.AdvanceCandidateReq,
) (*response.CandidateResp, error) {
	candidate, err := uc.candidates.GetByID(ctx, candidateID)
	if err != nil {
		if errors.Is(err, repository.ErrCandidateNotFound) {
			return nil, ErrCandidateMissing
		}
		return nil, err
	}

	switch req.Action {
	case request.AdvanceActionSendOffer:
		return uc.advanceWithMail(ctx, operatorID, candidate, domain.CandidateOffered, req, "发送 offer")
	case request.AdvanceActionHire:
		if !candidate.Status.CanTransitionTo(domain.CandidateHired) {
			return nil, ErrCandidateTransitionInvalid
		}
		return uc.applyTransition(ctx, operatorID, candidate, domain.CandidateHired, req.Note)
	case request.AdvanceActionReject:
		return uc.advanceWithMail(ctx, operatorID, candidate, domain.CandidateRejected, req, "发送感谢信")
	default:
		// binding 层已限三值，这里兜底。
		return nil, ErrCandidateTransitionInvalid
	}
}

// advanceWithMail 发信 → 成功才流转的共用路径（send_offer / reject）。
func (uc *RecruitmentInvitationUseCase) advanceWithMail(
	ctx context.Context,
	operatorID int,
	candidate *domain.Candidate,
	target domain.CandidateStatus,
	req *request.AdvanceCandidateReq,
	actionName string,
) (*response.CandidateResp, error) {
	if !candidate.Status.CanTransitionTo(target) {
		return nil, ErrCandidateTransitionInvalid
	}
	if req.TemplateID <= 0 {
		return nil, ErrActionTemplateRequired
	}

	user, err := uc.users.GetByID(ctx, candidate.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("%w: 候选人账号已不存在", ErrCandidateMissing)
		}
		return nil, err
	}

	// 邮件先行：失败即返回，状态与时间线都不动。
	if _, err := uc.mailer.SendByTemplate(ctx, operatorID, req.TemplateID, user, req.Vars); err != nil {
		return nil, err
	}

	return uc.applyTransition(ctx, operatorID, candidate, target, req.Note)
}

// applyTransition 落状态 + 时间线。
func (uc *RecruitmentInvitationUseCase) applyTransition(
	ctx context.Context,
	operatorID int,
	candidate *domain.Candidate,
	target domain.CandidateStatus,
	note string,
) (*response.CandidateResp, error) {
	from := candidate.Status
	candidate.Status = target
	if err := uc.candidates.Update(ctx, candidate); err != nil {
		return nil, err
	}

	uc.candidates.AppendTimeline(ctx, &domain.CandidateTimeline{
		CandidateID: candidate.ID,
		FromStatus:  string(from),
		ToStatus:    target,
		Note:        note,
		OperatorID:  operatorID,
	})

	return uc.toCandidateResp(ctx, candidate)
}

/* ------------------------------------------------------------------ *
 * 用户端：本人候选人记录
 * ------------------------------------------------------------------ */

// ListMyCandidates 当前登录用户的候选人记录（经申请归属过滤）。
func (uc *RecruitmentInvitationUseCase) ListMyCandidates(
	ctx context.Context,
	userID int,
) (*response.MyCandidateListResp, error) {
	candidates, err := uc.candidates.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]response.MyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		app, err := uc.applications.GetApplicationByID(ctx, candidate.ApplicationID)
		if err != nil {
			log.Printf("[recruitment] 候选人 %d 关联的申请 %d 不存在，跳过", candidate.ID, candidate.ApplicationID)
			continue
		}
		items = append(items, response.MyCandidate{
			ApplicationID: app.ID,
			Direction:     app.DirectionCode,
			Status:        string(candidate.Status),
			CreatedAt:     formatTime(candidate.CreatedAt),
			UpdatedAt:     formatTime(candidate.UpdatedAt),
		})
	}
	return &response.MyCandidateListResp{Items: items, Total: len(items)}, nil
}

/* ------------------------------------------------------------------ *
 * 内部
 * ------------------------------------------------------------------ */

// deliverInvitation 渲染并发送邀请邮件；失败返回错误，不改动邀请行。
//
// 预填变量：模板常见变量照常预填（收件人资料），额外注入 invitation_token
// 与有效期，供「点此接受」类正文使用；调用方传同键覆盖。
func (uc *RecruitmentInvitationUseCase) deliverInvitation(
	ctx context.Context,
	operatorID int,
	model *domain.MailModel,
	invitation *domain.Invitation,
	app *domain.Application,
	overrides request.MailVars,
) error {
	user, err := uc.users.GetByID(ctx, app.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return fmt.Errorf("%w: 申请人账号已不存在", ErrNoEmail)
		}
		return err
	}

	vars := request.MailVars{}
	for key, value := range overrides {
		vars[key] = value
	}
	vars["invitation_token"] = invitation.Token
	vars["invitation_expires_at"] = invitation.ExpiresAt.Format("2006-01-02 15:04")

	if _, err := uc.mailer.SendByTemplate(ctx, operatorID, model.ID, user, vars); err != nil {
		return err
	}
	return nil
}

// mailerTemplateCheck 校验模板存在且类型为 invitation，返回模板行。
func (uc *RecruitmentInvitationUseCase) mailerTemplateCheck(ctx context.Context, templateID int) (*domain.MailModel, error) {
	model, err := uc.templates.GetByID(ctx, templateID)
	if err != nil {
		return nil, ErrTemplateNotInvitation
	}
	if model.Type != "invitation" {
		return nil, ErrTemplateNotInvitation
	}
	return model, nil
}// existingCandidateView 幂等接受返回：查既有候选人并组装响应。
func (uc *RecruitmentInvitationUseCase) existingCandidateView(
	ctx context.Context,
	userID int,
	applicationID int,
) (*response.AcceptInvitationResp, error) {
	candidate, err := uc.candidates.GetByApplicationID(ctx, applicationID)
	if err != nil {
		if errors.Is(err, repository.ErrCandidateNotFound) {
			// 理论上不可达（accepted 必有候选人）；兜底成明确错误。
			return nil, ErrCandidateMissing
		}
		return nil, err
	}
	app, err := uc.applications.GetApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationMissing
	}
	_ = userID
	return toAcceptResp(candidate, app), nil
}

// advanceApplication 接受邀请后推进申请状态（申请域归 #88；失败只记日志）。
//
// 行数 0（并发下已被流转到终态等）不是错误：#88 的语义由上层领域规则判断，
// 邀请与候选人已落库，回滚只会制造不一致。
func (uc *RecruitmentInvitationUseCase) advanceApplication(ctx context.Context, app *domain.Application, toStatus domain.ApplicationStatus) {
	app.Status = toStatus
	if _, err := uc.applications.UpdateApplicationStatus(ctx, app.ID, toStatus); err != nil {
		log.Printf("[recruitment] 申请 %d 状态推进失败（仅记录）: %v", app.ID, err)
	}
}

// toCandidateResp 组装候选人视图（补申请人姓名/邮箱/方向）。
func (uc *RecruitmentInvitationUseCase) toCandidateResp(ctx context.Context, candidate *domain.Candidate) (*response.CandidateResp, error) {
	app, err := uc.applications.GetApplicationByID(ctx, candidate.ApplicationID)
	if err != nil {
		return nil, err
	}
	return &response.CandidateResp{
		ID:            candidate.ID,
		ApplicationID: candidate.ApplicationID,
		UserID:        candidate.UserID,
		Name:          app.Name,
		Email:         app.Email,
		Direction:     app.DirectionCode,
		Status:        string(candidate.Status),
		CreatedAt:     formatTime(candidate.CreatedAt),
		UpdatedAt:     formatTime(candidate.UpdatedAt),
	}, nil
}

// toInvitationResp 邀请行 → 管理端视图。
func toInvitationResp(invitation *domain.Invitation) *response.InvitationResp {
	return &response.InvitationResp{
		ID:            invitation.ID,
		ApplicationID: invitation.ApplicationID,
		Status:        string(invitation.Status),
		ExpiresAt:     formatTime(invitation.ExpiresAt),
		EmailSentAt:   formatTimePtr(invitation.EmailSentAt),
		SentBy:        invitation.SentBy,
		AcceptedAt:    formatTimePtr(invitation.AcceptedAt),
		CreatedAt:     formatTime(invitation.CreatedAt),
	}
}

// toAcceptResp 接受结果组装。
func toAcceptResp(candidate *domain.Candidate, app *domain.Application) *response.AcceptInvitationResp {
	return &response.AcceptInvitationResp{
		Candidate: response.MyCandidate{
			ApplicationID: app.ID,
			Direction:     app.DirectionCode,
			Status:        string(candidate.Status),
			CreatedAt:     formatTime(candidate.CreatedAt),
			UpdatedAt:     formatTime(candidate.UpdatedAt),
		},
	}
}

// formatTime 时间戳统一 RFC3339 序列化；零值返回空串。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// formatTimePtr 可空时间戳序列化；nil 返回空串。
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// newInvitationToken 生成 128-bit 随机 token（hex），不可预测、不可枚举。
func newInvitationToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成邀请凭据失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
