import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import { clearAuthSession } from "@/auth/session";

vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

import { ElMessage } from "element-plus";

import type { MyProfileUpdate, MyProfile } from "@/api/profile";
import { currentProfile, profileDisplayName, setCurrentProfile } from "../profileIdentity";

import {
  nextFormFromProfile,
  useUserSettings,
  type UserSettingsDeps,
} from "./useUserSettings";

const SUCCESS = vi.mocked(ElMessage.success);
const ERROR = vi.mocked(ElMessage.error);

afterEach(() => {
  vi.clearAllMocks();
  setCurrentProfile(null);
  vi.unstubAllGlobals();
});

function profile(overrides: Partial<MyProfile> = {}): MyProfile {
  return {
    user_id: 1,
    username: "ada",
    name: "Ada",
    role: "member",
    detail: {
      class: "物联网230301",
      student_id: 202311451,
      qq: "1044696157",
      direction: "front-end",
      email: "ada@example.edu.cn",
    },
    ...overrides,
  };
}

function makeDeps(overrides: Partial<UserSettingsDeps> = {}): UserSettingsDeps {
  return {
    fetchMyApplications: async () => ({ items: [] }),
    fetchMyProfile: async () => profile(),
    updateMyProfile: async (input) =>
      profile({ name: input.name ?? profile().name, detail: { ...profile().detail, ...input } }),
    unfreeze: async () => ({ user_id: 1, frozen: false, access_token: "a", refresh_token: "r", scope: "full" }),
    ...overrides,
  };
}

