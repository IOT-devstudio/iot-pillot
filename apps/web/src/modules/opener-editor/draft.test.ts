/**
 * 编辑器草稿操作与代码生成的测试。
 *
 * 重点：
 *  1. 编辑操作是纯函数，逐条锁住语义（尤其是"删到少于 3 个顶点要拒绝"这类边界）
 *  2. 生成的代码必须**真能被解析并且数值无损**——所以这里把生成物当 JS 求值
 *     再和草稿比对，而不是只做字符串包含断言。字符串断言挡不住
 *     "少了个逗号"或"数字被 round 成另一个值"。
 */
import { describe, expect, it } from "vitest";

import { BUILDINGS } from "../opener/buildings";
import { rectFootprint } from "../opener/footprint";
import {
  addBuilding,
  addVertexOnEdge,
  createBuilding,
  deleteBuilding,
  deleteVertex,
  footprintSpan,
  fromBuildingData,
  isOriginInsideFootprint,
  moveBuilding,
  moveVertex,
  nextBuildingName,
  renameBuilding,
  setStudioBuilding,
  toBuildingData,
  validateDraft,
} from "./draft";
import {
  deriveCameraSnippet,
  generateBuildingsCode,
  generateCameraCode,
  generateCode,
} from "./codegen";

/** 把生成的 BUILDINGS 片段当 JS 求值（去掉 TS 注解与 export） */
function evaluateBuildingsCode(code: string): unknown {
  const expression = code
    .replace("export const BUILDINGS: BuildingData[] =", "return")
    .replace(/;\s*$/, "");
  // eslint-disable-next-line no-new-func
  return new Function(expression)();
}

describe("draft operations", () => {
  it("round-trips the current BUILDINGS without dropping data", () => {
    const drafts = fromBuildingData(BUILDINGS);

    expect(drafts).toHaveLength(BUILDINGS.length);
    expect(toBuildingData(drafts)).toEqual(BUILDINGS);
  });

  it("gives every draft a distinct id so renaming cannot break selection", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const ids = new Set(drafts.map((draft) => draft.id));

    expect(ids.size).toBe(drafts.length);
  });

  it("moves a building without touching the others", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const target = drafts[0];
    const other = drafts[1];
    expect(target && other).toBeTruthy();

    const moved = moveBuilding(drafts, target!.id, 5, -3);

    expect(moved[0]?.x).toBeCloseTo(target!.x + 5);
    expect(moved[0]?.z).toBeCloseTo(target!.z - 3);
    // 其他楼栋应当原样返回（含引用，说明没有整表重建）
    expect(moved[1]).toBe(other);
  });

  it("moves only the addressed vertex", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const target = drafts[0]!;
    const before = target.footprint.map(([x, z]) => [x, z]);

    const moved = moveVertex(drafts, target.id, 1, 99, -99);
    const after = moved[0]!.footprint;

    expect(after[1]).toEqual([99, -99]);
    expect(after.filter((_, i) => i !== 1)).toEqual(
      before.filter((_, i) => i !== 1),
    );
  });

  it("inserts a new vertex at the midpoint of the chosen edge", () => {
    const drafts = fromBuildingData([
      { name: "R", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
    ]);
    const target = drafts[0]!;
    // 边 0 连接 (-5,-5) 与 (5,-5)，中点应为 (0,-5)
    const withVertex = addVertexOnEdge(drafts, target.id, 0);

    expect(withVertex[0]?.footprint).toHaveLength(5);
    expect(withVertex[0]?.footprint[1]).toEqual([0, -5]);
  });

  it("refuses to delete a vertex below three points", () => {
    const triangle = fromBuildingData([
      {
        name: "T",
        x: 0,
        z: 0,
        h: 5,
        footprint: [
          [-5, -5],
          [5, -5],
          [0, 5],
        ],
      },
    ]);

    // 三角形再删一个顶点就构不出体，必须原样返回
    expect(deleteVertex(triangle, triangle[0]!.id, 0)).toEqual(triangle);
  });

  it("deletes a vertex when enough remain", () => {
    const drafts = fromBuildingData([
      { name: "R", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
    ]);

    const result = deleteVertex(drafts, drafts[0]!.id, 0);

    expect(result[0]?.footprint).toHaveLength(3);
  });

  it("renames and trims a building", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const renamed = renameBuilding(drafts, drafts[0]!.id, "  Building_Renamed  ");

    expect(renamed[0]?.name).toBe("Building_Renamed");
  });

  it("moves the studio flag without creating duplicate names", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const studio = drafts.find((draft) => draft.name === "Building_Studio");
    const other = drafts.find((draft) => draft.name === "Building_Library");
    expect(studio && other).toBeTruthy();

    const result = setStudioBuilding(drafts, other!.id, "Building_Studio");

    // 新工作室拿到名字，老工作室必须让位
    expect(result.filter((d) => d.name === "Building_Studio")).toHaveLength(1);
    expect(result.find((d) => d.id === other!.id)?.name).toBe("Building_Studio");
    expect(result.find((d) => d.id === studio!.id)?.name).not.toBe(
      "Building_Studio",
    );
  });

  it("adds and deletes buildings", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const added = addBuilding(drafts);

    expect(added).toHaveLength(drafts.length + 1);
    expect(added[added.length - 1]?.name).toBe(nextBuildingName(drafts));

    const removed = deleteBuilding(added, added[added.length - 1]!.id);
    expect(removed).toHaveLength(drafts.length);
  });

  it("creates a building with a unique name", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const fresh = createBuilding(drafts);
    const names = new Set(drafts.map((draft) => draft.name));

    expect(names.has(fresh.name)).toBe(false);
    expect(fresh.footprint.length).toBeGreaterThanOrEqual(3);
  });
});

