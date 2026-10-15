import type { RouteRecordRaw } from "vue-router";
import { MEMBER_ONLY, ANY_SIGNED_IN } from "@/router/route-specs";
import ApplicationsView from "./views/ApplicationsView.vue";
import InvitationsView from "./views/InvitationsView.vue";
import CandidatesView from "./views/CandidatesView.vue";
import CandidateDetailView from "./views/CandidateDetailView.vue";
import MyInvitationsView from "./views/MyInvitationsView.vue";
export const recruitmentRoutes: RouteRecordRaw[] = [
  { path: "/recruitment", redirect: "/recruitment/applications" },
  { path: "/recruitment/prospects", redirect: "/recruitment/applications" },
  { path: "/recruitment/prospects/:id", redirect: "/recruitment/applications" },
  {
    path: "/recruitment/applications",
    name: "recruitment-applications",
    component: ApplicationsView,
    meta: { title: "申请审核", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/recruitment/invitations",
    name: "recruitment-invitations",
    component: InvitationsView,
    meta: { title: "邀请管理", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/recruitment/candidates",
    name: "recruitment-candidates",
    component: CandidatesView,
    meta: { title: "候选人处理", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/recruitment/candidates/:id",
    name: "recruitment-candidate-detail",
    component: CandidateDetailView,
    meta: { title: "候选人详情", allowRoles: MEMBER_ONLY, adminShell: true },
  },
  {
    path: "/user/invitations",
    name: "user-invitations",
    component: MyInvitationsView,
    meta: { title: "邀请与招新结果", allowRoles: ANY_SIGNED_IN },
  },
];
