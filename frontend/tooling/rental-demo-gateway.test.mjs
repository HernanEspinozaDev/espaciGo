import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "rental-demo-"),
);
const configSource = await readFile(
  path.join(
    root,
    "src/features/rental-demo/infrastructure/demo-server-config.ts",
  ),
  "utf8",
);
const routeSource = await readFile(
  path.join(root, "src/app/api/rental-demo/[...path]/route.ts"),
  "utf8",
);
await writeFile(
  path.join(scratch, "demo-server-config.mjs"),
  ts.transpileModule(configSource, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
await writeFile(
  path.join(scratch, "route.mjs"),
  ts.transpileModule(
    routeSource.replace(
      'from "@/features/rental-demo/infrastructure/demo-server-config"',
      'from "./demo-server-config.mjs"',
    ),
    {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    },
  ).outputText,
);
const { GET, POST, PUT, DELETE } = await import(
  pathToFileURL(path.join(scratch, "route.mjs"))
);
const oldFetch = globalThis.fetch;
const oldEnabled = process.env.RENTAL_DEMO_ENABLED;
const oldBase = process.env.RENTAL_DEMO_BASE_URL;
const uuid = "11111111-1111-4111-8111-111111111111";
const request = (method, route, opts = {}) =>
  new Request(`http://127.0.0.1:3000/api/rental-demo/${route}`, {
    method,
    headers: { host: "127.0.0.1:3000", ...(opts.headers ?? {}) },
    body: opts.body,
  });
test.after(async () => {
  globalThis.fetch = oldFetch;
  if (oldEnabled === undefined) delete process.env.RENTAL_DEMO_ENABLED;
  else process.env.RENTAL_DEMO_ENABLED = oldEnabled;
  if (oldBase === undefined) delete process.env.RENTAL_DEMO_BASE_URL;
  else process.env.RENTAL_DEMO_BASE_URL = oldBase;
  await rm(scratch, { recursive: true, force: true });
});
function enabled() {
  process.env.RENTAL_DEMO_ENABLED = "1";
  process.env.RENTAL_DEMO_BASE_URL = "http://127.0.0.1:18084";
}

test("disabled and remote target reject without fetching", async () => {
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return new Response("{}");
  };
  delete process.env.RENTAL_DEMO_ENABLED;
  process.env.RENTAL_DEMO_BASE_URL = "http://127.0.0.1:18084";
  assert.equal(
    (
      await GET(request("GET", "spaces"), {
        params: Promise.resolve({ path: ["spaces"] }),
      })
    ).status,
    404,
  );
  enabled();
  process.env.RENTAL_DEMO_BASE_URL = "http://example.com";
  assert.equal(
    (
      await GET(request("GET", "spaces"), {
        params: Promise.resolve({ path: ["spaces"] }),
      })
    ).status,
    404,
  );
  assert.equal(calls, 0);
});
test("allowlisted route forwards only demo cookie and exact path without following redirects", async () => {
  enabled();
  let target, init;
  globalThis.fetch = async (url, options) => {
    target = String(url);
    init = options;
    return new Response('{"actor":"host"}', {
      headers: { "content-type": "application/json" },
    });
  };
  const res = await POST(
    request("POST", "session", {
      headers: {
        origin: "http://127.0.0.1:3000",
        cookie: "spurious=x; espacigo_rental_demo=abc",
      },
      body: '{"actor":"host"}',
    }),
    { params: Promise.resolve({ path: ["session"] }) },
  );
  assert.equal(res.status, 200);
  assert.equal(new URL(target).pathname, "/api/v1/local/rental-demo/session");
  assert.equal(init.redirect, "manual");
  assert.equal(init.headers.get("cookie"), "espacigo_rental_demo=abc");
  assert.equal(init.headers.has("authorization"), false);
  assert.equal(res.headers.get("cache-control"), "no-store");
});
test("forwards Go Set-Cookie with bounded Max-Age as canonical demo cookie", async () => {
  enabled();
  globalThis.fetch = async () =>
    new Response('{"ok":true}', {
      headers: {
        "set-cookie":
          "espacigo_rental_demo=abc_123; Path=/api/rental-demo; Max-Age=28800; HttpOnly; SameSite=Strict",
      },
    });
  const res = await POST(
    request("POST", "session", {
      headers: { origin: "http://127.0.0.1:3000" },
      body: "{}",
    }),
    { params: Promise.resolve({ path: ["session"] }) },
  );
  assert.equal(
    res.headers.get("set-cookie"),
    "espacigo_rental_demo=abc_123; Path=/api/rental-demo; Max-Age=28800; HttpOnly; SameSite=Strict",
  );
});
test("logout 204 emits expired demo cookie", async () => {
  enabled();
  globalThis.fetch = async () =>
    new Response(null, {
      status: 204,
      headers: {
        "set-cookie":
          "espacigo_rental_demo=; Path=/api/rental-demo; Max-Age=0; HttpOnly; SameSite=Strict",
      },
    });
  const res = await DELETE(
    request("DELETE", "session", {
      headers: {
        origin: "http://127.0.0.1:3000",
        cookie: "espacigo_rental_demo=abc",
      },
    }),
    { params: Promise.resolve({ path: ["session"] }) },
  );
  assert.equal(res.status, 204);
  assert.equal(
    res.headers.get("set-cookie"),
    "espacigo_rental_demo=; Path=/api/rental-demo; Max-Age=0; HttpOnly; SameSite=Strict",
  );
});
test("rejects duplicate demo cookies and malformed upstream JSON without leaking details", async () => {
  enabled();
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return new Response("internal secret");
  };
  const dup = request("GET", "spaces", {
    headers: { cookie: "espacigo_rental_demo=a; espacigo_rental_demo=b" },
  });
  assert.equal(
    (await GET(dup, { params: Promise.resolve({ path: ["spaces"] }) })).status,
    400,
  );
  assert.equal(calls, 0);
  globalThis.fetch = async () => new Response("not-json internal secret");
  const res = await GET(request("GET", "spaces"), {
    params: Promise.resolve({ path: ["spaces"] }),
  });
  assert.equal(res.status, 502);
  assert.doesNotMatch(await res.text(), /internal secret/);
});
test("rejects arbitrary cookie attributes and passes request abort signal upstream", async () => {
  enabled();
  let signal;
  globalThis.fetch = async (_url, init) => {
    signal = init.signal;
    return new Response('{"ok":true}', {
      headers: {
        "set-cookie":
          "espacigo_rental_demo=abc; Path=/api/rental-demo; Max-Age=28800; HttpOnly; SameSite=Strict; Domain=evil.test",
      },
    });
  };
  const controller = new AbortController();
  const req = new Request("http://127.0.0.1:3000/api/rental-demo/spaces", {
    headers: { host: "127.0.0.1:3000" },
    signal: controller.signal,
  });
  const res = await GET(req, { params: Promise.resolve({ path: ["spaces"] }) });
  assert.equal(signal.aborted, false);
  controller.abort();
  assert.equal(signal.aborted, true);
  assert.equal(res.headers.has("set-cookie"), false);
});
test("upstream body stream errors become generic 502", async () => {
  enabled();
  globalThis.fetch = async () =>
    new Response(
      new ReadableStream({
        start(controller) {
          controller.error(new Error("private stream detail"));
        },
      }),
    );
  const res = await GET(request("GET", "spaces"), {
    params: Promise.resolve({ path: ["spaces"] }),
  });
  assert.equal(res.status, 502);
  assert.doesNotMatch(await res.text(), /private stream detail/);
});
test("mutations reject cross-origin and path/query violations", async () => {
  enabled();
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return new Response("{}");
  };
  const badOrigin = request("POST", "session", {
    headers: { origin: "https://evil.test" },
    body: '{"actor":"host"}',
  });
  assert.equal(
    (await POST(badOrigin, { params: Promise.resolve({ path: ["session"] }) }))
      .status,
    403,
  );
  assert.equal(
    (
      await GET(
        request("GET", `spaces/${uuid}/month?month=2026-01&month=2026-02`),
        { params: Promise.resolve({ path: ["spaces", uuid, "month"] }) },
      )
    ).status,
    400,
  );
  assert.equal(calls, 0);
});
test("oversized request is rejected before upstream; redirects are sanitized", async () => {
  enabled();
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return new Response("secret", {
      status: 302,
      headers: { location: "http://evil.test" },
    });
  };
  const body = JSON.stringify({ actor: "host", pad: "x".repeat(17000) });
  assert.equal(
    (
      await POST(
        request("POST", "session", {
          headers: { origin: "http://127.0.0.1:3000" },
          body,
        }),
        { params: Promise.resolve({ path: ["session"] }) },
      )
    ).status,
    413,
  );
  const res = await GET(request("GET", "spaces"), {
    params: Promise.resolve({ path: ["spaces"] }),
  });
  assert.equal(res.status, 502);
  assert.doesNotMatch(await res.text(), /secret/);
  assert.equal(calls, 1);
});
test("enforces cache-control no-store and nosniff on both success and error", async () => {
  enabled();
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ spaces: [] }), { status: 200 });
  const okRes = await GET(request("GET", "spaces"), {
    params: Promise.resolve({ path: ["spaces"] }),
  });
  assert.equal(okRes.status, 200);
  assert.equal(okRes.headers.get("cache-control"), "no-store");
  assert.equal(okRes.headers.get("x-content-type-options"), "nosniff");

  const errRes = await GET(request("GET", "unknown-path"), {
    params: Promise.resolve({ path: ["unknown-path"] }),
  });
  assert.equal(errRes.status, 404);
  assert.equal(errRes.headers.get("cache-control"), "no-store");
  assert.equal(errRes.headers.get("x-content-type-options"), "nosniff");

  const deleteRes = await DELETE(request("DELETE", "unknown-path"), {
    params: Promise.resolve({ path: ["unknown-path"] }),
  });
  assert.equal(deleteRes.status, 404);
  assert.equal(deleteRes.headers.get("cache-control"), "no-store");
  assert.equal(deleteRes.headers.get("x-content-type-options"), "nosniff");
});

