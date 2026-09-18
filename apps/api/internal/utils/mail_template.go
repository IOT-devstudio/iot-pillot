package utils

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

// placeholderPattern 匹配 {{变量名}}。变量名限字母/数字/下划线。
var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// TemplateVars 模板变量表：键是占位符名，值是替换内容。
type TemplateVars map[string]string

// RenderTemplate 用 vars 替换模板里的 {{变量名}} 占位符。
//
// 两个刻意的行为：
//
//  1. **缺变量直接报错**，并把缺的名字列出来。模板和变量都由管理端提供，
//     静默留空会让收件人收到「你好，{{name}}」这种半成品——邮件发出去就收不回来了，
//     宁可发不出去也不要发错。
//
//  2. **变量值做 HTML 转义**。邮件正文按 text/html 发送，而变量里可能是用户
//     报名时自己填的姓名/班级；不转义的话，一个把姓名填成
//     `<img src=x onerror=...>` 的报名者就能往管理员发出的邮件里注入标签。
//     只有变量值被转义，模板本身的 HTML 不受影响。
func RenderTemplate(template string, vars TemplateVars) (string, error) {
	missing := map[string]struct{}{}

	rendered := placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		groups := placeholderPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}

		name := groups[1]
		value, ok := vars[name]
		if !ok {
			missing[name] = struct{}{}
			return match
		}

		return html.EscapeString(value)
	})

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		return "", fmt.Errorf("模板缺少变量：%s", strings.Join(names, "、"))
	}

	return rendered, nil
}

// ValidateTemplateSyntax 检查模板的占位符括号是否成对。
//
// 只查 {{{ 与 }}} 的数量：真正危险的错误不是"用了未知变量"（渲染时会报错），
// 而是少写一个括号导致占位符**根本没被识别**，于是原样发给收件人，且不报任何错。
func ValidateTemplateSyntax(template string) error {
	opens := strings.Count(template, "{{")
	closes := strings.Count(template, "}}")
	if opens != closes {
		return fmt.Errorf("模板占位符括号不匹配：{{ 出现 %d 次，}} 出现 %d 次", opens, closes)
	}
	return nil
}

// ListTemplateVariables 列出模板里用到的变量名（去重、按出现顺序）。
//
// 管理端编辑模板时用它提示"这个模板需要哪些变量"，避免手打名字时写错。
func ListTemplateVariables(template string) []string {
	seen := map[string]struct{}{}
	names := make([]string, 0)

	for _, groups := range placeholderPattern.FindAllStringSubmatch(template, -1) {
		if len(groups) < 2 {
			continue
		}
		if _, exists := seen[groups[1]]; exists {
			continue
		}
		seen[groups[1]] = struct{}{}
		names = append(names, groups[1])
	}

	return names
}
