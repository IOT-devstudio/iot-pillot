import { describe, expect, it } from "vitest";

import pathsSource from "@/modules/learning-roadmap/views/LearningPathsView.vue?raw";
import detailSource from "@/modules/learning-roadmap/views/LearningPathDetailView.vue?raw";
import homeSource from "@/modules/home/views/UserHomeView.vue?raw";

/**
 * 作品提交的挂载位置。
 *
 * 需求明确：提交入口要放在**页面下方**通栏，不要堆到右侧。
 * 这条测试锁的就是位置——如果有人把它挪进侧栏或改成固定浮层，
 * 这里会失败。
 */

/** 取出 <template> 里的内容，只关心渲染结构。 */
function templateOf(source: string): string {
  const start = source.indexOf("<template>");
  const end = source.lastIndexOf("</template>");
  return start >= 0 && end > start ? source.slice(start, end) : source;
}

describe("作品提交的挂载位置", () => {
  it("在学习路线列表页，位于列表与分页之后", () => {
    const tpl = templateOf(pathsSource);
    const showcaseAt = tpl.indexOf("<ShowcasePanel");
    expect(showcaseAt).toBeGreaterThan(-1);
    // 必须在路线卡片列表之后：学员先看路线，再往下交作业。
    expect(tpl.indexOf("paths__grid")).toBeLessThan(showcaseAt);
    expect(tpl.indexOf("el-pagination")).toBeLessThan(showcaseAt);
  });

  it("在学习路线详情页，位于阶段列表之后", () => {
    const tpl = templateOf(detailSource);
    const showcaseAt = tpl.indexOf("<ShowcasePanel");
    expect(showcaseAt).toBeGreaterThan(-1);
    expect(tpl.indexOf("path__stages")).toBeLessThan(showcaseAt);
    // 详情页要带上路线 id，提交记录才能留住「交的是哪条路线」。
    expect(tpl).toContain(':roadmap-id="roadmap.id"');
  });

  it("在首页，位于报名流程区之后、收尾语之前", () => {
    const tpl = templateOf(homeSource);
    const showcaseAt = tpl.indexOf("<ShowcasePanel");
    expect(showcaseAt).toBeGreaterThan(-1);
    expect(tpl.indexOf('id="journey"')).toBeLessThan(showcaseAt);
    expect(tpl.indexOf("closing-note")).toBeGreaterThan(showcaseAt);
  });

  it("首页页内导航里有「作品提交」，且挨着「关于我们」", () => {
    const tpl = templateOf(homeSource);
    // 只在页内导航段里找：文件其他位置（注释、面板标题）也含「作品提交」，
    // 直接 indexOf 会命中注释而不是那个 <a>。
    const navStart = tpl.indexOf('aria-label="页面内导航"');
    expect(navStart).toBeGreaterThan(-1);
    const nav = tpl.slice(navStart, tpl.indexOf("</nav>", navStart));

    expect(nav).toContain("作品提交");
    expect(nav).toContain('@click="goShowcase"');

    // 需求：放在关于我们旁边（导航里紧邻）。
    const showcaseAt = nav.indexOf("作品提交");
    const aboutAt = nav.indexOf("关于我们");
    expect(aboutAt).toBeGreaterThan(showcaseAt);
    expect(aboutAt - showcaseAt).toBeLessThan(200);
  });

  it("三个挂载点都不使用右侧栏/固定浮层这类布局", () => {
    // 需求原话：「前端不要一直往右侧堆，放到页面下方」。
    // 面板本身是通栏 section，若被包进 position: fixed 的容器就不符合要求。
    for (const source of [pathsSource, detailSource, homeSource]) {
      const tpl = templateOf(source);
      const showcaseAt = tpl.indexOf("<ShowcasePanel");
      // 就近往上找 200 字符，确认没有被塞进 aside / 侧栏容器。
      const nearby = tpl.slice(Math.max(0, showcaseAt - 200), showcaseAt);
      expect(nearby).not.toMatch(/<aside[^>]*>\s*$/);
    }
  });
});
