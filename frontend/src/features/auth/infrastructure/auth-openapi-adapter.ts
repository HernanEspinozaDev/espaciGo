import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { AuthPort, LoginRequest } from "../application/auth.port";
import type { AccessToken } from "../application/auth.port";

export class OpenApiAuthAdapter implements AuthPort {
  async login(input: LoginRequest, signal: AbortSignal) {
    const result = await apiClient.POST("/auth/login", {
      body: input,
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data || result.data.token_type !== "Bearer") {
      throw new ApiProtocolError();
    }

    return {
      accessToken: result.data.access_token as AccessToken,
      session: {
        accountId: result.data.account_id,
        roles: result.data.roles,
        capabilities: result.data.capabilities,
        restrictedMode: result.data.restricted_mode,
        expiresAt: result.data.expires_at,
      },
    };
  }

  async currentSession(accessToken: AccessToken, signal: AbortSignal) {
    const result = await apiClient.GET("/auth/session", {
      headers: { Authorization: `Bearer ${accessToken}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();

    return {
      accountId: result.data.account_id,
      roles: result.data.roles,
      capabilities: result.data.capabilities,
      restrictedMode: result.data.restricted_mode,
    };
  }

  async logout(accessToken: AccessToken, signal: AbortSignal): Promise<void> {
    const result = await apiClient.POST("/auth/logout", {
      headers: { Authorization: `Bearer ${accessToken}` },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }
}
