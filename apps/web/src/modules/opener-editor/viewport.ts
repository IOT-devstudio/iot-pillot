/**
 * 俯视画布的视口变换与命中测试（纯数学）。
 *
 * 为什么要单独一层：拖拽手感全在这几个公式里，而组件里的鼠标事件在无浏览器
 * 环境下没法验证。把变换抽成纯函数后，缩放/平移/点选的正确性都有单测兜底，
 * 组件只剩"把鼠标像素坐标丢进来、把结果发给草稿操作"这点接线。
 *
 * 坐标约定：
 *   世界坐标 (x, z) —— 与 BUILDINGS 一致，+x 向右、+z 朝向观察者（画面下方）
 *   屏幕坐标 (sx, sy) —— SVG 用户单位，原点在左上角
 *   因此 sz = z，屏幕纵轴与世界 z 轴同向，画面正好是"从正上方往下看"。
 */
import type { FootprintPoint } from "../opener/footprint";
import { footprintToWorld, pointInPolygon } from "../opener/footprint";
import type { BuildingDraft } from "./draft";
import { toBuildingData } from "./draft";

export interface Viewport {
  /** 每米对应多少 SVG 用户单位 */
  scale: number;
  /** 世界原点在屏幕上的位置 */
  originX: number;
  originY: number;
}

/** 世界坐标 → 屏幕坐标 */
export function worldToScreen(
  viewport: Viewport,
  x: number,
  z: number,
): [number, number] {
  return [
    viewport.originX + x * viewport.scale,
    viewport.originY + z * viewport.scale,
  ];
}

/** 屏幕坐标 → 世界坐标 */
export function screenToWorld(
  viewport: Viewport,
  screenX: number,
  screenY: number,
): [number, number] {
  return [
    (screenX - viewport.originX) / viewport.scale,
    (screenY - viewport.originY) / viewport.scale,
  ];
}

/**
 * 计算一个刚好装下所有楼栋的视口。
 *
 * @param padding 每边留白（SVG 用户单位），避免楼栋贴着画布边缘不好拖
 */
export function fitViewport(
  drafts: BuildingDraft[],
  width: number,
  height: number,
  padding = 48,
): Viewport {
  if (drafts.length === 0) {
    return { scale: 1, originX: width / 2, originY: height / 2 };
  }

  let minX = Infinity;
  let maxX = -Infinity;
  let minZ = Infinity;
  let maxZ = -Infinity;

  for (const building of toBuildingData(drafts)) {
    for (const [x, z] of footprintToWorld(building.footprint, building.x, building.z)) {
      minX = Math.min(minX, x);
      maxX = Math.max(maxX, x);
      minZ = Math.min(minZ, z);
      maxZ = Math.max(maxZ, z);
    }
  }

  const spanX = Math.max(1, maxX - minX);
  const spanZ = Math.max(1, maxZ - minZ);
  const usableWidth = Math.max(1, width - padding * 2);
  const usableHeight = Math.max(1, height - padding * 2);
  const scale = Math.min(usableWidth / spanX, usableHeight / spanZ);

  // 让内容居中：先算内容中心在屏幕上的目标位置，再反推世界原点该放哪
  const centerWorldX = (minX + maxX) / 2;
  const centerWorldZ = (minZ + maxZ) / 2;

  return {
    scale,
    originX: width / 2 - centerWorldX * scale,
    originY: height / 2 - centerWorldZ * scale,
  };
}

/** 以光标为锚点缩放（滚轮缩放时"光标下的东西不动"，比以画布中心缩放自然） */
export function zoomAt(
  viewport: Viewport,
  screenX: number,
  screenY: number,
  factor: number,
  minScale = 0.05,
  maxScale = 20,
): Viewport {
  const nextScale = Math.min(maxScale, Math.max(minScale, viewport.scale * factor));
  const applied = nextScale / viewport.scale;

  // 保持光标处的世界坐标不变：origin' = cursor - world * scale'
  const [worldX, worldZ] = screenToWorld(viewport, screenX, screenY);

  return {
    scale: nextScale,
    originX: screenX - worldX * nextScale,
    originY: screenY - worldZ * nextScale,
  };
}

/**
 * 命中测试：光标落在哪栋楼上。
 *
 * 逆序遍历（后画的在上面），点选结果与视觉层叠一致。
 */
export function hitTestBuilding(
  viewport: Viewport,
  drafts: BuildingDraft[],
  screenX: number,
  screenY: number,
): string | null {
  const [worldX, worldZ] = screenToWorld(viewport, screenX, screenY);
  const point: FootprintPoint = [worldX, worldZ];

  for (let i = drafts.length - 1; i >= 0; i -= 1) {
    const draft = drafts[i];
    if (!draft) {
      continue;
    }
    const worldPolygon = footprintToWorld(draft.footprint, draft.x, draft.z);
    if (pointInPolygon(worldPolygon, point)) {
      return draft.id;
    }
  }

  return null;
}

/**
 * 命中测试：光标落在指定楼栋的哪个顶点上。
 *
 * @param tolerance 容差（SVG 用户单位）；顶点只有几个像素大，必须给容差才点得中
 * @returns 顶点序号；没命中返回 null
 */
export function hitTestVertex(
  viewport: Viewport,
  draft: BuildingDraft,
  screenX: number,
  screenY: number,
  tolerance = 10,
): number | null {
  let bestIndex: number | null = null;
  let bestDistance = tolerance;

  draft.footprint.forEach(([localX, localZ], index) => {
    const [sx, sy] = worldToScreen(
      viewport,
      draft.x + localX,
      draft.z + localZ,
    );
    const distance = Math.hypot(sx - screenX, sy - screenY);
    if (distance <= bestDistance) {
      bestDistance = distance;
      bestIndex = index;
    }
  });

  return bestIndex;
}
