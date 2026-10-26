import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bff-session-"),
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

const serverSessionPath = await compile(
  "src/lib/api/server-session.ts",
  "server-session.mjs",
  [['import "server-only";', ""]],
);

const {
  SESSION_COOKIE_NAME,
  EXPIRY_COOKIE_NAME,
  sanitizeHostHeader,
  validateCsrfOrigin,
  parseSessionCookies,
  buildSessionCookies,
  buildClearSessionCookies,
  errorResponse,
} = await import(pathToFileURL(serverSessionPath).href);

test("sanitizeHostHeader accepts valid hosts and rejects malicious or invalid inputs", () => {
  assert.equal(sanitizeHostHeader("example.com"), "example.com");
  assert.equal(sanitizeHostHeader("127.0.0.1:4201"), "127.0.0.1:4201");
  assert.equal(sanitizeHostHeader("  example.com  "), "example.com");
  assert.equal(sanitizeHostHeader(""), null);
  assert.equal(sanitizeHostHeader(null), null);
  assert.equal(sanitizeHostHeader("bad host with space"), null);
  assert.equal(sanitizeHostHeader("host/path"), null);
  assert.equal(sanitizeHostHeader("host@user"), null);
  assert.equal(sanitizeHostHeader("host?query"), null);
  assert.equal(sanitizeHostHeader("a".repeat(256)), null);
});

test("validateCsrfOrigin enforces HTTPS same-origin and blocks mutations without Origin", () => {
  const createReq = (headers) =>
    new Request("http://127.0.0.1:4201/api/backend/auth/login", {
      headers,
    });

  assert.equal(
    validateCsrfOrigin(
      createReq({
        host: "app.espacigo.com",
        origin: "https://app.espacigo.com",
      }),
      { isMutation: true },
    ),
    true,
  );

  assert.equal(
    validateCsrfOrigin(createReq({ host: "app.espacigo.com" }), {
      isMutation: true,
    }),
    false,
  );

  assert.equal(
    validateCsrfOrigin(createReq({ host: "app.espacigo.com" }), {
      isMutation: false,
    }),
    true,
  );

  assert.equal(
    validateCsrfOrigin(
      createReq({
        host: "app.espacigo.com",
        origin: "http://app.espacigo.com",
      }),
      { isMutation: true },
    ),
    false,
  );

  assert.equal(
    validateCsrfOrigin(
      createReq({ host: "app.espacigo.com", origin: "null" }),
      { isMutation: true },
    ),
    false,
  );

  assert.equal(
    validateCsrfOrigin(
      createReq({
        host: "app.espacigo.com",
        origin: "https://app.espacigo.com",
        "sec-fetch-site": "cross-site",
      }),
      { isMutation: true },
    ),
    false,
  );
});

