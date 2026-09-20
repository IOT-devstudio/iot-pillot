/**
 * about 模块（用户侧「关于我们」）的静态文案与假数据。
 *
 * ⚠️ 这里的**每一句涉及事实的话都是编的**——成立年份、成员人数、项目成果、
 * 成员姓名全部是占位内容，不是工作室的真实资料。页面底部因此有一行明确的
 * 标注提醒读者。按 CLAUDE.md「M0」的约定，fixture 只能服务界面开发，
 * **不能伪装成已经完成**。
 *
 * 正式资料由工作室提供后只需要改这一个文件，页面与组件都不用动。
 *
 * ── 为什么方向数据不和 home 模块共用 ──
 * home 的 `fixtures.ts` 里也有一份「前端开发 / 后端开发 / Agent 开发」，
 * 但两边有意分开、不互相 import：
 *   - 语义不同：那边是「**本轮**开放报名的方向」（带 `isOpen`，随时会关掉），
 *     这里是「工作室**长期**的技术方向」；
 *   - 生命周期不同：home 的 fixture 自述「后端接口就绪后应当整体删除」，
 *     而这里是纯静态介绍内容，后端永远不会给它供数。
 * 绑在一起会让两边互相拖累。
 * 代价是方向名称在两处各写了一遍——改动时**两边都要看一眼**。
 */
import type { ID } from "@iot-pillot/shared-types";

/** 简介里的一个数字（成立年份 / 成员人数 / 在做项目数） */
export interface StudioStat {
  label: string;
  value: string;
}

export interface StudioProfile {
  name: string;
  tagline: string;
  description: string;
  stats: StudioStat[];
}

/**
 * 工作室概况。
 *
 * `tagline` 与 `description` 沿用登录页品牌面板（views/AuthView.vue）里
 * 已有的文案，保证两个页面口径一致——改的话两边都要改。
 */
export const STUDIO_INTRO: StudioProfile = {
  name: "iot 全栈工作室",
  tagline: "从界面到服务，把想法做成作品。",
  description:
    "iot 全栈工作室以前端与后端开发为核心，围绕真实问题做项目：拆需求、写代码、联调服务，再把结果交付给真正的用户。",
  stats: [
    { label: "成立", value: "2021" },
    { label: "成员", value: "12" },
    { label: "在做项目", value: "4" },
  ],
};

/**
 * 工作室的技术方向。
 *
 * 这里的「方向」是**长期定位**，不是「本轮招不招人」——所以没有 `isOpen`
 * 这类字段（对比 home 模块的 `RecruitmentDirection`）。
 */
export interface StudioFocus {
  id: ID;
  title: string;
  summary: string;
  /** 主要技术栈，页面渲染成一行小标签 */
  stack: string[];
}

export const STUDIO_FOCUS: StudioFocus[] = [
  {
    id: "focus-frontend",
    title: "前端开发",
    summary: "负责工作室的管理后台与用户侧界面，从排版到交互都由前端落地。",
    stack: ["Vue 3", "TypeScript", "Vite"],
  },
  {
    id: "focus-backend",
    title: "后端开发",
    summary: "负责 API、数据存储、邮件流程与部署，让服务稳定而且好维护。",
    stack: ["Go", "Gin", "PostgreSQL"],
  },
  {
    id: "focus-agent",
    title: "Agent 开发",
    summary: "负责大模型接入、工具调用与工作流编排，把智能体能力接进真实项目。",
    stack: ["LLM API", "Tool Calling", "Workflow"],
  },
];

/** 一件成果 / 项目 */
export interface StudioWork {
  id: ID;
  title: string;
  summary: string;
  /** 年份。用字符串而不是数字，避免页面上渲染出千分位分隔符 */
  year: string;
}

export const STUDIO_WORKS: StudioWork[] = [
  {
    id: "work-pillot",
    title: "iot-pillot 招新管理后台",
    summary: "工作室自用的招新系统：表单收集、邮件邀请、候选人流转一站式处理。",
    year: "2026",
  },
  {
    id: "work-campus-model",
    title: "校园楼栋可视化",
    summary: "把校园楼栋数据渲染成可交互的三维白模，用于展示与布局编辑。",
    year: "2025",
  },
  {
    id: "work-device-gateway",
    title: "设备接入网关",
    summary: "为社团设备提供统一的数据上报与转发入口，支撑后续的数据看板。",
    year: "2025",
  },
];

/** 一位工作室成员 */
export interface StudioMember {
  id: ID;
  name: string;
  /**
   * 工作室内部职能（如「前端负责人」）。
   *
   * ⚠️ 这**不是** shared-types 里的 `Role`（`"admin" | "member"`）——
   * 那个是系统的 RBAC 权限角色，跟「在工作室里负责什么」完全是两回事，
   * 不要为了「复用类型」把两者混起来。
   */
  title: string;
  bio: string;
}

export const STUDIO_MEMBERS: StudioMember[] = [
  {
    id: "member-lead",
    name: "林知远",
    title: "工作室负责人",
    bio: "统筹招新与项目排期，自己也写后端。",
  },
  {
    id: "member-frontend",
    name: "苏晚",
    title: "前端负责人",
    bio: "负责界面与交互，偏爱克制的排版。",
  },
  {
    id: "member-backend",
    name: "周叙",
    title: "后端负责人",
    bio: "管 API、数据与部署，喜欢把事情做简单。",
  },
  {
    id: "member-agent",
    name: "程野",
    title: "Agent 方向负责人",
    bio: "研究大模型应用，把智能体接进真实流程。",
  },
];
