<script setup lang="ts">
/**
 * 工作室简介（纯展示组件）。
 *
 * 主张与描述沿用登录页品牌面板已有的文案，口径与那边保持一致。
 * 组件不持状态、不发请求，数据由页面传入。
 */
import type { StudioProfile } from "../fixtures";

defineProps<{
  profile: StudioProfile;
}>();
</script>

<template>
  <section class="intro" aria-labelledby="about-intro-title">
    <h2 id="about-intro-title" class="intro__title">工作室简介</h2>

    <p class="intro__tagline">{{ profile.tagline }}</p>
    <p class="intro__description">{{ profile.description }}</p>

    <dl class="intro__stats">
      <div v-for="stat in profile.stats" :key="stat.label" class="intro__stat">
        <dt class="intro__stat-label">{{ stat.label }}</dt>
        <dd class="intro__stat-value">{{ stat.value }}</dd>
      </div>
    </dl>
  </section>
</template>

<style scoped>
.intro {
  padding: 18px 20px;
  border: 1px solid var(--about-border);
  border-radius: 10px;
  background: #fff;
}

.intro__title {
  margin: 0 0 14px;
  color: var(--about-ink);
  font-size: 15px;
}

.intro__tagline {
  margin: 0 0 8px;
  color: var(--about-accent);
  font-size: 17px;
  font-weight: 600;
  line-height: 1.5;
}

.intro__description {
  margin: 0;
  color: var(--about-muted);
  font-size: 13px;
  line-height: 1.7;
}

.intro__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 40px;
  margin: 18px 0 0;
  padding-top: 16px;
  border-top: 1px solid var(--about-border);
}

.intro__stat {
  min-width: 72px;
}

.intro__stat-label {
  margin-bottom: 4px;
  color: var(--about-muted);
  font-size: 12px;
}

.intro__stat-value {
  margin: 0;
  color: var(--about-ink);
  font-size: 20px;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}
</style>
