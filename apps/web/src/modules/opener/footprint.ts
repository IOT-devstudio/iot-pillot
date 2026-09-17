/**
 * 楼栋底面多边形的几何原语。
 *
 * 纯函数、零依赖（**不引 three**）：buildings.ts 的数据校验、scene/ 的挤出建模、
 * 以及可视化编辑器的命中测试都复用这里，保证三处用的是同一套判定。
 *
 * 坐标约定：底面顶点用 [x, z] 表示，是**相对楼栋原点的局部坐标**（米）。
 * 与 Three.js 的世界坐标一致：+x 向右、+z 朝向观察者（南）。
 * 多边形按逆时针（CCW）为正；本模块的三角化会先归一化绕向，输入顺序无所谓。
 */

/** 底面多边形的一个顶点：[x, z]（米，局部坐标） */
export type FootprintPoint = [number, number];

/** 三角化后的一个三角形，顶点顺序保持原多边形的绕向 */
export type Triangle = [FootprintPoint, FootprintPoint, FootprintPoint];

export interface Bounds {
  minX: number;
  maxX: number;
  minZ: number;
  maxZ: number;
}

/**
 * 判定「重叠」时容忍的浮点误差（米）。
 * 取正数是为了让「恰好共边/共点」被判为不重叠——所以用 `<= EPSILON` 判分离。
 */
const EPSILON = 1e-9;

/** 保留 3 位小数：生成代码时避免 0.30000000000000004 这种噪音 */
function round(value: number): number {
  return Math.round(value * 1000) / 1000;
}

/* ------------------------------------------------------------------ *
 * 构造
 * ------------------------------------------------------------------ */

/** 轴对齐矩形底面：宽 w（x 方向）× 深 d（z 方向），中心在局部原点 */
export function rectFootprint(w: number, d: number): FootprintPoint[] {
  const hw = w / 2;
  const hd = d / 2;
  return [
    [-hw, -hd],
    [hw, -hd],
    [hw, hd],
    [-hw, hd],
  ];
}

/**
 * 正 N 边形底面。
 *
 * @param sides       边数（≥3）
 * @param radius      外接圆半径（米）
 * @param rotationDeg 整体旋转角（度），用来让正多边形的边不总是平行于坐标轴
 */
export function regularFootprint(
  sides: number,
  radius: number,
  rotationDeg = 0,
): FootprintPoint[] {
  const count = Math.max(3, Math.floor(sides));
  const offset = (rotationDeg * Math.PI) / 180;
  const points: FootprintPoint[] = [];

  for (let i = 0; i < count; i += 1) {
    const angle = offset + (i * 2 * Math.PI) / count;
    points.push([
      round(radius * Math.cos(angle)),
      round(radius * Math.sin(angle)),
    ]);
  }

  return points;
}

/* ------------------------------------------------------------------ *
 * 度量
 * ------------------------------------------------------------------ */

/** 有向面积的两倍。>0 为逆时针（CCW），<0 为顺时针（CW） */
function signedArea2(polygon: FootprintPoint[]): number {
  let sum = 0;
  for (let i = 0; i < polygon.length; i += 1) {
    const a = polygon[i];
    const b = polygon[(i + 1) % polygon.length];
    if (!a || !b) {
      continue;
    }
    sum += a[0] * b[1] - b[0] * a[1];
  }
  return sum;
}

/** 多边形面积（平方米，恒为非负） */
export function polygonArea(polygon: FootprintPoint[]): number {
  return Math.abs(signedArea2(polygon)) / 2;
}

/** 面积形心（局部坐标）；退化多边形返回 [0, 0] */
export function polygonCentroid(polygon: FootprintPoint[]): FootprintPoint {
  const area2 = signedArea2(polygon);
  if (Math.abs(area2) < EPSILON) {
    return [0, 0];
  }

  let cx = 0;
  let cz = 0;
  for (let i = 0; i < polygon.length; i += 1) {
    const a = polygon[i];
    const b = polygon[(i + 1) % polygon.length];
    if (!a || !b) {
      continue;
    }
    const factor = a[0] * b[1] - b[0] * a[1];
    cx += (a[0] + b[0]) * factor;
    cz += (a[1] + b[1]) * factor;
  }

  return [round(cx / (3 * area2)), round(cz / (3 * area2))];
}

