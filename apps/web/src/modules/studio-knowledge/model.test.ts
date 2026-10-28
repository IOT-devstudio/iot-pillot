import { describe, expect, it } from "vitest";

import type { StudioKnowledgeEntry } from "@/api/studio-knowledge";
import {
  EMPTY_KNOWLEDGE_DRAFT,
  bodyExcerpt,
  deleteConfirmText,
  draftErrors,
  formatDay,
  permissionLabel,
  resetDraft,
  toDraft,
  toUpdateInput,
  versionSummary,
  type KnowledgeDraft,
} from "./model";

function entry(overrides: Partial<StudioKnowledgeEntry> = {}): StudioKnowledgeEntry {
  return {
    id: 1,
    title: "部署手册",
    body: "第一版正文",
    author_id: 7,
    is_public: false,
    permission: "admin",
    parent_id: null,
    version: 1,
    created_at: "2026-10-27T12:00:00+08:00",
    updated_at: "2026-10-28T09:00:00+08:00",
    ...overrides,
  };
}

function validDraft(overrides: Partial<KnowledgeDraft> = {}): KnowledgeDraft {
  return { ...EMPTY_KNOWLEDGE_DRAFT, title: "部署手册", ...overrides };
}

describe("knowledge draft validation", () => {
  it("accepts a complete draft and an empty body", () => {
    expect(draftErrors(validDraft())).toEqual({});
    // 正文可以是空串：先建骨架、回头再写是正常流程。
    expect(draftErrors(validDraft({ body: "" }))).toEqual({});
  });

  it("requires a title and treats whitespace as empty", () => {
    expect(draftErrors(EMPTY_KNOWLEDGE_DRAFT).title).toBe("标题必填");
    expect(draftErrors(validDraft({ title: "   " })).title).toBe("标题必填");
  });

  it("caps the title at 200 characters", () => {
    const longTitle = "标".repeat(201);
    expect(draftErrors(validDraft({ title: longTitle })).title).toContain("200");
  });

  it("rejects an unknown permission", () => {
    const errors = draftErrors(
      validDraft({ permission: "everyone" as KnowledgeDraft["permission"] }),
    );
    expect(errors.permission).toBe("请选择有效的权限");
  });
});

describe("knowledge payload mapping", () => {
  it("sends only changed fields plus the version lock", () => {
    const original = entry();
    const draft = toDraft(original);
    // 什么都没改：补丁里只应剩 version（乐观锁必须带）。
    expect(toUpdateInput(draft, original)).toEqual({ version: 1 });
  });

  it("includes the edited body and keeps untouched fields out", () => {
    const original = entry();
    const draft = toDraft(original);
    draft.body = "第二版正文";
    expect(toUpdateInput(draft, original)).toEqual({
      body: "第二版正文",
      version: 1,
    });
  });

  it("carries the current version so concurrent edits are detected", () => {
    const original = entry({ version: 5 });
    const draft = toDraft(original);
    draft.title = "新标题";
    // 不带 version 会让后端退化成「总是覆盖」，并发编辑静默丢改动。
    expect(toUpdateInput(draft, original).version).toBe(5);
  });

  it("trims the title but leaves the body verbatim", () => {
    const original = entry();
    const draft = toDraft(original);
    draft.title = "  新标题  ";
    draft.body = "  缩进是 Markdown 的一部分  ";
    const patch = toUpdateInput(draft, original);
    expect(patch.title).toBe("新标题");
    expect(patch.body).toBe("  缩进是 Markdown 的一部分  ");
  });

  it("round-trips an entry into a draft and back", () => {
    const original = entry({ body: "多行\n正文", permission: "member" });
    const draft = toDraft(original);
    expect(draft).toEqual({
      title: "部署手册",
      body: "多行\n正文",
      permission: "member",
    });
    expect(resetDraft(original)).toEqual(draft);
  });
});

describe("knowledge presentation helpers", () => {
  it("labels the three permission levels and falls back to raw values", () => {
    expect(permissionLabel("admin")).toBe("仅管理员");
    expect(permissionLabel("member")).toBe("成员可读");
    expect(permissionLabel("public")).toBe("公开");
    expect(permissionLabel("future")).toBe("future");
  });

  it("summarises the version with a day-precision date", () => {
    expect(versionSummary(entry({ version: 3 }))).toBe("v3 · 2026-10-28");
  });

  it("formats a day and falls back for unparsable input", () => {
    expect(formatDay("2026-10-28T09:00:00+08:00")).toBe("2026-10-28");
    expect(formatDay("not-a-date")).toBe("not-a-date");
  });

  it("flattens newlines before truncating the excerpt", () => {
    expect(bodyExcerpt("第一行\n\n第二行")).toBe("第一行 第二行");
    const long = "字".repeat(200);
    expect(bodyExcerpt(long, 10)).toBe("字字字字字字字字字字…");
  });

  it("names the entry and its version count in the delete confirmation", () => {
    const text = deleteConfirmText(entry({ version: 4 }));
    expect(text).toContain("部署手册");
    expect(text).toContain("4");
  });
});
