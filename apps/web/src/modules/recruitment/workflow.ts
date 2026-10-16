import type {
  CandidateStatus,
  AdvanceCandidateInput,
  MyInvitationView,
} from "@iot-pillot/shared-types";
export const candidateLabels: Record<CandidateStatus, string> = {
  interviewing: "面试中",
  offered: "已发 Offer",
  hired: "已录用",
  rejected: "未通过",
};
export const actionLabels = {
  send_offer: "通过面试并发送 Offer",
  hire: "确认录用",
  reject: "不通过并发送感谢信",
};
export function availableActions(
  status: CandidateStatus,
): AdvanceCandidateInput["action"][] {
  return status === "interviewing"
    ? ["send_offer", "reject"]
    : status === "offered"
      ? ["hire", "reject"]
      : [];
}
export function invitationLabel(
  invitation: Pick<MyInvitationView, "status" | "expired">,
): string {
  if (invitation.status === "accepted") return "已接受";
  if (invitation.status === "revoked") return "已撤销";
  return invitation.expired ? "已过期" : "待接受";
}
export function dateTime(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "—"
    : date.toLocaleString("zh-CN", { hour12: false });
}
export const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : "操作失败，请稍后重试";
