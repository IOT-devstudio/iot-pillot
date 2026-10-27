package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// StudioMember 工作室成员档案（迁移 0013）。
//
// 与招新候选人（recruitment_candidates）**分开**建表（ADR 0001）：
//   - user_id 可空，允许收录没有系统账号的校友/外部人员；
//   - 唯一键是 (name, birth_year)，同名不同届可以共存；
//   - 毕业/退出用 archived_at + is_active 归档，不物理删除，
//     以免丢掉后续项目参与历史的锚点。
type StudioMember struct {
	ID int `json:"id" gorm:"primaryKey"`
	// UserID 可空：NULL = 没有系统账号。
	UserID            *int       `json:"user_id" gorm:"column:user_id"`
	Name              string     `json:"name" gorm:"column:name"`
	BirthYear         int        `json:"birth_year" gorm:"column:birth_year"`
	Cohort            string     `json:"cohort" gorm:"column:cohort"`
	Direction         string     `json:"direction" gorm:"column:direction"`
	RoleInStudio      string     `json:"role_in_studio" gorm:"column:role_in_studio"`
	Internship        string     `json:"internship" gorm:"column:internship"`
	Employment        string     `json:"employment" gorm:"column:employment"`
	IsActive          bool       `json:"is_active" gorm:"column:is_active"`
	ArchivedAt        *time.Time `json:"archived_at" gorm:"column:archived_at"`
	SourceCandidateID *int       `json:"source_candidate_id" gorm:"column:source_candidate_id"`
	CreatedAt         time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 显式表名。
//
// 与 domain/table_names.go 同一理由：GORM 的复数推导得到 studio_members 虽然
// 恰好正确，但表名归迁移所有，显式声明后重命名结构体不会静默换表。
func (StudioMember) TableName() string { return "studio_members" }

/* ------------------------------------------------------------------ *
 * 字段口径（ADR 0001 / 检票报告 §5）
 * ------------------------------------------------------------------ */

const (
	// StudioMemberNameMaxRunes 姓名上限。
	//
	// 刻意与注册姓名的「2–20 全中文」**解耦**：校友可能用英文名，
	// 也可能带间隔号，档案是人工录入而不是账号凭据。
	StudioMemberNameMaxRunes = 40
	// StudioMemberCohortMaxRunes 届别（如「2024 级」）上限。
	StudioMemberCohortMaxRunes = 40
	// StudioMemberDirectionMaxRunes 方向文本上限。
	StudioMemberDirectionMaxRunes = 40
	// StudioMemberRoleMaxRunes 工作室职能上限。
	StudioMemberRoleMaxRunes = 60
	// StudioMemberOptionalMaxRunes 实习 / 就业去向这类自由文本的上限。
	StudioMemberOptionalMaxRunes = 200

	// StudioMemberBirthYearMin / Max 出生年合理区间。
	StudioMemberBirthYearMin = 1900
	StudioMemberBirthYearMax = 2100
)

// ErrStudioMemberInvalid 成员档案字段不合法（消息里带具体原因，直接给管理端看）。
//
// 用哨兵而不是裸 errors.New：handler 必须能用 errors.Is 认出「这是校验失败」
// 并映射成 400，而不是掉进 500 的兜底分支。
var ErrStudioMemberInvalid = errors.New("成员档案不合法")

// StudioMemberFields 一次录入 / 更新的字段快照（校验的输入）。
//
// 单独抽出来而不是直接校验实体：服务层要先校验、再决定写哪些列，
// 校验函数因此必须是纯的（没有指针语义、不依赖 DB）。
type StudioMemberFields struct {
	Name         string
	BirthYear    int
	Cohort       string
	Direction    string
	RoleInStudio string
	Internship   string
	Employment   string
}

// Normalize 去掉首尾空白并归一化可选字段。
//
// 名字与职能前后带空格是最常见的录入错误；统一 Trim 后写库，
// 否则「张三」与「张三 」会绕过 (name, birth_year) 唯一键变成两条档案。
func (f StudioMemberFields) Normalize() StudioMemberFields {
	f.Name = strings.TrimSpace(f.Name)
	f.Cohort = strings.TrimSpace(f.Cohort)
	f.Direction = strings.TrimSpace(f.Direction)
	f.RoleInStudio = strings.TrimSpace(f.RoleInStudio)
	f.Internship = strings.TrimSpace(f.Internship)
	f.Employment = strings.TrimSpace(f.Employment)
	return f
}

// Validate 校验字段口径，返回带可读原因的包装错误。
//
// 一律用 ErrStudioMemberInvalid 包裹，调用方 errors.Is 一次即可识别。
func (f StudioMemberFields) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("%w: 姓名必填", ErrStudioMemberInvalid)
	}
	if utf8.RuneCountInString(f.Name) > StudioMemberNameMaxRunes {
		return fmt.Errorf("%w: 姓名最多 %d 个字符", ErrStudioMemberInvalid, StudioMemberNameMaxRunes)
	}
	if f.BirthYear < StudioMemberBirthYearMin || f.BirthYear > StudioMemberBirthYearMax {
		return fmt.Errorf("%w: 出生年须在 %d–%d 之间", ErrStudioMemberInvalid, StudioMemberBirthYearMin, StudioMemberBirthYearMax)
	}
	if f.Cohort == "" {
		return fmt.Errorf("%w: 届别必填（如「2024 级」）", ErrStudioMemberInvalid)
	}
	if utf8.RuneCountInString(f.Cohort) > StudioMemberCohortMaxRunes {
		return fmt.Errorf("%w: 届别最多 %d 个字符", ErrStudioMemberInvalid, StudioMemberCohortMaxRunes)
	}
	if f.Direction == "" {
		return fmt.Errorf("%w: 方向必填", ErrStudioMemberInvalid)
	}
	if utf8.RuneCountInString(f.Direction) > StudioMemberDirectionMaxRunes {
		return fmt.Errorf("%w: 方向最多 %d 个字符", ErrStudioMemberInvalid, StudioMemberDirectionMaxRunes)
	}
	// 职能必填且至少 1 个字符：迁移里的 DEFAULT '' 只是为了让加列不阻塞历史行，
	// 不代表空串是合法业务值（ADR 0001 明确「可空串禁止」）。
	if f.RoleInStudio == "" {
		return fmt.Errorf("%w: 工作室职能必填", ErrStudioMemberInvalid)
	}
	if utf8.RuneCountInString(f.RoleInStudio) > StudioMemberRoleMaxRunes {
		return fmt.Errorf("%w: 工作室职能最多 %d 个字符", ErrStudioMemberInvalid, StudioMemberRoleMaxRunes)
	}
	if utf8.RuneCountInString(f.Internship) > StudioMemberOptionalMaxRunes {
		return fmt.Errorf("%w: 实习去向最多 %d 个字符", ErrStudioMemberInvalid, StudioMemberOptionalMaxRunes)
	}
	if utf8.RuneCountInString(f.Employment) > StudioMemberOptionalMaxRunes {
		return fmt.Errorf("%w: 就业去向最多 %d 个字符", ErrStudioMemberInvalid, StudioMemberOptionalMaxRunes)
	}
	return nil
}
