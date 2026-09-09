// 邮件模板类型：用于招新各环节（邀请、offer、感谢信）。
// body 用 Handlebars 风格占位符（{{name}}、{{interview_time}} 等）。

import type { ID, ISODateTime } from "./common";

export type TemplateKind = "invitation" | "offer" | "rejection" | "custom";

export interface EmailTemplate {
  id: ID;
  kind: TemplateKind;
  name: string;
  subject: string;
  body: string;
  // 从 body 扫描出的变量名，供前端渲染表单与后端校验使用。
  variables: string[];
  createdAt: ISODateTime;
  updatedAt: ISODateTime;
}

export interface CreateTemplateInput {
  kind: TemplateKind;
  name: string;
  subject: string;
  body: string;
}
