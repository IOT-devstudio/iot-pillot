/**
 * 报名入口的资料状态（issue #152）。
 *
 * 报名弹窗、首页统计区资料卡、收尾区三处入口共用同一个判定结果，
 * 避免同页出现「查看资料 / 完善资料」两种说法。
 *
 * 判定只有一套来源：`missingRequiredProfile`（班级 + 学号）。本文件只负责
 * 把它翻译成入口状态，不新增必填字段，也不读组件内部状态——传入的
 * `profile` 是 `profileIdentity` 里服务端确认过的共享资料。
 */
import type { MyProfile } from "@/api/profile";
import { missingRequiredProfile } from "@/modules/user-settings/profileCompletion";

export type ProfileEntryState =
  /** 资料完整：入口是「查看资料」，报名放行 */
  | "ready"
  /** 明确缺字段：入口是「完善资料」，报名前先跳设置页 */
  | "needs-completion"
  /** 资料未知（未登录、加载中、读取失败）：中性文案，报名放行 */
  | "unknown"
  /** 创始管理员没有个人设置页：不渲染入口 */
  | "hidden";

export interface ProfileEntry {
  state: ProfileEntryState;
  /** 缺失的必填项，仅 `needs-completion` 非空 */
  missing: string[];
  /** 入口文案；`hidden` 为空串，调用方不渲染 */
  label: string;
  /** 是否拦截报名动作（只有明确缺字段才拦） */
  blocksApplication: boolean;
}

const LABELS: Record<Exclude<ProfileEntryState, "hidden">, string> = {
  ready: "查看资料",
  "needs-completion": "完善资料",
  unknown: "个人资料",
};

/**
 * 四态判定。顺序有讲究：
 *
 *   1. `null` 先判——加载中/读取失败时不能当成「资料完整」，也不能当成
 *      「缺字段」去拦报名，只能退回中性态（fail-open）。
 *   2. 创始管理员次之——它没有设置页，缺字段的提示对它无意义。
 *   3. 最后才看必填项。
 *
 * 只有「读到资料且确实缺字段」才拦；接口抖动不该让报名按钮变成死路。
 */
export function profileEntryState(profile: MyProfile | null): ProfileEntry {
  if (profile === null) {
    return {
      state: "unknown",
      missing: [],
      label: LABELS.unknown,
      blocksApplication: false,
    };
  }
  if (profile.is_founding === true) {
    return { state: "hidden", missing: [], label: "", blocksApplication: false };
  }
  const missing = missingRequiredProfile(profile.detail);
  if (missing.length === 0) {
    return {
      state: "ready",
      missing,
      label: LABELS.ready,
      blocksApplication: false,
    };
  }
  return {
    state: "needs-completion",
    missing,
    label: LABELS["needs-completion"],
    blocksApplication: true,
  };
}
