import { ElMessage } from "element-plus";
import { fetchMyProfile, type MyProfile } from "@/api/profile";
import { beginProfileLoad, currentProfile, setCurrentProfile } from "@/modules/user-settings/profileIdentity";

/** 两个认证入口共用；每次登录重新检查，令牌刷新不触发。 */
export async function checkProfileAfterLogin(): Promise<MyProfile | undefined> {
  setCurrentProfile(null);
  const publish = beginProfileLoad();
  try {
    const profile = await fetchMyProfile();
    publish(profile);
    // 被退出或新会话丢弃的结果也不能用于决定登录落点。
    return currentProfile.value === profile ? profile : undefined;
  } catch {
    ElMessage.warning("登录成功，但暂时无法检查资料完整性，请到个人设置重试。");
  }
}
