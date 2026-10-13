package domain

// 招新域的显式表名映射。
//
// 为什么需要这个文件：GORM 默认按结构体名复数化推导表名，而迁移 0007/0008
// 建的表带 `recruitment_` 前缀，两边对不上：
//
//	Application         → applications            ✗  实际表 recruitment_applications
//	ApplicationTimeline → application_timelines   ✗  实际表 recruitment_application_timeline
//	Invitation          → invitations             ✗  实际表 recruitment_invitations
//	Candidate           → candidates              ✗  实际表 recruitment_candidates
//	CandidateTimeline   → candidate_timelines     ✗  实际表 recruitment_candidate_timeline
//
// 症状是运行期 SQLSTATE 42P01（relation "applications" does not exist），
// 而桩单测不会碰数据库，所以只在真实连库时暴露。
//
// 修在结构体一侧而不是改迁移：schema 归迁移所有，0007/0008 已经应用并有种子数据，
// 改表名要新增迁移去 RENAME，代价与风险都更大；表名显式声明本来就比"猜复数形式"可靠。
// （RecruitmentRound / RecruitmentDirection 恰好与迁移一致，但仍在这里显式声明，
// 免得以后有人重命名结构体时静默换掉表名。）
//
// 每个结构体只声明一次，位置集中在此便于对照迁移；新增招新域实体请同时补上。

func (RecruitmentRound) TableName() string { return "recruitment_rounds" }

func (RecruitmentDirection) TableName() string { return "recruitment_directions" }

func (Application) TableName() string { return "recruitment_applications" }

func (ApplicationTimeline) TableName() string { return "recruitment_application_timeline" }

func (Invitation) TableName() string { return "recruitment_invitations" }

func (Candidate) TableName() string { return "recruitment_candidates" }

func (CandidateTimeline) TableName() string { return "recruitment_candidate_timeline" }
