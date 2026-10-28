<script setup lang="ts">
/**
 * 工作室知识库（#196）。
 *
 * 闭环：创建条目（同时产生 v1）→ 列表筛选 → 编辑正文（版本 +1）→
 * 浏览修订历史 → 删除（级联修订）。
 *
 * **没有**一键回滚（ADR 0001 §3 明确拒绝）：恢复 = 在历史里打开旧版，
 * 把它「带回到编辑器」再保存一版。历史因此始终是追加式的。
 *
 * 全部接口仅管理员可用；permission 可写但不参与鉴权——
 * 首期不为 member/public 挂读取入口。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import {
  createStudioKnowledge,
  deleteStudioKnowledge,
  fetchStudioKnowledge,
  fetchStudioKnowledgeRevision,
  listStudioKnowledge,
  listStudioKnowledgeRevisions,
  updateStudioKnowledge,
  type StudioKnowledgeEntry,
  type StudioKnowledgeFilters,
  type StudioKnowledgePermission,
  type StudioKnowledgeRevision,
} from "@/api/studio-knowledge";
import {
  EMPTY_KNOWLEDGE_DRAFT,
  KNOWLEDGE_PERMISSION_OPTIONS,
  bodyExcerpt,
  deleteConfirmText,
  describeKnowledgeError,
  draftErrors,
  formatDay,
  permissionLabel,
  toDraft,
  toUpdateInput,
  versionSummary,
  type KnowledgeDraft,
} from "../model";
import "../studio-knowledge.css";

/* ── 列表 ─────────────────────────────────────────────────────── */

const entries = ref<StudioKnowledgeEntry[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;

const loading = ref(false);
const loadError = ref("");
/** 列表区动作（删除）的错误。 */
const actionError = ref("");

const keyword = ref("");
const permissionFilter = ref<StudioKnowledgePermission | "">("");

const filters = computed<StudioKnowledgeFilters>(() => ({
  q: keyword.value,
  permission: permissionFilter.value,
}));

const hasActiveFilters = computed(
  () => keyword.value.trim() !== "" || permissionFilter.value !== "",
);

async function load(nextPage = page.value): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const data = await listStudioKnowledge(filters.value, nextPage, pageSize);
    entries.value = data.items;
    total.value = data.total;
    page.value = data.page;
  } catch (error) {
    loadError.value = describeKnowledgeError(error);
  } finally {
    loading.value = false;
  }
}

onMounted(() => void load());

function applyFilters(): void {
  void load(1);
}

function resetFilters(): void {
  keyword.value = "";
  permissionFilter.value = "";
  void load(1);
}

/* ── 编辑器（创建 / 编辑共用） ─────────────────────────────────── */

const dialogOpen = ref(false);
/** 正在编辑的条目；null = 新建。 */
const editing = ref<StudioKnowledgeEntry | null>(null);
const draft = ref<KnowledgeDraft>({ ...EMPTY_KNOWLEDGE_DRAFT });
const fieldErrors = ref<Record<string, string>>({});
const saving = ref(false);
/** 保存错误——与列表错误分开，否则保存失败会同时弹在两处。 */
const saveError = ref("");

const isEditing = computed(() => editing.value !== null);
const dialogTitle = computed(() =>
  isEditing.value
    ? `编辑「${editing.value?.title}」· v${editing.value?.version}`
    : "新建知识库条目",
);

function openCreate(): void {
  editing.value = null;
  draft.value = { ...EMPTY_KNOWLEDGE_DRAFT };
  fieldErrors.value = {};
  saveError.value = "";
  dialogOpen.value = true;
}

async function openEdit(entry: StudioKnowledgeEntry): Promise<void> {
  // 列表项就是完整条目（后端返回全文），不必再请求一次详情。
  editing.value = entry;
  draft.value = toDraft(entry);
  fieldErrors.value = {};
  saveError.value = "";
  dialogOpen.value = true;
}

