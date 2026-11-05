import "server-only";

import { getApiBackendBaseUrl } from "@/lib/api/server-config";
import { getCloudRunIdentityHeadersForBackend } from "@/lib/api/cloud-run-identity";
import {
  BrowserSession,
  buildSessionCookies,
  errorResponse,
  validateCsrfOrigin,
} from "@/lib/api/server-session";
import {
  readAndValidateLoginBody,
  readBoundedUpstreamJson,
  revokeUpstreamSession,
  sanitizeErrorCode,
  sanitizeRetryAfter,
  validateGoLoginResponse,
  validateGoSessionResponse,
} from "@/lib/api/bff-login-validation";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

function newRequestId(): string {
  return crypto.randomUUID();
}

const safeRequestId = /^[A-Za-z0-9._-]{1,128}$/;

export async function POST(request: Request): Promise<Response> {
  const requestId = request.headers.get("x-request-id") ?? newRequestId();
  if (!safeRequestId.test(requestId)) {
    return errorResponse(400, "invalid_request_id", newRequestId());
  }

  if (request.headers.has("authorization")) {
    return errorResponse(400, "authorization_not_allowed", requestId);
  }

  if (!validateCsrfOrigin(request, { isMutation: true })) {
    return errorResponse(403, "origin_not_allowed", requestId);
  }

  const contentType = request.headers.get("content-type");
  const mediaType = contentType?.split(";", 1)[0].trim().toLowerCase();
  if (mediaType !== "application/json") {
    return errorResponse(415, "content_type_not_allowed", requestId);
  }

  const bodyResult = await readAndValidateLoginBody(request);
  if (!bodyResult.ok) {
    return errorResponse(bodyResult.status, bodyResult.code, requestId);
  }

  let baseUrl: URL;
  try {
    baseUrl = getApiBackendBaseUrl();
  } catch {
    return errorResponse(503, "api_not_configured", requestId);
  }

  const idHeaders = await getCloudRunIdentityHeadersForBackend(baseUrl);

  const loginUpstreamUrl = new URL("/api/v1/auth/login", baseUrl);
  const loginHeaders = new Headers({
    "content-type": "application/json",
    accept: "application/json",
    "x-request-id": requestId,
  });
  if (idHeaders["X-Serverless-Authorization"]) {
    loginHeaders.set(
      "X-Serverless-Authorization",
      idHeaders["X-Serverless-Authorization"],
    );
  }
  if (idHeaders["Authorization"]) {
    loginHeaders.set("authorization", idHeaders["Authorization"]);
  }

  const upstreamPayload = JSON.stringify({
    email: bodyResult.data.email,
    password: bodyResult.data.password,
  });

  let loginRes: Response;
  try {
    loginRes = await fetch(loginUpstreamUrl, {
      method: "POST",
      headers: loginHeaders,
      body: upstreamPayload,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]),
    });
  } catch {
    return errorResponse(502, "api_unavailable", requestId);
  }

  if (loginRes.status >= 300 && loginRes.status < 400) {
    await loginRes.body?.cancel().catch(() => undefined);
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  if (!loginRes.ok) {
    const status = loginRes.status;
    let errorPayload: unknown = null;
    const ct = loginRes.headers
      .get("content-type")
      ?.split(";", 1)[0]
      .trim()
      .toLowerCase();
    if (ct === "application/json") {
      const boundedRes = await readBoundedUpstreamJson(loginRes);
      if (boundedRes.ok) errorPayload = boundedRes.data;
    } else {
      await loginRes.body?.cancel().catch(() => undefined);
    }
    const rawCode = (errorPayload as { error?: { code?: unknown } } | null)
      ?.error?.code;
    const code = sanitizeErrorCode(rawCode, status);
    const retryAfter = sanitizeRetryAfter(loginRes.headers.get("retry-after"));
    const extraHeaders: Record<string, string> = {};
    if (retryAfter) extraHeaders["retry-after"] = retryAfter;
    return errorResponse(status, code, requestId, extraHeaders);
  }

  if (loginRes.status !== 200) {
    await loginRes.body?.cancel().catch(() => undefined);
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  const loginJson = await readBoundedUpstreamJson(loginRes);
  if (!loginJson.ok) {
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  const loginData = validateGoLoginResponse(loginJson.data);
  if (!loginData) {
    return errorResponse(502, "invalid_upstream_response", requestId);
  }

  const token = loginData.access_token;

  if (request.signal.aborted) {
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(499, "request_aborted", requestId);
  }

  const sessionUpstreamUrl = new URL("/api/v1/auth/session", baseUrl);
  const sessionHeaders = new Headers({
    accept: "application/json",
    authorization: `Bearer ${token}`,
    "x-request-id": requestId,
  });
  if (idHeaders["X-Serverless-Authorization"]) {
    sessionHeaders.set(
      "X-Serverless-Authorization",
      idHeaders["X-Serverless-Authorization"],
    );
  }

  let sessionRes: Response;
  try {
    sessionRes = await fetch(sessionUpstreamUrl, {
      method: "GET",
      headers: sessionHeaders,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]),
    });
  } catch {
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(502, "api_unavailable", requestId);
  }

  if (sessionRes.status !== 200) {
    await sessionRes.body?.cancel().catch(() => undefined);
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(502, "session_confirmation_failed", requestId);
  }

  let sessionJson: Awaited<ReturnType<typeof readBoundedUpstreamJson>>;
  try {
    sessionJson = await readBoundedUpstreamJson(sessionRes);
  } catch {
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(502, "session_confirmation_failed", requestId);
  }
  if (!sessionJson.ok) {
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(502, "session_confirmation_failed", requestId);
  }

  const confirmed = validateGoSessionResponse(sessionJson.data, token);
  if (!confirmed || confirmed.account_id !== loginData.account_id) {
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(502, "session_confirmation_failed", requestId);
  }

  if (request.signal.aborted) {
    await revokeUpstreamSession(baseUrl, token, requestId, idHeaders);
    return errorResponse(499, "request_aborted", requestId);
  }

  const [c1, c2] = buildSessionCookies(token, loginData.expires_at);
  const outSession: BrowserSession = {
    account_id: confirmed.account_id,
    roles: confirmed.roles,
    capabilities: confirmed.capabilities,
    restricted_mode: confirmed.restricted_mode,
    expires_at: loginData.expires_at,
  };

  const responseHeaders = new Headers({
    "cache-control": "no-store",
    "content-type": "application/json; charset=utf-8",
    "x-request-id": requestId,
  });
  responseHeaders.append("set-cookie", c1);
  responseHeaders.append("set-cookie", c2);

  return new Response(JSON.stringify(outSession), {
    status: 200,
    headers: responseHeaders,
  });
}
