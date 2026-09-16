import { createRenderer, nextTick, type Component } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthRequestError, type AuthSession } from "@/api/auth";
import AuthView from "./AuthView.vue";

const mocks = vi.hoisted(() => ({
  login: vi.fn(),
  register: vi.fn(),
  saveAuthSession: vi.fn(),
  push: vi.fn(),
}));

vi.mock("@/api/auth", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api/auth")>();
  return {
    ...actual,
    login: mocks.login,
    register: mocks.register,
  };
});

vi.mock("@/auth/session", () => ({
  saveAuthSession: mocks.saveAuthSession,
}));

vi.mock("vue-router", () => ({
  useRouter: () => ({ push: mocks.push }),
}));

type TestNode = TestElement | TestText;
type TestEvent = { target: TestElement };
type EventHandler = (event: TestEvent) => void;

interface TestText {
  kind: "text" | "comment";
  text: string;
  parent: TestElement | null;
}

interface TestElement {
  kind: "element";
  type: string;
  props: Record<string, unknown>;
  children: TestNode[];
  text: string;
  value: unknown;
  parent: TestElement | null;
  listeners: Map<string, Set<EventHandler>>;
  addEventListener: (name: string, handler: EventHandler) => void;
}

function createElement(type: string): TestElement {
  const listeners = new Map<string, Set<EventHandler>>();
  const element: TestElement = {
    kind: "element",
    type,
    props: {},
    children: [],
    text: "",
    value: "",
    parent: null,
    listeners,
    addEventListener(name, handler) {
      const handlers = listeners.get(name) ?? new Set<EventHandler>();
      handlers.add(handler);
      listeners.set(name, handlers);
    },
  };
  return element;
}

function detach(node: TestNode): void {
  const parent = node.parent;
  if (!parent) {
    return;
  }
  const index = parent.children.indexOf(node);
  if (index >= 0) {
    parent.children.splice(index, 1);
  }
  node.parent = null;
}

const renderer = createRenderer<TestNode, TestElement>({
  patchProp(element, key, _previousValue, nextValue) {
    element.props[key] =
      key === "disabled" ? nextValue === "" || nextValue === true : nextValue;
  },
  insert(node, parent, anchor) {
    detach(node);
    const index = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(index >= 0 ? index : parent.children.length, 0, node);
    node.parent = parent;
  },
  remove: detach,
  createElement,
  createText: (text) => ({ kind: "text", text, parent: null }),
  createComment: (text) => ({ kind: "comment", text, parent: null }),
  setText(node, text) {
    node.text = text;
  },
  setElementText(element, text) {
    element.text = text;
    element.children = [];
  },
  parentNode: (node) => node.parent,
  nextSibling(node) {
    const parent = node.parent;
    if (!parent) {
      return null;
    }
    const index = parent.children.indexOf(node);
    return parent.children[index + 1] ?? null;
  },
});

function mount(component: Component = AuthView): {
  root: TestElement;
  unmount: () => void;
} {
  const root = createElement("root");
  const app = renderer.createApp(component);
  app.mount(root);
  return { root, unmount: () => app.unmount() };
}

function allElements(root: TestElement): TestElement[] {
  const result: TestElement[] = [];
  const visit = (node: TestNode): void => {
    if (node.kind !== "element") {
      return;
    }
    result.push(node);
    node.children.forEach(visit);
  };
  visit(root);
  return result;
}

function visibleText(node: TestNode): string {
  if (node.kind !== "element") {
    return node.kind === "comment" ? "" : node.text;
  }
  return [node.text, ...node.children.map(visibleText)].join("");
}

function findElement(
  root: TestElement,
  predicate: (element: TestElement) => boolean,
): TestElement {
  const element = allElements(root).find(predicate);
  if (!element) {
    throw new Error("Expected rendered element was not found");
  }
  return element;
}

function findById(root: TestElement, id: string): TestElement {
  return findElement(root, (element) => element.props.id === id);
}

function findButton(root: TestElement, label: string): TestElement {
  return findElement(
    root,
    (element) => element.type === "button" && visibleText(element).trim() === label,
  );
}

async function click(element: TestElement): Promise<void> {
  const handler = element.props.onClick as (() => void) | undefined;
  if (!handler) {
    throw new Error("Expected button to have a click handler");
  }
  handler();
  await nextTick();
}

async function input(element: TestElement, value: string): Promise<void> {
  element.value = value;
  element.listeners.get("input")?.forEach((handler) => handler({ target: element }));
  await nextTick();
}

