import { describe, expect, it } from "vitest";
import { missingRequiredProfile, validateRequiredProfile } from "../profileCompletion";

describe("required profile fields", () => {
  it("reminds when either class or student ID is missing, including server sentinel zero", () => {
    expect(missingRequiredProfile({ class: "iot", student_id: 0 })).toEqual(["学号"]);
    expect(missingRequiredProfile({ class: " ", student_id: 12 })).toEqual(["班级"]);
    expect(missingRequiredProfile()).toEqual(["班级", "学号"]);
    expect(missingRequiredProfile({ class: "iot", student_id: 12 })).toEqual([]);
  });

  it("rejects empty, fractional, negative, unsafe and nonnumeric student IDs without clearing them", () => {
    for (const studentId of ["", "0", "-1", "1.5", "hello", "1e3", "9007199254740992"]) {
      expect(validateRequiredProfile({ class: "iot", studentId }).studentId).toBeTruthy();
    }
    expect(validateRequiredProfile({ class: " ", studentId: 123 }).class).toBeTruthy();
    expect(validateRequiredProfile({ class: "a".repeat(65), studentId: 123 }).class).toBeTruthy();
    expect(validateRequiredProfile({ class: "iot", studentId: "2023114514" })).toEqual({});
    expect(validateRequiredProfile({ class: "iot", studentId: 123 })).toEqual({});
  });
});
