/**
 * 邮件中心 → 发送编排（issue #52）。
 *
 * 三种发送模式共用一个状态机：按用户单发 / 按邮箱单发 / 群发。
 * 发送请求只携带 template_id + vars（契约决策：不做临时改写正文），
 * 变量留空由服务端用收件人资料预填，这里只管理用户显式填写的部分。
 *
 * 缺变量不在前端硬拦：注册用户有资料预填，按邮箱发给未注册地址则全靠
 * vars——能不能发由服务端 RenderTemplate 说了算，前端把它的报错原样转达。
 */
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";

import {
  MAIL_BULK_LIMIT,
  sendMailBulk,
  sendMailToEmail,
  sendMailToUser,
  type MailBulkResult,
  type MailSendResult,
  type MailVars,
} from "@/api/mail";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";

export type SendMode = "user" | "email" | "bulk";

export interface SendTarget {
  to_user_id: number;
  /** 仅按邮箱单发时使用 */
  email?: string;
}

export function useMailSend() {
  const mode = ref<SendMode>("user");
  /** 单发目标（用户 ID 或邮箱由 mode 决定）；始终非 null，模板里可直接 v-model */
  const target = ref<SendTarget>({ to_user_id: 0 });
  /** 群发目标的用户 ID 列表 */
  const bulkUserIds = ref<number[]>([]);
  /** 用户显式填写的变量；留空的键不发送，交给服务端预填 */
  const vars = ref<MailVars>({});
  const sending = ref(false);
  const error = ref("");
  /** 最近一次单发结果 */
  const lastSend = ref<MailSendResult | null>(null);
  /** 最近一次群发结果 */
  const bulkResult = ref<MailBulkResult | null>(null);

  const bulkCount = computed(() => bulkUserIds.value.length);
  const bulkOverLimit = computed(() => bulkCount.value > MAIL_BULK_LIMIT);
  const canSend = computed(() => {
    if (sending.value) return false;
    if (mode.value === "user") return (target.value?.to_user_id ?? 0) > 0;
    if (mode.value === "email") {
      return (target.value?.email ?? "").trim().length > 0;
    }
    return bulkCount.value > 0 && !bulkOverLimit.value;
  });

  function reset(): void {
    mode.value = "user";
    target.value = { to_user_id: 0 };
    bulkUserIds.value = [];
    vars.value = {};
    error.value = "";
    lastSend.value = null;
    bulkResult.value = null;
  }

  /** 只发送非空值：空字符串会被服务端 putIfNotEmpty 忽略，传了也无意义。 */
  function filledVars(): MailVars {
    const filled: MailVars = {};
    for (const [key, value] of Object.entries(vars.value)) {
      if (value.trim() !== "") filled[key] = value;
    }
    return filled;
  }

  async function send(templateId: number): Promise<void> {
    if (!canSend.value) return;
    sending.value = true;
    error.value = "";
    lastSend.value = null;
    bulkResult.value = null;

    try {
      const overrides = filledVars();
      if (mode.value === "user") {
        lastSend.value = await sendMailToUser({
          template_id: templateId,
          to_user_id: target.value.to_user_id,
          vars: overrides,
        });
        ElMessage.success(`已发送至 ${lastSend.value.to_email}`);
      } else if (mode.value === "email") {
        lastSend.value = await sendMailToEmail({
          template_id: templateId,
          email: (target.value.email ?? "").trim(),
          vars: overrides,
        });
        ElMessage.success(`已发送至 ${lastSend.value.to_email}`);
      } else {
        bulkResult.value = await sendMailBulk({
          template_id: templateId,
          recipients: bulkUserIds.value.map((id) => ({
            to_user_id: id,
            vars: overrides,
          })),
        });
        const { sent, failed } = bulkResult.value;
        if (failed.length === 0) {
          ElMessage.success(`群发完成：${sent.length} 封全部成功`);
        } else {
          ElMessage.warning(
            `群发完成：成功 ${sent.length} 封，失败 ${failed.length} 封`,
          );
        }
      }
    } catch (e) {
      // 服务端的「模板缺少变量」「SMTP 投递失败」「收件人超上限」等
      // 报错都是可读中文，describeAuthError 会原样带出。
      error.value = describeAuthError(e).message;
    } finally {
      sending.value = false;
    }
  }

  return {
    mode,
    target,
    bulkUserIds,
    vars,
    sending,
    error,
    lastSend,
    bulkResult,
    bulkCount,
    bulkOverLimit,
    canSend,
    reset,
    send,
  };
}
