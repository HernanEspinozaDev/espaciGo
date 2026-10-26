import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import { VerificationFacade } from "./application/verification-facade";
import { bindVerificationSession } from "./application/verification-session";
import { OpenApiVerificationAdapter } from "./infrastructure/verification-openapi-adapter";

export const verificationAdapter = new OpenApiVerificationAdapter();
export const verificationFacade = new VerificationFacade(verificationAdapter);
export const verificationRuntime = bindVerificationSession(
  authFacade,
  verificationFacade,
  () => privateBlobUrls.clear(),
);
