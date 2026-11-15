import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratchRoot = process.env.TMPDIR;
if (!scratchRoot)
  throw new Error("TMPDIR must point to isolated test scratch storage");
const consentVersion =
  process.env.NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION;
assert.equal(consentVersion, "id-consent-test-v2");
const backendBaseURL = new URL(process.env.ESPACIGO_TEST_API_BASE_URL ?? "");
assert.equal(backendBaseURL.protocol, "http:");
assert.equal(backendBaseURL.hostname, "127.0.0.1");
assert.notEqual(backendBaseURL.port, "");
assert.equal(backendBaseURL.pathname, "/api/v1");
assert.equal(backendBaseURL.username, "");
assert.equal(backendBaseURL.password, "");
assert.equal(backendBaseURL.search, "");
assert.equal(backendBaseURL.hash, "");

test("real Next BFF saves and separately reads own declaration through Go", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-identity-declaration-bff-go-"),
  );
  try {
    const operationsSource = await readFile(
      path.join(root, "src/lib/api/generated/gateway-operations.ts"),
      "utf8",
    );
    const operationsModule = ts.transpileModule(operationsSource, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText;
    await writeFile(
      path.join(scratch, "gateway-operations.mjs"),
      operationsModule,
    );
    const domainSource = await readFile(
      path.join(
        root,
        "src/features/identity-declaration/domain/identity-declaration.ts",
      ),
      "utf8",
    );
    const domainPath = path.join(scratch, "identity-declaration-domain.mjs");
    await writeFile(
      domainPath,
      ts.transpileModule(domainSource, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
        },
      }).outputText,
    );
    await writeFile(
      path.join(scratch, "server-config.mjs"),
      `export function getApiBackendBaseUrl() { return new URL(${JSON.stringify(backendBaseURL.href)}); }`,
    );
    const routeSource = (
      await readFile(
        path.join(root, "src/app/api/backend/[...path]/route.ts"),
        "utf8",
      )
    )
      .replace(
        'import { gatewayOperations } from "@/lib/api/generated/gateway-operations";',
        'import { gatewayOperations } from "./gateway-operations.mjs";',
      )
      .replace(
        'import { getApiBackendBaseUrl } from "@/lib/api/server-config";',
        'import { getApiBackendBaseUrl } from "./server-config.mjs";',
      );
    const routePath = path.join(scratch, "route.mjs");
    await writeFile(
      routePath,
      ts.transpileModule(routeSource, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
        },
      }).outputText,
    );
    const { POST, GET } = await import(pathToFileURL(routePath));
    const {
      buildIdentityDeclarationRequest,
      parseIdentityDeclarationResponse,
    } = await import(pathToFileURL(domainPath));
    const route =
      "http://127.0.0.1:3000/api/backend/verifications/identity-declaration";
    const payload = buildIdentityDeclarationRequest({
      firstName: "Synthetic",
      lastName: "Person",
      dateOfBirth: "1990-01-02",
      rut: "11.222.333-9",
    });
    assert.equal(payload.consent_version, consentVersion);
    const post = await POST(
      new Request(route, {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          "content-type": "application/json",
        },
        body: JSON.stringify(payload),
      }),
      {
        params: Promise.resolve({
          path: ["verifications", "identity-declaration"],
        }),
      },
    );
    assert.equal(post.status, 201, await post.clone().text());
    assert.equal(post.headers.get("cache-control"), "private, no-store");
    assert.equal(post.headers.get("set-cookie"), null);
    const saved = await post.json();
    assert.deepEqual(saved, {
      id: "11111111-1111-4111-8111-000000000001",
      revision: 1,
      first_name: "Synthetic",
      last_name: "Person",
      date_of_birth: "1990-01-02",
      rut: "11222333-9",
      id_country: "CHL",
      document_type: "ID",
      consent_version: consentVersion,
      consent_given_at: "2026-11-13T12:00:00Z",
    });
    assert.equal(
      parseIdentityDeclarationResponse(saved).consentVersion,
      consentVersion,
    );

    const get = await GET(
      new Request(route, {
        method: "GET",
        headers: { authorization: "Bearer synthetic-session" },
      }),
      {
        params: Promise.resolve({
          path: ["verifications", "identity-declaration"],
        }),
      },
    );
    assert.equal(get.status, 200, await get.clone().text());
    assert.equal(get.headers.get("cache-control"), "private, no-store");
    assert.equal(get.headers.get("set-cookie"), null);
    assert.deepEqual(await get.json(), saved);
    assert.equal(
      parseIdentityDeclarationResponse(saved).consentVersion,
      consentVersion,
    );

    const stale = await POST(
      new Request(route, {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          "content-type": "application/json",
        },
        body: JSON.stringify({
          ...payload,
          consent_version: "id-consent-20261108",
        }),
      }),
      {
        params: Promise.resolve({
          path: ["verifications", "identity-declaration"],
        }),
      },
    );
    assert.equal(stale.status, 422, await stale.clone().text());
    assert.equal(stale.headers.get("cache-control"), "private, no-store");
    assert.equal(stale.headers.get("set-cookie"), null);

    const cookie = await POST(
      new Request(route, {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          cookie: "synthetic=session",
          "content-type": "application/json",
        },
        body: JSON.stringify(payload),
      }),
      {
        params: Promise.resolve({
          path: ["verifications", "identity-declaration"],
        }),
      },
    );
    assert.equal(cookie.status, 400, await cookie.clone().text());
    assert.equal(cookie.headers.get("cache-control"), "private, no-store");
    assert.equal(cookie.headers.get("set-cookie"), null);
    const cookieError = await cookie.json();
    assert.equal(cookieError.error.code, "cookies_not_supported");
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
