<script setup lang="ts">
/**
 * 招新问答详情（#219 只读 → #220 增加回复与作者操作 → #242 回答收口）。
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
 * #242 回答收口：招新问答的**新建回复仅限管理员**（后端 ADR 0003 决策 8 修订），
 * 所以普通用户不再看到回复框，只留一行说明。这里刻意不用「禁用输入框」：
 * 禁用的输入框暗示「等我有权限就能用」，而这个身份永远不会获得该权限，
 * 一行说明更诚实，也不白占一屏。
 * 同一身份下「提问 / 编辑 / 删除 / 标记已解决」的权限没有被收回，仍然可见可用。
 *
 * 权限只用于**隐藏**按钮：真正的判定在后端（无权与不存在都回 404，
 * 回答则回 403 + 可读文案）。
 */
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import MarkdownContent from "@/components/markdown/MarkdownContent.vue";
import MarkdownEditor from "../components/MarkdownEditor.vue";
import { useForumScope } from "../useForumScope";
import {
  createQaReply,
  deleteQaPost,
  deleteQaReply,
  fetchQaPost,
  resolveQaPost,
  type QaPostDetail,
} from "@/api/qa";
import { currentProfile } from "@/modules/user-settings/profileIdentity";
import { formatDateTime, replyRoleLabel } from "../model";
// 双壳适配（#242）：同一套组件在 AdminLayout 与 SignalShell 下各自成立。
import "../forum.css";

/** 正文上限，与后端 CHECK 一致。 */
const BODY_MAX = 20000;

const route = useRoute();
const router = useRouter();
// 内容域来自路由 meta（#221）：内部论坛复用本组件。
const forum = useForumScope();

const post = ref<QaPostDetail | null>(null);
const loading = ref(false);
const loadError = ref("");

const replyBody = ref("");
const replying = ref(false);
const resolving = ref(false);
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

/**
 * 能否回答（#242）。
 *
 * 招新问答与内部论坛的新建回复都收口到管理员（后端 ADR 0003 决策 8 修订），
 * 因此这里只看角色——作者身份不再带来回答权限。
 * 普通用户在详情页看到一行说明而不是灰掉的输入框：这个身份永远不会获得回答权限。
 */
const canReply = computed(() => amAdmin.value);
/** 普通用户的说明文案：说清「谁能答」+「我还能做什么」，而不是只说不行。 */
const replyRestrictedNote = computed(() =>
  "只有管理员可以回答招新问答。你仍然可以提问、编辑或删除自己的内容，也可以标记已解决。",
);

async function load(): Promise<void> {
  if (!Number.isInteger(postId) || postId <= 0) {
    loadError.value = "无效的问答编号";
    return;
  }
  loading.value = true;
  loadError.value = "";
  try {
    post.value = await fetchQaPost(postId, forum.scope.value);
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
  void router.push(forum.listPath.value);
}

/** 回复框的提交闸门（失败/上传中的图）。 */
const replyBlocked = computed(
  () =>
    replyEditorRef.value?.submitBlockReason !== null &&
    replyEditorRef.value?.submitBlockReason !== undefined,
);
/** 「回复」按钮是否可点：有权限、有正文、没有失败/上传中的图。 */
const canSubmitReply = computed(
  () =>
    canReply.value &&
    !replying.value &&
    replyBody.value.trim().length > 0 &&
    !replyBlocked.value,
);

async function submitReply(): Promise<void> {
  if (!canSubmitReply.value) return;
  const block = replyEditorRef.value?.submitBlockReason ?? null;
  if (block) {
    ElMessage.warning(block);
    return;
  }
  replying.value = true;
  try {
    await createQaReply(postId, { body: replyBody.value }, forum.scope.value);
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
    await deleteQaPost(postId, forum.scope.value);
    ElMessage.success("已删除");
    void router.replace(forum.listPath.value);
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
    await deleteQaReply(replyId, forum.scope.value);
    ElMessage.success("已删除");
    await load();
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "删除失败，请重试");
  }
}

/**
 * 标记 / 取消「已解决」（#221）。
 *
 * 只在招新问答暴露（AC）：内部论坛没有「解决问题」的语义，
 * 数据上两个域都有这两列，但界面只对外部问答开放。
 */
