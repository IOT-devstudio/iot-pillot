package utils

import (
	"strings"
	"testing"
)

func TestRenderTemplate_ReplacesPlaceholders(t *testing.T) {
	rendered, err := RenderTemplate(
		"你好 {{name}}，面试定在 {{time}}。",
		TemplateVars{"name": "张三", "time": "周三 14:00"},
	)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	want := "你好 张三，面试定在 周三 14:00。"
	if rendered != want {
		t.Errorf("渲染结果 = %q，期望 %q", rendered, want)
	}
}

func TestRenderTemplate_ToleratesSpacesInsideBraces(t *testing.T) {
	rendered, err := RenderTemplate("你好 {{ name }}", TemplateVars{"name": "张三"})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if rendered != "你好 张三" {
		t.Errorf("渲染结果 = %q，期望 %q", rendered, "你好 张三")
	}
}

// 缺变量必须报错：静默留空会让收件人收到「你好，{{name}}」，而邮件发出就收不回来。
func TestRenderTemplate_ErrorsOnMissingVariable(t *testing.T) {
	_, err := RenderTemplate(
		"你好 {{name}}，{{missing_a}} 与 {{missing_b}}",
		TemplateVars{"name": "张三"},
	)
	if err == nil {
		t.Fatal("缺变量时应当报错")
	}

	message := err.Error()
	// 缺的名字要按顺序列出，便于一次性补齐
	if !strings.Contains(message, "missing_a") || !strings.Contains(message, "missing_b") {
		t.Errorf("错误信息应当列出缺失的变量名，实际: %v", err)
	}
	if strings.Contains(message, "{{name}}") {
		t.Errorf("已有的变量不该出现在缺失列表里: %v", err)
	}
	if !strings.Contains(message, "missing_a、missing_b") {
		t.Errorf("缺失变量应当排序后拼接，实际: %v", err)
	}
}

// 变量值里的 HTML 必须被转义。
//
// 邮件按 text/html 发送，而变量可能来自用户自己填的报名资料；
// 不转义的话，把姓名填成标签的人就能往管理员发出的邮件里注入内容。
func TestRenderTemplate_EscapesVariableValues(t *testing.T) {
	attack := `<img src=x onerror="alert(1)">`
	rendered, err := RenderTemplate("<p>报名者：{{name}}</p>", TemplateVars{"name": attack})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	if strings.Contains(rendered, "<img") {
		t.Errorf("变量值里的标签必须被转义，实际结果: %q", rendered)
	}
	if !strings.Contains(rendered, "&lt;img") {
		t.Errorf("期望转义为实体，实际结果: %q", rendered)
	}
	// 模板自身的 HTML 不受影响
	if !strings.Contains(rendered, "<p>报名者：") {
		t.Errorf("模板自身的标签不该被转义，实际结果: %q", rendered)
	}
}

func TestRenderTemplate_LeavesUnknownSyntaxAlone(t *testing.T) {
	// 空占位符、没有括号的普通文本都原样保留，不当成变量
	rendered, err := RenderTemplate("价格 {100} 与 {{ }}", TemplateVars{})
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if rendered != "价格 {100} 与 {{ }}" {
		t.Errorf("渲染结果 = %q，期望原样保留", rendered)
	}
}

func TestValidateTemplateSyntax(t *testing.T) {
	cases := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{name: "正常", template: "你好 {{name}}"},
		{name: "多个占位符", template: "{{a}} 和 {{b}}"},
		{name: "无占位符", template: "纯文本"},
		{name: "少一个右括号", template: "你好 {{name}", wantErr: true},
		{name: "少一个左括号", template: "你好 name}}", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateTemplateSyntax(testCase.template)
			if testCase.wantErr && err == nil {
				t.Error("期望报错，实际通过")
			}
			if !testCase.wantErr && err != nil {
				t.Errorf("不该报错: %v", err)
			}
		})
	}
}

func TestListTemplateVariables(t *testing.T) {
	names := ListTemplateVariables("你好 {{name}}，{{class}} 的 {{name}}")

	want := []string{"name", "class"}
	if len(names) != len(want) {
		t.Fatalf("变量列表 = %v，期望 %v（去重且保持出现顺序）", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Errorf("第 %d 个 = %q，期望 %q", i, names[i], name)
		}
	}
}
