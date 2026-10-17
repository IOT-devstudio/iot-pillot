<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import {
  STUDIO_ACHIEVEMENTS,
  STUDIO_FOCUS,
  STUDIO_INTRO,
  STUDIO_ROLES,
  STUDIO_WORKS,
} from "../fixtures";
import FocusList from "../components/FocusList.vue";
import RoleList from "../components/RoleList.vue";
import StudioIntroCard from "../components/StudioIntroCard.vue";
import WorksList from "../components/WorksList.vue";

const sections = [
  { id: "intro", label: "工作室简介", english: "THE STUDIO" },
  { id: "focus", label: "技术主题", english: "OUR FOCUS" },
  { id: "works", label: "项目成果", english: "SELECTED WORK" },
  { id: "roles", label: "团队分工", english: "TEAM ROLES" },
] as const;
type SectionId = typeof sections[number]["id"];
const active = ref<SectionId>("intro");
const navigation = ref<HTMLElement | null>(null);
const isMobile = ref(false);
let mobileQuery: MediaQueryList | undefined;
function syncMobile(): void { isMobile.value = mobileQuery?.matches ?? false; }
onMounted(() => {
  mobileQuery = window.matchMedia("(max-width: 540px)");
  syncMobile();
  mobileQuery.addEventListener("change", syncMobile);
});
onUnmounted(() => mobileQuery?.removeEventListener("change", syncMobile));

