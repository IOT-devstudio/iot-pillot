import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthRequestError } from "@/api/auth";

// ElMessage 用作保存成功 / 失败的 toast；纯副作用，桩掉即可。
vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

import { ElMessage } from "element-plus";

import type { MyProfile } from "@/api/profile";

import {
  nextFormFromProfile,
  useUserSettings,
  type UserSettingsDeps,
} from "./useUserSettings";

const SUCCESS = vi.mocked(ElMessage.success);
const ERROR = vi.mocked(ElMessage.error);

afterEach(() => {
  vi.clearAllMocks();
});

function profile(overrides: Partial<MyProfile> = {}): MyProfile {
  return {
    user_id: 1,
    username: "ada",
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
    fetchMyProfile: async () => profile(),
    updateMyProfile: async (input) => profile({ detail: { ...profile().detail, ...input } }),
    ...overrides,
  };
}

describe("useUserSettings", () => {
  it("loads profile on first call and hydrates the form", async () => {
    const state = useUserSettings(makeDeps());

    // 调用前先把 loading=false 的初态观察一下：用于确认 load() 的方向
    expect(state.loading.value).toBe(true);
    expect(state.profile.value).toBeNull();

    await state.load();

    expect(state.loading.value).toBe(false);
    expect(state.loadError.value).toBe("");
    expect(state.profile.value).not.toBeNull();
    expect(state.form).toEqual({
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

    await state.load();

    expect(state.needsCompletion.value).toBe(true);
  });

  it("translates a load failure into loadError + loadUnauthorized", async () => {
    const state = useUserSettings(
      makeDeps({
        fetchMyProfile: async () => {
          throw new AuthRequestError("token 已过期", 401, 4002);
        },
      }),
    );

    await state.load();

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
    await state.load();

    state.form.qq = "99999";

    const ok = await state.save();
    expect(ok).toBe(true);
    expect(update).toHaveBeenCalledWith({
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
    await state.load();

    state.form.class = "iot 2402";

    const ok = await state.save();
    expect(ok).toBe(false);
    expect(state.saveError.value).toContain("网络异常");
    expect(state.form.class).toBe("iot 2402");
    expect(ERROR).toHaveBeenCalledTimes(1);
  });

  it("ignores double-clicks while saving is in flight", async () => {
    let resolveSave!: (value: MyProfile) => void;
    const update = vi.fn(
      () => new Promise<MyProfile>((resolve) => (resolveSave = resolve)),
    );
    const state = useUserSettings(makeDeps({ updateMyProfile: update }));
    await state.load();

    const first = state.save();
    const second = state.save();

    expect(state.saving.value).toBe(true);
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
      class: "iot 2301",
      studentId: "2023114514",
      qq: "1044696157",
      direction: "front-end",
    });
  });

  it("treats missing detail as empty form (defensive default)", () => {
    // 类型上 detail 必填；强制测一下「类型万一漂移」也不会爆运行时
    expect(
      nextFormFromProfile({ ...profile(), detail: {} as MyProfile["detail"] }),
    ).toEqual({
      class: "",
      studentId: "",
      qq: "",
      direction: "",
    });
  });
});