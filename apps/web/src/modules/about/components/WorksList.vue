<script setup lang="ts">
import type { StudioWork } from "../fixtures";
defineProps<{ works: StudioWork[] }>();
</script>

<template>
  <div class="works">
    <header class="works__head"><div><span class="works__eyebrow">SELECTED WORK</span><h2>让作品，承载想法。</h2></div><span aria-hidden="true">03</span></header>
    <div v-if="!works.length" class="works__empty">
      <span class="works__icon" aria-hidden="true">▱</span>
      <div><h3>成果资料待补充</h3><p>正式项目清单确认后展示。</p><small>WORK SPEAKS FOR ITSELF</small></div>
    </div>
    <ul v-else class="works__list">
      <li v-for="work in works" :key="work.id"><div><h3>{{ work.title }}</h3><span>{{ work.year }}</span></div><p>{{ work.summary }}</p></li>
    </ul>
  </div>
</template>

<style scoped>
.works { min-height: 540px; padding: 32px 36px; border: 1px solid white; border-radius: 24px; background: #ffffff91; box-shadow: 0 18px 48px -32px #0a102038, inset 0 1px 0 white; }
.works__head { display: flex; justify-content: space-between; gap: 24px; margin-bottom: 26px; }
.works__eyebrow { display: block; margin-bottom: 14px; color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.works__head h2 { margin: 0; color: var(--about-ink); font-size: 30px; line-height: 1.5; }
.works__head > span { color: var(--about-blue); font-size: 44px; font-weight: 300; }
.works__empty { display: flex; align-items: center; gap: 25px; margin-top: 36px; padding: 42px 0; border-block: 1px solid var(--about-line); }
.works__icon { display: grid; flex: 0 0 70px; place-items: center; width: 70px; height: 86px; background: var(--about-yellow); color: var(--about-blue); font-size: 30px; }
.works__empty h3 { margin: 0 0 10px; font-size: 23px; }
.works__empty p { margin: 0; color: var(--about-muted); font-size: 13px; }
.works__empty small { display: block; margin-top: 16px; color: var(--about-blue); font: 9px/1.6 var(--about-mono); letter-spacing: 1px; }
.works__list { margin: 36px 0 0; padding: 0; border-top: 1px solid var(--about-line); list-style: none; }
.works__list li { padding: 24px 0; border-bottom: 1px solid var(--about-line); }
.works__list li > div { display: flex; justify-content: space-between; gap: 16px; }
.works__list h3 { margin: 0; font-size: 21px; }
.works__list span { color: var(--about-blue); font: 12px var(--about-mono); }
.works__list p { margin: 10px 0 0; color: var(--about-muted); font-size: 13px; }
@media (max-width: 540px) { .works { min-height: 450px; padding: 26px 24px; } .works__head h2 { font-size: 25px; } .works__head > span { font-size: 34px; } .works__empty { align-items: start; gap: 16px; } .works__icon { flex-basis: 44px; width: 44px; height: 60px; font-size: 23px; } .works__empty h3 { font-size: 20px; } }
</style>
