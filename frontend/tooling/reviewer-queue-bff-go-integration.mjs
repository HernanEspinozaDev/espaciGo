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

test("exported Next GET forwards reviewer queue to registered Go handler", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-reviewer-queue-bff-go-"),
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
    const { GET } = await import(pathToFileURL(routePath));
    const segments = ["document-verification", "reviewer", "cases"];
    const response = await GET(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        headers: { authorization: "Bearer synthetic-reviewer-session" },
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    assert.equal(response.status, 200);
    assert.match(
      response.headers.get("content-type") ?? "",
      /^application\/json(?:;|$)/,
    );
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.equal(response.headers.get("set-cookie"), null);
    const text = await response.text();
    const payload = JSON.parse(text);
    assert.deepEqual(Object.keys(payload), ["items"]);
    assert.equal(payload.items.length, 1);
    const item = payload.items[0];
    assert.deepEqual(Object.keys(item).sort(), [
      "document_format_declared",
      "files",
      "id",
      "purpose",
      "revision",
      "state",
    ]);
    assert.deepEqual(item, {
      id: "99999999-9999-4999-8999-999999999999",
      purpose: "identity",
      state: "enviada",
      revision: 3,
      document_format_declared: "new",
      files: [],
    });
    for (const forbidden of [
      "subject_id",
      "identity_declaration",
      "declaration_revision",
      "consent",
      "storage_key",
      "storageKey",
      "legal_name",
      "rut",
    ])
      assert.equal(
        text.toLowerCase().includes(forbidden),
        false,
        `response leaked ${forbidden}`,
      );
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
