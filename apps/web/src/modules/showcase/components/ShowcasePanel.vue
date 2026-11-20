<script setup lang="ts">
/**
 * 作品提交面板。
 *
 * 通栏横排的表单 + 自己的作品列表，**始终放在所在页面的下方**，
 * 不做成右侧浮层或窄侧栏：提交是一个完整动作，塞进侧栏会让
 * 「填什么」与「交什么」被切得太碎。
 *
 * 三种载体：仓库链接 / 源码 zip / 两者都给，至少一个（后端也会拦）。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";

import {
  deleteShowcase,
  listMyShowcase,
  submitShowcase,
  showcaseArchivePath,
  type ShowcaseSubmission,
} from "@/api/showcase";
import { readAuthSession } from "@/auth/session";

const props = defineProps<{
  /**
   * 关联的学习路线（可选）。带上就记一次标题快照，
   * 之后路线改名或删除，这次提交仍能看出交的是哪条。
   */
  roadmapId?: number | null;
  /** 面板标题，默认「提交作品」。 */
  heading?: string;
  /** 一句话说明，随所在页面的语境变化。 */
  hint?: string;
}>();

const title = ref("");
const summary = ref("");
const repoURL = ref("");
const archive = ref<File | null>(null);

const items = ref<ShowcaseSubmission[]>([]);
const loading = ref(false);
const submitting = ref(false);
const error = ref("");

const signedIn = computed(() => readAuthSession() !== null);

/** 至少一个载体才能提交；与后端校验同口径，避免白跑一次。 */
const canSubmit = computed(
  () =>
    !submitting.value &&
    title.value.trim() !== "" &&
    (repoURL.value.trim() !== "" || archive.value !== null),
);

/** 源码包大小上限，与后端 domain.ShowcaseArchiveMaxBytes 一致。 */
const MAX_ARCHIVE_BYTES = 50 * 1024 * 1024;

function onFileChange(event: Event): void {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0] ?? null;
  if (!file) {
    archive.value = null;
    return;
  }
  if (!file.name.toLowerCase().endsWith(".zip")) {
    ElMessage.warning("源码包只支持 .zip");
    input.value = "";
    return;
  }
  if (file.size > MAX_ARCHIVE_BYTES) {
    ElMessage.warning("源码包不能超过 50 MB");
    input.value = "";
    return;
  }
  archive.value = file;
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return "";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

async function load(): Promise<void> {
  if (!signedIn.value) return;
  loading.value = true;
  error.value = "";
  try {
    const result = await listMyShowcase();
    items.value = result.items;
  } catch (err) {
    error.value = err instanceof Error ? err.message : "加载我的作品失败";
  } finally {
    loading.value = false;
  }
}

async function submit(): Promise<void> {
  if (!canSubmit.value) return;
  submitting.value = true;
  try {
    await submitShowcase({
      title: title.value.trim(),
      summary: summary.value,
      repoURL: repoURL.value.trim(),
      roadmapId: props.roadmapId ?? null,
      archive: archive.value,
    });
    ElMessage.success("作品已提交");
    // 清空表单，但保留路线关联（多半是同一条路线的连续提交）。
    title.value = "";
    summary.value = "";
    repoURL.value = "";
    archive.value = null;
    const input = document.getElementById("showcase-archive") as HTMLInputElement | null;
    if (input) input.value = "";
    await load();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "提交失败，请重试");
  } finally {
    submitting.value = false;
  }
}

async function remove(item: ShowcaseSubmission): Promise<void> {
  try {
    await deleteShowcase(item.id);
    ElMessage.success("已删除");
    await load();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "删除失败");
  }
}

/**
 * 下载源码包。
 *
 * 走带鉴权的接口而不是直接链到对象存储：对象键是内部实现细节，
 * 且内容私有，不能让任何人拿到 URL 就能下。
 * 用 fetch + blob 是因为普通 <a href> 带不上 Authorization 头。
 */
async function download(item: ShowcaseSubmission): Promise<void> {
  const token = readAuthSession()?.access_token;
  if (!token) {
    ElMessage.error("登录状态已失效，请重新登录");
    return;
  }
  try {
    const resp = await fetch(showcaseArchivePath(item.id), {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!resp.ok) {
      const body = await resp.json().catch(() => null);
      throw new Error(body?.message || "下载失败");
    }
    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = item.archive_name || `showcase-${item.id}.zip`;
    link.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "下载失败");
  }
}

const STATUS_LABEL: Record<string, string> = {
  submitted: "审核中",
  approved: "已通过",
  rejected: "需修改",
};

onMounted(() => {
  void load();
});
</script>

