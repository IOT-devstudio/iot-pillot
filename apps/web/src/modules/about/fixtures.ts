/**
 * about 模块（用户侧「关于我们」）的静态文案。
 *
 * ── 数据口径（issue #181）──
 * 页面上的每一个数字、名称都来自工作室自己的资料：飞书知识库
 * 「工作室基本信息和资料」「成果展示（截至到22级）」「项目板块」
 * 「负责人职责」，以及 2025 年 10 月的宣讲 PPT。资料没给的事实
 * **一律不补**（issue #94 立下的规矩，这里继续遵守）：
 *   - 不写当前成员人数，也不写「在做项目数」；
 *   - 不展示未经确认的现任成员姓名，团队栏目只放岗位职责；
 *   - 历史项目资料只列了名称，就不补功能、成效和实施年份；
 *   - 成果数字必须和「截至 2022 级」的统计范围同屏出现。
 *
 * 页面只做静态介绍：本轮可报名的方向仍以招新模块（`modules/home`）的
 * 数据和开放状态为准，本文件不表达任何「招不招人」的信息。
 *
 * ── 为什么主题数据不和 home 模块共用 ──
 * home 的 fixture 里也有一份「前端开发 / 后端开发 / Agent 开发」，
 * 但两边有意分开、不互相 import：
 *   - 语义不同：那边是「**本轮**开放报名的方向」（带 `isOpen`，随时会关掉），
 *     这里是「工作室资料里提到的**技术主题**」，纯介绍；
 *   - 生命周期不同：home 的 fixture 自述「后端接口就绪后应当整体删除」，
 *     而这里是纯静态介绍内容，后端永远不会给它供数。
 * 绑在一起会让两边互相拖累。
 * 代价是主题名称在两处各写了一遍——改动时**两边都要看一眼**。
 */
import type { ID } from "@iot-pillot/shared-types";

/** 简介里的一个数字位（成立年份 / 地点 / 累计项目） */
export interface StudioStat {
  label: string;
  value: string;
}

export interface StudioProfile {
  name: string;
  tagline: string;
  description: string;
  stats: StudioStat[];
  /** 数字位的统计范围说明，必须与数字同屏，避免把历史统计读成当前规模 */
  statsNote: string;
}

/**
 * 工作室概况。
 *
 * `tagline` 与登录页品牌面板（views/AuthView.vue）里已有的文案保持一致，
 * 改的话两边都要改；`description` 只陈述知识库与宣讲资料里写明的事实。
 */
export const STUDIO_INTRO: StudioProfile = {
  name: "IOT 全栈工作室",
  tagline: "从界面到服务，把想法做成作品。",
  description:
    "IOT 全栈工作室成立于 2018 年，是信息工程学院八大双创工作室之一，由学院老师指导、学生组成。工作室以前端与后端开发为核心，围绕真实问题做项目：拆需求、写代码、联调服务，再把结果交付给真正的用户。",
  stats: [
    { label: "成立", value: "2018" },
    { label: "地点", value: "第十教学楼 B413" },
    { label: "累计实战项目", value: "50 余个" },
  ],
  statsNote: "项目数据截至 2022 级（历史统计），不代表当前规模。",
};

/**
 * 工作室的技术主题。
 *
 * 这里的「主题」是资料里讲到的技术内容，**不是**「本轮招不招人」——
 * 所以没有 `isOpen` 这类字段（对比 home 模块的 `RecruitmentDirection`），
 * 也不声明长期技术定位。
 */
export interface StudioFocus {
  id: ID;
  title: string;
  summary: string;
  /** 学习内容，页面渲染成一行小标签。**不是**工作室的正式技术栈 */
  topics: string[];
  /** 资料来源说明；有值时在卡片内可见，用来说明这条主题的依据 */
  source?: string;
}

export const STUDIO_FOCUS: StudioFocus[] = [
  {
    id: "focus-frontend",
    title: "前端开发",
    summary: "关注网页界面与交互：把需求拆成页面与流程，完成排版、组件与交互细节，并与后端接口联调。",
    topics: ["网页界面", "交互实现", "组件化开发", "接口联调"],
  },
  {
    id: "focus-backend",
    title: "后端开发",
    summary: "关注服务、API 与数据处理：设计接口与数据结构，实现业务逻辑，处理邮件、权限等系统流程并完成部署。",
    topics: ["服务与 API", "数据处理", "系统集成", "部署运维"],
  },
  {
    id: "focus-agent",
    title: "Agent 开发",
    summary:
      "宣讲资料在「后端 AI 开发」章节介绍了 Agent、RAG 等概念。这是宣讲涉及的技术主题，不代表工作室已有 Agent 项目，也不代表本轮开放报名。",
    topics: ["Agent", "RAG", "后端 AI 开发"],
    source: "2025 年宣讲主题「后端 AI 开发」",
  },
];

