import { gatewayOperations } from "@/lib/api/generated/gateway-operations";
import { getApiBackendBaseUrl } from "@/lib/api/server-config";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

const maximumBodyBytes = 50 * 1024 * 1024;
const acceptedApplicationHeaders = new Set([
  "accept",
  "authorization",
  "content-type",
  "x-request-id",
]);
const ignoredTransportHeaders = new Set([
  "accept-encoding",
  "accept-language",
  "cache-control",
  "connection",
  "content-length",
  "host",
  "keep-alive",
  "origin",
  "pragma",
  "priority",
  "referer",
  "transfer-encoding",
  "user-agent",
  "x-forwarded-for",
  "x-forwarded-host",
  "x-forwarded-port",
  "x-forwarded-proto",
]);
const responseHeaders = [
  "accept-ranges",
  "cache-control",
  "content-disposition",
  "content-range",
  "content-type",
  "etag",
  "last-modified",
  "retry-after",
  "vary",
  "www-authenticate",
];
const safePathSegment = /^[A-Za-z0-9][A-Za-z0-9._~-]{0,254}$/;
const safeHeaderValue = /^[\t\x20-\x7e\x80-\xff]*$/;
const safeRequestId = /^[A-Za-z0-9._-]{1,128}$/;
const sensitiveQueryKey =
  /(?:token|password|secret|authorization|credential|bearer)/i;

interface RouteContext {
  params: Promise<{ path: string[] }>;
}

type GatewayOperation = (typeof gatewayOperations)[number];

function newRequestId(): string {
  return crypto.randomUUID();
}

function errorResponse(
  status: number,
  code: string,
  requestId: string,
): Response {
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
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        "x-request-id": requestId,
      },
    },
  );
}

function matchesPath(template: string, segments: string[]): boolean {
  const parts = template.split("/").filter(Boolean);
  if (parts.length !== segments.length) return false;

  return parts.every((part, index) => {
    const segment = segments[index];
    if (!safePathSegment.test(segment) || segment === "." || segment === "..") {
      return false;
    }
    return part.startsWith("{") && part.endsWith("}") ? true : part === segment;
  });
}

function operationFor(
  method: string,
  segments: string[],
): { operation?: GatewayOperation; allowedMethods: string[] } {
  const pathMatches = gatewayOperations.filter((entry) =>
    matchesPath(entry.path, segments),
  );
  const operation = pathMatches.find((entry) => entry.method === method);
  return {
    operation,
    allowedMethods: [
      ...new Set(pathMatches.map((entry) => entry.method)),
    ].sort(),
  };
}

function validHeaderValue(value: string, maximumLength: number): boolean {
  return value.length <= maximumLength && safeHeaderValue.test(value);
}

