/**
 * home 模块（用户侧）界面开发用的本地假数据。
 *
 * ⚠️ 这里**全部是 fixture，不是联调数据**。按 CLAUDE.md「M0」的约定：
 * 后端接口未交付时可以用与最终 DTO 一致的本地 fixture 推进界面，
 * 但 fixture 只能服务界面开发，**不能当作接口已经打通**。
 *
 * 后端接口就绪后这个文件应当整体删除，改为在页面里拉真实数据。
 */
import type {
  ID,
  ISODateTime,
  ProspectStatus,
} from "@iot-pillot/shared-types";

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

/**
 * 三条方向：前端开发 / 后端开发 / Agent 开发，本轮全部开放报名。
 *
 * 注意 `isOpen` 目前三条都是 true —— 也就是说「方向关闭、按钮禁用」这条
 * 分支当前没有 fixture 覆盖。字段和 DirectionList 里的禁用逻辑都保留，
 * 因为它是真实存在的领域状态（与 `FormDefinition.isOpen` 同义），
 * 等真的有方向关闭时自然会走到。
 */
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
    id: "direction-agent",
    title: "Agent 开发",
    summary: "大模型接入、工具调用与工作流编排，负责工作室的智能体应用。",
    isOpen: true,
  },
];

/**
 * 一条报名记录。
 *
 * **状态直接复用 shared-types 的 `ProspectStatus`**（pending → invited →
 * accepted，旁支 rejected），不自定义枚举：`Prospect` 就是「意向新生」，
 * 它的生命周期描述的正是「报名者本人走到哪一步了」，与这里要显示的
 * 「当前状态」同义。
 *
 * 之所以不用 `CandidateStatus`（interviewing / offered / hired / rejected）：
 * 那是**接受邀请之后**的候选人阶段，是管理侧推进面试与发 offer 的视角，
 * 放进用户侧「我的报名」里会错位。两个状态机不揉进同一个字段——将来要在
 * 用户侧展示候选人进展，另加一段。
 *
 * ⚠️ 一个必须说明的缺口：shared-types 的 `Prospect` **没有「方向」字段**
 * （只有 email / sourceFormId / invitedAt / acceptedAt / createdAt）。
 * 也就是说「方向 + 状态 + 报名时间」这个形状，后端目前的模型**表达不出来**，
 * 招新接口落地前不要以为它已经存在。
 */
export interface DirectionApplication {
  id: ID;
  /** 指向 RECRUITMENT_DIRECTIONS 的 id。不冗余存标题，避免两处标题各说各话 */
  directionId: ID;
  status: ProspectStatus;
  appliedAt: ISODateTime;
}

/**
 * 我的报名记录（假数据）。
 *
 * 三条分别落在前端开发 / 后端开发 / Agent 开发上，覆盖 `accepted` /
 * `invited` / `pending` 三种状态，正好把状态标签的三种配色都演示到。
 */
export const MY_APPLICATIONS: DirectionApplication[] = [
  {
    id: "application-frontend",
    directionId: "direction-frontend",
    status: "accepted",
    appliedAt: "2026-09-02T20:14:00+08:00",
  },
  {
    id: "application-backend",
    directionId: "direction-backend",
    status: "invited",
    appliedAt: "2026-09-05T11:05:00+08:00",
  },
  {
    id: "application-agent",
    directionId: "direction-agent",
    status: "pending",
    appliedAt: "2026-09-08T09:30:00+08:00",
  },
];

/**
 * 按 id 找方向；找不到返回 undefined，由调用方决定怎么降级。
 * 与 opener 模块 `findBuilding(name, data = BUILDINGS)` 同一路数。
 */
export function findDirection(
  id: ID,
  directions: RecruitmentDirection[] = RECRUITMENT_DIRECTIONS,
): RecruitmentDirection | undefined {
  return directions.find((direction) => direction.id === id);
}
