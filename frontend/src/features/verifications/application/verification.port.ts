import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationEligibilityItem,
  VerificationHistoryEntry,
  VerificationType,
} from "../domain/verification";

export interface VerificationPort {
  listVerifications(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<VerificationCase[]>;

  getVerification(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationCase | null>;

  startVerification(
    token: AccessToken,
    idempotencyKey: string,
    type: VerificationType,
    signal?: AbortSignal,
  ): Promise<VerificationCase>;

  retryVerification(
    token: AccessToken,
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
    signal?: AbortSignal,
  ): Promise<VerificationCase>;

  listHistory(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationHistoryEntry[]>;

  listEvidence(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence[]>;

  uploadEvidence(
    token: AccessToken,
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence>;

  deleteEvidence(
    token: AccessToken,
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<void>;

  getEvidenceContent(
    token: AccessToken,
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  getEligibility(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<VerificationEligibilityItem[]>;
}
