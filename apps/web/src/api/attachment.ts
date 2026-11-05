/**
 * 内容附件 API（#216 上传 / #217 读取，#220 前端接入）。
 *
 * 后端契约（ADR 0004）：
 *   POST /api/v1/attachments        multipart/form-data，一次一张
 *   GET  /api/v1/attachments/:id    只要求登录，归属判定在 service
 *
 * 两个要点：
 *   1. 上传用**更长的超时**（60s），其余接口仍是全站 15s 默认值。
 *   2. 读取端点要 Bearer，而 `<img src>` 带不上 Authorization 头。
 *      因此站内附件不能直接当 src 用，必须走后端 blob 转换（见 loadAttachmentObjectUrl）。
 */
import type { AttachmentUploadResp } from "@iot-pillot/shared-types";
import { requestWithSession, requestWithSessionOptions } from "./session-request";

export type { AttachmentUploadResp };

/**
 * 上传超时（60s）。
 *
 * 比常规 15s 长得多是一次性权衡：单文件上限 10MiB，弱网下 15s 传不完是常态，
 * 但超时太长又会让真正卡住的请求占着 UI。60s 覆盖 10MiB@1.4Mbps，
 * 再慢的链路本来就该提示用户换网络。
 */
export const ATTACHMENT_UPLOAD_TIMEOUT_MS = 60_000;

/** 站内附件 URL 前缀（与后端 router.go 的常量一致）。 */
export const ATTACHMENT_URL_PREFIX = "/api/v1/attachments/";

/** ---------------- 边界 ----------------
 * 一次性最多插几张图。与 ADR 0004 决策 4 一致：这是业务上限，不是单请求上限。
 */
export const ATTACHMENT_MAX_PER_DOCUMENT = 9;

/** 从站内附件 id 拼出正文里用的相对地址。 */
export function attachmentUrl(id: number): string {
  return `${ATTACHMENT_URL_PREFIX}${id}`;
}

/**
 * 判断一个图片地址是否是本站附件。
 *
 * 只认**本站前缀**的站内相对路径：外链 http(s) 图片（含别的域名下同名路径）
 * 不参与附件逻辑，也不走鉴权 blob 加载。与 ADR 0004 决策 3「只匹配本站前缀」同口径。
 */
export function isAttachmentUrl(src: string): boolean {
  if (!src.startsWith(ATTACHMENT_URL_PREFIX)) return false;
  const rest = src.slice(ATTACHMENT_URL_PREFIX.length);
  // 只接受纯数字 id，容忍尾部的 ?query / #hash（正文里可能手写）。
  const idPart = rest.split(/[?#]/)[0];
  return /^\d+$/.test(idPart);
}

/** 从站内附件地址解析出 id；不是本站附件返回 null。 */
export function parseAttachmentId(src: string): number | null {
  if (!isAttachmentUrl(src)) return null;
  const idPart = src.slice(ATTACHMENT_URL_PREFIX.length).split(/[?#]/)[0];
  return Number(idPart);
}

/**
 * 上传一张图片。
 *
 * 一次只传一张（ADR 0004 决策 4）：串行由调用方保证，这里不做并发控制。
 * FormData 的字段名后端不校验（只取第一个 part），用 "file" 语义最清楚。
 */
export function uploadAttachment(file: File): Promise<AttachmentUploadResp> {
  const form = new FormData();
  form.append("file", file, file.name);
  return requestWithSessionOptions<AttachmentUploadResp>(
    "/api/v1/attachments",
    { method: "POST", body: form },
    { timeoutMs: ATTACHMENT_UPLOAD_TIMEOUT_MS },
  );
}

/**
 * 把站内附件读成可用的 <img src>。
 *
 * 为什么需要这一步：读取端点挂了 authRequired，而浏览器加载 <img> 不会带
 * Authorization 头，直接写 `/api/v1/attachments/123` 必然 401，退化成
 * 「图片不可用」占位块。这里用带 token 的 fetch 取回二进制，再用
 * URL.createObjectURL 生成本地地址给 <img> 用。
 *
 * 调用方必须负责在组件卸载时调用 revokeObjectURL，否则 blob 会一直占内存。
 * 401 的刷新重试由 requestWithSessionOptions 统一处理（与其它接口一致）。
 */
export async function loadAttachmentObjectUrl(id: number): Promise<string> {
  const response = await fetch(attachmentUrl(id), {
    headers: await attachmentAuthHeaders(),
  });
  if (!response.ok) {
    throw new Error(`附件加载失败（${response.status}）`);
  }
  const blob = await response.blob();
  return URL.createObjectURL(blob);
}

/** 取当前会话的 Authorization 头；未登录直接抛错（不发起匿名请求）。 */
async function attachmentAuthHeaders(): Promise<HeadersInit> {
  // 延迟 import 避免 api 层与 auth 层在模块初始化时互相依赖。
  const { readAuthSession } = await import("@/auth/session");
  const session = readAuthSession();
  if (!session?.access_token) {
    throw new Error("登录状态已失效，请重新登录");
  }
  return { Authorization: `Bearer ${session.access_token}` };
}
