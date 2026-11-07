<script setup lang="ts">
/**
 * 发帖 / 编辑帖子（#220）。
 *
 * 新建与编辑共用一页：字段完全一致（标题 + 方向 + 正文），
 * 拆成正反两个组件只会让校验与提交逻辑重复一遍。
 *
 * 路由参数 id 存在即编辑态；编辑态下方向不可改（后端契约也不接受）。
 *
 * 路径前缀来自 `forum.base`（#242）：管理端入口编辑完帖子后要留在管理台，
 * 不能被送回 /user/qa——那会直接跳出管理台外壳。
 */
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import MarkdownEditor from "../components/MarkdownEditor.vue";
import { useForumScope } from "../useForumScope";
import {
  createQaPost,
  fetchQaPost,
  updateQaPost,
} from "@/api/qa";
import { fetchDirections } from "@/api/recruitment";
// 双壳适配（#242）：同一套组件在 AdminLayout 与 SignalShell 下各自成立。
import "../forum.css";

const route = useRoute();
const router = useRouter();
// 内容域来自路由 meta（#221）。
const forum = useForumScope();

/** 标题上限，与后端 CHECK（char_length <= 200）一致。 */
const TITLE_MAX = 200;
/** 正文上限，与后端 CHECK 一致。 */
const BODY_MAX = 20000;

const editId = computed(() => {
  const raw = route.params.id;
  if (raw === undefined) return null;
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
});
const isEdit = computed(() => editId.value !== null);

const title = ref("");
const body = ref("");
const direction = ref<string>("");
const directions = ref<{ code: string; label: string }[]>([]);
const loading = ref(false);
const submitting = ref(false);
const loadError = ref("");
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);

const titleLength = computed(() => [...title.value].length);
const titleOver = computed(() => titleLength.value > TITLE_MAX);

/** 编辑器内部的闸门（失败/上传中的图）与长度限制一起决定能否提交。 */
const editorBlocked = computed(
  () =>
    editorRef.value?.submitBlockReason !== null &&
    editorRef.value?.submitBlockReason !== undefined,
);
const canSubmit = computed(
  () =>
    !submitting.value &&
    title.value.trim().length > 0 &&
    body.value.trim().length > 0 &&
    !titleOver.value &&
    !editorBlocked.value,
);

async function loadDirections(): Promise<void> {
  try {
    const resp = await fetchDirections();
    // 只列开放中的方向：已关闭的方向不该再被拿来归类新帖。
    directions.value = (resp.items ?? [])
      .filter((d) => d.open)
      .map((d) => ({ code: d.code, label: d.name }));
  } catch {
    // 方向只是筛选辅助信息，取不到不该阻塞发帖：下拉留空，用户照样能发。
    directions.value = [];
  }
}

async function loadExisting(): Promise<void> {
  const id = editId.value;
  if (id === null) return;
  loading.value = true;
  loadError.value = "";
  try {
    const post = await fetchQaPost(id, forum.scope.value);
    title.value = post.title;
    body.value = post.body;
    direction.value = post.direction_code ?? "";
  } catch {
    loadError.value = "帖子不存在或你无权编辑";
  } finally {
    loading.value = false;
  }
}

async function submit(): Promise<void> {
  if (!canSubmit.value) return;

  // 失败图/上传中的图必须拦住，否则正文里没有该图语法，等于静默丢图。
  const block = editorRef.value?.submitBlockReason ?? null;
  if (block) {
    ElMessage.warning(block);
    return;
  }

  submitting.value = true;
  try {
    const id = editId.value;
    if (id === null) {
      const created = await createQaPost(
        {
          title: title.value.trim(),
          body: body.value,
          // 没选就不传字段：让服务端走「资料方向 → 最新报名 → 留空」回落，
          // 用户不该感知这条规则（AC 要求）。
          ...(direction.value ? { direction_code: direction.value } : {}),
        },
        forum.scope.value,
      );
      ElMessage.success("发布成功");
      void router.replace(forum.detailPath(created.id));
    } else {
      await updateQaPost(
        id,
        { title: title.value.trim(), body: body.value },
        forum.scope.value,
      );
      ElMessage.success("已保存");
      void router.replace(forum.detailPath(id));
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "提交失败，请重试");
  } finally {
    submitting.value = false;
  }
}

async function cancel(): Promise<void> {
  if (title.value.trim() || body.value.trim()) {
    try {
      await ElMessageBox.confirm("放弃当前内容？已填写的内容不会保存。", "确认离开", {
        confirmButtonText: "放弃",
        cancelButtonText: "继续编辑",
        type: "warning",
      });
    } catch {
      return;
    }
  }
  const id = editId.value;
  void router.push(id === null ? forum.listPath.value : forum.detailPath(id));
}

onMounted(async () => {
  await Promise.all([loadDirections(), loadExisting()]);
});
</script>

<template>
  <section class="qa-compose">
    <!-- 品牌色柔光晕（与 /user/home、/user/settings、/user/about 同一层）。 -->
    <div class="glass-backdrop" aria-hidden="true"></div>

    <PageHeader
      :eyebrow="forum.label.value"
      :title="isEdit ? '编辑帖子' : '提问'"
      :meta="loading ? '加载中…' : undefined"
    />

    <p v-if="loadError" class="qa-compose__error" role="alert">{{ loadError }}</p>

    <template v-else>
      <label class="qa-compose__label" for="qa-title">标题</label>
      <el-input
        id="qa-title"
        v-model="title"
        placeholder="一句话说清你的问题"
        :disabled="submitting"
        maxlength="256"
        show-word-limit
      />
      <p v-if="titleOver" class="qa-compose__error" role="alert">
        标题最多 {{ TITLE_MAX }} 个字符，当前 {{ titleLength }} 个
      </p>

      <template v-if="!isEdit">
        <label class="qa-compose__label" for="qa-direction">
          方向 <span class="qa-compose__optional">（选填）</span>
        </label>
        <el-select
          id="qa-direction"
          v-model="direction"
          placeholder="不选则由系统按你的资料自动归类"
          clearable
          :disabled="submitting"
          style="width: 100%"
        >
          <el-option
            v-for="d in directions"
            :key="d.code"
            :label="d.label"
            :value="d.code"
          />
        </el-select>
      </template>

      <label class="qa-compose__label">正文</label>
      <MarkdownEditor
        ref="editorRef"
        v-model:body="body"
        :max-length="BODY_MAX"
        :disabled="submitting"
        placeholder="描述你的问题，可以贴图片"
      />

      <div class="qa-compose__actions">
        <el-button @click="cancel">取消</el-button>
        <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="submit">
          {{ isEdit ? "保存" : "发布" }}
        </el-button>
      </div>
    </template>
  </section>
</template>

<style scoped>
/* 颜色只用语义令牌（--qa-*），取值由 forum.css 按外壳注入（#242）。 */
.qa-compose {
  max-width: 760px;
  margin: 0 auto;
  padding: 24px 16px 48px;
  /* .glass-backdrop 是 fixed + z-index:0 的装饰层；isolation 建独立层叠上下文，
     再让内容以 z-index:1 压在光斑之上。 */
  position: relative;
  isolation: isolate;
}
.qa-compose > :not(.glass-backdrop) {
  position: relative;
  z-index: 1;
}
.qa-compose__label {
  display: block;
  margin: 18px 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--qa-ink);
}
.qa-compose__optional {
  font-weight: 400;
  color: var(--qa-faint);
}
.qa-compose__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 20px;
}
.qa-compose__error {
  color: var(--qa-danger);
  font-size: 12px;
  margin: 6px 0 0;
}
</style>
