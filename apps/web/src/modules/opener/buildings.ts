/**
 * 楼栋布局数据 —— 唯一需要按学校地图手改的地方。
 *
 * 字段含义与单位：
 *   name  楼栋标识。必须唯一；等于 CONFIG.studioBuildingName 的那一栋会被当成
 *         工作室（studioColor 上色 + EdgesGeometry 描边）并成为阶段 2 的对焦点。
 *   x     楼栋**底面中心**的地面 X 坐标（米）。正方向朝屏幕右侧。
 *   z     楼栋**底面中心**的地面 Z 坐标（米）。正方向朝屏幕外（近处/南）。
 *   w     宽度，沿 X 轴方向的尺寸（米）。
 *   h     高度，沿 Y 轴方向的尺寸（米）。同时决定升起动画时长：越高升得越慢。
 *   d     进深，沿 Z 轴方向的尺寸（米）。
 *
 * 怎么换成真实学校地图坐标：
 *   1. 先定一个原点（建议放校园中心或主楼），量出每栋楼底面中心相对原点的
 *      东西向偏移 → 填 x，南北向偏移 → 填 z。
 *   2. 比例保持一致即可：用地图测距工具量出实际米数填 w/d，高度按
 *      「楼层数 × 3.2 米」估算填 h。三者不必与真实建筑完全一致，
 *      白模只要相对关系对就行。
 *   3. 每栋楼都是矩形（BoxGeometry）。想拼 L 形 / U 形就写成多栋相邻的小 box
 *      （例如 Building_Dorm_A / Building_Dorm_B），name 保持唯一即可。
 *   4. 加完新楼栋后确认与已有楼栋的 x/z/w/d 范围不重叠，否则会穿模；
 *      findOverlappingBuildings() 会在控制台把这些冲突名单打出来。
 *
 * 下面 13 条是占位数据，已刻意做成错落有致（高度 6~34 米、进深宽度不一），
 * 可以当作「画面对不对」的基准。
 */

export interface BuildingData {
  name: string;
  x: number;
  z: number;
  w: number;
  h: number;
  d: number;
}

export const BUILDINGS: BuildingData[] = [
  // name                      x    z    w   h   d
  // ↓ 镜头焦点：h=20 对应 CONFIG.cameraStudioTarget.y=10（中部高度）
  { name: "Building_Studio", x: 15, z: 5, w: 22, h: 20, d: 16 },
  { name: "Building_Library", x: -38, z: -22, w: 26, h: 14, d: 20 },
  { name: "Building_Lab_A", x: -6, z: -26, w: 18, h: 22, d: 18 },
  { name: "Building_Lab_B", x: -30, z: 12, w: 16, h: 26, d: 14 },
  { name: "Building_Canteen", x: 44, z: -14, w: 24, h: 9, d: 22 },
  { name: "Building_Dorm_North", x: -46, z: 34, w: 30, h: 18, d: 14 },
  { name: "Building_Dorm_South", x: -8, z: 38, w: 30, h: 18, d: 14 },
  { name: "Building_Gym", x: 34, z: 40, w: 28, h: 12, d: 24 },
  { name: "Building_Admin", x: 2, z: -46, w: 20, h: 28, d: 16 },
  // z 从 16 挪到 12：原来与 Building_Gym 的底面在 z 方向重叠 1 米（会穿模）
  { name: "Building_Auditorium", x: 46, z: 12, w: 26, h: 16, d: 26 },
  { name: "Building_Workshop", x: -20, z: -6, w: 14, h: 8, d: 12 },
  { name: "Building_Tower", x: 24, z: -30, w: 12, h: 34, d: 12 },
  { name: "Building_Gate", x: -2, z: 22, w: 8, h: 6, d: 6 },
];

/** 按 name 找楼栋；找不到返回 undefined（调用方负责降级） */
export function findBuilding(
  name: string,
  data: BuildingData[] = BUILDINGS,
): BuildingData | undefined {
  return data.find((building) => building.name === name);
}

/**
 * 找出底面矩形互相重叠的楼栋对。
 *
 * 手改地图坐标时最容易犯的错就是把两栋楼摆到同一个位置，画面上表现为穿模，
 * 但不会报任何异常、很难定位。启动时调一次，把结果打出来即可。
 * 只做轴对齐矩形相交判断——BoxGeometry 都是轴对齐的，够用。
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

      // 两个半宽之和 > 中心距 → 该轴重叠；两轴都重叠才算真的撞上
      const overlapX = Math.abs(a.x - b.x) < (a.w + b.w) / 2;
      const overlapZ = Math.abs(a.z - b.z) < (a.d + b.d) / 2;
      if (overlapX && overlapZ) {
        overlaps.push([a.name, b.name]);
      }
    }
  }

  return overlaps;
}

/** 找出重复的 name（name 要当 mesh.name 用，重复会让调试与查找失真）*/
export function findDuplicateNames(
  data: BuildingData[] = BUILDINGS,
): string[] {
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
