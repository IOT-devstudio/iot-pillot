/**
 * 作品提交 API（学员交源码 zip 或仓库链接）。
 *
 * 提交走 multipart/form-data：仓库链接是文本字段，源码包是文件字段。
 * 不能复用 requestWithSession 的 JSON 默认行为——它会固定 Content-Type，
 * 而 multipart 的 boundary 必须由浏览器自己生成，手写会把请求体切坏。
 */
import { requestBackend, UNAVAILABLE_MESSAGE, AuthRequestError } from "./auth";
import { readAuthSession } from "@/auth/session";

/** 作品状态。 */
export type ShowcaseStatus = "submitted" | "approved" | "rejected";

/** 一份作品提交。 */
export interface ShowcaseSubmission {
  id: number;
  title: string;
  summary: string;
  roadmap_id: number | null;
  roadmap_title: string;
  repo_url: string;
  has_archive: boolean;
  archive_name: string;
  archive_size: number;
  status: ShowcaseStatus;
  review_note: string;
  reviewed_at: string;
  created_at: string;
  /** 管理端列表才有：是否是自己提交的。 */
  is_mine?: boolean;
}

export interface ShowcaseList {
  items: ShowcaseSubmission[];
  total: number;
  page: number;
  page_size: number;
}

/** 提交入参。 */
export interface ShowcaseInput {
  title: string;
  summary?: string;
  repoURL?: string;
  roadmapId?: number | null;
  /** 源码包；不传就是只交仓库链接。 */
  archive?: File | null;
}

const BASE = "/api/v1/me/showcase";
const ADMIN_BASE = "/api/v1/admin/studio/showcase";

/**
 * 读本地令牌。
 *
 * 这里不复用 session-request 的自动刷新：它按固定 JSON 头重放请求，
 * 而 multipart 请求体的 boundary 与原始头绑定，重放会损坏文件。
 * 代价是令牌恰好在提交瞬间过期时要重新登录——概率低，且用户会看到明确提示。
 */
function bearer(): string {
  const session = readAuthSession();
  if (!session) {
    throw new AuthRequestError("登录状态已失效，请重新登录", 401);
  }
  return `Bearer ${session.access_token}`;
}

/** 把 multipart 响应解包成统一形状。 */
async function sendForm<T>(path: string, init: RequestInit, timeoutMs = 120000): Promise<T> {
  const { body } = await requestBackend<T>(path, init, timeoutMs);
  if (body?.code !== 0 || body.data === undefined) {
    throw new AuthRequestError(body?.message || UNAVAILABLE_MESSAGE, 200, body?.code);
  }
  return body.data as T;
}

/**
 * 提交作品。
 *
 * 超时给到 120 秒：源码包可能有几十 MB，用全站默认的 15 秒会在慢网下
 * 稳定超时，而现实中上传本来就需要更久。
 */
export async function submitShowcase(input: ShowcaseInput): Promise<ShowcaseSubmission> {
  const form = new FormData();
  form.append("title", input.title);
  form.append("summary", input.summary ?? "");
  form.append("repo_url", input.repoURL ?? "");
  if (input.roadmapId != null) {
    form.append("roadmap_id", String(input.roadmapId));
  }
  if (input.archive) {
    form.append("archive", input.archive);
  }

  // 刻意不设 Content-Type：浏览器要自己拼 multipart boundary。
  return sendForm<ShowcaseSubmission>(BASE, {
    method: "POST",
    headers: { Authorization: bearer() },
    body: form,
  });
}

/** 我的作品列表。 */
export async function listMyShowcase(page = 1, pageSize = 20): Promise<ShowcaseList> {
  const { body } = await requestBackend<ShowcaseList>(
    `${BASE}?page=${page}&page_size=${pageSize}`,
    { headers: { Authorization: bearer() } },
  );
  if (body?.code !== 0 || body.data === undefined) {
    throw new AuthRequestError(body?.message || UNAVAILABLE_MESSAGE, 200, body?.code);
  }
  return body.data;
}

/** 删除本人的作品。 */
export async function deleteShowcase(id: number): Promise<void> {
  await requestBackend(`${BASE}/${id}`, {
    method: "DELETE",
    headers: { Authorization: bearer() },
  });
}

/** 源码包下载地址（下载走带鉴权的端点，不能直接用对象存储地址）。 */
export function showcaseArchivePath(id: number): string {
  return `${BASE}/${id}/archive`;
}

/** 管理员：全量作品列表。 */
export async function listAllShowcase(status: string | null = null, page = 1, pageSize = 20): Promise<ShowcaseList> {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
  if (status) params.set("status", status);
  const { body } = await requestBackend<ShowcaseList>(
    `${ADMIN_BASE}?${params.toString()}`,
    { headers: { Authorization: bearer() } },
  );
  if (body?.code !== 0 || body.data === undefined) {
    throw new AuthRequestError(body?.message || UNAVAILABLE_MESSAGE, 200, body?.code);
  }
  return body.data;
}

/** 管理员：审阅。 */
export async function reviewShowcase(id: number, status: ShowcaseStatus, note = ""): Promise<void> {
  const { body } = await requestBackend(
    `${ADMIN_BASE}/${id}/review`,
    {
      method: "PATCH",
      headers: { Authorization: bearer(), "Content-Type": "application/json" },
      body: JSON.stringify({ status, note }),
    },
  );
  if (body?.code !== 0) {
    throw new AuthRequestError(body?.message || UNAVAILABLE_MESSAGE, 200, body?.code);
  }
}