<template>
  <section class="showcase" aria-labelledby="showcase-heading">
    <header class="showcase__head">
      <div>
        <p class="showcase__eyebrow">SHOWCASE</p>
        <h2 id="showcase-heading" class="showcase__title">{{ heading || "提交作品" }}</h2>
        <p class="showcase__desc">
          {{ hint || "把这段路线上做出来的东西交上来：可以贴 GitHub 链接，也可以直接上传源码压缩包。" }}
        </p>
      </div>
      <span class="showcase__count">{{ items.length }} 份已提交</span>
    </header>

    <p v-if="!signedIn" class="showcase__notice">
      登录后即可提交作品，并在下方看到自己的提交记录。
    </p>

    <template v-else>
      <form class="showcase__form" @submit.prevent="submit">
        <div class="showcase__row">
          <label class="showcase__field">
            <span class="showcase__label">作品标题</span>
            <input v-model="title" class="showcase__input" maxlength="100" placeholder="例如：校园二手交易平台" :disabled="submitting" />
          </label>
        </div>

        <div class="showcase__row showcase__row--split">
          <label class="showcase__field">
            <span class="showcase__label">GitHub / 仓库链接<span class="showcase__optional">（选填，与源码包至少填一个）</span></span>
            <input
              v-model="repoURL"
              class="showcase__input"
              type="url"
              placeholder="https://github.com/你的账号/仓库"
              :disabled="submitting"
            />
          </label>
          <label class="showcase__field">
            <span class="showcase__label">源码压缩包<span class="showcase__optional">（.zip，最大 50 MB）</span></span>
            <input
              id="showcase-archive"
              class="showcase__input showcase__input--file"
              type="file"
              accept=".zip,application/zip"
              :disabled="submitting"
              @change="onFileChange"
            />
            <span v-if="archive" class="showcase__file">{{ archive.name }} · {{ formatSize(archive.size) }}</span>
          </label>
        </div>

        <label class="showcase__field">
          <span class="showcase__label">作品说明<span class="showcase__optional">（选填）</span></span>
          <textarea
            v-model="summary"
            class="showcase__textarea"
            rows="3"
            maxlength="2000"
            placeholder="用了什么技术、解决了什么问题、怎么运行"
            :disabled="submitting"
          ></textarea>
        </label>

        <div class="showcase__actions">
          <el-button type="primary" native-type="submit" :loading="submitting" :disabled="!canSubmit">
            提交作品
          </el-button>
          <span class="showcase__tip">仓库链接与源码包至少提供一项</span>
        </div>
      </form>

      <p v-if="error" class="showcase__error" role="alert">{{ error }}</p>
      <p v-else-if="loading && !items.length" class="showcase__empty">正在加载我的作品…</p>
      <p v-else-if="!items.length" class="showcase__empty">还没有提交过作品。</p>

      <ul v-else class="showcase__list">
        <li v-for="item in items" :key="item.id" class="showcase-item">
          <div class="showcase-item__main">
            <h3 class="showcase-item__title">{{ item.title }}</h3>
            <p v-if="item.roadmap_title" class="showcase-item__meta">关联路线：{{ item.roadmap_title }}</p>
            <p v-if="item.summary" class="showcase-item__summary">{{ item.summary }}</p>
            <p v-if="item.review_note" class="showcase-item__note">审阅意见：{{ item.review_note }}</p>
          </div>
          <div class="showcase-item__side">
            <span class="showcase-item__status" :data-status="item.status">
              {{ STATUS_LABEL[item.status] || item.status }}
            </span>
            <a
              v-if="item.repo_url"
              class="showcase-item__link"
              :href="item.repo_url"
              target="_blank"
              rel="noopener noreferrer"
              >看仓库 ↗</a
            >
            <button
              v-if="item.has_archive"
              type="button"
              class="showcase-item__link"
              @click="download(item)"
            >
              下载源码（{{ formatSize(item.archive_size) }}）
            </button>
            <button type="button" class="showcase-item__remove" @click="remove(item)">删除</button>
          </div>
        </li>
      </ul>
    </template>
  </section>
</template>

<style scoped>
.showcase {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 24px;
  border-top: 1px solid var(--line, #e5e7eb);
  max-width: 1100px;
  margin: 24px auto 0;
}

.showcase__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.showcase__eyebrow {
  margin: 0 0 4px;
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #8a8f99;
}

.showcase__title {
  margin: 0 0 6px;
  font-size: 20px;
}

.showcase__desc {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
  max-width: 60ch;
}

.showcase__count {
  flex: none;
  font-size: 12px;
  color: #8a8f99;
  white-space: nowrap;
}

.showcase__notice {
  margin: 0;
  padding: 16px;
  border: 1px dashed var(--line, #e5e7eb);
  border-radius: 10px;
  font-size: 13px;
  color: #6b7280;
}

.showcase__form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--line, #e5e7eb);
  border-radius: 12px;
  background: #fff;
}

.showcase__row--split {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.showcase__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.showcase__label {
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}

.showcase__optional {
  font-weight: 400;
  color: #9ca3af;
}

.showcase__input,
.showcase__textarea {
  width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--line, #d1d5db);
  border-radius: 8px;
  font: inherit;
  font-size: 13px;
  background: #fff;
}

.showcase__input--file {
  padding: 7px 10px;
}

.showcase__textarea {
  resize: vertical;
}

.showcase__file {
  font-size: 12px;
  color: #6b7280;
}

.showcase__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.showcase__tip {
  font-size: 12px;
  color: #9ca3af;
}

.showcase__error {
  margin: 0;
  color: #b91c1c;
  font-size: 13px;
}

.showcase__empty {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
}

.showcase__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.showcase-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--line, #e5e7eb);
  border-radius: 10px;
  background: #fff;
}

.showcase-item__title {
  margin: 0 0 4px;
  font-size: 15px;
}

.showcase-item__meta,
.showcase-item__summary,
.showcase-item__note {
  margin: 0 0 4px;
  font-size: 12px;
  color: #6b7280;
}

.showcase-item__note {
  color: #b45309;
}

.showcase-item__side {
  flex: none;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
}

.showcase-item__status {
  padding: 2px 8px;
  border-radius: 999px;
  background: #f3f4f6;
  color: #4b5563;
}

.showcase-item__status[data-status="approved"] {
  background: #ecfdf5;
  color: #047857;
}

.showcase-item__status[data-status="rejected"] {
  background: #fef2f2;
  color: #b91c1c;
}

.showcase-item__link,
.showcase-item__remove {
  padding: 0;
  border: 0;
  background: none;
  color: var(--blue, #2563eb);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

.showcase-item__remove {
  color: #b91c1c;
}

@media (max-width: 720px) {
  .showcase__row--split {
    grid-template-columns: 1fr;
  }

  .showcase-item {
    flex-direction: column;
  }

  .showcase-item__side {
    flex-wrap: wrap;
  }
}
</style>
