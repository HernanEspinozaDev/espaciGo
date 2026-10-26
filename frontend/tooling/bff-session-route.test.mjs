import assert from "node:assert/strict";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bff-session-route-"),
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
  if (process.env.MOCK_IAM_HEADER) {
    return { "X-Serverless-Authorization": process.env.MOCK_IAM_HEADER };
  }
  return {};
}`,
);

const serverSessionPath = await compile(
  "src/lib/api/server-session.ts",
  "server-session.mjs",
  [['import "server-only";', ""]],
);

await compile(
  "src/lib/api/bff-login-validation.ts",
  "bff-login-validation.mjs",
  [['import "server-only";', ""]],
);

const { buildSessionCookies } = await import(
  pathToFileURL(serverSessionPath).href
);

const routePath = await compile(
  "src/app/api/backend/auth/session/route.ts",
  "session-route.mjs",
  [
    ['import "server-only";', ""],
    ["@/lib/api/server-config", "./server-config.mjs"],
    ["@/lib/api/cloud-run-identity", "./cloud-run-identity.mjs"],
    ["@/lib/api/server-session", "./server-session.mjs"],
    ["@/lib/api/bff-login-validation", "./bff-login-validation.mjs"],
  ],
);

const { GET } = await import(pathToFileURL(routePath).href);

const originalFetch = globalThis.fetch;
const validToken = "a".repeat(32);
const validExpiry = new Date(Date.now() + 3600_000).toISOString();
const [validCookie1, validCookie2] = buildSessionCookies(
  validToken,
  validExpiry,
);
const validCookieHeader = `${validCookie1}; ${validCookie2}`;

test("GET rejects browser Authorization header with 400 before upstream or cookies", async (t) => {
  let fetchCalled = false;
  globalThis.fetch = async () => {
    fetchCalled = true;
    return new Response(JSON.stringify({}), { status: 200 });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      authorization: "Bearer browser-token",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 400);
  assert.equal(fetchCalled, false);
  const data = await res.json();
  assert.equal(data.error.code, "authorization_not_allowed");
});

test("GET preserves cookies and returns 403 forbidden when upstream returns 403", async (t) => {
  globalThis.fetch = async () => {
    return new Response(JSON.stringify({ error: { code: "forbidden" } }), {
      status: 403,
      headers: { "content-type": "application/json" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 403);
  assert.equal(res.headers.get("set-cookie"), null);
  const data = await res.json();
  assert.equal(data.error.code, "forbidden");
});

test("GET clears cookies and returns 401 unauthenticated when upstream returns 401", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({ error: { code: "unauthenticated" } }),
      {
        status: 401,
        headers: { "content-type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 401);
  const setCookie = res.headers.get("set-cookie");
  assert.ok(setCookie && setCookie.includes("Max-Age=0"));
  const data = await res.json();
  assert.equal(data.error.code, "unauthenticated");
});

test("GET blocks upstream 3xx redirects and returns sanitized 502 without Location or Set-Cookie", async (t) => {
  globalThis.fetch = async () => {
    return new Response(null, {
      status: 302,
      headers: {
        location: "https://attacker.example.com/steal",
        "set-cookie": "malicious=1; Path=/",
      },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  assert.equal(res.headers.get("location"), null);
  assert.equal(res.headers.get("set-cookie"), null);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});

test("GET rejects non-JSON upstream Content-Type with 502 invalid_upstream_response", async (t) => {
  globalThis.fetch = async () => {
    return new Response("<html><body>Not Found</body></html>", {
      status: 200,
      headers: { "content-type": "text/html" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});

test("GET rejects upstream payloads exceeding 64KiB with 502 invalid_upstream_response", async (t) => {
  globalThis.fetch = async () => {
    const largeString = "x".repeat(70 * 1024);
    return new Response(JSON.stringify({ data: largeString }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});

test("GET sanitizes upstream error codes and propagates safe retry-after without leaking body", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({
        error: { code: "rate_limited", secret_leak: "sensitive_data" },
      }),
      {
        status: 429,
        headers: {
          "content-type": "application/json",
          "retry-after": "30",
        },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 429);
  assert.equal(res.headers.get("retry-after"), "30");
  const data = await res.json();
  assert.equal(data.error.code, "rate_limited");
  assert.equal(data.error.secret_leak, undefined);
});

test("GET rejects invalid UUID in account_id with 502 invalid_upstream_response", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({
        account_id: "not-a-valid-uuid",
        roles: ["user"],
        capabilities: ["read"],
        restricted_mode: false,
      }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});

test("GET rejects roles or capabilities containing session token with 502", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({
        account_id: "a0000000-0000-4000-8000-000000000001",
        roles: [validToken],
        capabilities: ["read"],
        restricted_mode: false,
      }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});

test("GET rejects non-boolean restricted_mode with 502", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({
        account_id: "a0000000-0000-4000-8000-000000000001",
        roles: ["user"],
        capabilities: ["read"],
        restricted_mode: "true",
      }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});

test("GET returns safe 5-field DTO solely using session.expiresAt and private no-store cache without cookie refresh", async (t) => {
  let capturedAuth = null;
  let capturedIam = null;
  process.env.MOCK_IAM_HEADER = "Bearer mock-iam-token";
  t.after(() => {
    delete process.env.MOCK_IAM_HEADER;
  });

  globalThis.fetch = async (url, options) => {
    capturedAuth = options.headers.get("authorization");
    capturedIam = options.headers.get("X-Serverless-Authorization");
    return new Response(
      JSON.stringify({
        account_id: "a0000000-0000-4000-8000-000000000001",
        roles: ["cliente", "anfitrion"],
        capabilities: ["spaces:read", "bookings:create"],
        restricted_mode: false,
        upstream_secret: "leak_candidate",
        access_token: "should_not_leak",
      }),
      {
        status: 200,
        headers: { "content-type": "application/json; charset=utf-8" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("cache-control"), "private, no-store");
  assert.equal(res.headers.get("set-cookie"), null);
  assert.equal(capturedAuth, `Bearer ${validToken}`);
  assert.equal(capturedIam, "Bearer mock-iam-token");

  const body = await res.json();
  assert.deepEqual(Object.keys(body).sort(), [
    "account_id",
    "capabilities",
    "expires_at",
    "restricted_mode",
    "roles",
  ]);
  assert.equal(body.account_id, "a0000000-0000-4000-8000-000000000001");
  assert.deepEqual(body.roles, ["cliente", "anfitrion"]);
  assert.deepEqual(body.capabilities, ["spaces:read", "bookings:create"]);
  assert.equal(body.restricted_mode, false);
  assert.equal(body.expires_at, validExpiry);
  assert.equal(body.upstream_secret, undefined);
  assert.equal(body.access_token, undefined);
});

test("GET sanitizes arbitrary alphanumeric error codes to http_${status}", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({
        error: { code: "syntheticOpaqueSecret123" },
      }),
      {
        status: 500,
        headers: { "content-type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 500);
  const data = await res.json();
  assert.equal(data.error.code, "http_500");
});

test("GET rejects roles with whitespace (no normalization) with 502", async (t) => {
  globalThis.fetch = async () => {
    return new Response(
      JSON.stringify({
        account_id: "a0000000-0000-4000-8000-000000000001",
        roles: [" administrador_global "],
        capabilities: [],
        restricted_mode: false,
      }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  };
  t.after(() => {
    globalThis.fetch = originalFetch;
  });

  const req = new Request("http://127.0.0.1:4201/api/backend/auth/session", {
    headers: {
      host: "127.0.0.1:4201",
      cookie: validCookieHeader,
    },
  });

  const res = await GET(req);
  assert.equal(res.status, 502);
  const data = await res.json();
  assert.equal(data.error.code, "invalid_upstream_response");
});
