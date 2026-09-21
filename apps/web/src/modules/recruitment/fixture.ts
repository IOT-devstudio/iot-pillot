/**
 * 招新流程的本地 fixture + 类型。
 *
 * 后端 Form / Recruitment 接口尚未落地，这些数据只服务界面开发。
 * 意向成员的基础信息（班级/发展意向/参加原因等）来自报名表单，这里先写死。
 */
import type { EmailType, ProspectStatus } from "./status";

/** 意向成员（prospect）。基础信息字段与报名表单对齐（当前写死）。 */
export interface Prospect {
  id: string;
  name: string;
  email: string;
  className: string; // 班级
  registeredAt: string; // 报名时间（ISO）
  intention: string; // 发展意向
  reason: string; // 参加原因
  status: ProspectStatus;
  emails: EmailRecord[]; // 邮件记录（含备注）
}

export interface EmailRecord {
  id: string;
  type: EmailType;
  subject: string;
  sentAt: string; // ISO
  note: string; // 备注
}

/** 邮件模板：仅提供默认主题/正文，发送前管理员可编辑 */
export interface EmailTemplate {
  type: EmailType;
  name: string;
  subject: string;
  body: string;
}

export function formatDateTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? iso
    : d.toLocaleString("zh-CN", { hour12: false });
}

/** 用收件人姓名替换 {{name}} */
export function fillName(template: string, name: string): string {
  return template.replace(/\{\{\s*name\s*\}\}/g, name);
}

/** 邮件默认文案：通过（一面/录用）与拒录 */
export const emailTemplates: EmailTemplate[] = [
  {
    type: "pass",
    name: "通过邮件",
    subject: "【IOT 工作室】面试结果通知",
    body: "{{name}} 你好：\n恭喜你通过本轮面试，后续安排将另行通知。",
  },
  {
    type: "reject",
    name: "拒录邮件",
    subject: "【IOT 工作室】面试结果通知",
    body: "{{name}} 你好：\n感谢你参与本次招新，很遗憾未能通过。",
  },
];

export const initialProspects: Prospect[] = [
  {
    id: "p1",
    name: "陈晓",
    email: "chenxiao@example.com",
    className: "计科 2301",
    registeredAt: "2026-09-15T08:30:00Z",
    intention: "前端开发",
    reason: "想参与真实项目练手，熟悉 Vue。",
    status: "ready_first",
    emails: [],
  },
  {
    id: "p2",
    name: "李然",
    email: "liran@example.com",
    className: "软工 2202",
    registeredAt: "2026-09-14T10:00:00Z",
    intention: "后端开发",
    reason: "对 Go 和分布式感兴趣。",
    status: "first_passed",
    emails: [],
  },
  {
    id: "p3",
    name: "王舒",
    email: "wangshu@example.com",
    className: "计科 2302",
    registeredAt: "2026-09-13T09:00:00Z",
    intention: "全栈",
    reason: "希望从前端到后端完整走一遍。",
    status: "ready_second",
    emails: [
      {
        id: "e1",
        type: "pass",
        subject: "【IOT 工作室】面试结果通知",
        sentAt: "2026-09-14T10:05:00Z",
        note: "一面表现不错，已约二面。",
      },
    ],
  },
  {
    id: "p4",
    name: "赵琳",
    email: "zhaolin@example.com",
    className: "信安 2101",
    registeredAt: "2026-09-12T14:00:00Z",
    intention: "前端开发",
    reason: "喜欢交互设计。",
    status: "hired",
    emails: [
      {
        id: "e2",
        type: "pass",
        subject: "【IOT 工作室】面试结果通知",
        sentAt: "2026-09-13T10:05:00Z",
        note: "一面通过。",
      },
      {
        id: "e3",
        type: "pass",
        subject: "【IOT 工作室】录用通知",
        sentAt: "2026-09-15T09:05:00Z",
        note: "二面通过，已发录用。",
      },
    ],
  },
  {
    id: "p5",
    name: "周杰",
    email: "zhoujie@example.com",
    className: "网工 2201",
    registeredAt: "2026-09-11T16:00:00Z",
    intention: "后端开发",
    reason: "对数据库感兴趣。",
    status: "rejected",
    emails: [
      {
        id: "e4",
        type: "reject",
        subject: "【IOT 工作室】面试结果通知",
        sentAt: "2026-09-12T11:05:00Z",
        note: "一面未通过。",
      },
    ],
  },
];
