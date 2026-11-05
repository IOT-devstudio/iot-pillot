<script setup lang="ts">
/**
 * 学习路线管理端（#223，ADR 0006）。
 *
 * 单页整体编辑：左侧选路线，右侧改标题/正文/阶段列表，一次「保存」全量提交。
 * 不做草稿态（ADR 0006 决策 5）：写完就是要给用户看的，草稿会让每个查询
 * 多一个 WHERE，也让「为什么用户看不到」变成需要排查的问题。
 *
 * 阶段与资源排序用上下移动按钮而不是拖拽（决策 6）：仓内无拖拽库，
 * 自写一套拖拽逻辑的复杂度远大于它带来的体验收益。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import type {
  RoadmapDetail,
  RoadmapDirectionOption,
  RoadmapListItem,
  RoadmapStage,
} from "@iot-pillot/shared-types";

import MarkdownContent from "@/components/markdown/MarkdownContent.vue";
import { createRoadmap, deleteRoadmap, fetchRoadmap, listRoadmaps, updateRoadmap } from "@/api/roadmap";

import {
  MAX_RESOURCES_PER_STAGE,
  MAX_STAGES,
  isAllowedResourceUrl,
  moveResource,
  moveStage,
  newResource,
  newStage,
  validateRoadmapDraft,
} from "../model";

const items = ref<RoadmapListItem[]>([]);
const directions = ref<RoadmapDirectionOption[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const directionFilter = ref<string | null>(null);
const loading = ref(false);
const loadError = ref("");

/** 当前正在编辑的路线；null 表示未选中（右侧显示空态）。 */
const draft = ref<RoadmapDetail | null>(null);
/** 新建模式：还没有 id，保存走 POST。 */
const isNew = ref(false);
const saving = ref(false);
const previewVisible = ref(false);

const stages = computed<RoadmapStage[]>(() => draft.value?.stages ?? []);

async function loadList(): Promise<void> {
  loading.value = true;
  loadError.value = "";
  try {
    const resp = await listRoadmaps({
      direction: directionFilter.value,
      page: page.value,
      page_size: pageSize,
    });
    items.value = resp.items;
    directions.value = resp.directions;
    total.value = resp.total;
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : "加载学习路线失败";
  } finally {
    loading.value = false;
  }
}

async function selectRoadmap(id: number): Promise<void> {
  if (saving.value) return;
  loading.value = true;
  try {
    draft.value = await fetchRoadmap(id);
    isNew.value = false;
    previewVisible.value = false;
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "加载详情失败");
  } finally {
    loading.value = false;
  }
}

function startCreate(): void {
  // 方向沿用当前筛选，省一次选择；筛选为空时留空由用户选。
  draft.value = {
    id: 0,
    title: "",
    body: "",
    direction_code: directionFilter.value ?? "",
    stages: [newStage()],
    version: 0,
    stage_count: 0,
    created_at: "",
    updated_at: "",
  };
  isNew.value = true;
  previewVisible.value = false;
}

function closeDraft(): void {
  draft.value = null;
  isNew.value = false;
}

/* ------------------------------------------------------------------ *
 * 阶段与资源编辑
 * ------------------------------------------------------------------ */

function addStage(): void {
  if (!draft.value) return;
  if (draft.value.stages.length >= MAX_STAGES) {
    ElMessage.warning(`阶段最多 ${MAX_STAGES} 个`);
    return;
  }
  draft.value.stages.push(newStage());
}

function removeStage(index: number): void {
  draft.value?.stages.splice(index, 1);
}

function shiftStage(index: number, delta: -1 | 1): void {
  if (!draft.value) return;
  // 整体替换数组而不是就地改：Vue 能稳定重渲染，也让「不改入参」的模型函数
  // 与响应式数组各司其职。
  draft.value.stages = moveStage(draft.value.stages, index, delta);
}

function addResource(stageIndex: number): void {
  const stage = draft.value?.stages[stageIndex];
  if (!stage) return;
  if (stage.resources.length >= MAX_RESOURCES_PER_STAGE) {
    ElMessage.warning(`每个阶段最多 ${MAX_RESOURCES_PER_STAGE} 个资源`);
    return;
  }
  // 新建资源时同样不生成 id：资源本身没有 id（只有阶段有）。
  stage.resources.push(newResource());
}

function removeResource(stageIndex: number, resIndex: number): void {
  draft.value?.stages[stageIndex]?.resources.splice(resIndex, 1);
}

function shiftResource(stageIndex: number, resIndex: number, delta: -1 | 1): void {
  const stage = draft.value?.stages[stageIndex];
  if (!stage) return;
  stage.resources = moveResource(stage.resources, resIndex, delta);
}

/* ------------------------------------------------------------------ *
 * 保存 / 删除
 * ------------------------------------------------------------------ */

