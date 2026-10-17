<script setup lang="ts">
import { ACHIEVEMENT_SCOPE } from "../fixtures";
import type { StudioAchievement, StudioWork } from "../fixtures";

defineProps<{ works: StudioWork[]; achievements: StudioAchievement[] }>();
</script>

<template>
  <div class="works">
    <header class="works__head"><div><span class="works__eyebrow">SELECTED WORK</span><h2>让作品，承载想法。</h2></div><span aria-hidden="true">03</span></header>

    <section class="works__achievements" aria-labelledby="works-achievements-title">
      <div class="works__scope">
        <h3 id="works-achievements-title">成果统计</h3>
        <b>{{ ACHIEVEMENT_SCOPE }}</b>
      </div>
      <dl class="works__stats">
        <div v-for="item in achievements" :key="item.id">
          <dt>{{ item.label }}</dt>
          <dd>{{ item.value }}</dd>
        </div>
      </dl>
    </section>

    <ul class="works__list">
      <li v-for="work in works" :key="work.id">
        <div><h3>{{ work.title }}</h3><span class="works__source">资料来源：{{ work.source }}</span></div>
        <p>{{ work.summary }}</p>
      </li>
    </ul>

    <p class="works__note">资料来源时间是资料整理的时间，不是项目实施年份；累计项目数为历史统计，不代表当前在做项目。</p>
  </div>
</template>

<style scoped>
.works { min-height: 540px; padding: 32px 36px; border: 1px solid white; border-radius: 24px; background: #ffffff91; box-shadow: 0 18px 48px -32px #0a102038, inset 0 1px 0 white; }
.works__head { display: flex; justify-content: space-between; gap: 24px; margin-bottom: 26px; }
.works__eyebrow { display: block; margin-bottom: 14px; color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.works__head h2 { margin: 0; color: var(--about-ink); font-size: 30px; line-height: 1.5; }
.works__head > span { color: var(--about-blue); font-size: 44px; font-weight: 300; }
.works__achievements { margin-top: 4px; padding: 18px 0 20px; border-block: 1px solid var(--about-line); }
.works__scope { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.works__scope h3 { margin: 0; color: var(--about-ink); font-size: 13px; }
.works__scope b { color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 1px; }
.works__stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin: 14px 0 0; }
.works__stats > div { min-width: 0; }
.works__stats dt { color: var(--about-muted); font-size: 11px; line-height: 1.6; }
.works__stats dd { margin: 4px 0 0; color: var(--about-ink); font-size: 20px; font-weight: 600; }
.works__list { margin: 26px 0 0; padding: 0; border-top: 1px solid var(--about-line); list-style: none; }
.works__list li { padding: 22px 0; border-bottom: 1px solid var(--about-line); }
.works__list li > div { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 6px 16px; }
.works__list h3 { margin: 0; font-size: 19px; }
.works__source { color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: .5px; }
.works__list p { margin: 8px 0 0; color: var(--about-muted); font-size: 13px; }
.works__note { margin: 20px 0 0; color: var(--about-muted); font-size: 10px; line-height: 1.7; }
@media (max-width: 900px) { .works__stats { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 12px; } }
@media (max-width: 540px) { .works { min-height: 450px; padding: 26px 24px; } .works__head h2 { font-size: 25px; } .works__head > span { font-size: 34px; } .works__stats dd { font-size: 18px; } .works__list li > div { display: block; } .works__source { display: block; margin-top: 6px; } }
</style>
