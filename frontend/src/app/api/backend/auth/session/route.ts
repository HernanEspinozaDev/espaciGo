import "server-only";

import { getApiBackendBaseUrl } from "@/lib/api/server-config";
import { getCloudRunIdentityHeadersForBackend } from "@/lib/api/cloud-run-identity";
import {
  BrowserSession,
  buildClearSessionCookies,
  errorResponse,
  parseSessionCookies,
  validateCsrfOrigin,
} from "@/lib/api/server-session";
import {
  sanitizeErrorCode,
  validateGoSessionResponse,
} from "@/lib/api/bff-login-validation";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

function newRequestId(): string {
  return crypto.randomUUID();
}

const safeRequestId = /^[A-Za-z0-9._-]{1,128}$/;
const MAX_RESPONSE_BYTES = 64 * 1024;

async function readBoundedBody(
  res: Response,
  maxBytes: number,
): Promise<string> {
  if (!res.body) return "";
  const reader = res.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      if (value) {
        total += value.byteLength;
        if (total > maxBytes) {
          await reader.cancel();
          throw new Error("Payload too large");
        }
        chunks.push(value);
      }
    }
  } finally {
    reader.releaseLock();
  }
  const combined = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    combined.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(combined);
}

export async function GET(request: Request): Promise<Response> {
  const requestId = request.headers.get("x-request-id") ?? newRequestId();
  if (!safeRequestId.test(requestId)) {
    return errorResponse(400, "invalid_request_id", newRequestId());
  }

  if (request.headers.has("authorization")) {
    return errorResponse(400, "authorization_not_allowed", requestId);
  }

  if (!validateCsrfOrigin(request, { isMutation: false })) {
    return errorResponse(403, "origin_not_allowed", requestId);
  }

  const cookieHeader = request.headers.get("cookie");
  const session = parseSessionCookies(cookieHeader);
  if (!session) {
    const [c1, c2] = buildClearSessionCookies();
    const res = errorResponse(401, "unauthenticated", requestId);
    res.headers.append("set-cookie", c1);
    res.headers.append("set-cookie", c2);
    return res;
  }

  let baseUrl: URL;
  try {
    baseUrl = getApiBackendBaseUrl();
  } catch {
    return errorResponse(503, "api_not_configured", requestId);
  }

  const idHeaders = await getCloudRunIdentityHeadersForBackend(baseUrl);

  const sessionUpstreamUrl = new URL("/api/v1/auth/session", baseUrl);
  const sessionHeaders = new Headers({
    accept: "application/json",
    authorization: `Bearer ${session.token}`,
    "x-request-id": requestId,
  });
  if (idHeaders["X-Serverless-Authorization"]) {
    sessionHeaders.set(
      "X-Serverless-Authorization",
      idHeaders["X-Serverless-Authorization"],
    );
  }

  let upstreamRes: Response;
  try {
    upstreamRes = await fetch(sessionUpstreamUrl, {
      method: "GET",
      headers: sessionHeaders,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]),
    });
  } catch {
    return errorResponse(502, "api_unavailable", requestId);
  }

  if (
    (upstreamRes.status >= 300 && upstreamRes.status < 400) ||
    upstreamRes.type === "opaqueredirect"
  ) {
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  if (upstreamRes.status === 401) {
    const [c1, c2] = buildClearSessionCookies();
    const res = errorResponse(401, "unauthenticated", requestId);
    res.headers.append("set-cookie", c1);
    res.headers.append("set-cookie", c2);
    return res;
  }

  if (upstreamRes.status === 403) {
    return errorResponse(403, "forbidden", requestId);
  }

  if (!upstreamRes.ok) {
    const status = upstreamRes.status;
    const retryAfter = upstreamRes.headers.get("retry-after");
    const extraHeaders: Record<string, string> = {};
    if (retryAfter && /^\d{1,6}$/.test(retryAfter.trim())) {
      extraHeaders["retry-after"] = retryAfter.trim();
    }
    let code = `http_${status}`;
    try {
      const text = await readBoundedBody(upstreamRes, 4096);
      const json = JSON.parse(text);
      code = sanitizeErrorCode(json?.error?.code, status);
    } catch {
      // fallback to http_${status}
    }
    return errorResponse(status, code, requestId, extraHeaders);
  }

  const upstreamContentType = upstreamRes.headers.get("content-type");
  const mediaType = upstreamContentType?.split(";", 1)[0].trim().toLowerCase();
  if (mediaType !== "application/json") {
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  let payload: unknown;
  try {
    const raw = await readBoundedBody(upstreamRes, MAX_RESPONSE_BYTES);
    payload = JSON.parse(raw);
  } catch {
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  const validated = validateGoSessionResponse(payload, session.token);
  if (!validated) {
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  const browserSession: BrowserSession = {
    account_id: validated.account_id,
    roles: validated.roles,
    capabilities: validated.capabilities,
    restricted_mode: validated.restricted_mode,
    expires_at: session.expiresAt,
  };

  return new Response(JSON.stringify(browserSession), {
    status: 200,
    headers: {
      "cache-control": "private, no-store",
      "content-type": "application/json; charset=utf-8",
      "x-request-id": requestId,
    },
  });
}
