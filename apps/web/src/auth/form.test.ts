import { describe, expect, it } from "vitest";
import {
  toLoginPayload,
  toRegisterPayload,
  validateLoginForm,
  validateRegisterForm,
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