describe("useUserSettings（字段级自动保存，#149）", () => {
  it("loads profile on first call and hydrates the form", async () => {
    const state = useUserSettings(makeDeps());
    expect(state.loading.value).toBe(true);
    await state.load();
    expect(state.form).toEqual({
      name: "Ada",
      class: "物联网230301",
      studentId: "202311451",
      qq: "1044696157",
      direction: "front-end",
    });
    expect(state.needsCompletion.value).toBe(false);
  });

  it("saves only the edited field on blur and marks it saved", async () => {
    const update = vi.fn(async (_input?: MyProfileUpdate, _userId?: number) => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();

    state.form.class = "物联网240201";
    await state.saveField("class");

    // 只带 class 字段，不带姓名/学号/QQ
    expect(update).toHaveBeenCalledTimes(1);
    const [payload, userId] = update.mock.calls[0]!;
    expect(payload).toEqual({ class: "物联网240201" });
    expect(userId).toBe(1);
    expect(state.fieldStatuses.class).toBe("saved");
  });

  it("skips the request when a field value is unchanged", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    await state.saveField("name"); // name 没改
    expect(update).not.toHaveBeenCalled();
    expect(state.fieldStatuses.name).toBe("idle");
  });

  it("blocks invalid names before any request, counting Unicode code points", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    for (const name of ["", "张", "张".repeat(21), "Alice", "张三A", "张三1", "张三 "]) {
      state.form.name = name;
      await state.saveField("name");
      expect(state.fieldErrors.value.name).toContain("2–20");
    }
    expect(update).not.toHaveBeenCalled();
    // 输入保留，不被清空
    expect(state.form.name).toBe("张三 ");
  });

  it("accepts two Chinese characters and persists", async () => {
    const update = vi.fn(async (input) => profile({ name: input.name }));
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.name = "张三";
    await state.saveField("name");
    expect(update).toHaveBeenCalledWith({ name: "张三" }, 1);
    expect(state.fieldStatuses.name).toBe("saved");
  });

  it("blocks empty class / invalid student id without submitting", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.class = " ";
    state.form.studentId = "-1";
    await state.saveField("class");
    await state.saveField("studentId");
    expect(state.fieldErrors.value.class).toBeTruthy();
    expect(state.fieldErrors.value.studentId).toBeTruthy();
    expect(update).not.toHaveBeenCalled();
    expect(state.form.studentId).toBe("-1");
  });

  it("blocks saving when latest application direction conflicts with stored direction", async () => {
    const items = [{ direction_code: "back-end" }];
    const fetchMyApplications = vi.fn(async () => ({ items }));
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ fetchMyApplications, updateMyProfile: update }));
    await state.load();
    expect(state.latestDirection.value).toBe("back-end");
    expect(state.directionError.value).toContain("联系管理员");

    state.form.class = "物联网240201";
    await state.saveField("class");
    expect(update).not.toHaveBeenCalled();
    expect(state.fieldStatuses.class).toBe("error");
  });

  it("fails closed when direction check itself errors", async () => {
    const fetchMyApplications = vi.fn(async () => { throw new Error("offline"); });
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ fetchMyApplications, updateMyProfile: update }));
    await state.load();
    expect(state.directionError.value).toContain("重试");
    state.form.qq = "99999";
    await state.saveField("qq");
    expect(update).not.toHaveBeenCalled();
  });

  it("marks error on save failure, keeps input, and retryField recovers", async () => {
    let calls = 0;
    const update = vi.fn(async () => {
      calls++;
      if (calls === 1) throw new AuthRequestError("服务暂时不可用", 0);
      return profile();
    });
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.class = "物联网240201";
    await state.saveField("class");
    expect(state.fieldStatuses.class).toBe("error");
    expect(state.fieldErrors.value.class).toBeTruthy();
    expect(state.form.class).toBe("物联网240201");

    await state.retryField("class");
    expect(state.fieldStatuses.class).toBe("saved");
    expect(update).toHaveBeenCalledTimes(2);
  });

  it("discards a stale in-flight response when the user edits the field again", async () => {
    let resolveFirst!: (v: MyProfile) => void;
    const update = vi.fn(() => new Promise<MyProfile>((r) => { resolveFirst = r; }));
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();

    state.form.qq = "11111";
    const p = state.saveField("qq");
    await vi.waitFor(() => expect(resolveFirst).toBeTypeOf("function"));

    // 响应回来前用户又改了 QQ
    state.form.qq = "22222";

    resolveFirst(profile({ detail: { ...profile().detail, qq: "11111" } }));
    await p;

    expect(update).toHaveBeenCalledTimes(1);
    // 表单保留用户最新输入，旧响应没覆盖
    expect(state.form.qq).toBe("22222");
    expect(state.fieldStatuses.qq).toBe("idle");
    // baseline 也没被旧响应污染
    expect(currentProfile.value?.detail?.qq).toBe("1044696157");
  });

  it("does not republish after logout while a save is pending", async () => {
    let resolveSave!: (v: MyProfile) => void;
    const state = useUserSettings(makeDeps({
      updateMyProfile: () => new Promise<MyProfile>((r) => { resolveSave = r; }),
    }));
    await state.load();
    state.form.name = "新姓名";
    const saving = state.saveField("name");
    await vi.waitFor(() => expect(resolveSave).toBeTypeOf("function"));
    clearAuthSession({ length: 0, clear() { }, getItem() { return null; }, key() { return null; }, setItem() { }, removeItem() { } });
    resolveSave(profile({ name: "新姓名" }));
    await saving;
    expect(currentProfile.value).toBeNull();
  });

  it("autoSave merges all dirty fields into one PUT on SPA navigation", async () => {
    const update = vi.fn(async (_input?: MyProfileUpdate) => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.class = "物联网240401";
    state.form.qq = "88888";
    await state.autoSave();
    expect(update).toHaveBeenCalledTimes(1);
    const [payload] = update.mock.calls[0]!;
    expect(payload).toMatchObject({ class: "物联网240401", qq: "88888" });
  });

  it("waits for an in-flight field save before auto-saving the latest draft", async () => {
    let resolveFirst!: (value: MyProfile) => void;
    let resolveSecond!: (value: MyProfile) => void;
    const update = vi.fn()
      .mockImplementationOnce(() => new Promise<MyProfile>((resolve) => { resolveFirst = resolve; }))
      .mockImplementationOnce(() => new Promise<MyProfile>((resolve) => { resolveSecond = resolve; }));
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();

    state.form.class = "物联网240201";
    const fieldSave = state.saveField("class");
    await vi.waitFor(() => expect(resolveFirst).toBeTypeOf("function"));

    state.form.class = "物联网240301";
    const pageSave = state.autoSave();
    await Promise.resolve();
    expect(update).toHaveBeenCalledTimes(1);

    resolveFirst(profile({ detail: { ...profile().detail, class: "物联网240201" } }));
    await fieldSave;
    await vi.waitFor(() => expect(resolveSecond).toBeTypeOf("function"));
    expect(update.mock.calls[1]?.[0]).toMatchObject({ class: "物联网240301" });

    resolveSecond(profile({ detail: { ...profile().detail, class: "物联网240301" } }));
    await pageSave;
    expect(update).toHaveBeenCalledTimes(2);
    expect(state.form.class).toBe("物联网240301");
  });

  it("coalesces overlapping page auto-saves into one PUT", async () => {
    let resolveSave!: (value: MyProfile) => void;
    const update = vi.fn(() => new Promise<MyProfile>((resolve) => { resolveSave = resolve; }));
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.qq = "88888";

    const firstSave = state.autoSave();
    const secondSave = state.autoSave();
    await vi.waitFor(() => expect(resolveSave).toBeTypeOf("function"));
    expect(update).toHaveBeenCalledTimes(1);

    resolveSave(profile({ detail: { ...profile().detail, qq: "88888" } }));
    await Promise.all([firstSave, secondSave]);
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("autoSave skips when a dirty field fails validation", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.class = "物联网240401";
    state.form.studentId = "0"; // 非法
    expect(await state.autoSave()).toBe(false);
    expect(update).not.toHaveBeenCalled();
  });

  it("keeps invalid and failed auto-save drafts retryable", async () => {
    const update = vi.fn(async () => { throw new AuthRequestError("服务暂时不可用", 0); });
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();
    state.form.qq = "88888";

    expect(await state.autoSave()).toBe(false);
    expect(state.form.qq).toBe("88888");
    expect(state.fieldStatuses.qq).toBe("error");
    expect(state.fieldErrors.value.qq).toContain("网络异常");
  });

  it("persistOnUnload skips invalid input without firing fetch", async () => {
    const request = vi.fn();
    vi.stubGlobal("fetch", request);
    try {
      const state = useUserSettings(makeDeps());
      await state.load();
      state.form.name = "张1"; // 非法
      state.persistOnUnload();
      expect(request).not.toHaveBeenCalled();
    } finally { vi.unstubAllGlobals(); }
  });

  it("admin can save direction via dropdown", async () => {
    const update = vi.fn(async (input) =>
      profile({ role: "admin", detail: { ...profile().detail, direction: input.direction ?? null } }),
    );
    const state = useUserSettings(makeDeps({
      fetchMyProfile: async () => profile({ role: "admin" }),
      updateMyProfile: update,
    }));
    await state.load();
    state.form.direction = "back-end";
    await state.saveDirection();
    expect(update).toHaveBeenCalledWith({ direction: "back-end" }, 1);
    expect(SUCCESS).toHaveBeenCalledWith("方向已保存");
  });

  it("flips needsCompletion when required detail is missing", async () => {
    const state = useUserSettings(makeDeps({
      fetchMyProfile: async () => profile({ detail: { class: "", student_id: undefined, qq: "", direction: null, email: "" } }),
    }));
    await state.load();
    expect(state.needsCompletion.value).toBe(true);
  });

  it("translates a load failure into loadError + loadUnauthorized on 401", async () => {
    const state = useUserSettings(makeDeps({
      fetchMyProfile: async () => { throw new AuthRequestError("token 已过期", 401, 4002); },
    }));
    await state.load();
    expect(state.loadError.value).toContain("重新登录");
    expect(state.loadUnauthorized.value).toBe(true);
  });

  it("loads profile without detail (pre-#57 backend) without crashing", async () => {
    const state = useUserSettings(makeDeps({
      fetchMyProfile: async () => {
        const { detail: _d, ...rest } = profile();
        return rest;
      },
    }));
    await state.load();
    expect(state.form).toEqual({ name: "Ada", class: "", studentId: "", qq: "", direction: "" });
    expect(state.needsCompletion.value).toBe(true);
  });
});

