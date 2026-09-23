import { beforeEach, describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import type { MailRecord } from "@/api/admin";
import { useMailRecords } from "./useMailRecords";

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
    });

    await load();

    expect(items.value).toHaveLength(1);
    expect(total.value).toBe(9);
  });

  it("surfaces a readable error when a page load fails", async () => {
    const { items, error, unauthorized, load } = useMailRecords({
      list: vi
        .fn()
        .mockResolvedValueOnce({
          items: [record(1)],
          total: 1,
          page: 1,
          page_size: 20,
        })
        .mockRejectedValueOnce(new AuthRequestError("网络异常", 0)),
    });

    await load();
    await load();

    // 失败走错误态分支展示（不清的「保留旧数据」语义是选择删除
    // 为跨页勾选引入的，本模块无勾选，维持原版：失败即错误提示）
    expect(error.value).toBe("网络异常，请检查连接后重试");
    expect(unauthorized.value).toBe(false);
    expect(items.value).toHaveLength(0);
  });

  it("search passes keyword/dates to the list dep and rewinds to page 1", async () => {
    const list = vi.fn().mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const { keyword, dates, page, search } = useMailRecords({ list });

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
