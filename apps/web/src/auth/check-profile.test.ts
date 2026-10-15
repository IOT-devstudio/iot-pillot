import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchMyProfile } from "@/api/profile";
import { ElMessage } from "element-plus";
import { currentProfile, setCurrentProfile } from "@/modules/user-settings/profileIdentity";
import { checkProfileAfterLogin } from "./check-profile";

vi.mock("@/api/profile", () => ({ fetchMyProfile: vi.fn() }));
vi.mock("element-plus", () => ({ ElMessage: { warning: vi.fn() } }));
afterEach(() => { vi.clearAllMocks(); setCurrentProfile(null); });

describe("profile check after login", () => {
  it("replaces the prior account and fetches fresh completeness on every login", async () => {
    const next = { user_id: 2, username: "new", role: "member", detail: { class: "iot", student_id: 123 } };
    setCurrentProfile({ user_id: 1, username: "old", role: "member" });
    vi.mocked(fetchMyProfile).mockResolvedValue(next);
    await checkProfileAfterLogin();
    expect(currentProfile.value).toEqual(next);
    await checkProfileAfterLogin();
    expect(fetchMyProfile).toHaveBeenCalledTimes(2);
  });

  it("does not mistake a failed fetch for missing fields or fail authentication", async () => {
    vi.mocked(fetchMyProfile).mockRejectedValueOnce(new Error("offline"));
    await expect(checkProfileAfterLogin()).resolves.toBeUndefined();
    expect(currentProfile.value).toBeNull();
    expect(ElMessage.warning).toHaveBeenCalledWith(expect.stringContaining("无法检查"));
  });

  it("does not publish a delayed response after logout", async () => {
    let resolve!: (value: Awaited<ReturnType<typeof fetchMyProfile>>) => void;
    vi.mocked(fetchMyProfile).mockImplementationOnce(() => new Promise(r => { resolve = r; }));
    const pending = checkProfileAfterLogin();
    setCurrentProfile(null);
    resolve({ user_id: 1, username: "old", role: "member" });
    await pending;
    expect(currentProfile.value).toBeNull();
  });
});
