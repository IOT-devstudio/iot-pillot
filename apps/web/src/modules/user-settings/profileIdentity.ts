import { shallowReadonly, shallowRef } from "vue";
import type { MyProfile } from "@/api/profile";

/** 仅持有服务端确认过的资料；供常驻菜单与当前页面同步展示。 */
const profileState = shallowRef<MyProfile | null>(null);
export const currentProfile = shallowReadonly(profileState);
let revision = 0;
let sessionRevision = 0;

export function setCurrentProfile(profile: MyProfile | null): void {
  revision += 1;
  if (profile === null) sessionRevision += 1;
  profileState.value = profile;
}

/** 保存可覆盖同会话内的读取，但不能在退出会话后再发布。 */
export function beginProfileSave(): (profile: MyProfile) => void {
  const startedSession = sessionRevision;
  return (profile) => {
    if (startedSession === sessionRevision) setCurrentProfile(profile);
  };
}

/** 保存或清除会话后，先前发出的读取结果不再有权发布。 */
export function beginProfileLoad(): (profile: MyProfile | null) => void {
  const startedAt = revision;
  return (profile) => {
    if (startedAt !== revision) return;
    // 一次读取失败不能使并行的成功读取失效。
    if (profile === null) profileState.value = null;
    else setCurrentProfile(profile);
  };
}

export function profileDisplayName(profile: MyProfile | null): string {
  if (!profile) return "";
  // 明确清空姓名时不回退到 JWT 的旧 username；兼容旧接口缺少 name 的情况。
  return profile.name === undefined
    ? profile.username
    : profile.name || profile.detail?.email || `用户 ${profile.user_id}`;
}
