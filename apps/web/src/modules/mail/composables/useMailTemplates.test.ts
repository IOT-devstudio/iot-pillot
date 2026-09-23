// ElMessageBox.confirm 会弹真实对话框，必须打桩；ElMessage 同理只是通知。
vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
}));

import { describe, expect, it, beforeEach, vi } from "vitest";
import { ElMessage, ElMessageBox } from "element-plus";

import { AuthRequestError } from "@/api/auth";
import type { MailTemplate, MailTemplateInput } from "@/api/mail";
import { useMailTemplates } from "./useMailTemplates";

const CONFIRM = vi.mocked(ElMessageBox.confirm);
const SUCCESS = vi.mocked(ElMessage.success);
const ERROR = vi.mocked(ElMessage.error);

const template: MailTemplate = {
  id: 1,
  name: "面试邀请",
  type: "invitation",
  title: "{{name}} 面试邀请",
  mail_example: "",
  mail_model: "Hi {{name}}",
  variables: ["name"],
};

const input: MailTemplateInput = {
  name: "面试邀请",
  type: "invitation",
  title: "{{name}} 面试邀请",
  mail_example: "",
  mail_model: "Hi {{name}}",
};

beforeEach(() => {
  vi.clearAllMocks();
  CONFIRM.mockResolvedValue("confirm" as never);
});

describe("useMailTemplates", () => {
  it("loads templates into state", async () => {
    const { templates, loading, error, load } = useMailTemplates({
      list: async () => ({ items: [template], total: 1 }),
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
    });

    await load();

    expect(loading.value).toBe(false);
    expect(error.value).toBe("");
    expect(templates.value).toEqual([template]);
  });

  it("maps a load failure to a readable message without a retry flag flip", async () => {
    const { error, unauthorized, load } = useMailTemplates({
      list: async () => {
        throw new AuthRequestError("登录状态已失效，请重新登录", 401);
      },
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
    });

    await load();

    expect(error.value).toBe("登录状态已失效，请重新登录");
    expect(unauthorized.value).toBe(true);
  });

  it("creates on save when there is no editing id and reloads", async () => {
    const create = vi.fn().mockResolvedValue(template);
    const list = vi.fn().mockResolvedValue({ items: [template], total: 1 });
    const { save } = useMailTemplates({
      list,
      create,
      update: vi.fn(),
      remove: vi.fn(),
    });

    await expect(save(input, null)).resolves.toBe(true);

    expect(create).toHaveBeenCalledWith(input);
    expect(list).toHaveBeenCalled();
    expect(SUCCESS).toHaveBeenCalledWith("模板已创建");
  });

  it("updates on save when editing an existing template", async () => {
    const update = vi.fn().mockResolvedValue(template);
    const { save } = useMailTemplates({
      list: vi.fn().mockResolvedValue({ items: [template], total: 1 }),
      create: vi.fn(),
      update,
      remove: vi.fn(),
    });

    await expect(save(input, 1)).resolves.toBe(true);
    expect(update).toHaveBeenCalledWith(1, input);
    expect(SUCCESS).toHaveBeenCalledWith("模板已更新");
  });

  it("surfaces the backend duplicate-type message on a failed save", async () => {
    const { save } = useMailTemplates({
      list: vi.fn(),
      create: vi.fn().mockRejectedValue(
        new AuthRequestError("模板名或模板类型已存在", 400),
      ),
      update: vi.fn(),
      remove: vi.fn(),
    });

    await expect(save(input, null)).resolves.toBe(false);
    expect(ERROR).toHaveBeenCalledWith("模板名或模板类型已存在");
  });

  it("does not delete when the user cancels the confirm", async () => {
    CONFIRM.mockRejectedValue("cancel" as never);
    const remove = vi.fn();
    const { remove: del } = useMailTemplates({
      list: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      remove,
    });

    await expect(del(template)).resolves.toBe(false);
    expect(remove).not.toHaveBeenCalled();
  });

  it("deletes after confirmation and reloads the list", async () => {
    const remove = vi.fn().mockResolvedValue(undefined);
    const list = vi.fn().mockResolvedValue({ items: [], total: 0 });
    const { remove: del, templates } = useMailTemplates({
      list,
      create: vi.fn(),
      update: vi.fn(),
      remove,
    });

    await expect(del(template)).resolves.toBe(true);
    expect(remove).toHaveBeenCalledWith(1);
    expect(list).toHaveBeenCalled();
    expect(templates.value).toEqual([]);
    expect(SUCCESS).toHaveBeenCalledWith("模板已删除");
  });
});
