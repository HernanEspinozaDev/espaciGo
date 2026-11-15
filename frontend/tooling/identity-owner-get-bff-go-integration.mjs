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

test("Next BFF gets own identity case through registered Go route", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-owner-get-bff-go-"),
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
    const compiled = ts.transpileModule(source, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText;
    const routePath = path.join(scratch, "route.mjs");
    await writeFile(routePath, compiled);
    const { GET } = await import(pathToFileURL(routePath));
    const id = "99999999-9999-4999-8999-999999999999";
    const segments = ["document-verification", "cases", id];
    const response = await GET(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        method: "GET",
        headers: { authorization: "Bearer synthetic-session" },
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    assert.equal(response.status, 200, await response.clone().text());
    assert.equal(response.headers.get("etag"), '"8"');
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
      "reviewer_id",
      "audit",
      "storage_key",
      "document_expires_at",
      "88888888-8888-4888-8888-888888888888",
      "synthetic-storage",
    ])
      assert.equal(text.includes(forbidden), false, `leaked ${forbidden}`);
    const payload = JSON.parse(text);
    assert.deepEqual(Object.keys(payload).sort(), [
      "document_format_declared",
      "files",
      "id",
      "owner_decision_message",
      "purpose",
      "revision",
      "state",
    ]);
    assert.equal(payload.id, id);
    assert.equal(payload.purpose, "identity");
    assert.equal(payload.state, "requiere_antecedentes");
    assert.equal(payload.revision, 8);
    assert.equal(payload.document_format_declared, "new");
    assert.equal(
      payload.owner_decision_message,
      "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente.",
    );
    assert.equal(payload.files.length, 3);
    assert.deepEqual(payload.files.map((f) => f.evidence_role).sort(), [
      "identity_back",
      "identity_front",
      "identity_selfie",
    ]);
    for (const file of payload.files)
      assert.deepEqual(Object.keys(file).sort(), [
        "evidence_role",
        "id",
        "mime",
        "pages",
        "size",
        "state",
      ]);
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
