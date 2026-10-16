<script setup lang="ts">
/** 关于我们：侧栏选择内容，复用原有玻璃卡与流光动画，不新增事实数据。 */
import { ref } from "vue";
import { STUDIO_FOCUS, STUDIO_INTRO, STUDIO_MEMBERS, STUDIO_WORKS } from "../fixtures";
import FocusList from "../components/FocusList.vue";
import MemberList from "../components/MemberList.vue";
import StudioIntroCard from "../components/StudioIntroCard.vue";
import WorksList from "../components/WorksList.vue";

const sections = [
  { id: "intro", label: "工作室简介", detail: "我们是谁" },
  { id: "focus", label: "方向 / 定位", detail: "我们在探索什么" },
  { id: "works", label: "成果", detail: "把想法做成作品" },
  { id: "members", label: "成员", detail: "一起动手的人" },
] as const;
type SectionId = typeof sections[number]["id"];
const active = ref<SectionId>("intro");
const navigation = ref<HTMLElement | null>(null);

function preview(event: PointerEvent, id: SectionId): void {
  if (event.pointerType === "mouse") active.value = id;
}

function move(event: KeyboardEvent, index: number): void {
  let next: number;
  if (event.key === "ArrowDown" || event.key === "ArrowRight") next = (index + 1) % sections.length;
  else if (event.key === "ArrowUp" || event.key === "ArrowLeft") next = (index + sections.length - 1) % sections.length;
  else if (event.key === "Home") next = 0;
  else if (event.key === "End") next = sections.length - 1;
  else return;
  event.preventDefault();
  active.value = sections[next].id;
  navigation.value?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus();
}
</script>

<template>
  <div class="about">
    <div class="glass-backdrop" aria-hidden="true"></div>
    <div class="about__inner">
      <header class="about__header">
        <h1 class="about__title">关于我们</h1>
        <p class="about__subtitle">{{ STUDIO_INTRO.name }} · 招新项目手册</p>
      </header>

      <div class="about__workspace">
        <nav ref="navigation" class="about__sidebar" aria-label="关于我们栏目" role="tablist" aria-orientation="vertical">
          <button
            v-for="(section, index) in sections"
            :id="`about-tab-${section.id}`"
            :key="section.id"
            type="button"
            role="tab"
            class="about__tab liquid-glass"
            :class="{ 'about__tab--active': active === section.id }"
            :aria-selected="active === section.id"
            aria-controls="about-content"
            :tabindex="active === section.id ? 0 : -1"
            @pointerenter="preview($event, section.id)"
            @focus="active = section.id"
            @click="active = section.id"
            @keydown="move($event, index)"
          >
            <span class="about__tab-label">{{ section.label }}</span>
            <span class="about__tab-detail">{{ section.detail }}</span>
            <span class="about__tab-arrow" aria-hidden="true">↗</span>
          </button>
        </nav>

        <div
          id="about-content"
          class="about__content"
          role="tabpanel"
          :aria-labelledby="`about-tab-${active}`"
          tabindex="0"
        >
          <Transition name="about-panel" mode="out-in">
            <div :key="active" class="about__panel" :data-section="active">
              <StudioIntroCard v-if="active === 'intro'" :profile="STUDIO_INTRO" />
              <FocusList v-else-if="active === 'focus'" :focuses="STUDIO_FOCUS" />
              <WorksList v-else-if="active === 'works'" :works="STUDIO_WORKS" />
              <MemberList v-else :members="STUDIO_MEMBERS" />
            </div>
          </Transition>
        </div>
      </div>

      <p class="about__disclaimer">以上为占位示例内容，待工作室提供正式资料。</p>
    </div>
  </div>
</template>

<style scoped>
.about {
  --about-ink: #102b4e;
  --about-muted: #5c6b7a;
  --about-accent: #164d80;
  --about-border: #d8dee4;
  min-height: 100vh;
  padding: 40px clamp(16px, 4vw, 48px) 64px;
  color: var(--about-ink);
  background: #f4f6f8;
  font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", sans-serif;
}
.about__inner { position: relative; z-index: 1; max-width: 1120px; margin: 0 auto; }
.about__header { margin-bottom: 32px; }
.about__title { margin: 0; font-size: 28px; }
.about__subtitle { margin: 8px 0 0; color: var(--about-muted); font-size: 13px; }
.about__workspace { display: grid; grid-template-columns: 220px minmax(0, 1fr); align-items: start; gap: 28px; min-height: 440px; }
.about__sidebar { display: grid; gap: 12px; }
.about__tab {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 6px 10px;
  width: 100%;
  min-height: 92px;
  padding: 20px;
  color: var(--about-ink);
  text-align: left;
  font: inherit;
  cursor: pointer;
  transition: color 240ms ease, box-shadow 240ms ease;
}
.about__tab--active { color: var(--about-accent); box-shadow: 0 18px 48px -22px rgba(10, 16, 40, 0.35), inset 3px 0 0 #1739f5; }
.about__tab-label { font-size: 15px; font-weight: 600; }
.about__tab-detail { grid-column: 1; color: var(--about-muted); font-size: 11px; line-height: 1.6; }
.about__tab-arrow { grid-column: 2; grid-row: 1 / 3; align-self: center; font-size: 22px; transition: transform 240ms ease; }
.about__tab--active .about__tab-arrow { transform: translateX(3px); }
.about__tab:focus-visible, .about__content:focus-visible { outline: 2px solid #1739f5; outline-offset: 5px; }
.about__content { min-width: 0; border-radius: 24px; }
.about__panel { width: 100%; }
.about-panel-enter-active, .about-panel-leave-active { transition: opacity 220ms ease, transform 320ms cubic-bezier(.22, 1, .36, 1); }
.about-panel-enter-from { opacity: 0; transform: translateX(-16px); }
.about-panel-leave-to { opacity: 0; transform: translateX(10px); }
.about__disclaimer { margin: 24px 0 0 248px; color: var(--about-muted); font-size: 11px; line-height: 1.6; }
@media (max-width: 640px) {
  .about { padding-top: 28px; }
  .about__header { margin-bottom: 24px; }
  .about__workspace { grid-template-columns: 96px minmax(0, 1fr); gap: 14px; }
  .about__tab { display: block; min-height: 80px; padding: 16px 12px; border-radius: 18px; }
  .about__tab-label { font-size: 13px; line-height: 1.5; }
  .about__tab-detail, .about__tab-arrow { display: none; }
  .about__disclaimer { margin-left: 110px; }
  .about__panel :deep(.liquid-glass) { padding: 18px 14px; }
  .about__panel :deep(.intro__stats) { gap: 12px; }
  .about__panel :deep(.focus__item) { padding: 12px; }
}
@media (prefers-reduced-motion: reduce) {
  .about__tab, .about__tab-arrow, .about-panel-enter-active, .about-panel-leave-active { transition: none; }
  .about-panel-enter-from, .about-panel-leave-to { transform: none; }
}
</style>
