import type { RouteRecordRaw } from "vue-router";
import { ANY_SIGNED_IN, MEMBER_ONLY } from "@/router/route-specs";
const editor = () => import("./views/FormEditorView.vue");
export const formRoutes: RouteRecordRaw[] = [
  {
    path: "/forms",
    name: "forms",
    component: () => import("./views/FormsView.vue"),
    meta: { title: "表单管理", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/forms/new",
    name: "form-new",
    component: editor,
    meta: { title: "新建报名表", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/forms/:id/edit",
    name: "form-edit",
    component: editor,
    meta: { title: "编辑报名表", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/forms/:id/submissions",
    name: "form-submissions",
    component: () => import("./views/FormSubmissionsView.vue"),
    meta: { title: "表单提交记录", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/f/:shareToken",
    name: "form-share",
    component: () => import("./views/ShareFormView.vue"),
    meta: { title: "招新申请表", allowRoles: ANY_SIGNED_IN, signalShell: true },
  },
];
