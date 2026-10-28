import { describe, expect, it } from "vitest";

import type { StudioProjectMember } from "@/api/studio-projects";
import {
  EMPTY_MEMBER_DRAFT,
  EMPTY_PROJECT_DRAFT,
  deleteProjectConfirmText,
  formatDay,
  isExternalParticipant,
  memberDraftErrors,
  memberEndSummary,
  memberToDraft,
  projectDraftErrors,
  projectExcerpt,
  projectRoleLabel,
  projectStatusLabel,
  toCreateInput,
  toMemberCreateInput,
  toMemberUpdateInput,
  toUpdateInput,
  projectToDraft,
  type MemberDraft,
  type ProjectDraft,
} from "./model";

function project(overrides: Partial<{
  id: number; name: string; year: string; summary: string; status: "active" | "completed" | "archived";
}> = {}) {
  return {
    id: 1,
    name: "校园导览小程序",
    year: "2024",
    summary: "新生报到用的校园导览。",
    status: "active" as const,
    created_at: "2026-10-28T10:00:00+08:00",
    updated_at: "2026-10-28T10:00:00+08:00",
    ...overrides,
  };
}

function member(overrides: Partial<StudioProjectMember> = {}): StudioProjectMember {
  return {
    id: 100,
    project_id: 1,
    member_id: 7,
    name: "张三",
    role: "lead",
    started_at: "2024-03-01T00:00:00+08:00",
    ended_at: null,
    created_at: "2024-03-01T00:00:00+08:00",
    ...overrides,
  };
}

function validProjectDraft(overrides: Partial<ProjectDraft> = {}): ProjectDraft {
  return { ...EMPTY_PROJECT_DRAFT, name: "校园导览小程序", year: "2024", ...overrides };
}

function validMemberDraft(overrides: Partial<MemberDraft> = {}): MemberDraft {
  return { ...EMPTY_MEMBER_DRAFT, name: "张三", ...overrides };
}

describe("project draft validation", () => {
  it("accepts a complete draft", () => {
    expect(projectDraftErrors(validProjectDraft())).toEqual({});
  });

  it("requires name and year", () => {
    const errors = projectDraftErrors(EMPTY_PROJECT_DRAFT);
    expect(errors.name).toBe("项目名必填");
    expect(errors.year).toBe("年份必填（如「2023」）");
  });

  it("treats whitespace-only input as empty", () => {
    expect(projectDraftErrors(validProjectDraft({ name: "  " })).name).toBe("项目名必填");
    expect(projectDraftErrors(validProjectDraft({ year: " " })).year).toBe("年份必填（如「2023」）");
  });

  it("bounds name, year and summary at the backend limits", () => {
    expect(projectDraftErrors(validProjectDraft({ name: "项".repeat(101) })).name).toContain("100");
    expect(projectDraftErrors(validProjectDraft({ year: "y".repeat(21) })).year).toContain("20");
    expect(
      projectDraftErrors(validProjectDraft({ summary: "x".repeat(2001) })).summary,
    ).toContain("2000");
  });

  it("rejects a status outside the enum", () => {
    expect(projectDraftErrors(validProjectDraft({ status: "in_progress" as never })).status).toBe(
      "请选择有效的状态",
    );
  });
});

describe("project payload mapping", () => {
  it("sends only changed fields on update", () => {
    const original = project();
    const draft = projectToDraft(original);
    expect(toUpdateInput(draft, original)).toEqual({});
  });

  it("detects a changed year and keeps untouched fields out of the patch", () => {
    const original = project();
    const draft = projectToDraft(original);
    draft.year = "2025";
    expect(toUpdateInput(draft, original)).toEqual({ year: "2025" });
  });

  it("trims every field", () => {
    const input = toCreateInput(validProjectDraft({ name: "  项目  ", year: " 2024 " }));
    expect(input.name).toBe("项目");
    expect(input.year).toBe("2024");
  });

  it("round-trips a project into a draft", () => {
    const original = project({ summary: "多行\n简介", status: "completed" });
    expect(projectToDraft(original)).toEqual({
      name: "校园导览小程序",
      year: "2024",
      summary: "多行\n简介",
      status: "completed",
    });
  });
});

