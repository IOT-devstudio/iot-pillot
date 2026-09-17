<script setup lang="ts">
/**
 * 楼栋布局可视化编辑器（管理端）。
 *
 * 分工：
 *   draft.ts    草稿模型与全部编辑操作（纯函数）
 *   viewport.ts 俯视画布的世界↔屏幕换算与命中测试（纯函数）
 *   codegen.ts  导出可粘贴的 BUILDINGS 与相机 CONFIG 片段
 *   本组件只负责把三者接起来，以及工具栏/校验提示/代码预览
 *
 * 准入由路由 meta.requiresAdmin + router/guards.ts 负责，
 * 真正的强制在服务端 RequireRole 中间件。这里不做任何权限判断。
 */
import { computed, ref } from "vue";

import { BUILDINGS, type BuildingData } from "../../opener/buildings";
import { CONFIG } from "../../opener/config";
import { roundCoord } from "../../opener/footprint";
import BuildingInspector from "../components/BuildingInspector.vue";
import FootprintCanvas from "../components/FootprintCanvas.vue";
import { generateCode } from "../codegen";
import {
  addBuilding,
  addVertexOnEdge,
  deleteBuilding,
  deleteVertex,
  fromBuildingData,
  moveVertex,
  renameBuilding,
  setBuildingHeight,
  setBuildingPosition,
  setStudioBuilding,
  toBuildingData,
  validateDraft,
  type BuildingDraft,
} from "../draft";

const studioName = CONFIG.studioBuildingName;

const drafts = ref<BuildingDraft[]>(fromBuildingData(BUILDINGS));
const selectedId = ref<string | null>(null);
const selectedVertex = ref<number | null>(null);
const copied = ref(false);

const selectedDraft = computed(
  () => drafts.value.find((draft) => draft.id === selectedId.value) ?? null,
);

const issues = computed(() => validateDraft(drafts.value, studioName));
const errors = computed(() => issues.value.filter((issue) => issue.level === "error"));
const warnings = computed(() =>
  issues.value.filter((issue) => issue.level === "warning"),
);
const issueNames = computed(() =>
  issues.value.flatMap((issue) => issue.buildings ?? []),
);

const generatedCode = computed(() => generateCode(drafts.value, studioName));

/** 有阻断性错误时不允许复制：粘贴回去会让运行时也报同样的错 */
const canCopy = computed(() => errors.value.length === 0);

function selectBuilding(id: string | null): void {
  selectedId.value = id;
  selectedVertex.value = null;
}

function handlePosition(id: string, x: number, z: number): void {
  drafts.value = setBuildingPosition(drafts.value, id, x, z);
}

function handleVertex(id: string, index: number, localX: number, localZ: number): void {
  drafts.value = moveVertex(drafts.value, id, index, localX, localZ);
}

function handleAddVertex(id: string, edgeIndex: number): void {
  drafts.value = addVertexOnEdge(drafts.value, id, edgeIndex);
}

function handleDeleteSelected(): void {
  if (!selectedId.value) {
    return;
  }
  drafts.value = deleteBuilding(drafts.value, selectedId.value);
  selectedId.value = null;
  selectedVertex.value = null;
}

function handleRename(name: string): void {
  if (!selectedId.value) {
    return;
  }
  drafts.value = renameBuilding(drafts.value, selectedId.value, name);
}

function handleHeight(h: number): void {
  if (!selectedId.value) {
    return;
  }
  drafts.value = setBuildingHeight(drafts.value, selectedId.value, h);
}

function handleMakeStudio(): void {
  if (!selectedId.value) {
    return;
  }
  drafts.value = setStudioBuilding(drafts.value, selectedId.value, studioName);
}

function handleInspectorVertex(index: number, localX: number, localZ: number): void {
  if (!selectedId.value) {
    return;
  }
  drafts.value = moveVertex(drafts.value, selectedId.value, index, localX, localZ);
}

function handleDeleteVertex(index: number): void {
  if (!selectedId.value) {
    return;
  }
  drafts.value = deleteVertex(drafts.value, selectedId.value, index);
  selectedVertex.value = null;
}

/** 重置为源码里当前的 BUILDINGS（放弃本次编辑） */
function resetToSource(): void {
  drafts.value = fromBuildingData(BUILDINGS);
  selectedId.value = null;
  selectedVertex.value = null;
}

/** 给所有楼栋的原点做一次取整，方便手抄与 diff */
function roundAll(): void {
  drafts.value = drafts.value.map((draft) => ({
    ...draft,
    x: roundCoord(draft.x),
    z: roundCoord(draft.z),
    h: roundCoord(draft.h),
  }));
}

async function copyCode(): Promise<void> {
  const code = generatedCode.value;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(code);
    } else {
      throw new Error("clipboard unavailable");
    }
    copied.value = true;
    window.setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch {
    // 剪贴板被浏览器策略拦下（非 https / 无用户手势）时给出手动复制路径：
    // 代码区本来就是可全选的 textarea，这里只提示一下
    copied.value = false;
    window.alert("浏览器未允许自动复制，请在下方代码框中手动全选复制。");
  }
}

/** 把生成物导出成实际数据，便于在控制台核对（不写任何文件） */
function logParsedShape(): void {
  console.info("[BuildingsEditor] 当前草稿解析结果：", toBuildingData(drafts.value));
}

const buildingCount = computed(() => drafts.value.length);
const buildingData = computed<BuildingData[]>(() => toBuildingData(drafts.value));
</script>

