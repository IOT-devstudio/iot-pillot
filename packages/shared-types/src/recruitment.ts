// 招新领域类型：prospect（意向新生）→ candidate（候选人）→ hired/rejected。
// 时间戳统一用 ISO-8601 字符串（common.ISODateTime）。

import type { ID, ISODateTime } from "./common";

export type ProspectStatus = "pending" | "invited" | "accepted" | "rejected";
export type CandidateStatus = "interviewing" | "offered" | "hired" | "rejected";

export interface Prospect {
  id: ID;
  name?: string;
  email: string;
  status: ProspectStatus;
  sourceFormId?: ID;
  invitedAt?: ISODateTime;
  acceptedAt?: ISODateTime;
  createdAt: ISODateTime;
}

export interface Candidate {
  id: ID;
  prospectId: ID;
  name: string;
  email: string;
  status: CandidateStatus;
  notes?: string;
  offeredAt?: ISODateTime;
  hiredAt?: ISODateTime;
  rejectedAt?: ISODateTime;
  createdAt: ISODateTime;
}

export interface SendOfferInput {
  candidateId: ID;
  templateId: ID;
  customFields?: Record<string, string>;
}

export interface SendRejectionInput {
  candidateId: ID;
  templateId: ID;
  customFields?: Record<string, string>;
}