async function save(): Promise<void> {
  if (saving.value) return;
  fieldErrors.value = draftErrors(draft.value);
  if (Object.keys(fieldErrors.value).length > 0) return;

  saving.value = true;
  saveError.value = "";
  try {
    if (editing.value) {
      const patch = toUpdateInput(draft.value, editing.value);
      // 除了 version 没有任何改动：不必白跑一次写库。
      // 保存会推进版本号，无意义地升版会让历史里出现一串内容相同的快照。
      if (Object.keys(patch).length === 1) {
        ElMessage.info("没有需要保存的改动");
        dialogOpen.value = false;
        return;
      }
      const saved = await updateStudioKnowledge(editing.value.id, patch);
      ElMessage.success(`已保存为 v${saved.version}`);
    } else {
      const created = await createStudioKnowledge({
        title: draft.value.title.trim(),
        body: draft.value.body,
        permission: draft.value.permission,
      });
      ElMessage.success(`条目已创建（v${created.version}）`);
    }
    dialogOpen.value = false;
    await load();
  } catch (error) {
    saveError.value = describeKnowledgeError(error);
  } finally {
    saving.value = false;
  }
}

/* ── 修订历史 ─────────────────────────────────────────────────── */

const historyOpen = ref(false);
const historyEntry = ref<StudioKnowledgeEntry | null>(null);
const revisions = ref<StudioKnowledgeRevision[]>([]);
const historyLoading = ref(false);
const historyError = ref("");
/** 正在查看的旧版快照；null = 只看列表。 */
const previewRevision = ref<StudioKnowledgeRevision | null>(null);

async function openHistory(entry: StudioKnowledgeEntry): Promise<void> {
  historyEntry.value = entry;
  revisions.value = [];
  previewRevision.value = null;
  historyError.value = "";
  historyOpen.value = true;
  historyLoading.value = true;
  try {
    const data = await listStudioKnowledgeRevisions(entry.id);
    revisions.value = data.items;
  } catch (error) {
    historyError.value = describeKnowledgeError(error);
  } finally {
    historyLoading.value = false;
  }
}

async function previewVersion(version: number): Promise<void> {
  if (!historyEntry.value) return;
  historyError.value = "";
  try {
    previewRevision.value = await fetchStudioKnowledgeRevision(
      historyEntry.value.id,
      version,
    );
  } catch (error) {
    historyError.value = describeKnowledgeError(error);
  }
}

/**
 * 把旧版内容带回编辑器。
 *
 * 这是「恢复」的实现方式：没有回滚端点，管理员读回旧版内容后
 * 再保存一版，于是历史里多出一条新版本，中间的改动都还在。
 */
function restoreIntoEditor(revision: StudioKnowledgeRevision): void {
  const entry = historyEntry.value;
  if (!entry) return;
  draft.value = {
    title: revision.title,
    body: revision.body,
    permission: entry.permission,
  };
  editing.value = entry;
  fieldErrors.value = {};
  saveError.value = "";
  historyOpen.value = false;
  dialogOpen.value = true;
  ElMessage.info(`已载入 v${revision.version} 的内容，保存后会成为新版本`);
}

/* ── 删除 ─────────────────────────────────────────────────────── */

const deletingID = ref<number | null>(null);