async function save(): Promise<void> {
  const cur = draft.value;
  if (!cur) return;

  const issue = validateRoadmapDraft({
    title: cur.title,
    directionCode: cur.direction_code,
    stages: cur.stages,
  });
  if (issue) {
    ElMessage.warning(issue.message);
    return;
  }

  saving.value = true;
  try {
    if (isNew.value) {
      // 新建不传 version（后端从 1 起）。
      const created = await createRoadmap({
        title: cur.title.trim(),
        body: cur.body,
        direction_code: cur.direction_code,
        stages: cur.stages,
      });
      draft.value = created;
      isNew.value = false;
      ElMessage.success("已创建");
    } else {
      const saved = await updateRoadmap(cur.id, {
        title: cur.title.trim(),
        body: cur.body,
        direction_code: cur.direction_code,
        stages: cur.stages,
        // 必须带当前版本：被别人先保存过时后端会拒绝，而不是默默覆盖别人的改动。
        version: cur.version,
      });
      draft.value = saved;
      ElMessage.success("已保存");
    }
    await loadList();
  } catch (error) {
    // 版本冲突的提示由后端给出可读文案，这里原样透出，
    // 用户才知道该「刷新后重试」而不是反复点保存。
    ElMessage.error(error instanceof Error ? error.message : "保存失败");
  } finally {
    saving.value = false;
  }
}

async function remove(): Promise<void> {
  const cur = draft.value;
  if (!cur || isNew.value) return;
  try {
    await ElMessageBox.confirm(
      `删除后用户将看不到「${cur.title}」，历史学习进度会保留。确定删除？`,
      "确认删除",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" },
    );
  } catch {
    return; // 用户取消
  }
  try {
    await deleteRoadmap(cur.id);
    ElMessage.success("已删除");
    closeDraft();
    await loadList();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "删除失败");
  }
}

function applyFilter(): void {
  page.value = 1;
  void loadList();
}

onMounted(() => {
  void loadList();
});
</script>

<template>
  <section class="roadmap">
    <header class="roadmap__head">
      <div>
        <p class="roadmap__eyebrow">工作室</p>
        <h1 class="roadmap__title">学习路线</h1>
        <p class="roadmap__desc">
          按方向维护阶段与资源。阶段顺序会影响用户看到的顺序，但用户的学习进度记的是阶段本身。
        </p>
      </div>
      <div class="roadmap__head-actions">
        <el-select
          v-model="directionFilter"
          placeholder="全部方向"
          clearable
          class="roadmap__direction"
          @change="applyFilter"
        >
          <el-option label="全部方向" :value="null" />
          <el-option
            v-for="d in directions"
            :key="d.code"
            :label="`${d.name}（${d.count}）`"
            :value="d.code"
          />
        </el-select>
        <el-button type="primary" @click="startCreate">新建路线</el-button>
      </div>
    </header>

    <div class="roadmap__body">
      <!-- 左：列表 -->
      <aside class="roadmap__list">
        <p v-if="loadError" class="roadmap__error" role="alert">{{ loadError }}</p>
        <p v-else-if="!items.length && !loading" class="roadmap__empty">暂无学习路线</p>
        <ul v-else class="roadmap__items">
          <li
            v-for="item in items"
            :key="item.id"
            class="roadmap__item"
            :class="{ 'roadmap__item--active': draft && !isNew && draft.id === item.id }"
            @click="selectRoadmap(item.id)"
          >
            <h2 class="roadmap__item-title">{{ item.title }}</h2>
            <p class="roadmap__item-meta">
              {{ item.direction_code }} · {{ item.stage_count }} 个阶段
            </p>
          </li>
        </ul>
        <el-pagination
          v-if="total > pageSize"
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          class="roadmap__pager"
          @current-change="loadList"
        />
      </aside>

      <!-- 右：编辑 -->
      <div class="roadmap__editor">
        <p v-if="!draft" class="roadmap__placeholder">从左侧选择一条路线，或新建一条</p>

        <template v-else>
          <div class="roadmap__field">
            <label class="roadmap__label">标题</label>
            <el-input v-model="draft.title" maxlength="200" show-word-limit />
          </div>

          <div class="roadmap__field">
            <label class="roadmap__label">方向</label>
            <el-select v-model="draft.direction_code" placeholder="选择方向" filterable allow-create>
              <el-option
                v-for="d in directions"
                :key="d.code"
                :label="d.name"
                :value="d.code"
              />
            </el-select>
          </div>

          <div class="roadmap__field">
            <div class="roadmap__label-row">
              <label class="roadmap__label">正文</label>
              <el-button size="small" @click="previewVisible = !previewVisible">
                {{ previewVisible ? "继续编辑" : "预览" }}
              </el-button>
            </div>
            <MarkdownContent v-if="previewVisible" :source="draft.body" class="roadmap__preview" />
            <el-input v-else v-model="draft.body" type="textarea" :rows="8" placeholder="支持 Markdown" />
          </div>

          <div class="roadmap__stages">
            <div class="roadmap__label-row">
              <label class="roadmap__label">阶段（{{ stages.length }}/{{ MAX_STAGES }}）</label>
              <el-button size="small" @click="addStage">新增阶段</el-button>
            </div>

            <div v-for="(stage, si) in stages" :key="si" class="roadmap__stage">
              <div class="roadmap__stage-head">
                <span class="roadmap__stage-no">第 {{ si + 1 }} 阶段</span>
                <div class="roadmap__stage-tools">
                  <el-button size="small" :disabled="si === 0" @click="shiftStage(si, -1)">上移</el-button>
                  <el-button
                    size="small"
                    :disabled="si === stages.length - 1"
                    @click="shiftStage(si, 1)"
                  >
                    下移
                  </el-button>
                  <el-button size="small" type="danger" plain @click="removeStage(si)">删除</el-button>
                </div>
              </div>

              <el-input v-model="stage.title" placeholder="阶段标题" class="roadmap__stage-input" />
              <el-input
                v-model="stage.summary"
                placeholder="一句话说明"
                class="roadmap__stage-input"
              />

              <div class="roadmap__resources">
                <div
                  v-for="(res, ri) in stage.resources"
                  :key="ri"
                  class="roadmap__resource"
                >
                  <el-input v-model="res.title" placeholder="资源标题" class="roadmap__res-title" />
                  <el-input
                    v-model="res.url"
                    placeholder="https://… 或 /站内路径"
                    class="roadmap__res-url"
                    :class="{ 'is-invalid': res.url && !isAllowedResourceUrl(res.url) }"
                  />
                  <div class="roadmap__res-tools">
                    <el-button size="small" :disabled="ri === 0" @click="shiftResource(si, ri, -1)">
                      上移
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="ri === stage.resources.length - 1"
                      @click="shiftResource(si, ri, 1)"
                    >
                      下移
                    </el-button>
                    <el-button size="small" type="danger" plain @click="removeResource(si, ri)">
                      删除
                    </el-button>
                  </div>
                </div>
                <el-button size="small" @click="addResource(si)">新增资源</el-button>
                <p
                  v-if="
                    stage.resources.some((r) => r.url && !isAllowedResourceUrl(r.url))
                  "
                  class="roadmap__hint"
                >
                  只允许 http/https 或站内相对路径
                </p>
              </div>
            </div>
          </div>

          <div class="roadmap__actions">
            <el-button @click="closeDraft">关闭</el-button>
            <el-button v-if="!isNew" type="danger" plain @click="remove">删除</el-button>
            <el-button type="primary" :loading="saving" @click="save">
              {{ isNew ? "创建" : "保存" }}
            </el-button>
          </div>
          <p v-if="!isNew" class="roadmap__version">当前版本 v{{ draft.version }}</p>
        </template>
      </div>
    </div>
  </section>
