import { shallowRef } from "vue";
import type { MyProfile } from "@/api/profile";

/** 仅持有服务端确认过的资料；供常驻菜单与当前页面同步展示。 */
export const currentProfile = shallowRef<MyProfile | null>(null);

export function profileDisplayName(profile: MyProfile | null): string {
  if (!profile) return "";
  // 明确清空姓名时不回退到 JWT 的旧 username；兼容旧接口缺少 name 的情况。
  return profile.name === undefined
    ? profile.username
    : profile.name || profile.detail?.email || `用户 ${profile.user_id}`;
}
