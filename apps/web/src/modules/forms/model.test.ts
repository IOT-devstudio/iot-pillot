import { describe, expect, it } from "vitest";
import { answerErrors, editorError, initialAnswers, answerRows, firstErrorFieldId } from "./model";
import type { FormField } from "@/api/forms";

const fields: FormField[] = [
  {
    id: 1,
    type: "text",
    label: "作品",
    required: true,
    placeholder: "",
    sort_order: 0,
  },
  {
    id: 2,
    type: "email",
    label: "联系邮箱",
    required: false,
    placeholder: "",
    sort_order: 1,
  },
  {
    id: 3,
    type: "textarea",
    label: "项目",
    required: true,
    placeholder: "",
    sort_order: 2,
  },
  {
    id: 4,
    type: "select",
    label: "语言",
    required: true,
    placeholder: "",
    options: ["Go", "Java"],
    sort_order: 3,
  },
  {
    id: 5,
    type: "checkbox",
    label: "确认真实",
    required: true,
    placeholder: "",
    sort_order: 4,
  },
];

describe("form answers", () => {
  it("initializes all five types and validates required, email and select answers", () => {
    const values = initialAnswers(fields);
    expect(values).toEqual({ "1": "", "2": "", "3": "", "4": "", "5": [] });
    expect(Object.keys(answerErrors(fields, values))).toEqual([
      "1",
      "3",
      "4",
      "5",
    ]);
    expect(
      answerErrors(fields, {
        "1": "url",
        "2": "bad",
        "3": "项目",
        "4": "Rust",
        "5": ["确认真实"],
      }),
    ).toMatchObject({ "2": expect.any(String), "4": expect.any(String) });
    expect(
      answerErrors(fields, {
        "1": "url",
        "2": "a@b.cn",
        "3": "项目",
        "4": "Go",
        "5": ["确认真实"],
      }),
    ).toEqual({});
  });
  it("rejects whitespace, wrong shapes and oversized answers", () => {
    expect(
      answerErrors(fields, {
        "1": " ",
        "2": [],
        "3": "x".repeat(2001),
        "4": [],
        "5": "yes",
      }),
    ).toHaveProperty("5");
    expect(
      answerErrors(fields, { "3": "😀".repeat(2000) })["3"],
    ).toBeUndefined();
  });
  it("locates the first error by question order instead of error key order", () => {
    // 题目顺序由 sort_order 决定，与字段 id 的大小无关；错误对象的键却总是
    // 按整数升序枚举。只取 Object.keys(errors)[0] 会把用户送到第三题。
    const ordered: FormField[] = [
      { id: 9, type: "text", label: "第一题", required: true, placeholder: "", sort_order: 0 },
      { id: 3, type: "text", label: "第二题", required: true, placeholder: "", sort_order: 1 },
      { id: 7, type: "text", label: "第三题", required: true, placeholder: "", sort_order: 2 },
    ];
    const errors = { "3": "请填写此必填项", "7": "请填写此必填项", "9": "请填写此必填项" };
    expect(Object.keys(errors)[0]).toBe("3");
    expect(firstErrorFieldId(ordered, errors)).toBe("9");
    expect(firstErrorFieldId(ordered, { "7": "请填写此必填项" })).toBe("7");
    expect(firstErrorFieldId(ordered, {})).toBeNull();
    // 已删除字段的历史错误不在页面上，不能拿它去定位
    expect(firstErrorFieldId(ordered, { "99": "已删除字段" })).toBeNull();
  });
  it("preserves historical unknown fields and renders all answer kinds", () => {
    const rows = answerRows(fields, {
      "1": "链接",
      "2": "a@b.cn",
      "3": "两行\n项目",
      "4": "Go",
      "5": ["确认真实"],
      "99": "历史值",
    });
    expect(rows.map((row) => row.value)).toEqual([
      "链接",
      "a@b.cn",
      "两行\n项目",
      "Go",
      "确认真实",
      "历史值",
    ]);
    expect(rows.at(-1)?.label).toBe("已删除字段 #99");
  });
});

describe("form editor validation", () => {
  const draft = () => ({
    title: "报名表",
    description: "",
    round_id: 1,
    direction_code: "back-end",
    fields: fields.map((f) => ({ ...f })),
  });
  it("accepts the actual API contract and rejects empty/duplicate select options", () => {
    expect(editorError(draft())).toBe("");
    const input = draft();
    input.fields[3]!.options = ["Go", " Go "];
    expect(editorError(input)).toContain("选项");
    input.fields = [];
    expect(editorError(input)).toContain("字段");
  });
});
