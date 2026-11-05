/**
 * 学习路线管理端的纯逻辑（#223）。
 *
 * 单独抽出来是因为这些规则最容易出错、也最值得测：
 * 阶段上下移动、新增阶段、资源上下移动，以及提交前的本地校验。
 *
 * 一条硬约束：**前端不生成阶段 id**（ADR 0006 决策 3）。新增阶段时 id 留空串，
 * 由服务端在保存时生成。前端自造的 id 会被服务端当成伪造而改发新 id，
 * 那等于白白丢掉「原阶段保留 id」的收益。
 */
import type { RoadmapResource, RoadmapStage } from "@iot-pillot/shared-types";

/** 与后端一致的边界。 */
export const MAX_STAGES = 20;
export const MAX_RESOURCES_PER_STAGE = 20;

/** 新建一个空阶段（id 留空，等服务端生成）。 */
export function newStage(): RoadmapStage {
  return { id: "", title: "", summary: "", resources: [] };
}

/** 新建一个空资源。 */
export function newResource(): RoadmapResource {
  return { title: "", url: "" };
}

/**
 * 把数组里第 index 项与它上/下一位交换。
 *
 * 越界时原样返回（而不是报错）：按钮在边界处会被禁用，
 * 但键盘操作或竞态仍可能触达，静默无操作比抛错更合适。
 */
function move<T>(items: T[], index: number, delta: -1 | 1): T[] {
  const target = index + delta;
  if (index < 0 || index >= items.length || target < 0 || target >= items.length) {
    return items;
  }
  const next = items.slice();
  [next[index], next[target]] = [next[target]!, next[index]!];
  return next;
}

/** 阶段上移 / 下移（AC：用上下移动按钮排序，不做拖拽）。 */
export function moveStage(stages: RoadmapStage[], index: number, delta: -1 | 1): RoadmapStage[] {
  return move(stages, index, delta);
}

/** 资源上移 / 下移（与阶段同样的交互，不做拖拽）。 */
export function moveResource(
  resources: RoadmapResource[],
  index: number,
  delta: -1 | 1,
): RoadmapResource[] {
  return move(resources, index, delta);
}

/**
 * 链接协议白名单 —— 与渲染层同一份规则（ADR 0005 决策 5）。
 *
 * 这里**只做即时反馈**：真正的拒绝在服务端（AC 明确要求「不在渲染层兜底」）。
 * 前端复用同一个判断是为了让用户在保存前就看到问题，而不是提交后被拒。
 */
export function isAllowedResourceUrl(url: string): boolean {
  const trimmed = url.trim();
  if (trimmed === "") return false;
  if (trimmed.startsWith("/")) {
    // 排除 "//host"：那是会跳到外部站点的协议相对 URL。
    return !trimmed.startsWith("//");
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

/** 提交前的本地校验结果。`null` 表示通过。 */
export interface RoadmapValidationIssue {
  /** 出错位置的可读描述，用于把用户定位到具体输入框。 */
  where: string;
  message: string;
}

/**
 * 提交前校验，返回第一个问题或 null。
 *
 * 与后端 ValidateRoadmapStages 同口径，但**不是唯一防线**：
 * 服务端仍会再校验一次，这里只是让用户少一次「提交后才被拒」。
 * 用「第 N 个阶段」定位而不是 id —— id 对管理员是不可见的技术细节。
 */
export function validateRoadmapDraft(input: {
  title: string;
  directionCode: string;
  stages: RoadmapStage[];
}): RoadmapValidationIssue | null {
  if (input.title.trim() === "") {
    return { where: "标题", message: "标题不能为空" };
  }
  if (input.directionCode.trim() === "") {
    return { where: "方向", message: "请选择方向" };
  }
  if (input.stages.length > MAX_STAGES) {
    return {
      where: "阶段",
      message: `阶段最多 ${MAX_STAGES} 个，当前 ${input.stages.length} 个`,
    };
  }

  for (let i = 0; i < input.stages.length; i += 1) {
    const stage = input.stages[i]!;
    const pos = i + 1;
    if (stage.title.trim() === "") {
      return { where: `第 ${pos} 个阶段`, message: `第 ${pos} 个阶段缺少标题` };
    }
    if (stage.resources.length > MAX_RESOURCES_PER_STAGE) {
      return {
        where: `第 ${pos} 个阶段`,
        message: `第 ${pos} 个阶段最多 ${MAX_RESOURCES_PER_STAGE} 个资源，当前 ${stage.resources.length} 个`,
      };
    }
    for (let j = 0; j < stage.resources.length; j += 1) {
      const res = stage.resources[j]!;
      const rpos = j + 1;
      if (res.title.trim() === "") {
        return {
          where: `第 ${pos} 个阶段的第 ${rpos} 个资源`,
          message: `第 ${pos} 个阶段的第 ${rpos} 个资源缺少标题`,
        };
      }
      if (!isAllowedResourceUrl(res.url)) {
        return {
          where: `第 ${pos} 个阶段的第 ${rpos} 个资源`,
          message: `第 ${pos} 个阶段的第 ${rpos} 个资源链接不合规：只允许 http/https 或站内相对路径`,
        };
      }
    }
  }
  return null;
}
