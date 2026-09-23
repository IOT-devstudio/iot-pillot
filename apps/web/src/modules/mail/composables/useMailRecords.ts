/**
 * 邮件中心 → 发信记录分页（issue #52）。
 *
 * 服务端分页（后端按发送时间倒序），前端不模拟：page/page_size 原样上送。
 * 「操作者」列只拿到 from_user_id，姓名映射由视图层拿用户名单解决——
 * 记录接口本身不返回姓名，这是后端契约，不在前端造。
 */
import { ref } from "vue";

import { listMails, type MailRecord } from "@/api/admin";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";

export interface MailRecordsDeps {
  list: (page: number, pageSize: number) => Promise<{
    items: MailRecord[];
    total: number;
    page: number;
    page_size: number;
  }>;
}

const defaultDeps: MailRecordsDeps = { list: listMails };

export function useMailRecords(deps: MailRecordsDeps = defaultDeps) {
  const items = ref<MailRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(20);
  const loading = ref(false);
  const error = ref("");
  const unauthorized = ref(false);

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
      items.value = [];
      total.value = 0;
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

  return { items, total, page, pageSize, loading, error, unauthorized, load, changePage, changePageSize };
}
