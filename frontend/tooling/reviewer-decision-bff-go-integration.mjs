import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratchRoot = process.env.TMPDIR;
if (!scratchRoot) {
  throw new Error("TMPDIR must point to isolated test scratch storage");
}
const rawBackendBaseURL = process.env.ESPACIGO_TEST_API_BASE_URL;
if (!rawBackendBaseURL) {
  throw new Error(
    "ESPACIGO_TEST_API_BASE_URL must point to the Go httptest server",
  );
}
const backendBaseURL = new URL(rawBackendBaseURL);
assert.equal(backendBaseURL.protocol, "http:");
assert.equal(backendBaseURL.hostname, "127.0.0.1");
assert.notEqual(backendBaseURL.port, "");
assert.equal(backendBaseURL.pathname, "/api/v1");
assert.equal(backendBaseURL.username, "");
assert.equal(backendBaseURL.password, "");
assert.equal(backendBaseURL.search, "");
assert.equal(backendBaseURL.hash, "");

test("Next BFF forwards reviewer identity decision to the real Go handler", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-reviewer-bff-go-"),
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
    const pathSegments = [
      "document-verification",
      "reviewer",
      "cases",
      caseID,
      "review",
    ];
    const body =
      '{"decision":"needs_evidence","reason_code":"antecedentes_incompletos","expected_revision":7,"document_format_observed":"new","identity_review_checklist":{"names_match":false,"rut_matches":true,"birth_date_and_majority_match":true,"portrait_matches_selfie":true,"evidence_legible":true,"document_unexpired":true,"issue_date_and_format_verified":true}}';
    const request = new Request(
      `http://127.0.0.1:3000/api/backend/${pathSegments.join("/")}`,
      {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          "content-type": "application/json",
        },
        body,
      },
    );
    const response = await POST(request, {
      params: Promise.resolve({ path: pathSegments }),
    });

    assert.equal(response.status, 200);
    assert.equal(response.headers.get("etag"), '"8"');
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.match(
      response.headers.get("content-type") ?? "",
      /^application\/json(?:;|$)/,
    );
    assert.equal(response.headers.get("set-cookie"), null);
    const responseText = await response.text();
    assert.equal(
      responseText.includes("88888888-8888-4888-8888-888888888888"),
      false,
    );
    assert.equal(responseText.includes("subject_id"), false);
    assert.equal(responseText.includes("storage_key"), false);
    const payload = JSON.parse(responseText);
    assert.deepEqual(Object.keys(payload).sort(), [
      "document_format_declared",
      "document_format_observed",
      "files",
      "id",
      "purpose",
      "revision",
      "state",
    ]);
    assert.equal(payload.id, caseID);
    assert.equal(payload.purpose, "identity");
    assert.equal(payload.state, "requiere_antecedentes");
    assert.equal(payload.revision, 8);
    assert.equal(payload.document_format_observed, "new");
    assert.equal(payload.files.length, 3);
    for (const file of payload.files) {
      assert.deepEqual(Object.keys(file).sort(), [
        "evidence_role",
        "id",
        "mime",
        "pages",
        "size",
        "state",
      ]);
    }
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
