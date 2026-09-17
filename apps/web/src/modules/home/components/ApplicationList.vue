<script setup lang="ts">
/**
 * 「我的招新情况」——当前用户的报名记录列表（纯展示组件）。
 *
 * 状态枚举来自 shared-types 的 `ProspectStatus`，本模块不自定义。
 * `STATUS_LABELS` 特意标成 `Record<ProspectStatus, string>` 而不是
 * `Record<string, string>`：这样 shared-types 将来新增一个状态，
 * 这里会在 typecheck 阶段报错，而不是页面上静默少显示一种状态。
 *
 * 方向标题用 `findDirection` 现查，不在记录里冗余存一份。查不到
 * （记录指向的方向已经下线）时降级成明确的占位文案，不留空白。
 *
 * 状态小标签只是本组件内部的 span + 修饰类，没有单独拆成组件：
 * 目前只有这一处用。将来管理侧也要用同一套标签，再提取到 CLAUDE.md
 * 规划的 `components/common/` 去。
 */
import type { ProspectStatus } from "@iot-pillot/shared-types";

import type { DirectionApplication } from "../fixtures";
import { findDirection } from "../fixtures";
import { formatISODate } from "../format";

defineProps<{
  applications: DirectionApplication[];
}>();

const STATUS_LABELS: Record<ProspectStatus, string> = {
  pending: "待处理",
  invited: "已邀请",
  accepted: "已接受",
  rejected: "未通过",
};

/** 记录引用的方向可能已经下线，降级成一个明确的占位 */
function directionTitle(application: DirectionApplication): string {
  return findDirection(application.directionId)?.title ?? "（方向已下线）";
}
</script>

<template>
  <section class="applications" aria-labelledby="home-applications-title">
    <h2 id="home-applications-title" class="applications__title">我的招新情况</h2>

    <ul class="applications__list">
      <li
        v-for="application in applications"
        :key="application.id"
        class="applications__item"
      >
        <span class="applications__direction">
          {{ directionTitle(application) }}
        </span>

        <span
          class="applications__status"
          :class="`applications__status--${application.status}`"
        >
          {{ STATUS_LABELS[application.status] }}
        </span>

        <time class="applications__time" :datetime="application.appliedAt">
          {{ formatISODate(application.appliedAt) }} 报名
        </time>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.applications {
  padding: 18px 20px;
  border: 1px solid var(--home-border);
  border-radius: 10px;
  background: #fff;
}

.applications__title {
  margin: 0 0 14px;
  color: var(--home-ink);
  font-size: 15px;
}

.applications__list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.applications__item {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid var(--home-border);
  border-radius: 8px;
}

.applications__direction {
  min-width: 0;
  flex: 1 1 auto;
  color: var(--home-ink);
  font-size: 14px;
}

.applications__status {
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 11px;
  white-space: nowrap;
}

.applications__time {
  color: var(--home-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

/* 四档状态各一套配色。色值沿用仓库里已经用过的语义色：
   绿 = BuildingsEditor 的 ok / DirectionList 的 open
   红 = BuildingsEditor 的 error
   蓝 = 品牌蓝，表示「轮到你了」
   灰 = 中性，表示仍在等待 */
.applications__status--pending {
  color: var(--home-muted);
  background: #eef1f4;
}

.applications__status--invited {
  color: var(--home-accent);
  background: #e6eef7;
}

.applications__status--accepted {
  color: #1c6b45;
  background: #e9f6ef;
}

.applications__status--rejected {
  color: #8e2f27;
  background: #fbeae6;
}
</style>
