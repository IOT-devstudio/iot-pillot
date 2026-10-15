import { describe, expect, it, vi } from "vitest";
import { useFormFilling, type FillingDeps } from "./useFormFilling";
import type { ShareForm, SubmitResult } from "@/api/forms";

const definition = (submitted = false): ShareForm => ({
  form: { id: 1, title: "报名", description: "", round_id: 1, direction_code: "back-end", share_token: "token", is_open: true, submission_count: 0, created_at: "", updated_at: "" },
  fields: [{ id: 2, type: "text", label: "项目", required: true, placeholder: "", sort_order: 0 }], submitted, my_submission_id: 3,
});
const result: SubmitResult = { submission: { id: 3, form_id: 1, submitter_id: 7, application_id: 8, values: { "2": "项目" }, submitted_at: "2026-10-15T10:00:00Z" }, application_id: 8, application_status: "applied" };
function deps(overrides: Partial<FillingDeps> = {}): FillingDeps {
  return { read: async () => definition(), profile: async () => ({ user_id: 7, username: "user", role: "member" }), submit: async () => result, ...overrides };
}
describe("form filling", () => {
  it("validates before sending, prevents rapid duplicate submissions and remembers success", async () => {
    let finish!: (value: SubmitResult) => void;
    const submit = vi.fn(() => new Promise<SubmitResult>(resolve => { finish = resolve; }));
    const state = useFormFilling(deps({ submit }));
    await state.load("token");
    expect(await state.send()).toBe(false);
    expect(submit).not.toHaveBeenCalled();
    state.values.value["2"] = "项目";
    const pending = state.send();
    expect(await state.send()).toBe(false);
    finish(result); await pending;
    expect(submit).toHaveBeenCalledWith("token", { "2": "项目" }, 7);
    expect(state.done.value).toBe(true);
    expect(await state.send()).toBe(false);
    expect(submit).toHaveBeenCalledTimes(1);
  });
  it("preserves input and does not auto retry on failed submission", async () => {
    const submit = vi.fn(async () => { throw new Error("offline"); });
    const state = useFormFilling(deps({ submit }));
    await state.load("token"); state.values.value["2"] = "保留项目";
    expect(await state.send()).toBe(false);
    expect(state.values.value["2"]).toBe("保留项目");
    expect(state.submitError.value).not.toBe("");
    expect(submit).toHaveBeenCalledTimes(1);
  });
  it("blocks closed and previously submitted forms", async () => {
    const submit = vi.fn();
    const state = useFormFilling(deps({ submit, read: async () => definition(true) }));
    await state.load("token"); expect(state.done.value).toBe(true);
    expect(await state.send()).toBe(false);
    const closed = definition(); closed.form.is_open = false;
    const other = useFormFilling(deps({ submit, read: async () => closed }));
    await other.load("closed"); expect(await other.send()).toBe(false);
    expect(submit).not.toHaveBeenCalled();
  });
  it("ignores a stale response when navigating between share links", async () => {
    let finish!: (value: ShareForm) => void;
    const state = useFormFilling(deps({ read: token => token === "old" ? new Promise(resolve => { finish = resolve; }) : Promise.resolve({ ...definition(), form: { ...definition().form, title: "新表单" } }) }));
    const old = state.load("old"); await state.load("new"); finish(definition()); await old;
    expect(state.data.value?.form.title).toBe("新表单");
  });
});