test("PUT mutation checks CSRF origin and forwards to owner space upstream", async () => {
  enabled();
  let forwardedUrl, forwardedMethod;
  globalThis.fetch = async (url, opts) => {
    forwardedUrl = url;
    forwardedMethod = opts.method;
    return new Response(JSON.stringify({ id: "sp-1", state: "activa" }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
  const spaceId = "00000000-0000-4000-8000-000000000001";
  const pathParams = ["owner", "spaces", spaceId];

  // 1. Invalid origin rejected with 403
  const badOrigin = await PUT(
    request("PUT", `owner/spaces/${spaceId}`, {
      headers: { origin: "http://attacker.local" },
      body: JSON.stringify({ state: "activa" }),
    }),
    { params: Promise.resolve({ path: pathParams }) },
  );
  assert.equal(badOrigin.status, 403);

  // 2. Valid origin forwarded to upstream route
  const okRes = await PUT(
    request("PUT", `owner/spaces/${spaceId}`, {
      headers: { origin: "http://127.0.0.1:3000" },
      body: JSON.stringify({ state: "activa" }),
    }),
    { params: Promise.resolve({ path: pathParams }) },
  );
  assert.equal(okRes.status, 200);
  assert.equal(forwardedMethod, "PUT");
  assert.equal(
    String(forwardedUrl),
    `http://127.0.0.1:18084/api/v1/local/rental-demo/owner/spaces/${spaceId}`,
  );
  assert.equal(okRes.headers.get("cache-control"), "no-store");
  assert.equal(okRes.headers.get("x-content-type-options"), "nosniff");
});
