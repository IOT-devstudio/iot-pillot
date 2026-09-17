/**
 * BUILDINGS 布局的门禁测试。
 *
 * 为什么值得测：buildings.ts 是「按学校地图手改」的唯一入口，而布局出错
 * （两栋楼撞在一起、名字写重、工作室高度和相机平视高度对不上）都不会抛异常，
 * 只在画面上表现为穿模或平视偏掉——最难定位的那类问题。运行时已经会在控制台
 * 警告，这里把它升级成 CI 门禁，改错坐标直接红。
 */
import { describe, expect, it } from "vitest";

import {
  BUILDINGS,
  findBuilding,
  findDuplicateNames,
  findOverlappingBuildings,
} from "./buildings";
import { CONFIG } from "./config";

describe("opener building layout", () => {
  it("has unique building names", () => {
    // name 会直接当 mesh.name 用，重复会让调试与查找失真
    expect(findDuplicateNames()).toEqual([]);
  });

  it("has no overlapping footprints", () => {
    expect(findOverlappingBuildings()).toEqual([]);
  });

  it("actually detects an overlap", () => {
    // 反向用例：没有它，上面那条「返回空数组」既可能是数据干净，
    // 也可能是检查函数本身坏了（空转的测试最危险）
    const overlaps = findOverlappingBuildings([
      { name: "A", x: 0, z: 0, w: 10, h: 5, d: 10 },
      { name: "B", x: 5, z: 0, w: 10, h: 5, d: 10 },
    ]);

    expect(overlaps).toEqual([["A", "B"]]);
  });

  it("does not flag footprints that only touch on one axis", () => {
    const overlaps = findOverlappingBuildings([
      { name: "A", x: 0, z: 0, w: 10, h: 5, d: 10 },
      { name: "B", x: 5, z: 40, w: 10, h: 5, d: 10 },
    ]);

    expect(overlaps).toEqual([]);
  });

  it("contains the studio building that the camera focuses on", () => {
    expect(findBuilding(CONFIG.studioBuildingName)).toBeDefined();
  });

  it("aims the camera at the studio mid-height so it ends up level with it", () => {
    const studio = findBuilding(CONFIG.studioBuildingName);
    expect(studio).toBeDefined();

    // 阶段 2 的落点要与工作室平视，所以注视点高度必须 ≈ 楼栋中部高度。
    // 容差与运行时警告保持一致（> 3 就提示）
    expect(
      Math.abs(CONFIG.cameraStudioTarget.y - (studio?.h ?? 0) / 2),
    ).toBeLessThanOrEqual(3);
  });

  it("keeps every building above the ground", () => {
    expect(BUILDINGS.filter((building) => building.h <= 0)).toEqual([]);
  });
});
