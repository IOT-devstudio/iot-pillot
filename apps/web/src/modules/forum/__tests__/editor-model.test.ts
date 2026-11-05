import { describe, expect, it } from "vitest";
import {
  altFromFileName,
  blockReasonOnSubmit,
  insertSyntaxAtCursor,
  markdownSyntaxFor,
  overLimitMessage,
  reconcileWithBody,
  remainingSlots,
  removeSyntax,
  type ImageItem,
} from "../editor-model";
import { ATTACHMENT_MAX_PER_DOCUMENT } from "@/api/attachment";

/**
 * 编辑器插图状态机（#220 验收：上传失败重试、9 张上限、删缩略图即移除语法）。
 */

function doneItem(id: number, fileName = "a.png"): ImageItem {
  return {
    key: `k${id}`,
    attachmentId: id,
    fileName,
    status: "done",
    error: "",
    previewUrl: "",
  };
}

function failedItem(key = "kf"): ImageItem {
  return {
    key,
    attachmentId: null,
    fileName: "bad.png",
    status: "failed",
    error: "网络错误",
    previewUrl: "",
  };
}

function uploadingItem(key = "ku"): ImageItem {
  return {
    key,
    attachmentId: null,
    fileName: "slow.png",
    status: "uploading",
    error: "",
    previewUrl: "",
  };
}

describe("editor-model — alt 与语法生成", () => {
  it("文件名去扩展名做 alt，空名退回「图片」", () => {
    expect(altFromFileName("screenshot.png")).toBe("screenshot");
    expect(altFromFileName("设计稿.v2.jpg")).toBe("设计稿.v2");
    expect(altFromFileName("")).toBe("图片");
    expect(altFromFileName("   ")).toBe("图片");
  });

  it("已完成的条目生成标准 Markdown 语法", () => {
    expect(markdownSyntaxFor(doneItem(123, "图1.png"))).toBe(
      "![图1](/api/v1/attachments/123)",
    );
  });

  it("上传中/失败的条目没有语法（正文里本来就不该有）", () => {
    expect(markdownSyntaxFor(uploadingItem())).toBeNull();
    expect(markdownSyntaxFor(failedItem())).toBeNull();
  });
});

describe("editor-model — 9 张上限", () => {
  it("空条带剩 9 个空位", () => {
    expect(remainingSlots([])).toBe(ATTACHMENT_MAX_PER_DOCUMENT);
  });

  it("失败项也占名额（否则一屏红叉还能再选 9 张）", () => {
    const items = [failedItem("a"), failedItem("b")];
    expect(remainingSlots(items)).toBe(ATTACHMENT_MAX_PER_DOCUMENT - 2);
  });

  it("未超限时不提示", () => {
    expect(overLimitMessage(3, [doneItem(1)])).toBeNull();
  });

  it("正好满额时提示「先移除」而不是部分接收", () => {
    const items = Array.from({ length: 9 }, (_, i) => doneItem(i + 1));
    expect(overLimitMessage(1, items)).toContain("请先移除一些");
  });

  it("超限但有余位时明确说只接收前 N 张", () => {
    const items = Array.from({ length: 7 }, (_, i) => doneItem(i + 1));
    const msg = overLimitMessage(5, items);
    expect(msg).toContain("本次只接收前 2 张");
  });
});

describe("editor-model — 提交闸门", () => {
  it("有失败中的图必须拦住", () => {
    expect(blockReasonOnSubmit([doneItem(1), failedItem()])).toContain(
      "还有 1 张图片上传失败",
    );
  });

  it("有上传中的图也要等（否则会静默丢图）", () => {
    expect(blockReasonOnSubmit([uploadingItem()])).toContain("正在上传");
  });

  it("全部完成时放行", () => {
    expect(blockReasonOnSubmit([doneItem(1), doneItem(2)])).toBeNull();
    expect(blockReasonOnSubmit([])).toBeNull();
  });
});

describe("editor-model — 删除缩略图即从正文移除语法", () => {
  it("语法独占一行时连同前面的换行一起删，不留空行", () => {
    const item = doneItem(7, "x.png");
    const body = "第一段\n![x](/api/v1/attachments/7)\n第二段";
    expect(removeSyntax(body, item)).toBe("第一段\n第二段");
  });

  it("语法夹在文字中间时只删语法本身", () => {
    const item = doneItem(8, "y.png");
    const body = "前![y](/api/v1/attachments/8)后";
    expect(removeSyntax(body, item)).toBe("前后");
  });

  it("上传中的条目正文里没有语法，原样返回", () => {
    const body = "正常正文";
    expect(removeSyntax(body, uploadingItem())).toBe(body);
  });

  it("只删自己那条，不误伤别的图", () => {
    const a = doneItem(1, "a.png");
    const body = "![a](/api/v1/attachments/1)\n![b](/api/v1/attachments/2)";
    expect(removeSyntax(body, a)).toBe("![b](/api/v1/attachments/2)");
  });
});

describe("editor-model — 光标处插入", () => {
  it("空正文直接插入并以换行结尾", () => {
    const r = insertSyntaxAtCursor("", 0, "![a](/api/v1/attachments/1)");
    expect(r.body).toBe("![a](/api/v1/attachments/1)\n");
    expect(r.cursor).toBe(r.body.length);
  });

  it("插在段落中间时前面补换行，避免图片并进同一段", () => {
    const r = insertSyntaxAtCursor("前半后半", 2, "![a](/api/v1/attachments/1)");
    expect(r.body).toBe("前半\n![a](/api/v1/attachments/1)\n后半");
  });

  it("光标越界被夹到合法范围", () => {
    expect(insertSyntaxAtCursor("abc", 999, "X").body).toBe("abc\nX\n");
    expect(insertSyntaxAtCursor("abc", -5, "X").body).toBe("X\nabc");
  });
});

describe("editor-model — 正文是唯一真源", () => {
  it("正文里被手动删掉的图，其条目在条带里也消失", () => {
    const kept = doneItem(1, "a.png");
    const removed = doneItem(2, "b.png");
    const body = "![a](/api/v1/attachments/1)";
    const next = reconcileWithBody([kept, removed], body);
    expect(next.map((i) => i.attachmentId)).toEqual([1]);
  });

  it("上传中/失败的条目没有语法可比，一律保留", () => {
    const next = reconcileWithBody([uploadingItem(), failedItem()], "");
    expect(next).toHaveLength(2);
  });

  it("语法仍在时条目保留", () => {
    const item = doneItem(3, "c.png");
    expect(reconcileWithBody([item], "![c](/api/v1/attachments/3)")).toEqual([item]);
  });
});
