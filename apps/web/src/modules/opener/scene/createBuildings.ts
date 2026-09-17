/**
 * 用 BoxGeometry 生成整座校园（白模）。
 *
 * 不使用任何 GLB/GLTF 模型文件：每栋楼 = 一次 BoxGeometry + 一个
 * MeshStandardMaterial。布局完全来自 buildings.ts 的 BUILDINGS 数组。
 */
import * as THREE from "three";

import { BUILDINGS, findBuilding, findDuplicateNames, findOverlappingBuildings } from "../buildings";
import { CONFIG } from "../config";
import { type StudioScene, trackDisposable } from "./types";

/**
 * 按 BUILDINGS 逐栋建楼，结果写进 target.buildings。
 *
 * 关键一步是 `geo.translate(0, h / 2, 0)`：BoxGeometry 的原点在体心，
 * 平移后原点落在**底面中心**，于是
 *   - `mesh.position.y = 0` 就等于「楼立在地面上」；
 *   - 可以用 `mesh.scale.y = 0.001 → 1` 做从地面升起的动画。
 */
export function createBuildings(target: StudioScene): void {
  for (const [index, data] of BUILDINGS.entries()) {
    const geometry = new THREE.BoxGeometry(data.w, data.h, data.d);
    geometry.translate(0, data.h / 2, 0);

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
 * EdgesGeometry 复用「已经 translate 过」的同一份几何，所以描边天然贴在
 * 楼栋棱上；再作为 mesh 的子对象挂上去，于是它会自动跟着 scale.y 一起升起，
 * 不需要在时间轴里单独照顾它。
 */
function addStudioOutline(
  target: StudioScene,
  mesh: THREE.Mesh,
  geometry: THREE.BufferGeometry,
): void {
  const edgesGeometry = new THREE.EdgesGeometry(geometry);
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

  for (const [a, b] of findOverlappingBuildings()) {
    console.warn(
      `[StudioOpener] BUILDINGS 里 ${a} 与 ${b} 的底面范围重叠，画面上会穿模。`,
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
