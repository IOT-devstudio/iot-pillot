<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from "vue";
const props = defineProps<{ title: string }>();
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLDialogElement | null>(null);
let previous: HTMLElement | null = null;
let closing = false;
const active = new Set<Animation>();
const timers = new Set<ReturnType<typeof setTimeout>>();
const reduced =
  typeof matchMedia !== "undefined"
    ? matchMedia("(prefers-reduced-motion: reduce)")
    : null;
function settle() {
  active.forEach((a) => {
    try {
      a.finish();
    } catch {
      a.cancel();
    }
  });
}
async function animate(
  frames: Keyframe[],
  duration: number,
  element: Element | null = dialog.value,
  delay = 0,
) {
  if (
    !dialog.value ||
    reduced?.matches ||
    document.hidden ||
    dialog.value.closest(".motion-paused")
  )
    return;
  if (!element) return;
  const a = element.animate(frames, {
    duration,
    delay,
    easing: "cubic-bezier(.22,1,.36,1)",
  });
  active.add(a);
  const timer = setTimeout(
    () => {
      try {
        a.finish();
      } catch {
        a.cancel();
      }
    },
    duration + delay + 350,
  );
  timers.add(timer);
  await a.finished.catch(() => {});
  clearTimeout(timer);
  timers.delete(timer);
  active.delete(a);
  a.cancel();
}
async function close() {
  if (closing) return;
  closing = true;
  await animate(
    [
      { opacity: 1, transform: "translateY(0) scale(1)" },
      { opacity: 0, transform: "translateY(15px) scale(.98)" },
    ],
    190,
  );
  dialog.value?.close();
  emit("close");
  previous?.focus();
}
function backdrop(event: MouseEvent) {
  const rect = dialog.value?.getBoundingClientRect();
  if (
    event.target === dialog.value &&
    rect &&
    (event.clientX < rect.left ||
      event.clientX > rect.right ||
      event.clientY < rect.top ||
      event.clientY > rect.bottom)
  )
    void close();
}
onMounted(async () => {
  previous = document.activeElement as HTMLElement;
  await nextTick();
  dialog.value?.showModal();
  document.addEventListener("visibilitychange", settle);
  reduced?.addEventListener("change", settle);
  dialog.value
    ?.querySelectorAll(
      "h2, .dialog-description, .direction-facts > div, .detail-steps li, .action-button",
    )
    .forEach((el, i) => {
      void animate(
        [
          { opacity: 0, transform: "translateY(14px)" },
          { opacity: 1, transform: "translateY(0)" },
        ],
        430,
        el,
        70 + i * 45,
      );
    });
  await animate(
    [
      { opacity: 0, transform: "translateY(28px) scale(.97)" },
      { opacity: 1, transform: "translateY(0) scale(1)" },
    ],
    430,
  );
});
onUnmounted(() => {
  active.forEach((a) => a.cancel());
  timers.forEach(clearTimeout);
  document.removeEventListener("visibilitychange", settle);
  reduced?.removeEventListener("change", settle);
  dialog.value?.close();
  previous?.focus();
});
</script>
<template>
  <dialog
    ref="dialog"
    aria-labelledby="signal-dialog-title"
    @cancel.prevent="close"
    @click="backdrop"
  >
    <button class="dialog-close" aria-label="关闭详情" autofocus @click="close">
      ×
    </button>
    <span class="mono">IoT STUDIO / YOUR NEXT STEP</span>
    <h2 id="signal-dialog-title">{{ props.title }}</h2>
    <div class="dialog-content"><slot /></div>
  </dialog>
</template>
