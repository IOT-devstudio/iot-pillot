/**
 * 首页 hash 锚点定位（issue #150）。
 *
 * 个人设置的「去选择」只负责把路由推到 /user/home#directions；真正的
 * 定位在这里完成。方向卡是异步数据，接口结束 ≠ 已经渲染进 DOM，
 * 必须等卡片出现再滚，否则滚到的是骨架/空白版块。
 *
 * 抽成纯函数便于单测；视图只负责传响应式状态和真实 DOM。
 */

export interface WaitOptions {
  timeoutMs: number;
  intervalMs: number;
  /** 注入时钟与等待器，单测里不必真的 sleep */
  now?: () => number;
  wait?: (ms: number) => Promise<void>;
}

/** 轮询直到条件成立；超时返回 false，由调用方决定是否兜底继续。 */
export async function waitUntil(
  check: () => boolean,
  options: WaitOptions,
): Promise<boolean> {
  const now = options.now ?? Date.now;
  const wait =
    options.wait ??
    ((ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms)));
  const deadline = now() + options.timeoutMs;
  while (!check()) {
    if (now() >= deadline) return false;
    await wait(options.intervalMs);
  }
  return true;
}

export interface DirectionsSettledState {
  loading: boolean;
  error: string;
  /** 接口返回的方向数 */
  count: number;
  /** 已经渲染进 DOM 的方向卡数 */
  rendered: number;
}

/**
 * 方向区是否已经「尘埃落定」：
 * - 接口还在飞 → 再等；
 * - 接口失败或本轮没有方向 → 错误态/空态就是最终布局，可以滚；
 * - 有方向 → 至少渲染出一张卡片（v-for 一次 flush，出现即全部）。
 */
export function directionsSettled(state: DirectionsSettledState): boolean {
  if (state.loading) return false;
  if (state.error !== "" || state.count === 0) return true;
  return state.rendered > 0;
}

/** 只依赖 offsetTop/offsetParent，单测可以用普通对象构造链条。 */
export interface OffsetChainNode {
  offsetTop: number;
  offsetParent: unknown;
}

/**
 * 元素相对文档顶部的布局位置。
 *
 * 显式按文档纵向滚动计算，不复用 scrollIntoView：首页在部分移动端浏览器上
 * 的最近滚动祖先判断并不可靠，而方向 rail 只是内部的横向滚动容器，
 * 不该参与纵向定位。
 *
 * 用 offsetTop 链条而不是 getBoundingClientRect：方向区有 translateY 入场
 * 动画，rect 会把还没走完的位移算进目标位置，滚动停稳后反而越过版块顶部。
 * offsetTop 是布局值，不受 transform 影响，对应动画结束后的真实位置。
 */
export function documentOffsetTop(element: OffsetChainNode | null): number {
  let top = 0;
  let node = element;
  while (node) {
    top += node.offsetTop;
    node = node.offsetParent as OffsetChainNode | null;
  }
  return top;
}
