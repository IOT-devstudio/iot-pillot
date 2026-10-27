import { describe, expect, it } from "vitest";

import type { StudioMember } from "@/api/studio-members";
import {
  EMPTY_STUDIO_MEMBER_DRAFT,
  archiveConfirmText,
  draftErrors,
  studioDirectionLabel,
  toCreateInput,
  toDraft,
  toUpdateInput,
  type StudioMemberDraft,
} from "./model";

function member(overrides: Partial<StudioMember> = {}): StudioMember {
  return {
    id: 1,
    user_id: null,
    name: "张三",
    birth_year: 2003,
    cohort: "2024 级",
    direction: "front-end",
    role_in_studio: "前端负责人",
    internship: "",
    employment: "",
    is_active: true,
    archived_at: null,
    source_candidate_id: null,
    created_at: "2026-10-27T12:00:00+08:00",
    updated_at: "2026-10-27T12:00:00+08:00",
    ...overrides,
  };
}

function validDraft(overrides: Partial<StudioMemberDraft> = {}): StudioMemberDraft {
  return {
    ...EMPTY_STUDIO_MEMBER_DRAFT,
    name: "张三",
    birth_year: "2003",
    cohort: "2024 级",
    direction: "front-end",
    role_in_studio: "前端负责人",
    ...overrides,
  };
}

describe("studio member draft validation", () => {
  it("accepts a complete draft and trims optional blanks", () => {
    expect(draftErrors(validDraft())).toEqual({});
    expect(draftErrors(validDraft({ internship: "  " }))).toEqual({});
  });

  it("requires the fields the backend requires", () => {
    const errors = draftErrors(EMPTY_STUDIO_MEMBER_DRAFT);
    expect(Object.keys(errors).sort()).toEqual([
      "birth_year",
      "cohort",
      "direction",
      "name",
      "role_in_studio",
    ]);
  });

  it("treats whitespace-only input as empty", () => {
    const errors = draftErrors(validDraft({ name: "   ", cohort: "  " }));
    expect(errors.name).toBe("姓名必填");
    expect(errors.cohort).toBe("届别必填（如「2024 级」）");
  });

  it("bounds the birth year to 1900-2100 and requires four digits", () => {
    expect(draftErrors(validDraft({ birth_year: "1899" })).birth_year).toContain("1900");
    expect(draftErrors(validDraft({ birth_year: "2101" })).birth_year).toContain("2100");
    expect(draftErrors(validDraft({ birth_year: "03" })).birth_year).toBe("出生年须为 4 位数字");
    expect(draftErrors(validDraft({ birth_year: "" })).birth_year).toBe("出生年必填");
  });

  it("rejects a non positive account id but allows leaving it blank", () => {
    expect(draftErrors(validDraft({ user_id: "" })).user_id).toBeUndefined();
    expect(draftErrors(validDraft({ user_id: "0" })).user_id).toContain("正整数");
    expect(draftErrors(validDraft({ user_id: "abc" })).user_id).toContain("正整数");
    expect(draftErrors(validDraft({ user_id: "7" })).user_id).toBeUndefined();
  });

  it("caps free text lengths at the backend limits", () => {
    const longName = "张".repeat(41);
    expect(draftErrors(validDraft({ name: longName })).name).toContain("40");
    const longEmployment = "x".repeat(201);
    expect(draftErrors(validDraft({ employment: longEmployment })).employment).toContain("200");
  });
});

describe("studio member payload mapping", () => {
  it("omits user_id entirely when the draft has no account", () => {
    // 无账号成员是正常情况：请求体里不能出现 user_id: null，
    // 否则后端会把它当成「显式绑定空账号」而不是「不绑定」。
    const input = toCreateInput(validDraft());
    expect("user_id" in input).toBe(false);
    expect(input).toMatchObject({ name: "张三", birth_year: 2003 });
  });

  it("includes a numeric user_id when one is provided", () => {
    expect(toCreateInput(validDraft({ user_id: "7" })).user_id).toBe(7);
  });

  it("sends only changed fields on update", () => {
    const original = member();
    const patch = toUpdateInput(toDraft(original), original);
    expect(patch).toEqual({});
  });

  it("detects a changed role and keeps untouched fields out of the patch", () => {
    const original = member();
    const draft = toDraft(original);
    draft.role_in_studio = "前端负责人 / 招新";
    expect(toUpdateInput(draft, original)).toEqual({
      role_in_studio: "前端负责人 / 招新",
    });
  });

  it("unbinds the account with an explicit null only when it actually changed", () => {
    const bound = member({ user_id: 7 });
    // 没动 user_id：不能把它塞进补丁，否则一次纯改姓名的保存会抹掉绑定。
    const untouched = toDraft(bound);
    untouched.name = "张三丰";
    expect(toUpdateInput(untouched, bound)).toEqual({ name: "张三丰" });

    const cleared = toDraft(bound);
    cleared.user_id = "";
    expect(toUpdateInput(cleared, bound)).toEqual({ user_id: null });
  });

  it("round-trips a member into a draft and back", () => {
    const original = member({ user_id: 12, internship: "某厂", employment: "某司" });
    expect(toDraft(original)).toMatchObject({
      name: "张三",
      birth_year: "2003",
      user_id: "12",
      internship: "某厂",
      employment: "某司",
    });
  });

  it("labels known directions and falls back to the raw code", () => {
    expect(studioDirectionLabel("front-end")).toBe("前端开发");
    expect(studioDirectionLabel("game-design")).toBe("game-design");
  });

  it("names the member in the archive confirmation", () => {
    expect(archiveConfirmText(member())).toContain("张三");
  });
});
