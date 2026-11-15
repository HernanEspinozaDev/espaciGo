import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { ReviewerQueueFacade } from "./application/reviewer-queue-facade";
import { OpenApiReviewerQueueAdapter } from "./infrastructure/reviewer-queue-openapi-adapter";

const reviewerQueuePort = new OpenApiReviewerQueueAdapter();

export const reviewerQueueFacade = new ReviewerQueueFacade(
  reviewerQueuePort,
  authFacade,
);
