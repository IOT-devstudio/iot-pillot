import { AuthRequestError } from "@/api/auth";
import type {
  CreateStudioMemberInput,
  StudioMember,
  UpdateStudioMemberInput,
} from "@/api/studio-members";

/**
 * 工作室成员名录的纯展示/校验逻辑（#195）。
 *
 * 抽成纯函数而不是写在 .vue 里：字段口径与错误文案要能被单测锁住，
 * 而组件测试需要挂载 Element Plus，成本远高于测一个函数。
 */

/** 档案表单的草稿形态（全部是字符串，方便 v-model 绑定输入框）。 */
export interface StudioMemberDraft {
  name: string;
  birth_year: string;
  cohort: string;
  direction: string;
  role_in_studio: string;
  internship: string;
  employment: string;
  user_id: string;
}

export const EMPTY_STUDIO_MEMBER_DRAFT: StudioMemberDraft = {
  name: "",
  birth_year: "",
  cohort: "",
  direction: "",
  role_in_studio: "",
  internship: "",
  employment: "",
  user_id: "",
};

/** 字段口径，与后端 domain.StudioMemberFields 的校验保持一致。 */
export const STUDIO_MEMBER_LIMITS = {
  name: 40,
  cohort: 40,
  direction: 40,
  role: 60,
  optional: 200,
  birthYearMin: 1900,
  birthYearMax: 2100,
} as const;

/** 方向候选项：与招新方向 code 对齐，额外留一个「其他」兜底。 */
export const STUDIO_DIRECTION_OPTIONS = [
  { value: "front-end", label: "前端开发" },
  { value: "back-end", label: "后端开发" },
  { value: "agent", label: "智能体" },
  { value: "all", label: "全栈" },
  { value: "game", label: "游戏" },
  { value: "other", label: "其他" },
] as const;

/** 方向 code → 中文标签；未知 code 原样返回（后端可能新增方向）。 */
export function studioDirectionLabel(code: string): string {
  return (
    STUDIO_DIRECTION_OPTIONS.find((option) => option.value === code)?.label ??
    code
  );
}

const runes = (value: string) => [...value].length;

/**
 * 校验草稿，返回字段 → 错误文案。空对象表示通过。
 *
 * 前端校验只做即时反馈，后端仍是最终判定：同一套口径写两遍是有意的
 * （前端为了体验，后端为了正确），差异只会表现为「后端拒绝而前端没提示」，
 * 不会反过来放行。
 */
export function draftErrors(draft: StudioMemberDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  const name = draft.name.trim();
  if (!name) errors.name = "姓名必填";
  else if (runes(name) > STUDIO_MEMBER_LIMITS.name)
    errors.name = `姓名最多 ${STUDIO_MEMBER_LIMITS.name} 个字符`;

  const birthYear = draft.birth_year.trim();
  if (!birthYear) errors.birth_year = "出生年必填";
  else if (!/^\d{4}$/.test(birthYear)) errors.birth_year = "出生年须为 4 位数字";
  else {
    const value = Number(birthYear);
    if (value < STUDIO_MEMBER_LIMITS.birthYearMin || value > STUDIO_MEMBER_LIMITS.birthYearMax)
      errors.birth_year = `出生年须在 ${STUDIO_MEMBER_LIMITS.birthYearMin}–${STUDIO_MEMBER_LIMITS.birthYearMax} 之间`;
  }

  if (!draft.cohort.trim()) errors.cohort = "届别必填（如「2024 级」）";
  else if (runes(draft.cohort.trim()) > STUDIO_MEMBER_LIMITS.cohort)
    errors.cohort = `届别最多 ${STUDIO_MEMBER_LIMITS.cohort} 个字符`;

  if (!draft.direction.trim()) errors.direction = "方向必填";
  else if (runes(draft.direction.trim()) > STUDIO_MEMBER_LIMITS.direction)
    errors.direction = `方向最多 ${STUDIO_MEMBER_LIMITS.direction} 个字符`;

  if (!draft.role_in_studio.trim()) errors.role_in_studio = "工作室职能必填";
  else if (runes(draft.role_in_studio.trim()) > STUDIO_MEMBER_LIMITS.role)
    errors.role_in_studio = `工作室职能最多 ${STUDIO_MEMBER_LIMITS.role} 个字符`;

  if (runes(draft.internship.trim()) > STUDIO_MEMBER_LIMITS.optional)
    errors.internship = `实习去向最多 ${STUDIO_MEMBER_LIMITS.optional} 个字符`;
  if (runes(draft.employment.trim()) > STUDIO_MEMBER_LIMITS.optional)
    errors.employment = `就业去向最多 ${STUDIO_MEMBER_LIMITS.optional} 个字符`;

  // user_id 可空；填了就必须是正整数（后端还会核对账号存在且未注销）。
  const userID = draft.user_id.trim();
  if (userID && !/^[1-9]\d*$/.test(userID))
    errors.user_id = "系统账号 ID 须为正整数，留空表示无账号成员";

  return errors;
}

