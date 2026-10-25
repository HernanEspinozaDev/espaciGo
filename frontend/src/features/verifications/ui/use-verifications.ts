"use client";

import { useCallback, useSyncExternalStore } from "react";
import { verificationRuntime } from "../verification-runtime";
import type { VerificationSnapshot } from "../application/verification-facade";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationType,
} from "../domain/verification";

export interface VerificationView {
  snapshot: VerificationSnapshot;
  loadOverview(token: AccessToken): Promise<VerificationSnapshot>;
  loadCaseDetails(
    token: AccessToken,
    id: string,
  ): Promise<VerificationCase | null>;
  startVerification(
    token: AccessToken,
    idempotencyKey: string,
    type: VerificationType,
  ): Promise<VerificationCase>;
  retryVerification(
    token: AccessToken,
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
  ): Promise<VerificationCase>;
  uploadEvidence(token: AccessToken, id: string): Promise<SyntheticEvidence>;
  deleteEvidence(
    token: AccessToken,
    id: string,
    evidenceId: string,
  ): Promise<void>;
  getEvidenceContent(
    token: AccessToken,
    id: string,
    evidenceId: string,
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
    (token: AccessToken) => verificationRuntime.loadOverview(token),
    [],
  );
  const loadCaseDetails = useCallback(
    (token: AccessToken, id: string) =>
      verificationRuntime.loadCaseDetails(token, id),
    [],
  );
  const startVerification = useCallback(
    (token: AccessToken, idempotencyKey: string, type: VerificationType) =>
      verificationRuntime.startVerification(token, idempotencyKey, type),
    [],
  );
  const retryVerification = useCallback(
    (
      token: AccessToken,
      id: string,
      idempotencyKey: string,
      correctionCode: VerificationCorrectionCode,
    ) =>
      verificationRuntime.retryVerification(
        token,
        id,
        idempotencyKey,
        correctionCode,
      ),
    [],
  );
  const uploadEvidence = useCallback(
    (token: AccessToken, id: string) =>
      verificationRuntime.uploadEvidence(token, id),
    [],
  );
  const deleteEvidence = useCallback(
    (token: AccessToken, id: string, evidenceId: string) =>
      verificationRuntime.deleteEvidence(token, id, evidenceId),
    [],
  );
  const getEvidenceContent = useCallback(
    (token: AccessToken, id: string, evidenceId: string) =>
      verificationRuntime.getEvidenceContent(token, id, evidenceId),
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