</template>

<style scoped>
.roadmap {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.roadmap__head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.roadmap__eyebrow {
  margin: 0 0 4px;
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #8a8f99;
}

.roadmap__title {
  margin: 0 0 6px;
  font-size: 22px;
}

.roadmap__desc {
  margin: 0;
  max-width: 560px;
  font-size: 13px;
  color: #6b7280;
}

.roadmap__head-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.roadmap__direction {
  width: 180px;
}

.roadmap__body {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 16px;
  align-items: start;
}

.roadmap__list {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 8px;
  background: #fff;
}

.roadmap__items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.roadmap__item {
  padding: 10px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid transparent;
}

.roadmap__item:hover {
  background: #f9fafb;
}

.roadmap__item--active {
  background: #eef2ff;
  border-color: #c7d2fe;
}

.roadmap__item-title {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 600;
}

.roadmap__item-meta {
  margin: 0;
  font-size: 12px;
  color: #6b7280;
}

.roadmap__pager {
  margin-top: 8px;
}

.roadmap__editor {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 16px;
  background: #fff;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.roadmap__placeholder,
.roadmap__empty {
  margin: 0;
  padding: 24px;
  text-align: center;
  color: #9ca3af;
  font-size: 13px;
}

.roadmap__error {
  margin: 0;
  padding: 12px;
  color: #b91c1c;
  font-size: 13px;
}

.roadmap__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.roadmap__label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.roadmap__label {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}

.roadmap__preview {
  border: 1px dashed #d1d5db;
  border-radius: 6px;
  padding: 12px;
  min-height: 120px;
}

.roadmap__stages {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.roadmap__stage {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: #fcfcfd;
}

.roadmap__stage-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.roadmap__stage-no {
  font-size: 13px;
  font-weight: 600;
  color: #4338ca;
}

.roadmap__stage-tools,
.roadmap__res-tools {
  display: flex;
  gap: 6px;
}

.roadmap__resources {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-left: 12px;
  border-left: 2px solid #e5e7eb;
}

.roadmap__resource {
  display: grid;
  grid-template-columns: 1fr 1.4fr auto;
  gap: 6px;
  align-items: start;
}

.roadmap__res-url.is-invalid :deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px #ef4444 inset;
}

.roadmap__hint {
  margin: 0;
  font-size: 12px;
  color: #b91c1c;
}

.roadmap__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid #e5e7eb;
}

.roadmap__version {
  margin: 0;
  text-align: right;
  font-size: 12px;
  color: #9ca3af;
}

@media (max-width: 900px) {
  .roadmap__body {
    grid-template-columns: 1fr;
  }

  .roadmap__resource {
    grid-template-columns: 1fr;
  }
}
</style>
