/**
 * 工作室项目与参与的纯展示/校验逻辑（#198）。
 *
 * 抽成纯函数而不是写在 .vue 里：字段口径、状态标签与三态映射要能被单测锁住，
 * 而组件测试需要挂载 Element Plus，成本远高于测一个函数。
 */
import type {
  AddStudioProjectMemberInput,
  CreateStudioProjectInput,
  StudioProject,
  StudioProjectMember,
  StudioProjectMemberRole,
  StudioProjectStatus,
  UpdateStudioProjectInput,
  UpdateStudioProjectMemberInput,
} from "@iot-pillot/shared-types";

/** 状态 → 中文标签。 */
export const PROJECT_STATUS_LABELS: Record<StudioProjectStatus, string> = {
  active: "进行中",
  completed: "已完成",
  archived: "已归档",
};

/** 状态下拉选项（顺序即展示顺序，active 在最前因为它是缺省值）。 */
export const PROJECT_STATUS_OPTIONS = [
  { value: "active", label: "进行中" },
  { value: "completed", label: "已完成" },
  { value: "archived", label: "已归档" },
] as const;

/** 角色 → 中文标签。 */
export const PROJECT_ROLE_LABELS: Record<StudioProjectMemberRole, string> = {
  lead: "负责人",
  contributor: "参与者",
};

export const PROJECT_ROLE_OPTIONS = [
  { value: "lead", label: "负责人" },
  { value: "contributor", label: "参与者" },
] as const;

/** 状态标签；未知值原样返回（后端可能新增状态）。 */
export function projectStatusLabel(status: string): string {
  return PROJECT_STATUS_LABELS[status as StudioProjectStatus] ?? status;
}

/** 角色标签；未知值原样返回。 */
export function projectRoleLabel(role: string): string {
  return PROJECT_ROLE_LABELS[role as StudioProjectMemberRole] ?? role;
}

/* ------------------------------------------------------------------ *
 * 字段口径（与后端 domain.StudioProjectFields 的校验保持一致）
 * ------------------------------------------------------------------ */

export const PROJECT_LIMITS = {
  name: 100,
  year: 20,
  summary: 2000,
} as const;

/** 项目草稿（全部是字符串，方便 v-model 绑定输入框）。 */
export interface ProjectDraft {
  name: string;
  year: string;
  summary: string;
  status: StudioProjectStatus;
}

export const EMPTY_PROJECT_DRAFT: ProjectDraft = {
  name: "",
  year: "",
  summary: "",
  status: "active",
};

const runes = (value: string) => [...value].length;

/**
 * 校验项目草稿，返回字段 → 错误文案。空对象表示通过。
 *
 * 项目名与年份必填；状态只接受后端枚举里的三个值。
 */
export function projectDraftErrors(draft: ProjectDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  const name = draft.name.trim();
  if (!name) errors.name = "项目名必填";
  else if (runes(name) > PROJECT_LIMITS.name)
    errors.name = "项目名最多 " + PROJECT_LIMITS.name + " 个字符";

  if (!draft.year.trim()) errors.year = "年份必填（如「2023」）";
  else if (runes(draft.year.trim()) > PROJECT_LIMITS.year)
    errors.year = "年份最多 " + PROJECT_LIMITS.year + " 个字符";

  if (runes(draft.summary.trim()) > PROJECT_LIMITS.summary)
    errors.summary = "项目简介最多 " + PROJECT_LIMITS.summary + " 个字符";

  if (!PROJECT_STATUS_OPTIONS.some((option) => option.value === draft.status))
    errors.status = "请选择有效的状态";

  return errors;
}

/** 项目草稿 → 创建请求体（去掉首尾空白）。 */
export function toCreateInput(draft: ProjectDraft): CreateStudioProjectInput {
  return {
    name: draft.name.trim(),
    year: draft.year.trim(),
    summary: draft.summary.trim(),
    status: draft.status,
  };
}

