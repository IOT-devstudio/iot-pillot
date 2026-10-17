<script setup lang="ts">
import type { StudioRole } from "../fixtures";

defineProps<{ roles: StudioRole[] }>();
</script>

<template>
  <div class="roles">
    <header class="roles__head"><div><span class="roles__eyebrow">TEAM ROLES</span><h2>谁负责什么。</h2></div><span aria-hidden="true">04</span></header>
    <p class="roles__intro">工作室按岗位分工推进招新、例会、知识库与项目对接。</p>
    <ul class="roles__list">
      <li v-for="(role, index) in roles" :key="role.id">
        <small>{{ String(index + 1).padStart(2, "0") }}</small>
        <h3>{{ role.title }}</h3>
        <p>{{ role.summary }}</p>
        <ul class="roles__duties">
          <li v-for="duty in role.duties" :key="duty">{{ duty }}</li>
        </ul>
      </li>
    </ul>
    <p class="roles__note">以上为岗位职责，不是现任成员名单，也不代表未经确认的人数；与系统的 admin / member 权限角色无关。</p>
  </div>
</template>

<style scoped>
.roles { min-height: 540px; padding: 32px 36px; border: 1px solid white; border-radius: 24px; background: #ffffff91; box-shadow: 0 18px 48px -32px #0a102038, inset 0 1px 0 white; }
.roles__head { display: flex; justify-content: space-between; gap: 24px; margin-bottom: 26px; }
.roles__eyebrow { display: block; margin-bottom: 14px; color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.roles__head h2 { margin: 0; color: var(--about-ink); font-size: 30px; line-height: 1.5; }
.roles__head > span { color: var(--about-blue); font-size: 44px; font-weight: 300; }
.roles__intro { max-width: 680px; margin: 0; color: var(--about-muted); line-height: 2.1; }
.roles__list { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; margin: 26px 0 0; padding: 0; list-style: none; }
.roles__list > li { min-width: 0; padding: 22px 20px; border: 1px solid var(--about-line); background: #ffffffe0; }
.roles__list > li > small { color: var(--about-blue); font: 11px var(--about-mono); letter-spacing: 1px; }
.roles__list h3 { margin: 12px 0 8px; color: var(--about-ink); font-size: 19px; }
.roles__list p { margin: 0; color: var(--about-muted); font-size: 12px; line-height: 1.8; }
.roles__duties { display: flex; flex-wrap: wrap; gap: 6px; margin: 14px 0 0; padding: 0; list-style: none; }
.roles__duties li { padding: 2px 8px; border: 1px solid var(--about-line); background: var(--about-paper); color: var(--about-ink); font-size: 10px; }
.roles__note { margin: 22px 0 0; color: var(--about-muted); font-size: 10px; line-height: 1.7; }
@media (max-width: 900px) { .roles__list { grid-template-columns: 1fr; gap: 12px; } }
@media (max-width: 540px) { .roles { min-height: 450px; padding: 26px 24px; } .roles__head h2 { font-size: 25px; } .roles__head > span { font-size: 34px; } .roles__list > li { padding: 18px 16px; } }
</style>
