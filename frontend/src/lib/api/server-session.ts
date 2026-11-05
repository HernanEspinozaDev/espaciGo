import "server-only";

export const SESSION_COOKIE_NAME = "__Host-espacigo-session";
export const EXPIRY_COOKIE_NAME = "__Host-espacigo-session-expiry";

export interface BrowserSession {
  account_id: string;
  roles: readonly string[];
  capabilities: readonly string[];
  restricted_mode: boolean;
  expires_at: string;
}

export interface ExtractedSession {
  token: string;
  expiresAt: string;
}

const safeOpaqueToken = /^[A-Za-z0-9._~+/-]{16,2048}=*$/;
const rfc3339Timestamp =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

export function sanitizeHostHeader(host: string | null): string | null {
  if (!host) return null;
  const trimmed = host.trim();
  if (
    trimmed.length === 0 ||
    trimmed.length > 255 ||
    /[\s,/@\?#]/.test(trimmed)
  ) {
    return null;
  }
  return trimmed;
}

export function validateCsrfOrigin(
  request: Request,
  options: { isMutation: boolean },
): boolean {
  const host = sanitizeHostHeader(request.headers.get("host"));
  if (!host) return false;

  const secFetchSite = request.headers.get("sec-fetch-site");
  if (secFetchSite && secFetchSite.toLowerCase() === "cross-site") {
    return false;
  }

  const origin = request.headers.get("origin");
  if (!origin) {
    return !options.isMutation;
  }

  if (origin === "null" || origin.length > 512) {
    return false;
  }

  try {
    const parsedOrigin = new URL(origin);
    if (parsedOrigin.protocol !== "https:") {
      return false;
    }
    if (
      parsedOrigin.username !== "" ||
      parsedOrigin.password !== "" ||
      parsedOrigin.pathname !== "/" ||
      parsedOrigin.search !== "" ||
      parsedOrigin.hash !== "" ||
      parsedOrigin.origin !== origin
    ) {
      return false;
    }
    const requestOrigin = new URL(`${parsedOrigin.protocol}//${host}`);
    return (
      requestOrigin.username === "" &&
      requestOrigin.password === "" &&
      requestOrigin.pathname === "/" &&
      requestOrigin.search === "" &&
      requestOrigin.hash === "" &&
      requestOrigin.origin === parsedOrigin.origin
    );
  } catch {
    return false;
  }
}

export function parseSessionCookies(
  cookieHeader: string | null,
): ExtractedSession | null {
  if (!cookieHeader) return null;

  const cookieEntries = cookieHeader.split(";");
  let rawToken: string | null = null;
  let rawExpiry: string | null = null;
  let tokenCount = 0;
  let expiryCount = 0;

  for (const entry of cookieEntries) {
    const parts = entry.split("=");
    if (parts.length < 2) continue;
    const name = parts[0].trim();
    const val = parts.slice(1).join("=").trim();

    if (name === SESSION_COOKIE_NAME) {
      tokenCount++;
      rawToken = val;
    } else if (name === EXPIRY_COOKIE_NAME) {
      expiryCount++;
      rawExpiry = val;
    }
  }

  if (tokenCount !== 1 || expiryCount !== 1) {
    return null;
  }
  if (!rawToken || !safeOpaqueToken.test(rawToken)) {
    return null;
  }
  if (!rawExpiry || !rfc3339Timestamp.test(rawExpiry)) {
    return null;
  }

  const parsedMs = Date.parse(rawExpiry);
  if (Number.isNaN(parsedMs) || parsedMs <= Date.now()) {
    return null;
  }

  return {
    token: rawToken,
    expiresAt: rawExpiry,
  };
}

export interface RevocationSessionResult {
  status: "valid" | "empty" | "malformed";
  token: string | null;
}

export function parseRevocationSessionToken(
  cookieHeader: string | null,
): RevocationSessionResult {
  if (!cookieHeader) {
    return { status: "empty", token: null };
  }

  const cookieEntries = cookieHeader.split(";");
  let rawToken: string | null = null;
  let tokenCount = 0;

  for (const entry of cookieEntries) {
    const parts = entry.split("=");
    if (parts.length < 2) continue;
    const name = parts[0].trim();
    const val = parts.slice(1).join("=").trim();

    if (name === SESSION_COOKIE_NAME) {
      tokenCount++;
      rawToken = val;
    }
  }

  if (tokenCount === 0) {
    return { status: "empty", token: null };
  }
  if (tokenCount > 1) {
    return { status: "malformed", token: null };
  }
  if (!rawToken || !safeOpaqueToken.test(rawToken)) {
    return { status: "malformed", token: null };
  }

  return {
    status: "valid",
    token: rawToken,
  };
}

export function buildSessionCookies(
  token: string,
  expiresAtIso: string,
): [string, string] {
  const expiresMs = Date.parse(expiresAtIso);
  const nowMs = Date.now();
  const maxAgeSeconds = Math.max(0, Math.floor((expiresMs - nowMs) / 1000));
  const expiresUtc = new Date(expiresMs).toUTCString();

  const cookie1 = `${SESSION_COOKIE_NAME}=${token}; Path=/; HttpOnly; Secure; SameSite=Lax; Expires=${expiresUtc}; Max-Age=${maxAgeSeconds}`;
  const cookie2 = `${EXPIRY_COOKIE_NAME}=${expiresAtIso}; Path=/; HttpOnly; Secure; SameSite=Lax; Expires=${expiresUtc}; Max-Age=${maxAgeSeconds}`;

  return [cookie1, cookie2];
}

export function buildClearSessionCookies(): [string, string] {
  const epoch = "Thu, 01 Jan 1970 00:00:00 GMT";
  const cookie1 = `${SESSION_COOKIE_NAME}=; Path=/; HttpOnly; Secure; SameSite=Lax; Expires=${epoch}; Max-Age=0`;
  const cookie2 = `${EXPIRY_COOKIE_NAME}=; Path=/; HttpOnly; Secure; SameSite=Lax; Expires=${epoch}; Max-Age=0`;

  return [cookie1, cookie2];
}

export function errorResponse(
  status: number,
  code: string,
  requestId: string,
  headers?: Record<string, string>,
): Response {
  const responseHeaders = new Headers({
    "cache-control": "no-store",
    "content-type": "application/json; charset=utf-8",
    "x-request-id": requestId,
    ...(headers ?? {}),
  });
  return new Response(
    JSON.stringify({
      error: {
        code,
        message: "La solicitud no pudo completarse.",
        request_id: requestId,
      },
    }),
    {
      status,
      headers: responseHeaders,
    },
  );
}