test("parseSessionCookies extracts token and expiry, rejects anomalies and duplicates", () => {
  const validToken = "synthetic-go-opaque-token-12345";
  const futureIso = new Date(Date.now() + 3600 * 1000).toISOString();
  const futureNanoIso = new Date(Date.now() + 3600 * 1000)
    .toISOString()
    .replace(".000Z", ".123456789Z");
  const expiredIso = new Date(Date.now() - 1000).toISOString();

  const normal = `${SESSION_COOKIE_NAME}=${validToken}; ${EXPIRY_COOKIE_NAME}=${futureIso}`;
  assert.deepEqual(parseSessionCookies(normal), {
    token: validToken,
    expiresAt: futureIso,
  });

  // Preserva precisión original de Go (incluyendo nanosegundos)
  const withNano = `${SESSION_COOKIE_NAME}=${validToken}; ${EXPIRY_COOKIE_NAME}=${futureNanoIso}`;
  assert.deepEqual(parseSessionCookies(withNano), {
    token: validToken,
    expiresAt: futureNanoIso,
  });

  // VIOLACIÓN DE CONTRATO PREVIA CORREGIDA: el contrato fail-closed exige
  // exactamente 1 metadato de expiración RFC3339 válido y futuro. Sin metadato
  // debe retornar null (evitando inventar expiraciones o restaurar sesiones sin expiry).
  const onlyToken = `${SESSION_COOKIE_NAME}=${validToken}`;
  assert.equal(parseSessionCookies(onlyToken), null);

  // Expirada debe ser rechazada
  const expiredCookie = `${SESSION_COOKIE_NAME}=${validToken}; ${EXPIRY_COOKIE_NAME}=${expiredIso}`;
  assert.equal(parseSessionCookies(expiredCookie), null);

  // Metadato inválido debe ser rechazado
  const invalidExpiry = `${SESSION_COOKIE_NAME}=${validToken}; ${EXPIRY_COOKIE_NAME}=not-a-date`;
  assert.equal(parseSessionCookies(invalidExpiry), null);

  assert.equal(parseSessionCookies(null), null);
  assert.equal(parseSessionCookies(""), null);
  assert.equal(parseSessionCookies(`${EXPIRY_COOKIE_NAME}=${futureIso}`), null);
  assert.equal(
    parseSessionCookies(
      `${SESSION_COOKIE_NAME}=short; ${EXPIRY_COOKIE_NAME}=${futureIso}`,
    ),
    null,
  );
  assert.equal(
    parseSessionCookies(
      `${SESSION_COOKIE_NAME}=${validToken}; ${SESSION_COOKIE_NAME}=${validToken}; ${EXPIRY_COOKIE_NAME}=${futureIso}`,
    ),
    null,
  );
  assert.equal(
    parseSessionCookies(
      `${SESSION_COOKIE_NAME}=${validToken}; ${EXPIRY_COOKIE_NAME}=${futureIso}; ${EXPIRY_COOKIE_NAME}=${futureIso}`,
    ),
    null,
  );
});

test("buildSessionCookies and buildClearSessionCookies format standard host-only cookies", () => {
  const validToken = "synthetic-go-opaque-token-12345";
  const futureIso = new Date(Date.now() + 3600 * 1000).toISOString();

  const [c1, c2] = buildSessionCookies(validToken, futureIso);
  assert.ok(c1.startsWith(`${SESSION_COOKIE_NAME}=${validToken};`));
  assert.ok(c1.includes("Path=/"));
  assert.ok(c1.includes("HttpOnly"));
  assert.ok(c1.includes("Secure"));
  assert.ok(c1.includes("SameSite=Lax"));
  assert.ok(c1.includes("Max-Age="));
  assert.ok(!c1.includes("Domain="));

  assert.ok(c2.startsWith(`${EXPIRY_COOKIE_NAME}=${futureIso};`));
  assert.ok(c2.includes("Path=/"));
  assert.ok(c2.includes("HttpOnly"));
  assert.ok(c2.includes("Secure"));
  assert.ok(c2.includes("SameSite=Lax"));

  const [clear1, clear2] = buildClearSessionCookies();
  assert.ok(clear1.startsWith(`${SESSION_COOKIE_NAME}=;`));
  assert.ok(clear1.includes("Max-Age=0"));
  assert.ok(clear1.includes("Expires=Thu, 01 Jan 1970 00:00:00 GMT"));
  assert.ok(clear2.startsWith(`${EXPIRY_COOKIE_NAME}=;`));
  assert.ok(clear2.includes("Max-Age=0"));
});

test("errorResponse generates standardized sanitized API errors", async () => {
  const res = errorResponse(401, "unauthenticated", "req-123", {
    "x-extra": "val",
  });
  assert.equal(res.status, 401);
  assert.equal(res.headers.get("x-request-id"), "req-123");
  assert.equal(res.headers.get("cache-control"), "no-store");
  assert.equal(res.headers.get("x-extra"), "val");
  const body = await res.json();
  assert.deepEqual(body, {
    error: {
      code: "unauthenticated",
      message: "La solicitud no pudo completarse.",
      request_id: "req-123",
    },
  });
});

test.after(async () => rm(scratch, { recursive: true, force: true }));
