<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { listMailTemplates, type MailTemplate } from "@/api/mail";
import MailHtmlPreview from "@/modules/mail/components/MailHtmlPreview.vue";
import { renderPreview } from "@/modules/mail/utils/template-vars";
import { errorMessage } from "../workflow";
const props = defineProps<{
  type: "invitation" | "offer" | "rejection";
  disabled: boolean;
}>();
const emit = defineEmits<{
  change: [value: { template_id?: number; vars: Record<string, string> }];
}>();
const templates = ref<MailTemplate[]>([]),
  selectedId = ref<number>(),
  vars = ref<Record<string, string>>({}),
  error = ref(""),
  loading = ref(false);
const selected = computed(() =>
  templates.value.find((item) => item.id === selectedId.value),
);
const fields = computed(() =>
  (selected.value?.variables ?? []).filter(
    (name) =>
      props.type !== "invitation" ||
      !["invitation_token", "invitation_expires_at"].includes(name),
  ),
);
const invitationLink = `${window.location.origin}/user/invitations?token={{invitation_token}}`;
watch(
  [selectedId, vars],
  () =>
    emit("change", {
      template_id: selectedId.value,
      vars: Object.fromEntries(
        Object.entries(vars.value).filter(([, value]) => value.trim()),
      ),
    }),
  { deep: true },
);
watch(selectedId, () => {
  vars.value = {};
});
async function load() {
  loading.value = true;
  error.value = "";
  try {
    templates.value = (await listMailTemplates()).items.filter(
      (item) => item.type === props.type,
    );
  } catch (e) {
    error.value = errorMessage(e);
  } finally {
    loading.value = false;
  }
}
onMounted(load);
</script>
<template>
  <el-alert
    v-if="error"
    :title="error"
    type="error"
    :closable="false"
  /><el-button v-if="error" @click="load">重试加载模板</el-button>
  <el-form-item label="邮件模板" required
    ><el-select
      v-model="selectedId"
      :disabled="disabled"
      :loading="loading"
      placeholder="选择对应类型的模板"
      ><el-option
        v-for="item in templates"
        :key="item.id"
        :label="item.name"
        :value="item.id" /></el-select
  ></el-form-item>
  <p v-if="!loading && !error && !templates.length">
    没有可用模板，请先在邮件中心创建 {{ type }} 类型模板。
  </p>
  <p v-if="type === 'invitation'" class="intro">
    邀请模板中可使用此链接（发送时自动替换凭据）：<br /><code
      style="overflow-wrap: anywhere"
      >{{ invitationLink }}</code
    >。用户也可登录后从“邀请与招新结果”接受。
  </p>
  <el-form-item v-for="name in fields" :key="name" :label="name"
    ><el-input
      v-model="vars[name]"
      :disabled="disabled"
      placeholder="可选；留空时由服务端填入已有资料"
  /></el-form-item>
  <p v-if="selected">
    预览中未替换的变量将在发送时由服务端校验；缺少资料会导致发送失败。
  </p>
  <MailHtmlPreview
    v-if="selected"
    :subject="renderPreview(selected.title, vars)"
    :html="renderPreview(selected.mail_model, vars)"
  />
</template>
