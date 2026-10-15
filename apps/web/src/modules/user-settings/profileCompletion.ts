import type { FormShape, UserDetail } from "@/api/profile";

export interface RequiredProfileErrors {
  class?: string;
  studentId?: string;
}

export function validateRequiredProfile(form: Pick<FormShape, "class" | "studentId">): RequiredProfileErrors {
  const errors: RequiredProfileErrors = {};
  const className = form.class.trim();
  if (!className) errors.class = "请填写班级";
  else if (Array.from(className).length > 64) errors.class = "班级不能超过 64 个字符";
  const raw = String(form.studentId).trim();
  if (!raw) errors.studentId = "请填写学号";
  else if (!/^\d+$/.test(raw) || !Number.isSafeInteger(Number(raw)) || Number(raw) <= 0) {
    // 后端以 0 表示未填；不能把 0 保存成功后又提示缺失。
    errors.studentId = "请输入大于 0 的完整学号（整数）";
  }
  return errors;
}

export function missingRequiredProfile(detail?: UserDetail | null): string[] {
  // detail 可能是 null（如创始管理员没有资料行）：默认值只兜 undefined，
  // null 会把 .class 读崩——这里显式兜底
  const safeDetail = detail ?? {};
  const errors = validateRequiredProfile({ class: safeDetail.class ?? "", studentId: safeDetail.student_id ?? "" });
  return [errors.class ? "班级" : "", errors.studentId ? "学号" : ""].filter(Boolean);
}
