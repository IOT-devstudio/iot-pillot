// ElMessageBox.confirm 会弹真实对话框，必须打桩；ElMessage 同理只是通知。
vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
}));

import { beforeEach, describe, expect, it, vi } from "vitest";
import { ElMessage, ElMessageBox } from "element-plus";

import { AuthRequestError } from "@/api/auth";
import type { MailRecord } from "@/api/admin";
import { useMailRecords } from "./useMailRecords";

const CONFIRM = vi.mocked(ElMessageBox.confirm);
const SUCCESS = vi.mocked(ElMessage.success);
const ERROR = vi.mocked(ElMessage.error);

const record = (id: number): MailRecord => ({
  id,
  title: `主题 ${id}`,
  from_user_id: 1,
  to_user_id: 2,
  to_email: `u${id}@x.y`,
  created_at: "2026-09-23T10:00:00Z",
});

beforeEach(() => {
  vi.clearAllMocks();
  CONFIRM.mockResolvedValue("confirm" as never);
});

describe("useMailRecords", () => {
  it("loads a page of records", async () => {
    const { items, total, load } = useMailRecords({
      list: async () => ({
        items: [record(1)],
        total: 9,
        page: 1,
        page_size: 20,
      }),
      batchDelete: vi.fn(),
    });

    await load();

    expect(items.value).toHaveLength(1);
    expect(total.value).toBe(9);
  });

  it("keeps previous items when a page reload fails (cross-page selection depends on it)", async () => {
    const { items, error, load } = useMailRecords({
      list: vi
        .fn()
        .mockResolvedValueOnce({
          items: [record(1)],
          total: 1,
          page: 1,
          page_size: 20,
        })
        .mockRejectedValueOnce(new AuthRequestError("网络异常", 0)),
      batchDelete: vi.fn(),
    });

    await load();
    await load();

    // 表格不能卸载，否则 reserve-selection 的跨页勾选会丢
    expect(items.value).toHaveLength(1);
    expect(error.value).toBe("网络异常，请检查连接后重试");
  });

  it("does not delete without confirmation", async () => {
    CONFIRM.mockRejectedValue("cancel" as never);
    const batchDelete = vi.fn();
    const { selected, removeSelected } = useMailRecords({
      list: vi.fn(),
      batchDelete,
    });
    selected.value = [record(1)];

    await expect(removeSelected()).resolves.toBe(false);
    expect(batchDelete).not.toHaveBeenCalled();
  });

  it("batch-deletes selection, clears it, and reloads", async () => {
    const batchDelete = vi.fn().mockResolvedValue({ deleted: 2 });
    const list = vi.fn().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const { selected, removeSelected, load } = useMailRecords({
      list,
      batchDelete,
    });
    selected.value = [record(7), record(8)];

    await expect(removeSelected()).resolves.toBe(true);

    expect(batchDelete).toHaveBeenCalledWith([7, 8]);
    expect(selected.value).toEqual([]);
    expect(list).toHaveBeenCalled();
    expect(SUCCESS).toHaveBeenCalledWith("已删除 2 条记录");
  });

  it("steps back a page when the current page is emptied by deletion", async () => {
    const batchDelete = vi.fn().mockResolvedValue({ deleted: 2 });
    let listCall = 0;
    const { items, page, selected, removeSelected, load } = useMailRecords({
      // 第一次 load：第 2 页有 2 条（将被删光）；删除后的 reload：空；回退后：有数据
      list: vi.fn(async (p: number) => {
        listCall += 1;
        if (listCall === 1) {
          return { items: [record(3), record(4)], total: 4, page: p, page_size: 2 };
        }
        if (listCall === 2) {
          return { items: [], total: 2, page: p, page_size: 2 };
        }
        return { items: [record(1), record(2)], total: 2, page: p, page_size: 2 };
      }),
      batchDelete,
    });

    await load();
    page.value = 2;
    selected.value = [record(3), record(4)];
    await removeSelected();

    // 删除后第 2 页被删空 → 回到第 1 页重新加载
    expect(page.value).toBe(1);
    expect(items.value).toHaveLength(2);
  });

  it("surfaces the backend delete failure as a toast", async () => {
    const batchDelete = vi
      .fn()
      .mockRejectedValue(new AuthRequestError("服务暂时不可用，请稍后重试", 500));
    const { selected, removeSelected } = useMailRecords({
      list: vi.fn(),
      batchDelete,
    });
    selected.value = [record(1)];

    await expect(removeSelected()).resolves.toBe(false);
    expect(ERROR).toHaveBeenCalledWith("服务暂时不可用，请稍后重试");
    // 失败后选中不清：用户可以修好网络后直接重试
    expect(selected.value).toHaveLength(1);
  });

  it("skips deletion when nothing is selected", async () => {
    const batchDelete = vi.fn();
    const { removeSelected } = useMailRecords({
      list: vi.fn(),
      batchDelete,
    });

    await expect(removeSelected()).resolves.toBe(false);
    expect(batchDelete).not.toHaveBeenCalled();
    expect(CONFIRM).not.toHaveBeenCalled();
  });

  it("search passes keyword/dates to the list dep and rewinds to page 1", async () => {
    const list = vi.fn().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const { keyword, dates, page, search } = useMailRecords({
      list,
      batchDelete: vi.fn(),
    });

    page.value = 3;
    keyword.value = " 面试 ";
    dates.value = ["2026-09-01", "2026-09-30"];
    await search();

    expect(page.value).toBe(1);
    expect(list).toHaveBeenCalledWith(1, 20, {
      keyword: " 面试 ",
      from: "2026-09-01",
      to: "2026-09-30",
    });
  });

  it("resetFilter clears conditions and reloads from page 1", async () => {
    const list = vi.fn().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const { keyword, dates, page, resetFilter, hasFilter } = useMailRecords({
      list,
      batchDelete: vi.fn(),
    });

    keyword.value = "abc";
    dates.value = ["2026-09-01", "2026-09-30"];
    page.value = 5;

    await resetFilter();

    expect(hasFilter.value).toBe(false);
    expect(page.value).toBe(1);
    expect(list).toHaveBeenCalledWith(1, 20, { keyword: "", from: "", to: "" });
  });

  it("visibleRecords narrows the loaded page by keyword and dates locally", async () => {
    const rows: MailRecord[] = [
      { ...record(1), title: "面试邀请", to_email: "a@x.y", created_at: "2026-09-22T10:00:00Z" },
      { ...record(2), title: "offer 恭喜", to_email: "b@x.y", created_at: "2026-09-23T10:00:00Z" },
    ];
    const { items, keyword, dates, visibleRecords, load } = useMailRecords({
      list: vi.fn().mockResolvedValue({ items: rows, total: 2, page: 1, page_size: 20 }),
      batchDelete: vi.fn(),
    });
    await load();

    keyword.value = "面试";
    expect(visibleRecords.value.map((r) => r.id)).toEqual([1]);

    keyword.value = "";
    dates.value = ["2026-09-23", "2026-09-30"];
    expect(visibleRecords.value.map((r) => r.id)).toEqual([2]);

    // 兜底过滤只作用于视图，不改动已加载的原始页
    expect(items.value).toHaveLength(2);
  });
});
