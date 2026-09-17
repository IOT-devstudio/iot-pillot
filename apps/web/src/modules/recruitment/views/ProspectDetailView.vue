<script setup lang="ts">
/**
 * 意向成员详情。
 *
 * 基础信息（姓名/邮箱/班级/报名时间/发展意向/参加原因，来自报名表单，先写死）、
 * 当前状态、按状态可推进的动作，以及带备注的邮件记录。
 */
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";

import { formatDateTime } from "../fixture";
import { emailTypeMeta } from "../status";
import { findProspect } from "../store";
import EmailSendDialog from "../components/EmailSendDialog.vue";
import StatusTag from "../components/StatusTag.vue";
import { useProspectActions } from "../composables/useProspectActions";

const route = useRoute();
const router = useRouter();

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
  <div class="detail-page">
    <div class="page-head">
      <el-button link @click="router.push('/recruitment/prospects')">← 返回列表</el-button>
    </div>

    <el-empty v-if="!prospect" description="未找到该意向成员" />

    <template v-else>
      <div class="head-card">
        <div class="head-title">
          <h2>{{ prospect.name }}</h2>
          <StatusTag :status="prospect.status" />
        </div>
        <p class="muted">{{ prospect.email }}</p>
      </div>

      <el-row :gutter="16">
        <el-col :xs="24" :md="16">
          <el-card shadow="never" header="基础信息">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="姓名">{{ prospect.name }}</el-descriptions-item>
              <el-descriptions-item label="邮箱">{{ prospect.email }}</el-descriptions-item>
              <el-descriptions-item label="班级">{{ prospect.className }}</el-descriptions-item>
              <el-descriptions-item label="报名时间">{{ formatDateTime(prospect.registeredAt) }}</el-descriptions-item>
              <el-descriptions-item label="发展意向">{{ prospect.intention }}</el-descriptions-item>
              <el-descriptions-item label="参加原因" :span="2">{{ prospect.reason }}</el-descriptions-item>
            </el-descriptions>
          </el-card>

          <el-card shadow="never" header="邮件记录" class="mt">
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
          </el-card>
        </el-col>

        <el-col :xs="24" :md="8">
          <el-card shadow="never" header="推进动作">
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
          </el-card>
        </el-col>
      </el-row>
    </template>

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
.detail-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.head-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.head-title {
  display: flex;
  align-items: center;
  gap: 12px;
}
.head-title h2 {
  margin: 0;
}
.mt {
  margin-top: 16px;
}
.muted {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.mail-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mail-subject {
  font-weight: 500;
}
.mail-note {
  margin-top: 4px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
</style>
