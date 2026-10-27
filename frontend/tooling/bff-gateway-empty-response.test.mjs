import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bff-gateway-empty-"),
);

async function compile(sourcePath, outputName, replacements = []) {
  let source = await readFile(path.join(root, sourcePath), "utf8");
  for (const [before, after] of replacements)
    source = source.replaceAll(before, after);
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const destination = path.join(scratch, outputName);
  await writeFile(destination, output);
  return destination;
}

await writeFile(
  path.join(scratch, "server-config.mjs"),
  `export function getApiBackendBaseUrl() {
  return new URL(process.env.API_BACKEND_BASE_URL || "http://127.0.0.1:18082/api/v1");
}`,
);

await writeFile(
  path.join(scratch, "cloud-run-identity.mjs"),
  `export async function getCloudRunIdentityHeadersForBackend() {
  return {};
}`,
);

await compile("src/lib/api/server-session.ts", "server-session.mjs", [
  ['import "server-only";', ""],
]);

const origOpsPath = await compile(
  "src/lib/api/generated/gateway-operations.ts",
  "orig-gateway-operations.mjs",
);
const { gatewayOperations: rawOps } = await import(
  pathToFileURL(origOpsPath).href
);

const extendedOperations = [
  ...rawOps,
  {
    method: "POST",
    path: "/test/generic",
    operationId: "postTestGeneric",
    query: [],
    headers: [],
    bearer: false,
    requestMediaTypes: ["application/json"],
    responseMediaTypes: ["application/json"],
  },
  {
    method: "HEAD",
    path: "/test/generic",
    operationId: "headTestGeneric",
    query: [],
    headers: [],
    bearer: false,
    requestMediaTypes: [],
    responseMediaTypes: ["application/json"],
  },
];

await writeFile(
  path.join(scratch, "gateway-operations.mjs"),
  `export const gatewayOperations = ${JSON.stringify(extendedOperations)};`,
);

const routePath = await compile(
  "src/app/api/backend/[...path]/route.ts",
  "route.mjs",
  [
    ['import "server-only";', ""],
    ["@/lib/api/server-config", "./server-config.mjs"],
    ["@/lib/api/cloud-run-identity", "./cloud-run-identity.mjs"],
    ["@/lib/api/server-session", "./server-session.mjs"],
    ["@/lib/api/generated/gateway-operations", "./gateway-operations.mjs"],
  ],
);

const { POST, HEAD } = await import(pathToFileURL(routePath).href);

const originalFetch = globalThis.fetch;

test.after(async () => {
  globalThis.fetch = originalFetch;
  await rm(scratch, { recursive: true, force: true });
});

function makeRequest(
  pathname,
  method = "POST",
  bodyJson = { token: "synthetic-token-12345" },
) {
  const headers = {
    host: "127.0.0.1:4201",
    "x-request-id": "req-unit-test-123",
  };
  if (method === "POST") {
    headers.origin = "https://127.0.0.1:4201";
    headers["content-type"] = "application/json";
  }
  return new Request(`http://127.0.0.1:4201${pathname}`, {
    method,
    headers,
    body: method === "POST" ? JSON.stringify(bodyJson) : undefined,
  });
}

function makeContext(segments) {
  return {
    params: Promise.resolve({ path: segments }),
  };
}

test("POST /auth/verification 204 with text/plain preserves 204, no-store, x-request-id and empty body", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 204,
      statusText: "No Content",
      headers: {
        "content-type": "text/plain; charset=utf-8",
        "x-request-id": "req-upstream-xyz",
      },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 204);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(res.headers.get("x-request-id"), "req-upstream-xyz");
  const text = await res.text();
  assert.equal(text, "");
});

test("POST /auth/verification 204 with application/octet-stream preserves 204 without 502", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 204,
      headers: {
        "content-type": "application/octet-stream",
        "x-request-id": "req-upstream-oct",
      },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 204);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(res.headers.get("x-request-id"), "req-upstream-oct");
  assert.equal(await res.text(), "");
});

test("POST /auth/verification 204 without Content-Type preserves 204", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 204,
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 204);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(await res.text(), "");
});

