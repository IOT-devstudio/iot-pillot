/**
 * 多边形几何原语的测试。
 *
 * 重点不是覆盖率，而是锁死两件容易出错的事：
 *   1. 三角化不丢面积（耳切法写错时面积会悄悄不守恒）
 *   2. 相交判定对**凹**多边形和**斜放**多边形都精确
 *      —— 这两类正是旧的轴对齐包围盒（AABB）判定会误报的场景，
 *      下面用「包围盒相交但形体不相交」的用例把差异固定下来。
 */
import { describe, expect, it } from "vitest";

import {
  type FootprintPoint,
  footprintBounds,
  footprintToWorld,
  isConvex,
  polygonArea,
  polygonCentroid,
  polygonsOverlap,
  rectFootprint,
  regularFootprint,
  triangulate,
} from "./footprint";

/** 一个 L 形底面：整条底边 + 左侧竖臂，凹口在右上（x∈[1,11], z∈[0,8]） */
const L_SHAPE: FootprintPoint[] = [
  [-11, -8],
  [11, -8],
  [11, 0],
  [1, 0],
  [1, 8],
  [-11, 8],
];

/** L 形面积：底带 22×8 + 左臂 12×8 */
const L_AREA = 176 + 96;

/** 面积的相对误差容忍度（耳切法会引入浮点误差） */
function expectAreaClose(actual: number, expected: number): void {
  expect(Math.abs(actual - expected)).toBeLessThan(1e-6);
}

function sumTriangleArea(triangles: ReturnType<typeof triangulate>): number {
  return triangles.reduce(
    (total, [a, b, c]) =>
      total +
      Math.abs(
        (b[0] - a[0]) * (c[1] - a[1]) - (c[0] - a[0]) * (b[1] - a[1]),
      ) /
        2,
    0,
  );
}

describe("footprint construction", () => {
  it("builds a centred rectangle footprint", () => {
    const rect = rectFootprint(10, 4);

    expect(rect).toHaveLength(4);
    expect(footprintBounds(rect)).toEqual({
      minX: -5,
      maxX: 5,
      minZ: -2,
      maxZ: 2,
    });
    expectAreaClose(polygonArea(rect), 40);
    expect(polygonCentroid(rect)).toEqual([0, 0]);
  });

  it("builds a regular hexagon with the expected area", () => {
    const hexagon = regularFootprint(6, 10);

    expect(hexagon).toHaveLength(6);
    // 正六边形面积 = (3√3/2) r²
    expect(Math.abs(polygonArea(hexagon) - (3 * Math.sqrt(3) * 100) / 2)).toBeLessThan(0.01);
    expect(isConvex(hexagon)).toBe(true);
  });
});

describe("polygon metrics", () => {
  it("ignores winding when measuring area", () => {
    const rect = rectFootprint(6, 3);

    expectAreaClose(polygonArea(rect), polygonArea([...rect].reverse()));
  });

  it("detects convexity", () => {
    expect(isConvex(rectFootprint(4, 4))).toBe(true);
    expect(isConvex(regularFootprint(8, 5))).toBe(true);
    expect(isConvex([[0, 0], [4, 0], [0, 4]])).toBe(true);
    // L 形是凹的：这正是必须三角化、不能直接跑 SAT 的原因
    expect(isConvex(L_SHAPE)).toBe(false);
  });
});

describe("triangulate", () => {
  it("splits a square into two triangles without losing area", () => {
    const triangles = triangulate(rectFootprint(2, 2));

    expect(triangles).toHaveLength(2);
    expectAreaClose(sumTriangleArea(triangles), 4);
  });

  it("splits a concave L shape into n-2 triangles without losing area", () => {
    const triangles = triangulate(L_SHAPE);

    // 耳切法对 n 个顶点的简单多边形恒产出 n-2 个三角形
    expect(triangles).toHaveLength(L_SHAPE.length - 2);
    // 面积守恒：写错耳切逻辑时这条会先炸
    expectAreaClose(sumTriangleArea(triangles), L_AREA);
  });

  it("normalises clockwise input instead of failing", () => {
    const clockwise = [...L_SHAPE].reverse();
    const triangles = triangulate(clockwise);

    expect(triangles).toHaveLength(L_SHAPE.length - 2);
    expectAreaClose(sumTriangleArea(triangles), L_AREA);
  });

  it("returns nothing for a degenerate polygon", () => {
    expect(triangulate([[0, 0], [1, 1]])).toEqual([]);
  });
});

