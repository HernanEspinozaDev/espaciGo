import "client-only";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type {
  AccessToken,
  AuthPort,
  LoginRequest,
  PublicAuthSession,
} from "../application/auth.port";

async function sessionRequest(
  path: "login" | "session",
  signal: AbortSignal,
  input?: LoginRequest,
): Promise<PublicAuthSession> {
  const response = await fetch(`/api/backend/auth/${path}`, {
    method: path === "login" ? "POST" : "GET",
    headers: {
      Accept: "application/json",
      ...(input ? { "Content-Type": "application/json" } : {}),
    },
    ...(input ? { body: serializeExactJson(input) } : {}),
    credentials: "same-origin",
    cache: "no-store",
    signal,
  });
  const data: unknown = await response.json().catch(() => null);
  if (!response.ok) throw normalizeApiError(response, data);
  if (response.status !== 200 || !data || typeof data !== "object")
    throw new ApiProtocolError();
  const p = data as Record<string, unknown>;
  if (
    typeof p.account_id !== "string" ||
    !Array.isArray(p.roles) ||
    !Array.isArray(p.capabilities) ||
    typeof p.restricted_mode !== "boolean" ||
    typeof p.expires_at !== "string" ||
    !Number.isFinite(Date.parse(p.expires_at)) ||
    "access_token" in p
  )
    throw new ApiProtocolError();
  return {
    accountId: p.account_id,
    roles: p.roles as PublicAuthSession["roles"],
    capabilities: p.capabilities as PublicAuthSession["capabilities"],
    restrictedMode: p.restricted_mode,
    expiresAt: p.expires_at,
  };
}

export class OpenApiAuthAdapter implements AuthPort {
  async login(input: LoginRequest, signal: AbortSignal) {
    const session = await sessionRequest("login", signal, input);
    return { accessToken: "bff-session" as AccessToken, session };
  }
  async currentSession(accessToken: AccessToken, signal: AbortSignal) {
    void accessToken;
    return sessionRequest("session", signal);
  }
  async logout(accessToken: AccessToken, signal: AbortSignal): Promise<void> {
    void accessToken;
    const response = await fetch("/api/backend/auth/logout", {
      method: "POST",
      credentials: "same-origin",
      cache: "no-store",
      signal,
    });
    if (response.status !== 204)
      throw normalizeApiError(
        response,
        await response.json().catch(() => null),
      );
  }
}
