import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ATTACHMENT_MAX_PER_DOCUMENT,
  ATTACHMENT_UPLOAD_TIMEOUT_MS,
  attachmentUrl,
  isAttachmentUrl,
  parseAttachmentId,
} from "./attachment";

/**
 * 附件地址工具的纯逻辑（#220）。
 *
 * 重点在「只认本站前缀」：ADR 0004 决策 3 要求外链不参与附件逻辑，
 * 否则别人的域名下同名路径也会被当成我们的附件去带 token 拉取。
 */

describe("attachment — URL 拼接与识别", () => {
  it("由 id 拼出相对地址", () => {
    expect(attachmentUrl(123)).toBe("/api/v1/attachments/123");
  });

  it("识别本站附件并解析 id", () => {
    expect(isAttachmentUrl("/api/v1/attachments/123")).toBe(true);
    expect(parseAttachmentId("/api/v1/attachments/123")).toBe(123);
  });

  it("容忍正文里手写的 query 与 hash", () => {
    expect(parseAttachmentId("/api/v1/attachments/9?v=2")).toBe(9);
    expect(parseAttachmentId("/api/v1/attachments/9#top")).toBe(9);
  });

  it("外链不认（即使路径相同），避免拿我们的 token 去拉别人的图", () => {
    expect(isAttachmentUrl("https://example.com/api/v1/attachments/1")).toBe(false);
    expect(parseAttachmentId("https://example.com/api/v1/attachments/1")).toBeNull();
  });

  it("非数字 id 不算本站附件", () => {
    expect(isAttachmentUrl("/api/v1/attachments/abc")).toBe(false);
    expect(isAttachmentUrl("/api/v1/attachments/")).toBe(false);
    expect(isAttachmentUrl("/api/v1/attachment/1")).toBe(false);
  });

  it("上传超时明显长于常规 15s（ADR 0004：上传要更长超时）", () => {
    expect(ATTACHMENT_UPLOAD_TIMEOUT_MS).toBeGreaterThan(15000);
  });

  it("一次最多 9 张（ADR 0004 决策 4 的业务上限）", () => {
    expect(ATTACHMENT_MAX_PER_DOCUMENT).toBe(9);
  });
});
