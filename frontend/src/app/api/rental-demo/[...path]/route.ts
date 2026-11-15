import { getRentalDemoBaseUrl } from "@/features/rental-demo/infrastructure/demo-server-config";

const PREFIX = "/api/v1/local/rental-demo";
const COOKIE = "espacigo_rental_demo";
const MAX_REQUEST = 16 * 1024;
const MAX_RESPONSE = 2 * 1024 * 1024;
const UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const error = (status: number, code: string) =>
  new Response(JSON.stringify({ error: { code } }), {
    status,
    headers: {
      "content-type": "application/json",
      "cache-control": "no-store",
      "x-content-type-options": "nosniff",
    },
  });
const successHeaders = {
  "cache-control": "no-store",
  "x-content-type-options": "nosniff",
};
function route(p: string[], method: string): string | null {
  if (
    p.length === 1 &&
    p[0] === "session" &&
    ["GET", "POST", "DELETE"].includes(method)
  )
    return "session";
  if (p.length === 1 && p[0] === "spaces" && method === "GET") return "spaces";
  if (
    p.length === 2 &&
    p[0] === "owner" &&
    p[1] === "spaces" &&
    method === "GET"
  )
    return "owner/spaces";
  if (
    p.length === 3 &&
    p[0] === "spaces" &&
    UUID.test(p[1]) &&
    p[2] === "month" &&
    method === "GET"
  )
    return `spaces/${p[1]}/month`;
  if (
    p.length === 4 &&
    p[0] === "owner" &&
    p[1] === "spaces" &&
    UUID.test(p[2]) &&
    p[3] === "month" &&
    method === "GET"
  )
    return `owner/spaces/${p[2]}/month`;
  if (
    p.length === 3 &&
    p[0] === "owner" &&
    p[1] === "spaces" &&
    UUID.test(p[2]) &&
    method === "PUT"
  )
    return `owner/spaces/${p[2]}`;
  if (
    p.length === 4 &&
    p[0] === "owner" &&
    p[1] === "spaces" &&
    UUID.test(p[2]) &&
    p[3] === "blocks" &&
    method === "POST"
  )
    return `owner/spaces/${p[2]}/blocks`;
  if (
    p.length === 5 &&
    p[0] === "owner" &&
    p[1] === "spaces" &&
    UUID.test(p[2]) &&
    p[3] === "blocks" &&
    UUID.test(p[4]) &&
    method === "DELETE"
  )
    return `owner/spaces/${p[2]}/blocks/${p[4]}`;
  if (p.length === 1 && p[0] === "quotes" && method === "POST") return "quotes";
  if (
    p.length === 3 &&
    p[0] === "quotes" &&
    UUID.test(p[1]) &&
    p[2] === "confirm" &&
    method === "POST"
  )
    return `quotes/${p[1]}/confirm`;
  if (
    p.length === 2 &&
    p[0] === "attempts" &&
    UUID.test(p[1]) &&
    method === "GET"
  )
    return `attempts/${p[1]}`;
  return null;
}
async function readBounded(
  stream: ReadableStream<Uint8Array> | null,
  max: number,
): Promise<Uint8Array | null> {
  if (!stream) return new Uint8Array();
  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.byteLength;
    if (size > max) {
      await reader.cancel();
      return null;
    }
    chunks.push(value);
  }
  const out = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    out.set(chunk, offset);
    offset += chunk.length;
  }
  return out;
}
function demoCookie(raw: string): string | null | false {
  const matches = raw
    .split(";")
    .map((x) => x.trim())
    .filter((x) => x.startsWith(`${COOKIE}=`));
  if (matches.length > 1) return false;
  if (!matches.length) return null;
  const value = matches[0].slice(COOKIE.length + 1);
  return /^[A-Za-z0-9._~-]{1,2048}$/.test(value) ? value : false;
}
function canonicalSetCookie(raw: string | null): string | null {
  if (!raw) return null;
  const parts = raw.split(";").map((x) => x.trim());
  const eq = parts[0].indexOf("=");
  if (eq < 1 || parts[0].slice(0, eq) !== COOKIE) return null;
  const value = parts[0].slice(eq + 1);
  if (value && !/^[A-Za-z0-9._~-]{1,2048}$/.test(value)) return null;
  const attrs = new Map<string, string | true>();
  for (const part of parts.slice(1)) {
    const i = part.indexOf("=");
    const key = (i < 0 ? part : part.slice(0, i)).toLowerCase();
    if (attrs.has(key)) return null;
    attrs.set(key, i < 0 ? true : part.slice(i + 1));
  }
  if (
    attrs.size !== 4 ||
    attrs.get("path") !== "/api/rental-demo" ||
    attrs.get("httponly") !== true ||
    attrs.get("samesite") !== "Strict"
  )
    return null;
  const maxAge = attrs.get("max-age");
  if (
    typeof maxAge !== "string" ||
    !/^(0|[1-9][0-9]{0,4})$/.test(maxAge) ||
    Number(maxAge) > 28800 ||
    (Number(maxAge) === 0 && value !== "") ||
    (Number(maxAge) > 0 && !value)
  )
    return null;
  return `${COOKIE}=${value}; Path=/api/rental-demo; Max-Age=${maxAge}; HttpOnly; SameSite=Strict`;
}
async function handle(
  req: Request,
  ctx: { params: Promise<{ path: string[] }> },
) {
  const base = getRentalDemoBaseUrl();
  if (!base) return error(404, "not_found");
  const host = req.headers.get("host") ?? "";
  const hostMatch = /^127\.0\.0\.1:(\d{1,5})$/.exec(host);
  if (
    !hostMatch ||
    +hostMatch[1] < 1 ||
    +hostMatch[1] > 65535 ||
    req.headers.has("authorization")
  )
    return error(403, "forbidden");
  const { path } = await ctx.params;
  if (path.some((x) => !x || x === "." || x === ".." || x.includes("/")))
    return error(404, "not_found");
  const url = new URL(req.url);
  const upstream = route(path, req.method);
  if (!upstream) return error(404, "not_found");
  const monthRoute = path.at(-1) === "month";
  const keys = [...url.searchParams.keys()];
  if (
    (!monthRoute && keys.length) ||
    (monthRoute &&
      (keys.some((k) => !["month", "duration"].includes(k)) ||
        ["month", "duration"].some(
          (k) => url.searchParams.getAll(k).length !== 1,
        ) ||
        !/^\d{4}-(0[1-9]|1[0-2])$/.test(url.searchParams.get("month") ?? "") ||
        !/^(1|2|3|4)$/.test(url.searchParams.get("duration") ?? "")))
  )
    return error(400, "invalid_request");
  const mutating = ["POST", "PUT", "DELETE"].includes(req.method);
  if (mutating && req.headers.get("origin") !== `${url.protocol}//${host}`)
    return error(403, "forbidden");
  const body = mutating
    ? await readBounded(req.body, MAX_REQUEST)
    : new Uint8Array();
  if (body === null) return error(413, "payload_too_large");
  const headers = new Headers();
  const cookie = demoCookie(req.headers.get("cookie") ?? "");
  if (cookie === false) return error(400, "invalid_request");
  if (cookie) headers.set("cookie", `${COOKIE}=${cookie}`);
  if (mutating && body.length)
    headers.set(
      "content-type",
      req.headers.get("content-type") ?? "application/json",
    );
  const ifMatch = req.headers.get("if-match");
  if (ifMatch && path.includes("blocks")) headers.set("if-match", ifMatch);
  let response: Response;
  try {
    response = await fetch(
      new URL(`${PREFIX}/${upstream}${url.search}`, base),
      {
        method: req.method,
        headers,
        body:
          mutating && body.length
            ? new Blob([new Uint8Array(body).buffer])
            : undefined,
        redirect: "manual",
        cache: "no-store",
        credentials: "omit",
        signal: req.signal,
      },
    );
  } catch {
    return error(502, "upstream_error");
  }
  if (response.status >= 300 && response.status < 400) {
    await response.body?.cancel();
    return error(502, "upstream_error");
  }
  if (response.status === 204) {
    const out = new Headers(successHeaders);
    const expired = canonicalSetCookie(response.headers.get("set-cookie"));
    if (expired?.includes("Max-Age=0")) out.set("set-cookie", expired);
    return new Response(null, { status: 204, headers: out });
  }
  let bytes: Uint8Array | null;
  try {
    bytes = await readBounded(response.body, MAX_RESPONSE);
  } catch {
    return error(502, "upstream_error");
  }
  if (!bytes) return error(502, "upstream_error");
  if (response.status >= 400) return error(response.status, "upstream_error");
  let parsed: unknown;
  try {
    parsed = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    return error(502, "upstream_error");
  }
  const outHeaders = new Headers({
    ...successHeaders,
    "content-type": "application/json",
  });
  const setCookie = canonicalSetCookie(response.headers.get("set-cookie"));
  if (setCookie) outHeaders.set("set-cookie", setCookie);
  return new Response(JSON.stringify(parsed), {
    status: response.status,
    headers: outHeaders,
  });
}
export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const DELETE = handle;
