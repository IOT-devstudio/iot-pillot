import { requestMessageWithSession, requestWithSession } from "./session-request";

/**
 * 邮件中心 API（issue #52）。
 *
 * 发送一律走 template_id + vars，不提供「临时改写正文」的入口——后端发送
 * 接口只有这三种形状，前端另造一套自由文本会跟发信记录对不上账。
 * 变量值允许留空：服务端会用收件人资料预填（见 mail_usec.render）。
 */

/** 后端 MailModelResp：模板 + 由后端扫描出的变量名。 */
export interface MailTemplate {
  id: number;
  name: string;
  type: string;
  title: string;
  mail_example: string;
  mail_model: string;
  variables: string[];
}

export interface MailTemplateList {
  items: MailTemplate[];
  total: number;
}

/** 新建/更新模板的请求体（对应后端 request.MailTemplateReq）。 */
export interface MailTemplateInput {
  name: string;
  type: string;
  title: string;
  mail_example: string;
  mail_model: string;
}

export type MailVars = Record<string, string>;

/** 单发（按用户）结果，对应后端 response.MailSendResp。 */
export interface MailSendResult {
  to_user_id: number;
  to_email: string;
  title: string;
}

export interface MailSendFailure {
  to_user_id: number;
  to_email: string;
  reason: string;
}

/** 群发结果：逐条成败分开列，单个失败不影响已发出的邮件。 */
export interface MailBulkResult {
  sent: MailSendResult[];
  failed: MailSendFailure[];
  total: number;
}

export interface MailSendToUserInput {
  template_id: number;
  to_user_id: number;
  vars?: MailVars;
}

export interface MailSendToEmailInput {
  template_id: number;
  email: string;
  vars?: MailVars;
}

export interface MailBulkRecipient {
  to_user_id: number;
  vars?: MailVars;
}

export interface MailSendBulkInput {
  template_id: number;
  recipients: MailBulkRecipient[];
}

/** 单次群发的收件人上限，与后端 service.MaxBulkRecipients 保持一致。 */
export const MAIL_BULK_LIMIT = 200;

/** 模板列表（不分页，后端一次性返回全部）。 */
export function listMailTemplates(): Promise<MailTemplateList> {
  return requestWithSession<MailTemplateList>("/api/v1/admin/mail-templates");
}

export function createMailTemplate(
  input: MailTemplateInput,
): Promise<MailTemplate> {
  return requestWithSession<MailTemplate>("/api/v1/admin/mail-templates", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function updateMailTemplate(
  id: number,
  input: MailTemplateInput,
): Promise<MailTemplate> {
  return requestWithSession<MailTemplate>(
    `/api/v1/admin/mail-templates/${id}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
}

/**
 * 删除模板：后端返回 OKWithMsg("模板已删除", nil)，data 被 omitempty 丢掉，
 * 必须走 message 出口——用要求 data 存在的 requestWithSession 会把
 * 成功的删除当成「服务暂时不可用」弹给用户。
 */
export function deleteMailTemplate(id: number): Promise<string> {
  return requestMessageWithSession(`/api/v1/admin/mail-templates/${id}`, {
    method: "DELETE",
  });
}

/** 按注册用户单发。 */
export function sendMailToUser(
  input: MailSendToUserInput,
): Promise<MailSendResult> {
  return requestWithSession<MailSendResult>("/api/v1/admin/mails/send", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

/** 按邮箱单发（收件人未注册时用）。 */
export function sendMailToEmail(
  input: MailSendToEmailInput,
): Promise<MailSendResult> {
  return requestWithSession<MailSendResult>(
    "/api/v1/admin/mails/send-by-email",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
}

/** 群发：同一模板逐个投递，响应里分别给出成功与失败清单。 */
export function sendMailBulk(
  input: MailSendBulkInput,
): Promise<MailBulkResult> {
  return requestWithSession<MailBulkResult>("/api/v1/admin/mails/send-bulk", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}
