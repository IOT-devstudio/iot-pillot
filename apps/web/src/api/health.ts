export interface HealthData {
  status: string;
  time: string;
}

interface HealthResponse {
  data?: HealthData;
}

/**
 * 健康检查失败的分类文案。
 *
 * 与 useAdminPermissions 的 describeAuthError 同一思路：把异常翻成用户能看懂的
 * 中文，**绝不**把引擎抛出的原始 message（TypeError: Failed to fetch、
 * SyntaxError: Unexpected token '<'…）直接放到界面上。
 */
export const HEALTH_UNREACHABLE = "连接失败，无法访问后端服务";
export const HEALTH_BAD_RESPONSE = "后端服务响应异常";

/** 超时也归为「连接失败」：后端挂起时卡片不应永远停在「检测中…」 */
const FETCH_TIMEOUT_MS = 5000;

/**
 * 请求后端根健康检查接口。
 *
 * 三层失败各自归位：
 *   - fetch 直接抛（网络不可达 / 超时）        → 连接失败
 *   - 响应不是 JSON（代理返回 HTML 错误页等）  → 连接失败
 *   - 响应是 JSON 但非 2xx 或缺 data           → 服务响应异常
 *
 * 失败一律抛 Error，message 已经是上面的中文文案。
 */
export async function fetchHealth(): Promise<HealthData> {
  let response: Response;
  try {
    response = await fetch("/health", {
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
  } catch {
    // 网络不可达、超时（AbortError）都在这里，不区分——用户视角都是「连不上」
    throw new Error(HEALTH_UNREACHABLE);
  }

  let body: Partial<HealthResponse> | null;
  try {
    body = (await response.json()) as Partial<HealthResponse> | null;
  } catch {
    // 非 JSON：本机 vite 代理打不到后端时会回 500 + HTML，正是这种场景
    throw new Error(HEALTH_UNREACHABLE);
  }

  if (!response.ok || body?.data === undefined) {
    throw new Error(HEALTH_BAD_RESPONSE);
  }

  return body.data;
}

/**
 * 给 UI 用的失败翻译器：只透传本模块自己分类过的文案，其余一律兜底。
 *
 * 这样即使有人在别处误用了别的异常（或未来 fetchHealth 改动漏了分类），
 * 界面上也不会冒出引擎的原始错误。
 */
export function describeHealthError(error: unknown): string {
  if (
    error instanceof Error &&
    (error.message === HEALTH_UNREACHABLE ||
      error.message === HEALTH_BAD_RESPONSE)
  ) {
    return error.message;
  }
  return "暂时无法获取后端状态，请稍后重试";
}
