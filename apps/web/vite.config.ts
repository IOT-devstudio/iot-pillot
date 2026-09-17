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
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      // 后端健康检查在根级 /health（不在 /api/ 下），需单独反代。
      "/health": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    environmentMatchGlobs: [
      ["src/views/AuthView.test.ts", "./src/views/AuthView.environment.ts"],
    ],
  },
});
