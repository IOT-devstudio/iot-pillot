import type {
  InvitationListView,
  InvitationView,
  SendInvitationInput,
  MyInvitationListView,
  AcceptInvitationView,
  CandidateListView,
  CandidateDetailView,
  CandidateView,
  AdvanceCandidateInput,
  MyCandidateListView,
  CandidateStatus,
} from "@iot-pillot/shared-types";
import { requestWithSession } from "./session-request";
const post = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});
export const listInvitations = (page = 1, applicationId?: number) =>
  requestWithSession<InvitationListView>(
    `/api/v1/admin/invitations?page=${page}&page_size=20${applicationId ? `&application_id=${applicationId}` : ""}`,
  );
export const sendInvitation = (input: SendInvitationInput, userId: number) =>
  requestWithSession<InvitationView>(
    "/api/v1/admin/invitations/send",
    post(input),
    userId,
  );
export const revokeInvitation = (id: number, userId: number) =>
  requestWithSession<InvitationView>(
    `/api/v1/admin/invitations/${id}/revoke`,
    post({}),
    userId,
  );
export const listCandidates = (page = 1, status?: CandidateStatus) =>
  requestWithSession<CandidateListView>(
    `/api/v1/admin/candidates?page=${page}&page_size=20${status ? `&status=${status}` : ""}`,
  );
export const getCandidate = (id: number) =>
  requestWithSession<CandidateDetailView>(`/api/v1/admin/candidates/${id}`);
export const advanceCandidate = (
  id: number,
  input: AdvanceCandidateInput,
  userId: number,
) =>
  requestWithSession<CandidateView>(
    `/api/v1/admin/candidates/${id}/advance`,
    post(input),
    userId,
  );
export const myInvitations = (userId: number) =>
  requestWithSession<MyInvitationListView>(
    "/api/v1/me/invitations",
    undefined,
    userId,
  );
export const myCandidates = (userId: number) =>
  requestWithSession<MyCandidateListView>(
    "/api/v1/me/candidates",
    undefined,
    userId,
  );
export const acceptInvitation = (token: string, userId: number) =>
  requestWithSession<AcceptInvitationView>(
    "/api/v1/me/invitations/accept",
    post({ token }),
    userId,
  );
