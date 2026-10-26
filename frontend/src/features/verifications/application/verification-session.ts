import type { AuthSnapshot } from "@/features/auth/application/auth-facade";
import type { VerificationSnapshot } from "./verification-facade";
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationCorrectionCode,
  VerificationType,
} from "../domain/verification";

export interface AuthSourceLike {
  getSnapshot(): AuthSnapshot;
  subscribe(listener: () => void): () => void;
  checkSession?(): Promise<boolean>;
}

export interface VerificationFacadeLike {
  getSnapshot(): VerificationSnapshot;
  getServerSnapshot?(): VerificationSnapshot;
  subscribe(listener: () => void): () => void;
  clear(): void;
  cancel?(): void;
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
}

export interface VerificationSessionRuntime {
  getSnapshot(): VerificationSnapshot;
  getServerSnapshot(): VerificationSnapshot;
  subscribe(listener: () => void): () => void;
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
  cancel(): void;
}

function haveEqualArrays(a: readonly string[], b: readonly string[]): boolean {
  if (a === b) return true;
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

function isSameAuthenticatedSession(
  prev: AuthSnapshot,
  current: AuthSnapshot,
): boolean {
  return (
    prev.status === "authenticated" &&
    current.status === "authenticated" &&
    prev.accountId === current.accountId &&
    prev.restrictedMode === current.restrictedMode &&
    prev.expiresAt === current.expiresAt &&
    haveEqualArrays(prev.roles, current.roles) &&
    haveEqualArrays(prev.capabilities, current.capabilities)
  );
}

function isAuthError(error: unknown): boolean {
  if (typeof error === "object" && error !== null && "status" in error) {
    const status = (error as { status?: unknown }).status;
    return status === 401 || status === 403;
  }
  return false;
}

export function bindVerificationSession(
  auth: AuthSourceLike,
  facade: VerificationFacadeLike,
  onPrivatePurge?: (() => void) | { onPrivatePurge?: () => void },
): VerificationSessionRuntime {
  const purgeBlobs =
    typeof onPrivatePurge === "function"
      ? onPrivatePurge
      : onPrivatePurge?.onPrivatePurge;
  let sessionGeneration = 0;
  let lastAuth = auth.getSnapshot();
  let sessionAbortController: AbortController | null = null;

  auth.subscribe(() => {
    const current = auth.getSnapshot();
    if (lastAuth.status === "authenticated") {
      if (
        current.status !== "authenticated" ||
        !isSameAuthenticatedSession(lastAuth, current)
      ) {
        sessionGeneration++;
        if (sessionAbortController) {
          sessionAbortController.abort();
          sessionAbortController = null;
        }
        facade.clear();
        purgeBlobs?.();
      }
    }
    lastAuth = current;
  });

  const ensureAuthenticated = (): void => {
    const current = auth.getSnapshot();
    if (current.status !== "authenticated") {
      throw new Error("unauthenticated");
    }
  };

  const getSessionSignal = (): AbortSignal => {
    if (!sessionAbortController || sessionAbortController.signal.aborted) {
      sessionAbortController = new AbortController();
    }
    return sessionAbortController.signal;
  };

  const combineSignals = (signal?: AbortSignal): AbortSignal => {
    const sessionSignal = getSessionSignal();
    if (!signal) return sessionSignal;
    if (typeof AbortSignal.any === "function") {
      return AbortSignal.any([sessionSignal, signal]);
    }
    const controller = new AbortController();
    const abortHandler = () => controller.abort();
    if (sessionSignal.aborted || signal.aborted) {
      controller.abort();
      return controller.signal;
    }
    sessionSignal.addEventListener("abort", abortHandler, { once: true });
    signal.addEventListener("abort", abortHandler, { once: true });
    return controller.signal;
  };

  const withSessionOp = async <T>(
    signal: AbortSignal | undefined,
    op: (signal: AbortSignal) => Promise<T>,
  ): Promise<T> => {
    ensureAuthenticated();
    const initialAuth = auth.getSnapshot();
    const initialGen = sessionGeneration;
    const combinedSignal = combineSignals(signal);

    try {
      return await op(combinedSignal);
    } catch (error) {
      if (isAuthError(error)) {
        if (sessionAbortController) {
          sessionAbortController.abort();
          sessionAbortController = null;
        }
        facade.clear();
        purgeBlobs?.();

        if (typeof auth.checkSession === "function") {
          try {
            await auth.checkSession();
          } catch {
            // ignore revalidation error
          }
        }

        const currentAuth = auth.getSnapshot();
        if (
          initialGen !== sessionGeneration ||
          !isSameAuthenticatedSession(initialAuth, currentAuth)
        ) {
          throw new DOMException("The operation was aborted", "AbortError");
        }
      }
      throw error;
    }
  };

  const loadOverview = (signal?: AbortSignal): Promise<VerificationSnapshot> =>
    withSessionOp(signal, (s) => facade.loadOverview(s));

  const loadCaseDetails = (
    id: string,
    signal?: AbortSignal,
  ): Promise<VerificationCase | null> =>
    withSessionOp(signal, (s) => facade.loadCaseDetails(id, s));

  const startVerification = (
    idempotencyKey: string,
    type: VerificationType,
    signal?: AbortSignal,
  ): Promise<VerificationCase> =>
    withSessionOp(signal, (s) =>
      facade.startVerification(idempotencyKey, type, s),
    );

  const retryVerification = (
    id: string,
    idempotencyKey: string,
    correctionCode: VerificationCorrectionCode,
    signal?: AbortSignal,
  ): Promise<VerificationCase> =>
    withSessionOp(signal, (s) =>
      facade.retryVerification(id, idempotencyKey, correctionCode, s),
    );

  const uploadEvidence = (
    id: string,
    signal?: AbortSignal,
  ): Promise<SyntheticEvidence> =>
    withSessionOp(signal, (s) => facade.uploadEvidence(id, s));

  const deleteEvidence = (
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<void> =>
    withSessionOp(signal, (s) => facade.deleteEvidence(id, evidenceId, s));

  const getEvidenceContent = (
    id: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null> =>
    withSessionOp(signal, (s) => facade.getEvidenceContent(id, evidenceId, s));

  const clear = (): void => {
    sessionGeneration++;
    if (sessionAbortController) {
      sessionAbortController.abort();
      sessionAbortController = null;
    }
    facade.clear();
    purgeBlobs?.();
  };

  const cancel = (): void => {
    if (sessionAbortController) {
      sessionAbortController.abort();
      sessionAbortController = null;
    }
    if (typeof facade.cancel === "function") {
      facade.cancel();
    }
  };

  const getSnapshot = (): VerificationSnapshot => facade.getSnapshot();
  const getServerSnapshot = (): VerificationSnapshot =>
    typeof facade.getServerSnapshot === "function"
      ? facade.getServerSnapshot()
      : facade.getSnapshot();
  const subscribe = (listener: () => void): (() => void) =>
    facade.subscribe(listener);

  return {
    getSnapshot,
    getServerSnapshot,
    subscribe,
    loadOverview,
    loadCaseDetails,
    startVerification,
    retryVerification,
    uploadEvidence,
    deleteEvidence,
    getEvidenceContent,
    clear,
    cancel,
  };
}
