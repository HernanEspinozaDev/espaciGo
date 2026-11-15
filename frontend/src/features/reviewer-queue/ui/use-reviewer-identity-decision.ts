"use client";

import { useCallback, useState } from "react";
import { buildReviewerIdentityDecisionRequest } from "../domain/reviewer-case";
import type {
  IdentityReviewDraft,
  ReviewerIdentityDecisionResult,
} from "../domain/reviewer-case";
import { reviewerQueueFacade } from "../reviewer-queue-runtime";

export function useReviewerIdentityDecision(
  caseId: string,
  revision: number | string,
  onCommitted: (result: ReviewerIdentityDecisionResult) => void,
  onUncertain: () => void,
) {
  const [submitting, setSubmitting] = useState(false);
  const [failed, setFailed] = useState(false);

  const submit = useCallback(
    async (draft: IdentityReviewDraft): Promise<void> => {
      setFailed(false);
      if (!buildReviewerIdentityDecisionRequest(draft, revision)) {
        setFailed(true);
        return;
      }
      setSubmitting(true);
      try {
        const result = await reviewerQueueFacade.reviewAssignedIdentityCase(
          caseId,
          revision,
          draft,
        );
        onCommitted(result);
      } catch {
        onUncertain();
      } finally {
        setSubmitting(false);
      }
    },
    [caseId, onCommitted, onUncertain, revision],
  );

  return { failed, submitting, submit } as const;
}