/* ------------------------------------------------------------------ *
 * 0012 自助解冻：受限会话必须能存下资料
 *
 * 这一组守着本轮修的 bug：受限令牌访问 /recruitment/applications 必然
 * 403，而方向核对失败会让 runFieldSave / autoSave 拦下所有写入。
 * 结果就是「表单填满了却存不进去 → 后端读到的还是旧资料 → 解不了冻」，
 * 整个自助解冻闭环被方向校验锁死。
 * ------------------------------------------------------------------ */

/** 受限会话看得到的 403：后端 RequireFullScope 拒绝时的原样错误。 */
function scopeForbidden(): AuthRequestError {
  return new AuthRequestError("账号资料未补全，请先补齐后再使用其它功能", 403, 4003);
}

describe("useUserSettings — 受限会话（0012 自助解冻）", () => {
  it("skips the direction check instead of failing closed for a restricted session", async () => {
    const fetchMyApplications = vi.fn(async () => ({ items: [] }));
    const update = vi.fn(async () => profile({ detail: { ...profile().detail, class: "软件工程230301" } }));
    const state = useUserSettings(makeDeps({
      fetchMyApplications,
      updateMyProfile: update,
      isRestrictedSession: () => true,
    }));
    await state.load();

    // 关键：受限会话压根不去打注定 403 的业务端点。
    expect(fetchMyApplications).not.toHaveBeenCalled();
    expect(state.directionError.value).toBe("");

    state.form.class = "软件工程230301";
    await state.saveField("class");

    expect(state.directionError.value).toBe("");
    expect(update).toHaveBeenCalledWith({ class: "软件工程230301" }, 1);
    expect(state.fieldStatuses.class).toBe("saved");
  });

  it("saves the whole form on autoSave for a restricted session", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({
      updateMyProfile: update,
      isRestrictedSession: () => true,
    }));
    await state.load();

    state.form.class = "软件工程230301";
    state.form.studentId = "202311451";

    // autoSave 是「提交解冻」前的落盘步骤：返回 false 就意味着永远解不了冻。
    expect(await state.autoSave()).toBe(true);
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("treats a 403 from the direction check as frozen, not as a direction failure", async () => {
    // 本地会话缺 scope（旧存储格式 / 旧版本登录）时认不出受限状态，
    // 服务端仍会回 403——这条兜底保证那种会话也能存下资料。
    const fetchMyApplications = vi.fn(async () => { throw scopeForbidden(); });
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({
      fetchMyApplications,
      updateMyProfile: update,
      isRestrictedSession: () => false,
    }));
    await state.load();

    expect(state.directionError.value).toBe("");
    state.form.class = "软件工程230301";
    await state.saveField("class");
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("still fails closed on a non-403 direction error", async () => {
    // 网络异常与 403 必须区别对待：前者是真的核对不了，照旧禁止写入。
    const fetchMyApplications = vi.fn(async () => { throw new AuthRequestError("服务暂时不可用", 0); });
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({
      fetchMyApplications,
      updateMyProfile: update,
      isRestrictedSession: () => false,
    }));
    await state.load();

    expect(state.directionError.value).toContain("重试");
    state.form.class = "软件工程230301";
    await state.saveField("class");
    expect(update).not.toHaveBeenCalled();
  });

  it("adopts the new tokens when a save response says the account was unfrozen", async () => {
    // PUT /me 顺手解冻时后端会带回新双令牌；不换掉，用户手里还是受限令牌，
    // 之后每一步都是 403。
    const persistSession = vi.fn();
    const update = vi.fn(async () => profile({
      access_token: "full-access",
      refresh_token: "full-refresh",
      scope: "full",
      unfrozen: true,
    }));
    const state = useUserSettings(makeDeps({
      updateMyProfile: update,
      isRestrictedSession: () => true,
      persistSession,
    }));
    await state.load();

    state.form.class = "软件工程230301";
    await state.saveField("class");

    expect(persistSession).toHaveBeenCalledWith({
      access_token: "full-access",
      refresh_token: "full-refresh",
      user_id: 1,
      scope: "full",
      frozen: false,
    });
    // 「提交解冻」据此跳过 POST /me/unfreeze：旧令牌已被换掉，再打只会 401。
    expect(state.autoUnfrozen.value).toBe(true);
  });

  it("leaves the session alone when a save response carries no tokens", async () => {
    // 后端只在真的解冻时返回令牌。无条件替换会把别的标签页顶下线。
    const persistSession = vi.fn();
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({
      updateMyProfile: update,
      isRestrictedSession: () => true,
      persistSession,
    }));
    await state.load();

    state.form.class = "软件工程230301";
    await state.saveField("class");

    expect(persistSession).not.toHaveBeenCalled();
  });
});

