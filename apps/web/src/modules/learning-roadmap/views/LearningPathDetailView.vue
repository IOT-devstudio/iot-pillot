<script setup lang="ts">
/**
 * 学习路线详情（#224，ADR 0006 决策 4/7）。
 *
 * 按顺序展示阶段与资源，资源链接可点击；登录用户可以标记「当前阶段」，
 * 再点同一阶段即撤销。
 *
 * 进度记的是**阶段的稳定 id** 而不是序号：管理员重排阶段后，
 * 你标的那一阶段仍然是你标的那个，不会静默跳到别的阶段上。
 */
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";

import type { RoadmapDetail } from "@iot-pillot/shared-types";

import MarkdownContent from "@/components/markdown/MarkdownContent.vue";
import { fetchMyRoadmap, markRoadmapProgress } from "@/api/roadmap";

import { isAllowedResourceUrl } from "../model";

const route = useRoute();
const router = useRouter();

const roadmap = ref<RoadmapDetail | null>(null);
const loading = ref(false);
const loadError = ref("");
const marking = ref(false);

const roadmapId = computed(() => Number(route.params.id));

/** 当前标注的阶段 id；空串表示还没标记。 */
const myStageId = computed(() => roadmap.value?.my_stage_id ?? "");

async function load(): Promise<void> {
  if (!Number.isInteger(roadmapId.value) || roadmapId.value <= 0) {
    loadError.value = "路线不存在";
    return;
  }
  loading.value = true;
  loadError.value = "";
  try {
    roadmap.value = await fetchMyRoadmap(roadmapId.value);
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : "加载学习路线失败";
  } finally {
    loading.value = false;
  }
}

/**
 * 点「标记当前阶段」。
 *
 * 点同一个阶段 = 撤销（传空串）：这样只有一个按钮语义，
 * 用户不需要理解「标记」与「取消」是两个不同操作。
 */
async function toggleStage(stageId: string): Promise<void> {
  const cur = roadmap.value;
  if (!cur || marking.value) return;

  const next = cur.my_stage_id === stageId ? "" : stageId;
  marking.value = true;
  try {
    const resp = await markRoadmapProgress(cur.id, next);
    // 用后端返回的实际状态更新，而不是本地乐观值：
    // 撤销可能因为并发点击而拿到不同结果，以服务端为准最不容易骗人。
    cur.my_stage_id = resp.stage_id;
    ElMessage.success(next === "" ? "已取消标记" : "已标记为当前阶段");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "操作失败，请重试");
  } finally {
    marking.value = false;
  }
}

function back(): void {
  void router.push("/user/learning-paths");
}

function openResource(url: string): void {
  // 与渲染层同一份白名单：不合规的链接不打开（服务端也拒绝保存这类链接）。
  if (!isAllowedResourceUrl(url)) {
    ElMessage.warning("该链接不合规，无法打开");
    return;
  }
  window.open(url, "_blank", "noopener,noreferrer");
}

onMounted(() => {
  void load();
});
</script>

<template>
  <section class="path">
    <p v-if="loadError" class="path__error" role="alert">{{ loadError }}</p>
    <p v-else-if="loading && !roadmap" class="path__loading">加载中…</p>

    <template v-else-if="roadmap">
      <header class="path__head">
        <el-button link @click="back">← 返回列表</el-button>
        <h1 class="path__title">{{ roadmap.title }}</h1>
        <p class="path__meta">
          {{ roadmap.direction_code }} · {{ roadmap.stage_count }} 个阶段
          <span v-if="myStageId" class="path__marked">已标记当前阶段</span>
        </p>
        <MarkdownContent
          v-if="roadmap.body"
          :source="roadmap.body"
          class="path__body"
        />
      </header>

      <ol class="path__stages">
        <li
          v-for="(stage, index) in roadmap.stages"
          :key="stage.id"
          class="path-stage"
          :class="{ 'path-stage--current': myStageId === stage.id }"
        >
          <div class="path-stage__head">
            <span class="path-stage__no">{{ index + 1 }}</span>
            <div class="path-stage__titles">
              <h2 class="path-stage__title">{{ stage.title }}</h2>
              <p v-if="stage.summary" class="path-stage__summary">{{ stage.summary }}</p>
            </div>
            <el-button
              size="small"
              :type="myStageId === stage.id ? 'success' : 'default'"
              :loading="marking"
              @click="toggleStage(stage.id)"
            >
              {{ myStageId === stage.id ? "当前阶段" : "标记为当前" }}
            </el-button>
          </div>

          <ul v-if="stage.resources.length" class="path-stage__resources">
            <li v-for="(res, ri) in stage.resources" :key="ri">
              <a
                class="path-stage__link"
                :class="{ 'is-invalid': !isAllowedResourceUrl(res.url) }"
                href="#"
                @click.prevent="openResource(res.url)"
              >
                {{ res.title }}
              </a>
            </li>
          </ul>
          <p v-else class="path-stage__no-res">这个阶段还没有资源</p>
        </li>
      </ol>
    </template>
  </section>
</template>

<style scoped>
.path {
  max-width: 820px;
  margin: 0 auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.path__title {
  margin: 8px 0 6px;
  font-size: 26px;
}

.path__meta {
  margin: 0 0 12px;
  font-size: 13px;
  color: #6b7280;
  display: flex;
  align-items: center;
  gap: 8px;
}

.path__marked {
  font-size: 12px;
  color: #047857;
  background: #ecfdf5;
  border-radius: 999px;
  padding: 2px 8px;
}

.path__body {
  font-size: 14px;
  color: #374151;
}

.path__stages {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.path-stage {
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 14px;
  background: #fff;
}

.path-stage--current {
  border-color: #34d399;
  box-shadow: 0 0 0 3px rgb(52 211 153 / 12%);
}

.path-stage__head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.path-stage__no {
  flex: none;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: #eef2ff;
  color: #4338ca;
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
}

.path-stage__titles {
  flex: 1;
  min-width: 0;
}

.path-stage__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.path-stage__summary {
  margin: 4px 0 0;
  font-size: 13px;
  color: #6b7280;
}

.path-stage__resources {
  list-style: none;
  margin: 10px 0 0;
  padding: 0 0 0 38px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.path-stage__link {
  font-size: 13px;
  color: #4338ca;
  text-decoration: none;
}

.path-stage__link:hover {
  text-decoration: underline;
}

.path-stage__link.is-invalid {
  color: #b91c1c;
  text-decoration: line-through;
}

.path-stage__no-res {
  margin: 10px 0 0 38px;
  font-size: 12px;
  color: #9ca3af;
}

.path__loading,
.path__error {
  margin: 0;
  padding: 40px;
  text-align: center;
  font-size: 14px;
  color: #9ca3af;
}

.path__error {
  color: #b91c1c;
}

@media (max-width: 640px) {
  .path-stage__head {
    flex-wrap: wrap;
  }

  .path-stage__resources {
    padding-left: 0;
  }

  .path-stage__no-res {
    margin-left: 0;
  }
}
</style>
