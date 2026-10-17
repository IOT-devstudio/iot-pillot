<script setup lang="ts">
import type { StudioFocus } from "../fixtures";
defineProps<{ focuses: StudioFocus[] }>();
const englishById: Record<string, string> = {
  "focus-frontend": "FRONT-END DEVELOPMENT",
  "focus-backend": "BACK-END DEVELOPMENT",
  "focus-agent": "AGENT DEVELOPMENT",
};
</script>

<template>
  <div class="focus">
    <header class="focus__head"><div><span class="focus__eyebrow">OUR FOCUS</span><h2>以全栈视野，持续探索。</h2></div><span aria-hidden="true">02</span></header>
    <p class="focus__intro">资料里讲到的三个技术主题：界面、服务，以及后端 AI 开发。</p>
    <ol class="focus__list">
      <li v-for="(item, index) in focuses" :key="item.id" class="focus__row">
        <span class="focus__number">{{ String(index + 1).padStart(2, "0") }}</span>
        <div>
          <h3>{{ item.title }}</h3>
          <small class="focus__english">{{ englishById[item.id] }}</small>
          <p>{{ item.summary }}</p>
          <p v-if="item.source" class="focus__source">{{ item.source }}</p>
          <div class="focus__topics">
            <small>学习内容</small>
            <ul class="focus__tags"><li v-for="topic in item.topics" :key="topic">{{ topic }}</li></ul>
          </div>
        </div>
      </li>
    </ol>
    <p class="focus__note">技术主题依据工作室知识库与 2025 年宣讲资料整理。标签是学习内容，不是工作室的正式技术栈；本轮可报名的方向以招新页面为准。</p>
  </div>
</template>

<style scoped>
.focus { min-height: 540px; padding: 32px 36px; border: 1px solid white; border-radius: 24px; background: #ffffff91; box-shadow: 0 18px 48px -32px #0a102038, inset 0 1px 0 white; }
.focus__head { display: flex; justify-content: space-between; gap: 24px; margin-bottom: 26px; }
.focus__eyebrow { display: block; margin-bottom: 14px; color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.focus__head h2 { margin: 0; color: var(--about-ink); font-size: 30px; line-height: 1.5; }
.focus__head > span { color: var(--about-blue); font-size: 44px; font-weight: 300; }
.focus__intro { max-width: 680px; margin: 0; color: var(--about-muted); line-height: 2.1; }
.focus__list { margin: 24px 0 0; padding: 0; border-top: 1px solid var(--about-line); list-style: none; }
.focus__row { display: grid; grid-template-columns: 36px minmax(0, 1fr); gap: 12px; padding: 24px 0; border-bottom: 1px solid var(--about-line); }
.focus__number { color: var(--about-blue); font: 16px var(--about-mono); }
.focus__row h3 { margin: 0; color: var(--about-ink); font-size: 22px; }
.focus__english { color: var(--about-muted); font: 10px var(--about-mono); letter-spacing: 1px; }
.focus__row p { margin: 10px 0 0; color: var(--about-muted); font-size: 13px; line-height: 1.8; }
.focus__row .focus__source { margin-top: 8px; color: var(--about-blue); font: 10px/1.7 var(--about-mono); letter-spacing: .5px; }
.focus__topics { display: flex; flex-wrap: wrap; align-items: baseline; gap: 7px 10px; margin-top: 16px; }
.focus__topics small { color: var(--about-muted); font-size: 10px; }
.focus__tags { display: flex; flex-wrap: wrap; gap: 7px; margin: 0; padding: 0; list-style: none; }
.focus__tags li { padding: 2px 9px; border: 1px solid var(--about-line); background: var(--about-paper); color: var(--about-ink); font-size: 10px; }
.focus__note { margin: 22px 0 0; color: var(--about-muted); font-size: 10px; }
@media (max-width: 540px) { .focus { min-height: 450px; padding: 26px 24px; } .focus__head h2 { font-size: 25px; } .focus__head > span { font-size: 34px; } .focus__row { grid-template-columns: 24px minmax(0, 1fr); gap: 8px; } }
</style>
