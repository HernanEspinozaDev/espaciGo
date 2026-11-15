"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { authFacade } from "@/features/auth/auth-runtime";
import { isAbortError } from "@/lib/api/errors";
import type { ReviewerSummary } from "../domain/reviewer-case";
import { reviewerQueueFacade } from "../reviewer-queue-runtime";

export type ReviewerQueueViewState =
  "loading" | "unavailable" | "empty" | "ready";

interface QueueSnapshot {
  readonly sessionKey: string;
  readonly state: ReviewerQueueViewState;
  readonly rows: readonly ReviewerSummary[];
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

const initialSnapshot: QueueSnapshot = Object.freeze({
  sessionKey: "",
  state: "loading",
  rows: Object.freeze([]),
});

export function useReviewerQueue(): {
  readonly state: ReviewerQueueViewState;
  readonly rows: readonly ReviewerSummary[];
} {
  const auth = useSyncExternalStore(
    authFacade.subscribe,
    authFacade.getSnapshot,
    authFacade.getServerSnapshot,
  );
  const currentSessionKey = sessionKey(auth);
  const canLoad = auth.status === "authenticated" && !auth.restrictedMode;
  const [snapshot, setSnapshot] = useState(initialSnapshot);

  useEffect(() => authFacade.subscribe(() => setSnapshot(initialSnapshot)), []);

  useEffect(() => {
    if (!canLoad) return;
    let current = true;

    void reviewerQueueFacade
      .listAssigned()
      .then((rows) => {
        if (!current) return;
        setSnapshot({
          sessionKey: currentSessionKey,
          state: rows.length > 0 ? "ready" : "empty",
          rows,
        });
      })
      .catch((error: unknown) => {
        if (!current || isAbortError(error)) return;
        setSnapshot({
          sessionKey: currentSessionKey,
          state: "unavailable",
          rows: [],
        });
      });

    return () => {
      current = false;
    };
  }, [canLoad, currentSessionKey]);

  const visibleSnapshot =
    snapshot.sessionKey === currentSessionKey ? snapshot : null;
  return {
    state:
      visibleSnapshot?.state ??
      (auth.status === "checking" || canLoad ? "loading" : "unavailable"),
    rows: canLoad ? (visibleSnapshot?.rows ?? []) : [],
  };
}
