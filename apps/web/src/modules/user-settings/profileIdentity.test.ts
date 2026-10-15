import { afterEach, describe, expect, it } from "vitest";
import { clearAuthSession } from "@/auth/session";
import type { MyProfile } from "@/api/profile";
import { beginProfileLoad, currentProfile, setCurrentProfile } from "./profileIdentity";

const profile = (name: string): MyProfile => ({ user_id: 1, username: "旧姓名", name, role: "member" });
afterEach(() => setCurrentProfile(null));

describe("profile identity request ordering", () => {
  it("does not overwrite a successful save with an earlier GET", () => {
    const receiveOldGet = beginProfileLoad();
    setCurrentProfile(profile("保存后的姓名"));
    receiveOldGet(profile("保存前的姓名"));
    expect(currentProfile.value?.name).toBe("保存后的姓名");
  });

  it("invalidates pending profile requests when the auth session is cleared", () => {
    setCurrentProfile(profile("旧账号姓名"));
    const receiveOldGet = beginProfileLoad();
    clearAuthSession({ length: 0, clear() {}, getItem() { return null; }, key() { return null; }, setItem() {}, removeItem() {} });
    receiveOldGet(profile("旧账号姓名"));
    expect(currentProfile.value).toBeNull();
  });
});
