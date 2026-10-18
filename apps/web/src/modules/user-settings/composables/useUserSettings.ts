/**
 * 「个人设置」页的编排：拉取 / 保存用户资料（issue #149 字段级直接编辑）。
 *
 * 与旧版整页编辑模式（#101）的区别：
 * - 没有「更改/保存/取消」整页模式；字段常驻 input。
 * - 字段失焦后自动保存，每字段独立状态：idle / saving / saved / error。
 * - 后端 PUT /me 本就是部分更新，字段级保存只带改动字段，互不影响。
 * - 每个字段一个 epoch 计数器：保存请求在途时用户又改了值，响应回来直接丢弃，
 *   防止旧请求覆盖新输入（#149 验收标准）。
 *
 * 契约来自已合并的 #57 / #86：name 可改，邮箱只读，身份取自令牌。
 * 依赖通过参数注入，测试里塞假实现即可，不起网络。
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
import { readAuthSession } from "@/auth/session";
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

/** 可自动保存的资料字段。方向不在此列（普通成员只读；管理员下拉走单独入口）。 */
export type FieldKey = "name" | "class" | "studentId" | "qq";

export type FieldStatus = "idle" | "saving" | "saved" | "error";

export interface FieldErrors extends RequiredProfileErrors {
  name?: string;
  /** QQ 无本地格式校验，但保存失败时会在这里带错误文案（可重试）。 */
  qq?: string;
}

export interface UseUserSettings {
  profile: Ref<MyProfile | null>;
  form: FormShape;
  loading: Ref<boolean>;
  loadError: Ref<string>;
  loadUnauthorized: Ref<boolean>;
  /** 每字段保存状态。 */
  fieldStatuses: Record<FieldKey, FieldStatus>;
  /** 每字段校验错误（本地校验不过，未发请求）。 */
  fieldErrors: Ref<FieldErrors>;
  latestDirection: Ref<Direction | "">;
  directionError: Readonly<Ref<string>>;
  /** 服务端资料缺少任一必填字段时显示补全横幅。 */
  needsCompletion: Ref<boolean>;
  load: () => Promise<void>;
  /** 字段失焦触发：防抖由调用方（组件 blur → setTimeout）或本函数内部完成。 */
  saveField: (key: FieldKey) => Promise<void>;
  /** 失败后手动重试某字段。 */
  retryField: (key: FieldKey) => Promise<void>;
  retryDirection: () => Promise<void>;
  /** 管理员方向下拉变更后保存。 */
  saveDirection: () => Promise<void>;
  /** SPA 离开前自动保存；失败时保留字段错误并返回 false 阻止导航。 */
  autoSave: () => Promise<boolean>;
  /** 标签页关闭/刷新时的 keepalive 兜底保存。 */
  persistOnUnload: () => void;
}

export function nextFormFromProfile(profile: MyProfile): FormShape {
  return { ...detailToForm(profile.detail), name: profile.name ?? "" };
}

const FIELD_KEYS: FieldKey[] = ["name", "class", "studentId", "qq"];

function emptyStatuses(): Record<FieldKey, FieldStatus> {
  return { name: "idle", class: "idle", studentId: "idle", qq: "idle" };
}

