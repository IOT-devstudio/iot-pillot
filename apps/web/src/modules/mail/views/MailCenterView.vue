<script setup lang="ts">
/**
 * 邮件中心 `/templates`（issue #52）。
 *
 * 一块页面两个区块：模板管理（CRUD + 发送入口）与发信记录（服务端分页）。
 * 记录的操作者列只拿得到 from_user_id——后端 MailRecordResp 不返回姓名，
 * 这里用管理员名单做 id→姓名映射，映射不到就显示 #id，不编造。
 *
 * 发送/编辑弹窗的用户名单来自 /admin/users 第一页（100 条）：
 * 招新场景注册量在百级，先不做无限滚动，名单拉不下来时降级为
 * 「按邮箱发送」仍可用。
 */
import { computed, onMounted, ref } from "vue";

import { listAdminUsers, type AdminUser } from "@/api/admin";
import type { MailTemplate, MailTemplateInput } from "@/api/mail";
import AppHeader from "@/components/common/AppHeader.vue";
import BackToDashboard from "@/components/common/BackToDashboard.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";
import SendMailDialog from "../components/SendMailDialog.vue";
import TemplateEditorDialog from "../components/TemplateEditorDialog.vue";
import { useMailRecords } from "../composables/useMailRecords";
import { useMailTemplates } from "../composables/useMailTemplates";

const {
  templates,
  loading,
  error,
  unauthorized,
  saving,
  load,
  save,
  remove,
} = useMailTemplates();

const {
  items: records,
  total: recordsTotal,
  page: recordsPage,
  pageSize: recordsPageSize,
  loading: recordsLoading,
  error: recordsError,
  unauthorized: recordsUnauthorized,
  load: loadRecords,
  changePage: changeRecordsPage,
  changePageSize: changeRecordsPageSize,
} = useMailRecords();

/* —— 用户名单：发送弹窗的收件人选项 + 记录操作者姓名映射 —— */
const users = ref<AdminUser[]>([]);
const usersError = ref("");
async function loadUsers(): Promise<void> {
  try {
    const result = await listAdminUsers(1, 100);
    users.value = result.items;
    usersError.value = "";
  } catch (e) {
    // 名单失败不拖垮页面：发送退化为按邮箱模式，记录操作者显示 #id
    usersError.value = describeAuthError(e).message;
  }
}

const userNameById = computed(() => {
  const map = new Map<number, string>();
  for (const user of users.value) map.set(user.user_id, user.name);
  return map;
});

function operatorLabel(fromUserId: number): string {
  return userNameById.value.get(fromUserId) ?? `#${fromUserId}`;
}

/* —— 弹窗状态 —— */
const editorVisible = ref(false);
const editing = ref<MailTemplate | null>(null);
const sendVisible = ref(false);
const sendTarget = ref<MailTemplate | null>(null);

function openCreate(): void {
  editing.value = null;
  editorVisible.value = true;
}

function openEdit(template: MailTemplate): void {
  editing.value = template;
  editorVisible.value = true;
}

async function onSave(input: MailTemplateInput): Promise<void> {
  const ok = await save(input, editing.value?.id ?? null);
  if (ok) editorVisible.value = false;
}

function openSend(template: MailTemplate): void {
  sendTarget.value = template;
  sendVisible.value = true;
}

