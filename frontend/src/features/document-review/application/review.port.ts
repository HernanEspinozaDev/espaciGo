import type { AccessToken } from "@/features/auth/application/auth.port";
import type { ReviewCase, ReviewInput, RevokeInput } from "../domain/review";

export interface ReviewPort {
  list(token: AccessToken, signal: AbortSignal): Promise<ReviewCase[]>;
  get(
    token: AccessToken,
    caseId: string,
    signal: AbortSignal,
  ): Promise<ReviewCase>;
  download(
    token: AccessToken,
    caseId: string,
    fileId: string,
    signal: AbortSignal,
  ): Promise<Blob>;
  review(
    token: AccessToken,
    caseId: string,
    input: ReviewInput,
    signal: AbortSignal,
  ): Promise<ReviewCase>;
  revoke(
    token: AccessToken,
    caseId: string,
    input: RevokeInput,
    signal: AbortSignal,
  ): Promise<ReviewCase>;
}
