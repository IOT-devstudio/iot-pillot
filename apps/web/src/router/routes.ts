import type { RouteRecordRaw } from "vue-router";
import AuthView from "@/views/AuthView.vue";
import Home from "@/views/Home.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import { placeholderRouteSpecs } from "./route-specs";

export const routes: RouteRecordRaw[] = [
  { path: "/", name: "home", component: Home },
  { path: "/login", name: "login", component: AuthView, meta: { title: "登录" } },
  ...placeholderRouteSpecs
    .filter(({ path }) => path !== "/login")
    .map(({ path, name, title }) => ({
      path,
      name,
      component: PlaceholderView,
      meta: { title },
    })),
];
