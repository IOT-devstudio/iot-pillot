/**
 * 邮件中心 → 模板列表编排（issue #52）。
 *
 * 依赖注入的路子与 useAdminPermissions 一致：默认走真实 API，
 * 测试塞假实现即可跑，不必起网络。表单状态归编辑弹窗，
 * 这里只管「列表三态 + 增删改的时序」。
 */
import { ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import { AuthRequestError } from "@/api/auth";
import {
  createMailTemplate,
  deleteMailTemplate,
  listMailTemplates,
  updateMailTemplate,
  type MailTemplate,
  type MailTemplateInput,
} from "@/api/mail";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";

export interface MailTemplatesDeps {
  list: () => Promise<{ items: MailTemplate[]; total: number }>;
  create: (input: MailTemplateInput) => Promise<MailTemplate>;
  update: (id: number, input: MailTemplateInput) => Promise<MailTemplate>;
  remove: (id: number) => Promise<unknown>;
}

const defaultDeps: MailTemplatesDeps = {
  list: listMailTemplates,
  create: createMailTemplate,
  update: updateMailTemplate,
  remove: deleteMailTemplate,
};

export function useMailTemplates(deps: MailTemplatesDeps = defaultDeps) {
  const templates = ref<MailTemplate[]>([]);
  const loading = ref(false);
  const error = ref("");
  /** 401/403 时置真：重试无意义，界面只解释不放重试按钮。 */
  const unauthorized = ref(false);
  /** 保存/删除的进行中标记，防双击重复提交。 */
  const saving = ref(false);

  async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    unauthorized.value = false;
    try {
      const result = await deps.list();
      templates.value = result.items;
    } catch (e) {
      const info = describeAuthError(e);
      error.value = info.message;
      unauthorized.value = info.unauthorized || (e instanceof AuthRequestError && e.status === 403);
    } finally {
      loading.value = false;
    }
  }

  /** 新建或更新；成功返回 true，失败自行读 error 文案的场景由调用方弹 toast。 */
  async function save(
    input: MailTemplateInput,
    editingId: number | null,
  ): Promise<boolean> {
    saving.value = true;
    try {
      if (editingId === null) {
        await deps.create(input);
      } else {
        await deps.update(editingId, input);
      }
      ElMessage.success(editingId === null ? "模板已创建" : "模板已更新");
      await load();
      return true;
    } catch (e) {
      ElMessage.error(describeAuthError(e).message);
      return false;
    } finally {
      saving.value = false;
    }
  }

  /**
   * 删除前必须确认：模板删了发信记录还在（记录是审计凭据），
   * 但模板本身不可恢复，误删只能重建。
   */
  async function remove(template: MailTemplate): Promise<boolean> {
    try {
      await ElMessageBox.confirm(
        `删除后无法恢复模板「${template.name}」，已发出的发信记录不受影响。确定删除？`,
        "删除模板",
        { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" },
      );
    } catch {
      return false; // 用户取消
    }

    saving.value = true;
    try {
      await deps.remove(template.id);
      ElMessage.success("模板已删除");
      await load();
      return true;
    } catch (e) {
      ElMessage.error(describeAuthError(e).message);
      return false;
    } finally {
      saving.value = false;
    }
  }

  return { templates, loading, error, unauthorized, saving, load, save, remove };
}