test("HEAD /test/generic with upstream body cancels upstream stream and returns null body", async (t) => {
  let cancelled = false;
  const stream = new ReadableStream({
    start(controller) {
      controller.enqueue(new TextEncoder().encode("extra-bytes"));
      controller.close();
    },
    cancel() {
      cancelled = true;
    },
  });

  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(stream, {
      status: 200,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/test/generic", "HEAD");
  const res = await HEAD(req, makeContext(["test", "generic"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 200);
  assert.equal(cancelled, true);
  assert.equal(await res.text(), "");
});

test("POST /auth/verification/reissue 204 with text/plain preserves 204", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 204,
      headers: { "content-type": "text/plain; charset=utf-8" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification/reissue", "POST", {
    email: "user@example.com",
  });
  const res = await POST(req, makeContext(["auth", "verification", "reissue"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 204);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(await res.text(), "");
});

test("POST /auth/password/recovery/consume 204 with text/plain preserves 204", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 204,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest(
    "/api/backend/auth/password/recovery/consume",
    "POST",
    { token: "tok-123", new_password: "ValidPass123!" },
  );
  const res = await POST(
    req,
    makeContext(["auth", "password", "recovery", "consume"]),
  );

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 204);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(await res.text(), "");
});

test("POST /test/generic 205 (Reset Content) with text/plain preserves 205 and empty body", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 205,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/test/generic", "POST");
  const res = await POST(req, makeContext(["test", "generic"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 205);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(await res.text(), "");
});

test("POST /test/generic 304 (Not Modified) with text/plain preserves 304 and empty body", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 304,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/test/generic", "POST");
  const res = await POST(req, makeContext(["test", "generic"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 304);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(await res.text(), "");
});

test("HEAD /test/generic preserves status and empty body even with text/plain", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 200,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/test/generic", "HEAD");
  const res = await HEAD(req, makeContext(["test", "generic"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(await res.text(), "");
});

test("POST /auth/verification 200 with invalid media type (text/plain) STILL fails with 502", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 200,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 502);
  const json = await res.json();
  assert.equal(json.error.code, "upstream_content_type_not_allowed");
});

test("POST /auth/verification 201 with invalid media type (application/octet-stream) STILL fails with 502", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(null, {
      status: 201,
      headers: { "content-type": "application/octet-stream" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 502);
  const json = await res.json();
  assert.equal(json.error.code, "upstream_content_type_not_allowed");
});

test("POST /auth/verification with browser DNT: 1 header is ignored and returns 204 without forwarding dnt to upstream", async (t) => {
  let capturedUpstreamHeaders = null;
  globalThis.fetch = async (url, init) => {
    capturedUpstreamHeaders = init.headers;
    return new Response(null, {
      status: 204,
      headers: { "content-type": "text/plain" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  req.headers.set("dnt", "1");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(res.status, 204);
  assert.equal(capturedUpstreamHeaders.has("dnt"), false);
});

test("POST /auth/verification with arbitrary x-unknown header STILL returns 400 header_not_allowed", async (t) => {
  let upstreamCalled = false;
  globalThis.fetch = async () => {
    upstreamCalled = true;
    return new Response(null, { status: 204 });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  req.headers.set("x-unknown", "arbitrary");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalled, false);
  assert.equal(res.status, 400);
  const json = await res.json();
  assert.equal(json.error.code, "header_not_allowed");
});

test("POST /auth/verification 200 with valid application/json returns 200 and passes JSON payload", async (t) => {
  let upstreamCalls = 0;
  globalThis.fetch = async () => {
    upstreamCalls++;
    return new Response(JSON.stringify({ status: "verified" }), {
      status: 200,
      headers: { "content-type": "application/json; charset=utf-8" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = makeRequest("/api/backend/auth/verification", "POST");
  const res = await POST(req, makeContext(["auth", "verification"]));

  assert.equal(upstreamCalls, 1);
  assert.equal(res.status, 200);
  const json = await res.json();
  assert.deepEqual(json, { status: "verified" });
});
