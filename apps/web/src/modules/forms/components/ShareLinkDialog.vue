<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { sharePath } from "../model";
const props = defineProps<{ modelValue: boolean; token: string }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
const message = ref("");
const link = computed(
  () => new URL(sharePath(props.token), window.location.origin).href,
);
watch(
  () => props.modelValue,
  () => {
    message.value = "";
  },
);
async function copy() {
  try {
    await navigator.clipboard.writeText(link.value);
    message.value = "链接已复制";
  } catch {
    message.value = "无法自动复制，请选中上方链接手动复制。";
  }
}
</script>
<template>
  <el-dialog
    :model-value="modelValue"
    title="分享报名表"
    width="min(540px, calc(100vw - 32px))"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <p>填写者需登录；表单开放后才可提交。</p>
    <label for="form-share-url">分享链接</label>
    <input
      id="form-share-url"
      class="form-control"
      :value="link"
      readonly
      @focus="($event.target as HTMLInputElement).select()"
    />
    <p role="status">{{ message }}</p>
    <template #footer
      ><el-button type="primary" @click="copy">复制链接</el-button></template
    >
  </el-dialog>
</template>
