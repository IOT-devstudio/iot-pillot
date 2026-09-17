/**
 * 楼栋布局数据 —— 唯一需要按学校地图手改的地方。
 *
 * 每栋楼是一个**不规则多边形柱**：底面由 footprint 顶点数组定义，沿 Y 轴拉伸 h 高。
 * 不再是长方体，所以 L 形教学楼、梯形实验楼、六边形图书馆、八角体育馆都能直接表达。
 *
 * 字段含义与单位：
 *   name      楼栋标识。必须唯一；等于 CONFIG.studioBuildingName 的那一栋会被当成
 *             工作室（studioColor 上色 + 描边）并成为阶段 2 的对焦点。
 *   x, z      楼栋**底面局部原点**的地面坐标（米）。+x 向右，+z 朝向观察者（南）。
 *   h         高度，沿 Y 轴拉伸（米）。同时决定升起动画时长：越高升得越慢。
 *   footprint 底面多边形顶点数组，元素是 [x, z] 的**局部坐标**（相对 x/z 原点，米）。
 *             至少 3 个点；凹多边形（L 形）也支持；绕向随意（内部会归一化）。
 *             局部原点应当落在多边形内部，否则镜头会对着楼外看。
 *
 * 怎么换成真实学校地图坐标：
 *   1. 定一个原点（建议校园中心或主楼），量出每栋楼底面中心相对原点的东西向偏移
 *      → 填 x，南北向偏移 → 填 z。
 *   2. 用地图测距量出实际尺寸（米）写进 footprint；高度按「楼层数 × 3.2 米」估算填 h。
 *      比例一致即可，白模只要相对关系对。
 *   3. 形状建议用下面的 rectFootprint / regularFootprint 生成，或直接手写顶点数组。
 *   4. 改完确认没有两栋楼底面相交——findOverlappingBuildings() 会在控制台报出冲突的
 *      楼栋对，buildings.test.ts 也会在 CI 里拦住。注意判定是真正的多边形相交，
 *      所以「邻居正好落在 L 形的凹口里」不会被误报。
 *
 * 下面是 13 栋占位数据：高度 6~34 米、形状有 L 形/梯形/六边形/八边形/十二边形/
 * 不规则四边形/三角形，刻意做成错落有致，可当作「画面对不对」的基准。
 */

import {
  type FootprintPoint,
  footprintToWorld,
  polygonsOverlap,
  rectFootprint,
  regularFootprint,
} from "./footprint";

export interface BuildingData {
  name: string;
  x: number;
  z: number;
  h: number;
  footprint: FootprintPoint[];
}

export const BUILDINGS: BuildingData[] = [
  // ↓ 镜头焦点：L 形，h=20 对应 CONFIG.cameraStudioTarget.y=10（中部高度）
  {
    name: "Building_Studio",
    x: 15,
    z: 5,
    h: 20,
    footprint: [
      [-11, -8],
      [11, -8],
      [11, 0],
      [1, 0],
      [1, 8],
      [-11, 8],
    ],
  },
  // 正六边形图书馆
  { name: "Building_Library", x: -38, z: -22, h: 14, footprint: regularFootprint(6, 13) },
  { name: "Building_Lab_A", x: -6, z: -26, h: 22, footprint: rectFootprint(18, 18) },
  // 梯形实验楼
  {
    name: "Building_Lab_B",
    x: -30,
    z: 12,
    h: 26,
    footprint: [
      [-9, -7],
      [9, -7],
      [6, 7],
      [-6, 7],
    ],
  },
  // 正八边形食堂
  { name: "Building_Canteen", x: 44, z: -14, h: 9, footprint: regularFootprint(8, 13) },
  // 带凹口的长条宿舍：同样是凹多边形，用来压住三角化路径
  {
    name: "Building_Dorm_North",
    x: -46,
    z: 34,
    h: 18,
    footprint: [
      [-15, -7],
      [15, -7],
      [15, 7],
      [4, 7],
      [4, 2],
      [-15, 2],
    ],
  },
  { name: "Building_Dorm_South", x: -8, z: 38, h: 18, footprint: rectFootprint(30, 14) },
  // 正八边形体育馆
  { name: "Building_Gym", x: 34, z: 44, h: 12, footprint: regularFootprint(8, 14) },
  { name: "Building_Admin", x: 2, z: -46, h: 28, footprint: rectFootprint(20, 16) },
  // 正十二边形礼堂（近似圆形体量）
  {
    name: "Building_Auditorium",
    x: 46,
    z: 16,
    h: 16,
    footprint: regularFootprint(12, 15),
  },
  // 不规则四边形车间：四条边斜率都不同
  {
    name: "Building_Workshop",
    x: -20,
    z: -6,
    h: 8,
    footprint: [
      [-8, -6],
      [7, -8],
      [6, 5],
      [-4, 7],
    ],
  },
  { name: "Building_Tower", x: 22, z: -30, h: 34, footprint: rectFootprint(12, 12) },
  // 三角形门楼
  {
    name: "Building_Gate",
    x: -4,
    z: 22,
    h: 6,
    footprint: [
      [-5, -4],
      [5, -4],
      [0, 4],
    ],
  },
];

/** 按 name 找楼栋；找不到返回 undefined（调用方负责降级） */
export function findBuilding(
  name: string,
  data: BuildingData[] = BUILDINGS,
): BuildingData | undefined {
  return data.find((building) => building.name === name);
}

/** 底面多边形的世界坐标顶点（局部顶点 + 楼栋原点） */
export function worldFootprint(building: BuildingData): FootprintPoint[] {
  return footprintToWorld(building.footprint, building.x, building.z);
}

/**
 * 找出底面**真正相交**的楼栋对。
 *
 * 用的是多边形相交判定（凹多边形先三角化再逐对做 SAT），不是包围盒：
 *   - 邻居正好落在 L 形的凹口里 → 不报（包围盒判定会误报）
 *   - 斜放的多边形相互穿过 → 报（包围盒判定可能漏报）
 * 恰好共边/共点算相邻，不算相交。
 */
export function findOverlappingBuildings(
  data: BuildingData[] = BUILDINGS,
): Array<[string, string]> {
  const overlaps: Array<[string, string]> = [];

  for (let i = 0; i < data.length; i += 1) {
    for (let j = i + 1; j < data.length; j += 1) {
      const a = data[i];
      const b = data[j];
      if (!a || !b) {
        continue;
      }

      if (polygonsOverlap(worldFootprint(a), worldFootprint(b))) {
        overlaps.push([a.name, b.name]);
      }
    }
  }

  return overlaps;
}

/** 找出重复的 name（name 会当 mesh.name 用，重复会让调试与查找失真）*/
export function findDuplicateNames(data: BuildingData[] = BUILDINGS): string[] {
  const seen = new Set<string>();
  const duplicated = new Set<string>();

  for (const building of data) {
    if (seen.has(building.name)) {
      duplicated.add(building.name);
    }
    seen.add(building.name);
  }

  return [...duplicated];
}

/** 找出顶点数不足 3 的楼栋（构不出体）*/
export function findDegenerateFootprints(
  data: BuildingData[] = BUILDINGS,
): string[] {
  return data
    .filter((building) => building.footprint.length < 3)
    .map((building) => building.name);
}
