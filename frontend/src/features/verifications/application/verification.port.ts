import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationEligibilityItem,
  VerificationHistoryEntry,
  VerificationType,
} from "../domain/verification";

export interface VerificationPort {
  listVerifications(signal?: AbortSignal): Promise<VerificationCase[]>;

  getVerification(
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationCase | null>;

  startVerification(
    idempotencyKey: string,
    type: VerificationType,
    signal?: AbortSignal,
  ): Promise<VerificationCase>;

  retryVerification(
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
    signal?: AbortSignal,
  ): Promise<VerificationCase>;

  listHistory(
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationHistoryEntry[]>;

  listEvidence(id: string, signal?: AbortSignal): Promise<SyntheticEvidence[]>;

  uploadEvidence(id: string, signal?: AbortSignal): Promise<SyntheticEvidence>;

  deleteEvidence(
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<void>;

  getEvidenceContent(
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  getEligibility(signal?: AbortSignal): Promise<VerificationEligibilityItem[]>;
}
