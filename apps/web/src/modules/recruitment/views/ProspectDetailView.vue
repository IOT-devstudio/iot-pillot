<script setup lang="ts">
/**
 * 意向成员详情。
 *
 * 基础信息（姓名/邮箱/班级/报名时间/发展意向/参加原因，来自报名表单，先写死）、
 * 当前状态、按状态可推进的动作，以及带备注的邮件记录。
 */
import { computed } from "vue";
import { useRoute } from "vue-router";

import AppHeader from "@/components/common/AppHeader.vue";
import BackToDashboard from "@/components/common/BackToDashboard.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { formatDateTime } from "../fixture";
import { emailTypeMeta } from "../status";
import { findProspect } from "../store";
import EmailSendDialog from "../components/EmailSendDialog.vue";
import StatusTag from "../components/StatusTag.vue";
import { useProspectActions } from "../composables/useProspectActions";

const route = useRoute();
const prospect = computed(() => findProspect(String(route.params.id)));

const {
  visible,
  target,
  type,
  title,
  defaultSubject,
  defaultBody,
  openEmail,
  onSend,
  markPassed,
} = useProspectActions();
</script>

<template>
  <div class="page">
    <AppHeader />

    <main class="page__body">
      <!-- 详情页的上一层是列表，所以覆盖默认的「返回控制台」目标与文案 -->
      <BackToDashboard to="/recruitment/prospects" label="← 返回意向成员列表" />

      <el-empty v-if="!prospect" description="未找到该意向成员" />

      <template v-else>
        <PageHeader
          eyebrow="招新管理 · 意向档案"
          :title="prospect.name"
          :description="`${prospect.email} · ${prospect.className}`"
        >
          <template #actions>
            <StatusTag :status="prospect.status" />
          </template>
        </PageHeader>

        <el-row :gutter="20">
          <el-col :xs="24" :md="16">
            <section class="panel panel--pad">
              <h2 class="panel__title">基础信息</h2>
              <el-descriptions :column="2" border>
                <el-descriptions-item label="姓名">{{ prospect.name }}</el-descriptions-item>
                <el-descriptions-item label="邮箱">{{ prospect.email }}</el-descriptions-item>
                <el-descriptions-item label="班级">{{ prospect.className }}</el-descriptions-item>
                <el-descriptions-item label="报名时间">{{ formatDateTime(prospect.registeredAt) }}</el-descriptions-item>
                <el-descriptions-item label="发展意向">{{ prospect.intention }}</el-descriptions-item>
                <el-descriptions-item label="参加原因" :span="2">{{ prospect.reason }}</el-descriptions-item>
              </el-descriptions>
            </section>

            <section class="panel panel--pad">
              <h2 class="panel__title">邮件记录</h2>
              <el-empty v-if="prospect.emails.length === 0" description="暂无邮件记录" />
              <el-timeline v-else>
                <el-timeline-item
                  v-for="rec in prospect.emails"
                  :key="rec.id"
                  :timestamp="formatDateTime(rec.sentAt)"
                >
                  <div class="mail-row">
                    <el-tag :type="emailTypeMeta(rec.type).tagType" size="small">
                      {{ emailTypeMeta(rec.type).label }}
                    </el-tag>
                    <span class="mail-subject">{{ rec.subject }}</span>
                  </div>
                  <div class="mail-note">备注：{{ rec.note }}</div>
                </el-timeline-item>
              </el-timeline>
            </section>
          </el-col>

          <el-col :xs="24" :md="8">
            <section class="panel panel--pad">
              <h2 class="panel__title">推进动作</h2>
              <div class="actions">
                <template v-if="prospect.status === 'ready_first'">
                  <el-button type="primary" plain @click="markPassed(prospect)">通过一面</el-button>
                  <el-button type="danger" plain @click="openEmail(prospect, 'reject')">拒录</el-button>
                </template>
                <template v-else-if="prospect.status === 'first_passed'">
                  <el-button type="primary" @click="openEmail(prospect, 'pass')">发送通过邮件</el-button>
                  <p class="muted">发送后状态将变为「准备二面」。</p>
                </template>
                <template v-else-if="prospect.status === 'ready_second'">
                  <el-button type="primary" @click="openEmail(prospect, 'pass')">录用</el-button>
                  <el-button type="danger" plain @click="openEmail(prospect, 'reject')">拒录</el-button>
                  <p class="muted">录用将发送通过邮件；拒录将发送拒录邮件。</p>
                </template>
                <p v-else class="muted">该成员流程已结束，无可推进动作。</p>
              </div>
            </section>
          </el-col>
        </el-row>
      </template>
    </main>

    <EmailSendDialog
      v-model="visible"
      :type="type"
      :recipient-name="target?.name ?? ''"
      :title="title"
      :default-subject="defaultSubject"
      :default-body="defaultBody"
      @send="onSend"
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
  gap: 24px;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(32px, 5vw, 56px) clamp(20px, 4vw, 48px) 72px;
}

.panel__title {
  margin: 0 0 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--line);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.panel--pad + .panel--pad {
  margin-top: 20px;
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.muted {
  color: var(--ink-faint);
  font-size: 13px;
  line-height: 1.6;
}

.mail-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.mail-subject {
  font-weight: 500;
}

.mail-note {
  margin-top: 4px;
  color: var(--ink-soft);
  font-size: 13px;
}

:deep(.el-descriptions) {
  --el-descriptions-item-bordered-label-background: #f5f2ea;
}

:deep(.el-descriptions__label) {
  font-weight: 600;
}

@media (max-width: 768px) {
  .panel--pad + .panel--pad {
    margin-top: 20px;
  }
}
</style>