/** 项目草稿 → 更新请求体（补丁：只带与原项目不同的字段）。 */
export function toUpdateInput(
  draft: ProjectDraft,
  original: StudioProject,
): UpdateStudioProjectInput {
  const input: UpdateStudioProjectInput = {};
  const name = draft.name.trim();
  if (name !== original.name) input.name = name;
  const year = draft.year.trim();
  if (year !== original.year) input.year = year;
  const summary = draft.summary.trim();
  if (summary !== original.summary) input.summary = summary;
  if (draft.status !== original.status) input.status = draft.status;
  return input;
}

/** 项目 → 草稿（进入编辑态时填充）。 */
export function projectToDraft(project: StudioProject): ProjectDraft {
  return {
    name: project.name,
    year: project.year,
    summary: project.summary,
    status: project.status,
  };
}

/* ------------------------------------------------------------------ *
 * 参与行
 * ------------------------------------------------------------------ */

export const MEMBER_LIMITS = {
  name: 40,
} as const;

/**
 * 参与行草稿。
 *
 * member_id 用字符串表示三态：空串 = 纯外部参与者（不关联成员档案）；
 * 数字字符串 = 关联该成员档案。
 * ended_at 同理：空串 = 仍在参与（后端的 null）；非空 = 已结束的时刻。
 */
export interface MemberDraft {
  member_id: string;
  name: string;
  role: StudioProjectMemberRole;
  started_at: string;
  ended_at: string;
}

export const EMPTY_MEMBER_DRAFT: MemberDraft = {
  member_id: "",
  name: "",
  role: "contributor",
  started_at: "",
  ended_at: "",
};

/**
 * 校验参与行草稿，返回字段 → 错误文案。空对象表示通过。
 *
 * name 必填（姓名快照）；member_id 填了就必须是正整数。
 * 起止时间的先后关系由后端兜底（前端只做即时反馈）。
 */
export function memberDraftErrors(draft: MemberDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  const name = draft.name.trim();
  if (!name) errors.name = "参与者姓名必填（作为快照保留）";
  else if (runes(name) > MEMBER_LIMITS.name)
    errors.name = "参与者姓名最多 " + MEMBER_LIMITS.name + " 个字符";

  // member_id 空串 = 外部参与者，合法；填了就必须是正整数。
  const memberID = draft.member_id.trim();
  if (memberID && !/^[1-9]\d*$/.test(memberID))
    errors.member_id = "成员档案 ID 须为正整数，留空表示外部参与者";

  if (!draft.role) errors.role = "请选择参与角色";
  else if (!PROJECT_ROLE_OPTIONS.some((option) => option.value === draft.role))
    errors.role = "请选择有效的参与角色";

  return errors;
}

/**
 * 参与行草稿 → 创建请求体。
 *
 * member_id 空串映射成 null（外部参与者）；非空映射成数字。
 */
export function toMemberCreateInput(
  draft: MemberDraft,
): AddStudioProjectMemberInput {
  const memberID = draft.member_id.trim();
  return {
    member_id: memberID ? Number(memberID) : null,
    name: draft.name.trim(),
    role: draft.role,
    // started_at 空串 = 让后端填当前时间；非空才下发。
    ...(draft.started_at ? { started_at: draft.started_at } : {}),
    // ended_at 空串 = 仍在参与，映射成 null。
    ...(draft.ended_at ? { ended_at: draft.ended_at } : { ended_at: null }),
  };
}

/**
 * 参与行草稿 → 更新请求体（补丁）。
 *
 * 两个三态字段都只在「值真的变了」时才下发：
 *   - 没改就不发（避免一次纯改角色的保存顺手改掉关联或结束时间）；
 *   - 改了就按新的三态值下发（null / ISO 时间 / 数字）。
 */
