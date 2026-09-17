/**
 * 编辑器的草稿模型与全部编辑操作（纯函数）。
 *
 * 设计要点：
 *  1. 草稿项带一个稳定的 `id`，而不是用 name 当 key。编辑期间改名字不会打断
 *     选中状态与正在进行的拖拽——用 name 当 key 就会出现"改完名字选中丢了"。
 *  2. 所有操作都是纯函数、返回新对象，不改传入参数。这样撤销/重做、以及
 *     单测都不需要额外的框架支持。
 *  3. 校验直接复用 buildings.ts 的判定（多边形相交、重名、退化顶点），
 *     保证"编辑器说没问题"与"运行时校验通过"是同一套标准。
 */
import {
  BUILDINGS,
  type BuildingData,
  findBuilding,
  findDegenerateFootprints,
  findDuplicateNames,
  findOverlappingBuildings,
} from "../opener/buildings";
import { CONFIG } from "../opener/config";
import {
  type FootprintPoint,
  footprintBounds,
  pointInPolygon,
  polygonArea,
  polygonCentroid,
  rectFootprint,
  roundCoord,
} from "../opener/footprint";

/** 可编辑的楼栋草稿 */
export interface BuildingDraft {
  /** 稳定标识，仅供编辑器内部使用，不进入生成的代码 */
  id: string;
  name: string;
  x: number;
  z: number;
  h: number;
  footprint: FootprintPoint[];
}

export interface DraftIssue {
  level: "error" | "warning";
  message: string;
  /** 相关楼栋名，便于在画布上高亮 */
  buildings?: string[];
}

let idSeed = 0;

/** 生成一个进程内唯一的草稿 id（不用随机数，方便测试时稳定断言） */
export function nextDraftId(): string {
  idSeed += 1;
  return `draft-${idSeed}`;
}

/** 供测试重置 id 计数器 */
export function resetDraftIdSeed(): void {
  idSeed = 0;
}

/** 从现有 BUILDINGS 建一份草稿（编辑器打开时的初始状态） */
export function fromBuildingData(
  data: BuildingData[] = BUILDINGS,
): BuildingDraft[] {
  return data.map((building) => ({
    id: nextDraftId(),
    name: building.name,
    x: building.x,
    z: building.z,
    h: building.h,
    footprint: building.footprint.map(([px, pz]) => [px, pz] as FootprintPoint),
  }));
}

/** 草稿转回数据模型（代码生成与校验都基于它） */
export function toBuildingData(drafts: BuildingDraft[]): BuildingData[] {
  return drafts.map((draft) => ({
    name: draft.name,
    x: draft.x,
    z: draft.z,
    h: draft.h,
    footprint: draft.footprint.map(([px, pz]) => [px, pz] as FootprintPoint),
  }));
}

/** 起一个不重名的默认楼栋名 */
export function nextBuildingName(drafts: BuildingDraft[]): string {
  const used = new Set(drafts.map((draft) => draft.name));
  for (let i = 1; i < 1000; i += 1) {
    const candidate = `Building_New_${i}`;
    if (!used.has(candidate)) {
      return candidate;
    }
  }
  return `Building_New_${Date.now()}`;
}

/** 新建一栋默认矩形楼栋，摆在地面上一个尽量不冲突的位置 */
export function createBuilding(drafts: BuildingDraft[]): BuildingDraft {
  return {
    id: nextDraftId(),
    name: nextBuildingName(drafts),
    // 简单错开摆放：新楼逐个往右下角挪，避免一上来就完全重合
    x: roundCoord(60 + drafts.length * 4),
    z: roundCoord(-60 + drafts.length * 4),
    h: 12,
    footprint: rectFootprint(16, 12),
  };
}

/* ------------------------------------------------------------------ *
 * 编辑操作
 * ------------------------------------------------------------------ */

function replaceDraft(
  drafts: BuildingDraft[],
  id: string,
  update: (draft: BuildingDraft) => BuildingDraft,
): BuildingDraft[] {
  return drafts.map((draft) => (draft.id === id ? update(draft) : draft));
}

