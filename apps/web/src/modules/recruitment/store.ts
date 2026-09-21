/**
 * 招新流程的内存 store。
 *
 * 后端接口没落地，这里用 Vue 的响应式 ref 模拟「发送邮件 / 状态流转」，
 * 列表页和详情页共享同一份数据，互相同步。正式联调后替换为真实请求。
 */
import { ref } from "vue";

import { initialProspects, type Prospect } from "./fixture";
import { nextStatusAfterEmail, type EmailType } from "./status";

const prospects = ref<Prospect[]>(
  initialProspects.map((p) => ({ ...p, emails: [...p.emails] })),
);

let emailSeq = 1;

export function useProspects() {
  return prospects;
}

export function findProspect(id: string): Prospect | undefined {
  return prospects.value.find((p) => p.id === id);
}

export function sendEmail(
  id: string,
  type: EmailType,
  subject: string,
  body: string,
  note: string,
): Prospect | undefined {
  const p = findProspect(id);
  if (!p) return undefined;
  p.emails.push({
    id: `email-${emailSeq++}`,
    type,
    subject,
    sentAt: new Date().toISOString(),
    note,
  });
  p.status = nextStatusAfterEmail(p.status, type);
  return p;
}

export function markFirstPass(id: string): Prospect | undefined {
  const p = findProspect(id);
  if (!p || p.status !== "ready_first") return undefined;
  p.status = "first_passed";
  return p;
}
