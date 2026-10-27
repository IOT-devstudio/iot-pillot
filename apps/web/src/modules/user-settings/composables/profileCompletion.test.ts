import { describe, expect, it } from "vitest";
import { missingRequiredProfile, validateRequiredProfile } from "../profileCompletion";

describe("required profile fields", () => {
  it("reminds when either class or student ID is missing, including server sentinel zero", () => {
    expect(missingRequiredProfile({ class: "软件工程230301", student_id: 0 })).toEqual(["学号"]);
    expect(missingRequiredProfile({ class: " ", student_id: 202311451 })).toEqual(["班级"]);
    expect(missingRequiredProfile()).toEqual(["班级", "学号"]);
    expect(missingRequiredProfile({ class: "软件工程230301", student_id: 202311451 })).toEqual([]);
  });

  it("treats legacy or malformed formats as incomplete, not only empty values", () => {
    // 0012：注册满 7 天资料不合规会被冻结，因此前端横幅也要认格式，
    // 不能等用户收到冻结邮件才知道写错了。
    expect(missingRequiredProfile({ class: "软工 2 班", student_id: 202311451 })).toEqual(["班级"]);
    expect(missingRequiredProfile({ class: "物联网工程 2301", student_id: 202311451 })).toEqual(["班级"]);
    expect(missingRequiredProfile({ class: "软件工程230301", student_id: 2023114514 })).toEqual(["学号"]);
    expect(missingRequiredProfile({ class: "软件工程230301", student_id: 20231145 })).toEqual(["学号"]);
  });

  it("rejects empty, fractional, negative, unsafe and nonnumeric student IDs without clearing them", () => {
    for (const studentId of ["", "0", "-1", "1.5", "hello", "1e3", "9007199254740992"]) {
      expect(validateRequiredProfile({ class: "软件工程230301", studentId }).studentId).toBeTruthy();
    }
    expect(validateRequiredProfile({ class: " ", studentId: 202311451 }).class).toBeTruthy();
    expect(validateRequiredProfile({ class: "a".repeat(65), studentId: 202311451 }).class).toBeTruthy();
    expect(validateRequiredProfile({ class: "软件工程230301", studentId: 202311451 })).toEqual({});
  });

  it("要求班级是「汉字 + 6 位数字」", () => {
    expect(validateRequiredProfile({ class: "软件工程230301", studentId: "" }).class).toBeUndefined();
    expect(validateRequiredProfile({ class: "软工230301", studentId: "" }).class).toBeUndefined();
    for (const className of ["软件工程23030", "软件工程2303011", "软工", "230301", "软工 230301", "soft230301"]) {
      expect(validateRequiredProfile({ class: className, studentId: "" }).class).toBeTruthy();
    }
  });

  it("要求学号是 9 位数字且首位非 0", () => {
    expect(validateRequiredProfile({ class: "软件工程230301", studentId: "202311451" }).studentId).toBeUndefined();
    for (const studentId of ["2311451", "2023114514", "020311451", "20231145"]) {
      expect(validateRequiredProfile({ class: "软件工程230301", studentId }).studentId).toBeTruthy();
    }
  });
});
