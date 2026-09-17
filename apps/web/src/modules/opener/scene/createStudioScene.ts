/**
 * 搭建场景基础：Scene / Camera / Renderer / 光照 / 地面。
 *
 * 只负责「舞台」，楼栋由 createBuildings 填进来，动画由 createStudioTimeline 管。
 */
import * as THREE from "three";

import { CONFIG } from "../config";
import { type StudioScene, trackDisposable } from "./types";

/**
 * 创建 3D 场景并把 canvas 挂到 host 上。
 *
 * @throws 当浏览器不支持 / 禁用了 WebGL 时由 WebGLRenderer 抛出，调用方负责降级。
 */
export function createStudioScene(host: HTMLElement): StudioScene {
  const width = host.clientWidth || window.innerWidth;
  const height = host.clientHeight || window.innerHeight;

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(CONFIG.background);

  const camera = new THREE.PerspectiveCamera(
    CONFIG.fov,
    width / height,
    CONFIG.near,
    CONFIG.far,
  );
  camera.position.set(
    CONFIG.cameraStart.x,
    CONFIG.cameraStart.y,
    CONFIG.cameraStart.z,
  );

  const lookAt = {
    x: CONFIG.cameraStartTarget.x,
    y: CONFIG.cameraStartTarget.y,
    z: CONFIG.cameraStartTarget.z,
  };
  camera.lookAt(lookAt.x, lookAt.y, lookAt.z);

  const renderer = new THREE.WebGLRenderer({ antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, CONFIG.maxPixelRatio));
  // canvas 的 CSS 尺寸由样式表统一控制（宽高 100%），所以这里 updateStyle 传 false
  renderer.setSize(width, height, false);
  renderer.shadowMap.enabled = CONFIG.castShadow;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  host.appendChild(renderer.domElement);

  const studioScene: StudioScene = {
    host,
    scene,
    camera,
    renderer,
    clock: new THREE.Clock(),
    disposables: [],
    buildings: [],
    lookAt,
  };

  addLights(studioScene);
  addGround(studioScene);

  return studioScene;
}

/** 环境光补底 + 方向光当太阳（可选投影） */
function addLights(target: StudioScene): void {
  const ambient = new THREE.AmbientLight(0xffffff, CONFIG.ambientIntensity);
  target.scene.add(ambient);

  const sun = new THREE.DirectionalLight(
    0xffffff,
    CONFIG.directionalIntensity,
  );
  sun.position.set(
    CONFIG.directionalPosition.x,
    CONFIG.directionalPosition.y,
    CONFIG.directionalPosition.z,
  );
  sun.target.position.set(0, 0, 0);

  if (CONFIG.castShadow) {
    sun.castShadow = true;
    sun.shadow.mapSize.set(CONFIG.shadowMapSize, CONFIG.shadowMapSize);
    // 正交阴影相机必须罩住整个校园，否则远处的楼会被裁掉影子
    const extent = CONFIG.shadowExtent;
    sun.shadow.camera.left = -extent;
    sun.shadow.camera.right = extent;
    sun.shadow.camera.top = extent;
    sun.shadow.camera.bottom = -extent;
    sun.shadow.camera.near = 1;
    sun.shadow.camera.far = 500;
    sun.shadow.bias = -0.0005;
  }

  target.scene.add(sun);
  target.scene.add(sun.target);
}

/** 地面平面（颜色略深于楼栋）+ 可选的辅助网格 */
function addGround(target: StudioScene): void {
  const groundGeo = new THREE.PlaneGeometry(
    CONFIG.groundSize,
    CONFIG.groundSize,
  );
  // PlaneGeometry 默认立在 XY 平面，绕 X 轴转 -90° 才是水平地面
  groundGeo.rotateX(-Math.PI / 2);

  const groundMat = new THREE.MeshStandardMaterial({
    color: CONFIG.groundColor,
    roughness: 1,
    metalness: 0,
  });

  const ground = new THREE.Mesh(groundGeo, groundMat);
  ground.name = "Ground";
  ground.receiveShadow = CONFIG.castShadow;
  target.scene.add(ground);
  trackDisposable(target, groundGeo, groundMat);

  if (!CONFIG.showGrid) {
    return;
  }

  const grid = new THREE.GridHelper(
    CONFIG.groundSize,
    CONFIG.gridDivisions,
    CONFIG.gridColor,
    CONFIG.gridColor,
  );
  // 抬高一点点避免与地面共面产生 z-fighting。楼栋底面在 y=0，网格线落进
  // 楼栋内部会被墙体遮住，不会穿透到楼顶。
  grid.position.y = 0.05;
  const gridMaterial = grid.material as THREE.LineBasicMaterial;
  gridMaterial.transparent = true;
  gridMaterial.opacity = CONFIG.gridOpacity;
  target.scene.add(grid);
  trackDisposable(target, grid.geometry, gridMaterial);
}
