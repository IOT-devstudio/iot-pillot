import { describe, expect, it } from "vitest";

import { detailToForm, formToUpdate, isProfileEmpty } from "./profile";

// #57 后端落地前 GET /me 不返回 detail：detailToForm 必须把 undefined
// 当空资料渲染。此前该路径抛 TypeError，被 describeAuthError 兜底成
// 「暂时无法加载管理数据」——这里锁住真实崩溃路径。
describe("detailToForm", () => {
  it("maps a full detail into the form shape", () => {
    expect(
      detailToForm({
        class: "iot 2301",
        student_id: 2023114514,
        qq: "1044696157",
        direction: "front-end",
        email: "ada@example.edu.cn",
      }),
    ).toEqual({
      class: "iot 2301",
      studentId: "2023114514",
      qq: "1044696157",
      direction: "front-end",
    });
  });

  it("treats undefined detail as an empty form instead of throwing", () => {
    expect(detailToForm(undefined)).toEqual({
      class: "",
      studentId: "",
      qq: "",
      direction: "",
    });
  });

  it("treats student_id 0 / null fields as unfilled", () => {
    expect(
      detailToForm({ class: "", student_id: 0, qq: null, direction: null }),
    ).toEqual({ class: "", studentId: "", qq: "", direction: "" });
  });
});

describe("formToUpdate", () => {
  it("converts empty strings to null so the backend clears the field", () => {
    expect(
      formToUpdate({ class: "  ", studentId: "", qq: "", direction: "" }),
    ).toEqual({ class: null, student_id: null, qq: null, direction: null });
  });

  it("keeps non-empty values and normalizes trimmed class", () => {
    expect(
      formToUpdate({
        class: " iot 2301 ",
        studentId: "2023114514",
        qq: "1044696157",
        direction: "back-end",
      }),
    ).toEqual({
      class: "iot 2301",
      student_id: 2023114514,
      qq: "1044696157",
      direction: "back-end",
    });
  });

  it("maps illegal student ids (0 / negative / fractional) to null", () => {
    for (const studentId of ["0", "-3", "3.7"]) {
      expect(
        formToUpdate({ class: "", studentId, qq: "", direction: "" }).student_id,
      ).toBeNull();
    }
  });
});

describe("isProfileEmpty", () => {
  it("is true only when every editable field is blank", () => {
    expect(
      isProfileEmpty({ class: "", studentId: "", qq: "", direction: "" }),
    ).toBe(true);
    expect(
      isProfileEmpty({ class: "iot 2301", studentId: "", qq: "", direction: "" }),
    ).toBe(false);
  });
});
