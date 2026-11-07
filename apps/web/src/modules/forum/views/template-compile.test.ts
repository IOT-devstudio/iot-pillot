import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { parse } from "vue/compiler-sfc";

/**
 * 模板必须能编译（#242 踩过的坑）。
 *
 * dev server 遇到「标签不闭合」只会在终端日志里报 `Element is missing end tag`，
 * `vue-tsc --noEmit` 完全查不出来（它不校验模板标签配对），单测也照样全绿——
 * 结果就是页面直接 500、白屏，而所有本地检查都通过。
 *
 * 这里把 SFC 编译期错误拉进单测：`parse` 返回的 errors 非空即失败。
 * 以后任何一次模板结构调整（加 wrapper、挪 v-if 块）都会立刻在 CI 暴露。
 */
const FORUM_VIEWS = [
  "src/modules/forum/views/QaListView.vue",
  "src/modules/forum/views/QaDetailView.vue",
  "src/modules/forum/views/QaComposeView.vue",
];

describe("招新问答模板可编译", () => {
  for (const file of FORUM_VIEWS) {
    it(`${file} 无 SFC 编译错误`, () => {
      const { descriptor, errors } = parse(readFileSync(file, "utf8"), {
        filename: file,
      });
      expect(errors).toEqual([]);
      expect(descriptor.template).toBeDefined();
    });
  }
});
