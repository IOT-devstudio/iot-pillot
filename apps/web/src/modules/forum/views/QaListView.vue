<script setup lang="ts">
/**
 * 招新问答列表（#219，只读）。
 *
 * 闭环：拉方向列表（构建筛选下拉）→ 分页拉帖子 → 方向/关键词筛选 → 点进详情。
 * 不渲染正文：列表只给摘要与元信息（ADR 0005 决策 8：列表不唯一需要 20 个 Markdown VNode）。
 */
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import {
  fetchDirections,
  type RecruitmentDirection,
} from "@/api/recruitment";
import { listQaPosts, type QaPostSummary } from "@/api/qa";
import {
  formatDateTime,
  formatDay,
  postExcerpt,
  replyRoleLabel,
} from "../model";

const router = useRouter();

const posts = ref<QaPostSummary[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;

const loading = ref(false);
const loadError = ref("");

const keyword = ref("");
const directionCode = ref<string | null>(null);
const directions = ref<RecruitmentDirection[]>([]);

const hasActiveFilters = computed(
  () => keyword.value.trim() !== "" || directionCode.value !== null,
);

async function load(nextPage = page.value): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const data = await listQaPosts({
      q: keyword.value,
      direction: directionCode.value,
      page: nextPage,
      page_size: pageSize,
    });
    posts.value = data.items;
    total.value = data.total;
    page.value = data.page;
  } catch (error) {
    loadError.value =
      error instanceof Error ? error.message : "加载失败，请稍后重试";
    if (!(error instanceof Error)) console.error(error);
  } finally {
    loading.value = false;
  }
}

function applyFilters(): void {
  void load(1);
}

function resetFilters(): void {
  keyword.value = "";
  directionCode.value = null;
  void load(1);
}

function openPost(id: number): void {
  void router.push(`/user/qa/${id}`);
}

onMounted(async () => {
  // 方向下拉是筛选条件，独立于帖子列表加载；拉不到也不阻塞列表。
  try {
    const res = await fetchDirections();
    directions.value = res.items;
  } catch {
    // 方向列表失败仅影响筛选下拉，列表仍可加载。
  }
  await load();
});
</script>

<template>
  <section class="qa-list">
    <PageHeader
      eyebrow="招新问答"
      title="招新问答"
      description="成员与报名者整理的常见问题。按方向筛选，点开查看完整问答。"
      :meta="loading ? '加载中…' : `共 ${total} 条`"
    />

    <div class="qa-list__filters">
      <el-input
        v-model="keyword"
        placeholder="搜索标题或正文关键词"
        clearable
        class="qa-list__search"
        @keyup.enter="applyFilters"
        @clear="applyFilters"
      />
      <el-select
        v-model="directionCode"
        placeholder="全部方向"
        clearable
        class="qa-list__direction"
        @change="applyFilters"
      >
        <el-option label="全部方向" :value="null" />
        <el-option
          v-for="d in directions"
          :key="d.code"
          :label="d.name"
          :value="d.code"
        />
      </el-select>
      <el-button @click="resetFilters" :disabled="!hasActiveFilters">重置</el-button>
    </div>

    <p v-if="loadError" class="qa-list__error" role="alert">{{ loadError }}</p>

    <ul v-else-if="posts.length" class="qa-list__items">
      <li
        v-for="post in posts"
        :key="post.id"
        class="qa-card"
        @click="openPost(post.id)"
      >
        <div class="qa-card__head">
          <h2 class="qa-card__title">{{ post.title || "（无标题）" }}</h2>
          <span v-if="post.is_resolved" class="qa-card__badge qa-card__badge--resolved">
            已解决
          </span>
        </div>
        <p class="qa-card__excerpt">{{ postExcerpt(post.body) }}</p>
        <div class="qa-card__meta">
          <span>{{ post.author_name || "匿名" }}</span>
          <span v-if="post.direction_code">· {{ post.direction_code }}</span>
          <span>· {{ post.reply_count }} 回复</span>
          <span v-if="post.has_image" class="qa-card__img" title="含图片">图</span>
          <span class="qa-card__time">· 活跃于 {{ formatDay(post.last_active_at) }}</span>
        </div>
      </li>
    </ul>

    <p v-else-if="!loading" class="qa-list__empty">暂无问答</p>

    <el-pagination
      v-if="total > pageSize"
      class="qa-list__pager"
      layout="prev, pager, next"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="load"
    />
  </section>
</template>

<style scoped>
.qa-list {
  max-width: 860px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
.qa-list__filters {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin: 16px 0 20px;
}
.qa-list__search {
  flex: 1 1 240px;
}
.qa-list__direction {
  width: 180px;
}
.qa-list__error {
  color: #c0392b;
  margin: 12px 0;
}
.qa-list__items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 12px;
}
.qa-card {
  border: 1px solid #e3e8ef;
  border-radius: 10px;
  padding: 16px;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.qa-card:hover {
  border-color: #4c8bf5;
  box-shadow: 0 2px 10px rgba(76, 139, 245, 0.12);
}
.qa-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.qa-card__title {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
}
.qa-card__badge {
  font-size: 12px;
  padding: 1px 8px;
  border-radius: 999px;
}
.qa-card__badge--resolved {
  background: #e6f7ed;
  color: #1a8a4f;
}
.qa-card__excerpt {
  color: #5b6b82;
  margin: 8px 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.qa-card__meta {
  font-size: 13px;
  color: #8a97ab;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.qa-card__img {
  display: inline-block;
  border: 1px solid #c9d3e0;
  border-radius: 4px;
  padding: 0 4px;
  font-size: 11px;
  color: #4c8bf5;
}
.qa-list__empty {
  text-align: center;
  color: #8a97ab;
  padding: 48px 0;
}
.qa-list__pager {
  justify-content: center;
  margin-top: 24px;
}
</style>
