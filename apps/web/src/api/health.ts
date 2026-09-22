/** 后端健康检查接口的领域类型（apps/api/internal/handler/health_handle.go）。 */
export interface HealthData {
  status: string;
  time: string;
}

interface HealthResponse {
  data?: HealthData;
}

/**
 * 健康检查请求的失败类型，按"对用户有没有用"做分类。
 *
 * 跟 api/auth.ts 的 AuthRequestError 一个套路：把 fetch 抛错、JSON 解析失败、
 * HTTP 非 2xx、body 缺 data 这些都归一到一种 Error，调用方只关心"消息怎么
 * 展示给用户"和"网络挂了"两个事实。
 */
export class HealthRequestError extends Error {
  /** 网络不可达 = fetch 本身抛错 / JSON 解析失败；后端没响应或响应不是 JSON 都归这里 */
  public readonly network: boolean;
  /** 真正的 status：fetch 抛错时是 0 */
  public readonly status: number;

  constructor(message: string, network: boolean, status: number) {
    super(message);
    this.name = "HealthRequestError";
    this.network = network;
    this.status = status;
  }
}

/** 健康检查默认超时（毫秒）。后端挂起时让卡片别永远停在「检测中…」 */
const HEALTH_REQUEST_TIMEOUT_MS = 5000;

/** 请求后端根健康检查接口。失败一律抛 HealthRequestError，不让原始异常穿透。 */
export async function fetchHealth(): Promise<HealthData> {
  let response: Response;
  try {
    response = await fetch("/health", {
      signal: AbortSignal.timeout(HEALTH_REQUEST_TIMEOUT_MS),
    });
  } catch {
    // fetch 抛 TypeError: Failed to fetch / AbortError，代理返 HTML 让 .json() 抛 SyntaxError
    // —— 都属于"网络/协议层失败"，对后端给不了具体状态码，用 status=0 兜底。
    throw new HealthRequestError("网络异常", true, 0);
  }

  let body: HealthResponse;
  try {
    body = (await response.json()) as HealthResponse;
  } catch {
    throw new HealthRequestError("响应非 JSON", true, response.status);
  }

  if (!response.ok || body.data === undefined) {
    throw new HealthRequestError("后端不健康", false, response.status);
  }

  return body.data;
}

/**
 * 把异常翻译成给用户看的中文短句，避免把 `(e as Error).message` 原样塞 UI。
 *
 * 口径见 issue #56：
 * - 网络/协议失败 → 「连接失败，无法访问后端服务」
 * - 后端响应异常 → 「后端服务响应异常」
 */
export function describeHealthError(error: unknown): string {
  if (error instanceof HealthRequestError) {
    return error.network
      ? "连接失败，无法访问后端服务"
      : "后端服务响应异常";
  }
  // 兜底：未知错误不暴露原始 message
  return "后端服务状态未知";
}