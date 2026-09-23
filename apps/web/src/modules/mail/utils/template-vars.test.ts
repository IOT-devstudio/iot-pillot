import { describe, expect, it } from "vitest";

import {
  extractVariables,
  mergeVariables,
  missingVariables,
  renderPreview,
  validateBraces,
} from "./template-vars";

describe("extractVariables", () => {
  it("extracts names in order and dedupes", () => {
    expect(
      extractVariables("Hi {{name}}, class={{class}} again {{name}}"),
    ).toEqual(["name", "class"]);
  });

  it("allows whitespace inside braces and ignores invalid placeholders", () => {
    // {{a-b}} 变量名含连字符，不符合后端 [A-Za-z0-9_]+ 规则，不应被识别
    expect(extractVariables("{{ name }} {{a-b}} {{}}")).toEqual(["name"]);
  });
});

describe("mergeVariables", () => {
  it("puts title variables before body-only variables, deduped", () => {
    expect(mergeVariables("关于 {{name}} 的通知", "{{name}} / {{class}}")).toEqual([
      "name",
      "class",
    ]);
  });
});

describe("validateBraces", () => {
  it("passes balanced and empty text", () => {
    expect(validateBraces("no placeholders")).toBeNull();
    expect(validateBraces("{{a}} and {{b}}")).toBeNull();
  });

  it("rejects unbalanced braces with a readable reason", () => {
    expect(validateBraces("hello {{name")).toMatch(/括号不匹配/);
    expect(validateBraces("hello name}}")).toMatch(/括号不匹配/);
  });
});

describe("missingVariables", () => {
  it("treats absent and blank values as missing", () => {
    expect(
      missingVariables(["name", "class", "round"], {
        name: "张三",
        class: "  ",
      }),
    ).toEqual(["class", "round"]);
  });
});

describe("renderPreview", () => {
  it("substitutes values with HTML escaping", () => {
    expect(renderPreview("Hi {{name}}", { name: "<b>李四</b>" })).toBe(
      "Hi &lt;b&gt;李四&lt;/b&gt;",
    );
  });

  it("keeps placeholders for missing or blank values so gaps stay visible", () => {
    expect(renderPreview("Hi {{name}} / {{class}}", { name: "", class: "" })).toBe(
      "Hi {{name}} / {{class}}",
    );
  });
});