/** 局部坐标下的包围盒 */
export function footprintBounds(polygon: FootprintPoint[]): Bounds {
  if (polygon.length === 0) {
    return { minX: 0, maxX: 0, minZ: 0, maxZ: 0 };
  }

  let minX = Infinity;
  let maxX = -Infinity;
  let minZ = Infinity;
  let maxZ = -Infinity;

  for (const [x, z] of polygon) {
    minX = Math.min(minX, x);
    maxX = Math.max(maxX, x);
    minZ = Math.min(minZ, z);
    maxZ = Math.max(maxZ, z);
  }

  return { minX, maxX, minZ, maxZ };
}

/** 把局部底面平移到世界坐标（楼栋原点 + 局部顶点） */
export function footprintToWorld(
  polygon: FootprintPoint[],
  x: number,
  z: number,
): FootprintPoint[] {
  return polygon.map(([px, pz]) => [px + x, pz + z] as FootprintPoint);
}

/**
 * 是否为凸多边形（允许共线相邻点）。
 * 凸多边形可以直接用 SAT 判定相交；凹多边形必须先三角化，见 polygonsOverlap。
 */
export function isConvex(polygon: FootprintPoint[]): boolean {
  if (polygon.length < 3) {
    return false;
  }

  let sign = 0;
  for (let i = 0; i < polygon.length; i += 1) {
    const a = polygon[i];
    const b = polygon[(i + 1) % polygon.length];
    const c = polygon[(i + 2) % polygon.length];
    if (!a || !b || !c) {
      continue;
    }
    const turn = (b[0] - a[0]) * (c[1] - b[1]) - (b[1] - a[1]) * (c[0] - b[0]);
    if (Math.abs(turn) < EPSILON) {
      continue; // 共线不影响凸性
    }
    const current = turn > 0 ? 1 : -1;
    if (sign === 0) {
      sign = current;
    } else if (sign !== current) {
      return false;
    }
  }

  return true;
}

/* ------------------------------------------------------------------ *
 * 三角化（耳切法）
 * ------------------------------------------------------------------ */

/** 归一化：去掉与首点重复的收尾点，并统一成逆时针 */
function normalize(polygon: FootprintPoint[]): FootprintPoint[] {
  const points = polygon.slice();
  const first = points[0];
  const last = points[points.length - 1];
  if (
    points.length > 1 &&
    first &&
    last &&
    Math.abs(first[0] - last[0]) < EPSILON &&
    Math.abs(first[1] - last[1]) < EPSILON
  ) {
    points.pop();
  }

  return signedArea2(points) < 0 ? points.reverse() : points;
}

function cross(o: FootprintPoint, a: FootprintPoint, b: FootprintPoint): number {
  return (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0]);
}

function pointInTriangle(
  p: FootprintPoint,
  a: FootprintPoint,
  b: FootprintPoint,
  c: FootprintPoint,
): boolean {
  const d1 = cross(a, b, p);
  const d2 = cross(b, c, p);
  const d3 = cross(c, a, p);
  const hasNegative = d1 < 0 || d2 < 0 || d3 < 0;
  const hasPositive = d1 > 0 || d2 > 0 || d3 > 0;
  // 含边界：顶点恰好落在边上时也算「落在三角形内」，因此会阻止该耳被切掉。
  // 这偏保守（耳更少但每个耳都一定合法），代价是退化输入更容易走兜底分支。
  return !(hasNegative && hasPositive);
}

/**
 * 耳切法三角化任意简单多边形（含凹多边形）。
 *
 * 为什么要三角化：SAT（分离轴）只对**凸**多边形精确。凹多边形若不拆直接跑 SAT，
 * 会把凹口处的空隙误判成重叠。拆成三角形后逐对做 SAT，结果对任意简单多边形都是精确的。
 *
 * 输入自交/退化时可能找不到合法的耳，此时退化为从首点扇形三角化——
 * 结果偏保守（可能误报重叠，但不会漏报），调用方按「警告」使用是安全的。
 */
