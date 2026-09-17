<script setup lang="ts">
/**
 * 「编辑并发送邮件」弹窗。
 *
 * 主题 / 正文由管理员在发送前编辑（预填模板，{{name}} 已替换），备注必填，
 * 发送后记录一条邮件记录。真正的发送以后端 MailModule 为准，这里只 emit 出去
 * 由 store 落内存。
 */
import { ref, watch } from "vue";
import type { EmailType } from "../status";

const props = defineProps<{
  type: EmailType;
  recipientName: string;
  title: string;
  defaultSubject: string;
  defaultBody: string;
}>();

const visible = defineModel<boolean>({ default: false });

const subject = ref("");
const body = ref("");
const note = ref("");

watch(visible, (open) => {
  if (open) {
    subject.value = props.defaultSubject;
    body.value = props.defaultBody;
    note.value = "";
  }
});

const emit = defineEmits<{
  send: [subject: string, body: string, note: string];
}>();

function confirm() {
  if (!note.value.trim()) return;
  emit("send", subject.value, body.value, note.value.trim());
  visible.value = false;
}
</script>

<template>
  <el-dialog v-model="visible" :title="title" width="560px">
    <el-form label-position="top">
      <el-form-item label="主题">
        <el-input v-model="subject" />
      </el-form-item>
      <el-form-item label="正文">
        <el-input v-model="body" type="textarea" :rows="6" />
      </el-form-item>
      <el-form-item label="备注" required>
        <el-input v-model="note" type="textarea" :rows="2" placeholder="发送后会记录该备注，便于回溯" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :disabled="!note.trim()" @click="confirm">确认发送</el-button>
    </template>
  </el-dialog>
</template>
