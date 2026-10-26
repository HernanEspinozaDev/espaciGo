"use client";

import { useCallback, useSyncExternalStore } from "react";
import { verificationRuntime } from "../verification-runtime";
import type { VerificationSnapshot } from "../application/verification-facade";
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationType,
} from "../domain/verification";

export interface VerificationView {
  snapshot: VerificationSnapshot;
  loadOverview(signal?: AbortSignal): Promise<VerificationSnapshot>;
  loadCaseDetails(
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
  clear(): void;
}

export function useVerifications(): VerificationView {
  const snapshot = useSyncExternalStore(
    verificationRuntime.subscribe,
    verificationRuntime.getSnapshot,
    verificationRuntime.getServerSnapshot,
  );

  const loadOverview = useCallback(
    (signal?: AbortSignal) => verificationRuntime.loadOverview(signal),
    [],
  );
  const loadCaseDetails = useCallback(
    (id: string, signal?: AbortSignal) =>
      verificationRuntime.loadCaseDetails(id, signal),
    [],
  );
  const startVerification = useCallback(
    (idempotencyKey: string, type: VerificationType, signal?: AbortSignal) =>
      verificationRuntime.startVerification(idempotencyKey, type, signal),
    [],
  );
  const retryVerification = useCallback(
    (
      id: string,
      idempotencyKey: string,
      correctionCode: VerificationCorrectionCode,
      signal?: AbortSignal,
    ) =>
      verificationRuntime.retryVerification(
        id,
        idempotencyKey,
        correctionCode,
        signal,
      ),
    [],
  );
  const uploadEvidence = useCallback(
    (id: string, signal?: AbortSignal) =>
      verificationRuntime.uploadEvidence(id, signal),
    [],
  );
  const deleteEvidence = useCallback(
    (id: string, evidenceId: string, signal?: AbortSignal) =>
      verificationRuntime.deleteEvidence(id, evidenceId, signal),
    [],
  );
  const getEvidenceContent = useCallback(
    (id: string, evidenceId: string, signal?: AbortSignal) =>
      verificationRuntime.getEvidenceContent(id, evidenceId, signal),
    [],
  );
  const clear = useCallback(() => verificationRuntime.clear(), []);

  return {
    snapshot,
    loadOverview,
    loadCaseDetails,
    startVerification,
    retryVerification,
    uploadEvidence,
    deleteEvidence,
    getEvidenceContent,
    clear,
  };
}