function preview(event: PointerEvent, id: SectionId): void {
  if (event.pointerType === "mouse" && !navigation.value?.contains(document.activeElement)) active.value = id;
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
  <main class="about">
    <div class="glass-backdrop" aria-hidden="true"></div>
    <div class="about__inner">
      <header class="about__heading">
        <div>
          <span class="about__eyebrow">THE STUDIO / 关于我们</span>
          <h1>以技术，回应真实问题。</h1>
        </div>
        <p>认识工作室，了解我们的探索方向。</p>
      </header>

      <div class="about__workspace">
        <aside class="about__aside">
          <nav ref="navigation" class="about__sidebar" aria-label="关于我们栏目" role="tablist" :aria-orientation="isMobile ? 'horizontal' : 'vertical'">
            <button
              v-for="(section, index) in sections"
              :id="`about-tab-${section.id}`"
              :key="section.id"
              type="button"
              role="tab"
              class="about__tab"
              :aria-selected="active === section.id"
              :aria-controls="`about-content-${section.id}`"
              :tabindex="active === section.id ? 0 : -1"
              @pointerenter="preview($event, section.id)"
              @focus="active = section.id"
              @click="active = section.id"
              @keydown="move($event, index)"
            >
              <span class="about__tab-num">{{ String(index + 1).padStart(2, "0") }}</span>
              <span class="about__tab-copy"><strong>{{ section.label }}</strong><small>{{ section.english }}</small></span>
              <span class="about__tab-arrow" aria-hidden="true">↗</span>
            </button>
          </nav>
          <p class="about__side-note"><b>IDEAS INTO REALITY.</b>从理解一个问题开始，<br>到完成一件作品。</p>
        </aside>

        <div class="about__content">
          <section
            v-for="section in sections"
            :id="`about-content-${section.id}`"
            :key="section.id"
            class="about__panel"
            :data-section="section.id"
            role="tabpanel"
            :aria-labelledby="`about-tab-${section.id}`"
            :tabindex="active === section.id ? 0 : -1"
            :hidden="active !== section.id"
            :class="{ 'about__panel--enter': active === section.id }"
          >
            <StudioIntroCard v-if="section.id === 'intro'" :profile="STUDIO_INTRO" />
            <FocusList v-else-if="section.id === 'focus'" :focuses="STUDIO_FOCUS" />
            <WorksList v-else-if="section.id === 'works'" :works="STUDIO_WORKS" :achievements="STUDIO_ACHIEVEMENTS" />
            <RoleList v-else :roles="STUDIO_ROLES" />
          </section>
          <p class="about__note">关于工作室 / 内容依据工作室知识库与 2025 年宣讲资料整理，最新口径以工作室确认为准。</p>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
.about {
  --about-blue: #1739f5;
  --about-yellow: #e9ff6c;
  --about-paper: #f3f4ed;
  --about-ink: #141a32;
  --about-muted: #666b62;
  --about-line: #d2d5ca;
  --about-mono: "Cascadia Code", Consolas, monospace;
  position: relative;
  min-height: 100vh;
  padding: 44px clamp(16px, 5vw, 70px) 64px;
  color: var(--about-ink);
  background: var(--about-paper);
  font: 14px/1.85 Bahnschrift, "Microsoft YaHei", "PingFang SC", sans-serif;
}
.about__inner { position: relative; z-index: 1; width: 100%; max-width: 1240px; margin: 0 auto; zoom: 1.2; }
.about__heading { display: flex; align-items: end; justify-content: space-between; gap: 24px; margin-bottom: 32px; }
.about__eyebrow { display: flex; align-items: center; gap: 12px; color: var(--about-blue); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.about__eyebrow::before { content: ""; width: 24px; height: 2px; background: currentColor; }
.about__heading h1 { margin: 12px 0 0; font-size: clamp(25px, 3vw, 32px); line-height: 1.4; }
.about__heading p { margin: 0; color: var(--about-muted); font-size: 11px; }
.about__workspace { display: grid; grid-template-columns: 205px minmax(0, 1fr); gap: 36px; align-items: start; }
.about__sidebar { display: flex; flex-direction: column; border-top: 1px solid var(--about-line); }
.about__tab { display: grid; grid-template-columns: 24px minmax(0, 1fr) 15px; align-items: center; gap: 10px; width: 100%; padding: 24px 10px 24px 0; border: 0; border-bottom: 1px solid var(--about-line); background: transparent; color: var(--about-muted); text-align: left; font: inherit; cursor: pointer; transition: background .2s, color .2s; }
.about__tab:hover { background: var(--about-yellow); color: var(--about-ink); }
.about__tab[aria-selected="true"] { padding-left: 12px; background: var(--about-blue); color: white; }
.about__tab-num { color: var(--about-blue); font: 12px var(--about-mono); }
.about__tab[aria-selected="true"] .about__tab-num { color: var(--about-yellow); }
.about__tab-copy strong { display: block; font-size: 14px; font-weight: 600; }
.about__tab-copy small { display: block; margin-top: 6px; font: 9px/1.5 var(--about-mono); letter-spacing: 1px; }
.about__tab-arrow { opacity: 0; }
.about__tab[aria-selected="true"] .about__tab-arrow { color: var(--about-yellow); opacity: 1; }
.about__tab:focus-visible, .about__panel:focus-visible { outline: 3px solid var(--about-blue); outline-offset: 3px; }
.about__side-note { margin: 40px 0 0; color: var(--about-muted); font-size: 10px; line-height: 2; }
.about__side-note b { display: block; margin-bottom: 14px; color: var(--about-blue); font: 9px/1.6 var(--about-mono); letter-spacing: 1px; }
.about__content { min-width: 0; }
.about__panel[hidden] { display: none; }
.about__panel--enter { animation: about-panel-reveal .3s ease both; }
@keyframes about-panel-reveal { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
.about__note { margin: 22px 0 0; color: var(--about-muted); font-size: 10px; }
@media (max-width: 900px) { .about__workspace { grid-template-columns: 175px minmax(0, 1fr); gap: 24px; } .about__heading p { display: none; } }
@media (max-width: 540px) {
  .about { padding: 26px 16px 45px; }
  .about__heading { margin-bottom: 22px; }
  .about__eyebrow { font-size: 9px; letter-spacing: 1px; }
  .about__workspace { display: block; }
  .about__sidebar { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin-bottom: 20px; }
  .about__tab, .about__tab[aria-selected="true"] { display: block; min-width: 0; padding: 13px 2px; text-align: center; }
  .about__tab-num { display: block; margin-bottom: 5px; font-size: 10px; }
  .about__tab-copy strong { font-size: 12px; line-height: 1.4; }
  .about__tab-copy small, .about__tab-arrow, .about__side-note { display: none; }
}
@media (prefers-reduced-motion: reduce) { .about__tab { transition: none; } .about__panel--enter { animation: none; } }
</style>
