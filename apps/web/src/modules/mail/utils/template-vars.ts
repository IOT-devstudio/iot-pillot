/**
 * 模板占位符工具：与后端 utils/mail_template.go 保持同一套规则。
 *
 * 前端先校验再提交，只为在保存前给出即时反馈；权威校验仍在服务端
 * （ValidateTemplateSyntax / RenderTemplate），两边规则不一致时以服务端为准。
 */

/** 匹配 {{变量名}}，变量名限字母/数字/下划线——同后端 placeholderPattern。 */
const PLACEHOLDER_PATTERN = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

/** 扫描文本里的变量名，去重、按首次出现顺序返回。 */
export function extractVariables(text: string): string[] {
  const seen = new Set<string>();
  const names: string[] = [];
  for (const match of text.matchAll(PLACEHOLDER_PATTERN)) {
    const name = match[1];
    if (name !== undefined && !seen.has(name)) {
      seen.add(name);
      names.push(name);
    }
  }
  return names;
}

/**
 * 合并主题与正文的变量（主题在前）——对应后端 toMailModelResp 的 mergeVariables。
 */
export function mergeVariables(
  title: string,
  body: string,
): string[] {
  const seen = new Set<string>();
  const merged: string[] = [];
  for (const name of [...extractVariables(title), ...extractVariables(body)]) {
    if (!seen.has(name)) {
      seen.add(name);
      merged.push(name);
    }
  }
  return merged;
}

/**
 * 检查占位符括号是否成对——同后端 ValidateTemplateSyntax。
 *
 * 真正危险的错误不是「用了未知变量」（渲染时会报错），而是少写一个括号
 * 导致占位符根本没被识别，原样发给收件人且不报任何错。
 * 返回 null 表示通过，否则返回用户可读的原因。
 */
export function validateBraces(text: string): string | null {
  const opens = text.split("{{").length - 1;
  const closes = text.split("}}").length - 1;
  if (opens !== closes) {
    return `占位符括号不匹配：{{ 出现 ${opens} 次，}} 出现 ${closes} 次`;
  }
  return null;
}

/**
 * 列出模板缺少的变量：键在 vars 中缺失或值为空字符串都算缺。
 *
 * 后端会用收件人资料预填 name/email/class/direction/student_id/parse，
 * 但按邮箱发给未注册收件人时没有资料可预填——所以发送前端只把「确定
 * 不会被预填覆盖」的空缺当错误不现实，这里只做提示级检查（返回缺失清单，
 * 由调用方决定拦截还是放行）。值为空字符串与缺失同等对待：后端
 * putIfNotEmpty 同样不放行空值。
 */
export function missingVariables(
  required: string[],
  vars: Record<string, string>,
): string[] {
  return required.filter((name) => {
    const value = vars[name];
    return value === undefined || value.trim() === "";
  });
}

/**
 * 用 vars 渲染模板做本地预览——对应后端 RenderTemplate 的替换逻辑，
 * 变量值做 HTML 转义（正文按 text/html 发送，预览必须和实发一致）。
 * 缺失的变量保留原占位符，让预览里能一眼看出哪里还没填。
 */
export function renderPreview(
  text: string,
  vars: Record<string, string>,
): string {
  return text.replace(PLACEHOLDER_PATTERN, (match, name: string) => {
    const value = vars[name];
    if (value === undefined || value.trim() === "") {
      return match;
    }
    return escapeHtml(value);
  });
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
