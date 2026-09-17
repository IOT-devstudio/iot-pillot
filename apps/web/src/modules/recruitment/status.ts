/**
 * 招新流程的状态与动作。
 *
 * 状态以后端为准，这里只负责「怎么展示」和「哪些动作合法」。真正的邮件外发
 * 以后端 MailModule 为准；本模块在 fixture 阶段用 store 在内存里模拟状态流转。
 */

export type ProspectStatus =
  | "ready_first" // 准备一面
  | "first_passed" // 一面通过（待发「通过」邮件）
  | "ready_second" // 准备二面
  | "hired" // 录用
  | "rejected"; // 拒录

/** 邮件只有「通过」和「拒录」两类 */
export type EmailType = "pass" | "reject";

export type StatusTagType =
  | "success"
  | "info"
  | "warning"
  | "danger"
  | "primary";

export interface StatusMeta {
  label: string;
  tagType: StatusTagType;
}

const STATUS_META: Record<ProspectStatus, StatusMeta> = {
  ready_first: { label: "准备一面", tagType: "info" },
  first_passed: { label: "一面通过", tagType: "warning" },
  ready_second: { label: "准备二面", tagType: "primary" },
  hired: { label: "录用", tagType: "success" },
  rejected: { label: "拒录", tagType: "danger" },
};

const EMAIL_TYPE_META: Record<EmailType, StatusMeta> = {
  pass: { label: "通过", tagType: "success" },
  reject: { label: "拒录", tagType: "danger" },
};

export function prospectStatusMeta(status: ProspectStatus): StatusMeta {
  return STATUS_META[status];
}

export function emailTypeMeta(type: EmailType): StatusMeta {
  return EMAIL_TYPE_META[type];
}

export const PROSPECT_STATUSES = Object.keys(STATUS_META) as ProspectStatus[];

/**
 * 发送某类邮件后的下一个状态。
 * - 通过：一面通过 → 准备二面；准备二面 → 录用
 * - 拒录：任意可发状态 → 拒录
 */
export function nextStatusAfterEmail(
  status: ProspectStatus,
  type: EmailType,
): ProspectStatus {
  if (type === "reject") return "rejected";
  if (status === "first_passed") return "ready_second";
  if (status === "ready_second") return "hired";
  return status;
}

/** 当前状态下能否发某类邮件（防止重复/越权发信） */
export function canSendEmail(
  status: ProspectStatus,
  type: EmailType,
): boolean {
  if (type === "reject") {
    return status === "ready_first" || status === "ready_second";
  }
  // pass
  return status === "first_passed" || status === "ready_second";
}

/** 标记通过一面：准备一面 → 一面通过（不涉及邮件） */
export function canMarkFirstPass(status: ProspectStatus): boolean {
  return status === "ready_first";
}
