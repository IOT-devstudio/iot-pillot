package domain

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"strings"
)

/* ------------------------------------------------------------------ *
 * 阶段 id 的生成与对齐（ADR 0006 决策 3）
 * ------------------------------------------------------------------ */

// roadmapStageIDAlphabet base36 字母表。
const roadmapStageIDAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// roadmapStageIDChars id 的随机部分长度（`s_` + 8 位）。
const roadmapStageIDChars = 8

// NewRoadmapStageID 生成一个阶段 id：`s_` + 8 位 base36 随机串。
//
// 用随机而不是自增序号：顺序号在「删掉中间阶段后再新增」时会复用已删除的 id，
// 用户的进度就会静默指到一个全新的阶段上——比序号错位更难发现。
func NewRoadmapStageID() string {
	buf := make([]byte, roadmapStageIDChars)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 失败在正常系统里不会发生；真发生了就 panic，
		// 因为此时任何 id 都可能重复，而重复 id 会静默串掉用户进度。
		panic(fmt.Sprintf("生成阶段 id 失败: %v", err))
	}
	var sb strings.Builder
	sb.WriteString("s_")
	for _, b := range buf {
		sb.WriteByte(roadmapStageIDAlphabet[int(b)%len(roadmapStageIDAlphabet)])
	}
	return sb.String()
}

// IsValidRoadmapStageID 判断是不是本系统生成的阶段 id 形状。
//
// 只做形状校验，不查存在性：对齐规则里「有 id 则保留」用的是形状判断，
// 存在性由调用方（保存时的原 stages）决定。
func IsValidRoadmapStageID(id string) bool {
	if len(id) != 2+roadmapStageIDChars || !strings.HasPrefix(id, "s_") {
		return false
	}
	for _, c := range id[2:] {
		if !strings.ContainsRune(roadmapStageIDAlphabet, c) {
			return false
		}
	}
	return true
}

// AlignStageIDs 对齐阶段 id：有 id 则保留、无 id 则生成、旧有新无则丢弃。
//
// previous 是库里当前的阶段列表。返回新列表（不改动入参）。
//
// 哪些 id 允许保留：只在 previous 里出现过的 id 才认。这样前端伪造一个
// 看起来合法但库里没有的 id 不会凭空造出「已在别处使用」的进度指向——
// 它只会被当成新阶段，拿到一个新 id。代价是拿不到「保留原 id」的收益，
// 但那是错误输入，按新阶段处理是安全的。
func AlignStageIDs(next RoadmapStages, previous RoadmapStages) RoadmapStages {
	known := make(map[string]bool, len(previous))
	for _, s := range previous {
		if s.ID != "" {
			known[s.ID] = true
		}
	}

	out := make(RoadmapStages, 0, len(next))
	// 同一次提交里若出现重复 id，后一个改发新 id，避免同一路线内出现两个同号阶段。
	used := make(map[string]bool, len(next))
	for _, s := range next {
		st := s
		if st.ID == "" || !known[st.ID] || used[st.ID] {
			st.ID = NewRoadmapStageID()
		}
		used[st.ID] = true
		out = append(out, st)
	}
	return out
}

/* ------------------------------------------------------------------ *
 * 内容校验
 * ------------------------------------------------------------------ */

// ValidateRoadmapStages 校验阶段数组的结构与边界。
//
// 返回 *RoadmapValidationError 时带可读原因（handler 直接回给用户）。
func ValidateRoadmapStages(stages RoadmapStages) error {
	if len(stages) > RoadmapMaxStages {
		return &RoadmapValidationError{
			Reason: fmt.Sprintf("阶段最多 %d 个，当前 %d 个", RoadmapMaxStages, len(stages)),
		}
	}

	for i, s := range stages {
		// 用序号而不是 id 报错：id 对管理员是不可见的技术细节，
		// 「第 3 个阶段」才是他能定位的东西。
		pos := i + 1
		if strings.TrimSpace(s.Title) == "" {
			return &RoadmapValidationError{Reason: fmt.Sprintf("第 %d 个阶段缺少标题", pos)}
		}
		if len(s.Resources) > RoadmapMaxResourcesPerStage {
			return &RoadmapValidationError{
				Reason: fmt.Sprintf("第 %d 个阶段最多 %d 个资源，当前 %d 个",
					pos, RoadmapMaxResourcesPerStage, len(s.Resources)),
			}
		}
		for j, res := range s.Resources {
			rpos := j + 1
			if strings.TrimSpace(res.Title) == "" {
				return &RoadmapValidationError{
					Reason: fmt.Sprintf("第 %d 个阶段的第 %d 个资源缺少标题", pos, rpos),
				}
			}
			if !IsAllowedResourceURL(res.URL) {
				return &RoadmapValidationError{
					Reason: fmt.Sprintf(
						"第 %d 个阶段的第 %d 个资源链接不合规：只允许 http/https 或站内相对路径",
						pos, rpos),
				}
			}
		}
	}
	return nil
}

// IsAllowedResourceURL 资源链接的协议白名单（ADR 0006 决策 7 / ADR 0005 决策 5）。
//
// 与前端 protocolAllowlist.ts 的 isAllowedUrl 同口径：
// 只允许 http: / https: / 以 "/" 开头的站内相对路径。
//
// 为什么在后端再判一次而不是只靠前端：AC 明确要求「不在渲染层兜底」。
// 渲染层只决定「怎么显示」，能不能存进库是数据完整性问题，
// 存进去之后任何消费方（导出、邮件、其它前端）都会继承这个脏数据。
func IsAllowedResourceURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	// 站内相对路径：与前端一致，仅以单个 "/" 开头。
	// 排除 "//host" 形式（协议相对 URL 会跳到外部站点）。
	if strings.HasPrefix(s, "/") {
		return !strings.HasPrefix(s, "//")
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
