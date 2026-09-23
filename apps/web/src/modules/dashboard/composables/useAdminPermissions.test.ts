import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import type { AdminUser } from "@/api/admin";

// ElMessageBox.confirm 会弹真实对话框，helper 里必须打桩；ElMessage 同理只是个通知。
vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
}));

import { ElMessage, ElMessageBox } from "element-plus";

import {
  describeAuthError,
  describeMutation,
  rowActionFor,
  useAdminPermissions,
  UNKNOWN_IDENTITY_NOTE,
  type AdminPermissionsDeps,
} from "./useAdminPermissions";

const CONFIRM = vi.mocked(ElMessageBox.confirm);
const SUCCESS = vi.mocked(ElMessage.success);
const ERROR = vi.mocked(ElMessage.error);
const WARNING = vi.mocked(ElMessage.warning);

afterEach(() => {
  vi.clearAllMocks();
});

function user(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    user_id: 1,
    name: "Ada",
    created_at: "2026-09-17T10:00:00Z",
    is_admin: false,
    ...overrides,
  };
}

/** 一套默认依赖：所有网络调用都成功，可逐个覆盖。 */
function makeDeps(overrides: Partial<AdminPermissionsDeps> = {}) {
  const deps: AdminPermissionsDeps = {
    fetchCurrentUser: async () => ({
      user_id: 1,
      username: "ada",
      role: "admin",
    }),
    listUsers: async () => ({ items: [user()], total: 1 }),
    listAdmins: async () => ({ items: [] }),
    grantAdmin: async () => ({
      user_id: 1,
      is_admin: true,
      session_revoked: true,
    }),
    revokeAdmin: async () => ({
      user_id: 1,
      is_admin: false,
      session_revoked: true,
    }),
    ...overrides,
  };
  return deps;
}

describe("describeAuthError", () => {
  it("分开网络异常、401 与 403", () => {
    const network = describeAuthError(new AuthRequestError("x", 0));
    expect(network.message).toContain("网络异常");
    expect(network.unauthorized).toBe(false);

    const unauthorized = describeAuthError(new AuthRequestError("x", 401));
    expect(unauthorized.message).toContain("重新登录");
    expect(unauthorized.unauthorized).toBe(true);

    const forbidden = describeAuthError(new AuthRequestError("x", 403));
    expect(forbidden.message).toContain("管理员权限");
    expect(forbidden.unauthorized).toBe(false);
  });

  it("非 AuthRequestError 走兜底文案", () => {
    expect(describeAuthError(new Error("boom")).message).toContain("暂时无法");
  });
});

describe("describeMutation", () => {
  it("说清角色变更后需要重新登录", () => {
    expect(
      describeMutation({ user_id: 2, is_admin: true, session_revoked: true }),
    ).toContain("重新登录");
  });

  it("session_revoked=false 时额外提示旧登录可能仍有效", () => {
    const text = describeMutation({
      user_id: 2,
      is_admin: false,
      session_revoked: false,
    });
    expect(text).toContain("重新登录");
    expect(text).toContain("仍然有效");
  });
});

describe("rowActionFor", () => {
  it("普通用户给提升，管理员给撤销", () => {
    expect(rowActionFor(user({ user_id: 5, is_admin: false }), 1)).toEqual({
      kind: "grant",
    });
    expect(rowActionFor(user({ user_id: 5, is_admin: true }), 1)).toEqual({
      kind: "revoke",
    });
  });

  it("认得出自己，不给撤销入口", () => {
    expect(rowActionFor(user({ user_id: 1, is_admin: true }), 1)).toEqual({
      kind: "self",
    });
    // 即便这一行在名单里标成非管理员，只要 user_id == 登录人，也不该给出提升/撤销
    expect(rowActionFor(user({ user_id: 1, is_admin: false }), 1)).toEqual({
      kind: "self",
    });
  });

  it("身份未知时保守处理，不给出任何变更入口", () => {
    // 这正是被替换掉的那个 bug：currentUserId 为 null 时会落进 revoke 分支
    expect(rowActionFor(user({ user_id: 5, is_admin: true }), null)).toEqual({
      kind: "unknown",
    });
    expect(rowActionFor(user({ user_id: 5, is_admin: false }), null)).toEqual({
      kind: "unknown",
    });
  });
});

