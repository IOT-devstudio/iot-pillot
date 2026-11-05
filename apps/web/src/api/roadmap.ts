/**
 * 学习路线 API（#223 管理端；#224 会复用同一批读取函数）。
 *
 * 后端契约：/api/v1/admin/studio/roadmaps（仅管理员）。
 * 网络层保持后端 snake_case。
 *
 * 保存语义是**整体覆盖 + 版本号乐观锁**：一个页面改完标题、正文、阶段列表后
 * 一次 PUT 全量提交，必须带上当前看到的 version。缺 version 后端会明确报错
 * 而不是假装成功——那会让乐观锁退化成「总是覆盖」。
 */
import type {
  RoadmapDetail,
  RoadmapListQuery,
  RoadmapListResp,
  RoadmapWritePayload,
} from "@iot-pillot/shared-types";
import { requestMessageWithSession, requestWithSession } from "./session-request";

/** 管理端接口前缀。用户端只读入口在 #224 落地后另行给出。 */
const ADMIN_BASE = "/api/v1/admin/studio/roadmaps";

function listQuery(query: RoadmapListQuery): string {
  const params = new URLSearchParams();
  if (query.direction) {
    params.set("direction", query.direction);
  }
  if (query.page !== undefined) {
    params.set("page", String(query.page));
  }
  if (query.page_size !== undefined) {
    params.set("page_size", String(query.page_size));
  }
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

/** 分页列出路线。响应里一并带回方向筛选项（只含有路线的方向）。 */
export function listRoadmaps(query: RoadmapListQuery = {}): Promise<RoadmapListResp> {
  return requestWithSession<RoadmapListResp>(`${ADMIN_BASE}${listQuery(query)}`);
}

/** 取单条路线的完整内容（含正文与阶段）。 */
export function fetchRoadmap(id: number): Promise<RoadmapDetail> {
  return requestWithSession<RoadmapDetail>(`${ADMIN_BASE}/${id}`);
}

/** 新建路线。阶段 id 留空串，由服务端生成。 */
export function createRoadmap(payload: RoadmapWritePayload): Promise<RoadmapDetail> {
  return requestWithSession<RoadmapDetail>(ADMIN_BASE, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * 整体保存路线。
 *
 * `version` 必须带上当前看到的版本号：被别人先保存过时后端会拒绝并给出可读错误，
 * 而不是默默覆盖别人的改动。
 */
export function updateRoadmap(
  id: number,
  payload: RoadmapWritePayload,
): Promise<RoadmapDetail> {
  return requestWithSession<RoadmapDetail>(`${ADMIN_BASE}/${id}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

/**
 * 软删路线（后端记录操作人，不做物理删除）。
 *
 * 走后端「只回 message 不回 data」的出口：删除接口的 data 会被 omitempty 丢掉，
 * 若按 data 是否缺失判断成败，成功的删除会弹出「服务暂时不可用」的假失败。
 */
export function deleteRoadmap(id: number): Promise<string> {
  return requestMessageWithSession(`${ADMIN_BASE}/${id}`, { method: "DELETE" });
}
