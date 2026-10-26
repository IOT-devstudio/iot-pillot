package domain

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 资料合规规则（0012 定时冻结的唯一口径）。
//
// 为什么集中在这一层：同一套规则有两个消费者——
//   1. 定时扫描任务判定「已注册满 7 天但资料不合规」；
//   2. PUT /me 写入时拒绝明显不合规的输入。
// 两处各写一份正则迟早会漂移（扫描说合规、写入说拒绝），因此规则只在
// 这里定义一次，其余地方只调用函数。
//
// 维护者 2026-10-26 裁决的口径：
//   - 姓名：2–20 个汉字，且**全部**是汉字；
//   - 班级：汉字开头，紧跟**恰好 6 位数字**（如 软件工程230301）；
//   - 学号：**恰好 9 位数字**。
//
// ⚠️ 该口径比历史数据严格：存量里存在「软工 2 班」「物联网工程 2301」这类
// 班级、以及 8 位学号。上线后这些账号会被判为不合规并按裁决冻结，
// 这是刻意选择的结果（维护者选择「照常执行」而不是给存量开豁免）。

const (
	// NameMinRunes / NameMaxRunes 姓名长度边界，沿用 #154 的个人资料口径。
	NameMinRunes = 2
	NameMaxRunes = 20

	// ClassDigits 班级末尾的学制/班号位数。
	ClassDigits = 6

	// StudentIDDigits 学号位数。
	StudentIDDigits = 9
)

// classPattern 班级：至少一个汉字，紧跟恰好 6 位数字。
//
// 用 \p{Han} 而不是 [\x{4e00}-\x{9fff}]：前者覆盖扩展区汉字，
// 与姓名校验里的 unicode.Han 判断同源，避免「姓名认得出、班级认不出」。
// 中间不允许空格、字母、标点：口径是「汉字 + 6 位数字」，不接受夹带。
var classPattern = regexp.MustCompile(`^\p{Han}+[0-9]{6}$`)

// studentIDPattern 学号：恰好 9 位数字，且首位不能是 0。
//
// 首位非 0 的依据：学号是「入学年份 + 序号」，以 0 开头的 9 位数不是真实学号；
// 而 Detail.StudentID 是 int 类型，前端传 "023123456" 会被解析成 23123456
// （8 位），落库后两种写法无法区分。拒绝首位 0 可以让不合规输入在写入阶段
// 就被明确指出，而不是落库后变成一个位数不对的数字。
var studentIDPattern = regexp.MustCompile(`^[1-9][0-9]{8}$`)

// ProfileIssue 一条资料不合规的具体原因。
//
// 同时带字段名与用户可读的说明：字段名供程序分支（例如将来按字段分组统计），
// 说明直接进冻结原因与通知邮件正文。
type ProfileIssue struct {
	Field   string // name / class / student_id
	Message string // 中文说明，可直接展示给用户
}

// 字段名常量，避免各处写裸字符串。
const (
	ProfileFieldName      = "name"
	ProfileFieldClass     = "class"
	ProfileFieldStudentID = "student_id"
)

// ValidateName 校验姓名：2–20 个汉字且全为汉字。
//
// 刻意**不** TrimSpace 后再判定：本函数同时是冻结判定口径，若「 张三」被判
// 合规，库里就会留下一批首尾带空格的姓名，而写入接口又拒绝这种输入——
// 两边口径不一致比多一点严格更糟。
//
// 返回空串表示合规，否则返回中文错误说明。
func ValidateName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "姓名未填写"
	}
	count := utf8.RuneCountInString(name)
	if count < NameMinRunes || count > NameMaxRunes {
		return "姓名须为 2–20 个中文汉字"
	}
	for _, r := range name {
		if !unicode.Is(unicode.Han, r) {
			return "姓名须全部为中文汉字"
		}
	}
	return ""
}

// ValidateClass 校验班级：汉字开头 + 恰好 6 位数字。
//
// 同样不做 TrimSpace：正则本身已经禁止空格，首尾空白一律判不合规。
func ValidateClass(class string) string {
	if strings.TrimSpace(class) == "" {
		return "班级未填写"
	}
	if !classPattern.MatchString(class) {
		return "班级须为「汉字 + 6 位数字」，例如 软件工程230301"
	}
	return ""
}

// ValidateStudentID 校验学号：恰好 9 位数字且首位非 0。
//
// 0 是既有契约里的「未填」哨兵（见 UserProfileDetail 注释），单独给出
// 「未填写」而不是「格式不对」——前者用户去补，后者用户去改。
func ValidateStudentID(studentID int) string {
	if studentID == 0 {
		return "学号未填写"
	}
	if !studentIDPattern.MatchString(strconv.Itoa(studentID)) {
		return "学号须为 9 位数字"
	}
	return ""
}

// ProfileIssues 返回该用户资料的全部不合规项。
//
// 一次返回全部而不是首条：冻结原因与通知邮件要能让用户一次看全要改什么，
// 而不是改一项、等一天、再被冻结一次。
//
// 返回 nil 表示资料合规。
func ProfileIssues(user *User) []ProfileIssue {
	if user == nil {
		return nil
	}

	var issues []ProfileIssue
	if msg := ValidateName(user.Name); msg != "" {
		issues = append(issues, ProfileIssue{Field: ProfileFieldName, Message: msg})
	}
	if msg := ValidateClass(user.Detail.Class); msg != "" {
		issues = append(issues, ProfileIssue{Field: ProfileFieldClass, Message: msg})
	}
	if msg := ValidateStudentID(user.Detail.StudentID); msg != "" {
		issues = append(issues, ProfileIssue{Field: ProfileFieldStudentID, Message: msg})
	}
	return issues
}

// ProfileIssuesSummary 把不合规项拼成一行摘要，用于 freeze_reason 与日志。
//
// 例如：「班级须为「汉字 + 6 位数字」；学号未填写」。
// 顺序固定为姓名 → 班级 → 学号，与 ProfileIssues 的追加顺序一致。
func ProfileIssuesSummary(issues []ProfileIssue) string {
	if len(issues) == 0 {
		return ""
	}
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Message)
	}
	return strings.Join(parts, "；")
}
