import assert from "node:assert/strict";
import http from "node:http";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bff-login-"),
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

await compile("src/lib/api/server-session.ts", "server-session.mjs", [
  ['import "server-only";', ""],
]);

await compile(
  "src/lib/api/bff-login-validation.ts",
  "bff-login-validation.mjs",
  [['import "server-only";', ""]],
);

await writeFile(
  path.join(scratch, "server-config.mjs"),
  `let backendUrl = "http://127.0.0.1:49999/api/v1";
export function setMockBackendUrl(url) { backendUrl = url; }
export function getApiBackendBaseUrl() { return new URL(backendUrl); }
`,
);

await writeFile(
  path.join(scratch, "cloud-run-identity.mjs"),
  `export async function getCloudRunIdentityHeadersForBackend() { return {}; }
`,
);

const routePath = await compile(
  "src/app/api/backend/auth/login/route.ts",
  "route.mjs",
  [
    ['import "server-only";', ""],
    ["@/lib/api/server-config", "./server-config.mjs"],
    ["@/lib/api/cloud-run-identity", "./cloud-run-identity.mjs"],
    ["@/lib/api/server-session", "./server-session.mjs"],
    ["@/lib/api/bff-login-validation", "./bff-login-validation.mjs"],
  ],
);

const { POST } = await import(pathToFileURL(routePath).href);
const { setMockBackendUrl } = await import(
  pathToFileURL(path.join(scratch, "server-config.mjs")).href
);
const {
  validateStringArray,
  sanitizeErrorCode,
  validateGoSessionResponse,
  validateGoLoginResponse,
} = await import(
  pathToFileURL(path.join(scratch, "bff-login-validation.mjs")).href
);

async function withUpstreamServer(handler, fn) {
  const server = http.createServer((req, res) => {
    res.setHeader("connection", "close");
    handler(req, res);
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const port = server.address().port;
  setMockBackendUrl(`http://127.0.0.1:${port}/api/v1`);
  try {
    await fn(`http://127.0.0.1:${port}/api/v1`);
  } finally {
    if (server.closeAllConnections) server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}

test("rejects browser Authorization header with 400 authorization_not_allowed before upstream", async () => {
  const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
    method: "POST",
    headers: {
      host: "127.0.0.1:4201",
      origin: "https://127.0.0.1:4201",
      "content-type": "application/json",
      authorization: "Bearer browser-injected-token",
    },
    body: JSON.stringify({
      email: "bff-test@example.invalid",
      password: "SyntheticBff!123",
    }),
  });

  const res = await POST(req);
  assert.equal(res.status, 400);
  const body = await res.json();
  assert.equal(body.error?.code, "authorization_not_allowed");
});

test("rejects Content-Length exceeding 64KiB with 413 too_large", async () => {
  const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
    method: "POST",
    headers: {
      host: "127.0.0.1:4201",
      origin: "https://127.0.0.1:4201",
      "content-type": "application/json",
      "content-length": "65537",
    },
    body: JSON.stringify({ email: "a@example.invalid", password: "x" }),
  });
  const res = await POST(req);
  assert.equal(res.status, 413);
  const body = await res.json();
  assert.equal(body.error?.code, "too_large");
});

test("rejects invalid Content-Length with 400 invalid_body", async () => {
  const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
    method: "POST",
    headers: {
      host: "127.0.0.1:4201",
      origin: "https://127.0.0.1:4201",
      "content-type": "application/json",
      "content-length": "not-a-number",
    },
    body: JSON.stringify({ email: "a@example.invalid", password: "x" }),
  });
  const res = await POST(req);
  assert.equal(res.status, 400);
  const body = await res.json();
  assert.equal(body.error?.code, "invalid_body");
});

test("cancels stream and returns 413 when body stream exceeds 64KiB", async () => {
  let cancelCalled = false;
  const chunk = new Uint8Array(16 * 1024);
  let chunkIndex = 0;
  const stream = new ReadableStream({
    pull(controller) {
      if (chunkIndex >= 5) {
        controller.close();
        return;
      }
      chunkIndex++;
      controller.enqueue(chunk);
    },
    cancel() {
      cancelCalled = true;
    },
  });
  const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
    method: "POST",
    headers: {
      host: "127.0.0.1:4201",
      origin: "https://127.0.0.1:4201",
      "content-type": "application/json",
    },
    body: stream,
    duplex: "half",
  });
  const res = await POST(req);
  assert.equal(res.status, 413);
  assert.equal(cancelCalled, true);
  const body = await res.json();
  assert.equal(body.error?.code, "too_large");
});

