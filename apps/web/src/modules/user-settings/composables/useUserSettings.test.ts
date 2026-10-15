import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";
import { clearAuthSession } from "@/auth/session";

// ElMessage 用作保存成功 / 失败的 toast；纯副作用，桩掉即可。
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
});

function profile(overrides: Partial<MyProfile> = {}): MyProfile {
  return {
    user_id: 1,
    username: "ada",
    name: "Ada",
    role: "member",
    detail: {
      class: "iot 2301",
      student_id: 2023114514,
      qq: "1044696157",
      direction: "front-end",
      email: "ada@example.edu.cn",
    },
    ...overrides,
  };
}

function makeDeps(overrides: Partial<UserSettingsDeps> = {}): UserSettingsDeps {
  return {
    fetchMyApplications: async () => ({ items: [], total: 0, page: 1, page_size: 20 }),
    fetchMyProfile: async () => profile(),
    updateMyProfile: async (input) => profile({ name: input.name ?? "", detail: { ...profile().detail, ...input } }),
    ...overrides,
  };
}

describe("useUserSettings", () => {
  it("does not submit from read-only mode or after logout during direction verification", async () => {
    let resolve!: (value: { items: [] }) => void;
    const update = vi.fn(async () => profile());
    const fetchMyApplications = vi.fn(async () => ({ items: [] }));
    const state = useUserSettings(makeDeps({ updateMyProfile: update, fetchMyApplications }));
    await state.load();
    expect(await state.save()).toBe(false);
    state.edit();
    fetchMyApplications.mockImplementationOnce(() => new Promise(r => { resolve = r; }));
    const pending = state.save();
    setCurrentProfile(null);
    resolve({ items: [] });
    expect(await pending).toBe(false);
    expect(update).not.toHaveBeenCalled();
  });

  it("retries direction verification without discarding edited fields", async () => {
    const state = useUserSettings(makeDeps());
    await state.load(); state.edit();
    state.form.class = "unsaved";
    await state.retryDirection();
    expect(state.form.class).toBe("unsaved");
    expect(state.editing.value).toBe(true);
  });
  it("starts read-only, resets drafts and exits editing on cancel or success", async () => {
    const state = useUserSettings(makeDeps());
    await state.load();
    expect(state.editing.value).toBe(false);
    state.edit();
    state.form.class = "draft";
    state.reset();
    expect(state.form.class).toBe("iot 2301");
    expect(state.editing.value).toBe(true);
    state.form.class = "another draft";
    state.cancel();
    expect(state.editing.value).toBe(false);
    expect(state.form.class).toBe("iot 2301");
    state.edit();
    expect(await state.save()).toBe(true);
    expect(state.editing.value).toBe(false);
  });

  it("blocks invalid class/student ID without submitting and keeps the draft", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load(); state.edit();
    state.form.class = " "; state.form.studentId = "-1";
    expect(await state.save()).toBe(false);
    expect(state.fieldErrors.value.class).toBeTruthy();
    expect(state.fieldErrors.value.studentId).toBeTruthy();
    expect(state.form.studentId).toBe("-1");
    expect(update).not.toHaveBeenCalled();
    expect(state.editing.value).toBe(true);
  });

  it("automatically fills an empty direction from the latest application and blocks conflicting saved values", async () => {
    const items = [{ direction_code: "back-end" }, { direction_code: "front-end" }];
    const fetchMyApplications = vi.fn(async () => ({ items, total: 2, page: 1, page_size: 20 }));
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ fetchMyApplications, updateMyProfile: update }));
    await state.load(); state.edit();
    expect(state.latestDirection.value).toBe("back-end");
    expect(await state.save()).toBe(false);
    expect(state.directionError.value).toContain("联系管理员");
    expect(update).not.toHaveBeenCalled();
    const empty = useUserSettings(makeDeps({
      fetchMyProfile: async () => profile({ detail: { ...profile().detail, direction: null } }),
      fetchMyApplications,
    }));
    await empty.load(); empty.edit();
    expect(empty.form.direction).toBe("back-end");
    expect(await empty.save()).toBe(true);
  });

  it("fails closed if application loading fails or a newer application conflicts before saving", async () => {
    const update = vi.fn(async () => profile());
    let unavailable = false;
    let direction_code = "front-end";
    const state = useUserSettings(makeDeps({
      updateMyProfile: update,
      fetchMyApplications: async () => {
        if (unavailable) throw new Error("offline");
        return { items: [{ direction_code }], total: 1, page: 1, page_size: 20 };
      },
    }));
    await state.load(); state.edit();
    direction_code = "back-end";
    expect(await state.save()).toBe(false);
    unavailable = true;
    expect(await state.save()).toBe(false);
    expect(state.directionError.value).toContain("重试");
    expect(update).not.toHaveBeenCalled();
  });
  it("does not republish an account after logout while its save is pending", async () => {
    let resolveSave!: (value: MyProfile) => void;
    const state = useUserSettings(makeDeps({
      updateMyProfile: () => new Promise(resolve => { resolveSave = resolve; }),
    }));
    await state.load(); state.edit();
    const saving = state.save();
    await vi.waitFor(() => expect(resolveSave).toBeTypeOf("function"));
    clearAuthSession({ length: 0, clear() {}, getItem() { return null; }, key() { return null; }, setItem() {}, removeItem() {} });
    resolveSave(profile({ name: "退出前的姓名" }));
    await saving;
    expect(currentProfile.value).toBeNull();
  });
  it("saves the name, publishes the server identity and reloads it", async () => {
    let stored = profile();
    const state = useUserSettings(makeDeps({
      fetchMyProfile: async () => stored,
      updateMyProfile: async (input) => {
        stored = profile({ name: input.name ?? "" });
        return stored;
      },
    }));
    await state.load(); state.edit();
    state.form.name = " 新的姓名 ";
    expect(await state.save()).toBe(true);
    expect(state.form.name).toBe("新的姓名");
    expect(profileDisplayName(currentProfile.value)).toBe("新的姓名");
    await state.load(); state.edit();
    expect(state.form.name).toBe("新的姓名");
  });

  it("rejects invalid names before making a request, counting Unicode code points", async () => {
    const update = vi.fn(async () => profile());
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load(); state.edit();
    for (const name of ["张三", "a".repeat(21), "𠮷".repeat(21)]) {
      state.form.name = name;
      expect(await state.save()).toBe(false);
      expect(state.nameError.value).toContain("3–20");
    }
    expect(update).not.toHaveBeenCalled();
    state.form.name = "𠮷".repeat(20);
    expect(await state.save()).toBe(true);
  });

  it("clears the name with null and never falls back to the old JWT name", async () => {
    const update = vi.fn(async (_input: MyProfileUpdate) => profile({ name: "" }));
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load(); state.edit();
    state.form.name = "  ";
    expect(await state.save()).toBe(true);
    expect(update.mock.calls[0]?.[0]).toMatchObject({ name: null });
    expect(profileDisplayName(currentProfile.value)).toBe("ada@example.edu.cn");
    expect(state.form.name).toBe("");
  });

  it("loads profile on first call and hydrates the form", async () => {
    const state = useUserSettings(makeDeps());

    // 调用前先把 loading=false 的初态观察一下：用于确认 load() 的方向
    expect(state.loading.value).toBe(true);
    expect(state.profile.value).toBeNull();

    await state.load(); state.edit();

    expect(state.loading.value).toBe(false);
    expect(state.loadError.value).toBe("");
    expect(state.profile.value).not.toBeNull();
    expect(state.form).toEqual({
      name: "Ada",
      class: "iot 2301",
      studentId: "2023114514",
      qq: "1044696157",
      direction: "front-end",
    });
    // 已有内容 → 不提示「建议补全」
    expect(state.needsCompletion.value).toBe(false);
  });

  it("flips needsCompletion when the detail is entirely empty", async () => {
    const state = useUserSettings(
      makeDeps({
        fetchMyProfile: async () =>
          profile({ detail: { class: "", student_id: undefined, qq: "", direction: null, email: "" } }),
      }),
    );

    await state.load(); state.edit();

    expect(state.needsCompletion.value).toBe(true);
  });

  // #57 后端落地前 GET /me 不返回 detail——真实崩溃路径（此前只测过 detail: {}，
  // 空对象不会抛错，测不到 TypeError 落入兜底文案的问题）。
  it("loads a profile without detail (pre-#57 backend) without crashing", async () => {
    const state = useUserSettings(
      makeDeps({
        fetchMyProfile: async () => {
          const { detail: _dropped, ...rest } = profile();
          return rest;
        },
      }),
    );

    await state.load(); state.edit();

    expect(state.loading.value).toBe(false);
    expect(state.loadError.value).toBe("");
    expect(state.profile.value).not.toBeNull();
    expect(state.form).toEqual({ name: "Ada", class: "", studentId: "", qq: "", direction: "" });
    expect(state.needsCompletion.value).toBe(true);
  });

  it("reports profile wording (not admin wording) when hydration throws", async () => {
    const state = useUserSettings(
      makeDeps({
        // 模拟页面自身代码抛出的非 AuthRequestError（如字段形状漂移）
        fetchMyProfile: async () => {
          throw new TypeError("detail is undefined");
        },
      }),
    );

    await state.load(); state.edit();

    expect(state.loadError.value).toContain("个人资料");
    expect(state.loadError.value).not.toContain("管理数据");
  });

  it("translates a load failure into loadError + loadUnauthorized", async () => {
    const state = useUserSettings(
      makeDeps({
        fetchMyProfile: async () => {
          throw new AuthRequestError("token 已过期", 401, 4002);
        },
      }),
    );

    await state.load(); state.edit();

    expect(state.loading.value).toBe(false);
    expect(state.loadError.value).toContain("重新登录");
    expect(state.loadUnauthorized.value).toBe(true);
    expect(state.profile.value).toBeNull();
  });

  it("saves and refreshes the form from the server response", async () => {
    const update = vi.fn(async () =>
      profile({ detail: { ...profile().detail, qq: "99999" } }),
    );
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load(); state.edit();

    state.form.qq = "99999";

    const ok = await state.save();
    expect(ok).toBe(true);
    expect(update).toHaveBeenCalledWith({
      name: "Ada",
      class: "iot 2301",
      student_id: 2023114514,
      qq: "99999",
      direction: "front-end",
    });
    // 保存成功后表单已被最新响应刷过
    expect(state.form.qq).toBe("99999");
    expect(SUCCESS).toHaveBeenCalledWith("资料已保存");
  });

  it("surfaces a save error and keeps the local edit intact", async () => {
    const state = useUserSettings(
      makeDeps({
        updateMyProfile: async () => {
          throw new AuthRequestError("服务暂时不可用，请稍后重试", 0);
        },
      }),
    );
    await state.load(); state.edit();

    state.form.class = "iot 2402";
    state.form.name = "未保存姓名";

    const ok = await state.save();
    expect(ok).toBe(false);
    expect(state.saveError.value).toContain("网络异常");
    expect(state.form.class).toBe("iot 2402");
    expect(state.form.name).toBe("未保存姓名");
    expect(currentProfile.value?.name).toBe("Ada");
    expect(ERROR).toHaveBeenCalledTimes(1);
  });

  it("ignores double-clicks while saving is in flight", async () => {
    let resolveSave!: (value: MyProfile) => void;
    const update = vi.fn(
      () => new Promise<MyProfile>((resolve) => (resolveSave = resolve)),
    );
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load(); state.edit();

    const first = state.save();
    const second = state.save();

    expect(state.saving.value).toBe(true);
    await vi.waitFor(() => expect(resolveSave).toBeTypeOf("function"));
    resolveSave(profile());
    await first;
    await second;

    expect(update).toHaveBeenCalledTimes(1);
  });
});

describe("nextFormFromProfile", () => {
  it("maps detail into the form shape", () => {
    expect(
      nextFormFromProfile(profile()),
    ).toEqual({
      name: "Ada",
      class: "iot 2301",
      studentId: "2023114514",
      qq: "1044696157",
      direction: "front-end",
    });
  });

  it("treats missing detail as empty form (defensive default)", () => {
    // #57 落地前后端不返回 detail；类型上已放宽为可选，这里锁住运行时不抛错
    expect(
      nextFormFromProfile({ ...profile(), detail: undefined }),
    ).toEqual({
      name: "Ada",
      class: "",
      studentId: "",
      qq: "",
      direction: "",
    });
  });
});
