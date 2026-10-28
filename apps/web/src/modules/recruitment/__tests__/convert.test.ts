import { describe, expect, it } from "vitest";

import {
  EMPTY_CONVERT_DRAFT,
  canConvertToMember,
  convertDraftErrors,
  toConvertInput,
  type ConvertDraft,
} from "../convert";

function validDraft(overrides: Partial<ConvertDraft> = {}): ConvertDraft {
  return {
    ...EMPTY_CONVERT_DRAFT,
    birth_year: "2003",
    cohort: "2024 级",
    role_in_studio: "前端成员",
    ...overrides,
  };
}

describe("canConvertToMember", () => {
  it("only offers the entry for hired candidates", () => {
    expect(canConvertToMember("hired")).toBe(true);
    // 其他状态由后端拒绝；把注定失败的按钮摆出来只会让人点一次、读一遍错误。
    expect(canConvertToMember("interviewing")).toBe(false);
    expect(canConvertToMember("offered")).toBe(false);
    expect(canConvertToMember("rejected")).toBe(false);
  });
});

describe("convertDraftErrors", () => {
  it("accepts a complete draft", () => {
    expect(convertDraftErrors(validDraft())).toEqual({});
  });

  it("requires the fields the recruitment data does not carry", () => {
    const errors = convertDraftErrors(EMPTY_CONVERT_DRAFT);
    expect(Object.keys(errors).sort()).toEqual([
      "birth_year",
      "cohort",
      "role_in_studio",
    ]);
  });

  it("treats whitespace-only input as empty", () => {
    const errors = convertDraftErrors(
      validDraft({ cohort: "  ", role_in_studio: "  " }),
    );
    expect(errors.cohort).toContain("必填");
    expect(errors.role_in_studio).toBe("工作室职能必填");
  });

  it("bounds the birth year and requires four digits", () => {
    expect(convertDraftErrors(validDraft({ birth_year: "1899" })).birth_year).toContain("1900");
    expect(convertDraftErrors(validDraft({ birth_year: "2101" })).birth_year).toContain("2100");
    expect(convertDraftErrors(validDraft({ birth_year: "03" })).birth_year).toBe(
      "出生年须为 4 位数字",
    );
  });

  it("caps optional free text at the backend limit", () => {
    const long = "x".repeat(201);
    expect(convertDraftErrors(validDraft({ employment: long })).employment).toContain("200");
  });
});

describe("toConvertInput", () => {
  it("carries the candidate id and the filled-in fields only", () => {
    const input = toConvertInput(55, validDraft());
    expect(input).toEqual({
      candidate_id: 55,
      birth_year: 2003,
      cohort: "2024 级",
      role_in_studio: "前端成员",
      internship: "",
      employment: "",
    });
    // 姓名与方向由后端从申请带过来，请求体里不该出现。
    expect("name" in input).toBe(false);
    expect("direction" in input).toBe(false);
  });

  it("trims every field", () => {
    const input = toConvertInput(
      1,
      validDraft({ cohort: "  2024 级  ", role_in_studio: " 成员 " }),
    );
    expect(input.cohort).toBe("2024 级");
    expect(input.role_in_studio).toBe("成员");
  });
});
