/**
 * 邮件中心 → 发信记录分页与批量删除（issue #72）。
 *
 * 服务端分页（后端按发送时间倒序），前端不模拟：page/page_size 原样上送。
 * 「操作者」列只拿到 from_user_id，姓名映射由视图层拿用户名单解决——
 * 记录接口本身不返回姓名，这是后端契约，不在前端造。
 *
 * 批量删除：选中行由 el-table 的 reserve-selection 维护（跨页保留），
 * 这里只存引用、发删除、清选、刷新。删除后当前页可能整个空掉
 * （比如只选了本页 3 条全删），回退一页重载，避免停在没有数据的页码上。
 */
import { ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import { listMails, type MailRecord } from "@/api/admin";
import { batchDeleteMailRecords } from "@/api/mail";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";

export interface MailRecordsDeps {
  list: (page: number, pageSize: number) => Promise<{
    items: MailRecord[];
    total: number;
    page: number;
    page_size: number;
  }>;
  batchDelete: (ids: number[]) => Promise<{ deleted: number }>;
}

const defaultDeps: MailRecordsDeps = {
  list: listMails,
  batchDelete: batchDeleteMailRecords,
};

export function useMailRecords(deps: MailRecordsDeps = defaultDeps) {
  const items = ref<MailRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(20);
  const loading = ref(false);
  const error = ref("");
  const unauthorized = ref(false);
  /** 当前勾选的行（跨页保留由表格的 reserve-selection 负责） */
  const selected = ref<MailRecord[]>([]);
  /** 删除请求进行中，防双击 */
  const deleting = ref(false);

  async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    unauthorized.value = false;
    try {
      const result = await deps.list(page.value, pageSize.value);
      items.value = result.items;
      total.value = result.total;
    } catch (e) {
      const info = describeAuthError(e);
      error.value = info.message;
      unauthorized.value = info.unauthorized;
      // 不清 items：翻页失败时保留上一页数据，表格不卸载，
      // 跨页勾选（reserve-selection）才不会被一次失败的请求冲掉
    } finally {
      loading.value = false;
    }
  }

  async function changePage(next: number): Promise<void> {
    page.value = next;
    await load();
  }

  /** 换每页条数要回到第一页：停留在旧页码可能已超出新范围。 */
  async function changePageSize(size: number): Promise<void> {
    pageSize.value = size;
    page.value = 1;
    await load();
  }

  function onSelectionChange(rows: MailRecord[]): void {
    selected.value = rows;
  }

  /**
   * 批量删除选中记录。二次确认不可省：记录是硬删除，
   * 删掉的发信历史无法找回。
   */
  async function removeSelected(): Promise<boolean> {
    if (selected.value.length === 0) return false;
    const ids = selected.value.map((row) => row.id);

    try {
      await ElMessageBox.confirm(
        `将永久删除选中的 ${ids.length} 条发信记录，删除后无法恢复。确定删除？`,
        "删除发信记录",
        { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" },
      );
    } catch {
      return false; // 用户取消
    }

    deleting.value = true;
    try {
      const result = await deps.batchDelete(ids);
      ElMessage.success(`已删除 ${result.deleted} 条记录`);
      selected.value = [];
      await load();
      // 当前页被删空且不是第一页：回退一页，否则停在一个没有数据的页码上
      if (items.value.length === 0 && page.value > 1) {
        page.value -= 1;
        await load();
      }
      return true;
    } catch (e) {
      ElMessage.error(describeAuthError(e).message);
      return false;
    } finally {
      deleting.value = false;
    }
  }

  return {
    items,
    total,
    page,
    pageSize,
    loading,
    error,
    unauthorized,
    selected,
    deleting,
    load,
    changePage,
    changePageSize,
    onSelectionChange,
    removeSelected,
  };
}
