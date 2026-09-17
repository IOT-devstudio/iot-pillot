/**
 * 视口变换与命中测试的测试。
 *
 * 这些公式决定拖拽手感：屏幕/世界换换算错一点，就会表现为"光标和楼栋对不上"、
 * "缩放后拖拽速度不对"。而鼠标交互本身在无浏览器环境下测不了，
 * 所以正确性必须由这一层保证。
 */
import { describe, expect, it } from "vitest";

import { rectFootprint } from "../opener/footprint";
import { fromBuildingData, type BuildingDraft } from "./draft";
import {
  fitViewport,
  hitTestBuilding,
  hitTestVertex,
  screenToWorld,
  worldToScreen,
  zoomAt,
  type Viewport,
} from "./viewport";

function twoBuildings(): BuildingDraft[] {
  return fromBuildingData([
    { name: "A", x: 0, z: 0, h: 10, footprint: rectFootprint(20, 20) },
    { name: "B", x: 60, z: 0, h: 10, footprint: rectFootprint(20, 20) },
  ]);
}

describe("viewport transforms", () => {
  const viewport: Viewport = { scale: 2, originX: 100, originY: 50 };

  it("maps world coordinates to screen coordinates", () => {
    expect(worldToScreen(viewport, 0, 0)).toEqual([100, 50]);
    expect(worldToScreen(viewport, 10, -5)).toEqual([120, 40]);
  });

  it("round-trips through screen coordinates", () => {
    const [sx, sy] = worldToScreen(viewport, 12.5, -7.25);
    const [wx, wz] = screenToWorld(viewport, sx, sy);

    expect(wx).toBeCloseTo(12.5);
    expect(wz).toBeCloseTo(-7.25);
  });
});

describe("fitViewport", () => {
  it("centres the content inside the canvas", () => {
    const drafts = twoBuildings();
    const vp = fitViewport(drafts, 800, 400, 40);

    // A 与 B 的底面横跨 x ∈ [-10, 70]，中心在 x = 30
    const [centerScreenX] = worldToScreen(vp, 30, 0);
    expect(centerScreenX).toBeCloseTo(400, 5);
  });

  it("keeps the whole content inside the padded area", () => {
    const drafts = twoBuildings();
    const vp = fitViewport(drafts, 800, 400, 40);

    const [left] = worldToScreen(vp, -10, 0);
    const [right] = worldToScreen(vp, 70, 0);

    expect(left).toBeGreaterThanOrEqual(40 - 1e-6);
    expect(right).toBeLessThanOrEqual(800 - 40 + 1e-6);
  });

  it("falls back to a usable viewport when there is nothing to show", () => {
    const vp = fitViewport([], 600, 300);

    expect(vp.scale).toBeGreaterThan(0);
    expect(worldToScreen(vp, 0, 0)).toEqual([300, 150]);
  });
});

describe("zoomAt", () => {
  it("keeps the world point under the cursor fixed", () => {
    const vp: Viewport = { scale: 1, originX: 100, originY: 100 };
    const [beforeX, beforeZ] = screenToWorld(vp, 300, 220);

    const zoomed = zoomAt(vp, 300, 220, 1.5);
    const [afterX, afterZ] = screenToWorld(zoomed, 300, 220);

    expect(afterX).toBeCloseTo(beforeX);
    expect(afterZ).toBeCloseTo(beforeZ);
    expect(zoomed.scale).toBeCloseTo(1.5);
  });

  it("clamps the scale to the allowed range", () => {
    const vp: Viewport = { scale: 1, originX: 0, originY: 0 };

    expect(zoomAt(vp, 0, 0, 1000, 0.05, 20).scale).toBe(20);
    expect(zoomAt(vp, 0, 0, 0.0001, 0.05, 20).scale).toBe(0.05);
  });
});

describe("hit testing", () => {
  const viewport: Viewport = { scale: 1, originX: 200, originY: 200 };

  it("finds the building under the cursor", () => {
    const drafts = twoBuildings();

    // A 覆盖 x ∈ [-10,10] → 世界 (0,0) 在屏幕 (200,200)
    expect(hitTestBuilding(viewport, drafts, 200, 200)).toBe(drafts[0]?.id);
    // B 覆盖 x ∈ [50,70] → 世界 (60,0) 在屏幕 (260,200)
    expect(hitTestBuilding(viewport, drafts, 260, 200)).toBe(drafts[1]?.id);
  });

  it("returns null on empty ground", () => {
    expect(hitTestBuilding(viewport, twoBuildings(), 400, 400)).toBeNull();
  });

  it("prefers the last drawn building when they overlap", () => {
    const drafts = fromBuildingData([
      { name: "Under", x: 0, z: 0, h: 10, footprint: rectFootprint(40, 40) },
      { name: "Over", x: 0, z: 0, h: 10, footprint: rectFootprint(20, 20) },
    ]);

    // 后画的在上层，点选结果应与视觉层叠一致
    expect(hitTestBuilding(viewport, drafts, 200, 200)).toBe(drafts[1]?.id);
  });

  it("finds a vertex within tolerance only", () => {
    const drafts = fromBuildingData([
      { name: "A", x: 0, z: 0, h: 10, footprint: rectFootprint(20, 20) },
    ]);
    const draft = drafts[0]!;
    // 顶点 0 在局部 (-10,-10) → 世界 (-10,-10) → 屏幕 (190,190)
    expect(hitTestVertex(viewport, draft, 193, 194, 10)).toBe(0);
    // 离所有顶点都远
    expect(hitTestVertex(viewport, draft, 200, 200, 5)).toBeNull();
  });

  it("returns null for vertex hits on a building with no vertices", () => {
    const draft: BuildingDraft = {
      id: "x",
      name: "X",
      x: 0,
      z: 0,
      h: 1,
      footprint: [],
    };

    expect(hitTestVertex(viewport, draft, 0, 0, 10)).toBeNull();
  });
});
