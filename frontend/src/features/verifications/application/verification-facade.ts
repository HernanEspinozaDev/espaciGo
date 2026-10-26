"use client";

import "client-only";

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
  #sessionController = new AbortController();
  #epoch = 0;
  #snapshot: VerificationSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();

  constructor(port: VerificationPort) {
    this.#port = port;
  }

  #isCurrent(epoch: number, signal?: AbortSignal): boolean {
    return epoch === this.#epoch && !signal?.aborted;
  }

  #guard(epoch: number, signal: AbortSignal): void {
    if (!this.#isCurrent(epoch, signal)) {
      throw (
        signal.reason ??
        new DOMException("The operation was aborted", "AbortError")
      );
    }
  }

  #createSignal(signal?: AbortSignal): { epoch: number; signal: AbortSignal } {
    const epoch = this.#epoch;
    const signals = [this.#sessionController.signal];
    if (signal) signals.push(signal);
    return { epoch, signal: AbortSignal.any(signals) };
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

  async loadOverview(signal?: AbortSignal): Promise<VerificationSnapshot> {
    this.cancel();
    const controller = new AbortController();
    this.#abortController = controller;
    const epoch = this.#epoch;
    const signals = [controller.signal, this.#sessionController.signal];
    if (signal) signals.push(signal);
    const activeSignal = AbortSignal.any(signals);

    this.#setSnapshot({ loading: true, error: null });

    try {
      const [verifications, eligibility] = await Promise.all([
        this.#port.listVerifications(activeSignal),
        this.#port.getEligibility(activeSignal),
      ]);

      if (
        !this.#isCurrent(epoch, activeSignal) ||
        this.#abortController !== controller
      ) {
        return this.#snapshot;
      }

      this.#setSnapshot({
        verifications: Object.freeze(verifications),
        eligibility: Object.freeze(eligibility),
        loading: false,
        error: null,
      });
      return this.#snapshot;
    } catch (err) {
      if (
        !this.#isCurrent(epoch, activeSignal) ||
        this.#abortController !== controller
      ) {
        throw err;
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar verificaciones";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      if (this.#abortController === controller) {
        this.#abortController = null;
      }
    }
  }

  async loadCaseDetails(
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationCase | null> {
    const { epoch, signal: activeSignal } = this.#createSignal(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const [currentCase, history, evidence] = await Promise.all([
        this.#port.getVerification(id, activeSignal),
        this.#port.listHistory(id, activeSignal),
        this.#port.listEvidence(id, activeSignal),
      ]);

      if (!this.#isCurrent(epoch, activeSignal)) {
        return null;
      }

      this.#setSnapshot({
        currentCase,
        history: Object.freeze(history),
        evidence: Object.freeze(evidence),
        loading: false,
        error: null,
      });
      return currentCase;
    } catch (err) {
      if (!this.#isCurrent(epoch, activeSignal)) {
        throw err;
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar caso";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async startVerification(
    idempotencyKey: string,
    type: VerificationType,
    signal?: AbortSignal,
  ): Promise<VerificationCase> {
    const { epoch, signal: activeSignal } = this.#createSignal(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const created = await this.#port.startVerification(
        idempotencyKey,
        type,
        activeSignal,
      );
      this.#guard(epoch, activeSignal);
      const verifications = await this.#port.listVerifications(activeSignal);
      this.#guard(epoch, activeSignal);
      this.#setSnapshot({
        verifications: Object.freeze(verifications),
        currentCase: created,
        loading: false,
        error: null,
      });
      return created;
    } catch (err) {
      if (!this.#isCurrent(epoch, activeSignal)) {
        throw err;
      }
      const message =
        err instanceof Error ? err.message : "Error al iniciar verificación";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async retryVerification(
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
    signal?: AbortSignal,
  ): Promise<VerificationCase> {
    const { epoch, signal: activeSignal } = this.#createSignal(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const retried = await this.#port.retryVerification(
        id,
        idempotencyKey,
        correctionCode,
        activeSignal,
      );
      this.#guard(epoch, activeSignal);
      const verifications = await this.#port.listVerifications(activeSignal);
      this.#guard(epoch, activeSignal);
      this.#setSnapshot({
        verifications: Object.freeze(verifications),
        currentCase: retried,
        loading: false,
        error: null,
      });
      return retried;
    } catch (err) {
      if (!this.#isCurrent(epoch, activeSignal)) {
        throw err;
      }
      const message =
        err instanceof Error ? err.message : "Error al reintentar verificación";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async uploadEvidence(
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence> {
    const { epoch, signal: activeSignal } = this.#createSignal(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const uploaded = await this.#port.uploadEvidence(id, activeSignal);
      this.#guard(epoch, activeSignal);
      const evidence = await this.#port.listEvidence(id, activeSignal);
      this.#guard(epoch, activeSignal);
      this.#setSnapshot({
        evidence: Object.freeze(evidence),
        loading: false,
        error: null,
      });
      return uploaded;
    } catch (err) {
      if (!this.#isCurrent(epoch, activeSignal)) {
        throw err;
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar evidencia";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async deleteEvidence(
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const { epoch, signal: activeSignal } = this.#createSignal(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.deleteEvidence(id, evidenceId, activeSignal);
      this.#guard(epoch, activeSignal);
      const evidence = await this.#port.listEvidence(id, activeSignal);
      this.#guard(epoch, activeSignal);
      this.#setSnapshot({
        evidence: Object.freeze(evidence),
        loading: false,
        error: null,
      });
    } catch (err) {
      if (!this.#isCurrent(epoch, activeSignal)) {
        throw err;
      }
      const message =
        err instanceof Error ? err.message : "Error al eliminar evidencia";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async getEvidenceContent(
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null> {
    const { epoch, signal: activeSignal } = this.#createSignal(signal);
    const content = await this.#port.getEvidenceContent(
      id,
      evidenceId,
      activeSignal,
    );
    if (!this.#isCurrent(epoch, activeSignal)) {
      return null;
    }
    return content;
  }

  clear(): void {
    this.#epoch++;
    this.#sessionController.abort();
    this.#sessionController = new AbortController();
    this.cancel();
    this.#setSnapshot(initialSnapshot);
  }
}
