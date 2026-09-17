/**
 * 用 ExtrudeGeometry 把底面多边形拉伸成**不规则多边形柱**（白模）。
 *
 * 不使用任何 GLB/GLTF 模型文件：形状完全来自 buildings.ts 的 footprint 顶点数组，
 * 所以 L 形、梯形、六边形、十二边形乃至凹多边形都能直接建出来，不再是长方体。
 */
import * as THREE from "three";

import {
  BUILDINGS,
  findBuilding,
  findDegenerateFootprints,
  findDuplicateNames,
  findOverlappingBuildings,
} from "../buildings";
import { CONFIG } from "../config";
import type { FootprintPoint } from "../footprint";
import { type StudioScene, trackDisposable } from "./types";

/** 描边阈值（度）：过滤掉挤出体顶/底面上由三角化产生的内部边 */
const EDGE_THRESHOLD_DEGREES = 15;

/**
 * 把底面多边形沿 Y 轴挤出成高度 height 的柱体，**底面正好落在 y=0**。
 *
 * 为什么是这两步（顺序不能反、符号不能错）：
 *  1. Shape 画在 XY 平面、沿 +Z 挤出 depth。建点时用 (px, -pz) 而不是 (px, pz)，
 *     是为了抵消第 2 步带来的镜像。
 *  2. rotateX(-90°) 把 +Z 转到 +Y：挤出方向变成竖直向上、y 落在 [0, height]。
 *     该变换把 (x, y, z) 映射为 (x, z, -y)，所以形状里的 -pz 正好还原成世界 z = pz。
 *
 * 结果与原 BoxGeometry 的 `translate(0, h / 2, 0)` 等价：几何原点在底面中心，
 * 于是 `mesh.position.y = 0` 就是「立在地面上」，scale.y 也才能做升起动画。
 */
export function createPrismGeometry(
  footprint: FootprintPoint[],
  height: number,
): THREE.ExtrudeGeometry {
  const shape = new THREE.Shape();

  footprint.forEach(([px, pz], index) => {
    if (index === 0) {
      shape.moveTo(px, -pz);
    } else {
      shape.lineTo(px, -pz);
    }
  });
  shape.closePath();

  const geometry = new THREE.ExtrudeGeometry(shape, {
    depth: height,
    bevelEnabled: false,
    // 底面是折线、没有曲线段，1 足以避免无谓细分
    curveSegments: 1,
  });
  geometry.rotateX(-Math.PI / 2);

  return geometry;
}

/**
 * 按 BUILDINGS 逐栋建楼，结果写进 target.buildings。
 *
 * 凹多边形（L 形）由 ExtrudeGeometry 自行三角化，顶/底面与侧壁都能正确成面，
 * 调用方不需要先拆成凸片。
 */
export function createBuildings(target: StudioScene): void {
  for (const [index, data] of BUILDINGS.entries()) {
    const geometry = createPrismGeometry(data.footprint, data.h);

    const isStudio = data.name === CONFIG.studioBuildingName;
    const material = new THREE.MeshStandardMaterial({
      color: isStudio ? CONFIG.studioColor : CONFIG.buildingColor,
      roughness: CONFIG.roughness,
      metalness: CONFIG.metalness,
    });

    const mesh = new THREE.Mesh(geometry, material);
    mesh.name = data.name;
    mesh.position.set(data.x, 0, data.z);
    // 初始压扁到几乎为零 = 藏在地面里，等时间轴把它拉起来
    mesh.scale.y = 0.001;
    mesh.castShadow = CONFIG.castShadow;
    mesh.receiveShadow = CONFIG.castShadow;
    target.scene.add(mesh);
    trackDisposable(target, geometry, material);

    if (isStudio) {
      addStudioOutline(target, mesh, geometry);
    }

    target.buildings.push({ mesh, height: data.h, index });
  }
}

/**
 * 给工作室楼栋描边高亮。
 *
 * EdgesGeometry 复用「已经挤出并旋转过」的同一份几何，所以描边天然贴在柱体棱上
 * （包括凹多边形的内棱）；再作为 mesh 的子对象挂上去，于是它会自动跟着 scale.y
 * 一起升起，不需要在时间轴里单独照顾它。
 */
function addStudioOutline(
  target: StudioScene,
  mesh: THREE.Mesh,
  geometry: THREE.BufferGeometry,
): void {
  const edgesGeometry = new THREE.EdgesGeometry(geometry, EDGE_THRESHOLD_DEGREES);
  const edgesMaterial = new THREE.LineBasicMaterial({
    color: CONFIG.studioEdgeColor,
  });
  mesh.add(new THREE.LineSegments(edgesGeometry, edgesMaterial));
  trackDisposable(target, edgesGeometry, edgesMaterial);
}

/**
 * 启动时自检 buildings.ts / CONFIG 的一致性，把问题打到控制台。
 *
 * 这些错误都不会抛异常、画面上却看得出来（穿模、没有高亮、平视偏掉），
 * 手改地图坐标时最难定位，所以统一在这里报出来。
 */
export function reportSceneIssues(): void {
  for (const name of findDuplicateNames()) {
    console.warn(`[StudioOpener] BUILDINGS 里存在重复的 name：${name}`);
  }

  for (const name of findDegenerateFootprints()) {
    console.warn(
      `[StudioOpener] BUILDINGS 里 ${name} 的 footprint 少于 3 个顶点，构不出体。`,
    );
  }

  for (const [a, b] of findOverlappingBuildings()) {
    console.warn(
      `[StudioOpener] BUILDINGS 里 ${a} 与 ${b} 的底面相交，画面上会穿模。`,
    );
  }

  const studio = findBuilding(CONFIG.studioBuildingName);
  if (!studio) {
    console.warn(
      `[StudioOpener] CONFIG.studioBuildingName="${CONFIG.studioBuildingName}" 在 BUILDINGS 里找不到，` +
        `本次没有任何楼栋被高亮为工作室。`,
    );
    return;
  }

  // 阶段 2 要跟工作室「平视」，所以注视点高度必须约等于楼栋中部高度
  const midHeight = studio.h / 2;
  if (Math.abs(CONFIG.cameraStudioTarget.y - midHeight) > 3) {
    console.warn(
      `[StudioOpener] CONFIG.cameraStudioTarget.y=${CONFIG.cameraStudioTarget.y} ` +
        `与工作室 ${studio.name} 的中部高度 ${midHeight} 相差超过 3，平视会偏。` +
        `建议改成 ${midHeight}。`,
    );
  }
}
