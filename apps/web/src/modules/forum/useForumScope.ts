/**
 * 内容域感知（#221）。
 *
 * 招新问答与内部论坛复用同一批组件，组件不该自己去解析路径判断「我是哪个域」——
 * 那样每加一个页面都要改一遍判断。这里统一从路由 meta 读，组件只问这个 composable。
 *
 * 为什么放在 meta 而不是从 URL 前缀推断：路由表是单一真源，
 * 以后改前缀（比如把 /studio/forum 挪进别的管理台）不用动任何组件。
 */
import { computed, type ComputedRef } from "vue";
import { useRoute } from "vue-router";

import { FORUM_SCOPE_META, type ForumScope } from "./routes";

/** 内容域相关的派生信息。 */
export interface ForumScopeInfo {
  /** 当前内容域。 */
  scope: ComputedRef<ForumScope>;
  /** 是否内部论坛（studio）。 */
  isStudio: ComputedRef<boolean>;
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
}

/** 各内容域的路径前缀与文案。 */
const SCOPE_CONFIG: Record<ForumScope, { base: string; label: string }> = {
  recruitment: { base: "/user/qa", label: "招新问答" },
  studio: { base: "/studio/forum", label: "内部论坛" },
};

/**
 * 读当前路由的内容域。
 *
 * 缺省回落 recruitment：只有页面忘了在 meta 里标 scope 时才会走到这里，
 * 那时按对外的招新问答处理比按内部论坛处理更安全（不会意外暴露内部内容）。
 */
export function useForumScope(): ForumScopeInfo {
  const route = useRoute();

  const scope = computed<ForumScope>(() =>
    route.meta?.[FORUM_SCOPE_META] === "studio" ? "studio" : "recruitment",
  );
  const base = computed(() => SCOPE_CONFIG[scope.value].base);

  return {
    scope,
    isStudio: computed(() => scope.value === "studio"),
    listPath: base,
    newPath: computed(() => `${base.value}/new`),
    detailPath: (id: number) => `${base.value}/${id}`,
    editPath: (id: number) => `${base.value}/${id}/edit`,
    label: computed(() => SCOPE_CONFIG[scope.value].label),
  };
}
