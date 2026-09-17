/**
 * 3D 开屏的可调参数。
 *
 * 想改镜头、节奏、配色，**只需要动这个文件**：scene/ 下的渲染与动画代码全部读它，
 * 没有任何魔法数字散落在别处。
 *
 * 单位约定：长度 = 米（与 buildings.ts 的 w/h/d 同量纲），角度 = 度，时间 = 秒。
 */

export interface Vec3 {
  x: number;
  y: number;
  z: number;
}

export const CONFIG = {
  /** 工作室楼栋的 name。改这里就能把任意一栋楼指定为镜头焦点与高亮对象 */
  studioBuildingName: "Building_Studio",

  // ── 相机 ──
  /** 阶段 0/1 的俯瞰机位 */
  cameraStart: { x: 0, y: 120, z: 120 },
  /** 阶段 0/1 俯瞰时看向的点（通常是校园中心）*/
  cameraStartTarget: { x: 0, y: 0, z: 0 },
  /** 阶段 2 结束后聚焦工作室时的机位；y 越小越接近平视 */
  cameraStudio: { x: 30, y: 15, z: 30 },
  /**
   * 阶段 2 结束后看向的点。
   * y 建议 ≈ 工作室楼栋高度的一半，这样相机与楼栋中部平视
   * （当前工作室 h=20 → 10）。改完 buildings.ts 里工作室的 h 之后这里要同步改，
   * 否则平视会偏；运行时会在偏差 > 3 时打印一条 console.warn 提醒。
   */
  cameraStudioTarget: { x: 15, y: 10, z: 5 },
  /** 阶段 3 相机沿 X 轴正方向（画面右侧）平移的距离，给登录面板让出左侧空间 */
  cameraShiftRight: 20,
  /** 透视相机视场角（度）*/
  fov: 45,
  near: 0.1,
  far: 2000,
  /** 阶段 1 里相机绕 Y 轴缓慢自转的角度；设为 0 关闭自转 */
  cameraSpinDegrees: 18,

  // ── 阶段 1：楼栋升起 ──
  /** 楼栋升起时长的取值范围：最矮的楼取 min，最高的楼取 max */
  buildingRiseDuration: { min: 1.2, max: 3.0 },
  /** 每栋楼的错峰延迟基数：第 index 栋延迟 index * 该值 秒 */
  buildingRiseDelay: 0.08,
  /** 在错峰延迟上再叠加的随机抖动上限，避免机械的整齐感 */
  buildingRiseRandomJitter: 0.15,
  buildingRiseEase: "power3.out",
  /**
   * 阶段 1 的名义时长下界。实际时长 = max(该值, 最后一栋楼的结束时刻)，
   * 因为「楼越高时长越长 + 错峰延迟」天然可能超过 3 秒。
   */
  phase1MinDuration: 3.0,

  // ── 阶段 2：聚焦工作室 ──
  phase2Duration: 2.5,
  focusEase: "power2.inOut",

  // ── 阶段 3：相机右移 + 面板滑入 ──
  phase3Duration: 1.5,
  /** 面板自身滑入的时长（比相机位移短一点，观感更利落）*/
  panelSlideDuration: 0.9,
  panelEase: "power3.out",

  // ── 登录面板 ──
  /** 面板宽度（px）。窄屏由 AuthPanel 的 CSS 兜底压窄 */
  loginPanelWidth: 360,

  // ── 空闲漂浮 ──
  /** 动画结束后相机漂浮的振幅（米）*/
  idleFloatAmplitude: 1.6,
  /** 漂浮速度系数，越大越晃 */
  idleFloatSpeed: 0.28,

  // ── 材质 / 光照 / 环境 ──
  /** 场景背景色，白模风格用接近纯白的浅灰 */
  background: 0xf2f4f7,
  /** 普通楼栋颜色 */
  buildingColor: 0xe0e0e0,
  /** 工作室楼栋颜色 */
  studioColor: 0xffffff,
  /** 工作室描边颜色（品牌蓝），便于在满屏白模里一眼找到焦点 */
  studioEdgeColor: 0x164d80,
  /** 地面颜色，故意比楼栋略深，让楼栋边界看得清 */
  groundColor: 0xcccccc,
  groundSize: 600,
  /** 地面网格：编辑 buildings.ts 坐标时很有用，上线嫌乱可以关掉 */
  showGrid: true,
  gridDivisions: 60,
  gridColor: 0xb9bec6,
  gridOpacity: 0.5,
  ambientIntensity: 0.85,
  directionalIntensity: 1.25,
  /** 方向光位置，target 固定为原点，等价于把太阳挂在校园右前上方 */
  directionalPosition: { x: 80, y: 140, z: 60 },
  /** 阴影开销不小；卡顿时把它关掉即可，画面依旧成立 */
  castShadow: true,
  shadowMapSize: 2048,
  /** 正交阴影相机半边长，必须罩住整个校园（当前楼栋最远约 ±60）*/
  shadowExtent: 150,
  roughness: 0.85,
  metalness: 0.04,
  /** 像素比上限，避免 4K 屏把 GPU 拖死 */
  maxPixelRatio: 2,

  // ── 注册验证码 ──
  /** 验证码发送后的冷却秒数 */
  verifyCodeCooldownSeconds: 60,
};