async function submit(root: TestElement): Promise<void> {
  const form = findElement(root, (element) => element.type === "form");
  const handler = form.props.onSubmit as
    | ((event: { preventDefault: () => void }) => Promise<void>)
    | undefined;
  if (!handler) {
    throw new Error("Expected form to have a submit handler");
  }
  await handler({ preventDefault: vi.fn() });
}

function deferred<T>(): {
  promise: Promise<T>;
  resolve: (value: T) => void;
} {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((promiseResolve) => {
    resolve = promiseResolve;
  });
  return { promise, resolve };
}

const session: AuthSession = {
  access_token: "access-token",
  refresh_token: "refresh-token",
  user_id: 7,
};

describe("AuthView", () => {
  let unmount: (() => void) | undefined;

  beforeEach(() => {
    mocks.login.mockResolvedValue(session);
    mocks.register.mockResolvedValue(session);
    mocks.push.mockResolvedValue(undefined);
  });

  afterEach(() => {
    unmount?.();
    unmount = undefined;
    vi.clearAllMocks();
  });

  it("切换到注册模式后渲染注册字段和禁用的验证码按钮", async () => {
    const mounted = mount();
    unmount = mounted.unmount;

    expect(findById(mounted.root, "username")).toBeDefined();
    await click(findButton(mounted.root, "注册"));

    expect(findById(mounted.root, "name")).toBeDefined();
    expect(findById(mounted.root, "email")).toBeDefined();
    expect(findById(mounted.root, "confirm-password")).toBeDefined();
    expect(findById(mounted.root, "code")).toBeDefined();
    expect(findButton(mounted.root, "获取验证码 · 暂未开放").props.disabled).toBe(true);
  });

  it("空登录提交显示字段错误且不发起请求", async () => {
    const mounted = mount();
    unmount = mounted.unmount;

    await submit(mounted.root);
    await nextTick();

    expect(visibleText(mounted.root)).toContain("请输入用户名");
    expect(visibleText(mounted.root)).toContain("请输入密码");
    expect(findById(mounted.root, "username").props["aria-invalid"]).toBe(true);
    expect(mocks.login).not.toHaveBeenCalled();
    expect(mocks.register).not.toHaveBeenCalled();
  });

  it("API 失败时显示后端错误消息", async () => {
    mocks.login.mockRejectedValueOnce(
      new AuthRequestError("用户名或密码错误", 401, 1001),
    );
    const mounted = mount();
    unmount = mounted.unmount;
    await input(findById(mounted.root, "username"), "researcher");
    await input(findById(mounted.root, "login-password"), "secret12");

    await submit(mounted.root);
    await nextTick();

    const alert = findElement(
      mounted.root,
      (element) => element.props.role === "alert",
    );
    expect(visibleText(alert)).toContain("用户名或密码错误");
  });

  it("提交期间锁定按钮，成功后保存会话并跳转", async () => {
    const pendingLogin = deferred<AuthSession>();
    mocks.login.mockReturnValueOnce(pendingLogin.promise);
    const mounted = mount();
    unmount = mounted.unmount;
    await input(findById(mounted.root, "username"), "researcher");
    await input(findById(mounted.root, "login-password"), "secret12");

    const submission = submit(mounted.root);
    await nextTick();

    expect(findButton(mounted.root, "正在校验…").props.disabled).toBe(true);

    pendingLogin.resolve(session);
    await submission;
    await nextTick();

    expect(mocks.login).toHaveBeenCalledWith({
      username: "researcher",
      password: "secret12",
    });
    expect(mocks.saveAuthSession).toHaveBeenCalledWith(session);
    expect(mocks.push).toHaveBeenCalledWith("/dashboard");
    expect(findButton(mounted.root, "进入工作台").props.disabled).toBe(false);
  });

  it("面向用户的辅助和字段文案使用中文", () => {
    const mounted = mount();
    unmount = mounted.unmount;
    const text = visibleText(mounted.root);

    expect(text).toContain("春季招新");
    expect(text).toContain("全栈工作室");
    expect(text).toContain("招新入口");
    expect(text).toContain("把想法做成");
    expect(text).toContain("用户名");
    expect(text).not.toMatch(
      /RECRUIT|FIELD NOTES|NODE|INPUT ARRAY|SIGNAL|SYSTEM STUDY|Research Manual|NO\.|ACCESS PROTOCOL|USERNAME|PASSWORD|\bNAME\b|EMAIL|CONFIRM|VERIFY CODE|传感器|云端|节点|信号|输入阵列/,
    );
  });
});