function isSameOriginRequest(request: Request, origin: string): boolean {
  const host = request.headers.get("host");
  if (
    !host ||
    host !== host.trim() ||
    host.length > 255 ||
    /[\s,/@\\?#]/.test(host) ||
    origin.length > 512
  ) {
    return false;
  }

  try {
    const parsedOrigin = new URL(origin);
    if (
      parsedOrigin.protocol !== "http:" &&
      parsedOrigin.protocol !== "https:"
    ) {
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

function validateQuery(requestUrl: URL, operation: GatewayOperation): boolean {
  const allowed = new Map<string, boolean>(
    operation.query.map((parameter) => [
      parameter.name,
      parameter.allowRepeated,
    ]),
  );
  const counts = new Map<string, number>();
  for (const key of requestUrl.searchParams.keys()) {
    const allowRepeated = allowed.get(key);
    if (sensitiveQueryKey.test(key) || allowRepeated === undefined)
      return false;
    const count = (counts.get(key) ?? 0) + 1;
    if (count > 1 && !allowRepeated) return false;
    counts.set(key, count);
  }
  return true;
}

function collectHeaders(
  request: Request,
  requestId: string,
  operation: GatewayOperation,
): { headers?: Headers; error?: Response } {
  const headers = new Headers();
  const operationHeaders = new Set<string>(operation.headers);

  if (request.headers.has("cookie")) {
    return { error: errorResponse(400, "cookies_not_supported", requestId) };
  }
  const origin = request.headers.get("origin");
  if (origin && !isSameOriginRequest(request, origin)) {
    return { error: errorResponse(403, "origin_not_allowed", requestId) };
  }

  for (const [rawName, value] of request.headers) {
    const name = rawName.toLowerCase();
    if (name === "cookie") continue;
    if (ignoredTransportHeaders.has(name) || name.startsWith("sec-")) continue;

    const isOperationHeader = operationHeaders.has(name);
    const isStandardHeader = acceptedApplicationHeaders.has(name);
    if (!isOperationHeader && !isStandardHeader) {
      return { error: errorResponse(400, "header_not_allowed", requestId) };
    }
    if (!validHeaderValue(value, name === "authorization" ? 4096 : 2048)) {
      return { error: errorResponse(400, "invalid_header", requestId) };
    }

    if (name === "authorization") {
      if (!operation.bearer || !/^Bearer [A-Za-z0-9._~+/-]+=*$/.test(value)) {
        return {
          error: errorResponse(400, "authorization_not_allowed", requestId),
        };
      }
    }
    if (name === "x-request-id" && !safeRequestId.test(value)) {
      return { error: errorResponse(400, "invalid_request_id", requestId) };
    }
    if (name === "content-type") {
      const mediaType = value.split(";", 1)[0].trim().toLowerCase();
      if (
        !(operation.requestMediaTypes as readonly string[]).includes(mediaType)
      ) {
        return {
          error: errorResponse(415, "content_type_not_allowed", requestId),
        };
      }
    }
    if (
      name === "accept" ||
      name === "authorization" ||
      name === "content-type" ||
      isOperationHeader
    ) {
      headers.set(name, value);
    }
  }

  if (request.headers.has("authorization") && !operation.bearer) {
    return {
      error: errorResponse(400, "authorization_not_allowed", requestId),
    };
  }

  headers.set("x-request-id", request.headers.get("x-request-id") ?? requestId);
  return { headers };
}

async function readBody(
  request: Request,
): Promise<Uint8Array<ArrayBuffer> | null> {
  if (request.body === null) return null;
  const declaredLength = request.headers.get("content-length");
  if (declaredLength !== null) {
    if (!/^\d+$/.test(declaredLength)) throw new RangeError("invalid_length");
    if (Number(declaredLength) > maximumBodyBytes)
      throw new RangeError("too_large");
  }

  const reader = request.body.getReader();
  const chunks: Uint8Array<ArrayBufferLike>[] = [];
  let total = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      total += value.byteLength;
      if (total > maximumBodyBytes) {
        await reader.cancel();
        throw new RangeError("too_large");
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const body = new Uint8Array(new ArrayBuffer(total));
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

async function proxy(
  request: Request,
  context: RouteContext,
): Promise<Response> {
  const requestId = request.headers.get("x-request-id") ?? newRequestId();
  if (!safeRequestId.test(requestId)) {
    return errorResponse(400, "invalid_request_id", newRequestId());
  }
  const rawPath = request.url.split(/[?#]/, 1)[0];
  if (
    rawPath.includes("%") ||
    rawPath.includes("\\") ||
    /(?:^|\/)\.{1,2}(?:\/|$)/.test(rawPath)
  ) {
    return errorResponse(400, "path_not_allowed", requestId);
  }

  const { path } = await context.params;
  if (!Array.isArray(path) || path.length === 0) {
    return errorResponse(404, "route_not_allowed", requestId);
  }
  const { operation, allowedMethods } = operationFor(
    request.method.toUpperCase(),
    path,
  );
  if (!operation) {
    if (allowedMethods.length > 0) {
      const response = errorResponse(405, "method_not_allowed", requestId);
      response.headers.set("allow", allowedMethods.join(", "));
      return response;
    }
    return errorResponse(404, "route_not_allowed", requestId);
  }

  const requestUrl = new URL(request.url);
  if (!validateQuery(requestUrl, operation)) {
    return errorResponse(400, "query_parameter_not_allowed", requestId);
  }
  const collected = collectHeaders(request, requestId, operation);
  if (collected.error) return collected.error;

  let body: Uint8Array<ArrayBuffer> | null;
  try {
    body = await readBody(request);
  } catch (error) {
    const code = error instanceof RangeError ? error.message : "invalid_body";
    return errorResponse(code === "too_large" ? 413 : 400, code, requestId);
  }
  if (body?.byteLength === 0) body = null;
  if (body !== null && operation.requestMediaTypes.length === 0) {
    return errorResponse(415, "request_body_not_allowed", requestId);
  }
  if (body !== null && !request.headers.has("content-type")) {
    return errorResponse(415, "content_type_required", requestId);
  }
  if (body === null && request.headers.has("content-type")) {
    return errorResponse(400, "unexpected_content_type", requestId);
  }

  let baseUrl: URL;
  try {
    baseUrl = getApiBackendBaseUrl();
  } catch {
    return errorResponse(503, "api_not_configured", requestId);
  }
  const encodedPath = path
    .map((segment) => encodeURIComponent(segment))
    .join("/");
  const upstreamUrl = new URL(`${baseUrl.pathname}/${encodedPath}`, baseUrl);
  upstreamUrl.search = requestUrl.search;

  let upstream: Response;
  try {
    upstream = await fetch(upstreamUrl, {
      method: operation.method,
      headers: collected.headers,
      body: body ?? undefined,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(30_000)]),
    });
  } catch {
    return errorResponse(502, "api_unavailable", requestId);
  }

  const upstreamContentType = upstream.headers.get("content-type");
  const upstreamMediaType = upstreamContentType
    ?.split(";", 1)[0]
    .trim()
    .toLowerCase();
  const allowedResponseMediaTypes = new Set<string>(
    operation.responseMediaTypes,
  );
  if (
    (upstream.body !== null && !upstreamMediaType) ||
    (upstreamMediaType && !allowedResponseMediaTypes.has(upstreamMediaType))
  ) {
    await upstream.body?.cancel().catch(() => undefined);
    return errorResponse(502, "upstream_content_type_not_allowed", requestId);
  }

  const outgoingHeaders = new Headers();
  for (const name of responseHeaders) {
    const value = upstream.headers.get(name);
    if (value !== null) outgoingHeaders.set(name, value);
  }
  const upstreamRequestId = upstream.headers.get("x-request-id");
  outgoingHeaders.set(
    "x-request-id",
    upstreamRequestId && safeRequestId.test(upstreamRequestId)
      ? upstreamRequestId
      : requestId,
  );
  outgoingHeaders.set("cache-control", "no-store");

  const responseBody =
    request.method.toUpperCase() === "HEAD" ||
    [204, 205, 304].includes(upstream.status)
      ? null
      : upstream.body;
  return new Response(responseBody, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: outgoingHeaders,
  });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const HEAD = proxy;
