/**
 * 「个人设置」页的编排：拉取 / 保存用户资料。
 *
 * 套路与 useAdminPermissions 一致：依赖通过参数注入，测试里塞假实现就能跑，
 * 不必起网络、不必起 Vue 组件。受保护请求全部走 apps/web/src/api/session-request.ts，
 * 这里不重复读 token，也不重复翻译 401/403。
 *
 * 契约来自已合并的 #57 / #86：name 可改，邮箱只读，身份取自令牌。
 */
import { computed, reactive, ref, type Ref } from "vue";
import type { Direction } from "@iot-pillot/shared-types";
import { fetchMyApplications, type RecruitmentApplication } from "@/api/recruitment";
import { ElMessage } from "element-plus";

import {
  detailToForm,
  fetchMyProfile,
  formToUpdate,
  updateMyProfile,
  type FormShape,
  type MyProfile,
  type MyProfileUpdate,
} from "@/api/profile";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";
import { beginProfileLoad, beginProfileSave, beginProfileSessionCheck, setCurrentProfile } from "../profileIdentity";
import { missingRequiredProfile, validateRequiredProfile, type RequiredProfileErrors } from "../profileCompletion";

export interface UserSettingsDeps {
  fetchMyProfile: () => Promise<MyProfile>;
  updateMyProfile: (input: MyProfileUpdate, expectedUserId: number) => Promise<MyProfile>;
  fetchMyApplications: () => Promise<{ items: Pick<RecruitmentApplication, "direction_code">[] }>;
}

const defaultDeps: UserSettingsDeps = {
  fetchMyProfile: () => fetchMyProfile(),
  updateMyProfile: (input, expectedUserId) => updateMyProfile(input, expectedUserId),
  fetchMyApplications,
};

/** 表单形状 + 响应式状态 + 保存动作。 */
export interface UseUserSettings {
  profile: Ref<MyProfile | null>;
  form: FormShape;
  loading: Ref<boolean>;
  loadError: Ref<string>;
  loadUnauthorized: Ref<boolean>;
  saving: Ref<boolean>;
  saveError: Ref<string>;
  nameError: Ref<string>;
  editing: Ref<boolean>;
  fieldErrors: Ref<RequiredProfileErrors>;
  latestDirection: Ref<Direction | "">;
  directionError: Readonly<Ref<string>>;
  edit: () => void;
  reset: () => void;
  cancel: () => void;
  retryDirection: () => Promise<void>;
  needsCompletion: Ref<boolean>;
  load: () => Promise<void>;
  save: () => Promise<boolean>;
}

/**
 * 把当前表单形状合并进旧 profile，得到下次展示用的初始表单。
 *
 * 为什么单独抽出来：保存成功后接口返回的是完整 profile，但表单只持有可编辑字段，
 * 不能直接 `Object.assign(form, profile.detail)` —— 那会把 class/student_id 等键
 * 也写进 form（form 用的是 camelCase studentId），并且不会触发响应式刷新。
 */
export function nextFormFromProfile(profile: MyProfile): FormShape {
  return { ...detailToForm(profile.detail), name: profile.name ?? "" };
}