describe("useAdminPermissions loading", () => {
  it("拉取用户列表与管理员名单", async () => {
    const deps = makeDeps({
      listUsers: async () => ({
        items: [user({ user_id: 7, name: "Ada" })],
        total: 42,
      }),
      listAdmins: async () => ({ items: [user({ user_id: 7, is_admin: true })] }),
    });
    const state = useAdminPermissions(deps);

    await state.loadAll();

    expect(state.users.value).toHaveLength(1);
    expect(state.total.value).toBe(42);
    expect(state.admins.value).toHaveLength(1);
    expect(state.usersError.value).toBe("");
    expect(state.adminsError.value).toBe("");
  });

  it("一次并行加载只确认一次会话，且初始即为加载中", async () => {
    const fetchCurrentUser = vi.fn(async () => ({
      user_id: 1,
      username: "ada",
      role: "admin",
    }));
    const state = useAdminPermissions(makeDeps({ fetchCurrentUser }));

    // 首帧就该是「加载中」，否则会闪一下空态
    expect(state.usersLoading.value).toBe(true);
    expect(state.adminsLoading.value).toBe(true);

    await state.loadAll();

    // 两个加载函数共用一次 /me；各调一次的话就是 3 次（加上路由守卫那次）
    expect(fetchCurrentUser).toHaveBeenCalledTimes(1);
  });

  it("身份只在批次开始时取一次，翻页不重复取", async () => {
    const fetchCurrentUser = vi.fn(async () => ({
      user_id: 1,
      username: "ada",
      role: "admin",
    }));
    const state = useAdminPermissions(
      makeDeps({
        fetchCurrentUser,
        listUsers: async () => ({ items: [user()], total: 60 }),
      }),
    );

    await state.loadAll();
    // loadAll 里 loadCurrentUser / loadUsers / loadAdmins 三者并发，但 /me 只打一次
    expect(fetchCurrentUser).toHaveBeenCalledTimes(1);

    await state.changePage(2);
    // 翻页只换数据，currentUserId 不会变，没必要再问一次 /me
    expect(fetchCurrentUser).toHaveBeenCalledTimes(1);
  });

  it("列表项的类型契约里没有密码字段", async () => {
    // 「不暴露密码」的真正保障在后端：AdminUserResp 刻意不含 Password，序列化时
    // 就没有这个字段（apps/api/internal/dto/response/user_resp.go）。前端这层
    // 不做字段白名单（只是搬运 result.items），所以能验的是**类型契约**：
    // 构造一个完整的 AdminUser 不需要、也不接受 password。
    // 运行时「响应里没有 password」由 api/auth.test.ts 的 toEqual 精确断言把关。
    // 注意：detail 字段由 fixture helper 在 loadUsers 内补全（#57 落地前的过渡），
    // 所以这里用 user_id=7 落在 fallback 分支（user_id % 7 === 0），detail 也会被
    // 填充为「未填」档位。字段集合不变性仍可断言。
    const deps = makeDeps({
      listUsers: async () => ({ items: [user({ user_id: 7 })], total: 1 }),
    });
    const state = useAdminPermissions(deps);

    await state.loadUsers();

    const row: AdminUser = state.users.value[0]!;
    // 只应存在约定字段；password 不在 AdminUser 上（见 auth.ts 的接口定义）
    expect(Object.keys(row).sort()).toEqual([
      "created_at",
      "detail",
      "is_admin",
      "name",
      "user_id",
    ]);
  });

  it("401 与 403 都能被区分，且只有 401 摘掉重试入口", async () => {
    // 注意：清会话已不归这里管——受保护请求统一走 api/session-request.ts，
    // 401 由它 clearSession + 跳登录（见 api/session-request.test.ts）。
    // 本层只负责把状态码翻成给用户看的话，并决定要不要露出「重试」。
    const forbidden = useAdminPermissions(
      makeDeps({
        listUsers: async () => {
          throw new AuthRequestError("x", 403);
        },
      }),
    );

    await forbidden.loadUsers();
    expect(forbidden.usersError.value).toContain("管理员权限");
    expect(forbidden.usersUnauthorized.value).toBe(false);

    const expired = useAdminPermissions(
      makeDeps({
        listUsers: async () => {
          throw new AuthRequestError("x", 401);
        },
      }),
    );

    await expired.loadUsers();
    expect(expired.usersError.value).toContain("重新登录");
    expect(expired.usersUnauthorized.value).toBe(true);
  });

  it("用户列表失败不影响管理员名单", async () => {
    const deps = makeDeps({
      listUsers: async () => {
        throw new AuthRequestError("x", 500);
      },
      listAdmins: async () => ({ items: [user({ user_id: 9, is_admin: true })] }),
    });
    const state = useAdminPermissions(deps);

    await state.loadAll();

    expect(state.usersError.value).not.toBe("");
    expect(state.users.value).toHaveLength(0);
    expect(state.admins.value).toHaveLength(1);
    expect(state.adminsError.value).toBe("");
  });

  it("筛选只作用于当前页", async () => {
    const deps = makeDeps({
      listUsers: async () => ({
        items: [
          user({ user_id: 1, name: "Ada", is_admin: true }),
          user({ user_id: 2, name: "Bob", is_admin: false }),
        ],
        total: 2,
      }),
    });
    const state = useAdminPermissions(deps);
    await state.loadUsers();

    state.keyword.value = "bob";
    expect(state.filteredUsers.value.map((u) => u.user_id)).toEqual([2]);

    state.keyword.value = "";
    state.roleFilter.value = "admin";
    expect(state.filteredUsers.value.map((u) => u.user_id)).toEqual([1]);
  });
});

