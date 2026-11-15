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
const rawBackendBaseURL = process.env.ESPACIGO_TEST_API_BASE_URL;
if (!rawBackendBaseURL)
  throw new Error(
    "ESPACIGO_TEST_API_BASE_URL must point to the Go httptest server",
  );
const backendBaseURL = new URL(rawBackendBaseURL);
assert.equal(backendBaseURL.protocol, "http:");
assert.equal(backendBaseURL.hostname, "127.0.0.1");
assert.notEqual(backendBaseURL.port, "");
assert.equal(backendBaseURL.pathname, "/api/v1");
assert.equal(backendBaseURL.username, "");
assert.equal(backendBaseURL.password, "");
assert.equal(backendBaseURL.search, "");
assert.equal(backendBaseURL.hash, "");

test("Next BFF submits owner identity case through actual Go handler", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-owner-submit-bff-go-"),
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
    const { POST } = await import(pathToFileURL(routePath));
    const caseID = "99999999-9999-4999-8999-999999999999";
    const segments = ["document-verification", "cases", caseID, "submit"];
    const response = await POST(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          "if-match": '"7"',
        },
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    if (process.env.ESPACIGO_OWNER_SUBMIT_SCENARIO === "stale") {
      assert.equal(response.status, 409, await response.clone().text());
      assert.equal(response.headers.get("cache-control"), "private, no-store");
      assert.equal(response.headers.get("etag"), null);
      assert.equal(response.headers.get("set-cookie"), null);
      const text = await response.text();
      assert.equal(text.includes("revision"), false);
      assert.equal(text.includes("conflict"), false);
      const payload = JSON.parse(text);
      assert.deepEqual(Object.keys(payload), ["error"]);
      assert.deepEqual(Object.keys(payload.error).sort(), [
        "code",
        "message",
        "request_id",
      ]);
      assert.equal(payload.error.code, "upstream_error");
      assert.equal(payload.error.message, "La solicitud no pudo completarse.");
      assert.match(payload.error.request_id, /^[0-9a-f-]{36}$/i);
      assert.equal(
        response.headers.get("x-request-id"),
        payload.error.request_id,
      );
      return;
    }
    assert.ok(
      ["success", "retry"].includes(
        process.env.ESPACIGO_OWNER_SUBMIT_SCENARIO ?? "success",
      ),
    );
    assert.equal(response.status, 200, await response.clone().text());
    assert.equal(response.headers.get("etag"), '"8"');
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.match(
      response.headers.get("content-type") ?? "",
      /^application\/json(?:;|$)/,
    );
    assert.equal(response.headers.get("set-cookie"), null);
    const text = await response.text();
    assert.equal(text.includes("88888888-8888-4888-8888-888888888888"), false);
    assert.equal(text.includes("subject_id"), false);
    assert.equal(text.includes("storage_key"), false);
    const payload = JSON.parse(text);
    assert.deepEqual(Object.keys(payload).sort(), [
      "document_format_declared",
      "files",
      "id",
      "purpose",
      "revision",
      "state",
    ]);
    assert.equal(payload.id, caseID);
    assert.equal(payload.purpose, "identity");
    assert.equal(payload.state, "enviada");
    assert.equal(payload.revision, 8);
    assert.equal(payload.document_format_declared, "new");
    const roles = payload.files.map((file) => file.evidence_role);
    assert.equal(payload.files.length, 3);
    assert.equal(new Set(roles).size, roles.length);
    assert.deepEqual(roles.sort(), [
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
