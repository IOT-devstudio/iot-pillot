<script setup lang="ts">
/**
 * 学习路线列表（#224，ADR 0006 决策 7）。
 *
 * 卡片只显示：标题 + 方向名 + 阶段数 + 正文摘要（ADR 0005 的 markdownExcerpt）。
 * **不显示阶段标题、不显示进度**：
 *   - 阶段标题要求列表接口把整个 stages 列读出来，与「列表页不渲染正文」相反；
 *   - 进度只在详情页表达，列表页每行都带一个进度标记会让「我只想知道有哪些路线」
 *     这件事变成视觉噪音。
 */
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import type { RoadmapDirectionOption, RoadmapListItem } from "@iot-pillot/shared-types";

import { listMyRoadmaps } from "@/api/roadmap";

import { excerptOf } from "../model";

const router = useRouter();

const items = ref<RoadmapListItem[]>([]);
const directions = ref<RoadmapDirectionOption[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 12;
const direction = ref<string | null>(null);
const loading = ref(false);
const loadError = ref("");

/**
 * 两种空状态的判断（AC 明确要求区分）。
 *
 * 「没有任何路线」与「这个方向下没有路线」对用户是两件事：
 * 前者该提示去找管理员，后者该提示换个方向看看。
 */
const isFiltered = computed(() => direction.value !== null);
const emptyText = computed(() =>
  isFiltered.value ? "该方向暂无路线" : "暂无学习路线",
);

async function load(): Promise<void> {
  loading.value = true;
  loadError.value = "";
  try {
    const resp = await listMyRoadmaps({
      direction: direction.value,
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

function applyFilter(): void {
  page.value = 1;
  void load();
}

function openRoadmap(id: number): void {
  void router.push(`/user/learning-paths/${id}`);
}

onMounted(() => {
  void load();
});
</script>

<template>
  <section class="paths">
    <header class="paths__head">
      <p class="paths__eyebrow">学习路线</p>
      <h1 class="paths__title">从这里开始</h1>
      <p class="paths__desc">按方向整理的阶段式学习清单，点开查看每个阶段的资源。</p>
    </header>

    <div class="paths__filters">
      <el-button
        :type="direction === null ? 'primary' : 'default'"
        size="small"
        @click="direction = null; applyFilter()"
      >
        全部
      </el-button>
      <el-button
        v-for="d in directions"
        :key="d.code"
        :type="direction === d.code ? 'primary' : 'default'"
        size="small"
        @click="direction = d.code; applyFilter()"
      >
        {{ d.name }}（{{ d.count }}）
      </el-button>
    </div>

    <p v-if="loadError" class="paths__error" role="alert">{{ loadError }}</p>

    <ul v-else-if="items.length" class="paths__grid">
      <li
        v-for="item in items"
        :key="item.id"
        class="paths-card"
        @click="openRoadmap(item.id)"
      >
        <div class="paths-card__head">
          <h2 class="paths-card__title">{{ item.title }}</h2>
          <span class="paths-card__direction">{{ item.direction_name }}</span>
        </div>
        <p class="paths-card__excerpt">{{ excerptOf(item.excerpt) }}</p>
        <p class="paths-card__meta">{{ item.stage_count }} 个阶段</p>
      </li>
    </ul>

    <p v-else-if="!loading" class="paths__empty">{{ emptyText }}</p>

    <el-pagination
      v-if="total > pageSize"
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="prev, pager, next"
      class="paths__pager"
      @current-change="load"
    />
  </section>
</template>

<style scoped>
.paths {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 24px;
  max-width: 1100px;
  margin: 0 auto;
}

.paths__eyebrow {
  margin: 0 0 4px;
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #8a8f99;
}

.paths__title {
  margin: 0 0 6px;
  font-size: 24px;
}

.paths__desc {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
}

.paths__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.paths__grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}

.paths-card {
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  padding: 14px;
  background: #fff;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.paths-card:hover {
  border-color: #c7d2fe;
  box-shadow: 0 2px 10px rgb(79 70 229 / 8%);
}

.paths-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.paths-card__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.paths-card__direction {
  flex: none;
  font-size: 12px;
  color: #4338ca;
  background: #eef2ff;
  border-radius: 999px;
  padding: 2px 8px;
}

.paths-card__excerpt {
  margin: 0;
  font-size: 13px;
  color: #4b5563;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.paths-card__meta {
  margin: 0;
  font-size: 12px;
  color: #9ca3af;
}

.paths__empty,
.paths__error {
  margin: 0;
  padding: 40px;
  text-align: center;
  font-size: 14px;
  color: #9ca3af;
}

.paths__error {
  color: #b91c1c;
}

.paths__pager {
  justify-content: center;
}
</style>
