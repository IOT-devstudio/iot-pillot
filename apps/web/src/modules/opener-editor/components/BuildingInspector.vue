<script setup lang="ts">
/**
 * 属性面板：编辑选中楼栋的名称、位置、高度与选中的顶点。
 *
 * 全部字段都是「受控输入 + 提交时校验」：直接 v-model 到草稿上会因为
 * 输入中间态（空串、负号、"1e"）产生 NaN 坐标，把楼栋扔到画布外面去。
 * 所以这里用本地值 + change 事件回写。
 */
import { computed, ref, watch } from "vue";

import type { BuildingDraft } from "../draft";

const props = defineProps<{
  draft: BuildingDraft | null;
  selectedVertex: number | null;
  studioName: string;
}>();

const emit = defineEmits<{
  (event: "rename", name: string): void;
  (event: "position", x: number, z: number): void;
  (event: "height", h: number): void;
  (event: "make-studio"): void;
  (event: "vertex", index: number, localX: number, localZ: number): void;
  (event: "delete-vertex", index: number): void;
  (event: "select-vertex", index: number | null): void;
}>();

const nameInput = ref("");
const xInput = ref("0");
const zInput = ref("0");
const hInput = ref("0");
const vertexXInput = ref("0");
const vertexZInput = ref("0");

/** 面板打开或切换楼栋时，把草稿值同步进输入框 */
watch(
  () => props.draft,
  (draft) => {
    if (!draft) {
      return;
    }
    nameInput.value = draft.name;
    xInput.value = String(draft.x);
    zInput.value = String(draft.z);
    hInput.value = String(draft.h);
  },
  { immediate: true },
);

watch(
  () => [props.draft?.id, props.selectedVertex] as const,
  () => {
    const vertex = currentVertex.value;
    if (!vertex) {
      return;
    }
    vertexXInput.value = String(vertex[0]);
    vertexZInput.value = String(vertex[1]);
  },
  { immediate: true },
);

const currentVertex = computed(() => {
  if (!props.draft || props.selectedVertex === null) {
    return null;
  }
  return props.draft.footprint[props.selectedVertex] ?? null;
});

const isStudio = computed(
  () => props.draft !== null && props.draft.name === props.studioName,
);

/** 解析数字输入；非法输入返回 null，调用方直接忽略，不把 NaN 写进草稿 */
function parseNumber(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "" || !Number.isFinite(Number(trimmed))) {
    return null;
  }
  return Number(trimmed);
}

function commitName(): void {
  const trimmed = nameInput.value.trim();
  if (trimmed !== "") {
    emit("rename", trimmed);
  } else if (props.draft) {
    // 名字不能为空，回滚显示
    nameInput.value = props.draft.name;
  }
}

function commitPosition(): void {
  const x = parseNumber(xInput.value);
  const z = parseNumber(zInput.value);
  if (x === null || z === null) {
    if (props.draft) {
      xInput.value = String(props.draft.x);
      zInput.value = String(props.draft.z);
    }
    return;
  }
  emit("position", x, z);
}

function commitHeight(): void {
  const h = parseNumber(hInput.value);
  if (h === null || h <= 0) {
    if (props.draft) {
      hInput.value = String(props.draft.h);
    }
    return;
  }
  emit("height", h);
}

function commitVertex(): void {
  if (props.selectedVertex === null || !currentVertex.value) {
    return;
  }
  const localX = parseNumber(vertexXInput.value);
  const localZ = parseNumber(vertexZInput.value);
  if (localX === null || localZ === null) {
    vertexXInput.value = String(currentVertex.value[0]);
    vertexZInput.value = String(currentVertex.value[1]);
    return;
  }
  emit("vertex", props.selectedVertex, localX, localZ);
}
</script>

