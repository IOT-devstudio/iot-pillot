import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  batchDeleteMailRecords,
  createMailTemplate,
  deleteMailTemplate,
  listMailTemplates,
  sendMailBulk,
  sendMailToEmail,
  sendMailToUser,
  updateMailTemplate,
} from "./mail";
import { AUTH_STORAGE_KEY } from "@/auth/session";

/**
 * 锁住邮件中心的**请求契约**（路径 / 方法 / 请求体 / Bearer 头）与解包后的数据。
 * 走受保护请求，所以照 admin.test.ts 的路子打桩 localStorage + fetch——
 * 少了会话桩，session-request 会把请求拐去刷新令牌，断言的路径就全错了。
 */
const session = {
  access_token: "access-token",
  refresh_token: "refresh-token",
  user_id: 1,
};

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

function authHeaderOf(call = 0): string | null {
  const init = fetchMock.mock.calls[call]?.[1] as RequestInit | undefined;
  return new Headers(init?.headers).get("Authorization");
}

const fetchMock = vi.fn();

/** 让下一次 fetch 返回固定 JSON——直接打在 fetchMock 上，调用记录留在它身上。 */
function mockJson(payload: unknown, status = 200) {
  fetchMock.mockResolvedValueOnce(
    new Response(JSON.stringify(payload), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

beforeEach(() => {
  store.clear();
  store.set(AUTH_STORAGE_KEY, JSON.stringify(session));
  vi.stubGlobal("window", {
    localStorage: fakeStorage,
    location: { assign: vi.fn() },
  });
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("mail api", () => {
  it("lists templates through the session wrapper with Bearer token", async () => {
    mockJson({ code: 0, message: "ok", data: { items: [], total: 0 } });

    await expect(listMailTemplates()).resolves.toEqual({ items: [], total: 0 });
    // GET 走 fetch 默认方法，init 里只有会话层塞进去的 headers
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/admin/mail-templates",
      expect.objectContaining({ headers: expect.any(Headers) }),
    );
    expect(authHeaderOf()).toBe("Bearer access-token");
  });

  it("creates a template with a JSON body", async () => {
    const input = {
      name: "面试邀请",
      type: "invitation",
      title: "{{name}} 面试邀请",
      mail_example: "Hi 张三",
      mail_model: "Hi {{name}}",
    };
    mockJson({
      code: 0,
      message: "ok",
      data: { id: 1, ...input, variables: ["name"] },
    });

    await createMailTemplate(input);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mail-templates");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual(input);
  });

  it("updates a template by id with PUT", async () => {
    mockJson({ code: 0, message: "ok", data: { id: 7 } });
    await updateMailTemplate(7, {
      name: "a",
      type: "custom",
      title: "t",
      mail_example: "",
      mail_model: "b",
    });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mail-templates/7");
    expect(init.method).toBe("PUT");
  });

  it("deletes a template through the message outlet (backend omits data)", async () => {
    // 后端 OKWithMsg("模板已删除", nil)：data 被 omitempty 整个丢掉。
    // 用要求 data 的出口会把这次成功当成「服务暂时不可用」——锁住这条契约。
    mockJson({ code: 0, message: "模板已删除" });
    await expect(deleteMailTemplate(3)).resolves.toBe("模板已删除");
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mail-templates/3");
    expect(init.method).toBe("DELETE");
    expect(authHeaderOf()).toBe("Bearer access-token");
  });

  it("sends to a user with template_id and vars", async () => {
    mockJson({
      code: 0,
      message: "ok",
      data: { to_user_id: 2, to_email: "a@b.c", title: "t" },
    });
    await sendMailToUser({
      template_id: 1,
      to_user_id: 2,
      vars: { name: "张三" },
    });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mails/send");
    expect(JSON.parse(String(init.body))).toEqual({
      template_id: 1,
      to_user_id: 2,
      vars: { name: "张三" },
    });
  });

  it("sends to a raw email address", async () => {
    mockJson({
      code: 0,
      message: "ok",
      data: { to_user_id: 0, to_email: "x@y.z", title: "t" },
    });
    await sendMailToEmail({ template_id: 1, email: "x@y.z" });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mails/send-by-email");
    expect(JSON.parse(String(init.body))).toEqual({
      template_id: 1,
      email: "x@y.z",
    });
  });

  it("bulk-sends with a recipient list", async () => {
    mockJson({
      code: 0,
      message: "ok",
      data: { sent: [], failed: [], total: 2 },
    });
    await sendMailBulk({
      template_id: 5,
      recipients: [{ to_user_id: 1 }, { to_user_id: 2 }],
    });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mails/send-bulk");
    expect(JSON.parse(String(init.body))).toEqual({
      template_id: 5,
      recipients: [{ to_user_id: 1 }, { to_user_id: 2 }],
    });
  });

  it("batch-deletes records with an ids array", async () => {
    mockJson({ code: 0, message: "ok", data: { deleted: 3 } });
    await expect(batchDeleteMailRecords([7, 8, 9])).resolves.toEqual({
      deleted: 3,
    });
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/mails/batch-delete");
    expect(init.method).toBe("DELETE");
    expect(JSON.parse(String(init.body))).toEqual({ ids: [7, 8, 9] });
    expect(authHeaderOf()).toBe("Bearer access-token");
  });

  it("surfaces the backend error message when the template is missing", async () => {
    mockJson({ code: 4000, message: "模板缺少变量：name", data: null }, 400);

    await expect(
      sendMailToUser({ template_id: 1, to_user_id: 2 }),
    ).rejects.toMatchObject({ message: "模板缺少变量：name", status: 400 });
  });
});
