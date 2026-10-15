import { ElMessage } from "element-plus";
import { fetchMyProfile } from "@/api/profile";
import { beginProfileLoad, setCurrentProfile } from "@/modules/user-settings/profileIdentity";

/** 两个认证入口共用；每次登录重新检查，令牌刷新不触发。 */
export async function checkProfileAfterLogin(): Promise<void> {
  setCurrentProfile(null);
  const publish = beginProfileLoad();
  try {
    publish(await fetchMyProfile());
  } catch {
    ElMessage.warning("登录成功，但暂时无法检查资料完整性，请到个人设置重试。");
  }
}
