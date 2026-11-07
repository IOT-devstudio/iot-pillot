/**
 * 需要在 node 里真正挂载 `.vue` 组件的测试环境。
 *
 * `transformMode: "web"` 是这里的全部意义：vitest 在 node 环境下默认用
 * **服务端**编译 SFC，产出的 render 函数会走 SSR 分支，runtime 期待
 * `@vue/server-renderer` 的上下文（如 `Symbol(v-scx)`），纯客户端
 * `createRenderer` 挂载会直接抛 "Cannot read properties of undefined
 * (reading 'modules')"。加上这个标记后组件按浏览器端编译，才能在自定义
 * renderer 里正常 mount / 触发更新。
 *
 * 文件名沿用 AuthView.environment.ts（它最先需要这套设置），
 * 通过 `test.environmentMatchGlobs` 复用到其他需要挂载组件的测试。
 */
import type { Environment } from "vitest/environments";

const environment: Environment = {
  name: "auth-view-renderer",
  transformMode: "web",
  setup(global) {
    const previousDocument = global.document;
    global.document = {
      activeElement: null,
      createElement: () => ({}),
    };

    return {
      teardown() {
        global.document = previousDocument;
      },
    };
  },
};

export default environment;
