/**
 * scene/ 内部共享的类型定义。
 *
 * 这一层刻意**不依赖 Vue**：scene/ 下所有模块都是纯粹的 Three.js/GSAP 代码，
 * 由 composables/useStudioScene.ts 负责与 Vue 生命周期对接。
 */
import type {
  BufferGeometry,
  Material,
  Mesh,
  PerspectiveCamera,
  Scene,
  WebGLRenderer,
} from "three";

/** 可以释放 GPU 资源的东西（几何体、材质都符合） */
export interface Disposable {
  dispose(): void;
}

/** 相机 lookAt 的目标点。GSAP 只能补间普通对象，所以用这个代理对象承载，
 *  再由渲染循环每帧把它喂给 camera.lookAt() */
export interface LookAtState {
  x: number;
  y: number;
  z: number;
}

/** 一栋楼的运行时记录 */
export interface BuildingRuntime {
  mesh: Mesh;
  /** 楼栋原始高度（米），用于把高度映射到升起动画时长 */
  height: number;
  /** 在 BUILDINGS 中的序号，用于错峰延迟 */
  index: number;
}

/**
 * 建好的 3D 场景句柄。由 createStudioScene 产出，再交给
 * createBuildings / createStudioTimeline / studioRuntime 逐层加工。
 */
export interface StudioScene {
  /** canvas 的宿主元素 */
  host: HTMLElement;
  scene: Scene;
  camera: PerspectiveCamera;
  renderer: WebGLRenderer;
  /** 需要随组件销毁一起释放的几何体/材质 */
  disposables: Disposable[];
  /** 已生成的楼栋 */
  buildings: BuildingRuntime[];
  /** 相机注视点代理 */
  lookAt: LookAtState;
}

/** 把任意 BufferGeometry / Material 收进待释放清单 */
export function trackDisposable(
  target: StudioScene,
  ...items: Array<BufferGeometry | Material | Disposable>
): void {
  for (const item of items) {
    target.disposables.push(item);
  }
}
