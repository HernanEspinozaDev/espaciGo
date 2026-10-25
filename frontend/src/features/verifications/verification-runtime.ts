import "client-only";

import { VerificationFacade } from "./application/verification-facade";
import { OpenApiVerificationAdapter } from "./infrastructure/verification-openapi-adapter";

export const verificationAdapter = new OpenApiVerificationAdapter();
export const verificationFacade = new VerificationFacade(verificationAdapter);
export const verificationRuntime = verificationFacade;
