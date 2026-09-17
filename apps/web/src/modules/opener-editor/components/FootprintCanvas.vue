<script setup lang="ts">
/**
 * 俯视画布：拖拽楼栋、拖顶点改形状、滚轮缩放、Shift 拖拽平移。
 *
 * 这里刻意不含任何几何计算——世界/屏幕换算、命中测试、缩放锚点全部来自
 * viewport.ts（有单测）。本组件只负责：
 *   1. 把鼠标像素坐标交给 viewport.ts 换算
 *   2. 把结果作为**绝对坐标**事件抛给父级（不抛增量）
 *
 * 为什么抛绝对坐标而不是增量：增量会在每一步都做一次 3 位小数取整，
 * 拖几百帧之后会累积出可见的漂移。父级拿到的绝对位置永远等于"光标所在处"。
 */
import { computed, onMounted, onUnmounted, ref } from "vue";

import { polygonCentroid } from "../../opener/footprint";
import type { BuildingDraft } from "../draft";
import {
  fitViewport,
  hitTestBuilding,
  screenToWorld,
  worldToScreen,
  zoomAt,
  type Viewport,
} from "../viewport";

const props = defineProps<{
  drafts: BuildingDraft[];
  selectedId: string | null;
  selectedVertex: number | null;
  /** 工作室楼栋名；它会被高亮成白色并加粗描边 */
  studioName: string;
  /** 有问题的楼栋名，画布上用红色标出 */
  issueNames: string[];
}>();

const emit = defineEmits<{
  (event: "select", id: string | null): void;
  (event: "select-vertex", index: number | null): void;
  (event: "position", id: string, x: number, z: number): void;
  (event: "vertex", id: string, index: number, localX: number, localZ: number): void;
  (event: "add-vertex", id: string, edgeIndex: number): void;
}>();

const svgRef = ref<SVGSVGElement | null>(null);
const viewport = ref<Viewport>({ scale: 1, originX: 300, originY: 260 });
const canvasSize = ref({ width: 900, height: 520 });

/** 一次拖拽的上下文；为 null 表示当前没有拖拽 */
type DragState =
  | { kind: "building"; id: string; grabOffsetX: number; grabOffsetZ: number }
  | { kind: "vertex"; id: string; index: number }
  | { kind: "pan"; lastScreenX: number; lastScreenY: number };

let drag: DragState | null = null;

const selectedDraft = computed(
  () => props.drafts.find((draft) => draft.id === props.selectedId) ?? null,
);

/** 楼栋底面的屏幕多边形点串 */
function screenPolygon(draft: BuildingDraft): string {
  return draft.footprint
    .map(([localX, localZ]) =>
      worldToScreen(viewport.value, draft.x + localX, draft.z + localZ).join(","),
    )
    .join(" ");
}

function originScreen(draft: BuildingDraft): [number, number] {
  return worldToScreen(viewport.value, draft.x, draft.z);
}

const labelScreen = (draft: BuildingDraft): { x: number; y: number } => {
  const [cx, cz] = polygonCentroid(draft.footprint);
  const [sx, sy] = worldToScreen(viewport.value, draft.x + cx, draft.z + cz);
  return { x: sx, y: sy };
};

const isStudio = (draft: BuildingDraft): boolean =>
  draft.name === props.studioName;

const hasIssue = (draft: BuildingDraft): boolean =>
  props.issueNames.includes(draft.name);

/** 世界原点的屏幕位置，用于画坐标参考线 */
const worldOrigin = computed<[number, number]>(() =>
  worldToScreen(viewport.value, 0, 0),
);

