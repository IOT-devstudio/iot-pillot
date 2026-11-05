<script setup lang="ts">
/**
 * 招新问答详情（#219，只读）。
 *
 * 闭合：列表点进来 → 拉详情 → 渲染正文（MarkdownContent，唯一正文渲染组件，
 * 见 ADR 0005 决策 6）+ 回复列表（官方回答已由后端置顶，这里只展示标签）。
 * 软删或不存在后端回 404，统一落到 loadError 文案。
 */
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import MarkdownContent from "@/components/markdown/MarkdownContent.vue";
import { fetchQaPost, type QaPostDetail } from "@/api/qa";
import { formatDateTime, replyRoleLabel } from "../model";

const route = useRoute();
const router = useRouter();

const post = ref<QaPostDetail | null>(null);
const loading = ref(false);
const loadError = ref("");

const postId = Number(route.params.id);

async function load(): Promise<void> {
  if (!Number.isInteger(postId) || postId <= 0) {
    loadError.value = "无效的问答编号";
    return;
  }
  loading.value = true;
  loadError.value = "";
  try {
    post.value = await fetchQaPost(postId);
  } catch (error) {
    // 后端对软删/越权统一回 404（防枚举），前端给中性文案。
    loadError.value =
      error instanceof Error ? "问答不存在或已被移除" : "加载失败，请稍后重试";
    if (!(error instanceof Error)) console.error(error);
  } finally {
    loading.value = false;
  }
}

function back(): void {
  void router.push("/user/qa");
}

onMounted(load);
</script>

<template>
  <section class="qa-detail">
    <PageHeader
      eyebrow="招新问答"
      :title="post ? (post.title || '（无标题）') : '问答详情'"
      :meta="loading ? '加载中…' : undefined"
    />

    <p v-if="loadError" class="qa-detail__error" role="alert">{{ loadError }}</p>

    <template v-else-if="post">
      <div class="qa-detail__meta">
        <span>作者 #{{ post.author_id }}</span>
        <span v-if="post.direction_code">· {{ post.direction_code }}</span>
        <span v-if="post.is_resolved" class="qa-detail__resolved">已解决</span>
        <span>· 发布于 {{ formatDateTime(post.created_at) }}</span>
      </div>

      <article class="qa-detail__body">
        <MarkdownContent :source="post.body" />
      </article>

      <h2 class="qa-detail__replies-title">
        回复（{{ post.replies.length }}）
      </h2>
      <ul v-if="post.replies.length" class="qa-detail__replies">
        <li
          v-for="reply in post.replies"
          :key="reply.id"
          class="qa-reply"
          :class="{ 'qa-reply--official': reply.is_official }"
        >
          <div class="qa-reply__head">
            <span class="qa-reply__role" :class="{ 'qa-reply__role--official': reply.is_official }">
              {{ replyRoleLabel(reply.is_official) }}
            </span>
            <span class="qa-reply__author">{{ reply.author_id }}</span>
            <span class="qa-reply__time">{{ formatDateTime(reply.created_at) }}</span>
          </div>
          <MarkdownContent :source="reply.body" />
        </li>
      </ul>
      <p v-else class="qa-detail__empty">还没有回复</p>

      <el-button class="qa-detail__back" @click="back">返回列表</el-button>
    </template>
  </section>
</template>

<style scoped>
.qa-detail {
  max-width: 760px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
.qa-detail__meta {
  font-size: 13px;
  color: #8a97ab;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.qa-detail__resolved {
  background: #e6f7ed;
  color: #1a8a4f;
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 12px;
}
.qa-detail__body {
  border-bottom: 1px solid #eef1f6;
  padding-bottom: 20px;
}
.qa-detail__replies-title {
  font-size: 16px;
  margin: 24px 0 12px;
}
.qa-detail__replies {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 12px;
}
.qa-reply {
  border: 1px solid #e3e8ef;
  border-radius: 10px;
  padding: 14px;
}
.qa-reply--official {
  border-color: #4c8bf5;
  background: #f5f9ff;
}
.qa-reply__head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #8a97ab;
  margin-bottom: 8px;
}
.qa-reply__role {
  font-size: 12px;
  padding: 1px 8px;
  border-radius: 999px;
  background: #eef1f6;
  color: #5b6b82;
}
.qa-reply__role--official {
  background: #4c8bf5;
  color: #fff;
}
.qa-reply__author {
  color: #5b6b82;
}
.qa-detail__empty {
  color: #8a97ab;
  padding: 12px 0;
}
.qa-detail__back {
  margin-top: 24px;
}
.qa-detail__error {
  color: #c0392b;
  margin: 12px 0;
}
</style>