describe("draft validation", () => {
  it("reports no errors for the current BUILDINGS", () => {
    const issues = validateDraft(fromBuildingData(BUILDINGS));
    const errors = issues.filter((issue) => issue.level === "error");

    expect(errors).toEqual([]);
  });

  it("detects footprints that genuinely intersect", () => {
    const drafts = fromBuildingData([
      { name: "A", x: 0, z: 0, h: 10, footprint: rectFootprint(20, 20) },
      { name: "B", x: 5, z: 0, h: 10, footprint: rectFootprint(20, 20) },
    ]);

    const errors = validateDraft(drafts).filter((issue) => issue.level === "error");

    expect(errors).toHaveLength(1);
    expect(errors[0]?.buildings).toEqual(["A", "B"]);
  });

  it("does not flag a neighbour that only touches an edge", () => {
    const drafts = fromBuildingData([
      { name: "A", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
      { name: "B", x: 10, z: 0, h: 10, footprint: rectFootprint(10, 10) },
    ]);

    expect(validateDraft(drafts).filter((i) => i.level === "error")).toEqual([]);
  });

  it("flags duplicate names", () => {
    const drafts = fromBuildingData([
      { name: "Same", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
      { name: "Same", x: 40, z: 0, h: 10, footprint: rectFootprint(10, 10) },
    ]);

    const errors = validateDraft(drafts).filter((issue) => issue.level === "error");
    expect(errors.some((issue) => issue.message.includes("重名"))).toBe(true);
  });

  it("warns when the studio building is missing", () => {
    const drafts = fromBuildingData([
      { name: "Building_Other", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
    ]);

    const warnings = validateDraft(drafts, "Building_Studio").filter(
      (issue) => issue.level === "warning",
    );

    expect(warnings.some((issue) => issue.message.includes("Building_Studio"))).toBe(true);
  });

  it("warns when the studio origin sits outside its own footprint", () => {
    // L 形：凹口在 x∈[1,11], z∈[0,8]，把原点挪到凹口里
    const drafts = fromBuildingData([
      {
        name: "Building_Studio",
        x: 0,
        z: 0,
        h: 20,
        footprint: [
          [-11, -8],
          [11, -8],
          [11, 0],
          [6, 0],
          [6, 8],
          [-11, 8],
        ],
      },
    ]);

    // 该形状的原点 (0,0) 仍在实体内 → 不应告警
    expect(
      isOriginInsideFootprint(toBuildingData(drafts)[0]!),
    ).toBe(true);

    // 一个明显不含原点的底面必须被判出来
    const away = toBuildingData([
      { id: "x", name: "Away", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
    ])[0]!;
    expect(isOriginInsideFootprint({ ...away, footprint: [[10, 10], [20, 10], [20, 20], [10, 20]] })).toBe(false);
  });

  it("reports the studio span for camera scaling", () => {
    const studio = toBuildingData(fromBuildingData(BUILDINGS)).find(
      (building) => building.name === "Building_Studio",
    );

    expect(studio).toBeDefined();
    expect(footprintSpan(studio!)).toBe(22);
  });
});

describe("code generation", () => {
  it("emits BUILDINGS code that evaluates back to the same data", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const code = generateBuildingsCode(drafts);

    const evaluated = evaluateBuildingsCode(code);

    // 数值无损 + 语法合法（解析失败会直接抛错）
    expect(evaluated).toEqual(toBuildingData(drafts));
  });

  it("emits explicit vertex arrays instead of helper calls", () => {
    const code = generateBuildingsCode(fromBuildingData(BUILDINGS));

    // 生成物要能直接粘贴，不能依赖调用方记得 import 辅助函数
    expect(code).not.toContain("rectFootprint");
    expect(code).not.toContain("regularFootprint");
    expect(code).toContain("footprint: [");
  });

  it("handles an empty draft list", () => {
    expect(generateBuildingsCode([])).toBe(
      "export const BUILDINGS: BuildingData[] = [];",
    );
  });

  it("derives a camera that ends up level with the studio", () => {
    const drafts = fromBuildingData(BUILDINGS);
    const snippet = deriveCameraSnippet(drafts);
    const studio = toBuildingData(drafts).find(
      (building) => building.name === "Building_Studio",
    );

    expect(snippet).not.toBeNull();
    // 平视的充要条件：注视点高度 = 楼栋高度的一半
    expect(snippet!.cameraStudioTarget.y).toBeCloseTo(studio!.h / 2);
    // 机位比中部再抬高 25%，避免贴地平线
    expect(snippet!.cameraStudio.y).toBeCloseTo(studio!.h * 0.75);
  });

  it("scales camera offsets with the studio span", () => {
    const small = deriveCameraSnippet(
      fromBuildingData([
        { name: "Building_Studio", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
      ]),
    );
    const large = deriveCameraSnippet(
      fromBuildingData([
        { name: "Building_Studio", x: 0, z: 0, h: 10, footprint: rectFootprint(40, 40) },
      ]),
    );

    const smallOffset = small!.cameraStudio.x - small!.cameraStudioTarget.x;
    const largeOffset = large!.cameraStudio.x - large!.cameraStudioTarget.x;

    expect(largeOffset).toBeGreaterThan(smallOffset);
  });

  it("explains itself when there is no studio building", () => {
    const code = generateCameraCode(
      fromBuildingData([
        { name: "Building_Other", x: 0, z: 0, h: 10, footprint: rectFootprint(10, 10) },
      ]),
    );

    expect(code).toContain("无法推导相机参数");
  });

  it("generates both sections in the combined output", () => {
    const code = generateCode(fromBuildingData(BUILDINGS));

    expect(code).toContain("BUILDINGS");
    expect(code).toContain("cameraStudio");
    expect(code).toContain("cameraStudioTarget");
  });
});