/** 平移整栋楼（世界坐标增量，米） */
export function moveBuilding(
  drafts: BuildingDraft[],
  id: string,
  deltaX: number,
  deltaZ: number,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => ({
    ...draft,
    x: roundCoord(draft.x + deltaX),
    z: roundCoord(draft.z + deltaZ),
  }));
}

/** 把楼栋原点设为指定世界坐标（属性面板里直接输入时用） */
export function setBuildingPosition(
  drafts: BuildingDraft[],
  id: string,
  x: number,
  z: number,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => ({
    ...draft,
    x: roundCoord(x),
    z: roundCoord(z),
  }));
}

export function setBuildingHeight(
  drafts: BuildingDraft[],
  id: string,
  h: number,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => ({
    ...draft,
    // 高度必须为正，否则挤出体厚度为 0（three 会画出空几何）
    h: roundCoord(Math.max(0.1, h)),
  }));
}

export function renameBuilding(
  drafts: BuildingDraft[],
  id: string,
  name: string,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => ({ ...draft, name: name.trim() }));
}

/** 拖动某个顶点（顶点坐标是**局部**坐标，米） */
export function moveVertex(
  drafts: BuildingDraft[],
  id: string,
  vertexIndex: number,
  localX: number,
  localZ: number,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => {
    if (vertexIndex < 0 || vertexIndex >= draft.footprint.length) {
      return draft;
    }
    const footprint = draft.footprint.map((point, index) =>
      index === vertexIndex
        ? ([roundCoord(localX), roundCoord(localZ)] as FootprintPoint)
        : point,
    );
    return { ...draft, footprint };
  });
}

/**
 * 在某条边的中点插入一个新顶点。
 *
 * @param edgeIndex 边的序号：边 i 连接顶点 i 与 i+1（末尾边回到 0 号顶点）
 */
export function addVertexOnEdge(
  drafts: BuildingDraft[],
  id: string,
  edgeIndex: number,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => {
    const count = draft.footprint.length;
    if (edgeIndex < 0 || edgeIndex >= count) {
      return draft;
    }

    const start = draft.footprint[edgeIndex];
    const end = draft.footprint[(edgeIndex + 1) % count];
    if (!start || !end) {
      return draft;
    }

    const midpoint: FootprintPoint = [
      roundCoord((start[0] + end[0]) / 2),
      roundCoord((start[1] + end[1]) / 2),
    ];

    const footprint = [...draft.footprint];
    footprint.splice(edgeIndex + 1, 0, midpoint);
    return { ...draft, footprint };
  });
}

/**
 * 删除一个顶点。
 * 少于 3 个顶点构不出体，所以拒绝并原样返回（由调用方给出提示）。
 */
export function deleteVertex(
  drafts: BuildingDraft[],
  id: string,
  vertexIndex: number,
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => {
    if (draft.footprint.length <= 3) {
      return draft;
    }
    if (vertexIndex < 0 || vertexIndex >= draft.footprint.length) {
      return draft;
    }
    return {
      ...draft,
      footprint: draft.footprint.filter((_, index) => index !== vertexIndex),
    };
  });
}

/** 用给定顶点整体替换底面（例如从预设形状生成） */
export function setFootprint(
  drafts: BuildingDraft[],
  id: string,
  footprint: FootprintPoint[],
): BuildingDraft[] {
  return replaceDraft(drafts, id, (draft) => ({
    ...draft,
    footprint: footprint.map(([x, z]) => [roundCoord(x), roundCoord(z)] as FootprintPoint),
  }));
}

export function addBuilding(drafts: BuildingDraft[]): BuildingDraft[] {
  return [...drafts, createBuilding(drafts)];
}

export function deleteBuilding(
  drafts: BuildingDraft[],
  id: string,
): BuildingDraft[] {
  return drafts.filter((draft) => draft.id !== id);
}

