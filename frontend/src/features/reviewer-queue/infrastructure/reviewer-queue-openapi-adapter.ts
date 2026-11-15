import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import type { ReviewerQueuePort } from "../application/reviewer-queue.port";
import type { IdentityReviewDecisionRequest } from "../domain/reviewer-case";

function authorization(token: AccessToken): { Authorization: string } {
  return { Authorization: ["Bearer", token].join(" ") };
}

export class OpenApiReviewerQueueAdapter implements ReviewerQueuePort {
  async listAssigned(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<unknown> {
    const result = await apiClient.GET(
      "/document-verification/reviewer/cases",
      {
        headers: authorization(token),
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data;
  }

  async getAssigned(
    token: AccessToken,
    caseId: string,
    signal?: AbortSignal,
  ): Promise<unknown> {
    const result = await apiClient.GET(
      "/document-verification/reviewer/cases/{caseID}",
      {
        params: { path: { caseID: caseId } },
        headers: authorization(token),
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data;
  }

  async reviewAssigned(
    token: AccessToken,
    caseId: string,
    input: IdentityReviewDecisionRequest,
    signal?: AbortSignal,
  ): Promise<unknown> {
    const result = await apiClient.POST(
      "/document-verification/reviewer/cases/{caseID}/review",
      {
        params: { path: { caseID: caseId } },
        headers: authorization(token),
        body: input,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return result.data;
  }

  async downloadAssignedFile(
    token: AccessToken,
    caseId: string,
    fileId: string,
    signal?: AbortSignal,
  ): Promise<Blob> {
    const result = await apiClient.GET(
      "/document-verification/reviewer/cases/{caseID}/files/{fileID}/content",
      {
        params: { path: { caseID: caseId, fileID: fileId } },
        headers: authorization(token),
        parseAs: "blob",
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    const blob = result.data as Blob | undefined;
    if (!(blob instanceof Blob)) throw new ApiProtocolError();
    return blob;
  }
}
