package response

import "github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"

// RecruitmentDirectionFormResp 方向的面试表单定位信息（#135）。
//
// 只给前端三个值：点「面试」要跳哪张表（share_token）、以及该表当前开不开放
// （is_open）好让按钮按条件置灰。不给标题 / 字段等定义内容 —— 那些由填写页
// 按 share_token 自己读（GET /forms/public/:shareToken）。
//
// share_token 只是定位符、不是凭据（迁移 0009 注释已明确「不构成鉴权绕过」，
// 读取与提交都挂在 AuthRequired 之后），交给已登录用户无安全问题。
type RecruitmentDirectionFormResp struct {
	ID         int    `json:"id"`
	ShareToken string `json:"share_token"`
	IsOpen     bool   `json:"is_open"`
}

// RecruitmentDirectionResp 招新方向（用户端读）。
type RecruitmentDirectionResp struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Open     bool   `json:"open"`
	SortOrder int   `json:"sort_order"`
	// Form 是该方向的面试表单；该方向无开放表单时为 null（指针类型才能序列化出 null，
	// 值类型会变成 {"id":0,...} 这种假对象，前端无法用 form === null 判断）。
	Form *RecruitmentDirectionFormResp `json:"form"`
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
//
// form 由调用方（handler）另行组装：表单是表单域的读能力，方向 DTO 的构造函数
// 只认招新域，不认识 domain.Form（跨域依赖停在 handler 层，#135 §2.2）。
func NewRecruitmentDirectionResp(d *domain.RecruitmentDirection) RecruitmentDirectionResp {
	return RecruitmentDirectionResp{
		ID:       d.ID,
		Code:     d.Code,
		Name:     d.Name,
		Open:     d.Open,
		SortOrder: d.SortOrder,
	}
}

// NewRecruitmentDirectionFormResp 领域表单 → 方向上的面试表单 DTO。
func NewRecruitmentDirectionFormResp(f *domain.Form) *RecruitmentDirectionFormResp {
	if f == nil {
		return nil
	}
	return &RecruitmentDirectionFormResp{
		ID:         f.ID,
		ShareToken: f.ShareToken,
		IsOpen:     f.IsOpen,
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