describe("member draft validation", () => {
  it("accepts a complete draft", () => {
    expect(memberDraftErrors(validMemberDraft())).toEqual({});
  });

  it("requires the name snapshot", () => {
    expect(memberDraftErrors(EMPTY_MEMBER_DRAFT).name).toContain("快照");
  });

  it("allows an empty member_id (external participant) but rejects a bad one", () => {
    expect(memberDraftErrors(validMemberDraft({ member_id: "" })).member_id).toBeUndefined();
    expect(memberDraftErrors(validMemberDraft({ member_id: "0" })).member_id).toContain("正整数");
    expect(memberDraftErrors(validMemberDraft({ member_id: "abc" })).member_id).toContain("正整数");
    expect(memberDraftErrors(validMemberDraft({ member_id: "7" })).member_id).toBeUndefined();
  });

  it("caps the name snapshot at 40 characters", () => {
    expect(memberDraftErrors(validMemberDraft({ name: "名".repeat(41) })).name).toContain("40");
  });
});

describe("member payload mapping", () => {
  it("maps an empty member_id to null (external participant)", () => {
    expect(toMemberCreateInput(validMemberDraft()).member_id).toBeNull();
  });

  it("maps a numeric member_id to a number", () => {
    expect(toMemberCreateInput(validMemberDraft({ member_id: "7" })).member_id).toBe(7);
  });

  it("omits started_at when blank and maps an empty ended_at to null", () => {
    const input = toMemberCreateInput(validMemberDraft());
    expect("started_at" in input).toBe(false);
    expect(input.ended_at).toBeNull();
  });

  it("sends an empty patch when nothing changed", () => {
    const original = member();
    const draft = memberToDraft(original);
    expect(toMemberUpdateInput(draft, original)).toEqual({});
  });

  it("clears the association when the member_id becomes empty", () => {
    const original = member({ member_id: 7 });
    const draft = memberToDraft(original);
    draft.member_id = "";
    expect(toMemberUpdateInput(draft, original)).toEqual({ member_id: null });
  });

  it("keeps the association when the member_id is untouched", () => {
    const original = member({ member_id: 7 });
    const draft = memberToDraft(original);
    draft.name = "张三丰";
    expect(toMemberUpdateInput(draft, original)).toEqual({ name: "张三丰" });
  });

  it("restores ongoing participation when ended_at is cleared", () => {
    const original = member({ ended_at: "2025-01-01T00:00:00+08:00" });
    const draft = memberToDraft(original);
    draft.ended_at = "";
    expect(toMemberUpdateInput(draft, original)).toEqual({ ended_at: null });
  });

  it("keeps ended_at when the key is absent from the patch", () => {
    const original = member({ ended_at: "2025-01-01T00:00:00+08:00" });
    const draft = memberToDraft(original);
    draft.role = "contributor";
    expect(toMemberUpdateInput(draft, original)).toEqual({ role: "contributor" });
  });

  it("round-trips a member into a draft", () => {
    const original = member({ member_id: null, ended_at: "2025-01-01T00:00:00+08:00" });
    const draft = memberToDraft(original);
    expect(draft.member_id).toBe("");
    expect(draft.ended_at).toBe("2025-01-01T00:00:00+08:00");
  });
});

describe("project presentation helpers", () => {
  it("labels status and role, falling back to raw values", () => {
    expect(projectStatusLabel("active")).toBe("进行中");
    expect(projectStatusLabel("completed")).toBe("已完成");
    expect(projectStatusLabel("archived")).toBe("已归档");
    expect(projectStatusLabel("future")).toBe("future");
    expect(projectRoleLabel("lead")).toBe("负责人");
    expect(projectRoleLabel("contributor")).toBe("参与者");
  });

  it("distinguishes ongoing from ended participation", () => {
    expect(memberEndSummary(member())).toBe("仍在参与");
    expect(
      memberEndSummary(member({ ended_at: "2025-01-01T00:00:00+08:00" })),
    ).toBe("已结束 · 2025-01-01");
  });

  it("flags external participants", () => {
    expect(isExternalParticipant(member())).toBe(false);
    expect(isExternalParticipant(member({ member_id: null }))).toBe(true);
  });

  it("formats a day and falls back for unparsable input", () => {
    expect(formatDay("2024-03-01T00:00:00+08:00")).toBe("2024-03-01");
    expect(formatDay("not-a-date")).toBe("not-a-date");
  });

  it("flattens newlines before truncating the excerpt", () => {
    expect(projectExcerpt("第一行\n\n第二行")).toBe("第一行 第二行");
    expect(projectExcerpt("字".repeat(200), 10)).toBe("字字字字字字字字字字…");
  });

  it("names the project and its participation count in the delete confirmation", () => {
    expect(deleteProjectConfirmText(project(), 3)).toContain("校园导览小程序");
    expect(deleteProjectConfirmText(project(), 3)).toContain("3");
  });
});
