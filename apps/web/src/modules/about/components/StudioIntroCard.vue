<script setup lang="ts">
import type { StudioProfile } from "../fixtures";
import structureImage from "../assets/studio-structure.svg";

defineProps<{ profile: StudioProfile }>();
</script>

<template>
  <div class="intro">
    <div class="intro__hero">
      <div class="intro__grid">
        <div class="intro__hero-copy">
          <span class="intro__eyebrow">ABOUT THE STUDIO</span>
          <h2>{{ profile.tagline.split("，")[0] }}，<br>{{ profile.tagline.split("，")[1] }}</h2>
          <p>以前端与后端开发为核心，围绕真实问题，<br>探索技术的可能，完成有价值的交付。</p>
          <span class="intro__word">IDEAS INTO REALITY.</span>
        </div>
        <div class="intro__art" aria-hidden="true">
          <img :src="structureImage" alt="">
          <small>INTERFACE / SERVICE / INTELLIGENCE</small>
        </div>
      </div>
      <div class="intro__hero-bottom"><span>IOT · FULL STACK STUDIO</span><span>技术探索 · 项目实践 · 协作交付</span></div>
    </div>
    <div class="intro__body">
      <p class="intro__description">{{ profile.description }}</p>
      <dl class="intro__stats">
        <div v-for="stat in profile.stats" :key="stat.label">
          <dt>{{ stat.label }}</dt>
          <dd>{{ stat.value }}</dd>
        </div>
      </dl>
      <p class="intro__stats-note">{{ profile.statsNote }}</p>
      <div class="intro__principles">
        <div><small>01 / DEFINE</small><h3>理解问题</h3><p>从真实需求出发，明确问题与目标。</p></div>
        <div><small>02 / BUILD</small><h3>动手构建</h3><p>写代码、联调服务，让想法逐步成形。</p></div>
        <div><small>03 / DELIVER</small><h3>完成交付</h3><p>把结果交付给真正的用户。</p></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.intro__hero { position: relative; isolation: isolate; overflow: hidden; background: var(--about-blue); color: var(--about-paper); }
.intro__hero::before { content: ""; position: absolute; inset: 0; z-index: -1; background-image: linear-gradient(90deg, #ffffff18 1px, transparent 1px), linear-gradient(#ffffff18 1px, transparent 1px); background-size: 20% 25%; }
.intro__grid { display: grid; grid-template-columns: 1.2fr 1fr; align-items: center; min-height: 365px; }
.intro__hero-copy { padding: 36px 0 36px 34px; }
.intro__eyebrow { display: flex; align-items: center; gap: 12px; color: var(--about-yellow); font: 10px/1.6 var(--about-mono); letter-spacing: 2px; }
.intro__eyebrow::before { content: ""; width: 24px; height: 2px; background: currentColor; }
.intro__hero h2 { margin: 26px 0 20px; font-size: clamp(30px, 3.1vw, 45px); line-height: 1.4; }
.intro__hero-copy p { max-width: 430px; margin: 0; color: #edf0ff; font-size: 12px; line-height: 2.1; }
.intro__word { display: block; margin-top: 28px; color: var(--about-yellow); font: 9px/1.6 var(--about-mono); letter-spacing: 3px; }
.intro__art { min-width: 0; padding: 10px; }
.intro__art img { display: block; width: 100%; max-height: 380px; }
.intro__art small { display: block; color: #edf0ff; text-align: right; font: 7px/1.5 var(--about-mono); letter-spacing: 1px; }
.intro__hero-bottom { display: flex; justify-content: space-between; gap: 20px; margin: 0 30px; padding: 18px 0; border-top: 1px solid #ffffff44; color: #edf0ff; font: 8px/1.6 var(--about-mono); letter-spacing: 1px; }
.intro__hero-bottom span:first-child { color: var(--about-yellow); }
.intro__body { padding: 32px 36px; border: 1px solid white; background: #ffffff91; box-shadow: 0 18px 48px -32px #0a102038, inset 0 1px 0 white; }
.intro__description { max-width: 680px; margin: 0; color: var(--about-muted); font-size: 14px; line-height: 2.1; }
.intro__stats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 24px 0 0; padding: 18px 0; border-block: 1px solid var(--about-line); }
.intro__stats > div { min-width: 0; }
.intro__stats dt { color: var(--about-muted); font-size: 11px; }
.intro__stats dd { margin: 4px 0 0; color: var(--about-ink); font-size: 16px; font-weight: 600; }
.intro__stats-note { margin: 12px 0 0; color: var(--about-muted); font-size: 10px; line-height: 1.7; }
.intro__principles { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; margin-top: 28px; border-top: 1px solid var(--about-line); }
.intro__principles > div { padding-top: 22px; }
.intro__principles small { color: var(--about-blue); font: 9px var(--about-mono); letter-spacing: 1px; }
.intro__principles h3 { margin: 12px 0 10px; color: var(--about-ink); font-size: 20px; }
.intro__principles p { margin: 0; color: var(--about-muted); font-size: 12px; line-height: 1.8; }
@media (max-width: 900px) { .intro__grid { grid-template-columns: 1fr; } .intro__art { display: none; } .intro__hero-copy { padding: 34px; } .intro__principles { grid-template-columns: 1fr; gap: 0; } .intro__principles > div { padding-top: 18px; } }
@media (max-width: 540px) { .intro__hero-copy { padding: 30px 24px; } .intro__hero h2 { font-size: clamp(20px, 6vw, 26px); white-space: nowrap; } .intro__grid { min-height: 330px; } .intro__hero-bottom { margin: 0 24px; gap: 12px; font-size: 7px; } .intro__body { padding: 26px 24px; } .intro__stats { grid-template-columns: 1fr; gap: 0; padding: 10px 0; } .intro__stats > div { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px dashed var(--about-line); } .intro__stats > div:last-child { border-bottom: 0; } .intro__stats dd { margin: 0; font-size: 14px; text-align: right; } .intro__principles > div { padding: 18px 0; border-bottom: 1px solid var(--about-line); } }
</style>
