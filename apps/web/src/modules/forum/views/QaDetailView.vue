<script setup lang="ts">
/**
 * 招新问答详情（#219 只读 → #220 增加回复与作者操作）。
 *
 * 闭合：列表点进来 → 拉详情 → 渲染正文（MarkdownContent，唯一正文渲染组件，
 * 见 ADR 0005 决策 6）+ 回复列表（官方回答已由后端置顶，这里只展示标签）。
 * 软删或不存在后端回 404，统一落到 loadError 文案。
 *
 * #220 新增：
 *   - 底部回复框（Markdown 编辑器，支持插图）；
 *   - 作者本人或管理员可编辑/删除帖子与回复；
 *   - 编辑过的内容显示「已编辑」。
 *
 * 权限只用于**隐藏**按钮：真正的判定在后端（无权与不存在都回 404）。
 * 前端根据 /me 的 role 与 author_id 提前收起按钮，避免用户点了才失败。
 */
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import MarkdownContent from "@/components/markdown/MarkdownContent.vue";
import MarkdownEditor from "../components/MarkdownEditor.vue";
import {
  createQaReply,
  deleteQaPost,
  deleteQaReply,
  fetchQaPost,
  type QaPostDetail,
} from "@/api/qa";
import { currentProfile } from "@/modules/user-settings/profileIdentity";
import { formatDateTime, replyRoleLabel } from "../model";

/** 正文上限，与后端 CHECK 一致。 */
const BODY_MAX = 20000;

const route = useRoute();
const router = useRouter();

const post = ref<QaPostDetail | null>(null);
const loading = ref(false);
const loadError = ref("");

const replyBody = ref("");
const replying = ref(false);
const replyEditorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);

const postId = Number(route.params.id);

/** 当前用户 id；未登录为 null。 */
const myId = computed(() => currentProfile.value?.user_id ?? null);
const amAdmin = computed(() => currentProfile.value?.role === "admin");

/** 能否管理某条内容：作者本人或管理员（与后端判定同口径）。 */
function canManage(authorId: number): boolean {
  if (amAdmin.value) return true;
  return myId.value !== null && myId.value === authorId;
}

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

/** 回复框的提交闸门（失败/上传中的图）。 */
const replyBlocked = computed(
  () =>
    replyEditorRef.value?.submitBlockReason !== null &&
    replyEditorRef.value?.submitBlockReason !== undefined,
);
const canReply = computed(
  () =>
    !replying.value &&
    replyBody.value.trim().length > 0 &&
    !replyBlocked.value,
);

async function submitReply(): Promise<void> {
  if (!canReply.value) return;
  const block = replyEditorRef.value?.submitBlockReason ?? null;
  if (block) {
    ElMessage.warning(block);
    return;
  }
  replying.value = true;
  try {
    await createQaReply(postId, { body: replyBody.value });
    replyBody.value = "";
    ElMessage.success("回复成功");
    await load();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "回复失败，请重试");
  } finally {
    replying.value = false;
  }
}

async function removePost(): Promise<void> {
  try {
    await ElMessageBox.confirm(
      "删除后帖子与它的回复都不再显示，且不可撤销。",
      "确认删除帖子",
      { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" },
    );
  } catch {
    return;
  }
  try {
    await deleteQaPost(postId);
    ElMessage.success("已删除");
    void router.replace("/user/qa");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "删除失败，请重试");
  }
}

async function removeReply(replyId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("删除后该回复不再显示。", "确认删除回复", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch {
    return;
  }
  try {
    await deleteQaReply(replyId);
    ElMessage.success("已删除");
    await load();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "删除失败，请重试");
  }
}

function editPost(): void {
  void router.push(`/user/qa/${postId}/edit`);
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
        <span v-if="post.edited_at" class="qa-detail__edited">
          · 已编辑于 {{ formatDateTime(post.edited_at) }}
        </span>
      </div>

      <div v-if="canManage(post.author_id)" class="qa-detail__actions">
        <el-button size="small" @click="editPost">编辑</el-button>
        <el-button size="small" type="danger" plain @click="removePost">
          删除
        </el-button>
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
            <span v-if="reply.edited_at" class="qa-reply__edited">
              已编辑于 {{ formatDateTime(reply.edited_at) }}
            </span>
            <button
              v-if="canManage(reply.author_id)"
              type="button"
              class="qa-reply__del"
              @click="removeReply(reply.id)"
            >
              删除
            </button>
          </div>
          <MarkdownContent :source="reply.body" />
        </li>
      </ul>
      <p v-else class="qa-detail__empty">还没有回复</p>

      <section class="qa-detail__compose">
        <h3 class="qa-detail__compose-title">写回复</h3>
        <MarkdownEditor
          ref="replyEditorRef"
          v-model:body="replyBody"
          :max-length="BODY_MAX"
          :disabled="replying"
          placeholder="写下你的回复，可以贴图片"
        />
        <div class="qa-detail__compose-actions">
          <el-button type="primary" :loading="replying" :disabled="!canReply" @click="submitReply">
            回复
          </el-button>
        </div>
      </section>

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
.qa-detail__edited {
  color: #9aa7ba;
}
.qa-detail__actions {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
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
  flex-wrap: wrap;
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
.qa-reply__edited {
  color: #9aa7ba;
}
.qa-reply__del {
  margin-left: auto;
  border: none;
  background: none;
  padding: 0;
  font-size: 12px;
  color: #c0392b;
  cursor: pointer;
}
.qa-detail__empty {
  color: #8a97ab;
  padding: 12px 0;
}
.qa-detail__compose {
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid #eef1f6;
}
.qa-detail__compose-title {
  font-size: 15px;
  margin: 0 0 10px;
  color: #35507a;
}
.qa-detail__compose-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
.qa-detail__back {
  margin-top: 24px;
}
.qa-detail__error {
  color: #c0392b;
  margin: 12px 0;
}
</style>