<template>
  <div class="editor">
    <header class="editor__header">
      <div>
        <h1 class="editor__title">楼栋布局编辑器</h1>
        <p class="editor__subtitle">
          共 {{ buildingCount }} 栋 · 工作室：{{ studioName }} ·
          编辑完成后复制生成的代码，替换 <code>buildings.ts</code> 与
          <code>config.ts</code> 中的对应片段
        </p>
      </div>
      <div class="editor__actions">
        <button type="button" @click="drafts = addBuilding(drafts)">新增楼栋</button>
        <button type="button" :disabled="!selectedId" @click="handleDeleteSelected">
          删除选中
        </button>
        <button type="button" @click="roundAll">坐标取整</button>
        <button type="button" @click="resetToSource">放弃修改</button>
        <button type="button" @click="logParsedShape">打印解析结果</button>
      </div>
    </header>

    <!-- 校验结果：与运行时 reportSceneIssues 用同一批判定函数 -->
    <div v-if="errors.length" class="editor__issues editor__issues--error">
      <p v-for="(issue, index) in errors" :key="`e-${index}`">✕ {{ issue.message }}</p>
    </div>
    <div v-if="warnings.length" class="editor__issues editor__issues--warn">
      <p v-for="(issue, index) in warnings" :key="`w-${index}`">! {{ issue.message }}</p>
    </div>
    <div v-if="!errors.length && !warnings.length" class="editor__issues editor__issues--ok">
      <p>✓ 布局校验通过，可以复制代码。</p>
    </div>

    <div class="editor__body">
      <FootprintCanvas
        class="editor__canvas"
        :drafts="drafts"
        :selected-id="selectedId"
        :selected-vertex="selectedVertex"
        :studio-name="studioName"
        :issue-names="issueNames"
        @select="selectBuilding"
        @select-vertex="selectedVertex = $event"
        @position="handlePosition"
        @vertex="handleVertex"
        @add-vertex="handleAddVertex"
      />

      <BuildingInspector
        :draft="selectedDraft"
        :selected-vertex="selectedVertex"
        :studio-name="studioName"
        @rename="handleRename"
        @position="(x, z) => selectedId && (drafts = setBuildingPosition(drafts, selectedId, x, z))"
        @height="handleHeight"
        @make-studio="handleMakeStudio"
        @vertex="handleInspectorVertex"
        @delete-vertex="handleDeleteVertex"
        @select-vertex="selectedVertex = $event"
      />
    </div>

    <section class="editor__code">
      <div class="editor__code-head">
        <h2>生成的代码</h2>
        <button type="button" :disabled="!canCopy" @click="copyCode">
          {{ copied ? "已复制 ✓" : "复制全部" }}
        </button>
      </div>
      <textarea class="editor__code-box" readonly :value="generatedCode" rows="18"></textarea>
      <p class="editor__code-hint">
        共 {{ buildingData.length }} 栋楼。相机片段由工作室的底面形心与高度推导，
        注视点高度恒等于楼栋高度的一半，因此阶段 2 结束时是与工作室平视。
      </p>
    </section>
  </div>
</template>

<style scoped>
/* 设计令牌：与开屏模块用同一套品牌色，避免两个页面观感割裂 */
.editor {
  --editor-ink: #102b4e;
  --editor-muted: #5c6b7a;
  --editor-accent: #164d80;
  --editor-border: #d8dee4;

  min-height: 100vh;
  padding: 24px clamp(16px, 4vw, 48px) 48px;
  color: var(--editor-ink);
  background: #f4f6f8;
  font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
}

.editor__header {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 16px;
}

.editor__title {
  margin: 0;
  font-size: 22px;
}

.editor__subtitle {
  margin: 6px 0 0;
  color: var(--editor-muted);
  font-size: 12px;
  line-height: 1.6;
}

.editor__subtitle code {
  padding: 1px 4px;
  border-radius: 4px;
  background: #e6ebf0;
  font-size: 11px;
}

.editor__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.editor__actions button {
  padding: 7px 12px;
  color: var(--editor-ink);
  border: 1px solid var(--editor-border);
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.editor__actions button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.editor__issues {
  margin-bottom: 12px;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.7;
}

.editor__issues p {
  margin: 0;
}

.editor__issues--error {
  color: #8e2f27;
  border-left: 3px solid #b84235;
  background: #fbeae6;
}

.editor__issues--warn {
  color: #7a5410;
  border-left: 3px solid #c98a1f;
  background: #fdf3e0;
}

.editor__issues--ok {
  color: #1c6b45;
  border-left: 3px solid #2e8b57;
  background: #e9f6ef;
}

.editor__body {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.editor__canvas {
  min-width: 0;
  flex: 1 1 auto;
}

.editor__code {
  margin-top: 20px;
}

.editor__code-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.editor__code-head h2 {
  margin: 0;
  font-size: 15px;
}

.editor__code-head button {
  padding: 7px 14px;
  color: #fff;
  border: 1px solid var(--editor-accent);
  border-radius: 6px;
  background: var(--editor-accent);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.editor__code-head button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.editor__code-box {
  width: 100%;
  padding: 14px;
  color: #e8eef4;
  border: 1px solid #0f2338;
  border-radius: 10px;
  background: #102b4e;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.55;
  resize: vertical;
  white-space: pre;
}

.editor__code-hint {
  margin: 8px 0 0;
  color: var(--editor-muted);
  font-size: 11px;
  line-height: 1.6;
}

@media (max-width: 1100px) {
  .editor__body {
    flex-direction: column;
  }

  .editor__canvas,
  .editor :deep(.inspector) {
    width: 100%;
    flex-basis: auto;
  }
}
</style>