export function useUserSettings(deps: UserSettingsDeps = defaultDeps): UseUserSettings {
  const profile = ref<MyProfile | null>(null);
  const loading = ref(true);
  const loadError = ref("");
  const loadUnauthorized = ref(false);

  const fieldStatuses = reactive<Record<FieldKey, FieldStatus>>(emptyStatuses());
  const fieldErrors = ref<FieldErrors>({});
  const latestDirection = ref<Direction | "">("");
  const directionLoadError = ref("");
  const directionError = computed(() => directionLoadError.value || (
    latestDirection.value && form.direction !== latestDirection.value
      ? "个人资料方向与最新报名方向不一致，请联系管理员处理后再保存。"
      : ""
  ));
  const needsCompletion = ref(false);

  const form = reactive<FormShape>({
    name: "",
    class: "",
    studentId: "",
    qq: "",
    direction: "",
  });

  /**
   * 每字段一个 epoch：发起保存时快照当前 epoch；响应回来时若 epoch 已变
   * （用户又改了/又失焦触发了新一次保存），丢弃响应，避免旧数据回写新输入。
   */
  const epochs: Record<FieldKey, number> = { name: 0, class: 0, studentId: 0, qq: 0 };
  const fieldSaveQueues: Partial<Record<FieldKey, Promise<void>>> = {};
  let autoSaveInFlight: Promise<boolean> | null = null;

  function hydrate(next: MyProfile, publish: (profile: MyProfile) => void = setCurrentProfile): void {
    profile.value = next;
    publish(next);
    const nextForm = nextFormFromProfile(next);
    form.name = nextForm.name;
    form.class = nextForm.class;
    form.studentId = nextForm.studentId;
    form.qq = nextForm.qq;
    form.direction = nextForm.direction;
    if (!form.direction && latestDirection.value) form.direction = latestDirection.value;
    needsCompletion.value = missingRequiredProfile(next.detail).length > 0;
  }

  async function checkDirection(): Promise<void> {
    directionLoadError.value = "";
    try {
      // API 明确按 id DESC 返回：第一条为最新报名。
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
    await checkDirection();
    if (!form.direction && !directionLoadError.value) form.direction = latestDirection.value;
  }

  async function load(): Promise<void> {
    const publish = beginProfileLoad();
    loading.value = true;
    loadError.value = "";
    loadUnauthorized.value = false;
    Object.assign(fieldStatuses, emptyStatuses());
    fieldErrors.value = {};
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

  /**
   * 校验单个字段。返回空串 = 通过；否则为错误文案（写入 fieldErrors）。
   * 只校验该字段，历史字段不阻塞其他字段保存。
   */
  function validateField(key: FieldKey): string {
    if (key === "name") {
      const name = form.name ?? "";
      if (name === (profile.value?.name ?? "")) return "";
      const length = Array.from(name).length;
      return length >= 2 && length <= 20 && /^\p{Script=Han}+$/u.test(name)
        ? "" : "姓名须为 2–20 个中文汉字";
    }
    const required = validateRequiredProfile(form);
    if (key === "class") return required.class ?? "";
    if (key === "studentId") return required.studentId ?? "";
    return ""; // qq 选填，无格式校验
  }

  /** 构造单字段 patch（复用 formToUpdate 的空串→null、学号归一逻辑）。 */
  function fieldPatch(key: FieldKey): MyProfileUpdate {
    const full = formToUpdate(form);
    if (key === "name") return { name: form.name ?? profile.value?.name ?? "" };
    if (key === "class") return { class: full.class ?? null };
    if (key === "studentId") return { student_id: full.student_id ?? null };
    return { qq: full.qq ?? null };
  }

  /** 该字段相对已保存 baseline 是否有实质改动。 */
  function fieldDirty(key: FieldKey): boolean {
    if (!profile.value) return false;
    const baseline = nextFormFromProfile(profile.value);
    if (key === "name") return (form.name ?? "") !== baseline.name;
    if (key === "class") return form.class !== baseline.class;
    if (key === "studentId") return String(form.studentId) !== String(baseline.studentId);
    return form.qq !== baseline.qq;
  }

  /** 保存成功后只更新 baseline 的对应字段，不整页 hydrate（避免冲掉其他正在输入的字段）。
   *  通过 publish 发布：切号后 publish 是 no-op，旧会话的响应不会污染新会话。 */
  function updateBaseline(key: FieldKey, next: MyProfile, publish: (p: MyProfile) => void): void {
    const updated: MyProfile = {
      ...profile.value!,
      name: key === "name" ? next.name : profile.value!.name,
      detail: {
        ...(profile.value!.detail ?? {}),
        ...(key === "class" ? { class: next.detail?.class ?? null } : {}),
        ...(key === "studentId" ? { student_id: next.detail?.student_id ?? null } : {}),
        ...(key === "qq" ? { qq: next.detail?.qq ?? null } : {}),
      },
    };
    profile.value = updated;
    publish(updated);
  }

  async function runFieldSave(key: FieldKey): Promise<void> {
    if (!profile.value) return;

    // 方向冲突时禁止任何资料写入（#101 规则）：挂起并提示。
    if (directionError.value) {
      fieldStatuses[key] = "error";
      fieldErrors.value = { ...fieldErrors.value, [key]: directionError.value };
      return;
    }

    const error = validateField(key);
    if (error) {
      fieldStatuses[key] = "error";
      fieldErrors.value = { ...fieldErrors.value, [key]: error };
      return;
    }
    // 本地校验通过，清掉该字段的旧错误
    if (fieldErrors.value[key as keyof FieldErrors]) {
      const next = { ...fieldErrors.value };
      delete next[key as keyof FieldErrors];
      fieldErrors.value = next;
    }

    if (!fieldDirty(key)) {
      fieldStatuses[key] = "idle";
      return;
    }

    const epoch = ++epochs[key];
    const snapshot = String(form[key as keyof FormShape] ?? "");
    fieldStatuses[key] = "saving";
    const publish = beginProfileSave();
    const sessionIsCurrent = beginProfileSessionCheck();
    try {
      // 保存前再次核对方向，发现其他标签页新报名（#101）。
      await checkDirection();
      if (!sessionIsCurrent()) return;
      if (directionError.value) {
        fieldStatuses[key] = "error";
        fieldErrors.value = { ...fieldErrors.value, [key]: directionError.value };
        return;
      }
      const next = await deps.updateMyProfile(fieldPatch(key), profile.value.user_id);
      // 竞态 1：并发 saveField 时旧响应丢弃。
      if (epochs[key] !== epoch) return;
      // 竞态 2：请求在途期间用户又输入过该字段，旧响应过期，不回写 baseline。
      if (String(form[key as keyof FormShape] ?? "") !== snapshot) {
        fieldStatuses[key] = "idle";
        return;
      }
      updateBaseline(key, next, publish);
      fieldStatuses[key] = "saved";
    } catch (err: unknown) {
      if (epochs[key] !== epoch) return;
      fieldStatuses[key] = "error";
      const message = describeAuthError(err, "保存失败").message;
      fieldErrors.value = { ...fieldErrors.value, [key]: message };
    }
  }

  /** 字段失焦触发。组件侧可加防抖；这里不再重复防抖（失焦本身就是离散事件）。 */
  async function saveField(key: FieldKey): Promise<void> {
    const previous = fieldSaveQueues[key] ?? Promise.resolve();
    const current = previous.catch(() => { }).then(() => runFieldSave(key));
    const queued = current.finally(() => {
      if (fieldSaveQueues[key] === queued) delete fieldSaveQueues[key];
    });
    fieldSaveQueues[key] = queued;
    return queued;
  }

  async function retryField(key: FieldKey): Promise<void> {
    return saveField(key);
  }

  /** 管理员方向下拉变更后保存（普通成员方向只读，不走这里）。 */
  async function saveDirection(): Promise<void> {
    if (!profile.value || profile.value.role !== "admin") return;
    if (directionError.value) return;
    const publish = beginProfileSave();
    const sessionIsCurrent = beginProfileSessionCheck();
    try {
      await checkDirection();
      if (!sessionIsCurrent() || directionError.value) return;
      const next = await deps.updateMyProfile(
        { direction: form.direction === "" ? null : form.direction },
        profile.value.user_id,
      );
      const updated: MyProfile = {
        ...profile.value,
        detail: { ...(profile.value.detail ?? {}), direction: next.detail?.direction ?? null },
      };
      profile.value = updated;
      publish(updated);
      ElMessage.success("方向已保存");
    } catch (err: unknown) {
      ElMessage.error(describeAuthError(err, "方向保存失败").message);
    }
  }

  /**
   * SPA 离开时静默保存：把所有脏字段合并成一个 patch 一次发，不 toast、失败静默。
   */
  function autoSave(): Promise<boolean> {
    if (autoSaveInFlight) return autoSaveInFlight;
    const inFlight = performAutoSave().finally(() => {
      autoSaveInFlight = null;
    });
    autoSaveInFlight = inFlight;
    return inFlight;
  }

  async function performAutoSave(): Promise<boolean> {
    await Promise.all(Object.values(fieldSaveQueues));
    if (!profile.value) return true;
    const dirty = FIELD_KEYS.filter((k) => fieldDirty(k));
    if (dirty.length === 0) return true;

    const markError = (message: string): void => {
      const next = { ...fieldErrors.value };
      for (const key of dirty) {
        fieldStatuses[key] = "error";
        next[key] = message;
      }
      fieldErrors.value = next;
    };

    if (directionError.value) {
      markError(directionError.value);
      return false;
    }

    // 任一脏字段校验不过就跳过（避免一次注定 400 的写库）。
    for (const k of dirty) {
      const error = validateField(k);
      if (error) {
        fieldStatuses[k] = "error";
        fieldErrors.value = { ...fieldErrors.value, [k]: error };
        return false;
      }
    }
    const sessionIsCurrent = beginProfileSessionCheck();
    const publish = beginProfileSave();
    try {
      await checkDirection();
      if (!sessionIsCurrent()) return false;
      if (directionError.value) {
        markError(directionError.value);
        return false;
      }
      const patch = formToUpdate(form);
      const next = await deps.updateMyProfile(patch, profile.value.user_id);
      profile.value = next;
      publish(next);
      for (const key of dirty) {
        fieldStatuses[key] = "saved";
        const updatedErrors = { ...fieldErrors.value };
        delete updatedErrors[key];
        fieldErrors.value = updatedErrors;
      }
      return true;
    } catch (error: unknown) {
      markError(describeAuthError(error, "保存失败").message);
      return false;
    }
  }

  /** 标签页关闭/刷新兜底（beforeunload 无法 await，keepalive fetch）。 */
  function persistOnUnload(): void {
    if (!profile.value) return;
    const dirty = FIELD_KEYS.filter((k) => fieldDirty(k));
    if (dirty.length === 0) return;
    for (const k of dirty) {
      if (validateField(k)) return;
    }
    const session = readAuthSession();
    if (!session) return;
    void fetch("/api/v1/me", {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session.access_token}`,
      },
      body: JSON.stringify(formToUpdate(form)),
      keepalive: true,
    }).catch(() => { });
  }

  return {
    profile,
    form,
    loading,
    loadError,
    loadUnauthorized,
    fieldStatuses,
    fieldErrors,
    latestDirection,
    directionError,
    needsCompletion,
    load,
    saveField,
    retryField,
    retryDirection,
    saveDirection,
    autoSave,
    persistOnUnload,
  };
}
