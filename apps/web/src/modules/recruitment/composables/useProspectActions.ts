/**
 * 意向成员的动作编排：发送（通过/拒录）邮件 + 标记通过一面。
 *
 * 列表页和详情页共用这套逻辑，避免两处各自维护弹窗状态与预填文案导致漂移。
 */
import { computed, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import { emailTemplates, fillName, type Prospect } from "../fixture";
import type { EmailType } from "../status";
import { markFirstPass, sendEmail } from "../store";

export function useProspectActions() {
  const visible = ref(false);
  const target = ref<Prospect | null>(null);
  const type = ref<EmailType>("pass");

  const title = computed(() => {
    const name = target.value?.name ?? "";
    const prefix = type.value === "pass" ? "发送通过邮件" : "发送拒录邮件";
    return name ? `${prefix} · ${name}` : prefix;
  });

  const template = computed(
    () => emailTemplates.find((t) => t.type === type.value) ?? emailTemplates[0],
  );

  const defaultSubject = computed(() =>
    target.value
      ? fillName(template.value.subject, target.value.name)
      : template.value.subject,
  );

  const defaultBody = computed(() =>
    target.value
      ? fillName(template.value.body, target.value.name)
      : template.value.body,
  );

  function openEmail(p: Prospect, emailType: EmailType) {
    target.value = p;
    type.value = emailType;
    visible.value = true;
  }

  function onSend(subject: string, body: string, note: string) {
    const t = target.value;
    if (!t) return;
    sendEmail(t.id, type.value, subject, body, note);
    const verb = type.value === "pass" ? "通过" : "拒录";
    ElMessage.success(`已向 ${t.name} 发送${verb}邮件`);
  }

  async function markPassed(p: Prospect) {
    try {
      await ElMessageBox.confirm(
        `将「${p.name}」标记为一面通过，之后可发送通过邮件。`,
        "标记通过一面",
        { type: "info", confirmButtonText: "确认", cancelButtonText: "取消" },
      );
      markFirstPass(p.id);
      ElMessage.success(`已将 ${p.name} 标记为一面通过`);
    } catch {
      // 用户取消
    }
  }

  return {
    visible,
    target,
    type,
    title,
    defaultSubject,
    defaultBody,
    openEmail,
    onSend,
    markPassed,
  };
}