<template>
  <aside class="inspector">
    <p v-if="!draft" class="inspector__empty">
      在画布上点选一栋楼开始编辑。
    </p>

    <template v-else>
      <h3 class="inspector__title">{{ draft.name }}</h3>

      <label class="inspector__field">
        <span>名称</span>
        <input v-model="nameInput" type="text" @change="commitName" />
      </label>

      <div class="inspector__row">
        <label class="inspector__field">
          <span>底面原点 X（米）</span>
          <input v-model="xInput" type="number" step="0.5" @change="commitPosition" />
        </label>
        <label class="inspector__field">
          <span>底面原点 Z（米）</span>
          <input v-model="zInput" type="number" step="0.5" @change="commitPosition" />
        </label>
      </div>

      <label class="inspector__field">
        <span>高度（米）</span>
        <input v-model="hInput" type="number" step="1" min="0.1" @change="commitHeight" />
      </label>

      <button
        type="button"
        class="inspector__button"
        :disabled="isStudio"
        @click="emit('make-studio')"
      >
        {{ isStudio ? "当前已是工作室" : "设为工作室" }}
      </button>

      <hr class="inspector__divider" />

      <div class="inspector__subhead">
        <span>底面顶点（{{ draft.footprint.length }} 个）</span>
        <button
          v-if="selectedVertex !== null"
          type="button"
          class="inspector__link"
          @click="emit('select-vertex', null)"
        >
          取消选中
        </button>
      </div>

      <p v-if="selectedVertex === null" class="inspector__hint">
        点击画布上的白色圆点选中顶点；Shift + 点边线可新增顶点。
      </p>

      <template v-else>
        <div class="inspector__row">
          <label class="inspector__field">
            <span>顶点 {{ selectedVertex }} 局部 X</span>
            <input v-model="vertexXInput" type="number" step="0.5" @change="commitVertex" />
          </label>
          <label class="inspector__field">
            <span>顶点 {{ selectedVertex }} 局部 Z</span>
            <input v-model="vertexZInput" type="number" step="0.5" @change="commitVertex" />
          </label>
        </div>
        <button
          type="button"
          class="inspector__button inspector__button--danger"
          :disabled="draft.footprint.length <= 3"
          @click="emit('delete-vertex', selectedVertex)"
        >
          删除该顶点
        </button>
        <p v-if="draft.footprint.length <= 3" class="inspector__hint">
          只剩 3 个顶点，再删就构不出体了。
        </p>
      </template>

      <ul class="inspector__vertex-list">
        <li v-for="([x, z], index) in draft.footprint" :key="index">
          <button
            type="button"
            :class="{ 'is-active': index === selectedVertex }"
            @click="emit('select-vertex', index)"
          >
            {{ index }}: ({{ x }}, {{ z }})
          </button>
        </li>
      </ul>
    </template>
  </aside>
</template>

<style scoped>
.inspector {
  display: flex;
  width: 300px;
  flex: 0 0 300px;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  overflow-y: auto;
  border: 1px solid var(--editor-border);
  border-radius: 10px;
  background: #fff;
}

.inspector__empty,
.inspector__hint {
  margin: 0;
  color: var(--editor-muted);
  font-size: 12px;
  line-height: 1.6;
}

.inspector__title {
  margin: 0;
  color: var(--editor-ink);
  font-size: 15px;
}

.inspector__subhead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--editor-ink);
  font-size: 12px;
  font-weight: 700;
}

.inspector__field {
  display: block;
  min-width: 0;
  font-size: 12px;
}

.inspector__field span {
  display: block;
  margin-bottom: 4px;
  color: var(--editor-muted);
}

.inspector__field input {
  width: 100%;
  padding: 6px 8px;
  color: var(--editor-ink);
  border: 1px solid var(--editor-border);
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
}

.inspector__row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.inspector__button {
  padding: 8px 12px;
  color: #fff;
  border: 1px solid var(--editor-accent);
  border-radius: 6px;
  background: var(--editor-accent);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.inspector__button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.inspector__button--danger {
  border-color: #b84235;
  background: #b84235;
}

.inspector__link {
  padding: 0;
  color: var(--editor-accent);
  border: 0;
  background: none;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}

.inspector__divider {
  width: 100%;
  margin: 4px 0;
  border: 0;
  border-top: 1px solid var(--editor-border);
}

.inspector__vertex-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.inspector__vertex-list button {
  width: 100%;
  padding: 4px 6px;
  color: var(--editor-muted);
  border: 1px solid transparent;
  border-radius: 4px;
  background: none;
  cursor: pointer;
  font: inherit;
  font-size: 11px;
  text-align: left;
}

.inspector__vertex-list button:hover {
  border-color: var(--editor-border);
}

.inspector__vertex-list button.is-active {
  color: var(--editor-accent);
  border-color: var(--editor-accent);
  background: rgb(30 101 159 / 8%);
}
</style>
