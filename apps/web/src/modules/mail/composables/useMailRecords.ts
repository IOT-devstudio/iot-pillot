/**
 * 邮件中心 → 发信记录分页与搜索（issue #72 搜索部分）。
 *
 * 过滤（keyword / 日期范围）由服务端完成——total 是过滤后的总数，
 * 跨页搜索与页码才对得上；「操作者」列的姓名映射仍由视图层解决。
 *
 * visibleRecords 是对当前页的**本地兜底过滤**：后端未部署查询参数时
 * （旧版本忽略 keyword/from/to 原样返回），搜索仍能筛当前页；
 * 部署后服务端已过滤，本地这层是无害交集。日期兜底按 ISO 日期切片
 * 粗比较，存在小时级时区边界误差，仅作过渡。
 */
import { computed, ref } from "vue";

import { listMails, type MailRecord, type MailRecordFilter } from "@/api/admin";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";

export interface MailRecordsDeps {
  list: (
    page: number,
    pageSize: number,
    filter?: MailRecordFilter,
  ) => Promise<{
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

  /* —— 搜索条件：keyword 即时绑定输入框，日期来自 daterange —— */
  const keyword = ref("");
  /** [YYYY-MM-DD, YYYY-MM-DD] 或 null */
  const dates = ref<[string, string] | null>(null);

  const hasFilter = computed(
    () => keyword.value.trim() !== "" || dates.value !== null,
  );

  function currentFilter(): MailRecordFilter {
    return {
      keyword: keyword.value,
      from: dates.value?.[0] ?? "",
      to: dates.value?.[1] ?? "",
    };
  }

  /** 本地兜底过滤（见文件头注释） */
  const visibleRecords = computed(() => {
    const kw = keyword.value.trim().toLowerCase();
    const from = dates.value?.[0] ?? "";
    const to = dates.value?.[1] ?? "";
    return items.value.filter((row) => {
      if (kw) {
        const hit =
          row.title.toLowerCase().includes(kw) ||
          row.to_email.toLowerCase().includes(kw);
        if (!hit) return false;
      }
      if (from || to) {
        // created_at 为 RFC3339（UTC），切日期粗比较，边界误差仅过渡期存在
        const day = row.created_at.slice(0, 10);
        if (from && day < from) return false;
        if (to && day > to) return false;
      }
      return true;
    });
  });

  async function load(): Promise<void> {
    loading.value = true;
    error.value = "";
    unauthorized.value = false;
    try {
      const result = await deps.list(page.value, pageSize.value, currentFilter());
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

  /** 应用搜索条件：总是回到第一页——停在旧页码可能已超出结果范围 */
  async function search(): Promise<void> {
    page.value = 1;
    await load();
  }

  /** 清空全部条件并回到全量第一页 */
  async function resetFilter(): Promise<void> {
    keyword.value = "";
    dates.value = null;
    await search();
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

  return {
    items,
    visibleRecords,
    total,
    page,
    pageSize,
    loading,
    error,
    unauthorized,
    keyword,
    dates,
    hasFilter,
    load,
    search,
    resetFilter,
    changePage,
    changePageSize,
  };
}
