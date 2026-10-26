import "client-only";

import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type {
  AuthLogoutResult,
  AuthPort,
  LoginRequest,
  PublicAuthSession,
} from "../application/auth.port";

function parseSessionPayload(data: unknown): PublicAuthSession {
  if (typeof data !== "object" || data === null || Array.isArray(data)) {
    throw new ApiProtocolError();
  }
  const rec = data as Record<string, unknown>;
  if (
    typeof rec.account_id !== "string" ||
    !Array.isArray(rec.roles) ||
    !Array.isArray(rec.capabilities) ||
    typeof rec.restricted_mode !== "boolean" ||
    typeof rec.expires_at !== "string"
  ) {
    throw new ApiProtocolError();
  }
  if (Number.isNaN(Date.parse(rec.expires_at))) {
    throw new ApiProtocolError();
  }
  return {
    accountId: rec.account_id,
    roles: rec.roles as PublicAuthSession["roles"],
    capabilities: rec.capabilities as PublicAuthSession["capabilities"],
    restrictedMode: rec.restricted_mode,
    expiresAt: rec.expires_at,
  };
}

async function extractErrorPayload(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

export class OpenApiAuthAdapter implements AuthPort {
  async login(
    input: LoginRequest,
    signal: AbortSignal,
  ): Promise<PublicAuthSession> {
    const response = await fetch("/api/backend/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: serializeExactJson(input),
      credentials: "same-origin",
      cache: "no-store",
      signal,
    });

    if (!response.ok) {
      const errorPayload = await extractErrorPayload(response);
      throw normalizeApiError(response, errorPayload);
    }
    if (response.status !== 200) {
      throw new ApiProtocolError();
    }

    const data = await response.json().catch(() => {
      throw new ApiProtocolError();
    });
    return parseSessionPayload(data);
  }

  async currentSession(signal: AbortSignal): Promise<PublicAuthSession> {
    const response = await fetch("/api/backend/auth/session", {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
      credentials: "same-origin",
      cache: "no-store",
      signal,
    });

    if (!response.ok) {
      const errorPayload = await extractErrorPayload(response);
      throw normalizeApiError(response, errorPayload);
    }
    if (response.status !== 200) {
      throw new ApiProtocolError();
    }

    const data = await response.json().catch(() => {
      throw new ApiProtocolError();
    });
    return parseSessionPayload(data);
  }

  async logout(signal: AbortSignal): Promise<AuthLogoutResult> {
    const response = await fetch("/api/backend/auth/logout", {
      method: "POST",
      headers: {
        Accept: "application/json",
      },
      credentials: "same-origin",
      cache: "no-store",
      signal,
    });

    const clearedHeader =
      response.headers.get("x-espacigo-session-cleared") === "1";

    if (response.status === 204) {
      return {
        cookieCleared: true,
        revocationConfirmed: true,
      };
    }

    if (response.status === 502 && clearedHeader) {
      return {
        cookieCleared: true,
        revocationConfirmed: false,
      };
    }

    const errorPayload = await extractErrorPayload(response);
    throw normalizeApiError(response, errorPayload);
  }
}