/**
 * 把某栋楼设为工作室，并把 CONFIG 里的工作室名一起改掉。
 *
 * 只改 name 不够：CONFIG.studioBuildingName 决定谁被高亮、谁成为镜头焦点，
 * 两处不一致会出现"改了没反应"。所以这里同时把原工作室改名让位，
 * 保证任一时刻只有一个工作室。
 */
export function setStudioBuilding(
  drafts: BuildingDraft[],
  id: string,
  studioName: string,
): BuildingDraft[] {
  return drafts.map((draft) => {
    if (draft.id === id) {
      return { ...draft, name: studioName };
    }
    // 原来顶着工作室名字的那栋要让位，否则会重名
    if (draft.name === studioName) {
      return { ...draft, name: nextBuildingName(drafts.filter((d) => d.id !== draft.id)) };
    }
    return draft;
  });
}

/* ------------------------------------------------------------------ *
 * 校验
 * ------------------------------------------------------------------ */

/**
 * 校验草稿。
 *
 * 与运行时 reportSceneIssues 用的是同一批判定函数，所以这里报出来的问题
 * 就是粘贴回代码后启动时会在控制台看到的问题——在复制之前就能发现。
 */
export function validateDraft(
  drafts: BuildingDraft[],
  studioName: string = CONFIG.studioBuildingName,
): DraftIssue[] {
  const issues: DraftIssue[] = [];
  const data = toBuildingData(drafts);

  for (const name of findDuplicateNames(data)) {
    issues.push({ level: "error", message: `存在重名的楼栋：${name}`, buildings: [name] });
  }

  for (const name of findDegenerateFootprints(data)) {
    issues.push({
      level: "error",
      message: `${name} 的底面顶点少于 3 个，构不出体`,
      buildings: [name],
    });
  }

  const zeroArea = data
    .filter((building) => polygonArea(building.footprint) < 1)
    .map((building) => building.name);
  for (const name of zeroArea) {
    issues.push({
      level: "error",
      message: `${name} 的底面面积接近 0，检查顶点是否共线`,
      buildings: [name],
    });
  }

  for (const [a, b] of findOverlappingBuildings(data)) {
    issues.push({
      level: "error",
      message: `${a} 与 ${b} 的底面相交，画面上会穿模`,
      buildings: [a, b],
    });
  }

  const studio = findBuilding(studioName, data);
  if (!studio) {
    issues.push({
      level: "warning",
      message: `没有任何楼栋叫 ${studioName}，开屏时不会有工作室高亮与聚焦`,
    });
    return issues;
  }

  if (!isOriginInsideFootprint(studio)) {
    issues.push({
      level: "warning",
      message: `${studio.name} 的底面原点落在多边形之外（例如落在 L 形的凹口里），镜头会对准楼外的空地`,
      buildings: [studio.name],
    });
  }

  return issues;
}

/**
 * 楼栋局部原点是否落在自己的底面内。
 *
 * 相机注视点就是「楼栋原点 + 底面形心」，而楼栋原点本身要是落在多边形的凹口
 * 或其他空地上，画面会出现"楼在旁边、镜头对着空气"。这里用真正的
 * pointInPolygon 判定，不用包围盒近似——L 形的凹口正是包围盒判不出来的地方。
 */
export function isOriginInsideFootprint(building: BuildingData): boolean {
  return pointInPolygon(building.footprint, [0, 0]);
}

/** 工作室底面的形心（世界坐标），用于推导相机注视点 */
export function studioFocusPoint(building: BuildingData): {
  x: number;
  y: number;
  z: number;
} {
  const [cx, cz] = polygonCentroid(building.footprint);
  return {
    x: roundCoord(building.x + cx),
    y: roundCoord(building.h / 2),
    z: roundCoord(building.z + cz),
  };
}

/** 底面在世界坐标下的最大跨度（相机距离按它成比例） */
export function footprintSpan(building: BuildingData): number {
  const bounds = footprintBounds(building.footprint);
  return roundCoord(Math.max(bounds.maxX - bounds.minX, bounds.maxZ - bounds.minZ));
}
