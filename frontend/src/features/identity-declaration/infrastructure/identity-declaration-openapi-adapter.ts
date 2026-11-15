import "client-only";
import type { AccessToken } from "@/features/auth/application/auth.port";
import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { IdentityDeclarationPort } from "../application/identity-declaration.port";
import {
  parseIdentityDeclarationResponse,
  type IdentityDeclarationRequest,
} from "../domain/identity-declaration";

const auth = (token: AccessToken) => ({
  Authorization: ["Bearer", token].join(" "),
});
export class IdentityDeclarationOpenApiAdapter implements IdentityDeclarationPort {
  async get(token: AccessToken, signal?: AbortSignal) {
    const result = await apiClient.GET("/verifications/identity-declaration", {
      headers: auth(token),
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    try {
      return parseIdentityDeclarationResponse(result.data);
    } catch {
      throw new ApiProtocolError();
    }
  }
  async save(
    token: AccessToken,
    request: IdentityDeclarationRequest,
    signal?: AbortSignal,
  ) {
    const result = await apiClient.POST("/verifications/identity-declaration", {
      headers: auth(token),
      body: request,
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok)
      throw normalizeApiError(result.response, result.error);
    try {
      return parseIdentityDeclarationResponse(result.data);
    } catch {
      throw new ApiProtocolError();
    }
  }
}
