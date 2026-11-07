/**
 * 内容域感知（#221，路径解耦见 #242）。
 *
 * 招新问答与内部论坛复用同一批组件，组件不该自己去解析路径判断「我是哪个域」——
 * 那样每加一个页面都要改一遍判断。这里统一从路由 meta 读，组件只问这个 composable。
 *
 * 为什么放在 meta 而不是从 URL 前缀推断：路由表是单一真源，
 * 以后改前缀（比如把 /studio/forum 挪进别的管理台）不用动任何组件。
 *
 * #242 的关键修正：**内容域不再等于路径前缀**。同一个 recruitment 域现在有两组
 * 前缀——用户端 `/user/qa` 与管理端 `/recruitment/qa`。如果还按「域 → 前缀」硬编码，
 * 管理端详情页的「返回列表」「编辑后跳转」「发布后跳转」会把管理员送回 /user/qa，
 * 直接跳出管理台外壳。因此路径改成从**当前路由的 meta** 派生：路由显式声明
 * `forumBase`，缺省才回落到该域的历史前缀。新增一个入口只需在路由表写一个字段。
 */
import { computed, type ComputedRef } from "vue";
import { useRoute } from "vue-router";

import { FORUM_BASE_META, FORUM_SCOPE_META, type ForumScope } from "./scope-meta";

// 常量从 scope-meta 读而不是 routes.ts：routes.ts 会 import 视图组件，
// 本模块（以及它的单测）不该因此把整条组件链和 Element Plus 拖进来。
export type { ForumScope };

/** 内容域相关的派生信息。 */
export interface ForumScopeInfo {
  /** 当前内容域。 */
  scope: ComputedRef<ForumScope>;
  /** 是否内部论坛（studio）。 */
  isStudio: ComputedRef<boolean>;
  /** 是否挂在管理台外壳里（决定文案与「提问」按钮可见性）。 */
  isAdminEntry: ComputedRef<boolean>;
  /** 列表页路径。 */
  listPath: ComputedRef<string>;
  /** 新建页路径。 */
  newPath: ComputedRef<string>;
  /** 详情页路径。 */
  detailPath: (id: number) => string;
  /** 编辑页路径。 */
  editPath: (id: number) => string;
  /** 页面标题用的域名词。 */
  label: ComputedRef<string>;
  /** 列表页说明文案。 */
  description: ComputedRef<string>;
  /**
   * 当前入口是否提供「提问」入口（#242）。
   *
   * 管理端的招新问答入口职责是「回答问题」，不发帖；管理员要发帖走用户端。
   * 只控制按钮可见性——发帖权限本身始终由后端判定。
   */
  canCompose: ComputedRef<boolean>;
}

/** 各内容域的文案，以及 meta 未声明 forumBase 时的回落前缀。 */
const SCOPE_CONFIG: Record<ForumScope, { fallbackBase: string; label: string }> = {
  recruitment: { fallbackBase: "/user/qa", label: "招新问答" },
  studio: { fallbackBase: "/studio/forum", label: "内部论坛" },
};

/**
 * 列表页说明。三个入口各说各的职责，而不是一套文案硬套：
 * 管理端入口是「回答」，用户端入口是「提问与查阅」。
 */
function descriptionOf(scope: ForumScope, isAdminEntry: boolean): string {
  if (scope === "studio") return "工作室内部讨论。仅管理员可见。";
  return isAdminEntry
    ? "成员与报名者提出的问题。在这里回答、标记已解决。发新帖请到用户端。"
    : "成员与报名者整理的常见问题。按方向筛选，点开查看完整问答。";
}

/**
 * 读当前路由的内容域与路径。
 *
 * 域缺省回落 recruitment：只有页面忘了在 meta 里标 scope 时才会走到这里，
 * 那时按对外的招新问答处理比按内部论坛处理更安全（不会意外暴露内部内容）。
 */
export function useForumScope(): ForumScopeInfo {
  const route = useRoute();

  const scope = computed<ForumScope>(() =>
    route.meta?.[FORUM_SCOPE_META] === "studio" ? "studio" : "recruitment",
  );
  const isAdminEntry = computed(() => route.meta?.adminShell === true);

  /**
   * 列表路径，也是新建 / 详情 / 编辑的共同父级。
   *
   * 优先读路由 meta 显式声明的 forumBase（路由表是唯一真源），
   * 未声明才回落到该域的历史前缀——漏标的页面行为与 #242 之前一致。
   */
  const base = computed(() => {
    const declared = route.meta?.[FORUM_BASE_META];
    if (typeof declared === "string" && declared !== "") return declared;
    return SCOPE_CONFIG[scope.value].fallbackBase;
  });

  return {
    scope,
    isStudio: computed(() => scope.value === "studio"),
    isAdminEntry,
    listPath: base,
    newPath: computed(() => `${base.value}/new`),
    detailPath: (id: number) => `${base.value}/${id}`,
    editPath: (id: number) => `${base.value}/${id}/edit`,
    label: computed(() => SCOPE_CONFIG[scope.value].label),
    description: computed(() => descriptionOf(scope.value, isAdminEntry.value)),
    // 内部论坛在管理台里也要发帖（它就是给工作室成员用的），
    // 只有「管理台的招新问答入口」这一种组合才收起提问按钮。
    canCompose: computed(() => !(isAdminEntry.value && scope.value === "recruitment")),
  };
}