async function toggleResolved(): Promise<void> {
  const cur = post.value;
  if (!cur) return;
  resolving.value = true;
  try {
    // 后端直接回完整详情，能省掉一次刷新请求。
    post.value = await resolveQaPost(postId, !cur.is_resolved, forum.scope.value);
    ElMessage.success(post.value.is_resolved ? "已标记为解决" : "已取消已解决");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "操作失败，请重试");
  } finally {
    resolving.value = false;
  }
}

function editPost(): void {
  void router.push(forum.editPath(postId));
}

onMounted(load);
</script>

<template>
  <section class="qa-detail">
    <PageHeader
      :eyebrow="forum.label.value"
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
        <el-button
          v-if="!forum.isStudio.value"
          size="small"
          :type="post.is_resolved ? 'default' : 'success'"
          :loading="resolving"
          @click="toggleResolved"
        >
          {{ post.is_resolved ? "取消已解决" : "标记已解决" }}
        </el-button>
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

      <!-- 回答收口（#242）：普通用户只看一行说明，不给灰掉的输入框 -->
      <p v-if="!canReply" class="qa-detail__answer-locked" role="note">
        {{ replyRestrictedNote }}
      </p>
      <section v-else class="qa-detail__compose">
        <h3 class="qa-detail__compose-title">写回复</h3>
        <MarkdownEditor
          ref="replyEditorRef"
          v-model:body="replyBody"
          :max-length="BODY_MAX"
          :disabled="replying"
          placeholder="写下你的回复，可以贴图片"
        />
        <div class="qa-detail__compose-actions">
          <el-button
            type="primary"
            :loading="replying"
            :disabled="!canSubmitReply"
            @click="submitReply"
          >
            回复
          </el-button>
        </div>
      </section>

      <el-button class="qa-detail__back" @click="back">返回列表</el-button>
    </template>
  </section>
</template>

<style scoped>
/*
 * 颜色只用语义令牌（--qa-*），取值由 forum.css 按外壳注入（#242）。
 * 版式（宽、间距、字号）未改动——本次只换壳与配色变量。
 */
.qa-detail {
  max-width: 760px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
.qa-detail__meta {
  font-size: 13px;
  color: var(--qa-faint);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.qa-detail__resolved {
  background: color-mix(in srgb, var(--qa-success) 14%, transparent);
  color: var(--qa-success);
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 12px;
}
.qa-detail__edited {
  color: var(--qa-faint);
}
.qa-detail__actions {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
.qa-detail__body {
  border-bottom: 1px solid var(--qa-line-soft);
  padding-bottom: 20px;
}
.qa-detail__replies-title {
  font-size: 16px;
  margin: 24px 0 12px;
  color: var(--qa-ink);
}
.qa-detail__replies {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 12px;
}
.qa-reply {
  border: 1px solid var(--qa-line);
  border-radius: var(--qa-radius);
  padding: 14px;
}
.qa-reply--official {
  border-color: var(--qa-accent);
  background: color-mix(in srgb, var(--qa-accent) 6%, transparent);
}
.qa-reply__head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--qa-faint);
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.qa-reply__role {
  font-size: 12px;
  padding: 1px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--qa-ink) 8%, transparent);
  color: var(--qa-muted);
}
.qa-reply__role--official {
  background: var(--qa-accent);
  color: #fff;
}
.qa-reply__author {
  color: var(--qa-muted);
}
.qa-reply__edited {
  color: var(--qa-faint);
}
.qa-reply__del {
  margin-left: auto;
  border: none;
  background: none;
  padding: 0;
  font-size: 12px;
  color: var(--qa-danger);
  cursor: pointer;
}
.qa-detail__empty {
  color: var(--qa-faint);
  padding: 12px 0;
}
/* 普通用户的回答限制说明（#242）：一行文字，不占一屏，也不用禁用态输入框。 */
.qa-detail__answer-locked {
  margin: 28px 0 0;
  padding-top: 20px;
  border-top: 1px solid var(--qa-line-soft);
  color: var(--qa-muted);
  font-size: 13px;
  line-height: 1.8;
}
.qa-detail__compose {
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid var(--qa-line-soft);
}
.qa-detail__compose-title {
  font-size: 15px;
  margin: 0 0 10px;
  color: var(--qa-ink);
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
  color: var(--qa-danger);
  margin: 12px 0;
}
</style>
