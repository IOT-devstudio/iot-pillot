import type { CandidateStatus } from "@iot-pillot/shared-types";
import type { ConvertCandidateToMemberInput } from "@/api/studio-members";

/**
 * 「hired 候选人转成员」的纯逻辑（#197）。
 *
 * 抽成纯函数而不是写在 .vue 里：入口的可见性与表单校验要能被单测锁住，
 * 而组件测试需要挂载 Element Plus，成本远高于测一个函数。
 */

/**
 * 该候选人是否应该显示「转为工作室成员」入口。
 *
 * 只有 hired 才显示：其他状态由后端拒绝，但把注定失败的按钮摆出来
 * 只会让管理员点一次、读一遍错误、再退回来。
 */
export function canConvertToMember(status: CandidateStatus): boolean {
  return status === "hired";
}

/** 转入表单草稿（全部是字符串，方便 v-model 绑定输入框）。 */
export interface ConvertDraft {
  birth_year: string;
  cohort: string;
  role_in_studio: string;
  internship: string;
  employment: string;
}

export const EMPTY_CONVERT_DRAFT: ConvertDraft = {
  birth_year: "",
  cohort: "",
  role_in_studio: "",
  internship: "",
  employment: "",
};

/** 字段口径，与后端 domain.StudioMemberFields 的校验保持一致。 */
const LIMITS = {
  name: 40,
  cohort: 40,
  role: 60,
  optional: 200,
  birthYearMin: 1900,
  birthYearMax: 2100,
} as const;

const runes = (value: string) => [...value].length;

/**
 * 校验转入草稿，返回字段 → 错误文案。空对象表示通过。
 *
 * 姓名与方向不在这里：它们由后端从申请带过来，管理员不需要也不能改。
 */
export function convertDraftErrors(draft: ConvertDraft): Record<string, string> {
  const errors: Record<string, string> = {};

  const birthYear = draft.birth_year.trim();
  if (!birthYear) errors.birth_year = "出生年必填（招新数据里没有这一项）";
  else if (!/^\d{4}$/.test(birthYear)) errors.birth_year = "出生年须为 4 位数字";
  else {
    const value = Number(birthYear);
    if (value < LIMITS.birthYearMin || value > LIMITS.birthYearMax)
      errors.birth_year = `出生年须在 ${LIMITS.birthYearMin}–${LIMITS.birthYearMax} 之间`;
  }

  if (!draft.cohort.trim()) errors.cohort = "届别必填（如「2024 级」）";
  else if (runes(draft.cohort.trim()) > LIMITS.cohort)
    errors.cohort = `届别最多 ${LIMITS.cohort} 个字符`;

  if (!draft.role_in_studio.trim()) errors.role_in_studio = "工作室职能必填";
  else if (runes(draft.role_in_studio.trim()) > LIMITS.role)
    errors.role_in_studio = `工作室职能最多 ${LIMITS.role} 个字符`;

  if (runes(draft.internship.trim()) > LIMITS.optional)
    errors.internship = `实习去向最多 ${LIMITS.optional} 个字符`;
  if (runes(draft.employment.trim()) > LIMITS.optional)
    errors.employment = `就业去向最多 ${LIMITS.optional} 个字符`;

  return errors;
}

/** 草稿 → 转入请求体。 */
export function toConvertInput(
  candidateID: number,
  draft: ConvertDraft,
): ConvertCandidateToMemberInput {
  return {
    candidate_id: candidateID,
    birth_year: Number(draft.birth_year.trim()),
    cohort: draft.cohort.trim(),
    role_in_studio: draft.role_in_studio.trim(),
    internship: draft.internship.trim(),
    employment: draft.employment.trim(),
  };
}
