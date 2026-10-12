import { createRenderer, defineComponent, h, nextTick, reactive } from "vue";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SendMailDialog from "./SendMailDialog.vue";

const api = vi.hoisted(() => ({ email: vi.fn(), user: vi.fn(), bulk: vi.fn() }));

// The renderer tests component state and events without loading browser CSS.
vi.mock("element-plus/es/components/base/style/css", () => ({}));
vi.mock("element-plus/es/components/dialog/style/css", () => ({}));
vi.mock("element-plus/es/components/button/style/css", () => ({}));
vi.mock("element-plus/es/components/input/style/css", () => ({}));
vi.mock("element-plus/es/components/select/style/css", () => ({}));
vi.mock("element-plus/es/components/option/style/css", () => ({}));
vi.mock("element-plus/es/components/radio-group/style/css", () => ({}));
vi.mock("element-plus/es/components/radio-button/style/css", () => ({}));
vi.mock("element-plus/es/components/message/style/css", () => ({}));

vi.mock("@/api/mail", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/api/mail")>(),
  sendMailToEmail: api.email,
  sendMailToUser: api.user,
  sendMailBulk: api.bulk,
}));

const elementPlusMock = vi.hoisted(() => async () => {
  const { defineComponent, h } = await import("vue");
  const input = (tag: string) => defineComponent({
    props: ["modelValue"],
    emits: ["update:modelValue"],
    setup: (props, { emit, slots }) => () => h(tag, {
      value: props.modelValue,
      onInput: (value: unknown) => emit("update:modelValue", value),
    }, slots.default?.()),
  });
  const container = defineComponent({
    setup: (_, { slots }) => () => h("container", [slots.default?.(), slots.footer?.()]),
  });
  return {
    ElMessage: { success: vi.fn(), warning: vi.fn() },
    ElDialog: container,
    ElButton: defineComponent({
      props: ["disabled", "loading"],
      setup: (props, { slots }) => () => h("button", { disabled: props.disabled }, slots.default?.()),
    }),
    ElInput: input("email-input"),
    ElSelect: input("user-select"),
    ElOption: container,
    ElRadioGroup: input("mode-select"),
    ElRadioButton: container,
  };
});
vi.mock("element-plus", elementPlusMock);
vi.mock("element-plus/es", elementPlusMock);

interface Node {
  type: string;
  props: Record<string, unknown>;
  children: Node[];
  parent: Node | null;
  text: string;
}
const node = (type: string, text = ""): Node => ({ type, props: {}, children: [], parent: null, text });
function detach(target: Node): void {
  const parent = target.parent;
  if (parent) parent.children.splice(parent.children.indexOf(target), 1);
  target.parent = null;
}
const renderer = createRenderer<Node, Node>({
  createElement: node,
  createText: (text) => node("text", text),
  createComment: (text) => node("comment", text),
  setText: (target, text) => { target.text = text; },
  setElementText: (target, text) => {
    target.text = text;
    for (const child of target.children) child.parent = null;
    target.children = [];
  },
  patchProp: (target, key, _old, value) => { target.props[key] = value; },
  insert(target, parent, anchor) {
    detach(target);
    target.parent = parent;
    const index = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(index < 0 ? parent.children.length : index, 0, target);
  },
  remove: detach,
  parentNode: (target) => target.parent,
  nextSibling: (target) => target.parent?.children[target.parent.children.indexOf(target) + 1] ?? null,
});

function find(root: Node, predicate: (target: Node) => boolean): Node {
  const visit = (target: Node): Node | undefined => {
    if (predicate(target)) return target;
    for (const child of target.children) {
      const result = visit(child);
      if (result) return result;
    }
  };
  const result = visit(root);
  if (!result) throw new Error("Rendered control not found");
  return result;
}

async function mount(defaultEmail = "") {
  const props = reactive({ visible: false, defaultEmail });
  const root = node("root");
  const app = renderer.createApp(defineComponent({
    setup: () => () => h(SendMailDialog, {
      ...props,
      template: {
        id: 7,
        name: "通知",
        type: "offer",
        title: "通知",
        mail_model: "正文",
        mail_example: "正文",
        variables: [],
      },
      recipients: [{ user_id: 2, name: "张三" }],
    }),
  }));
  app.mount(root);
  props.visible = true;
  await nextTick();
  const input = async (type: string, value: unknown) => {
    const control = find(root, (target) => target.type === type);
    (control.props.onInput as (value: unknown) => void)(value);
    await nextTick();
  };
  const sendButton = () => find(root, (target) => target.type === "button" && target.props.type === "primary");
  return { props, root, input, sendButton, unmount: () => app.unmount() };
}

beforeEach(() => {
  vi.clearAllMocks();
  api.email.mockResolvedValue({ to_user_id: 0, to_email: "test@example.com", title: "通知" });
});

describe("SendMailDialog email recipient", () => {
  it("enables sending when the email input changes and sends the trimmed address", async () => {
    const view = await mount();
    try {
      await view.input("mode-select", "email");
      expect(view.sendButton().props.disabled).toBe(true);
      await view.input("email-input", "  test@example.com  ");
      expect(view.sendButton().props.disabled).toBe(false);
      await (view.sendButton().props.onClick as () => Promise<void>)();
      expect(api.email).toHaveBeenCalledWith({ template_id: 7, email: "test@example.com", vars: {} });
      await view.input("email-input", "   ");
      expect(view.sendButton().props.disabled).toBe(true);
      expect(api.user).not.toHaveBeenCalled();
      expect(api.bulk).not.toHaveBeenCalled();
    } finally { view.unmount(); }
  });

  it("restores the default address on reopening and preserves it across mode changes", async () => {
    const view = await mount("default@example.com");
    try {
      await view.input("mode-select", "email");
      expect(view.sendButton().props.disabled).toBe(false);
      await view.input("email-input", "edited@example.com");
      await view.input("mode-select", "user");
      await view.input("user-select", 2);
      expect(view.sendButton().props.disabled).toBe(false);
      await view.input("mode-select", "email");
      expect(find(view.root, (target) => target.type === "email-input").props.value).toBe("edited@example.com");
      view.props.visible = false;
      await nextTick();
      view.props.visible = true;
      await nextTick();
      expect(view.sendButton().props.disabled).toBe(true);
      await view.input("mode-select", "email");
      expect(find(view.root, (target) => target.type === "email-input").props.value).toBe("default@example.com");
      expect(view.sendButton().props.disabled).toBe(false);
    } finally { view.unmount(); }
  });
});
