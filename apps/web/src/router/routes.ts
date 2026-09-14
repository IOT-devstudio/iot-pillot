import type { RouteRecordRaw } from "vue-router";
import Home from "@/views/Home.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import { placeholderRouteSpecs } from "./route-specs";

export const routes: RouteRecordRaw[] = [
  { path: "/", name: "home", component: Home },
  ...placeholderRouteSpecs.map(({ path, name, title }) => ({
    path,
    name,
    component: PlaceholderView,
    meta: { title },
  })),
];
