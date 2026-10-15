/** #93：网络 DTO 对齐 BE-05；共享包中的 camelCase 类型仍供旧演示使用。 */
import type { FormFieldType } from "@iot-pillot/shared-types";
import { requestWithSession } from "./session-request";

export interface FormFieldInput {
  id?: number;
  type: FormFieldType;
  label: string;
  required: boolean;
  placeholder: string;
  options?: string[];
}
export interface FormField extends FormFieldInput {
  id: number;
  sort_order: number;
}
export interface FormDefinition {
  id: number;
  title: string;
  description: string;
  round_id: number;
  direction_code: string;
  share_token: string;
  is_open: boolean;
  created_at: string;
  updated_at: string;
  submission_count: number;
}
export interface FormDetail {
  form: FormDefinition;
  fields: FormField[];
}
export interface FormCreateInput {
  title: string;
  description: string;
  round_id: number;
  direction_code: string;
  fields: FormFieldInput[];
}
export interface FormUpdateInput {
  title?: string;
  description?: string;
  is_open?: boolean;
  fields?: FormFieldInput[];
}
export type FormAnswers = Record<string, string | string[]>;
export interface FormSubmission {
  id: number;
  form_id: number;
  submitter_id: number;
  application_id: number;
  values: FormAnswers;
  submitted_at: string;
}
export interface ShareForm extends FormDetail {
  submitted: boolean;
  my_submission_id: number;
}
export interface SubmitResult {
  submission: FormSubmission;
  application_id: number;
  application_status: string;
}
export interface FormPage<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}
const json = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});
export const listForms = (page = 1): Promise<FormPage<FormDefinition>> =>
  requestWithSession(`/api/v1/forms?page=${page}&page_size=20`);
export const fetchForm = (id: number): Promise<FormDetail> =>
  requestWithSession(`/api/v1/forms/${id}`);
export const createForm = (input: FormCreateInput): Promise<FormDetail> =>
  requestWithSession("/api/v1/forms", json("POST", input));
export const updateForm = (
  id: number,
  input: FormUpdateInput,
): Promise<FormDetail> =>
  requestWithSession(`/api/v1/forms/${id}`, json("PATCH", input));
export const fetchShareForm = (token: string): Promise<ShareForm> =>
  requestWithSession(`/api/v1/forms/public/${encodeURIComponent(token)}`);
export const submitForm = (
  token: string,
  values: FormAnswers,
  expectedUserId: number,
): Promise<SubmitResult> =>
  requestWithSession(
    `/api/v1/forms/public/${encodeURIComponent(token)}/submit`,
    json("POST", { values }),
    expectedUserId,
  );
export const listFormSubmissions = (
  id: number,
  page = 1,
): Promise<FormPage<FormSubmission>> =>
  requestWithSession(
    `/api/v1/forms/${id}/submissions?page=${page}&page_size=20`,
  );
export const fetchFormSubmission = (
  id: number,
  submissionId: number,
): Promise<FormSubmission> =>
  requestWithSession(`/api/v1/forms/${id}/submissions/${submissionId}`);
