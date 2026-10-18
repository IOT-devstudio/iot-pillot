<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { StudioGallerySlide } from "../fixtures";

const props = defineProps<{ title: string; subtitle: string; slides: StudioGallerySlide[] }>();
const selected = ref(0);
const current = computed(() => props.slides[selected.value]);
function move(step: number): void {
  const count = props.slides.length;
  if (count) selected.value = (selected.value + step + count) % count;
}
watch(() => props.slides.length, () => { selected.value = 0; });
</script>

<template>
  <section v-if="current" class="gallery" :aria-label="title" aria-roledescription="轮播图">
    <header class="gallery__head"><h3>{{ title }}</h3><p>{{ subtitle }}</p></header>
    <div class="gallery__frame">
      <img v-if="current.image" :src="current.image" :alt="current.alt || current.title" loading="lazy" />
      <div v-else class="gallery__placeholder">
        <svg viewBox="0 0 64 48" fill="none" aria-hidden="true"><rect x="1" y="1" width="62" height="46" stroke="currentColor"/><circle cx="20" cy="16" r="5" stroke="currentColor"/><path d="m2 40 18-15 12 10 13-17 18 22" stroke="currentColor"/></svg>
        <span>{{ current.title }}</span><small>照片待补充</small>
      </div>
    </div>
    <div class="gallery__caption" aria-live="polite" aria-atomic="true">
      <h4>{{ current.title }}</h4><p>{{ current.description }}</p>
    </div>
    <div v-if="slides.length > 1" class="gallery__controls">
      <button type="button" class="gallery__arrow" :aria-label="`${title}：上一张`" @click="move(-1)">←</button>
      <div class="gallery__dots">
        <button v-for="(slide, index) in slides" :key="slide.id" type="button" class="gallery__dot" :aria-label="`查看${slide.title}`" :aria-pressed="selected === index" @click="selected = index"></button>
      </div>
      <span class="gallery__count">{{ String(selected + 1).padStart(2, '0') }} / {{ String(slides.length).padStart(2, '0') }}</span>
      <button type="button" class="gallery__arrow" :aria-label="`${title}：下一张`" @click="move(1)">→</button>
    </div>
  </section>
</template>

<style scoped>
.gallery { min-width: 0; margin-top: 32px; border-top: 1px solid var(--about-line); }
.gallery__head { padding: 22px 0 18px; }
.gallery__head h3 { margin: 0; color: var(--about-ink); font-size: 20px; }
.gallery__head p { margin: 6px 0 0; color: var(--about-muted); font-size: 12px; line-height: 1.8; }
.gallery__frame { aspect-ratio: 16 / 9; overflow: hidden; background: var(--about-blue); }
.gallery__frame img { display: block; width: 100%; height: 100%; object-fit: contain; background: var(--about-paper); }
.gallery__placeholder { display: flex; height: 100%; min-height: 150px; flex-direction: column; justify-content: center; align-items: center; gap: 12px; color: var(--about-yellow); background-image: linear-gradient(90deg, #ffffff12 1px, transparent 1px), linear-gradient(#ffffff12 1px, transparent 1px); background-size: 40px 40px; }
.gallery__placeholder svg { width: 64px; height: 48px; opacity: .7; }
.gallery__placeholder span { font-size: 15px; text-align: center; padding: 0 16px; }
.gallery__placeholder small { color: #edf0ff; font-size: 11px; }
.gallery__caption { padding: 18px 0 0; }
.gallery__caption h4 { margin: 0; color: var(--about-ink); font-size: 15px; }
.gallery__caption p { margin: 6px 0 0; min-height: 3.6em; color: var(--about-muted); font-size: 12px; line-height: 1.8; }
.gallery__controls { display: flex; align-items: center; gap: 12px; padding-top: 12px; }
.gallery__arrow { flex-shrink: 0; width: 40px; height: 40px; padding: 0; border: 1px solid var(--about-line); color: var(--about-blue); background: transparent; font: inherit; cursor: pointer; }
.gallery__arrow:hover { background: var(--about-yellow); }
.gallery__dots { display: flex; flex-wrap: wrap; }
.gallery__dot { display: grid; place-items: center; width: 24px; height: 32px; padding: 0; border: 0; background: transparent; cursor: pointer; }
.gallery__dot::after { content: ""; width: 6px; height: 6px; background: var(--about-line); }
.gallery__dot[aria-pressed="true"]::after { width: 18px; background: var(--about-blue); }
.gallery__count { margin-left: auto; white-space: nowrap; color: var(--about-muted); font: 10px var(--about-mono); }
.gallery button:focus-visible { outline: 3px solid var(--about-blue); outline-offset: 3px; }
@media (max-width: 540px) { .gallery__controls { gap: 6px; } .gallery__arrow { width: 32px; height: 36px; } .gallery__dot { width: 20px; } .gallery__head h3 { font-size: 18px; } }
</style>