export function useUserSettings(deps: UserSettingsDeps = defaultDeps): UseUserSettings {
  const profile = ref<MyProfile | null>(null);
  const loading = ref(true);
  const loadError = ref("");
  /** 加载失败后是否还把「重试」按钮露出来：401 得先重新登录，重试没意义 */
  const loadUnauthorized = ref(false);

  const saving = ref(false);
  /** 保存失败的提示：保存按钮还要露出来，所以单独持有一份 */
  const saveError = ref("");
  const nameError = ref("");
  const editing = ref(false);
  const fieldErrors = ref<RequiredProfileErrors>({});
  const latestDirection = ref<Direction | "">("");
  const directionLoadError = ref("");
  const directionError = computed(() => directionLoadError.value || (
    latestDirection.value && form.direction !== latestDirection.value
      ? "个人资料方向与最新报名方向不一致，请联系管理员处理后再保存。"
      : ""
  ));
  /** 服务端资料缺少任一必填字段时显示，保存成功后重算。 */
  const needsCompletion = ref(false);

  const form = reactive<FormShape>({
    name: "",
    class: "",
    studentId: "",
    qq: "",
    direction: "",
  });

  function hydrate(next: MyProfile, publish: (profile: MyProfile) => void = setCurrentProfile): void {
    profile.value = next;
    publish(next);
    const nextForm = nextFormFromProfile(next);
    form.name = nextForm.name;
    nameError.value = "";
    form.class = nextForm.class;
    form.studentId = nextForm.studentId;
    form.qq = nextForm.qq;
    form.direction = nextForm.direction;
    if (!form.direction && latestDirection.value) form.direction = latestDirection.value;
    needsCompletion.value = missingRequiredProfile(next.detail).length > 0;
  }

  function reset(): void {
    if (saving.value || !profile.value) return;
    hydrate(profile.value, () => {});
    fieldErrors.value = {};
    saveError.value = "";
  }

  function edit(): void {
    if (loading.value || !profile.value || saving.value) return;
    reset();
    editing.value = true;
  }

  function cancel(): void {
    if (saving.value) return;
    reset();
    editing.value = false;
  }

  async function checkDirection(): Promise<void> {
    directionLoadError.value = "";
    try {
      // API 明确按 id DESC 返回：第一条为最新报名，不使用首页的临时点选状态。
      const { items } = await deps.fetchMyApplications();
      const code = items[0]?.direction_code ?? "";
      if (!["", "front-end", "back-end", "agent", "all", "game", "other"].includes(code)) {
        throw new Error("未知报名方向");
      }
      latestDirection.value = code as Direction | "";
    } catch {
      directionLoadError.value = "暂时无法核对最新报名方向，请重试；核对成功前无法保存。";
    }
  }

  async function retryDirection(): Promise<void> {
    if (saving.value) return;
    saving.value = true;
    try {
      await checkDirection();
      if (!form.direction && !directionLoadError.value) form.direction = latestDirection.value;
    } finally {
      saving.value = false;
    }
  }

  async function load(): Promise<void> {
    const publish = beginProfileLoad();
    loading.value = true;
    loadError.value = "";
    loadUnauthorized.value = false;
    editing.value = false;
    fieldErrors.value = {};
    saveError.value = "";
    nameError.value = "";
    latestDirection.value = "";
    try {
      const [next] = await Promise.all([deps.fetchMyProfile(), checkDirection()]);
      hydrate(next, publish);
    } catch (error: unknown) {
      const info = describeAuthError(error, "暂时无法加载个人资料，请稍后重试");
      loadError.value = info.message;
      loadUnauthorized.value = info.unauthorized;
      profile.value = null;
    } finally {
      loading.value = false;
    }
  }

  async function save(): Promise<boolean> {
    if (saving.value || !editing.value || !profile.value) {
      return false;
    }
    saveError.value = "";
    fieldErrors.value = validateRequiredProfile(form);
    const nameLength = Array.from((form.name ?? "").trim()).length;
    nameError.value = nameLength !== 0 && (nameLength < 3 || nameLength > 20)
      ? "姓名需为 3–20 个字符，也可留空清除"
      : "";
    if (nameError.value || Object.keys(fieldErrors.value).length) return false;
    saving.value = true;
    const publish = beginProfileSave();
    const sessionIsCurrent = beginProfileSessionCheck();
    try {
      // 保存前再次获取，防止另一标签页新报名后继续使用旧方向。
      await checkDirection();
      if (!sessionIsCurrent()) return false;
      if (directionError.value) return false;
      const next = await deps.updateMyProfile(formToUpdate(form), profile.value.user_id);
      hydrate(next, publish);
      editing.value = false;
      ElMessage.success("资料已保存");
      return true;
    } catch (error: unknown) {
      saveError.value = describeAuthError(
        error,
        "暂时无法保存个人资料，请稍后重试",
      ).message;
      ElMessage.error(saveError.value);
      return false;
    } finally {
      saving.value = false;
    }
  }

  return {
    profile,
    form,
    loading,
    loadError,
    loadUnauthorized,
    saving,
    saveError,
    nameError,
    editing,
    fieldErrors,
    latestDirection,
    directionError,
    edit,
    reset,
    cancel,
    retryDirection,
    needsCompletion,
    load,
    save,
  };
}
