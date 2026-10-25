"use client";

import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import type { VerificationPort } from "./verification.port";
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationEligibilityItem,
  VerificationHistoryEntry,
  VerificationType,
} from "../domain/verification";

export interface VerificationSnapshot {
  readonly verifications: readonly VerificationCase[];
  readonly eligibility: readonly VerificationEligibilityItem[];
  readonly currentCase: VerificationCase | null;
  readonly history: readonly VerificationHistoryEntry[];
  readonly evidence: readonly SyntheticEvidence[];
  readonly loading: boolean;
  readonly error: string | null;
}

const initialSnapshot: VerificationSnapshot = Object.freeze({
  verifications: Object.freeze([]),
  eligibility: Object.freeze([]),
  currentCase: null,
  history: Object.freeze([]),
  evidence: Object.freeze([]),
  loading: false,
  error: null,
});

export class VerificationFacade {
  #port: VerificationPort;
  #abortController: AbortController | null = null;
  #snapshot: VerificationSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();

  constructor(port: VerificationPort) {
    this.#port = port;
  }

  getSnapshot = (): VerificationSnapshot => {
    return this.#snapshot;
  };

  getServerSnapshot = (): VerificationSnapshot => {
    return initialSnapshot;
  };

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  cancel(): void {
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
  }

  #setSnapshot(partial: Partial<VerificationSnapshot>): void {
    this.#snapshot = Object.freeze({
      ...this.#snapshot,
      ...partial,
    });
    for (const listener of this.#listeners) {
      listener();
    }
  }

  async loadOverview(token: AccessToken): Promise<VerificationSnapshot> {
    this.cancel();
    this.#abortController = new AbortController();
    const signal = this.#abortController.signal;

    this.#setSnapshot({ loading: true, error: null });

    try {
      const [verifications, eligibility] = await Promise.all([
        this.#port.listVerifications(token, signal),
        this.#port.getEligibility(token, signal),
      ]);

      this.#setSnapshot({
        verifications: Object.freeze(verifications),
        eligibility: Object.freeze(eligibility),
        loading: false,
        error: null,
      });
      return this.#snapshot;
    } catch (err) {
      if (signal.aborted) throw err;
      const message =
        err instanceof Error ? err.message : "Error al cargar verificaciones";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#abortController = null;
    }
  }

  async loadCaseDetails(
    token: AccessToken,
    id: string,
  ): Promise<VerificationCase | null> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const [currentCase, history, evidence] = await Promise.all([
        this.#port.getVerification(token, id),
        this.#port.listHistory(token, id),
        this.#port.listEvidence(token, id),
      ]);

      this.#setSnapshot({
        currentCase,
        history: Object.freeze(history),
        evidence: Object.freeze(evidence),
        loading: false,
        error: null,
      });
      return currentCase;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al cargar caso";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async startVerification(
    token: AccessToken,
    idempotencyKey: string,
    type: VerificationType,
  ): Promise<VerificationCase> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const created = await this.#port.startVerification(
        token,
        idempotencyKey,
        type,
      );
      const verifications = await this.#port.listVerifications(token);
      this.#setSnapshot({
        verifications: Object.freeze(verifications),
        currentCase: created,
        loading: false,
        error: null,
      });
      return created;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al iniciar verificación";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async retryVerification(
    token: AccessToken,
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
  ): Promise<VerificationCase> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const retried = await this.#port.retryVerification(
        token,
        id,
        idempotencyKey,
        correctionCode,
      );
      const verifications = await this.#port.listVerifications(token);
      this.#setSnapshot({
        verifications: Object.freeze(verifications),
        currentCase: retried,
        loading: false,
        error: null,
      });
      return retried;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al reintentar verificación";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async uploadEvidence(
    token: AccessToken,
    id: string,
  ): Promise<SyntheticEvidence> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const uploaded = await this.#port.uploadEvidence(token, id);
      const evidence = await this.#port.listEvidence(token, id);
      this.#setSnapshot({
        evidence: Object.freeze(evidence),
        loading: false,
        error: null,
      });
      return uploaded;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al cargar evidencia";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async deleteEvidence(
    token: AccessToken,
    id: string,
    evidenceId: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.deleteEvidence(token, id, evidenceId);
      const evidence = await this.#port.listEvidence(token, id);
      this.#setSnapshot({
        evidence: Object.freeze(evidence),
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al eliminar evidencia";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async getEvidenceContent(
    token: AccessToken,
    id: string,
    evidenceId: string,
  ): Promise<Blob | null> {
    return this.#port.getEvidenceContent(token, id, evidenceId);
  }

  clear(): void {
    this.cancel();
    this.#setSnapshot(initialSnapshot);
  }
}
