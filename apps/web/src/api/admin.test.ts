import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  grantAdmin,
  listAdminUsers,
  listAdmins,
  listMails,
  revokeAdmin,
} from "./admin";
import { AUTH_STORAGE_KEY } from "@/auth/session";

/**
 * 这几个接口是「用户与权限」页的数据来源，且都走受保护请求。
 * 打桩 localStorage + fetch，把**请求契约**（路径、方法、请求体、Authorization 头）
 * 与**解包后的数据**一起锁住——只测返回值的写法曾经漏掉过「POST 没带令牌」，
 * 那种错在单测里看不出来、一到真机就 401。
 */
const session = {
  access_token: "access-token",
  refresh_token: "refresh-token",
  user_id: 1,
};

/**
 * vitest 默认跑 node 环境（没有 window / localStorage），所以这里塞一个最小实现，
 * 并把它挂到 window 上——auth/session.ts 读的就是 window.localStorage。
 */
const store = new Map<string, string>();
const fakeStorage: Storage = {
  getItem: (k) => store.get(k) ?? null,
  setItem: (k, v) => void store.set(k, v),
  removeItem: (k) => void store.delete(k),
  clear: () => store.clear(),
  key: (i) => Array.from(store.keys())[i] ?? null,
  get length() {
    return store.size;
  },
};

function mockSession(): void {
  store.set(AUTH_STORAGE_KEY, JSON.stringify(session));
}

beforeEach(() => {
  store.clear();
  // window 上还要有 location：session-request 在会话失效时会
  // window.location.assign("/") 跳登录，缺了它会以 TypeError 收场，
  // 掩盖掉真正要断言的「未登录」错误。
  vi.stubGlobal("window", {
    localStorage: fakeStorage,
    location: { assign: vi.fn() },
  });
});

/**
 * 从 fetch 调用的第二个参数里取出 Authorization。
 *
 * 新架构（api/session-request.ts）用 `new Headers()` 里塞 Bearer，
 * 所以不能像以前那样对着普通对象断言——Headers 是不可枚举的，得用 get()。
 */
function authHeaderOf(fetchMock: { mock: { calls: unknown[][] } }, call = 0): string | null {
  const init = fetchMock.mock.calls[call]?.[1] as RequestInit | undefined;
  return new Headers(init?.headers).get("Authorization");
}

function mockJson(payload: unknown, status = 200) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify(payload), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

const adminUser = {
  user_id: 7,
  name: "Ada",
  created_at: "2026-09-17T10:00:00Z",
  is_admin: true,
};

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("admin api", () => {
  it("分页读取用户列表并带上令牌", async () => {
    mockSession();
    const fetchMock = mockJson({
      code: 0,
      message: "success",
      data: { items: [adminUser], total: 42, page: 2, page_size: 20 },
    });

    await expect(listAdminUsers(2, 20)).resolves.toEqual({
      items: [adminUser],
      total: 42,
      page: 2,
      page_size: 20,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/users?page=2&page_size=20",
      expect.anything(),
    );
    expect(authHeaderOf(fetchMock)).toBe("Bearer access-token");
  });

  it("读取管理员名单（不分页）", async () => {
    mockSession();
    const fetchMock = mockJson({
      code: 0,
      message: "success",
      data: { items: [adminUser], total: 1 },
    });

    await expect(listAdmins()).resolves.toEqual({
      items: [adminUser],
      total: 1,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/admins",
      expect.anything(),
    );
    expect(authHeaderOf(fetchMock)).toBe("Bearer access-token");
  });

  it("提升管理员：POST 到集合路径，体里带 user_id，且必须带令牌", async () => {
    mockSession();
    const fetchMock = mockJson({
      code: 0,
      message: "已提升为管理员，对方需要重新登录",
      data: { user_id: 7, is_admin: true, session_revoked: true },
    });

    await expect(grantAdmin(7)).resolves.toEqual({
      user_id: 7,
      is_admin: true,
      session_revoked: true,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/admins",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ user_id: 7 }),
      }),
    );
    // 少了这个头后端 AuthRequired 直接 401「缺少 Authorization 请求头」。
    // 真机上踩过一次，所以这里单独把它锁死。
    expect(authHeaderOf(fetchMock)).toBe("Bearer access-token");
  });

  it("撤销管理员：DELETE 到用户路径", async () => {
    mockSession();
    const fetchMock = mockJson({
      code: 0,
      message: "已撤销管理员，对方需要重新登录",
      data: { user_id: 7, is_admin: false, session_revoked: false },
    });

    await expect(revokeAdmin(7)).resolves.toEqual({
      user_id: 7,
      is_admin: false,
      session_revoked: false,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/admins/7",
      expect.objectContaining({ method: "DELETE" }),
    );
    expect(authHeaderOf(fetchMock)).toBe("Bearer access-token");
  });

  it("分页读取发信记录并带上令牌", async () => {
    mockSession();
    const fetchMock = mockJson({
      code: 0,
      message: "success",
      data: {
        items: [
          {
            id: 3,
            title: "面试邀请",
            from_user_id: 1,
            to_user_id: 2,
            to_email: "jun@example.com",
            created_at: "2026-09-22T10:00:00Z",
          },
        ],
        total: 41,
        page: 1,
        page_size: 8,
      },
    });

    await expect(listMails(1, 8)).resolves.toEqual({
      items: [
        {
          id: 3,
          title: "面试邀请",
          from_user_id: 1,
          to_user_id: 2,
          to_email: "jun@example.com",
          created_at: "2026-09-22T10:00:00Z",
        },
      ],
      total: 41,
      page: 1,
      page_size: 8,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/mails?page=1&page_size=8",
      expect.anything(),
    );
    expect(authHeaderOf(fetchMock)).toBe("Bearer access-token");
  });

  it("发信记录搜索把 keyword 与日期带上查询串", async () => {
    mockSession();
    const fetchMock = mockJson({
      code: 0,
      message: "success",
      data: { items: [], total: 0, page: 1, page_size: 20 },
    });

    await listMails(1, 20, {
      keyword: "  面试 ",
      from: "2026-09-01",
      to: "2026-09-30",
    });

    // keyword 两端空白由调用方裁掉；空条件不产生多余参数
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/mails?page=1&page_size=20&keyword=%E9%9D%A2%E8%AF%95&from=2026-09-01&to=2026-09-30",
      expect.anything(),
    );
  });

  it("没有会话时不发请求，直接抛未登录", async () => {
    store.clear(); // 无会话
    const fetchMock = vi.spyOn(globalThis, "fetch");

    await expect(listAdmins()).rejects.toThrow(/重新登录/);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("后端拒绝时把 message 透给调用方", async () => {
    mockSession();
    mockJson(
      { code: 4001, message: "不能撤销自己的管理员身份，请让另一位管理员操作" },
      400,
    );

    await expect(revokeAdmin(1)).rejects.toThrow(
      "不能撤销自己的管理员身份，请让另一位管理员操作",
    );
  });

  it("存储键名没变（会话读取依赖它）", () => {
    expect(AUTH_STORAGE_KEY).toBe("iot-pillot.auth");
  });
});
