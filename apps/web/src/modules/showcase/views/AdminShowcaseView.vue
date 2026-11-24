<script setup lang="ts">
/**
 * 作品审阅（管理员端）。
 *
 * 学员交上来的作品在这里按「待审优先」列出，管理员可以：
 *   - 看仓库链接 / 下载源码包；
 *   - 通过，或驳回并填理由（驳回必须带理由，让学员知道改什么）。
 *
 * 权限由路由 meta.allowRoles = MEMBER_ONLY 与后端 admin 分组双重把关，
 * 页面本身不做角色判断。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import {
  listAllShowcase,
  reviewShowcase,
  showcaseArchivePath,
  type ShowcaseStatus,
  type ShowcaseSubmission,
} from "@/api/showcase";
import { readAuthSession } from "@/auth/session";

const items = ref<ShowcaseSubmission[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;

const loading = ref(false);
const error = ref("");
const statusFilter = ref<"" | ShowcaseStatus>("");

const hasFilter = computed(() => statusFilter.value !== "");

const STATUS_LABEL: Record<ShowcaseStatus, string> = {
  submitted: "待审阅",
  approved: "已通过",
  rejected: "已驳回",
};

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const result = await listAllShowcase(statusFilter.value || null, page.value, pageSize);
    items.value = result.items;
    total.value = result.total;
  } catch (err) {
    error.value = err instanceof Error ? err.message : "加载作品列表失败";
  } finally {
    loading.value = false;
  }
}

function applyFilter(): void {
  page.value = 1;
  void load();
}

/** 下载源码包：管理员走同一条带鉴权端点（服务层对管理员放行）。 */
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

/**
 * 审阅一份作品。
 *
 * 驳回用 ElMessageBox.prompt 强制填理由：后端也会拦空理由，
 * 但在这里就挡住能少一次往返，也让管理员当场想清楚要说什么。
 */
async function review(item: ShowcaseSubmission, status: "approved" | "rejected"): Promise<void> {
  let note = "";
  if (status === "rejected") {
    try {
      const { value } = await ElMessageBox.prompt(
        `驳回「${item.title}」——请写明需要修改的地方，学员会看到这条意见。`,
        "驳回作品",
        {
          confirmButtonText: "确认驳回",
          cancelButtonText: "取消",
          inputPlaceholder: "例如：请补充运行说明与测试用例",
          inputValidator: (v: string | null) =>
            v && v.trim() ? true : "请填写驳回理由",
        },
      );
      note = (value ?? "").trim();
    } catch {
      return; // 管理员取消
    }
  }

  try {
    await reviewShowcase(item.id, status, note);
    ElMessage.success(status === "approved" ? "已通过" : "已驳回");
    await load();
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : "操作失败，请重试");
  }
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return "";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function formatDate(raw: string): string {
  if (!raw) return "";
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? raw : d.toLocaleString("zh-CN");
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="page review-page">
    <PageHeader
      eyebrow="IoT-Studio·作品"
      title="作品审阅"
      description="学员提交的源码包与仓库链接在这里审阅，待审的排在最前。"
    />

    <main class="page__body">
      <div class="review__toolbar">
        <el-select v-model="statusFilter" placeholder="全部状态" style="width: 160px" @change="applyFilter">
          <el-option label="全部状态" value="" />
          <el-option label="待审阅" value="submitted" />
          <el-option label="已通过" value="approved" />
          <el-option label="已驳回" value="rejected" />
        </el-select>
        <span class="review__total">共 {{ total }} 份</span>
      </div>

      <p v-if="error" class="review__error" role="alert">{{ error }}</p>
      <p v-else-if="loading && !items.length" class="review__empty">正在加载…</p>
      <p v-else-if="!items.length" class="review__empty">
        {{ hasFilter ? "该状态下暂无作品。" : "还没有学员提交作品。" }}
      </p>

      <ul v-else class="review__list">
        <li v-for="item in items" :key="item.id" class="review-item">
          <div class="review-item__head">
            <div>
              <h2 class="review-item__title">{{ item.title }}</h2>
              <p class="review-item__meta">
                <span class="review-item__status" :data-status="item.status">
                  {{ STATUS_LABEL[item.status] || item.status }}
                </span>
                <span v-if="item.roadmap_title">路线：{{ item.roadmap_title }}</span>
                <span>{{ formatDate(item.created_at) }}</span>
                <span v-if="item.is_mine">自己提交</span>
              </p>
            </div>
          </div>

          <p v-if="item.summary" class="review-item__summary">{{ item.summary }}</p>

          <div class="review-item__payload">
            <a
              v-if="item.repo_url"
              class="review-item__link"
              :href="item.repo_url"
              target="_blank"
              rel="noopener noreferrer"
              >打开仓库 ↗</a
            >
            <button
              v-if="item.has_archive"
              type="button"
              class="review-item__link"
              @click="download(item)"
            >
              下载源码（{{ formatSize(item.archive_size) }}）
            </button>
          </div>

          <p v-if="item.review_note" class="review-item__note">
            审阅意见：{{ item.review_note }}
          </p>

          <div v-if="item.status === 'submitted'" class="review-item__actions">
            <el-button type="success" size="small" @click="review(item, 'approved')">通过</el-button>
            <el-button type="danger" size="small" plain @click="review(item, 'rejected')">驳回</el-button>
          </div>
        </li>
      </ul>

      <el-pagination
        v-if="total > pageSize"
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        class="review__pager"
        @current-change="load"
      />
    </main>
  </div>
</template>

<style scoped>
.review-page {
  min-height: 100dvh;
}

.page__body {
  max-width: 1000px;
  margin: 0 auto;
  padding: 24px;
}

.review__toolbar {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}

.review__total {
  font-size: 13px;
  color: #6b7280;
}

.review__error {
  color: #b91c1c;
  font-size: 13px;
}

.review__empty {
  padding: 40px 0;
  text-align: center;
  color: #6b7280;
  font-size: 13px;
}

.review__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.review-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px;
  border: 1px solid var(--line, #e5e7eb);
  border-radius: 12px;
  background: #fff;
}

.review-item__title {
  margin: 0 0 6px;
  font-size: 16px;
}

.review-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin: 0;
  font-size: 12px;
  color: #6b7280;
}

.review-item__status {
  padding: 2px 8px;
  border-radius: 999px;
  background: #f3f4f6;
  color: #4b5563;
}

.review-item__status[data-status="approved"] {
  background: #ecfdf5;
  color: #047857;
}

.review-item__status[data-status="rejected"] {
  background: #fef2f2;
  color: #b91c1c;
}

.review-item__summary {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: #374151;
}

.review-item__payload {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.review-item__link {
  padding: 0;
  border: 0;
  background: none;
  color: var(--blue, #2563eb);
  font: inherit;
  font-size: 13px;
  text-decoration: none;
  cursor: pointer;
}

.review-item__link:hover {
  text-decoration: underline;
}

.review-item__note {
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: #fffbeb;
  color: #92400e;
  font-size: 12px;
}

.review-item__actions {
  display: flex;
  gap: 10px;
  padding-top: 4px;
  border-top: 1px solid var(--line-soft, #f0f0f0);
}

.review__pager {
  margin-top: 20px;
  justify-content: center;
}
</style>
