/**
 * 「个人设置」页的编排：拉取 / 保存用户资料。
 *
 * 套路与 useAdminPermissions 一致：依赖通过参数注入，测试里塞假实现就能跑，
 * 不必起网络、不必起 Vue 组件。受保护请求全部走 apps/web/src/api/session-request.ts，
 * 这里不重复读 token，也不重复翻译 401/403。
 *
 * #57 后端 PUT /me 落地后，本 composable 的契约自然成立；接口形状以最终合并版为准，
 * 当前按提案实现（嵌套 detail + 禁止 user_id）。
 */
import { reactive, ref, type Ref } from "vue";
import { ElMessage } from "element-plus";

import {
  detailToForm,
  fetchMyProfile,
  formToUpdate,
  isProfileEmpty,
  updateMyProfile,
  type FormShape,
  type MyProfile,
  type MyProfileUpdate,
} from "@/api/profile";
import { describeAuthError } from "@/modules/dashboard/composables/useAdminPermissions";

export interface UserSettingsDeps {
  fetchMyProfile: () => Promise<MyProfile>;
  updateMyProfile: (input: MyProfileUpdate) => Promise<MyProfile>;
}

const defaultDeps: UserSettingsDeps = {
  fetchMyProfile: () => fetchMyProfile(),
  updateMyProfile: (input) => updateMyProfile(input),
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
  return detailToForm(profile.detail);
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
  /** 「建议补全」提示：进入页面时表单全空则展示，不阻塞保存 */
  const needsCompletion = ref(false);

  const form = reactive<FormShape>({
    class: "",
    studentId: "",
    qq: "",
    direction: "",
  });

  function hydrate(next: MyProfile): void {
    profile.value = next;
    const nextForm = nextFormFromProfile(next);
    form.class = nextForm.class;
    form.studentId = nextForm.studentId;
    form.qq = nextForm.qq;
    form.direction = nextForm.direction;
    needsCompletion.value = isProfileEmpty(nextForm);
  }

  async function load(): Promise<void> {
    loading.value = true;
    loadError.value = "";
    loadUnauthorized.value = false;
    try {
      hydrate(await deps.fetchMyProfile());
    } catch (error: unknown) {
      const info = describeAuthError(error);
      loadError.value = info.message;
      loadUnauthorized.value = info.unauthorized;
      profile.value = null;
    } finally {
      loading.value = false;
    }
  }

  async function save(): Promise<boolean> {
    if (saving.value) {
      return false;
    }
    saving.value = true;
    saveError.value = "";
    try {
      const next = await deps.updateMyProfile(formToUpdate(form));
      hydrate(next);
      ElMessage.success("资料已保存");
      return true;
    } catch (error: unknown) {
      saveError.value = describeAuthError(error).message;
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
    needsCompletion,
    load,
    save,
  };
}