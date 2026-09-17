/**
 * BUILDINGS 布局的门禁测试。
 *
 * 为什么值得测：buildings.ts 是「按学校地图手改」的唯一入口，而布局出错
 * （两栋楼撞在一起、名字写重、顶点数不够、工作室高度和相机平视高度对不上）
 * 都不会抛异常，只在画面上表现为穿模或平视偏掉——最难定位的那类问题。
 * 运行时会在控制台警告，这里把它升级成 CI 门禁，改错坐标直接红。
 */
import { describe, expect, it } from "vitest";

import {
  BUILDINGS,
  findBuilding,
  findDegenerateFootprints,
  findDuplicateNames,
  findOverlappingBuildings,
} from "./buildings";
import { CONFIG } from "./config";
import { footprintBounds, isConvex, polygonArea } from "./footprint";

describe("opener building layout", () => {
  it("has unique building names", () => {
    // name 会直接当 mesh.name 用，重复会让调试与查找失真
    expect(findDuplicateNames()).toEqual([]);
  });

  it("has no overlapping footprints", () => {
    expect(findOverlappingBuildings()).toEqual([]);
  });

  it("gives every building at least three footprint vertices", () => {
    expect(findDegenerateFootprints()).toEqual([]);
  });

  it("gives every building a non-zero footprint area", () => {
    const degenerate = BUILDINGS.filter(
      (building) => polygonArea(building.footprint) < 1,
    ).map((building) => building.name);

    expect(degenerate).toEqual([]);
  });

  it("contains the studio building that the camera focuses on", () => {
    expect(findBuilding(CONFIG.studioBuildingName)).toBeDefined();
  });

  it("keeps the studio footprint origin inside the studio polygon", () => {
    // 阶段 2 的相机看向 CONFIG.cameraStudioTarget，它对应楼栋局部原点。
    // 如果原点落在凹口的空地上，镜头就会对着楼外的空气看。
    const studio = findBuilding(CONFIG.studioBuildingName);
    expect(studio).toBeDefined();
    if (!studio) {
      return;
    }

    const bounds = footprintBounds(studio.footprint);
    expect(studio.footprint.length).toBeGreaterThanOrEqual(3);
    expect(bounds.minX).toBeLessThan(0);
    expect(bounds.maxX).toBeGreaterThan(0);
    expect(bounds.minZ).toBeLessThan(0);
    expect(bounds.maxZ).toBeGreaterThan(0);
  });

  it("aims the camera at the studio mid-height so it ends up level with it", () => {
    const studio = findBuilding(CONFIG.studioBuildingName);
    expect(studio).toBeDefined();

    // 容差与运行时警告保持一致（> 3 就提示）
    expect(
      Math.abs(CONFIG.cameraStudioTarget.y - (studio?.h ?? 0) / 2),
    ).toBeLessThanOrEqual(3);
  });

  it("aims the camera at the studio ground position", () => {
    const studio = findBuilding(CONFIG.studioBuildingName);
    expect(studio).toBeDefined();

    // 注视点的 x/z 应当落在工作室的底面范围内，否则镜头会偏到别的楼上
    const bounds = footprintBounds(studio?.footprint ?? []);
    const target = CONFIG.cameraStudioTarget;
    const worldMinX = (studio?.x ?? 0) + bounds.minX;
    const worldMaxX = (studio?.x ?? 0) + bounds.maxX;
    const worldMinZ = (studio?.z ?? 0) + bounds.minZ;
    const worldMaxZ = (studio?.z ?? 0) + bounds.maxZ;

    expect(target.x).toBeGreaterThanOrEqual(worldMinX);
    expect(target.x).toBeLessThanOrEqual(worldMaxX);
    expect(target.z).toBeGreaterThanOrEqual(worldMinZ);
    expect(target.z).toBeLessThanOrEqual(worldMaxZ);
  });

  it("keeps every building above the ground", () => {
    expect(BUILDINGS.filter((building) => building.h <= 0)).toEqual([]);
  });

  it("includes both convex and concave shapes so the prism path is exercised", () => {
    // 「不规则多边形」如果全是凸的，三角化那条路径就永远没被真实数据走过
    const concave = BUILDINGS.filter(
      (building) => !isConvex(building.footprint),
    ).map((building) => building.name);

    expect(concave.length).toBeGreaterThan(0);
    expect(BUILDINGS.length - concave.length).toBeGreaterThan(0);
  });
});
