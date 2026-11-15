import type { AccessToken } from "@/features/auth/application/auth.port";
import {
  toReviewerSummary,
  isCanonicalReviewerCaseId,
  isCanonicalReviewerFileId,
  toReviewerIdentityDetail,
  buildReviewerIdentityDecisionRequest,
  toReviewerDecisionResult,
  type ReviewerSummary,
  type ReviewerIdentityDetail,
  type ReviewerIdentityFile,
  type IdentityReviewDraft,
  type ReviewerIdentityDecisionResult,
} from "../domain/reviewer-case";
import type { ReviewerQueuePort } from "./reviewer-queue.port";

interface AuthSnapshot {
  readonly status: string;
  readonly restrictedMode: boolean;
}

export interface ReviewerQueueAuth {
  getSnapshot(): AuthSnapshot;
  runAuthenticatedOperation<T>(
    operation: (token: AccessToken, signal: AbortSignal) => Promise<T>,
  ): Promise<T>;
}

export class ReviewerQueueFacade {
  constructor(
    private readonly port: ReviewerQueuePort,
    private readonly auth: ReviewerQueueAuth,
  ) {}

  listAssigned = async (): Promise<ReviewerSummary[]> => {
    const session = this.auth.getSnapshot();
    if (session.status !== "authenticated" || session.restrictedMode) {
      throw new Error("Reviewer queue unavailable");
    }

    return this.auth.runAuthenticatedOperation(async (token, signal) => {
      const response = await this.port.listAssigned(token, signal);
      return toReviewerSummary(response);
    });
  };

  getAssignedIdentityDetail = async (
    caseId: string,
  ): Promise<ReviewerIdentityDetail> => {
    const session = this.auth.getSnapshot();
    if (
      session.status !== "authenticated" ||
      session.restrictedMode ||
      !isCanonicalReviewerCaseId(caseId)
    ) {
      throw new Error("Reviewer detail unavailable");
    }

    return this.auth.runAuthenticatedOperation(async (token, signal) => {
      const response = await this.port.getAssigned(token, caseId, signal);
      return toReviewerIdentityDetail(response, caseId);
    });
  };

  reviewAssignedIdentityCase = async (
    caseId: string,
    revision: number | string,
    draft: IdentityReviewDraft,
  ): Promise<ReviewerIdentityDecisionResult> => {
    const session = this.auth.getSnapshot();
    if (
      session.status !== "authenticated" ||
      session.restrictedMode ||
      !isCanonicalReviewerCaseId(caseId)
    ) {
      throw new Error("Reviewer decision unavailable");
    }

    const input = buildReviewerIdentityDecisionRequest(draft, revision);
    if (!input) throw new Error("Reviewer decision unavailable");

    return this.auth.runAuthenticatedOperation(async (token, signal) => {
      const response = await this.port.reviewAssigned(
        token,
        caseId,
        input,
        signal,
      );
      return toReviewerDecisionResult(
        response,
        caseId,
        input.expected_revision,
        input.decision,
      );
    });
  };

  downloadAssignedIdentityFile = async (
    caseId: string,
    file: ReviewerIdentityFile,
    requestSignal?: AbortSignal,
  ): Promise<Blob> => {
    const session = this.auth.getSnapshot();
    if (
      session.status !== "authenticated" ||
      session.restrictedMode ||
      !isCanonicalReviewerCaseId(caseId) ||
      !isCanonicalReviewerFileId(file.id) ||
      !file.evidenceRole ||
      file.state !== "limpio" ||
      !["image/jpeg", "image/png"].includes(file.mime) ||
      !Number.isSafeInteger(file.size) ||
      file.size < 1 ||
      file.size > 10 * 1024 * 1024
    ) {
      throw new Error("Reviewer evidence unavailable");
    }

    return this.auth.runAuthenticatedOperation(async (token, sessionSignal) => {
      const signal = requestSignal
        ? AbortSignal.any([sessionSignal, requestSignal])
        : sessionSignal;
      const blob = await this.port.downloadAssignedFile(
        token,
        caseId,
        file.id,
        signal,
      );
      if (
        !(blob instanceof Blob) ||
        blob.size !== file.size ||
        blob.type !== file.mime
      ) {
        throw new TypeError("Invalid reviewer evidence response");
      }
      return blob;
    });
  };
}
