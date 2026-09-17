/**
 * 代码生成：把草稿导出成可直接粘贴回源码的 TS 片段。
 *
 * 刻意生成**显式顶点数组**，而不是复用 rectFootprint / regularFootprint：
 * 生成物要满足"复制 → 粘贴 → 能编译"，不能依赖调用方记得额外 import 了哪些
 * 辅助函数。形状再复杂，顶点写全了就是自洽的。
 *
 * 相机片段按工作室的几何推导：注视点取**底面形心**与中部高度（保证平视）。
 * 形心而不是局部原点，是因为 L 形等不规则体的原点可能偏在一侧，用形心才居中。
 * 机位偏移比例（0.68 / 1.14 / 0.25）是按当前手调值反推的：默认工作室跨度为
 * 22 米时，偏移正好是 15 / 25 米、抬升 5 米，与 config.ts 里的现有数值一致，
 * 因此只有"注视点改用形心"这一点会带来小幅位移，其余不会漂。
 */
import { type BuildingData, findBuilding } from "../opener/buildings";
import { CONFIG } from "../opener/config";
import { type FootprintPoint, footprintBounds, roundCoord } from "../opener/footprint";
import {
  type BuildingDraft,
  footprintSpan,
  toBuildingData,
  studioFocusPoint,
} from "./draft";

/** 相机机位相对注视点的偏移比例（按工作室底面跨度缩放） */
const CAMERA_OFFSET_RATIO = {
  x: 0.68,
  z: 1.14,
  /** 相对楼栋中部高度的抬升比例 */
  y: 0.25,
} as const;

function formatNumber(value: number): string {
  return String(roundCoord(value));
}

function formatPoint([x, z]: FootprintPoint): string {
  return `[${formatNumber(x)}, ${formatNumber(z)}]`;
}

/** 把一栋楼的底面格式化成多行顶点数组字面量 */
function formatFootprint(footprint: FootprintPoint[], indent: string): string {
  const lines = footprint.map((point) => `${indent}  ${formatPoint(point)},`);
  return `[\n${lines.join("\n")}\n${indent}]`;
}

/**
 * 生成 BUILDINGS 数组的完整代码。
 *
 * 输出是可直接替换 buildings.ts 里同名常量的片段（含类型注解），
 * 顺序与草稿一致，方便 diff。
 */
export function generateBuildingsCode(drafts: BuildingDraft[]): string {
  if (drafts.length === 0) {
    return "export const BUILDINGS: BuildingData[] = [];";
  }

  const entries = drafts.map((draft) => {
    return [
      "  {",
      `    name: ${JSON.stringify(draft.name)},`,
      `    x: ${formatNumber(draft.x)},`,
      `    z: ${formatNumber(draft.z)},`,
      `    h: ${formatNumber(draft.h)},`,
      `    footprint: ${formatFootprint(draft.footprint, "    ")},`,
      "  },",
    ].join("\n");
  });

  return [
    "export const BUILDINGS: BuildingData[] = [",
    entries.join("\n"),
    "];",
  ].join("\n");
}

export interface CameraSnippet {
  /** 相机机位 */
  cameraStudio: { x: number; y: number; z: number };
  /** 注视点（工作室底面形心 + 中部高度，因此是平视） */
  cameraStudioTarget: { x: number; y: number; z: number };
}

/**
 * 由工作室的几何推导相机参数。
 *
 * 平视的充要条件是注视点 y ≈ 楼栋高度的一半；机位 y 取中部再抬 25%，
 * 免得太贴地平线。距离按底面最大跨度缩放，换一栋更大的工作室也不用重新试。
 */
export function deriveCameraSnippet(
  drafts: BuildingDraft[],
  studioName: string = CONFIG.studioBuildingName,
): CameraSnippet | null {
  const studio = findBuilding(studioName, toBuildingData(drafts));
  if (!studio) {
    return null;
  }

  const target = studioFocusPoint(studio);
  const span = footprintSpan(studio);
  const height = studio.h;

  return {
    cameraStudio: {
      x: roundCoord(target.x + span * CAMERA_OFFSET_RATIO.x),
      y: roundCoord(height / 2 + height * CAMERA_OFFSET_RATIO.y),
      z: roundCoord(target.z + span * CAMERA_OFFSET_RATIO.z),
    },
    cameraStudioTarget: target,
  };
}

/** 生成 CONFIG 里相机相关字段的片段（连同缩进，可直接贴进 config.ts） */
export function generateCameraCode(
  drafts: BuildingDraft[],
  studioName: string = CONFIG.studioBuildingName,
): string {
  const snippet = deriveCameraSnippet(drafts, studioName);
  if (snippet === null) {
    return `// BUILDINGS 里没有名为 ${studioName} 的楼栋，无法推导相机参数。`;
  }

  const { cameraStudio, cameraStudioTarget } = snippet;

  return [
    "  /** 阶段 2 结束后聚焦工作室时的机位 */",
    `  cameraStudio: { x: ${formatNumber(cameraStudio.x)}, y: ${formatNumber(cameraStudio.y)}, z: ${formatNumber(cameraStudio.z)} },`,
    "  /** 阶段 2 结束后看向的点；y 取工作室中部高度，因此与楼栋平视 */",
    `  cameraStudioTarget: { x: ${formatNumber(cameraStudioTarget.x)}, y: ${formatNumber(cameraStudioTarget.y)}, z: ${formatNumber(cameraStudioTarget.z)} },`,
  ].join("\n");
}

/**
 * 生成可直接复制的完整代码：BUILDINGS 数组 + 相机 CONFIG 片段。
 *
 * 两段之间用注释分隔，并带上工作室名，避免粘贴时贴错位置。
 */
export function generateCode(
  drafts: BuildingDraft[],
  studioName: string = CONFIG.studioBuildingName,
): string {
  return [
    `// ── 1) 替换 apps/web/src/modules/opener/buildings.ts 里的 BUILDINGS ──`,
    generateBuildingsCode(drafts),
    "",
    `// ── 2) 把下面两行替换进 apps/web/src/modules/opener/config.ts 的 CONFIG ──`,
    `//    工作室：${studioName}`,
    generateCameraCode(drafts, studioName),
  ].join("\n");
}

/** 便捷函数：从数据模型直接生成代码（例如给"导入当前配置"用） */
export function generateCodeFromData(
  data: BuildingData[],
  studioName: string = CONFIG.studioBuildingName,
): string {
  const drafts: BuildingDraft[] = data.map((building, index) => ({
    id: `imported-${index + 1}`,
    name: building.name,
    x: building.x,
    z: building.z,
    h: building.h,
    footprint: building.footprint.map(([x, z]) => [x, z] as FootprintPoint),
  }));

  return generateCode(drafts, studioName);
}

/** 画布上用于自适应视图的包围盒（把草稿摊平成一个矩形范围） */
export function draftBounds(drafts: BuildingDraft[]): {
  minX: number;
  maxX: number;
  minZ: number;
  maxZ: number;
} {
  if (drafts.length === 0) {
    return { minX: -10, maxX: 10, minZ: -10, maxZ: 10 };
  }

  let minX = Infinity;
  let maxX = -Infinity;
  let minZ = Infinity;
  let maxZ = -Infinity;

  for (const building of toBuildingData(drafts)) {
    const bounds = footprintBounds(building.footprint);
    minX = Math.min(minX, bounds.minX + building.x);
    maxX = Math.max(maxX, bounds.maxX + building.x);
    minZ = Math.min(minZ, bounds.minZ + building.z);
    maxZ = Math.max(maxZ, bounds.maxZ + building.z);
  }

  return { minX, maxX, minZ, maxZ };
}
