import { describe, expect, it, vi } from "vitest";

import {
  directionsSettled,
  documentOffsetTop,
  waitUntil,
  type OffsetChainNode,
} from "./hashScroll";

/** 可控时钟：wait 只推进时间，不真的 sleep。 */
function fakeClock() {
  let time = 0;
  const wait = vi.fn(async (ms: number) => {
    time += ms;
  });
  return {
    now: () => time,
    wait,
    advance: (ms: number) => {
      time += ms;
    },
  };
}

describe("waitUntil", () => {
  it("returns immediately when the condition already holds", async () => {
    const clock = fakeClock();
    await expect(
      waitUntil(() => true, {
        timeoutMs: 100,
        intervalMs: 10,
        now: clock.now,
        wait: clock.wait,
      }),
    ).resolves.toBe(true);
    expect(clock.wait).not.toHaveBeenCalled();
  });

  it("polls until the condition flips", async () => {
    const clock = fakeClock();
    let checks = 0;
    const check = () => ++checks >= 3;
    await expect(
      waitUntil(check, {
        timeoutMs: 100,
        intervalMs: 10,
        now: clock.now,
        wait: clock.wait,
      }),
    ).resolves.toBe(true);
    expect(clock.wait).toHaveBeenCalledTimes(2);
  });

  it("gives up at the deadline instead of waiting forever", async () => {
    const clock = fakeClock();
    await expect(
      waitUntil(() => false, {
        timeoutMs: 100,
        intervalMs: 30,
        now: clock.now,
        wait: clock.wait,
      }),
    ).resolves.toBe(false);
    // 0/30/60/90 都还没到 deadline，等到 120 才发现已超时
    expect(clock.wait).toHaveBeenCalledTimes(4);
  });
});

describe("directionsSettled", () => {
  const base = { loading: false, error: "", count: 3, rendered: 3 };

  it("keeps waiting while the request is in flight, even with stale cards", () => {
    expect(directionsSettled({ ...base, loading: true, rendered: 3 })).toBe(false);
  });

  it("treats error and empty states as final layouts", () => {
    expect(directionsSettled({ ...base, count: 0, rendered: 0 })).toBe(true);
    expect(
      directionsSettled({ loading: false, error: "加载失败", count: 0, rendered: 0 }),
    ).toBe(true);
  });

  it("waits for cards to be rendered before allowing the scroll", () => {
    expect(directionsSettled({ ...base, rendered: 0 })).toBe(false);
    expect(directionsSettled({ ...base, rendered: 1 })).toBe(true);
  });
});

describe("documentOffsetTop", () => {
  /** 构造 offsetTop 链：[section, main, body] 这类关系。 */
  function chain(...tops: number[]) {
    let parent: OffsetChainNode | null = null;
    for (const top of tops.reverse()) {
      parent = { offsetTop: top, offsetParent: parent };
    }
    return parent;
  }

  it("sums offsetTop up the offsetParent chain", () => {
    expect(documentOffsetTop(chain(16, 40, 680))).toBe(736);
  });

  it("returns zero without a node", () => {
    expect(documentOffsetTop(null)).toBe(0);
  });

  it("ignores transforms because it only reads layout offsets", () => {
    // 入场动画中的 translateY 不会出现在 offsetTop 上；
    // 链条里只有布局值，正是动画结束后落点的位置。
    const section = { offsetTop: 737, offsetParent: null };
    expect(documentOffsetTop(section)).toBe(737);
  });
});
