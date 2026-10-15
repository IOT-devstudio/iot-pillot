import { AuthRequestError } from "@/api/auth";
import type { FormAnswers, FormCreateInput, FormFieldInput } from "@/api/forms";

export const FIELD_LABELS = {
  text: "短文本",
  email: "邮箱",
  textarea: "长文本",
  select: "下拉选择",
  checkbox: "单项确认",
} as const;
const length = (value: string) => [...value].length;
export const describeFormError = (error: unknown) =>
  error instanceof AuthRequestError
    ? error.message
    : "暂时无法完成操作，请稍后重试";
export function initialAnswers(fields: FormFieldInput[]): FormAnswers {
  return Object.fromEntries(
    fields.map((field) => [
      String(field.id),
      field.type === "checkbox" ? [] : "",
    ]),
  );
}
export function answerErrors(
  fields: FormFieldInput[],
  values: FormAnswers,
): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const field of fields) {
    const id = String(field.id);
    const value = values[id] ?? (field.type === "checkbox" ? [] : "");
    if (field.type === "checkbox") {
      if (
        !Array.isArray(value) ||
        value.some((v) => typeof v !== "string" || length(v) > 2000) ||
        value.length > 50
      )
        errors[id] = "确认内容无效";
      else if (field.required && !value.length) errors[id] = "请勾选确认后继续";
      continue;
    }
    if (typeof value !== "string") {
      errors[id] = "请填写有效内容";
      continue;
    }
    if (field.required && !value.trim()) errors[id] = "请填写此必填项";
    else if (length(value) > 2000) errors[id] = "最多填写 2000 个字符";
    else if (
      value.trim() &&
      field.type === "email" &&
      !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
    )
      errors[id] = "请输入有效的邮箱地址";
    else if (
      value.trim() &&
      field.type === "select" &&
      !field.options?.includes(value.trim())
    )
      errors[id] = "请选择有效选项";
  }
  return errors;
}
export function editorError(input: FormCreateInput): string {
  if (!input.title.trim() || length(input.title) > 200)
    return "表单标题必填，最多 200 个字符";
  if (length(input.description) > 2000) return "表单说明最多 2000 个字符";
  if (!input.round_id || !input.direction_code) return "请选择招新轮次和方向";
  if (!input.fields.length || input.fields.length > 50)
    return "请配置 1–50 个字段";
  for (const [i, field] of input.fields.entries()) {
    if (
      !field.label.trim() ||
      length(field.label) > 200 ||
      length(field.placeholder) > 200
    )
      return `第 ${i + 1} 题标题必填，标题与提示最多 200 字`;
    if (field.type === "select") {
      const options = (field.options ?? []).map((v) => v.trim());
      if (
        !options.length ||
        options.length > 100 ||
        options.some((v) => !v || length(v) > 200) ||
        new Set(options).size !== options.length
      )
        return `第 ${i + 1} 题选项须为 1–100 个非空、不重复的内容（每项最多 200 字）`;
    }
  }
  return "";
}
export function answerRows(fields: FormFieldInput[], values: FormAnswers) {
  const rows = fields.map((field) => ({
    id: String(field.id),
    label: field.label,
    value: renderAnswer(values[String(field.id)]),
  }));
  const ids = new Set(rows.map((row) => row.id));
  for (const [id, value] of Object.entries(values))
    if (!ids.has(id))
      rows.push({ id, label: `已删除字段 #${id}`, value: renderAnswer(value) });
  return rows;
}
function renderAnswer(value: string | string[] | undefined): string {
  return Array.isArray(value)
    ? value.join("、") || "未勾选"
    : value || "未填写";
}
export function sharePath(token: string): string {
  return `/f/${encodeURIComponent(token)}`;
}
