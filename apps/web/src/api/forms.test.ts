import { afterEach, describe, expect, it, vi } from "vitest";
import { createForm, updateForm, fetchShareForm, submitForm, listForms, listFormSubmissions, fetchFormSubmission } from "./forms";

vi.mock("@/auth/session", async importOriginal => ({
  ...await importOriginal<typeof import("@/auth/session")>(),
  readAuthSession: () => ({ user_id: 7, access_token: "access", refresh_token: "refresh" }),
  clearAuthSession: vi.fn(), saveAuthSession: vi.fn(),
}));
afterEach(() => vi.restoreAllMocks());
function network() {
  return vi.spyOn(globalThis, "fetch").mockImplementation(async () => new Response(JSON.stringify({ code: 0, data: { items: [] } })));
}
describe("forms API", () => {
  it("uses authenticated share endpoints and sends only values owned by the loaded account", async () => {
    const fetch = network();
    await fetchShareForm("a/b");
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/v1/forms/public/a%2Fb");
    await submitForm("token", { "1": "邮箱答案", "5": ["确认"] }, 7);
    const [path, init] = fetch.mock.calls[1]!;
    expect(path).toBe("/api/v1/forms/public/token/submit");
    expect(new Headers(init?.headers).get("Authorization")).toBe("Bearer access");
    expect(JSON.parse(String(init?.body))).toEqual({ values: { "1": "邮箱答案", "5": ["确认"] } });
    await expect(submitForm("token", {}, 8)).rejects.toThrow("账号已变更");
    expect(fetch).toHaveBeenCalledTimes(2);
  });
  it("preserves existing field IDs on PATCH and separates create bindings from updates", async () => {
    const fetch = network();
    const fields = [{ id: 11, type: "text" as const, label: "题目", required: false, placeholder: "" }];
    await createForm({ title: "报名", description: "", round_id: 2, direction_code: "back-end", fields });
    await updateForm(3, { title: "新标题", fields });
    expect(JSON.parse(String(fetch.mock.calls[0]?.[1]?.body))).toMatchObject({ round_id: 2, direction_code: "back-end" });
    expect(fetch.mock.calls[1]?.[0]).toBe("/api/v1/forms/3");
    expect(fetch.mock.calls[1]?.[1]?.method).toBe("PATCH");
    expect(JSON.parse(String(fetch.mock.calls[1]?.[1]?.body))).toEqual({ title: "新标题", fields });
  });
  it("paginates forms and real submissions and loads a submission detail", async () => {
    const fetch = network();
    await listForms(2); await listFormSubmissions(3, 4); await fetchFormSubmission(3, 8);
    expect(fetch.mock.calls.map(c => c[0])).toEqual(["/api/v1/forms?page=2&page_size=20", "/api/v1/forms/3/submissions?page=4&page_size=20", "/api/v1/forms/3/submissions/8"]);
  });
});
