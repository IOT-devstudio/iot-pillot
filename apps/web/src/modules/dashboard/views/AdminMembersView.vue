<script setup lang="ts">
/**
 * 管理台 → 用户与权限 `/dashboard/admins`。
 *
 * 两块内容：
 *  用户列表   服务端分页（/api/v1/admin/users），可本地筛选当前页
 *  管理员名单 一次性读取（/api/v1/admin/admins），不分页
 *
 * 为什么筛选/搜索只作用于当前页：后端这两个接口不接受 keyword / is_admin 之类的
 * 查询参数，所以真正的过滤只能在前端做，范围就是已加载的这一页——界面上一并注明，
 * 免得以为搜的是全量。翻页仍是服务端分页，不在这里模拟。
 *
 * 角色变更的编排（确认弹窗、toast、变更后同时刷新两张表）都在
 * composables/useAdminPermissions.ts，这里只做展示与接线。
 */
import { onMounted } from "vue";

import PageHeader from "@/components/common/PageHeader.vue";
import { useAdminPermissions } from "../composables/useAdminPermissions";

const {
  users,
  total,
  page,
  pageSize,
  usersLoading,
  usersError,
  usersUnauthorized,
  filteredUsers,
  admins,
  adminsLoading,
  adminsError,
  adminsUnauthorized,
  actionFor,
  isBusy,
  keyword,
  roleFilter,
  resetFilters,
  loadAll,
  loadUsers,
  loadAdmins,
  changePage,
  changePageSize,
  grant,
  revoke,
  UNKNOWN_IDENTITY_NOTE,
} = useAdminPermissions();

/** 当前页被筛选掉后，若本来有数据却筛没了，给一句提示而不是让人以为加载失败 */
const usersFilteredEmpty = () => users.value.length > 0 && filteredUsers.value.length === 0;

onMounted(loadAll);
</script>