describe("polygonsOverlap", () => {
  function worldRect(
    w: number,
    d: number,
    x: number,
    z: number,
  ): FootprintPoint[] {
    return footprintToWorld(rectFootprint(w, d), x, z);
  }

  it("detects genuinely overlapping rectangles", () => {
    expect(
      polygonsOverlap(worldRect(10, 10, 0, 0), worldRect(10, 10, 5, 0)),
    ).toBe(true);
  });

  it("treats identical polygons as overlapping", () => {
    expect(
      polygonsOverlap(worldRect(10, 10, 0, 0), worldRect(10, 10, 0, 0)),
    ).toBe(true);
  });

  it("does not treat a shared edge as an overlap", () => {
    // 共边 = 相邻，不是穿模；否则一整排贴着的楼会被全部报成冲突
    expect(
      polygonsOverlap(worldRect(10, 10, 0, 0), worldRect(10, 10, 10, 0)),
    ).toBe(false);
  });

  it("does not treat a shared corner as an overlap", () => {
    expect(
      polygonsOverlap(worldRect(10, 10, 0, 0), worldRect(10, 10, 10, 10)),
    ).toBe(false);
  });

  it("does not report distant polygons", () => {
    expect(
      polygonsOverlap(worldRect(10, 10, 0, 0), worldRect(10, 10, 100, 100)),
    ).toBe(false);
  });

  it("ignores a shape sitting inside a concave notch", () => {
    // 关键用例：L 形的凹口（x∈[1,11], z∈[0,8]）是空的，
    // 落在这里的楼不该被报成重叠 —— 旧的 AABB 判定会在这里误报。
    const inNotch = worldRect(8, 6, 6, 4);

    // 先证明这个场景确实能让 AABB 判定失效：包围盒是相交的
    const lBounds = footprintBounds(L_SHAPE);
    const notchBounds = footprintBounds(inNotch);
    expect(
      Math.min(lBounds.maxX, notchBounds.maxX) -
        Math.max(lBounds.minX, notchBounds.minX),
    ).toBeGreaterThan(0);
    expect(
      Math.min(lBounds.maxZ, notchBounds.maxZ) -
        Math.max(lBounds.minZ, notchBounds.minZ),
    ).toBeGreaterThan(0);

    // 但形体本身不相交
    expect(polygonsOverlap(L_SHAPE, inNotch)).toBe(false);
  });

  it("still detects a shape on the solid part of the same L", () => {
    // 上一条的对照组：同样的尺寸挪到 L 的实体（底带）上，必须报重叠。
    // 没有这条，上面那条「返回 false」可能只是因为判定整个坏掉了。
    const onSolid = worldRect(8, 6, 6, -4);

    expect(polygonsOverlap(L_SHAPE, onSolid)).toBe(true);
  });

  it("ignores a rotated diamond whose bounding box overlaps", () => {
    // 再一个 AABB 会误报的场景：斜放的正方形。中心 (5.5,5.5)、外接半径 2 的
    // 菱形满足 |x-5.5|+|z-5.5| <= 2，因此 x+z >= 9；
    // 而矩形最大只到 x+z = 8，两者形体不相交，但包围盒是相交的。
    const rect = worldRect(4, 4, 2, 2);
    const diamond = footprintToWorld(regularFootprint(4, 2), 5.5, 5.5);

    const rectBounds = footprintBounds(rect);
    const diamondBounds = footprintBounds(diamond);
    expect(
      Math.min(rectBounds.maxX, diamondBounds.maxX) -
        Math.max(rectBounds.minX, diamondBounds.minX),
    ).toBeGreaterThan(0);
    expect(
      Math.min(rectBounds.maxZ, diamondBounds.maxZ) -
        Math.max(rectBounds.minZ, diamondBounds.minZ),
    ).toBeGreaterThan(0);

    expect(polygonsOverlap(rect, diamond)).toBe(false);

    // 对照组：把菱形挪进矩形内部，必须报重叠
    expect(
      polygonsOverlap(rect, footprintToWorld(regularFootprint(4, 2), 2, 2)),
    ).toBe(true);
  });
});
