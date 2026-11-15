"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { authFacade } from "@/features/auth/auth-runtime";
import { isAbortError } from "@/lib/api/errors";
import type { ReviewerIdentityDetail } from "../domain/reviewer-case";
import { reviewerQueueFacade } from "../reviewer-queue-runtime";

export type ReviewerCaseDetailState = "loading" | "unavailable" | "ready";

interface DetailSnapshot {
  readonly sessionKey: string;
  readonly caseId: string;
  readonly requestKey: number;
  readonly state: ReviewerCaseDetailState;
  readonly detail: ReviewerIdentityDetail | null;
}

function sessionKey(
  snapshot: ReturnType<typeof authFacade.getSnapshot>,
): string {
  return JSON.stringify([
    snapshot.status,
    snapshot.accountId,
    [...snapshot.roles].sort(),
    [...snapshot.capabilities].sort(),
    snapshot.restrictedMode,
    snapshot.expiresAt,
  ]);
}

const initialSnapshot: DetailSnapshot = Object.freeze({
  sessionKey: "",
  caseId: "",
  requestKey: -1,
  state: "loading",
  detail: null,
});

export function useReviewerCaseDetail(caseId: string): {
  readonly state: ReviewerCaseDetailState;
  readonly detail: ReviewerIdentityDetail | null;
  readonly canReview: boolean;
  readonly reload: () => void;
  readonly invalidate: () => void;
} {
  const auth = useSyncExternalStore(
    authFacade.subscribe,
    authFacade.getSnapshot,
    authFacade.getServerSnapshot,
  );
  const currentSessionKey = sessionKey(auth);
  const canLoad = auth.status === "authenticated" && !auth.restrictedMode;
  const [snapshot, setSnapshot] = useState(initialSnapshot);
  const [reloadKey, setReloadKey] = useState(0);
  const reload = useCallback(() => setReloadKey((current) => current + 1), []);
  const invalidate = useCallback(() => setSnapshot(initialSnapshot), []);

  useEffect(() => authFacade.subscribe(() => setSnapshot(initialSnapshot)), []);

  useEffect(() => {
    let active = true;
    if (!canLoad) return;
    void reviewerQueueFacade
      .getAssignedIdentityDetail(caseId)
      .then((detail) => {
        if (active) {
          setSnapshot({
            sessionKey: currentSessionKey,
            caseId,
            requestKey: reloadKey,
            state: "ready",
            detail,
          });
        }
      })
      .catch((error: unknown) => {
        if (!active || isAbortError(error)) return;
        setSnapshot({
          sessionKey: currentSessionKey,
          caseId,
          requestKey: reloadKey,
          state: "unavailable",
          detail: null,
        });
      });

    return () => {
      active = false;
    };
  }, [canLoad, caseId, currentSessionKey, reloadKey]);

  const visibleSnapshot =
    snapshot.sessionKey === currentSessionKey &&
    snapshot.caseId === caseId &&
    snapshot.requestKey === reloadKey
      ? snapshot
      : null;
  return {
    state:
      visibleSnapshot?.state ??
      (auth.status === "checking" || canLoad ? "loading" : "unavailable"),
    detail: canLoad ? (visibleSnapshot?.detail ?? null) : null,
    canReview: canLoad,
    reload,
    invalidate,
  };
}
