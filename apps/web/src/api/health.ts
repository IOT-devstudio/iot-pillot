export interface HealthData {
  status: string;
  time: string;
}

interface HealthResponse {
  data?: HealthData;
}

/** 请求后端根健康检查接口。 */
export async function fetchHealth(): Promise<HealthData> {
  const response = await fetch("/health");
  const body = (await response.json()) as HealthResponse;

  if (!response.ok || body.data === undefined) {
    throw new Error("健康检查失败");
  }

  return body.data;
}
