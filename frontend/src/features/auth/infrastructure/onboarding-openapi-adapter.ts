import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type {
  EmailRequest,
  OnboardingPort,
  RecoveryConsumeRequest,
  RegisterRequest,
} from "../application/onboarding.port";
import type { ActionCredential } from "../domain/action-credential";

function requireOk(response: Response, error: unknown): void {
  if (!response.ok) throw normalizeApiError(response, error);
}

export class OpenApiOnboardingAdapter implements OnboardingPort {
  async terms(signal: AbortSignal) {
    const result = await apiClient.GET("/auth/terms", { signal });
    requireOk(result.response, result.error);
    if (result.response.status !== 200) throw new ApiProtocolError();
    if (!result.data) throw new ApiProtocolError();
    return result.data;
  }

  async register(
    input: RegisterRequest,
    signal: AbortSignal,
  ): Promise<201 | 202> {
    const result = await apiClient.POST("/auth/register", {
      body: input,
      bodySerializer: serializeExactJson,
      signal,
    });
    requireOk(result.response, result.error);
    if (result.response.status !== 201 && result.response.status !== 202) {
      throw new ApiProtocolError();
    }
    return result.response.status;
  }

  async reissueVerification(
    input: EmailRequest,
    signal: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST("/auth/verification/reissue", {
      body: input,
      bodySerializer: serializeExactJson,
      signal,
    });
    requireOk(result.response, result.error);
    if (result.response.status !== 204) throw new ApiProtocolError();
  }

  async verifyEmail(
    credential: ActionCredential,
    signal: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST("/auth/verification", {
      body: { token_id: credential.tokenId, token: credential.token },
      bodySerializer: serializeExactJson,
      signal,
    });
    requireOk(result.response, result.error);
    if (result.response.status !== 204) throw new ApiProtocolError();
  }

  async requestRecovery(
    input: EmailRequest,
    signal: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST("/auth/password/recovery", {
      body: input,
      bodySerializer: serializeExactJson,
      signal,
    });
    requireOk(result.response, result.error);
    if (result.response.status !== 202) throw new ApiProtocolError();
    if (
      !result.data ||
      result.data.status !== "accepted" ||
      typeof result.data.message !== "string" ||
      !result.data.message.trim()
    ) {
      throw new ApiProtocolError();
    }
  }

  async consumeRecovery(
    input: RecoveryConsumeRequest,
    signal: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.POST("/auth/password/recovery/consume", {
      body: input,
      bodySerializer: serializeExactJson,
      signal,
    });
    requireOk(result.response, result.error);
    if (result.response.status !== 204) throw new ApiProtocolError();
  }
}