/** 网格线：按当前缩放取一个合适的世界步长，避免缩小时画出上千条线 */
const gridLines = computed(() => {
  const vp = viewport.value;
  const steps = [1, 5, 10, 20, 50, 100, 200];
  const step = steps.find((candidate) => candidate * vp.scale >= 40) ?? 500;
  const [left, top] = screenToWorld(vp, 0, 0);
  const [right, bottom] = screenToWorld(vp, canvasSize.value.width, canvasSize.value.height);

  const verticals: number[] = [];
  for (let x = Math.ceil(left / step) * step; x <= right; x += step) {
    verticals.push(worldToScreen(vp, x, 0)[0]);
  }

  const horizontals: number[] = [];
  for (let z = Math.ceil(top / step) * step; z <= bottom; z += step) {
    horizontals.push(worldToScreen(vp, 0, z)[1]);
  }

  return { verticals, horizontals, step };
});

function measure(): void {
  const svg = svgRef.value;
  if (!svg) {
    return;
  }
  const width = svg.clientWidth || 900;
  const height = svg.clientHeight || 520;
  canvasSize.value = { width, height };
}

/** 让所有楼栋刚好铺满画布 */
function fitView(): void {
  measure();
  viewport.value = fitViewport(
    props.drafts,
    canvasSize.value.width,
    canvasSize.value.height,
  );
}

function zoomBy(factor: number): void {
  measure();
  viewport.value = zoomAt(
    viewport.value,
    canvasSize.value.width / 2,
    canvasSize.value.height / 2,
    factor,
  );
}

/** 鼠标事件 → SVG 用户坐标（等于 CSS 像素，因为没设 viewBox）。
 *  只依赖 clientX/clientY，所以 PointerEvent 与 WheelEvent 都能传进来 */
function toLocal(event: { clientX: number; clientY: number }): [number, number] {
  const svg = svgRef.value;
  if (!svg) {
    return [0, 0];
  }
  const rect = svg.getBoundingClientRect();
  return [event.clientX - rect.left, event.clientY - rect.top];
}

function handlePolygonPointerDown(event: PointerEvent, draft: BuildingDraft): void {
  event.stopPropagation();
  emit("select", draft.id);
  emit("select-vertex", null);

  const [screenX, screenY] = toLocal(event);
  const [worldX, worldZ] = screenToWorld(viewport.value, screenX, screenY);
  // 记下光标与楼栋原点的偏移，拖拽时保持这个偏移，手感才是"抓住不放"
  drag = {
    kind: "building",
    id: draft.id,
    grabOffsetX: worldX - draft.x,
    grabOffsetZ: worldZ - draft.z,
  };
  svgRef.value?.setPointerCapture(event.pointerId);
}

function handleVertexPointerDown(
  event: PointerEvent,
  draft: BuildingDraft,
  index: number,
): void {
  event.stopPropagation();
  emit("select", draft.id);
  emit("select-vertex", index);
  drag = { kind: "vertex", id: draft.id, index };
  svgRef.value?.setPointerCapture(event.pointerId);
}

function handleBackgroundPointerDown(event: PointerEvent): void {
  const [screenX, screenY] = toLocal(event);
  const hit = hitTestBuilding(viewport.value, props.drafts, screenX, screenY);

  if (hit === null) {
    emit("select", null);
    emit("select-vertex", null);
  }

  // Shift + 拖拽 = 平移画布；普通拖拽空白处不做事
  if (event.shiftKey) {
    drag = { kind: "pan", lastScreenX: screenX, lastScreenY: screenY };
    svgRef.value?.setPointerCapture(event.pointerId);
  }
}

function handlePointerMove(event: PointerEvent): void {
  // 先把 drag 收进局部常量：TS 无法对可变的模块级变量做跨回调收窄，
  // 直接用 drag?.id 会因为 DragState 里还有 pan 分支而报"属性不存在"
  const active = drag;
  if (active === null) {
    return;
  }

  const [screenX, screenY] = toLocal(event);

  if (active.kind === "pan") {
    // 平移直接在屏幕空间累加：视口平移本来就是像素级的，绕一圈换算成世界单位
    // 再乘回来只会引入取整误差
    viewport.value = {
      ...viewport.value,
      originX: viewport.value.originX + (screenX - active.lastScreenX),
      originY: viewport.value.originY + (screenY - active.lastScreenY),
    };
    active.lastScreenX = screenX;
    active.lastScreenY = screenY;
    return;
  }

  const [worldX, worldZ] = screenToWorld(viewport.value, screenX, screenY);

  if (active.kind === "building") {
    emit("position", active.id, worldX - active.grabOffsetX, worldZ - active.grabOffsetZ);
    return;
  }

  const draft = props.drafts.find((item) => item.id === active.id);
  if (!draft) {
    return;
  }
  // 顶点坐标是局部坐标，要减掉楼栋原点
  emit("vertex", active.id, active.index, worldX - draft.x, worldZ - draft.z);
}

