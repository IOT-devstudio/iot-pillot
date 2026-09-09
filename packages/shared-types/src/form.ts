// 表单系统类型：定义表单元数据与提交数据。
// 表单通过 shareToken 提供匿名填写能力。

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
  // 表单字段 id → 用户填写值；checkbox 类型允许 string[]。
  values: Record<string, string | string[]>;
  submittedAt: ISODateTime;
}

export interface CreateFormInput {
  title: string;
  description?: string;
  fields: FormField[];
}