test("rejects non-json content-type and invalid payload", async () => {
  const reqWrongType = new Request(
    "http://127.0.0.1:4201/api/backend/auth/login",
    {
      method: "POST",
      headers: {
        host: "127.0.0.1:4201",
        origin: "https://127.0.0.1:4201",
        "content-type": "text/plain",
      },
      body: "hello",
    },
  );
  const resWrongType = await POST(reqWrongType);
  assert.equal(resWrongType.status, 415);

  const reqBadJson = new Request(
    "http://127.0.0.1:4201/api/backend/auth/login",
    {
      method: "POST",
      headers: {
        host: "127.0.0.1:4201",
        origin: "https://127.0.0.1:4201",
        "content-type": "application/json",
      },
      body: "not-json{",
    },
  );
  const resBadJson = await POST(reqBadJson);
  assert.equal(resBadJson.status, 400);
  assert.equal((await resBadJson.json()).error?.code, "invalid_json");
});

test("blocks upstream 3xx redirect from passing through", async () => {
  await withUpstreamServer(
    (req, res) => {
      res.writeHead(302, {
        location: "https://attacker.example.invalid/phish",
      });
      res.end();
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
      });
      const res = await POST(req);
      assert.equal(res.status, 502);
      const body = await res.json();
      assert.equal(body.error?.code, "invalid_upstream_response");
    },
  );
});

test("sanitizes upstream error codes and retry-after header", async () => {
  await withUpstreamServer(
    (req, res) => {
      res.writeHead(401, {
        "content-type": "application/json",
        "retry-after": "120",
      });
      res.end(
        JSON.stringify({
          error: { code: "secret_raw_token_reflected!#$$" },
        }),
      );
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
      });
      const res = await POST(req);
      assert.equal(res.status, 401);
      assert.equal(res.headers.get("retry-after"), "120");
      const body = await res.json();
      assert.equal(body.error?.code, "http_401");
    },
  );
});

test("rejects invalid Deep DTO shapes from upstream Go login", async () => {
  await withUpstreamServer(
    (req, res) => {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(
        JSON.stringify({
          account_id: "not-a-valid-uuid",
          access_token: "valid-opaque-token-1234567890",
          expires_at: new Date(Date.now() + 60000).toISOString(),
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
        }),
      );
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
      });
      const res = await POST(req);
      assert.equal(res.status, 502);
      const body = await res.json();
      assert.equal(body.error?.code, "invalid_upstream_response");
    },
  );
});

test("triggers rollback and returns 502 when session confirmation account_id mismatches", async () => {
  let logoutCalled = false;
  const account1 = "550e8400-e29b-41d4-a716-446655440000";
  const account2 = "660e8400-e29b-41d4-a716-446655440000";
  await withUpstreamServer(
    (req, res) => {
      if (req.url === "/api/v1/auth/login" && req.method === "POST") {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account1,
            access_token: "valid-opaque-token-1234567890",
            expires_at: new Date(Date.now() + 60000).toISOString(),
            roles: ["arrendatario"],
            capabilities: [],
            restricted_mode: false,
          }),
        );
        return;
      }
      if (req.url === "/api/v1/auth/session" && req.method === "GET") {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account2,
            roles: ["arrendatario"],
            capabilities: [],
            restricted_mode: false,
          }),
        );
        return;
      }
      if (req.url === "/api/v1/auth/logout" && req.method === "POST") {
        logoutCalled = true;
        res.writeHead(204);
        res.end();
        return;
      }
      res.writeHead(404);
      res.end();
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
      });
      const res = await POST(req);
      assert.equal(res.status, 502);
      assert.equal(logoutCalled, true);
      const body = await res.json();
      assert.equal(body.error?.code, "session_confirmation_failed");
    },
  );
});

test("triggers rollback and no cookies when client request is aborted", async () => {
  let logoutCalled = false;
  const account = "550e8400-e29b-41d4-a716-446655440000";
  const controller = new AbortController();
  await withUpstreamServer(
    (req, res) => {
      if (req.url === "/api/v1/auth/login" && req.method === "POST") {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account,
            access_token: "valid-opaque-token-1234567890",
            expires_at: new Date(Date.now() + 60000).toISOString(),
            roles: ["arrendatario"],
            capabilities: [],
            restricted_mode: false,
          }),
        );
        return;
      }
      if (req.url === "/api/v1/auth/session" && req.method === "GET") {
        controller.abort();
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account,
            roles: ["arrendatario"],
            capabilities: [],
            restricted_mode: false,
          }),
        );
        return;
      }
      if (req.url === "/api/v1/auth/logout" && req.method === "POST") {
        logoutCalled = true;
        res.writeHead(204);
        res.end();
        return;
      }
      res.writeHead(404);
      res.end();
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
        signal: controller.signal,
      });
      const res = await POST(req);
      assert.equal(logoutCalled, true);
      assert.equal(res.headers.get("set-cookie"), null);
    },
  );
});