export function triangulate(polygon: FootprintPoint[]): Triangle[] {
  const points = normalize(polygon);
  if (points.length < 3) {
    return [];
  }

  const triangles: Triangle[] = [];
  const remaining = points.map((_, index) => index);

  // 每次至少消掉一个顶点，循环上限取 n² 足够收敛
  const maxIterations = points.length * points.length;
  for (let iteration = 0; iteration < maxIterations; iteration += 1) {
    if (remaining.length <= 3) {
      break;
    }

    let earCut = false;
    for (let i = 0; i < remaining.length; i += 1) {
      const prevIndex = remaining[(i - 1 + remaining.length) % remaining.length];
      const currIndex = remaining[i];
      const nextIndex = remaining[(i + 1) % remaining.length];
      const a = points[prevIndex as number];
      const b = points[currIndex as number];
      const c = points[nextIndex as number];
      if (!a || !b || !c) {
        continue;
      }

      // 逆时针多边形里，凸点满足 cross(a, b, c) > 0
      if (cross(a, b, c) <= EPSILON) {
        continue;
      }

      const containsOther = remaining.some((index) => {
        if (index === prevIndex || index === currIndex || index === nextIndex) {
          return false;
        }
        const p = points[index as number];
        return p ? pointInTriangle(p, a, b, c) : false;
      });
      if (containsOther) {
        continue;
      }

      triangles.push([a, b, c]);
      remaining.splice(i, 1);
      earCut = true;
      break;
    }

    if (!earCut) {
      break;
    }
  }

  if (remaining.length === 3) {
    const a = points[remaining[0] as number];
    const b = points[remaining[1] as number];
    const c = points[remaining[2] as number];
    if (a && b && c) {
      triangles.push([a, b, c]);
    }
    return triangles;
  }

  // 兜底：自交/退化输入下退化为扇形三角化
  const origin = points[remaining[0] as number];
  if (origin) {
    for (let i = 1; i < remaining.length - 1; i += 1) {
      const b = points[remaining[i] as number];
      const c = points[remaining[i + 1] as number];
      if (b && c) {
        triangles.push([origin, b, c]);
      }
    }
  }

  return triangles;
}

/* ------------------------------------------------------------------ *
 * 相交判定
 * ------------------------------------------------------------------ */

/** 凸多边形的边法线（SAT 的候选分离轴） */
function edgeNormals(polygon: FootprintPoint[]): FootprintPoint[] {
  const normals: FootprintPoint[] = [];

  for (let i = 0; i < polygon.length; i += 1) {
    const a = polygon[i];
    const b = polygon[(i + 1) % polygon.length];
    if (!a || !b) {
      continue;
    }
    const edgeX = b[0] - a[0];
    const edgeZ = b[1] - a[1];
    const length = Math.hypot(edgeX, edgeZ);
    if (length < EPSILON) {
      continue;
    }
    // 边的法线方向（任意取一侧即可，SAT 只看投影长度）
    normals.push([-edgeZ / length, edgeX / length]);
  }

  return normals;
}

function project(
  polygon: FootprintPoint[],
  axis: FootprintPoint,
): [number, number] {
  let min = Infinity;
  let max = -Infinity;

  for (const [x, z] of polygon) {
    const value = x * axis[0] + z * axis[1];
    min = Math.min(min, value);
    max = Math.max(max, value);
  }

  return [min, max];
}

function boundsOverlap(a: Bounds, b: Bounds): boolean {
  return (
    Math.min(a.maxX, b.maxX) - Math.max(a.minX, b.minX) > EPSILON &&
    Math.min(a.maxZ, b.maxZ) - Math.max(a.minZ, b.minZ) > EPSILON
  );
}

/** 两个**凸**多边形的 SAT 相交判定（恰好共边/共点算不相交） */
function convexOverlap(a: FootprintPoint[], b: FootprintPoint[]): boolean {
  for (const axis of [...edgeNormals(a), ...edgeNormals(b)]) {
    const [minA, maxA] = project(a, axis);
    const [minB, maxB] = project(b, axis);
    // 存在一个轴上的投影不重叠（或仅相切）→ 分离
    if (Math.min(maxA, maxB) - Math.max(minA, minB) <= EPSILON) {
      return false;
    }
  }

  return true;
}

/**
 * 两个任意简单多边形是否**真的**重叠（面积相交）。
 *
 * 凹多边形先用三角化拆成凸片，再逐对做 SAT：三角片恰好铺满原多边形，
 * 所以「某对三角片相交」等价于「原多边形相交」，对凸/凹都精确。
 *
 * 与旧的轴对齐包围盒（AABB）判定的区别：L 形楼的凹口是空的，AABB 会把
 * 「邻居正好落在凹口里」误报成穿模，也会漏报斜放多边形。这里两者都不会。
 */
export function polygonsOverlap(
  a: FootprintPoint[],
  b: FootprintPoint[],
): boolean {
  // 先用包围盒快速排除，绝大多数不相邻的组合走到这里就返回了
  if (!boundsOverlap(footprintBounds(a), footprintBounds(b))) {
    return false;
  }

  const trianglesA = triangulate(a);
  const trianglesB = triangulate(b);
  if (trianglesA.length === 0 || trianglesB.length === 0) {
    return false;
  }

  for (const triangleA of trianglesA) {
    for (const triangleB of trianglesB) {
      if (convexOverlap(triangleA, triangleB)) {
        return true;
      }
    }
  }

  return false;
}
