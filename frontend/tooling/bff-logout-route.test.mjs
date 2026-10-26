import assert from "node:assert/strict";
import http from "node:http";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bff-logout-"),
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

await compile("src/lib/api/server-config.ts", "server-config.mjs", [
  ['import "server-only";', ""],
]);

await compile("src/lib/api/cloud-run-identity.ts", "cloud-run-identity.mjs");

const logoutRoutePath = await compile(
  "src/app/api/backend/auth/logout/route.ts",
  "logout-route.mjs",
  [
    ['import "server-only";', ""],
    ["@/lib/api/server-config", "./server-config.mjs"],
    ["@/lib/api/cloud-run-identity", "./cloud-run-identity.mjs"],
    ["@/lib/api/server-session", "./server-session.mjs"],
  ],
);

const { POST } = await import(pathToFileURL(logoutRoutePath).href);

function createMockGoServer() {
  const calls = [];
  let nextStatus = 204;
  const server = http.createServer((req, res) => {
    calls.push({
      method: req.method,
      url: req.url,
      headers: { ...req.headers },
    });
    res.writeHead(nextStatus, { "content-type": "application/json" });
    if (nextStatus === 204) {
      res.end();
    } else {
      res.end(JSON.stringify({ error: { code: "upstream_error" } }));
    }
  });
  return {
    server,
    calls,
    setNextStatus(s) {
      nextStatus = s;
    },
    async listen() {
      await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
      return server.address().port;
    },
    async close() {
      await new Promise((resolve) => server.close(resolve));
    },
  };
}

test("RED 1: expiry faltante con Go Token llama a Go Logout con Bearer y limpia sesion", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const validToken = "synthetic-go-opaque-token-12345";
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          cookie: `__Host-espacigo-session=${validToken}`,
          "x-request-id": "req-logout-red1",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 204);
    assert.equal(res.headers.get("x-espacigo-session-cleared"), "1");
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.equal(res.headers.get("x-request-id"), "req-logout-red1");

    const setCookies = res.headers.getSetCookie();
    assert.equal(setCookies.length, 2);
    assert.ok(
      setCookies.some(
        (c) =>
          c.includes("__Host-espacigo-session=;") && c.includes("Max-Age=0"),
      ),
    );
    assert.ok(
      setCookies.some(
        (c) =>
          c.includes("__Host-espacigo-session-expiry=;") &&
          c.includes("Max-Age=0"),
      ),
    );

    assert.equal(mock.calls.length, 1);
    assert.equal(mock.calls[0].method, "POST");
    assert.equal(mock.calls[0].url, "/api/v1/auth/logout");
    assert.equal(mock.calls[0].headers.authorization, `Bearer ${validToken}`);
    assert.equal(mock.calls[0].headers.cookie, undefined);
  } finally {
    await mock.close();
  }
});

test("RED 2: Authorization header en request del browser es bloqueado con 400 sin limpiar sesion", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          authorization: "Bearer browser-injected-token",
          "x-request-id": "req-logout-red2",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 400);
    assert.equal(res.headers.get("x-espacigo-session-cleared"), null);
    assert.equal(res.headers.getSetCookie().length, 0);

    const body = await res.json();
    assert.equal(body.error?.code, "authorization_not_allowed");
    assert.equal(mock.calls.length, 0);
  } finally {
    await mock.close();
  }
});

test("RED 3: duplicate token o malformado limpia cookies con 502 sin afirmar revoke remoto", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const t1 = "token-opaque-duplicate-11111";
    const t2 = "token-opaque-duplicate-22222";
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          cookie: `__Host-espacigo-session=${t1}; __Host-espacigo-session=${t2}`,
          "x-request-id": "req-logout-red3",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 502);
    assert.equal(res.headers.get("x-espacigo-session-cleared"), "1");
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.equal(res.headers.getSetCookie().length, 2);

    const body = await res.json();
    assert.equal(body.error?.code, "remote_logout_unconfirmed");
    assert.equal(mock.calls.length, 0);
  } finally {
    await mock.close();
  }
});

test("sin cookie de sesion devuelve 204 idempotente local valido limpiando cookies", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          cookie: "other_cookie=value",
          "x-request-id": "req-logout-empty",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 204);
    assert.equal(res.headers.get("x-espacigo-session-cleared"), "1");
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.equal(res.headers.getSetCookie().length, 2);
    assert.equal(mock.calls.length, 0);
  } finally {
    await mock.close();
  }
});

test("fallo remoto upstream 503/red devuelve 502 remote_logout_unconfirmed con flag clear", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  mock.setNextStatus(503);
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const validToken = "synthetic-go-opaque-token-12345";
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          cookie: `__Host-espacigo-session=${validToken}`,
          "x-request-id": "req-logout-upfail",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 502);
    assert.equal(res.headers.get("x-espacigo-session-cleared"), "1");
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.equal(res.headers.getSetCookie().length, 2);
    const body = await res.json();
    assert.equal(body.error?.code, "remote_logout_unconfirmed");
    assert.equal(mock.calls.length, 1);
  } finally {
    await mock.close();
  }
});

test("upstream 401 inactivo devuelve 204 local con flag clear", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  mock.setNextStatus(401);
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const validToken = "synthetic-go-opaque-token-12345";
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          cookie: `__Host-espacigo-session=${validToken}`,
          "x-request-id": "req-logout-up401",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 204);
    assert.equal(res.headers.get("x-espacigo-session-cleared"), "1");
    assert.equal(res.headers.get("cache-control"), "private, no-store");
    assert.equal(res.headers.getSetCookie().length, 2);
    assert.equal(mock.calls.length, 1);
  } finally {
    await mock.close();
  }
});

test("CSRF origin no permitido bloquea con 403 antes de limpiar sesion", async () => {
  const req = new Request("https://app.espacigo.com/api/backend/auth/logout", {
    method: "POST",
    headers: {
      host: "app.espacigo.com",
      origin: "https://malicious.invalid",
      cookie: "__Host-espacigo-session=synthetic-go-opaque-token-12345",
      "x-request-id": "req-logout-csrf",
    },
  });

  const res = await POST(req);
  assert.equal(res.status, 403);
  assert.equal(res.headers.get("x-espacigo-session-cleared"), null);
  assert.equal(res.headers.getSetCookie().length, 0);
  const body = await res.json();
  assert.equal(body.error?.code, "origin_not_allowed");
});

test("expiry cookie vencida o invalida aun extrae token valido e intenta Go Logout", async () => {
  const mock = createMockGoServer();
  const port = await mock.listen();
  process.env.API_BACKEND_BASE_URL = `http://127.0.0.1:${port}/api/v1`;

  try {
    const validToken = "synthetic-go-opaque-token-12345";
    const expiredDate = new Date(Date.now() - 3600_000).toISOString();
    const req = new Request(
      "https://app.espacigo.com/api/backend/auth/logout",
      {
        method: "POST",
        headers: {
          host: "app.espacigo.com",
          origin: "https://app.espacigo.com",
          cookie: `__Host-espacigo-session=${validToken}; __Host-espacigo-session-expiry=${expiredDate}`,
          "x-request-id": "req-logout-expired-meta",
        },
      },
    );

    const res = await POST(req);
    assert.equal(res.status, 204);
    assert.equal(mock.calls.length, 1);
    assert.equal(mock.calls[0].headers.authorization, `Bearer ${validToken}`);
  } finally {
    await mock.close();
  }
});

test.after(async () => rm(scratch, { recursive: true, force: true }));
