// 表单系统类型：定义表单元数据与提交数据。
// 表单通过 shareToken 定位与分享；提交必须登录，不存在匿名提交端点（#92 BE-05 裁决）。

import type { ID, ISODateTime } from "./common";

export type FormFieldType =
  | "text"
  | "email"
  | "textarea"
  | "select"
  | "checkbox";

export interface FormField {
  id: ID;
  type: FormFieldType;
  label: string;
  required: boolean;
  placeholder?: string;
  // 仅 select 类型使用。
  options?: string[];
}

export interface FormDefinition {
  id: ID;
  title: string;
  description?: string;
  fields: FormField[];
  shareToken: string;
  isOpen: boolean;
  createdAt: ISODateTime;
}

export interface FormSubmission {
  id: ID;
  formId: ID;
  // 提交人 ID（登录态所有者）。匿名提交不被支持，所有提交均绑定到一个已登录用户（#92 BE-05 裁决）。
  submitterId: ID;
  // 表单字段 id → 用户填写值；checkbox 类型允许 string[]。
  values: Record<string, string | string[]>;
  submittedAt: ISODateTime;
}

export interface CreateFormInput {
  title: string;
  description?: string;
  fields: FormField[];
}