describe("useAdminPermissions mutations", () => {
  it("提升后刷新两张表并提示重新登录", async () => {
    const listUsers = vi.fn(async () => ({ items: [] as AdminUser[], total: 0 }));
    const listAdmins = vi.fn(async () => ({ items: [] as AdminUser[] }));
    const deps = makeDeps({ listUsers, listAdmins });
    const state = useAdminPermissions(deps);

    CONFIRM.mockResolvedValue("confirm" as never);
    await state.grant(user({ user_id: 7, name: "Ada" }));

    // 初始未加载，变更触发的刷新应各自请求一次
    expect(listUsers).toHaveBeenCalledTimes(1);
    expect(listAdmins).toHaveBeenCalledTimes(1);
    expect(SUCCESS).toHaveBeenCalledTimes(1);
    expect(String(SUCCESS.mock.calls[0]?.[0])).toContain("重新登录");
  });

  it("用户取消确认时不发请求", async () => {
    const grantAdmin = vi.fn();
    const state = useAdminPermissions(makeDeps({ grantAdmin }));
    CONFIRM.mockRejectedValue("cancel");

    await state.grant(user({ user_id: 7 }));

    expect(grantAdmin).not.toHaveBeenCalled();
    expect(SUCCESS).not.toHaveBeenCalled();
  });

  it("撤销自己会被前端拦下且不发请求", async () => {
    const revokeAdmin = vi.fn();
    const state = useAdminPermissions(makeDeps({ revokeAdmin }));
    await state.loadAll(); // 让 currentUserId=1 就位（身份由 loadAll 里的 loadCurrentUser 取）

    await state.revoke(user({ user_id: 1, is_admin: true }));

    expect(revokeAdmin).not.toHaveBeenCalled();
    expect(CONFIRM).not.toHaveBeenCalled();
    expect(WARNING).toHaveBeenCalledTimes(1);
  });

  it("认不出登录人时不发撤销请求，并解释原因", async () => {
    // 拿到当前用户就失败（如 403），currentUserId 停在 null
    const revokeAdmin = vi.fn();
    const state = useAdminPermissions(
      makeDeps({
        revokeAdmin,
        fetchCurrentUser: async () => {
          throw new AuthRequestError("x", 403);
        },
      }),
    );
    await state.loadAll();

    await state.revoke(user({ user_id: 7, is_admin: true }));

    expect(revokeAdmin).not.toHaveBeenCalled();
    expect(CONFIRM).not.toHaveBeenCalled();
    expect(String(WARNING.mock.calls[0]?.[0])).toBe(UNKNOWN_IDENTITY_NOTE);
  });

  it("撤销他人失败时给出可读提示", async () => {
    const state = useAdminPermissions(
      makeDeps({
        revokeAdmin: async () => {
          throw new AuthRequestError("x", 403);
        },
      }),
    );
    // 先让 currentUserId 就位（现在由 loadAll 里的 loadCurrentUser 负责），
    // 否则自锁守卫会在「认不出登录人」时先拦下，测不到后端报错分支
    await state.loadAll();
    CONFIRM.mockResolvedValue("confirm" as never);

    await state.revoke(user({ user_id: 7, is_admin: true }));

    expect(ERROR).toHaveBeenCalledTimes(1);
    expect(String(ERROR.mock.calls[0]?.[0])).toContain("管理员权限");
    expect(SUCCESS).not.toHaveBeenCalled();
  });
});
