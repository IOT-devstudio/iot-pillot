package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// LearningRoadmap 一条学习路线（#223，ADR 0006）。
//
// 阶段与资源以 JSONB 整体读写，没有独立查询需求：没有任何页面会问
// 「所有包含某资源链接的阶段」，也没有跨路线的资源复用。
type LearningRoadmap struct {
	ID            int           `json:"id" gorm:"primaryKey"`
	Title         string        `json:"title" gorm:"column:title"`
	Body          string        `json:"body" gorm:"column:body"`
	DirectionCode string        `json:"direction_code" gorm:"column:direction_code"`
	Stages        RoadmapStages `json:"stages" gorm:"column:stages;type:jsonb"`
	Version       int           `json:"version" gorm:"column:version"`
	AuthorID      int           `json:"author_id" gorm:"column:author_id"`
	CreatedAt     time.Time     `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time     `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`

	// 只有软删（ADR 0006 决策 5）：用户进度行指向路线，硬删会留下悬空引用。
	DeletedAt *time.Time `json:"-" gorm:"column:deleted_at"`
	DeletedBy *int       `json:"-" gorm:"column:deleted_by"`
}

// TableName 固定表名（GORM 默认会猜成 learning_roadmaps，这里显式写出以防改名）。
func (LearningRoadmap) TableName() string { return "learning_roadmaps" }

// RoadmapStage 一个学习阶段。
//
// ID 由**服务端**生成（ADR 0006 决策 3）：用户的学习进度记的是阶段 id 而不是序号。
// 管理员把第 2 阶段移到第 1 位时，用户的进度仍指向原来那个阶段；
// 用序号的话重排会让进度静默指到别的阶段。也不用 s_1/s_2 这类顺序号：
// 删掉中间阶段后再新增会复用已删除的 id，进度就会指到新阶段上。
type RoadmapStage struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Summary   string            `json:"summary"`
	Resources []RoadmapResource `json:"resources"`
}

// RoadmapResource 阶段下的一个资源链接。
type RoadmapResource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// RoadmapStages 阶段数组别名，便于 JSONB 序列化与校验。
//
// 必须自己实现 driver.Valuer / sql.Scanner：GORM 看到切片会默认按
// PostgreSQL **数组**字面量（{...}）写库，而 stages 列是 jsonb。
// 那种写法会被列上的 `jsonb_typeof(stages) = 'array'` 约束直接拒掉
// （SQLSTATE 23514）；空切片更糟——写进去的是 NULL，撞 NOT NULL（23502）。
// 表现出来就是新建路线一律 5000，且与内容无关。
type RoadmapStages []RoadmapStage

// Value 按 JSON 写库，nil/空切片一律存成 `[]` 而不是 NULL。
//
// 空切片存 `[]` 而不是 NULL 很关键：列是 NOT NULL，且约束要求 jsonb 是数组。
// 「新建一条还没加阶段就保存」是最常见的路径，不能在这里失败。
func (s RoadmapStages) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan 按 JSON 读库。库里的 NULL 按空数组处理，避免读到脏数据时整条路线打不开。
func (s *RoadmapStages) Scan(src any) error {
	if src == nil {
		*s = RoadmapStages{}
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("learning_roadmap: 无法解析 stages（类型 %T）", src)
	}
	if len(raw) == 0 {
		*s = RoadmapStages{}
		return nil
	}
	return json.Unmarshal(raw, s)
}

// 学习路线的边界与错误。
const (
	// RoadmapMaxStages 单条路线最多阶段数（AC 明确要求 20）。
	RoadmapMaxStages = 20
	// RoadmapMaxResourcesPerStage 每个阶段最多资源数（AC 明确要求 20）。
	RoadmapMaxResourcesPerStage = 20
	// RoadmapTitleMaxRunes 标题长度上限。
	RoadmapTitleMaxRunes = 200
)

var (
	// ErrRoadmapNotFound 路线不存在或已软删。
	ErrRoadmapNotFound = errors.New("学习路线不存在")
	// ErrRoadmapVersionConflict 版本冲突：有人先保存过。
	ErrRoadmapVersionConflict = errors.New("学习路线已被他人修改，请刷新后重试")
	// ErrRoadmapInvalid 路线内容不合法（具体原因由 RoadmapValidationError 携带）。
	ErrRoadmapInvalid = errors.New("学习路线内容不合法")
)

// RoadmapValidationError 带可读原因的内容错误。
//
// 单独一个类型而不是复用 ErrRoadmapInvalid：handler 要把具体原因回给用户
// （「阶段最多 20 个」比「内容不合法」有用得多），而 errors.Is 仍能匹配哨兵。
type RoadmapValidationError struct {
	Reason string
}

func (e *RoadmapValidationError) Error() string { return e.Reason }

// Unwrap 让 errors.Is(err, ErrRoadmapInvalid) 成立。
func (e *RoadmapValidationError) Unwrap() error { return ErrRoadmapInvalid }