describe("useUserSettings — 提交解冻（0012）", () => {
  it("flushes the filled-in form before asking the backend to re-check", async () => {
    // 这是用户报的现场：表单看得见都填好了，但那只是本地 v-model。
    // 不先落盘就提交，后端读库仍是旧资料，只能回「仍不全」。
    const update = vi.fn(async (_input: MyProfileUpdate, _userId: number) => profile());
    const unfreeze = vi.fn(async () => ({
      user_id: 1,
      frozen: false,
      scope: "full" as const,
      access_token: "a",
      refresh_token: "r",
    }));
    const state = useUserSettings(makeDeps({
      updateMyProfile: update,
      unfreeze,
      isRestrictedSession: () => true,
      persistSession: vi.fn(),
    }));
    await state.load();

    state.form.name = "张三";
    state.form.class = "软件工程230301";
    state.form.studentId = "202311451";

    const outcome = await state.submitUnfreeze();

    expect(outcome).toEqual({ kind: "unfrozen" });
    // 先 PUT /me 落盘，再 POST /me/unfreeze（没有 toHaveBeenCalledBefore，用调用序验证）。
    expect(update).toHaveBeenCalledTimes(1);
    expect(unfreeze).toHaveBeenCalledTimes(1);
    expect(update.mock.invocationCallOrder[0]!).toBeLessThan(
      unfreeze.mock.invocationCallOrder[0]!,
    );
    expect(update.mock.calls[0]![0]).toMatchObject({
      name: "张三",
      class: "软件工程230301",
      student_id: 202311451,
    });
  });

  it("does not call the unfreeze endpoint when saving the form failed", async () => {
    // 落盘失败还去打解冻接口，用户只会再吃一次「仍不全」，且不知道是保存的问题。
    const unfreeze = vi.fn();
    const state = useUserSettings(makeDeps({
      updateMyProfile: async () => { throw new AuthRequestError("服务暂时不可用", 0); },
      unfreeze,
      isRestrictedSession: () => true,
    }));
    await state.load();

    state.form.class = "软件工程230301";

    expect(await state.submitUnfreeze()).toEqual({ kind: "unsaved" });
    expect(unfreeze).not.toHaveBeenCalled();
    // 草稿必须留着，让用户改完能重试。
    expect(state.form.class).toBe("软件工程230301");
  });

  it("skips the extra unfreeze call when the save already auto-unfroze", async () => {
    // PUT /me 已经换发了完整令牌，旧令牌随即失效；
    // 再打 /me/unfreeze 只会拿到 401。
    const unfreeze = vi.fn();
    const state = useUserSettings(makeDeps({
      updateMyProfile: async () => profile({
        access_token: "full-access",
        refresh_token: "full-refresh",
        scope: "full",
        unfrozen: true,
      }),
      unfreeze,
      isRestrictedSession: () => true,
      persistSession: vi.fn(),
    }));
    await state.load();

    state.form.class = "软件工程230301";

    expect(await state.submitUnfreeze()).toEqual({ kind: "unfrozen" });
    expect(unfreeze).not.toHaveBeenCalled();
  });

  it("reports the remaining issues and refreshes the session list when still incomplete", async () => {
    // frozen=true 是业务结果（200），不是错误：要把清单交回去渲染，
    // 并覆盖登录时那份旧清单，否则用户会对着已经改好的项发愁。
    const issues = [{ field: "class", message: "班级须为「汉字 + 6 位数字」" }];
    const persistSession = vi.fn();
    const state = useUserSettings(makeDeps({
      unfreeze: async () => ({ user_id: 1, frozen: true, freeze_issues: issues }),
      isRestrictedSession: () => true,
      persistSession,
      readSession: () => ({ access_token: "a", refresh_token: "r", user_id: 1, scope: "profile_only", frozen: true }),
    }));
    await state.load();

    expect(await state.submitUnfreeze()).toEqual({ kind: "incomplete", issues });
    expect(persistSession).toHaveBeenCalledWith(
      expect.objectContaining({ frozen: true, freeze_issues: issues }),
    );
  });

  it("turns a failed unfreeze request into a readable message", async () => {
    const state = useUserSettings(makeDeps({
      unfreeze: async () => { throw new AuthRequestError("当前账号未被冻结，无需解冻", 400); },
      isRestrictedSession: () => true,
    }));
    await state.load();

    const outcome = await state.submitUnfreeze();
    expect(outcome).toEqual({ kind: "failed", message: "当前账号未被冻结，无需解冻" });
  });
});

describe("nextFormFromProfile", () => {
  it("maps detail into the form shape", () => {
    expect(nextFormFromProfile(profile())).toEqual({
      name: "Ada",
      class: "物联网230301",
      studentId: "202311451",
      qq: "1044696157",
      direction: "front-end",
    });
  });
});