/** 成果统计的统计范围。数字必须和它同屏出现 */
export const ACHIEVEMENT_SCOPE = "截至 2022 级";

/** 一项成果统计 */
export interface StudioAchievement {
  id: ID;
  label: string;
  value: string;
}

/**
 * 成果统计。
 *
 * 来自知识库「成果展示（截至到22级）」。统计范围写在
 * `ACHIEVEMENT_SCOPE` 里，由 WorksList 固定渲染在数字上方，
 * 不能只挂在某一张卡片上。
 */
export const STUDIO_ACHIEVEMENTS: StudioAchievement[] = [
  { id: "award-national", label: "国家级奖项", value: "8 项" },
  { id: "award-provincial", label: "省部级奖项", value: "25 项" },
  { id: "intellectual-property", label: "授权专利及软件著作权", value: "5 项" },
  { id: "cumulative-projects", label: "累计实战项目", value: "50 余个" },
];

/** 历史项目的资料来源。两份资料都只列了名称 */
const WORK_SOURCE = "2025 年宣讲资料 · 知识库项目板块";

/** 一件历史项目 */
export interface StudioWork {
  id: ID;
  title: string;
  summary: string;
  /**
   * 资料来源时间（「2025 年宣讲资料」等）。
   *
   * ⚠️ 这**不是**项目实施年份，页面也不能把它放进「年份」位——两份资料
   * 都没写年份，写成 2025 会被读成「2025 年做的项目」。
   */
  source: string;
}

/**
 * 历史项目。
 *
 * 名称来自宣讲 PPT 第 8 页与知识库「项目板块」，两份资料都没有实施年份
 * 和详细简介，所以 summary 统一是中性的「待核实」说明，不补造功能与成效。
 */
export const STUDIO_WORKS: StudioWork[] = [
  {
    id: "work-nongyu-app",
    title: "农屿 APP 开发",
    summary: "资料仅列出项目名称，实施年份与范围待核实。",
    source: WORK_SOURCE,
  },
  {
    id: "work-yaan-weather",
    title: "雅安气象项目",
    summary: "资料仅列出项目名称，实施年份与范围待核实。",
    source: WORK_SOURCE,
  },
  {
    id: "work-78511-website",
    title: "78511 部队强军网站建设",
    summary: "资料仅列出项目名称，实施年份与范围待核实。",
    source: WORK_SOURCE,
  },
  {
    id: "work-guanghan-logistics",
    title: "广汉国家级农业产业园掌上物流",
    summary: "资料仅列出项目名称，实施年份与范围待核实。",
    source: WORK_SOURCE,
  },
];

/** 一个岗位及其职责 */
export interface StudioRole {
  id: ID;
  title: string;
  summary: string;
  /** 该岗位负责的具体事务，页面渲染成一列职责条目 */
  duties: string[];
}

/**
 * 团队分工。
 *
 * 来自知识库「负责人职责」。这里是**岗位**，不是现任人员名单——没有可
 * 公开的名册就不生成姓名与头像（issue #181）。也**不是** shared-types 里的
 * `Role`（`"admin" | "member"`）：那个是系统 RBAC 权限角色，跟「在工作室里
 * 负责什么」完全是两回事。
 */
export const STUDIO_ROLES: StudioRole[] = [
  {
    id: "role-lead",
    title: "总负责人",
    summary: "统筹工作室整体运转。",
    duties: ["招新", "例会", "知识库", "项目对接", "团队建设"],
  },
  {
    id: "role-frontend-lead",
    title: "前端负责人",
    summary: "负责前端方向的技术分享与招新环节。",
    duties: ["技术分享", "招新出题", "面试"],
  },
  {
    id: "role-backend-lead",
    title: "后端负责人",
    summary: "负责后端方向的技术分享与招新环节。",
    duties: ["技术分享", "招新出题", "面试"],
  },
];
