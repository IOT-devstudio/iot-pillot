import { describe, expect, it } from "vitest";

import {
  detailToForm,
  formToUpdate,
  isProfileEmpty,
} from "./profile";

describe("detailToForm", () => {
  it("returns empty strings for a missing detail", () => {
    expect(detailToForm({})).toEqual({
      class: "",
      studentId: "",
      qq: "",
      direction: "",
    });
  });

  it("preserves defined fields and stringifies student_id", () => {
    expect(
      detailToForm({
        class: "iot 2301",
        student_id: 2023114514,
        qq: "1044696157",
        direction: "front-end",
      }),
    ).toEqual({
      class: "iot 2301",
      studentId: "2023114514",
      qq: "1044696157",
      direction: "front-end",
    });
  });

  it("treats null direction as '未选择'", () => {
    expect(detailToForm({ direction: null }).direction).toBe("");
  });

  it("treats student_id === 0 as empty (per #57 spec)", () => {
    expect(detailToForm({ student_id: 0 }).studentId).toBe("");
  });
});

describe("formToUpdate", () => {
  it("turns empty strings into null so the backend knows 'clear'", () => {
    expect(
      formToUpdate({
        class: "",
        studentId: "",
        qq: "",
        direction: "",
      }),
    ).toEqual({
      class: null,
      student_id: null,
      qq: null,
      direction: null,
    });
  });

  it("trims text fields and keeps the value if non-empty", () => {
    expect(
      formToUpdate({
        class: "  iot 2301  ",
        studentId: "",
        qq: " 1044696157 ",
        direction: "",
      }),
    ).toEqual({
      class: "iot 2301",
      student_id: null,
      qq: "1044696157",
      direction: null,
    });
  });

  it("parses positive integers and drops 0 / negative / non-numeric", () => {
    expect(
      formToUpdate({
        class: "",
        studentId: "0",
        qq: "",
        direction: "",
      }).student_id,
    ).toBeNull();
    expect(
      formToUpdate({
        class: "",
        studentId: "-5",
        qq: "",
        direction: "",
      }).student_id,
    ).toBeNull();
    expect(
      formToUpdate({
        class: "",
        studentId: "3.7",
        qq: "",
        direction: "",
      }).student_id,
    ).toBeNull();
    expect(
      formToUpdate({
        class: "",
        studentId: "2023114514",
        qq: "",
        direction: "",
      }).student_id,
    ).toBe(2023114514);
  });

  it("passes through a chosen direction", () => {
    expect(
      formToUpdate({
        class: "",
        studentId: "",
        qq: "",
        direction: "agent",
      }).direction,
    ).toBe("agent");
  });
});

describe("isProfileEmpty", () => {
  it("is true when every editable field is blank", () => {
    expect(
      isProfileEmpty({ class: "", studentId: "", qq: "", direction: "" }),
    ).toBe(true);
  });

  it("treats whitespace-only text as empty (matches formToUpdate's trim)", () => {
    expect(
      isProfileEmpty({ class: "  ", studentId: "", qq: "", direction: "" }),
    ).toBe(true);
    expect(
      isProfileEmpty({ class: "", studentId: "", qq: "", direction: "" }),
    ).toBe(true);
    expect(
      isProfileEmpty({ class: "", studentId: "", qq: "", direction: "all" }),
    ).toBe(false);
  });
});