test("happy path returns 200 with exactly 5 whitelisted fields and cookies", async () => {
  const account = "550e8400-e29b-41d4-a716-446655440000";
  const expiry = new Date(Date.now() + 120000).toISOString();
  await withUpstreamServer(
    (req, res) => {
      if (req.url === "/api/v1/auth/login" && req.method === "POST") {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account,
            access_token: "valid-opaque-token-1234567890",
            expires_at: expiry,
            roles: ["login-role"],
            capabilities: [],
            restricted_mode: false,
            extra_unwanted_field: "leak",
          }),
        );
        return;
      }
      if (req.url === "/api/v1/auth/session" && req.method === "GET") {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account,
            roles: ["confirmed-arrendatario"],
            capabilities: ["search"],
            restricted_mode: false,
            another_extra_field: "leak2",
          }),
        );
        return;
      }
      res.writeHead(404);
      res.end();
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
      });
      const res = await POST(req);
      assert.equal(res.status, 200);
      const cookies = res.headers.getSetCookie
        ? res.headers.getSetCookie()
        : [res.headers.get("set-cookie")];
      assert.equal(cookies.length >= 2, true);
      assert.match(cookies.join("; "), /__Host-espacigo-session=/);
      assert.match(cookies.join("; "), /__Host-espacigo-session-expiry=/);

      const data = await res.json();
      const keys = Object.keys(data).sort();
      assert.deepEqual(keys, [
        "account_id",
        "capabilities",
        "expires_at",
        "restricted_mode",
        "roles",
      ]);
      assert.equal(data.account_id, account);
      assert.deepEqual(data.roles, ["confirmed-arrendatario"]);
      assert.deepEqual(data.capabilities, ["search"]);
      assert.equal(data.expires_at, expiry);
      assert.equal(data.restricted_mode, false);
    },
  );
});

test("validateStringArray rejects whitespace in identifiers without normalizing", () => {
  assert.equal(validateStringArray([" administrador_global "]), null);
  assert.deepEqual(validateStringArray(["administrador_global"]), [
    "administrador_global",
  ]);
});

test("sanitizeErrorCode rejects arbitrary alphanumeric secrets and only allows finite allowlist", () => {
  const secret = "syntheticOpaqueSecret123";
  assert.equal(sanitizeErrorCode(secret, 500), "http_500");
  assert.equal(sanitizeErrorCode("internal_error", 500), "internal_error");
  assert.equal(sanitizeErrorCode("login_blocked", 429), "login_blocked");
  assert.equal(sanitizeErrorCode("rate_limited", 429), "rate_limited");
});

test("validateGoSessionResponse rejects roles, capabilities or account_id containing forbidden credential", () => {
  const account = "550e8400-e29b-41d4-a716-446655440000";
  const secret = "syntheticOpaqueSecret12345";
  const rejectedSession = validateGoSessionResponse(
    {
      account_id: account,
      roles: [secret],
      capabilities: [],
      restricted_mode: false,
    },
    secret,
  );
  assert.equal(rejectedSession, null);
});

test("validateGoLoginResponse rejects access_token leaking into roles or account_id", () => {
  const account = "550e8400-e29b-41d4-a716-446655440000";
  const secret = "valid-opaque-token-1234567890";
  const res = validateGoLoginResponse({
    account_id: account,
    access_token: secret,
    expires_at: new Date(Date.now() + 60000).toISOString(),
    roles: [secret],
    capabilities: [],
    restricted_mode: false,
  });
  assert.equal(res, null);
});

test("triggers rollback and returns 502 without cookies when session confirmation stream is aborted or broken", async () => {
  let logoutCalled = false;
  const account = "550e8400-e29b-41d4-a716-446655440000";
  await withUpstreamServer(
    (req, res) => {
      if (req.url === "/api/v1/auth/login" && req.method === "POST") {
        res.writeHead(200, { "content-type": "application/json" });
        res.end(
          JSON.stringify({
            account_id: account,
            access_token: "valid-opaque-token-1234567890",
            expires_at: new Date(Date.now() + 60000).toISOString(),
            roles: ["arrendatario"],
            capabilities: [],
            restricted_mode: false,
          }),
        );
        return;
      }
      if (req.url === "/api/v1/auth/session" && req.method === "GET") {
        res.writeHead(200, {
          "content-type": "application/json",
        });
        res.write('{"account_id":"' + account + '",');
        setTimeout(() => {
          res.destroy(new Error("stream aborted"));
        }, 5);
        return;
      }
      if (req.url === "/api/v1/auth/logout" && req.method === "POST") {
        logoutCalled = true;
        res.writeHead(204);
        res.end();
        return;
      }
      res.writeHead(404);
      res.end();
    },
    async () => {
      const req = new Request("http://127.0.0.1:4201/api/backend/auth/login", {
        method: "POST",
        headers: {
          host: "127.0.0.1:4201",
          origin: "https://127.0.0.1:4201",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          email: "bff-test@example.invalid",
          password: "SyntheticBff!123",
        }),
      });
      const res = await POST(req);
      assert.equal(res.status, 502);
      assert.equal(logoutCalled, true);
      assert.equal(res.headers.get("set-cookie"), null);
      const body = await res.json();
      assert.equal(body.error?.code, "session_confirmation_failed");
    },
  );
});
