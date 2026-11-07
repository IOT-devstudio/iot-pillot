/**
 * 招新问答的路由 meta 键与内容域（#221，路径解耦见 #242）。
 *
 * 单独成文件而不是写在 routes.ts 里：routes.ts 会 import 三个视图组件，
 * 而任何只想读 meta 约定的地方（useForumScope、路由表单测）都不该因此
 * 把整个视图链和 Element Plus 一起拉进来——那会让纯逻辑单测被迫跑组件样式。
 * 契约极薄，放这里不分叉。
 */

/** 路由 meta 里标记内容域的键。 */
export const FORUM_SCOPE_META = "forumScope";

/**
 * 路由 meta 里声明该入口路径前缀的键（#242）。
 *
 * 同一内容域可以有多组前缀（/user/qa 与 /recruitment/qa），
 * 组件据此生成「返回列表」「编辑后跳转」，不会把人带出当前外壳。
 */
export const FORUM_BASE_META = "forumBase";

/** 内容域：招新问答 / 工作室内部论坛。 */
export type ForumScope = "recruitment" | "studio";
