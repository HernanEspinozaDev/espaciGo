"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { isAbortError } from "@/lib/api/errors";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import {
  reviewerIdentityEvidenceResourceKey,
  type ReviewerIdentityFile,
} from "../domain/reviewer-case";
import { reviewerQueueFacade } from "../reviewer-queue-runtime";

export type ReviewerEvidenceState =
  "idle" | "loading" | "unavailable" | "ready";

interface EvidenceSnapshot {
  readonly resourceKey: string;
  readonly state: ReviewerEvidenceState;
  readonly url: string | null;
}

const emptySnapshot: EvidenceSnapshot = Object.freeze({
  resourceKey: "",
  state: "idle",
  url: null,
});

export function useReviewerIdentityEvidence(
  caseId: string,
  file: ReviewerIdentityFile,
): {
  readonly state: ReviewerEvidenceState;
  readonly url: string | null;
  readonly load: () => Promise<void>;
} {
  const resourceKey = reviewerIdentityEvidenceResourceKey(caseId, file);
  const [snapshot, setSnapshot] = useState(emptySnapshot);
  const mounted = useRef(true);
  const request = useRef<AbortController | null>(null);
  const current =
    snapshot.resourceKey === resourceKey ? snapshot : emptySnapshot;

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      request.current?.abort();
      privateBlobUrls.revoke(resourceKey);
    };
  }, [resourceKey]);

  const load = useCallback(async () => {
    if (current.state === "loading" || current.state === "ready") return;
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setSnapshot({ resourceKey, state: "loading", url: null });
    try {
      const blob = await reviewerQueueFacade.downloadAssignedIdentityFile(
        caseId,
        file,
        controller.signal,
      );
      if (!mounted.current || controller.signal.aborted) return;
      const privateUrl = privateBlobUrls.create(resourceKey, blob);
      setSnapshot({ resourceKey, state: "ready", url: privateUrl.url });
    } catch (error) {
      if (
        !mounted.current ||
        controller.signal.aborted ||
        isAbortError(error)
      ) {
        return;
      }
      setSnapshot({ resourceKey, state: "unavailable", url: null });
    }
  }, [caseId, current.state, file, resourceKey]);

  return { state: current.state, url: current.url, load };
}
