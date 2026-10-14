import { describe, expect, it } from "vitest";
import {
  toChangePasswordPayload,
  toLoginPayload,
  toRegisterPayload,
  toResetPayload,
  validateChangePasswordForm,
  validateLoginForm,
  validateRegisterForm,
  validateResetForm,
} from "./form";

describe("validateLoginForm", () => {
  it("rejects missing credentials and passwords shorter than six characters", () => {
    expect(validateLoginForm({ username: "", password: "123" })).toEqual({
      username: "请输入用户名",
      password: "密码至少 6 位",
    });
  });
});

describe("validateRegisterForm", () => {
  it("checks backend length rules, email format, code and confirmation", () => {
    expect(
      validateRegisterForm({
        name: "ab",
        email: "wrong",
        password: "123456",
        confirmPassword: "654321",
        code: "12345",
      }),
    ).toEqual({
      name: "姓名长度为 3-20 个字符",
      email: "请输入正确的邮箱地址",
      confirmPassword: "两次输入的密码不一致",
      code: "请输入 6 位验证码",
    });
  });
});

it("converts registration form without sending confirmPassword", () => {
  expect(
    toRegisterPayload({
      name: "  Ada  ",
      email: "  ada@example.com ",
      password: "123456",
      confirmPassword: "123456",
      code: "123456",
    }),
  ).toEqual({
    name: "Ada",
    email: "ada@example.com",
    password: "123456",
    code: "123456",
  });
});

it("converts login form to the exact backend DTO", () => {
  expect(toLoginPayload({ username: "ada", password: "123456" })).toEqual({
    username: "ada",
    password: "123456",
  });
});

describe("validateResetForm", () => {
  it("checks email format, password bounds, confirmation and the 6-digit code", () => {
    expect(
      validateResetForm({
        email: "wrong",
        password: "123",
        confirmPassword: "321",
        code: "12a",
      }),
    ).toEqual({
      email: "请输入正确的邮箱地址",
      password: "密码至少 6 位",
      confirmPassword: "两次输入的密码不一致",
      code: "请输入 6 位验证码",
    });
  });

  it("passes a complete form and maps it to the reset DTO", () => {
    const form = {
      email: "  ada@example.com ",
      password: "secret1",
      confirmPassword: "secret1",
      code: " 123456 ",
    };
    expect(validateResetForm(form)).toEqual({});
    expect(toResetPayload(form)).toEqual({
      email: "ada@example.com",
      code: "123456",
      new_password: "secret1",
    });
  });
});

describe("validateChangePasswordForm", () => {
  it("requires the current password", () => {
    expect(
      validateChangePasswordForm({
        oldPassword: "",
        newPassword: "new-secret",
        confirmPassword: "new-secret",
      }),
    ).toEqual({ oldPassword: "请输入当前密码" });
  });

  it("rejects reusing the current password", () => {
    expect(
      validateChangePasswordForm({
        oldPassword: "same12",
        newPassword: "same12",
        confirmPassword: "same12",
      }),
    ).toEqual({ newPassword: "新密码不能与当前密码相同" });
  });

  it("checks confirmation against the new password and maps the payload", () => {
    const form = {
      oldPassword: "old-secret",
      newPassword: "new-secret",
      confirmPassword: "mismatch",
    };
    expect(validateChangePasswordForm(form)).toEqual({
      confirmPassword: "两次输入的密码不一致",
    });
    expect(
      toChangePasswordPayload({
        oldPassword: "old-secret",
        newPassword: "new-secret",
        confirmPassword: "new-secret",
      }),
    ).toEqual({ old_password: "old-secret", new_password: "new-secret" });
  });
});