export function toMemberUpdateInput(
  draft: MemberDraft,
  original: StudioProjectMember,
): UpdateStudioProjectMemberInput {
  const input: UpdateStudioProjectMemberInput = {};

  const name = draft.name.trim();
  if (name !== original.name) input.name = name;

  if (draft.role !== original.role) input.role = draft.role;

  // member_id 三态：空串 = 解除关联（null），数字 = 关联该成员。
  const memberID = draft.member_id.trim();
  const nextMemberID = memberID ? Number(memberID) : null;
  if (nextMemberID !== original.member_id) input.member_id = nextMemberID;

  // ended_at 三态：空串 = 仍在参与（null），非空 = 已结束。
  const nextEndedAt = draft.ended_at ? draft.ended_at : null;
  if (nextEndedAt !== original.ended_at) input.ended_at = nextEndedAt;

  return input;
}

/** 参与行 → 草稿（进入编辑态时填充）。 */
export function memberToDraft(member: StudioProjectMember): MemberDraft {
  return {
    member_id: member.member_id === null ? "" : String(member.member_id),
    name: member.name,
    role: member.role,
    started_at: member.started_at,
    ended_at: member.ended_at ?? "",
  };
}

/** 是否为外部参与者（没有关联成员档案）。 */
export function isExternalParticipant(member: StudioProjectMember): boolean {
  return member.member_id === null;
}

/** 参与行的时间摘要：仍在参与 / 已结束于某日。 */
export function memberEndSummary(member: StudioProjectMember): string {
  if (member.ended_at === null) return "仍在参与";
  return "已结束 · " + formatDay(member.ended_at);
}

/**
 * ISO 时间 → YYYY-MM-DD；解析失败时退回原字符串前 10 位。
 *
 * 按字符串**自带的时区偏移**取日期，而不是按运行环境的本地时区。
 * 参与行的 started_at 存的是 "2024-03-01T00:00:00+08:00"，按本地时区格式化
 * 会在 UTC 的机器上得到 "2024-02-29"——同一个日期在不同部署时区显示成不同日期。
 */
export function formatDay(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso.slice(0, 10);
  // 墙上时间 = UTC 瞬间 + 该串自己的偏移量（+08:00 要加，不是减）。
  const shifted = new Date(date.getTime() + offsetMinutes(iso) * 60_000);
  return (
    shifted.getUTCFullYear() +
    "-" +
    String(shifted.getUTCMonth() + 1).padStart(2, "0") +
    "-" +
    String(shifted.getUTCDate()).padStart(2, "0")
  );
}

/**
 * ISO 串末尾时区偏移的分钟数："+08:00" → 480，"-05:30" → -330；
 * 无偏移（"Z" 或裸日期）按 0 处理。
 */
function offsetMinutes(iso: string): number {
  const match = iso.match(/(?:Z|[+-]\d{2}:?\d{2})$/);
  if (!match || match[0] === "Z") return 0;
  const sign = match[0][0] === "-" ? -1 : 1;
  const digits = match[0].slice(1).replace(":", "");
  return sign * (Number(digits.slice(0, 2)) * 60 + Number(digits.slice(2, 4)));
}

/**
 * 项目简介摘要：列表里截一段预览。
 *
 * 先压掉换行再截断，否则多行简介在单行预览里会变成一串空白。
 */
export function projectExcerpt(summary: string, max = 140): string {
  const flat = summary.replace(/\s+/g, " ").trim();
  if (flat.length <= max) return flat;
  return flat.slice(0, max) + "…";
}

/** 删除二次确认文案（含项目名与参与人数）。 */
export function deleteProjectConfirmText(
  project: StudioProject,
  memberCount: number,
): string {
  return (
    "删除「" +
    project.name +
    "」后，它的 " +
    memberCount +
    " 条参与记录也会一并删除，且无法恢复。"
  );
}

/** 移除参与行的二次确认文案。 */
export function removeMemberConfirmText(member: StudioProjectMember): string {
  return "移除「" + member.name + "」的参与记录？成员档案本身不会被删除。";
}

/** 把任意异常翻译成可读文案。 */
export function describeProjectError(error: unknown): string {
  return error instanceof Error ? error.message : "暂时无法完成操作，请稍后重试";
}