async function remove(entry: StudioKnowledgeEntry): Promise<void> {
  if (deletingID.value !== null) return;
  try {
    await ElMessageBox.confirm(deleteConfirmText(entry), "删除条目", {
      confirmButtonText: "删除",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch {
    // 用户取消：不是错误。
    return;
  }

  deletingID.value = entry.id;
  actionError.value = "";
  try {
    await deleteStudioKnowledge(entry.id);
    ElMessage.success(`已删除「${entry.title}」`);
    await load();
  } catch (error) {
    actionError.value = describeKnowledgeError(error);
  } finally {
    deletingID.value = null;
  }
}
</script>

<template>
  <div class="page knowledge-page">
    <main class="page__body">
      <PageHeader
        eyebrow="工作室运营 · 知识库"
        title="知识库"
        description="工作室的内部文档。每次保存都会留下一个版本，历史不会被覆盖。"
        :meta="`共 ${total} 篇`"
      >
        <template #actions>
          <button class="knowledge-button knowledge-button--primary" type="button" @click="openCreate">
            新建条目 ↗
          </button>
        </template>
      </PageHeader>

      <!-- 筛选条 -->
      <section class="panel panel--pad knowledge-filters" aria-label="条目筛选">
        <label class="knowledge-field">
          <span class="knowledge-field__label">关键词</span>
          <input
            v-model="keyword"
            class="knowledge-control"
            type="search"
            placeholder="标题 / 正文"
            @keyup.enter="applyFilters"
          />
        </label>
        <label class="knowledge-field">
          <span class="knowledge-field__label">权限</span>
          <select v-model="permissionFilter" class="knowledge-control">
            <option value="">全部权限</option>
            <option v-for="option in KNOWLEDGE_PERMISSION_OPTIONS" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <div class="knowledge-filters__actions">
          <button class="knowledge-button" type="button" @click="applyFilters">筛选</button>
          <button class="knowledge-button" type="button" @click="resetFilters">重置</button>
        </div>
      </section>

      <p v-if="actionError" class="knowledge-error" role="alert">{{ actionError }}</p>

      <!-- 加载 / 错误 / 空态 -->
      <section v-if="loadError" class="panel panel--pad" role="alert">
        <p class="knowledge-error">{{ loadError }}</p>
        <button class="knowledge-button" type="button" @click="load()">重试</button>
      </section>
      <p v-else-if="loading" class="panel panel--pad" role="status">正在加载知识库…</p>
      <!--
        空态分两种：「库里本来没有条目」和「筛选没命中」。
        混成一句会让管理员在筛选后误以为数据丢了，然后重复创建同一篇文档。
      -->
      <section v-else-if="!entries.length" class="panel panel--pad knowledge-empty">
        <template v-if="hasActiveFilters">
          <span class="knowledge-kicker">NO MATCH</span>
          <h2>没有符合筛选条件的条目。</h2>
          <p>换个关键词或权限试试。</p>
          <button class="knowledge-button" type="button" @click="resetFilters">清空筛选</button>
        </template>
        <template v-else>
          <span class="knowledge-kicker">START WITH A NOTE</span>
          <h2>知识库还是空的。</h2>
          <p>写下第一篇内部文档；之后每次保存都会自动留下版本。</p>
          <button class="knowledge-button" type="button" @click="openCreate">新建条目</button>
        </template>
      </section>

      <!-- 条目列表 -->
      <div v-else class="knowledge-list">
        <article v-for="entry in entries" :key="entry.id" class="panel knowledge-list__item">
          <div class="knowledge-list__heading">
            <span class="knowledge-kicker">DOC / {{ String(entry.id).padStart(3, "0") }}</span>
            <span class="knowledge-badge">{{ permissionLabel(entry.permission) }}</span>
          </div>
          <h2>{{ entry.title }}</h2>
          <p class="knowledge-list__meta">{{ versionSummary(entry) }}</p>
          <p class="knowledge-list__excerpt">
            {{ bodyExcerpt(entry.body) || "（还没有正文）" }}
          </p>
          <div class="knowledge-actions">
            <button class="knowledge-button" type="button" @click="openEdit(entry)">编辑</button>
            <button class="knowledge-button" type="button" @click="openHistory(entry)">
              历史 · {{ entry.version }} 版
            </button>
            <button
              class="knowledge-button knowledge-button--danger"
              type="button"
              :disabled="deletingID !== null"
              :aria-busy="deletingID === entry.id"
              @click="remove(entry)"
            >
              删除
            </button>
          </div>
        </article>
      </div>

      <el-pagination
        v-if="total > pageSize"
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        :disabled="loading"
        @current-change="load"
      />

      <!-- 编辑器 -->
      <el-dialog v-model="dialogOpen" :title="dialogTitle" width="min(760px, 94vw)">
        <form class="knowledge-form" novalidate @submit.prevent="save">
          <label class="knowledge-field">
            <span class="knowledge-field__label">标题 <em>必填</em></span>
            <input
              v-model="draft.title"
              class="knowledge-control"
              :aria-invalid="Boolean(fieldErrors.title)"
              placeholder="如：本地开发环境搭建"
            />
            <span v-if="fieldErrors.title" class="knowledge-field__error">{{ fieldErrors.title }}</span>
          </label>

          <label class="knowledge-field">
            <span class="knowledge-field__label">权限</span>
            <select v-model="draft.permission" class="knowledge-control">
              <option v-for="option in KNOWLEDGE_PERMISSION_OPTIONS" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
            <span class="knowledge-field__hint">
              首期所有读取入口都只对管理员开放；这里选的是以后的可见范围。
            </span>
          </label>

          <label class="knowledge-field">
            <span class="knowledge-field__label">正文</span>
            <textarea
              v-model="draft.body"
              class="knowledge-control knowledge-control--body"
              rows="16"
              placeholder="支持 Markdown 写法；可以留空，之后再补。"
              :aria-invalid="Boolean(fieldErrors.body)"
            />
            <span v-if="fieldErrors.body" class="knowledge-field__error">{{ fieldErrors.body }}</span>
          </label>

          <p v-if="saveError" class="knowledge-error" role="alert">{{ saveError }}</p>

          <div class="knowledge-form__actions">
            <button class="knowledge-button" type="button" @click="dialogOpen = false">取消</button>
            <button class="knowledge-button knowledge-button--primary" type="submit" :disabled="saving">
              {{ saving ? "保存中…" : isEditing ? "保存新版本" : "创建" }}
            </button>
          </div>
        </form>
      </el-dialog>

      <!-- 修订历史 -->
      <el-dialog
        v-model="historyOpen"
        :title="historyEntry ? `「${historyEntry.title}」的修订历史` : '修订历史'"
        width="min(820px, 94vw)"
      >
        <p v-if="historyError" class="knowledge-error" role="alert">{{ historyError }}</p>
        <p v-else-if="historyLoading" class="knowledge-hint" role="status">正在加载修订…</p>
        <p v-else-if="!revisions.length" class="knowledge-hint">还没有修订记录。</p>

        <template v-else>
          <ol class="knowledge-history">
            <li v-for="revision in revisions" :key="revision.id" class="knowledge-history__item">
              <div class="knowledge-history__head">
                <strong>v{{ revision.version }}</strong>
                <span class="knowledge-history__date">{{ formatDay(revision.created_at) }}</span>
                <span class="knowledge-history__editor">编辑者 #{{ revision.editor_id }}</span>
              </div>
              <p class="knowledge-history__title">{{ revision.title }}</p>
              <div class="knowledge-history__actions">
                <button class="knowledge-button knowledge-button--small" type="button" @click="previewVersion(revision.version)">
                  查看内容
                </button>
                <button
                  class="knowledge-button knowledge-button--small"
                  type="button"
                  @click="restoreIntoEditor(revision)"
                >
                  带回编辑器
                </button>
              </div>
            </li>
          </ol>

          <!-- 旧版预览：只读，不提供「一键回滚」——恢复要走带回编辑器再保存 -->
          <section v-if="previewRevision" class="knowledge-preview" aria-label="版本内容">
            <h3>v{{ previewRevision.version }} 的内容</h3>
            <pre class="knowledge-preview__body">{{ previewRevision.body || "（这一版没有正文）" }}</pre>
            <p class="knowledge-field__hint">
              恢复方式：点「带回编辑器」，确认后再保存——会生成一个新版本，历史不会被抹掉。
            </p>
          </section>
        </template>
      </el-dialog>
    </main>
  </div>
</template>
