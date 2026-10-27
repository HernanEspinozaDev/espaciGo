import type {
  components,
  Readable,
  Writable,
} from "@/lib/api/generated/openapi";
import type { ActionCredential } from "../domain/action-credential";

export type TermsResponse = Readable<components["schemas"]["TermsResponse"]>;
export type RegisterRequest = Writable<
  components["schemas"]["RegisterRequest"]
>;
export type EmailRequest = Writable<components["schemas"]["EmailRequest"]>;
export type RecoveryConsumeRequest = Writable<
  components["schemas"]["RecoveryConsumeRequest"]
>;

export type ActionLinkStatus = "ready" | "used" | "expired" | "invalid";

export interface OnboardingPort {
  terms(signal: AbortSignal): Promise<TermsResponse>;
  register(input: RegisterRequest, signal: AbortSignal): Promise<201 | 202>;
  reissueVerification(input: EmailRequest, signal: AbortSignal): Promise<void>;
  verifyEmail(credential: ActionCredential, signal: AbortSignal): Promise<void>;
  inspectVerification(
    credential: ActionCredential,
    signal: AbortSignal,
  ): Promise<ActionLinkStatus>;
  requestRecovery(input: EmailRequest, signal: AbortSignal): Promise<void>;
  consumeRecovery(
    input: RecoveryConsumeRequest,
    signal: AbortSignal,
  ): Promise<void>;
  inspectRecovery(
    credential: ActionCredential,
    signal: AbortSignal,
  ): Promise<ActionLinkStatus>;
}