function handlePointerUp(event: PointerEvent): void {
  drag = null;
  svgRef.value?.releasePointerCapture(event.pointerId);
}

function handleWheel(event: WheelEvent): void {
  event.preventDefault();
  const [screenX, screenY] = toLocal(event);
  // deltaY < 0 表示向上滚 = 放大
  const factor = event.deltaY < 0 ? 1.12 : 1 / 1.12;
  viewport.value = zoomAt(viewport.value, screenX, screenY, factor);
}

/** 双击楼栋边线：在该边中点插入一个顶点 */
function handleEdgePointerDown(
  event: PointerEvent,
  draft: BuildingDraft,
  edgeIndex: number,
): void {
  if (!event.shiftKey) {
    return;
  }
  event.stopPropagation();
  emit("add-vertex", draft.id, edgeIndex);
}

function edgeMidpoint(draft: BuildingDraft, edgeIndex: number): string {
  const start = draft.footprint[edgeIndex];
  const end = draft.footprint[(edgeIndex + 1) % draft.footprint.length];
  if (!start || !end) {
    return "";
  }
  const [sx, sy] = worldToScreen(
    viewport.value,
    draft.x + (start[0] + end[0]) / 2,
    draft.z + (start[1] + end[1]) / 2,
  );
  return `${sx},${sy}`;
}

let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  measure();
  fitView();

  const svg = svgRef.value;
  if (svg && typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver(() => {
      measure();
    });
    resizeObserver.observe(svg);
  }
});

onUnmounted(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
});

defineExpose({ fitView, zoomBy });
</script>

<template>
  <div class="canvas-wrap">
    <div class="canvas-toolbar">
      <button type="button" @click="zoomBy(1.25)">放大</button>
      <button type="button" @click="zoomBy(1 / 1.25)">缩小</button>
      <button type="button" @click="fitView">适配视图</button>
      <span class="canvas-toolbar__hint">
        拖动楼栋移动 · 拖圆点改顶点 · Shift+点边线加顶点 · Shift+拖拽平移 · 滚轮缩放
      </span>
    </div>

    <!-- 不设 viewBox：SVG 用户单位就等于 CSS 像素，屏幕坐标换算最简单 -->
    <svg
      ref="svgRef"
      class="canvas"
      role="application"
      aria-label="楼栋俯视编辑画布"
      @pointerdown="handleBackgroundPointerDown"
      @pointermove="handlePointerMove"
      @pointerup="handlePointerUp"
      @pointercancel="handlePointerUp"
      @wheel="handleWheel"
    >
      <rect
        x="0"
        y="0"
        width="100%"
        height="100%"
        class="canvas__ground"
      />

      <!-- 网格 -->
      <g class="canvas__grid">
        <line
          v-for="x in gridLines.verticals"
          :key="`v-${x}`"
          :x1="x"
          :y1="0"
          :x2="x"
          :y2="canvasSize.height"
        />
        <line
          v-for="y in gridLines.horizontals"
          :key="`h-${y}`"
          :x1="0"
          :y1="y"
          :x2="canvasSize.width"
          :y2="y"
        />
      </g>

      <!-- 世界原点参考线：相机注视点由楼栋原点推导，这里给出参照 -->
      <line
        :x1="worldOrigin[0]"
        :y1="0"
        :x2="worldOrigin[0]"
        :y2="canvasSize.height"
        class="canvas__axis"
      />
      <line
        :x1="0"
        :y1="worldOrigin[1]"
        :x2="canvasSize.width"
        :y2="worldOrigin[1]"
        class="canvas__axis"
      />

      <!-- 楼栋 -->
      <g v-for="draft in drafts" :key="draft.id">
        <polygon
          :points="screenPolygon(draft)"
          class="canvas__building"
          :class="{
            'canvas__building--selected': draft.id === selectedId,
            'canvas__building--studio': isStudio(draft),
            'canvas__building--issue': hasIssue(draft),
          }"
          @pointerdown="handlePolygonPointerDown($event, draft)"
        />

        <!-- 边中点手柄：Shift 点击在此边插入顶点 -->
        <circle
          v-for="(_, edgeIndex) in draft.footprint"
          :key="`edge-${draft.id}-${edgeIndex}`"
          :cx="edgeMidpoint(draft, edgeIndex).split(',')[0]"
          :cy="edgeMidpoint(draft, edgeIndex).split(',')[1]"
          r="3"
          class="canvas__edge-handle"
          @pointerdown="handleEdgePointerDown($event, draft, edgeIndex)"
        />

        <text :x="labelScreen(draft).x" :y="labelScreen(draft).y" class="canvas__label">
          {{ draft.name }}
        </text>
      </g>

      <!-- 选中楼的顶点手柄（画在最上层，保证可点） -->
      <g v-if="selectedDraft">
        <circle
          v-for="([localX, localZ], index) in selectedDraft.footprint"
          :key="`vertex-${index}`"
          :cx="worldToScreen(viewport, selectedDraft.x + localX, selectedDraft.z + localZ)[0]"
          :cy="worldToScreen(viewport, selectedDraft.x + localX, selectedDraft.z + localZ)[1]"
          r="6"
          class="canvas__vertex"
          :class="{ 'canvas__vertex--active': index === selectedVertex }"
          @pointerdown="handleVertexPointerDown($event, selectedDraft, index)"
        />
      </g>
    </svg>
  </div>
