import type { FormShape, UserDetail } from "@/api/profile";

/**
 * 报名入口拦截后跳转个人设置页的来源标记（issue #152）：`?from=apply`。
 *
 * 常量放在落地页这一侧（设置页负责解释这个标记），
 * 不放进 home 模块，避免「落地页反向依赖入口页」。
 */
export const PROFILE_SOURCE_PARAM = "from";
export const PROFILE_SOURCE_APPLY = "apply";

/**
 * 资料格式口径（0012）：必须与后端 `domain.ValidateClass` / `domain.ValidateStudentID`
 * 完全一致。
 *
 * 为什么前端也要按格式判定，而不只是「非空」：定时任务会在注册满 7 天后把
 * 不合规账号冻结，若前端只查空值，用户会以为资料已完整、直到收到冻结邮件
 * 才发现班级/学号写错。这里提前把问题暴露在设置页与报名入口上。
 */
const CLASS_PATTERN = /^\p{Script=Han}+\d{6}$/u;
const STUDENT_ID_PATTERN = /^[1-9]\d{8}$/;

export const CLASS_HINT = "班级须为「汉字 + 6 位数字」，例如 软件工程230301。";
export const STUDENT_ID_HINT = "学号须为 9 位数字，例如 202311451。";

export interface RequiredProfileErrors {
  class?: string;
  studentId?: string;
}

/**
 * 校验班级与学号：非空 + 格式。
 *
 * 返回空对象表示合规。文案与后端保持一致（后端返回的就是这两句），
 * 避免「前端说一套、服务端 400 又说另一套」。
 */
export function validateRequiredProfile(form: Pick<FormShape, "class" | "studentId">): RequiredProfileErrors {
  const errors: RequiredProfileErrors = {};

  const className = form.class.trim();
  if (!className) {
    errors.class = "请填写班级";
  } else if (!CLASS_PATTERN.test(className)) {
    errors.class = "班级须为「汉字 + 6 位数字」，例如 软件工程230301";
  }

  const raw = String(form.studentId).trim();
  if (!raw) {
    errors.studentId = "请填写学号";
  } else if (!STUDENT_ID_PATTERN.test(raw)) {
    // 后端以 0 表示未填；同时 9 位数字的首位不能是 0（那会解析成 8 位数）。
    // 前端统一给格式提示，不把「0」特殊化成另一句话——用户看到的动作都是「补 9 位学号」。
    errors.studentId = "学号须为 9 位数字，例如 202311451";
  }

  return errors;
}

/**
 * 返回待完善的字段名（用于提示条与报名入口拦截）。
 *
 * 缺失与格式不符都算「待完善」：这两种情况都会被定时任务判为不合规，
 * 分开处理只会让用户在冻结邮件里才第一次知道格式错了。
 */
export function missingRequiredProfile(detail?: UserDetail | null): string[] {
  // detail 可能是 null（如创始管理员没有资料行）：默认值只兜 undefined，
  // null 会把 .class 读崩——这里显式兜底
  const safeDetail = detail ?? {};
  const errors = validateRequiredProfile({ class: safeDetail.class ?? "", studentId: safeDetail.student_id ?? "" });
  return [errors.class ? "班级" : "", errors.studentId ? "学号" : ""].filter(Boolean);
}