/** 草稿 → 创建请求体。 */
export function toCreateInput(draft: StudioMemberDraft): CreateStudioMemberInput {
  const userID = draft.user_id.trim();
  return {
    name: draft.name.trim(),
    birth_year: Number(draft.birth_year.trim()),
    cohort: draft.cohort.trim(),
    direction: draft.direction.trim(),
    role_in_studio: draft.role_in_studio.trim(),
    internship: draft.internship.trim(),
    employment: draft.employment.trim(),
    ...(userID ? { user_id: Number(userID) } : {}),
  };
}

/** 草稿 → 更新请求体（补丁：只带与原档案不同的字段）。 */
export function toUpdateInput(
  draft: StudioMemberDraft,
  original: StudioMember,
): UpdateStudioMemberInput {
  const input: UpdateStudioMemberInput = {};
  const name = draft.name.trim();
  if (name !== original.name) input.name = name;
  const birthYear = Number(draft.birth_year.trim());
  if (birthYear !== original.birth_year) input.birth_year = birthYear;
  const cohort = draft.cohort.trim();
  if (cohort !== original.cohort) input.cohort = cohort;
  const direction = draft.direction.trim();
  if (direction !== original.direction) input.direction = direction;
  const role = draft.role_in_studio.trim();
  if (role !== original.role_in_studio) input.role_in_studio = role;
  const internship = draft.internship.trim();
  if (internship !== original.internship) input.internship = internship;
  const employment = draft.employment.trim();
  if (employment !== original.employment) input.employment = employment;

  // user_id 三态：留空 = 解除绑定（显式 null），有值 = 绑定该账号。
  // 只有「值真的变了」才下发，否则一次纯改姓名的保存会把绑定抹掉。
  const userID = draft.user_id.trim();
  const nextUserID = userID ? Number(userID) : null;
  if (nextUserID !== original.user_id) input.user_id = nextUserID;

  return input;
}

/** 档案 → 草稿（进入编辑态时填充）。 */
export function toDraft(member: StudioMember): StudioMemberDraft {
  return {
    name: member.name,
    birth_year: String(member.birth_year),
    cohort: member.cohort,
    direction: member.direction,
    role_in_studio: member.role_in_studio,
    internship: member.internship ?? "",
    employment: member.employment ?? "",
    user_id: member.user_id === null ? "" : String(member.user_id),
  };
}

/** 把任意异常翻译成可读文案（业务错误直接展示后端 message）。 */
export function describeStudioMemberError(error: unknown): string {
  return error instanceof AuthRequestError
    ? error.message
    : "暂时无法完成操作，请稍后重试";
}

/** 归档二次确认的文案（含姓名，避免点错行）。 */
export function archiveConfirmText(member: StudioMember): string {
  return `归档「${member.name}」后，ta 将不再出现在默认的在读名册里。档案会保留，可随时恢复为在读。`;
}
