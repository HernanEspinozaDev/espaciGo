import "server-only";

import { getApiBackendBaseUrl } from "@/lib/api/server-config";
import { getCloudRunIdentityHeadersForBackend } from "@/lib/api/cloud-run-identity";
import {
  buildClearSessionCookies,
  errorResponse,
  parseRevocationSessionToken,
  validateCsrfOrigin,
} from "@/lib/api/server-session";

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

  const [clear1, clear2] = buildClearSessionCookies();
  const cookieHeader = request.headers.get("cookie");
  const revocation = parseRevocationSessionToken(cookieHeader);

  if (revocation.status === "empty") {
    const responseHeaders = new Headers({
      "cache-control": "private, no-store",
      "x-request-id": requestId,
      "x-espacigo-session-cleared": "1",
    });
    responseHeaders.append("set-cookie", clear1);
    responseHeaders.append("set-cookie", clear2);
    return new Response(null, {
      status: 204,
      headers: responseHeaders,
    });
  }

  if (revocation.status === "malformed") {
    const res = errorResponse(502, "remote_logout_unconfirmed", requestId, {
      "cache-control": "private, no-store",
      "x-espacigo-session-cleared": "1",
    });
    res.headers.append("set-cookie", clear1);
    res.headers.append("set-cookie", clear2);
    return res;
  }

  let baseUrl: URL;
  try {
    baseUrl = getApiBackendBaseUrl();
  } catch {
    const res = errorResponse(502, "remote_logout_unconfirmed", requestId, {
      "x-espacigo-session-cleared": "1",
    });
    res.headers.append("set-cookie", clear1);
    res.headers.append("set-cookie", clear2);
    return res;
  }

  const idHeaders = await getCloudRunIdentityHeadersForBackend(baseUrl);

  const logoutUpstreamUrl = new URL("/api/v1/auth/logout", baseUrl);
  const logoutHeaders = new Headers({
    accept: "application/json",
    authorization: "Bearer " + revocation.token,
    "x-request-id": requestId,
  });
  if (idHeaders["X-Serverless-Authorization"]) {
    logoutHeaders.set(
      "X-Serverless-Authorization",
      idHeaders["X-Serverless-Authorization"],
    );
  }

  let upstreamRes: Response;
  try {
    upstreamRes = await fetch(logoutUpstreamUrl, {
      method: "POST",
      headers: logoutHeaders,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]),
    });
  } catch {
    const res = errorResponse(502, "remote_logout_unconfirmed", requestId, {
      "cache-control": "private, no-store",
      "x-espacigo-session-cleared": "1",
    });
    res.headers.append("set-cookie", clear1);
    res.headers.append("set-cookie", clear2);
    return res;
  }

  if (upstreamRes.status === 204 || upstreamRes.status === 401) {
    const responseHeaders = new Headers({
      "cache-control": "private, no-store",
      "x-request-id": requestId,
      "x-espacigo-session-cleared": "1",
    });
    responseHeaders.append("set-cookie", clear1);
    responseHeaders.append("set-cookie", clear2);
    return new Response(null, {
      status: 204,
      headers: responseHeaders,
    });
  }

  const res = errorResponse(502, "remote_logout_unconfirmed", requestId, {
    "cache-control": "private, no-store",
    "x-espacigo-session-cleared": "1",
  });
  res.headers.append("set-cookie", clear1);
  res.headers.append("set-cookie", clear2);
  return res;
}