<template>
  <div class="page">
    <main class="page__body">
      <PageHeader
        eyebrow="iot-pillot · 权限管理"
        title="用户与权限"
        description="维护管理员名单：提升或撤销某个账号的管理权限。角色变更会让对方强制退出，需重新登录。"
      />

      <!-- 用户列表 -->
      <section class="panel panel--pad users-panel">
        <header class="panel-head">
          <h2 class="panel-head__title">用户列表</h2>
          <span class="panel-head__meta">
            共 {{ total }} 人 · 当前第 {{ page }} 页
          </span>
        </header>

        <div class="toolbar">
          <div class="toolbar__filters">
            <el-input
              v-model="keyword"
              placeholder="搜索姓名"
              clearable
              style="width: 200px"
            />
            <el-select v-model="roleFilter" style="width: 150px">
              <el-option label="全部账号" value="all" />
              <el-option label="仅管理员" value="admin" />
              <el-option label="仅普通用户" value="member" />
            </el-select>
            <span class="toolbar__hint">筛选仅作用于当前页</span>
          </div>
          <el-button text @click="resetFilters">重置筛选</el-button>
        </div>

        <!-- 用骨架屏而不是一行「正在加载…」：后者约 50px，换成表格后高度跳到
             170px 上下，会把下面的「管理员名单」整块顶下去，看着就是两次突兀的
             重排。骨架屏的高度接近真实表格，数据到位时是原地替换。 -->
        <div v-if="usersLoading" role="status" aria-label="正在加载用户列表">
          <el-skeleton :rows="5" animated />
        </div>
        <div v-else-if="usersError" class="state state--error" role="alert">
          <span>{{ usersError }}</span>
          <el-button
            v-if="!usersUnauthorized"
            link
            type="primary"
            @click="loadUsers"
          >
            重试
          </el-button>
        </div>
        <template v-else>
          <p v-if="users.length === 0" class="state" role="status">
            暂无注册用户
          </p>
          <p v-else-if="usersFilteredEmpty()" class="state" role="status">
            当前页没有符合筛选条件的用户，试试翻页或重置筛选
          </p>
          <el-table v-else :data="filteredUsers">
            <el-table-column prop="user_id" label="用户 ID" width="110" />
            <el-table-column prop="name" label="姓名" min-width="160" />
            <el-table-column prop="created_at" label="注册时间" min-width="200" />
            <el-table-column label="角色" width="110">
              <template #default="{ row }">
                <el-tag v-if="row.is_admin" type="success" effect="plain">
                  管理员
                </el-tag>
                <el-tag v-else type="info" effect="plain">普通用户</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <template v-if="actionFor(row).kind === 'grant'">
                  <el-button
                    link
                    type="primary"
                    :loading="isBusy(row.user_id)"
                    @click="grant(row)"
                  >
                    提升为管理员
                  </el-button>
                </template>
                <el-button
                  v-else-if="actionFor(row).kind === 'revoke'"
                  link
                  type="danger"
                  :loading="isBusy(row.user_id)"
                  @click="revoke(row)"
                >
                  撤销管理员
                </el-button>
                <span
                  v-else-if="actionFor(row).kind === 'self'"
                  class="self-note"
                >
                  当前登录账号，不可撤销
                </span>
                <span v-else class="self-note">
                  {{ UNKNOWN_IDENTITY_NOTE }}
                </span>
              </template>
            </el-table-column>
          </el-table>

          <!-- 分页器留在空态分支之外：翻到某一页被本地筛选筛空、或整表为空时，
               仍然需要翻页入口，否则用户只能先重置筛选才能动。 -->
          <div class="pager">
            <el-pagination
              :current-page="page"
              :page-size="pageSize"
              :total="total"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next"
              background
              @current-change="changePage"
              @size-change="changePageSize"
            />
          </div>
        </template>
      </section>

      <!-- 管理员名单 -->
      <section class="panel panel--pad admins-panel">
        <header class="panel-head">
          <h2 class="panel-head__title">管理员名单</h2>
          <span class="panel-head__meta">
            {{ adminsLoading || adminsError ? "" : `共 ${admins.length} 人` }}
          </span>
        </header>

        <div v-if="adminsLoading" role="status" aria-label="正在加载管理员名单">
          <el-skeleton :rows="3" animated />
        </div>
        <div v-else-if="adminsError" class="state state--error" role="alert">
          <span>{{ adminsError }}</span>
          <el-button
            v-if="!adminsUnauthorized"
            link
            type="primary"
            @click="loadAdmins"
          >
            重试
          </el-button>
        </div>
        <p v-else-if="admins.length === 0" class="state" role="status">
          暂无管理员
        </p>
        <el-table v-else :data="admins">
          <el-table-column prop="user_id" label="用户 ID" width="110" />
          <el-table-column prop="name" label="姓名" min-width="160" />
          <el-table-column prop="created_at" label="注册时间" min-width="200" />
          <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="actionFor(row).kind === 'revoke'"
                link
                type="danger"
                :loading="isBusy(row.user_id)"
                @click="revoke(row)"
              >
                撤销管理员
              </el-button>
              <span v-else-if="actionFor(row).kind === 'self'" class="self-note">
                当前登录账号，不可撤销
              </span>
              <span v-else class="self-note">
                {{ UNKNOWN_IDENTITY_NOTE }}
              </span>
            </template>
          </el-table-column>
        </el-table>
      </section>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: calc(100dvh - 60px);
}

.page__body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  width: 100%;
  margin: 0;
  padding: 29px 0 44px;
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

/* —— 筛选工具条 —— */
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.toolbar__filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar__hint {
  color: var(--ink-faint);
  font-size: 12px;
}

/* —— 状态占位 —— */
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

/* —— 表格套用页面令牌 —— */
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
  .page__body {
    gap: 21px;
    padding-top: 22px;
  }

  .panel {
    padding: 18px 16px;
  }

  .toolbar {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
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
