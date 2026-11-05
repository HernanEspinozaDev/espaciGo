import "server-only";

export const MAX_BODY_BYTES = 64 * 1024;

export const safeOpaqueToken = /^[A-Za-z0-9._~+/-]{16,2048}=*$/;
export const rfc3339Timestamp =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;
export const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
export const safeIdentifier = /^[a-z0-9_.-]{1,64}$/i;
export const safeCapability = /^[a-z0-9_.:-]{1,96}$/i;
export const safeErrorCodePattern = /^[a-z0-9_-]{1,64}$/i;

export const ALLOWED_GO_IDENTITY_ERROR_CODES: ReadonlySet<string> = new Set([
  "internal_error",
  "login_blocked",
  "email_registered",
  "unauthenticated",
  "email_unverified",
  "forbidden",
  "credential_notice_not_recoverable",
  "credential_notice_recipient_inactive",
  "invalid_token",
  "current_password_invalid",
  "password_unchanged",
  "password_recently_used",
  "password_confirmation_mismatch",
  "rate_limited",
  "validation_error",
  "not_found",
  "mail_unavailable",
  "invalid_request",
  "unsupported_media_type",
  "invalid_value",
]);

export interface ValidatedGoLogin {
  account_id: string;
  roles: string[];
  capabilities: string[];
  restricted_mode: boolean;
  access_token: string;
  expires_at: string;
}

export interface ValidatedGoSession {
  account_id: string;
  roles: string[];
  capabilities: string[];
  restricted_mode: boolean;
}

export function sanitizeErrorCode(code: unknown, status: number): string {
  if (typeof code === "string" && ALLOWED_GO_IDENTITY_ERROR_CODES.has(code)) {
    return code;
  }
  return `http_${status}`;
}

export function sanitizeRetryAfter(val: string | null): string | null {
  if (!val) return null;
  const trimmed = val.trim();
  if (/^\d{1,6}$/.test(trimmed)) return trimmed;
  if (trimmed.length <= 128 && !/[\r\n]/.test(trimmed)) {
    const ms = Date.parse(trimmed);
    if (!Number.isNaN(ms)) return trimmed;
  }
  return null;
}

export function validateStringArray(
  arr: unknown,
  maxItems = 100,
  pattern?: RegExp,
  forbiddenTokens?: string | string[],
  maxItemLength = 64,
): string[] | null {
  if (!Array.isArray(arr) || arr.length > maxItems) return null;
  const forbiddenList = forbiddenTokens
    ? (Array.isArray(forbiddenTokens)
        ? forbiddenTokens
        : [forbiddenTokens]
      ).filter(Boolean)
    : [];
  const result: string[] = [];
  for (const item of arr) {
    if (typeof item !== "string") return null;
    if (item.trim() !== item || /\s/.test(item)) return null;
    if (item.length === 0 || item.length > maxItemLength) return null;
    if (pattern && !pattern.test(item)) return null;
    for (const f of forbiddenList) {
      if (item === f || item.includes(f)) return null;
    }
    result.push(item);
  }
  return result;
}

export function validateGoLoginResponse(
  data: unknown,
  forbiddenToken?: string,
): ValidatedGoLogin | null {
  if (typeof data !== "object" || data === null || Array.isArray(data))
    return null;
  const rec = data as Record<string, unknown>;
  if (typeof rec.account_id !== "string" || !uuidPattern.test(rec.account_id))
    return null;
  const accountId = rec.account_id;
  if (
    typeof rec.access_token !== "string" ||
    !safeOpaqueToken.test(rec.access_token)
  )
    return null;
  const token = rec.access_token;
  const forbidden = forbiddenToken ? [forbiddenToken, token] : [token];
  if (forbidden.some((f) => accountId === f || accountId.includes(f)))
    return null;
  if (
    typeof rec.expires_at !== "string" ||
    !rfc3339Timestamp.test(rec.expires_at)
  )
    return null;
  const expiresMs = Date.parse(rec.expires_at);
  if (Number.isNaN(expiresMs) || expiresMs <= Date.now()) return null;
  if (typeof rec.restricted_mode !== "boolean") return null;
  const roles = validateStringArray(
    rec.roles,
    64,
    safeIdentifier,
    forbidden,
    64,
  );
  if (roles === null) return null;
  const capabilities = validateStringArray(
    rec.capabilities,
    128,
    safeCapability,
    forbidden,
    96,
  );
  if (capabilities === null) return null;
  return {
    account_id: rec.account_id,
    roles,
    capabilities,
    restricted_mode: rec.restricted_mode,
    access_token: rec.access_token,
    expires_at: rec.expires_at,
  };
}