</template>

<style scoped>
.canvas-wrap {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 8px;
}

.canvas-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.canvas-toolbar button {
  padding: 6px 12px;
  color: var(--editor-ink);
  border: 1px solid var(--editor-border);
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.canvas-toolbar button:hover {
  border-color: var(--editor-accent);
  color: var(--editor-accent);
}

.canvas-toolbar__hint {
  color: var(--editor-muted);
  font-size: 11px;
}

.canvas {
  width: 100%;
  height: 520px;
  border: 1px solid var(--editor-border);
  border-radius: 10px;
  background: #f7f8fa;
  touch-action: none;
  user-select: none;
}

.canvas__ground {
  fill: #f7f8fa;
}

.canvas__grid line {
  stroke: #e2e6ea;
  stroke-width: 1;
}

.canvas__axis line,
.canvas__axis {
  stroke: #c9d2da;
  stroke-dasharray: 4 4;
  stroke-width: 1;
}

.canvas__building {
  fill: rgb(224 224 224 / 85%);
  stroke: #9aa7b4;
  stroke-width: 1.5;
  cursor: grab;
}

.canvas__building:hover {
  fill: rgb(210 218 226 / 90%);
}

.canvas__building--studio {
  fill: rgb(255 255 255 / 95%);
  stroke: #164d80;
  stroke-width: 2;
}

.canvas__building--selected {
  stroke: #1e659f;
  stroke-width: 2.5;
}

.canvas__building--issue {
  fill: rgb(248 228 224 / 90%);
  stroke: #b84235;
  stroke-width: 2;
}

.canvas__edge-handle {
  fill: transparent;
  stroke: none;
  cursor: copy;
}

.canvas__edge-handle:hover {
  fill: rgb(30 101 159 / 45%);
}

.canvas__vertex {
  fill: #fff;
  stroke: #1e659f;
  stroke-width: 2;
  cursor: move;
}

.canvas__vertex--active {
  fill: #1e659f;
}

.canvas__label {
  fill: #52657b;
  font-size: 10px;
  text-anchor: middle;
  pointer-events: none;
  user-select: none;
}
</style>
