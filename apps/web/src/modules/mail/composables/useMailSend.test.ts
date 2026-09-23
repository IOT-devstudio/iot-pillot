vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

vi.mock("@/api/mail", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/mail")>();
  return {
    ...actual,
    sendMailToUser: vi.fn(),
    sendMailToEmail: vi.fn(),
    sendMailBulk: vi.fn(),
  };
});

import { beforeEach, describe, expect, it, vi } from "vitest";
import { ElMessage } from "element-plus";

import {
  MAIL_BULK_LIMIT,
  sendMailBulk,
  sendMailToEmail,
  sendMailToUser,
} from "@/api/mail";
import { AuthRequestError } from "@/api/auth";
import { useMailSend } from "./useMailSend";

const toUser = vi.mocked(sendMailToUser);
const toEmail = vi.mocked(sendMailToEmail);
const bulk = vi.mocked(sendMailBulk);
const SUCCESS = vi.mocked(ElMessage.success);
const WARNING = vi.mocked(ElMessage.warning);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useMailSend", () => {
  it("blocks sending until a user is selected", async () => {
    const { canSend, send } = useMailSend();
    expect(canSend.value).toBe(false);
    await send(1);
    expect(toUser).not.toHaveBeenCalled();
  });

  it("sends to the selected user and drops blank vars", async () => {
    toUser.mockResolvedValue({
      to_user_id: 2,
      to_email: "a@b.c",
      title: "Hi 张三",
    });
    const { target, vars, send } = useMailSend();
    target.value = { to_user_id: 2 };
    vars.value = { name: "张三", class: "   " };

    await send(7);

    expect(toUser).toHaveBeenCalledWith({
      template_id: 7,
      to_user_id: 2,
      vars: { name: "张三" }, // 空白值不发送，交给服务端资料预填
    });
    expect(SUCCESS).toHaveBeenCalledWith("已发送至 a@b.c");
  });

  it("sends to a raw email in email mode", async () => {
    toEmail.mockResolvedValue({
      to_user_id: 0,
      to_email: "x@y.z",
      title: "t",
    });
    const { mode, target, send } = useMailSend();
    mode.value = "email";
    target.value = { to_user_id: 0, email: "  x@y.z " };

    await send(3);

    expect(toEmail).toHaveBeenCalledWith({
      template_id: 3,
      email: "x@y.z",
      vars: {},
    });
  });

  it("rejects an email-mode send when the address is blank", async () => {
    const { mode, target, canSend, send } = useMailSend();
    mode.value = "email";
    target.value = { to_user_id: 0, email: "   " };

    expect(canSend.value).toBe(false);
    await send(3);
    expect(toEmail).not.toHaveBeenCalled();
  });

  it("blocks bulk sends over the 200-recipient limit", async () => {
    const { mode, bulkUserIds, bulkOverLimit, canSend, send } = useMailSend();
    mode.value = "bulk";
    bulkUserIds.value = Array.from(
      { length: MAIL_BULK_LIMIT + 1 },
      (_, i) => i + 1,
    );

    expect(bulkOverLimit.value).toBe(true);
    expect(canSend.value).toBe(false);
    await send(1);
    expect(bulk).not.toHaveBeenCalled();
  });

  it("bulk-sends and warns when some recipients fail", async () => {
    bulk.mockResolvedValue({
      sent: [{ to_user_id: 1, to_email: "ok@x.y", title: "t" }],
      failed: [
        { to_user_id: 2, to_email: "bad@x.y", reason: "SMTP 投递失败" },
      ],
      total: 2,
    });
    const { mode, bulkUserIds, send, bulkResult } = useMailSend();
    mode.value = "bulk";
    bulkUserIds.value = [1, 2];

    await send(9);

    expect(bulk).toHaveBeenCalledWith({
      template_id: 9,
      recipients: [{ to_user_id: 1, vars: {} }, { to_user_id: 2, vars: {} }],
    });
    expect(bulkResult.value?.failed).toHaveLength(1);
    expect(WARNING).toHaveBeenCalled();
  });

  it("passes the backend missing-variable message through to the user", async () => {
    toUser.mockRejectedValue(
      new AuthRequestError("渲染正文失败：模板缺少变量：class", 400),
    );
    const { target, error, send } = useMailSend();
    target.value = { to_user_id: 2 };

    await send(1);

    expect(error.value).toBe("渲染正文失败：模板缺少变量：class");
  });

  it("reset clears mode, targets, vars and results", async () => {
    const { mode, target, bulkUserIds, vars, lastSend, reset } = useMailSend();
    mode.value = "bulk";
    target.value = { to_user_id: 5, email: "a@b.c" };
    bulkUserIds.value = [1, 2];
    vars.value = { name: "x" };
    lastSend.value = { to_user_id: 1, to_email: "a", title: "t" };

    reset();

    expect(mode.value).toBe("user");
    expect(target.value).toEqual({ to_user_id: 0 });
    expect(bulkUserIds.value).toEqual([]);
    expect(vars.value).toEqual({});
    expect(lastSend.value).toBeNull();
  });
});