export function validateGoSessionResponse(
  data: unknown,
  forbiddenToken?: string,
): ValidatedGoSession | null {
  if (typeof data !== "object" || data === null || Array.isArray(data))
    return null;
  const rec = data as Record<string, unknown>;
  if (typeof rec.account_id !== "string" || !uuidPattern.test(rec.account_id))
    return null;
  const accountId = rec.account_id;
  if (
    forbiddenToken &&
    (accountId === forbiddenToken || accountId.includes(forbiddenToken))
  )
    return null;
  if (typeof rec.restricted_mode !== "boolean") return null;
  const roles = validateStringArray(
    rec.roles,
    64,
    safeIdentifier,
    forbiddenToken,
    64,
  );
  if (roles === null) return null;
  const capabilities = validateStringArray(
    rec.capabilities,
    128,
    safeCapability,
    forbiddenToken,
    96,
  );
  if (capabilities === null) return null;
  return {
    account_id: rec.account_id,
    roles,
    capabilities,
    restricted_mode: rec.restricted_mode,
  };
}

export async function revokeUpstreamSession(
  baseUrl: URL,
  token: string,
  requestId: string,
  idHeaders: Record<string, string>,
): Promise<void> {
  try {
    const logoutUrl = new URL("/api/v1/auth/logout", baseUrl);
    const headers = new Headers({
      accept: "application/json",
      authorization: `Bearer ${token}`,
      "x-request-id": requestId,
    });
    if (idHeaders["X-Serverless-Authorization"]) {
      headers.set(
        "X-Serverless-Authorization",
        idHeaders["X-Serverless-Authorization"],
      );
    }
    await fetch(logoutUrl, {
      method: "POST",
      headers,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(5_000),
    });
  } catch {
    // Best-effort rollback: never throw
  }
}

export type ReadBodyResult =
  | { ok: true; data: { email: string; password: string }; rawText: string }
  | { ok: false; status: number; code: string };

export async function readAndValidateLoginBody(
  request: Request,
): Promise<ReadBodyResult> {
  const declaredLength = request.headers.get("content-length");
  if (declaredLength !== null) {
    if (!/^\d+$/.test(declaredLength)) {
      return { ok: false, status: 400, code: "invalid_body" };
    }
    if (Number(declaredLength) > MAX_BODY_BYTES) {
      return { ok: false, status: 413, code: "too_large" };
    }
  }
  if (!request.body) {
    return { ok: false, status: 400, code: "invalid_body" };
  }
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      total += value.byteLength;
      if (total > MAX_BODY_BYTES) {
        await reader.cancel();
        return { ok: false, status: 413, code: "too_large" };
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }
  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(body);
  } catch {
    return { ok: false, status: 400, code: "invalid_body" };
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { ok: false, status: 400, code: "invalid_json" };
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { ok: false, status: 400, code: "invalid_body" };
  }
  const rec = parsed as Record<string, unknown>;
  const keys = Object.keys(rec);
  if (
    keys.length !== 2 ||
    typeof rec.email !== "string" ||
    typeof rec.password !== "string" ||
    rec.email.length === 0 ||
    rec.email.length > 254 ||
    rec.password.length === 0 ||
    rec.password.length > 1024
  ) {
    return { ok: false, status: 400, code: "invalid_payload" };
  }
  return {
    ok: true,
    data: { email: rec.email, password: rec.password },
    rawText: text,
  };
}

export async function readBoundedUpstreamJson(
  res: Response,
  maxBytes = MAX_BODY_BYTES,
): Promise<
  { ok: true; data: unknown } | { ok: false; status: 502; code: string }
> {
  try {
    const contentType = res.headers.get("content-type");
    const mediaType = contentType?.split(";", 1)[0].trim().toLowerCase();
    if (mediaType !== "application/json") {
      await res.body?.cancel().catch(() => undefined);
      return { ok: false, status: 502, code: "invalid_upstream_response" };
    }
    if (!res.body) {
      return { ok: false, status: 502, code: "invalid_upstream_response" };
    }
    const reader = res.body.getReader();
    const chunks: Uint8Array[] = [];
    let total = 0;
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        total += value.byteLength;
        if (total > maxBytes) {
          await reader.cancel().catch(() => undefined);
          return { ok: false, status: 502, code: "invalid_upstream_response" };
        }
        chunks.push(value);
      }
    } finally {
      reader.releaseLock();
    }
    const body = new Uint8Array(total);
    let offset = 0;
    for (const chunk of chunks) {
      body.set(chunk, offset);
      offset += chunk.byteLength;
    }
    const text = new TextDecoder("utf-8", { fatal: true }).decode(body);
    const data = JSON.parse(text);
    return { ok: true, data };
  } catch {
    await res.body?.cancel().catch(() => undefined);
    return { ok: false, status: 502, code: "invalid_upstream_response" };
  }
}
