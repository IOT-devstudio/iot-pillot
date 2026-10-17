<script setup lang="ts">
import type { StudioMember } from "../fixtures";
defineProps<{ members: StudioMember[] }>();
</script>

<template>
  <div class="members">
    <header class="members__head"><div><span class="members__eyebrow">OUR PEOPLE</span><h2>一起动手的人。</h2></div><span aria-hidden="true">04</span></header>
    <div v-if="!members.length" class="members__empty">
      <span class="members__icon" aria-hidden="true">○</span>
      <div><h3>成员资料待补充</h3><p>正式成员名单确认后展示。</p><small>PEOPLE BEHIND THE WORK</small></div>
    </div>
    <ul v-else class="members__list">
      <li v-for="member in members" :key="member.id">
        <span class="members__avatar" aria-hidden="true">{{ member.name.slice(0, 1) }}</span>
        <div><h3>{{ member.name }}</h3><b>{{ member.title }}</b><p>{{ member.bio }}</p></div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.members { min-height: 540px; padding: 32px 36px; border: 1px solid white; border-radius: 24px; background: #ffffff91; box-shadow: 0 18px 48px -32px #0a102038, inset 0 1px 0 white; }
.members__head { display: flex; justify-content: space-between; gap: 24px; margin-bottom: 26px; }
.members__eyebrow { display: block; margin-bottom: 14px; color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.members__head h2 { margin: 0; color: var(--about-ink); font-size: 30px; line-height: 1.5; }
.members__head > span { color: var(--about-blue); font-size: 44px; font-weight: 300; }
.members__empty { display: flex; align-items: center; gap: 25px; margin-top: 36px; padding: 42px 0; border-block: 1px solid var(--about-line); }
.members__icon { display: grid; flex: 0 0 70px; place-items: center; width: 70px; height: 86px; background: var(--about-yellow); color: var(--about-blue); font-size: 30px; }
.members__empty h3 { margin: 0 0 10px; font-size: 23px; }
.members__empty p { margin: 0; color: var(--about-muted); font-size: 13px; }
.members__empty small { display: block; margin-top: 16px; color: var(--about-blue); font: 9px/1.6 var(--about-mono); letter-spacing: 1px; }
.members__list { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 36px 0 0; padding: 0; list-style: none; }
.members__list li { display: flex; gap: 12px; padding: 24px 0; border-top: 1px solid var(--about-line); }
.members__avatar { display: grid; flex: 0 0 44px; place-items: center; width: 44px; height: 44px; background: var(--about-yellow); color: var(--about-blue); font-size: 18px; }
.members__list h3 { margin: 0; font-size: 21px; }
.members__list b { color: var(--about-blue); font-size: 11px; }
.members__list p { margin: 10px 0 0; color: var(--about-muted); font-size: 13px; }
@media (max-width: 540px) { .members { min-height: 450px; padding: 26px 24px; } .members__head h2 { font-size: 25px; } .members__head > span { font-size: 34px; } .members__empty { align-items: start; gap: 16px; } .members__icon { flex-basis: 44px; width: 44px; height: 60px; font-size: 23px; } .members__empty h3 { font-size: 20px; } }
</style>