onMounted(() => {
  void load();
  void loadRecords();
  void loadUsers();
});
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <BackToDashboard />

      <PageHeader
        eyebrow="iot-pillot · 招新管理"
        title="邮件中心"
        description="管理邮件模板并发送邀请、通知与感谢信；所有发出的邮件都会留下可追溯的记录。"
        :meta="loading || error ? '' : `共 ${templates.length} 个模板`"
      >
        <template #actions>
          <el-button type="primary" @click="openCreate">新建模板</el-button>
        </template>
      </PageHeader>

      <!-- 模板列表 -->
      <section class="panel panel--pad" aria-label="邮件模板">
        <header class="panel-head">
          <h2 class="panel-head__title">模板</h2>
          <span class="panel-head__meta">
            {{ loading ? "加载中…" : `共 ${templates.length} 个` }}
          </span>
        </header>

        <div v-if="loading" role="status" aria-label="正在加载模板">
          <el-skeleton :rows="4" animated />
        </div>
        <div v-else-if="error" class="state state--error" role="alert">
          <span>{{ error }}</span>
          <el-button v-if="!unauthorized" link type="primary" @click="load">
            重试
          </el-button>
        </div>
        <p v-else-if="templates.length === 0" class="state" role="status">
          还没有模板，点右上角「新建模板」创建第一封
        </p>
        <el-table v-else :data="templates">
          <el-table-column prop="name" label="名称" min-width="140" />
          <el-table-column prop="type" label="类型" width="130">
            <template #default="{ row }">
              <el-tag size="small" effect="plain">{{ row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="title" label="主题" min-width="220" show-overflow-tooltip />
          <el-table-column label="变量" min-width="180">
            <template #default="{ row }">
              <template v-if="row.variables.length > 0">
                <el-tag
                  v-for="name in row.variables"
                  :key="name"
                  size="small"
                  effect="plain"
                  type="info"
                >
                  {{ name }}
                </el-tag>
              </template>
              <span v-else class="self-note">无变量</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openSend(row)">发送</el-button>
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button
                link
                type="danger"
                :loading="saving"
                @click="remove(row)"
              >
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </section>

      <!-- 发信记录 -->
      <section class="panel panel--pad" aria-label="发信记录">
        <header class="panel-head">
          <h2 class="panel-head__title">发信记录</h2>
          <span class="panel-head__meta">
            {{ recordsLoading ? "加载中…" : recordsError ? "" : `共 ${recordsTotal} 条` }}
          </span>
        </header>

        <div
          v-if="recordsLoading"
          role="status"
          aria-label="正在加载发信记录"
        >
          <el-skeleton :rows="4" animated />
        </div>
        <div v-else-if="recordsError" class="state state--error" role="alert">
          <span>{{ recordsError }}</span>
          <el-button
            v-if="!recordsUnauthorized"
            link
            type="primary"
            @click="loadRecords"
          >
            重试
          </el-button>
        </div>
        <p v-else-if="records.length === 0" class="state" role="status">
          还没有发过邮件，从上面的模板点「发送」开始
        </p>
        <template v-else>
          <el-table :data="records">
            <el-table-column prop="title" label="主题" min-width="220" show-overflow-tooltip />
            <el-table-column prop="to_email" label="收件人" min-width="180" show-overflow-tooltip />
            <el-table-column label="操作者" width="140">
              <template #default="{ row }">
                {{ operatorLabel(row.from_user_id) }}
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" width="200">
              <template #default="{ row }">
                {{ new Date(row.created_at).toLocaleString("zh-CN", { hour12: false }) }}
              </template>
            </el-table-column>
          </el-table>

          <div class="pager">
            <el-pagination
              :current-page="recordsPage"
              :page-size="recordsPageSize"
              :total="recordsTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next"
              background
              @current-change="changeRecordsPage"
              @size-change="changeRecordsPageSize"
            />
          </div>
        </template>

        <p v-if="usersError" class="self-note" role="note">
          用户名单加载失败（{{ usersError }}），记录中的操作者暂以 ID 显示，发送可用按邮箱模式
        </p>
      </section>
    </main>

    <TemplateEditorDialog
      v-model:visible="editorVisible"
      :template="editing"
      @save="onSave"
    />
    <SendMailDialog
      v-model:visible="sendVisible"
      :template="sendTarget"
      :recipients="users.map((u) => ({ user_id: u.user_id, name: u.name }))"
    />
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

.panel-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  margin: 0 0 18px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--line);
}

.panel-head__title {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.panel-head__meta {
  color: var(--ink-faint);
  font-family: var(--font-mono);
  font-size: 12px;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

.state {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 20px 0 8px;
  color: var(--ink-soft);
  font-size: 14px;
  text-align: center;
}

.state--error {
  color: var(--red);
}

.self-note {
  color: var(--ink-faint);
  font-size: 12px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

/* 变量列的标签挤在一起时换行而不是溢出 */
:deep(.el-table .el-tag) {
  margin: 2px 4px 2px 0;
}

:deep(.el-table) {
  --el-table-border-color: var(--line);
  font-size: 13px;
}

:deep(.el-table th.el-table__cell) {
  font-weight: 700;
  letter-spacing: 0.04em;
}

:deep(.el-table .cell) {
  color: var(--ink);
}

@media (max-width: 640px) {
  .panel {
    padding: 18px 16px;
  }

  .panel-head {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .pager {
    justify-content: center;
  }
}
</style>
