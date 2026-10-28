import { AuthRequestError } from "@/api/auth";
import type {
  StudioKnowledgeEntry,
  StudioKnowledgePermission,
  UpdateStudioKnowledgeInput,
} from "@/api/studio-knowledge";

/**
 * 工作室知识库的纯展示/校验逻辑（#196）。
 *
 * 抽成纯函数而不是写在 .vue 里：字段口径、权限标签与错误文案要能被单测锁住，
 * 而组件测试需要挂载 Element Plus，成本远高于测一个函数。
 */

/** 权限 → 中文标签。 */
export const KNOWLEDGE_PERMISSION_LABELS: Record<StudioKnowledgePermission, string> = {
  admin: "仅管理员",
  member: "成员可读",
  public: "公开",
};

/** 权限下拉选项（顺序即展示顺序，admin 在最前因为它是缺省值）。 */
export const KNOWLEDGE_PERMISSION_OPTIONS = [
  { value: "admin", label: "仅管理员" },
  { value: "member", label: "成员可读" },
  { value: "public", label: "公开" },
] as const;

/** 权限标签；未知值原样返回（后端可能新增分级）。 */
export function permissionLabel(permission: string): string {
  return (
    KNOWLEDGE_PERMISSION_LABELS[permission as StudioKnowledgePermission] ??
    permission
  );
}

/** 字段口径，与后端 domain.StudioKnowledgeFields 的校验保持一致。 */
export const KNOWLEDGE_LIMITS = {
  title: 200,
  body: 100_000,
} as const;

/** 条目草稿（全部是字符串，方便 v-model 绑定输入框）。 */
export interface KnowledgeDraft {
  title: string;
  body: string;
  permission: StudioKnowledgePermission;
}

export const EMPTY_KNOWLEDGE_DRAFT: KnowledgeDraft = {
  title: "",
  body: "",
  permission: "admin",
};

const runes = (value: string) => [...value].length;

/**
 * 校验草稿，返回字段 → 错误文案。空对象表示通过。
 *
 * 前端校验只做即时反馈，后端仍是最终判定。
 * 正文允许为空（先建骨架、回头再写），所以只校验长度上界。
 */
export function draftErrors(draft: KnowledgeDraft): Record<string, string> {
  const errors: Record<string, string> = {};
  const title = draft.title.trim();
  if (!title) errors.title = "标题必填";
  else if (runes(title) > KNOWLEDGE_LIMITS.title)
    errors.title = `标题最多 ${KNOWLEDGE_LIMITS.title} 个字符`;

  if (runes(draft.body) > KNOWLEDGE_LIMITS.body)
    errors.body = `正文最多 ${KNOWLEDGE_LIMITS.body} 个字符`;

  if (!KNOWLEDGE_PERMISSION_OPTIONS.some((o) => o.value === draft.permission))
    errors.permission = "请选择有效的权限";

  return errors;
}

/** 草稿 → 更新请求体（补丁：只带与原条目不同的字段）。 */
export function toUpdateInput(
  draft: KnowledgeDraft,
  original: StudioKnowledgeEntry,
): UpdateStudioKnowledgeInput {
  const input: UpdateStudioKnowledgeInput = {};
  const title = draft.title.trim();
  if (title !== original.title) input.title = title;
  if (draft.body !== original.body) input.body = draft.body;
  if (draft.permission !== original.permission)
    input.permission = draft.permission;
  // 乐观锁：无论改了什么，都带上当前读到的版本号。
  // 不带的话后端会退化成「总是覆盖」，并发编辑会静默丢改动。
  input.version = original.version;
  return input;
}

/** 条目 → 草稿（进入编辑态时填充）。 */
export function toDraft(entry: StudioKnowledgeEntry): KnowledgeDraft {
  return {
    title: entry.title,
    body: entry.body,
    permission: entry.permission,
  };
}

/** 把任意异常翻译成可读文案（业务错误直接展示后端 message）。 */
export function describeKnowledgeError(error: unknown): string {
  return error instanceof AuthRequestError
    ? error.message
    : "暂时无法完成操作，请稍后重试";
}

/** 删除二次确认文案（含标题，避免点错行）。 */
export function deleteConfirmText(entry: StudioKnowledgeEntry): string {
  return `删除「${entry.title}」后，它的 ${entry.version} 个历史版本也会一并删除，且无法恢复。`;
}

/**
 * 版本摘要：列表里展示「v3 · 2026-10-28」这种短标注。
 *
 * 只到日：知识库列表里精确到秒没有意义，反而挤掉标题的宽度。
 */
export function versionSummary(entry: StudioKnowledgeEntry): string {
  return `v${entry.version} · ${formatDay(entry.updated_at)}`;
}

/** ISO 时间 → YYYY-MM-DD；解析失败时退回原字符串前 10 位。 */
export function formatDay(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso.slice(0, 10);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

/**
 * 正文摘要：列表里截一段预览。
 *
 * 先压掉换行再截断，否则 Markdown 的多行正文在单行预览里会变成一串空白。
 */
export function bodyExcerpt(body: string, max = 120): string {
  const flat = body.replace(/\s+/g, " ").trim();
  if (flat.length <= max) return flat;
  return `${flat.slice(0, max)}…`;
}

/**
 * 撤销本地未保存改动时用：把草稿还原成条目的当前内容。
 *
 * 单独抽出来是为了让「重置」按钮的行为可测——
 * 直接写 draft.value = toDraft(entry) 在组件里没法验证。
 */
export function resetDraft(entry: StudioKnowledgeEntry): KnowledgeDraft {
  return toDraft(entry);
}
