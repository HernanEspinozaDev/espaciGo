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
const backendBaseURL = new URL(process.env.ESPACIGO_TEST_API_BASE_URL ?? "");
assert.equal(backendBaseURL.protocol, "http:");
assert.equal(backendBaseURL.hostname, "127.0.0.1");
assert.notEqual(backendBaseURL.port, "");
assert.equal(backendBaseURL.pathname, "/api/v1");
assert.equal(backendBaseURL.username, "");
assert.equal(backendBaseURL.password, "");
assert.equal(backendBaseURL.search, "");
assert.equal(backendBaseURL.hash, "");

test("Next BFF creates owner identity case through registered Go route", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-owner-create-bff-go-"),
  );
  try {
    const operationsSource = await readFile(
      path.join(root, "src/lib/api/generated/gateway-operations.ts"),
      "utf8",
    );
    await writeFile(
      path.join(scratch, "gateway-operations.mjs"),
      ts.transpileModule(operationsSource, {
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
    const source = (
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
      ts.transpileModule(source, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
        },
      }).outputText,
    );
    const { POST } = await import(pathToFileURL(routePath));
    const body = {
      purpose: "identity",
      document_format_declared: "new",
      capture_consent_given: true,
      capture_consent_version: "identity-capture-local-v1",
    };
    const segments = ["document-verification", "cases"];
    const response = await POST(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          "content-type": "application/json",
        },
        body: JSON.stringify(body),
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    assert.equal(response.status, 201, await response.clone().text());
    assert.equal(response.headers.get("etag"), '"1"');
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.match(
      response.headers.get("vary") ?? "",
      /(?:^|,\s*)Authorization(?:\s*,|$)/i,
    );
    assert.equal(response.headers.get("set-cookie"), null);
    const text = await response.text();
    for (const forbidden of [
      "subject_id",
      "declaration_revision",
      "reviewer",
      "storage_key",
      "document_expires_at",
      "88888888-8888-4888-8888-888888888888",
      "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    ])
      assert.equal(text.includes(forbidden), false, `leaked ${forbidden}`);
    const payload = JSON.parse(text);
    assert.deepEqual(Object.keys(payload).sort(), [
      "document_format_declared",
      "files",
      "id",
      "purpose",
      "revision",
      "state",
    ]);
    assert.match(
      payload.id,
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    assert.equal(payload.purpose, "identity");
    assert.equal(payload.state, "borrador");
    assert.equal(payload.revision, 1);
    assert.equal(payload.document_format_declared, "new");
    assert.deepEqual(payload.files, []);
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
