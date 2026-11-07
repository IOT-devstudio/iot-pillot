import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import AutoImport from "unplugin-auto-import/vite";
import Components from "unplugin-vue-components/vite";
import { ElementPlusResolver } from "unplugin-vue-components/resolvers";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({ resolvers: [ElementPlusResolver()] }),
    Components({ resolvers: [ElementPlusResolver()] }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://113.44.193.72",
        changeOrigin: true,
      },
      "/health": {
        target: "http://113.44.193.72",
        changeOrigin: true,
      },
    },
  },
  test: {
    environmentMatchGlobs: [
      ["src/views/AuthView.test.ts", "./src/views/AuthView.environment.ts"],
      ["src/modules/mail/components/SendMailDialog.test.ts", "./src/views/AuthView.environment.ts"],
      // 需要真正 mount `.vue` 组件的测试（#242 图片渲染）：node 环境默认按 SSR
      // 编译 SFC，客户端 renderer 挂不上去，必须走 transformMode: "web"。
      ["src/components/markdown/MarkdownImage.test.ts", "./src/views/AuthView.environment.ts"],
      ["src/modules/forum/views/ForumViews.test.ts", "./src/views/AuthView.environment.ts"],
    ],
  },
});
