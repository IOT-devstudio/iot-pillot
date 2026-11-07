import { beforeEach, describe, expect, it, vi } from "vitest";
import { createRenderer, defineComponent, h, nextTick } from "vue";

/**
 * 详情页的回答权限展示（#242 AC）。
 *
 * 三道门要一起成立，缺一条都算没做完：
 *   1. 普通用户在详情页**看不到回复框**，只看到一行说明；
 *   2. 管理员在用户端详情页仍然能回复（他是管理员）；
 *   3. 「编辑 / 删除 / 标记已解决」对普通用户**保持可见**——那些权限没有被收回。
 *
 * 断言的是「渲染出来的树里有没有那个东西」，不是内部布尔值：
 * 「禁用的输入框」在树里仍是输入框，而 AC 明确说不要那种形态。
 */

/** /me 的桩：角色决定回答权。 */
const profileState = vi.hoisted(() => ({ profile: null as null | Record<string, unknown> }));

vi.mock("@/modules/user-settings/profileIdentity", () => ({
  currentProfile: {
    get value() {
      return profileState.profile;
    },
  },
}));

/** 路由桩：内容域与路径都由 meta 决定，这里直接给用户端那组。 */
vi.mock("vue-router", () => ({
  useRoute: () => ({
    params: { id: "7" },
    meta: {
      title: "问答详情",
      forumScope: "recruitment",
      forumBase: "/user/qa",
    },
  }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

/** 接口桩：一条未解决的帖子 + 一条回复。 */
const api = vi.hoisted(() => ({
  fetchQaPost: vi.fn(),
  createQaReply: vi.fn(),
  deleteQaPost: vi.fn(),
  deleteQaReply: vi.fn(),
  resolveQaPost: vi.fn(),
}));

vi.mock("@/api/qa", () => api);

/** Element Plus 的命令式 API 在 node 环境里没有样式层，打桩即可。 */
vi.mock("element-plus", () => ({
  ElMessage: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
  ElButton: defineComponent({
    props: ["type", "size", "loading", "disabled", "plain"],
    setup: (_, { slots }) => () => h("button", { class: "stub-button" }, slots.default?.()),
  }),
}));

// 按需引入会把 EP 的 theme-chalk CSS 塞进模块图；node 环境解析不了 .css。
// 只关心渲染结构，样式模块一律打桩（与 SendMailDialog.test.ts 同一办法）。
vi.mock("element-plus/es/components/base/style/css", () => ({}));
vi.mock("element-plus/es/components/button/style/css", () => ({}));
vi.mock("element-plus/es/components/message/style/css", () => ({}));
vi.mock("element-plus/es/components/message-box/style/css", () => ({}));

/** 编辑器：只要它的存在与否，内容不参与断言。 */
vi.mock("../components/MarkdownEditor.vue", () => ({
  default: defineComponent({
    props: ["body", "maxLength", "disabled", "placeholder"],
    setup: () => () => h("div", { class: "stub-md-editor" }),
  }),
}));

/** 正文渲染牵涉 markdown-it 与图片组件，这里只关心结构。 */
vi.mock("@/components/markdown/MarkdownContent.vue", () => ({
  default: defineComponent({
    props: ["source"],
    setup: () => () => h("div", { class: "stub-md-body" }),
  }),
}));

import QaDetailView from "../views/QaDetailView.vue";

/* ---------------- 最小 renderer（项目没装 @vue/test-utils） ---------------- */

interface Node {
  type: string;
  props: Record<string, unknown>;
  children: Node[];
  parent: Node | null;
  text: string;
}

const node = (type: string, text = ""): Node => ({
  type,
  props: {},
  children: [],
  parent: null,
  text,
});

function detach(target: Node): void {
  const parent = target.parent;
  if (parent) parent.children.splice(parent.children.indexOf(target), 1);
  target.parent = null;
}

const renderer = createRenderer<Node, Node>({
  createElement: node,
  createText: (text) => node("text", text),
  createComment: (text) => node("comment", text),
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

/** 整棵树的文本，用于找说明文案。 */
function textOf(root: Node): string {
  let out = root.text;
  for (const child of root.children) out += `\n${textOf(child)}`;
  return out;
}

function findAll(root: Node, predicate: (n: Node) => boolean): Node[] {
  const found: Node[] = [];
  const visit = (target: Node): void => {
    if (predicate(target)) found.push(target);
    for (const child of target.children) visit(child);
  };
  visit(root);
  return found;
}

async function mount(): Promise<{ root: Node; unmount: () => void }> {
  const root = node("root");
  const app = renderer.createApp(
    defineComponent({ setup: () => () => h(QaDetailView) }),
  );
  app.mount(root);
  // onMounted 里发起的 load() 是异步的：等它 resolve 后的一轮渲染。
  for (let i = 0; i < 5; i += 1) {
    await Promise.resolve();
    await nextTick();
  }
  return { root, unmount: () => app.unmount() };
}

/** 作者 = 2，帖子本身的作者；另有一条别人写的回复。 */
const post = {
  id: 7,
  title: "怎么预约实验室？",
  body: "正文",
  author_id: 2,
  author_name: "张三",
  direction_code: "backend",
  is_resolved: false,
  created_at: "2026-11-01T10:00:00Z",
  edited_at: null,
  last_active_at: "2026-11-02T10:00:00Z",
  reply_count: 1,
  replies: [
    {
      id: 11,
      post_id: 7,
      body: "回复正文",
      author_id: 3,
      author_name: "李四",
      is_official: true,
      created_at: "2026-11-02T10:00:00Z",
      edited_at: null,
    },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  profileState.profile = null;
  api.fetchQaPost.mockResolvedValue(post);
});

describe("QaDetailView — 回答权限（#242）", () => {
  it("普通用户：看不到回复框，只看到一行说明", async () => {
    profileState.profile = { user_id: 2, username: "zhangsan", role: "member" };

    const view = await mount();
    try {
      // 没有编辑器，也就没有「禁用的输入框」这种半吊子形态
      expect(findAll(view.root, (n) => n.type === "div" && n.props.class === "stub-md-editor")).toHaveLength(0);
      // 说明文案在，且说清「谁能答 + 我还能做什么」
      const text = textOf(view.root);
      expect(text).toContain("只有管理员可以回答招新问答");
      expect(text).toContain("标记已解决");
    } finally {
      view.unmount();
    }
  });

  it("普通用户仍是作者：编辑 / 删除 / 标记已解决保持可见", async () => {
    profileState.profile = { user_id: 2, username: "zhangsan", role: "member" };

    const view = await mount();
    try {
      const labels = findAll(view.root, (n) => n.type === "button").map((n) =>
        textOf(n).trim(),
      );
      // 这三项权限没有被收回（AC 明确要求）
      expect(labels).toContain("编辑");
      expect(labels).toContain("删除");
      expect(labels).toContain("标记已解决");
      // 但「回复」按钮不该出现
      expect(labels).not.toContain("回复");
    } finally {
      view.unmount();
    }
  });

  it("管理员：回复框可用（用户端详情页也能回答）", async () => {
    profileState.profile = { user_id: 9, username: "admin", role: "admin" };

    const view = await mount();
    try {
      expect(findAll(view.root, (n) => n.type === "div" && n.props.class === "stub-md-editor")).toHaveLength(1);
      const labels = findAll(view.root, (n) => n.type === "button").map((n) =>
        textOf(n).trim(),
      );
      expect(labels).toContain("回复");
      // 有权限时不该再出那行限制说明
      expect(textOf(view.root)).not.toContain("只有管理员可以回答招新问答");
    } finally {
      view.unmount();
    }
  });

  it("管理员对自己没写过的帖子也有全部管理动作", async () => {
    profileState.profile = { user_id: 9, username: "admin", role: "admin" };

    const view = await mount();
    try {
      const labels = findAll(view.root, (n) => n.type === "button").map((n) =>
        textOf(n).trim(),
      );
      expect(labels).toContain("编辑");
      expect(labels).toContain("删除");
      expect(labels).toContain("标记已解决");
    } finally {
      view.unmount();
    }
  });
});
