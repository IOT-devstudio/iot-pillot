package response

import "github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

// RecruitmentDirectionResp 招新方向（用户端读）。
type RecruitmentDirectionResp struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Open     bool   `json:"open"`
	SortOrder int   `json:"sort_order"`
}

// RecruitmentDirectionListResp 方向列表。
type RecruitmentDirectionListResp struct {
	Items []RecruitmentDirectionResp `json:"items"`
}

// RecruitmentRoundResp 招新轮次（管理端读）。
type RecruitmentRoundResp struct {
	ID        int    `json:"id"`
	RoundNum  int    `json:"round_num"`
	Name      string `json:"name"`
	Accepting bool   `json:"accepting"`
}

// RecruitmentRoundListResp 轮次列表。
type RecruitmentRoundListResp struct {
	Items []RecruitmentRoundResp `json:"items"`
}

// RecruitmentApplicationResp 报名申请。
//
// 用户端与管理端共用同一份形状，但读取权限由中间件 + 服务层校验控制：
// 用户端只能通过 ListMyApplications / GetMyApplication 读自己。
type RecruitmentApplicationResp struct {
	ID             int    `json:"id"`
	UserID         int    `json:"user_id"`
	RoundID        int    `json:"round_id"`
	DirectionCode  string `json:"direction_code"`
	Status         string `json:"status"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Class          string `json:"class"`
	StudentID      int64  `json:"student_id"`
	QQ             string `json:"qq"`
	Note           string `json:"note"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// RecruitmentTimelineResp 时间线一条。
type RecruitmentTimelineResp struct {
	ID            int    `json:"id"`
	ToStatus      string `json:"to_status"`
	CreatedAt     string `json:"created_at"`
}

// RecruitmentApplicationDetailResp 申请详情（含时间线）。
type RecruitmentApplicationDetailResp struct {
	Application RecruitmentApplicationResp     `json:"application"`
	Timeline   []RecruitmentTimelineResp      `json:"timeline"`
}

// RecruitmentApplicationListResp 申请分页列表。
type RecruitmentApplicationListResp struct {
	Items    []RecruitmentApplicationResp `json:"items"`
	Total    int64                        `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

// NewRecruitmentDirectionResp 领域方向 → 响应 DTO。
func NewRecruitmentDirectionResp(d *domain.RecruitmentDirection) RecruitmentDirectionResp {
	return RecruitmentDirectionResp{
		ID:       d.ID,
		Code:     d.Code,
		Name:     d.Name,
		Open:     d.Open,
		SortOrder: d.SortOrder,
	}
}

// NewRecruitmentRoundResp 领域轮次 → 响应 DTO。
func NewRecruitmentRoundResp(r *domain.RecruitmentRound) RecruitmentRoundResp {
	return RecruitmentRoundResp{
		ID:        r.ID,
		RoundNum:  r.RoundNum,
		Name:      r.Name,
		Accepting: r.Accepting,
	}
}

// NewRecruitmentApplicationResp 领域申请 → 响应 DTO。
func NewRecruitmentApplicationResp(a *domain.Application) RecruitmentApplicationResp {
	return RecruitmentApplicationResp{
		ID:            a.ID,
		UserID:        a.UserID,
		RoundID:       a.RoundID,
		DirectionCode: a.DirectionCode,
		Status:        string(a.Status),
		Name:          a.Name,
		Email:         a.Email,
		Class:         a.Class,
		StudentID:     a.StudentID,
		QQ:            a.QQ,
		Note:          a.Note,
		CreatedAt:     a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// NewRecruitmentTimelineResp 领域时间线 → 响应 DTO。
func NewRecruitmentTimelineResp(t *domain.ApplicationTimeline) RecruitmentTimelineResp {
	return RecruitmentTimelineResp{
		ID:        t.ID,
		ToStatus:   t.ToStatus,
		CreatedAt:  t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// NewRecruitmentApplicationDetailResp 拼出申请详情。
func NewRecruitmentApplicationDetailResp(a *domain.Application, tl []*domain.ApplicationTimeline) RecruitmentApplicationDetailResp {
	timeline := make([]RecruitmentTimelineResp, 0, len(tl))
	for _, t := range tl {
		timeline = append(timeline, NewRecruitmentTimelineResp(t))
	}
	return RecruitmentApplicationDetailResp{
		Application: NewRecruitmentApplicationResp(a),
		Timeline:   timeline,
	}
}