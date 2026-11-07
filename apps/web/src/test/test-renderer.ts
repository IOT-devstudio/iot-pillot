import { createRenderer } from "vue";

/**
 * 测试用的最小 renderer（项目没装 @vue/test-utils）。
 *
 * 为什么需要它：本项目所有需要真挂载 `.vue` 组件的测试都用自定义 renderer
 * 记录 VNode 树，而不是 jsdom。node 环境没有 DOM，`createRenderer` 的宿主
 * 适配器可以换成任意对象——这样断言的是「渲染出了哪些节点」，不依赖浏览器。
 *
 * 既有的 AuthView.test.ts / SendMailDialog.test.ts 各有一套带额外能力
 * （事件监听、value 读写）的实现，它们服务各自的交互测试；这里只收拢
 * 「只需要一棵可遍历的树」这一种最简用法，避免每加一个组件测试就复制一遍。
 */

export interface TestNode {
  type: string;
  props: Record<string, unknown>;
  children: TestNode[];
  parent: TestNode | null;
  text: string;
}

export const makeNode = (type: string, text = ""): TestNode => ({
  type,
  props: {},
  children: [],
  parent: null,
  text,
});

function detach(target: TestNode): void {
  const parent = target.parent;
  if (parent) parent.children.splice(parent.children.indexOf(target), 1);
  target.parent = null;
}

export const testRenderer = createRenderer<TestNode, TestNode>({
  createElement: makeNode,
  createText: (text) => makeNode("text", text),
  createComment: (text) => makeNode("comment", text),
  setText: (target, text) => {
    target.text = text;
  },
  setElementText: (target, text) => {
    target.text = text;
    for (const child of target.children) child.parent = null;
    target.children = [];
  },
  patchProp: (target, key, _old, value) => {
    target.props[key] = value;
  },
  insert(target, parent, anchor) {
    detach(target);
    target.parent = parent;
    const index = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(index < 0 ? parent.children.length : index, 0, target);
  },
  remove: detach,
  parentNode: (target) => target.parent,
  nextSibling: (target) =>
    target.parent?.children[target.parent.children.indexOf(target) + 1] ?? null,
});

/** 整棵树的文本，用于断言「页面里出现了某句话」。 */
export function textOf(root: TestNode): string {
  let out = root.text;
  for (const child of root.children) out += `\n${textOf(child)}`;
  return out;
}

/** 按条件收集节点（深度优先，保留文档顺序）。 */
export function findAll(
  root: TestNode,
  predicate: (node: TestNode) => boolean,
): TestNode[] {
  const found: TestNode[] = [];
  const visit = (target: TestNode): void => {
    if (predicate(target)) found.push(target);
    for (const child of target.children) visit(child);
  };
  visit(root);
  return found;
}

/** 等异步状态与重新渲染落定。默认几轮足够覆盖「fetch resolve → 重渲染」。 */
export async function settle(rounds = 4): Promise<void> {
  const { nextTick } = await import("vue");
  for (let i = 0; i < rounds; i += 1) {
    await Promise.resolve();
    await nextTick();
  }
}
