/**
 * home 模块（用户侧）界面开发用的本地假数据。
 *
 * ⚠️ 这里**全部是 fixture，不是联调数据**。按 CLAUDE.md「M0」的约定：
 * 后端接口未交付时可以用与最终 DTO 一致的本地 fixture 推进界面，
 * 但 fixture 只能服务界面开发，**不能当作接口已经打通**。
 *
 * 后端接口就绪后这个文件应当整体删除，改为在页面里拉真实数据。
 */
import type { CurrentUser } from "@/api/auth";
import type { ID } from "@iot-pillot/shared-types";

/**
 * 当前登录用户（假数据）。
 *
 * 类型直接复用 `api/auth.ts` 的 `CurrentUser`，不另立门户——这样后端
 * `GET /api/v1/me` 的字段一旦变化，这里会在 typecheck 阶段报错，
 * 而不是悄悄漂移（M0 验收标准要求「类型检查能发现字段漂移」）。
 *
 * 两点如实说明：
 *   - **没有邮箱**。`CurrentUser` 只有 `username`，后端 `domain.User` 目前
 *     也没有 email 字段，所以界面不摆一个编造的邮箱，等后端补齐再加。
 *   - `user_id` 是 **number**，不是 shared-types 里通用 `ID` 的 UUID 字符串。
 *     这是当前后端的真实形状，不要混用。
 */
export const DEMO_CURRENT_USER: CurrentUser = {
  user_id: 1,
  username: "chenxinyu",
  role: "member",
};

/**
 * 招新方向。
 *
 * shared-types 里**没有**对应类型：最接近的 `ProspectStatus`
 * （pending / invited / accepted / rejected）描述的是**某个人**走到哪一步了，
 * 不是「这个方向还开不开放报名」，套上去会误导后来人。所以这是 home
 * 模块的前端内部类型，接口落地后按真实 DTO 校正。
 *
 * 字段命名沿用既有习惯：`isOpen` 与 `FormDefinition.isOpen` 同义。
 */
export interface RecruitmentDirection {
  id: ID;
  title: string;
  summary: string;
  /** 是否开放报名。关闭的方向按钮不可点 */
  isOpen: boolean;
}

/** 三条方向，其中一条关闭，用来覆盖 isOpen 的两种状态 */
export const RECRUITMENT_DIRECTIONS: RecruitmentDirection[] = [
  {
    id: "direction-frontend",
    title: "前端开发",
    summary: "Vue 3 + TypeScript，负责工作室的管理后台与用户侧界面。",
    isOpen: true,
  },
  {
    id: "direction-backend",
    title: "后端开发",
    summary: "Go + Gin + PostgreSQL，负责 API、邮件流程与部署。",
    isOpen: true,
  },
  {
    id: "direction-embedded",
    title: "嵌入式 / IoT",
    summary: "设备接入与数据采集。本学期先做技术预研，暂不开放报名。",
    isOpen: false,
  },
];
