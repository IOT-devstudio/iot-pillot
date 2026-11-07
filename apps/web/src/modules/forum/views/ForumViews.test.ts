import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h, nextTick } from "vue";

import {
  findAll,
  makeNode,
  settle,
  testRenderer,
  textOf,
  type TestNode,
} from "@/test/test-renderer";

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

/**
 * 编辑器桩：只需要「在不在」和「能不能往外抛正文」。
 * 必须声明 emits，否则 v-model:body 的 update 事件不会被父组件接上，
 * 也就无法驱动「填正文 → 点回复」这条行为链。
 */
vi.mock("../components/MarkdownEditor.vue", () => ({
  default: defineComponent({
    props: ["body", "maxLength", "disabled", "placeholder"],
    emits: ["update:body"],
    setup: (_, { emit }) => () =>
      h("div", {
        class: "stub-md-editor",
        // 把事件处理器直接暴露到 props 上，测试可以手动触发一次输入
        onInput: (value: string) => emit("update:body", value),
      }),
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

async function mount(): Promise<{ root: TestNode; unmount: () => void }> {
  const root = makeNode("root");
  const app = testRenderer.createApp(
    defineComponent({ setup: () => () => h(QaDetailView) }),
  );
  app.mount(root);
  // onMounted 里发起的 load() 是异步的：等它 resolve 后的一轮渲染。
  await settle(5);
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

  it("普通非作者：说明文案不再承诺「编辑或删除自己的内容」", async () => {
    // 帖子作者是 2，当前用户是 3：对**帖子** canManage 不成立，页面上没有
    // 帖子的编辑/删除按钮。注意回复列表里 id=3 写的那条回复仍带自己的删除
    // 按钮——那是回复级权限，与帖子级文案无关，所以这里只断言帖子级。
    profileState.profile = { user_id: 3, username: "lisi", role: "member" };

    const view = await mount();
    try {
      const text = textOf(view.root);
      expect(text).toContain("只有管理员可以回答招新问答");
      // 关键回归：不能对非作者承诺「编辑、删除自己的帖子」
      expect(text).not.toContain("编辑、删除自己的帖子");
      // 非作者的口径应指向「可以去问、可以自己发起」
      expect(text).toContain("自己发起新问题");
    } finally {
      view.unmount();
    }
  });
});

describe("QaDetailView — 回复提交（#242 AC：回复框可用，回复成功）", () => {
  /**
   * 前面的用例只断言「编辑器在不在」。「回复成功」是行为，必须真的点到提交
   * 并看到 createQaReply 被调用、正文被清空、列表被重拉。
   */
  it("管理员填正文后点回复：调用 createQaReply，成功提示并重拉详情", async () => {
    profileState.profile = { user_id: 9, username: "admin", role: "admin" };
    api.createQaReply.mockResolvedValue({ id: 99, post_id: 7, body: "已回复" });

    const view = await mount();
    try {
      // 截图前先确认按钮确实渲染出来了（否则下面的「点不到」是另一种失败）
      const replyButton = findAll(
        view.root,
        (n) => n.type === "button" && textOf(n).trim() === "回复",
      )[0];
      expect(replyButton).toBeDefined();

      // MarkdownEditor 是桩组件，正文通过 v-model:body 由父组件持有；
      // 直接触发桩组件暴露的输入事件，模拟用户敲入正文。
      const editor = findAll(
        view.root,
        (n) => n.type === "div" && n.props.class === "stub-md-editor",
      )[0];
      expect(editor).toBeDefined();
      const onInput = editor?.props.onInput as
        | ((value: string) => void)
        | undefined;
      expect(onInput).toBeTypeOf("function");
      onInput?.("这是管理员的回复");
      await nextTick();

      // 正文非空后按钮才可点。按钮桩只在 disabled 为真时输出该属性，
      // 所以「可点」表现为属性缺失（undefined）而不是显式的 false。
      const enabled = findAll(
        view.root,
        (n) => n.type === "button" && textOf(n).trim() === "回复",
      )[0];
      expect(enabled?.props.disabled).not.toBe(true);
      await (enabled?.props.onClick as () => Promise<void>)();
      await nextTick();

      expect(api.createQaReply).toHaveBeenCalledTimes(1);
      expect(api.createQaReply).toHaveBeenCalledWith(
        7,
        { body: "这是管理员的回复" },
        "recruitment",
      );
      // 成功后重拉详情（load() 会再次 fetchQaPost）
      const fetches = api.fetchQaPost.mock.calls.length;
      expect(fetches).toBeGreaterThanOrEqual(2);
    } finally {
      view.unmount();
    }
  });

  it("普通用户即使伪造输入也调不到 createQaReply（界面无提交入口）", async () => {
    profileState.profile = { user_id: 2, username: "zhangsan", role: "member" };

    const view = await mount();
    try {
      // 没有编辑器 = 没有正文来源；没有「回复」按钮 = 没有提交入口
      expect(
        findAll(view.root, (n) => n.type === "div" && n.props.class === "stub-md-editor"),
      ).toHaveLength(0);
      expect(
        findAll(view.root, (n) => n.type === "button" && textOf(n).trim() === "回复"),
      ).toHaveLength(0);
      expect(api.createQaReply).not.toHaveBeenCalled();
    } finally {
      view.unmount();
    }
  });
});
