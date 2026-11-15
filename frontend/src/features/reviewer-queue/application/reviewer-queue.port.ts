import type { AccessToken } from "@/features/auth/application/auth.port";
import type { IdentityReviewDecisionRequest } from "../domain/reviewer-case";

export interface ReviewerQueuePort {
  listAssigned(token: AccessToken, signal?: AbortSignal): Promise<unknown>;
  getAssigned(
    token: AccessToken,
    caseId: string,
    signal?: AbortSignal,
  ): Promise<unknown>;
  reviewAssigned(
    token: AccessToken,
    caseId: string,
    input: IdentityReviewDecisionRequest,
    signal?: AbortSignal,
  ): Promise<unknown>;
  downloadAssignedFile(
    token: AccessToken,
    caseId: string,
    fileId: string,
    signal?: AbortSignal,
  ): Promise<Blob>;
}
