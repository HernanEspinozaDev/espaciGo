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

test("exported Next GET gets reviewer detail from registered Go handler", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-reviewer-detail-bff-go-"),
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
    const segments = [
      "document-verification",
      "reviewer",
      "cases",
      "99999999-9999-4999-8999-999999999999",
    ];
    const response = await GET(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        headers: { authorization: "Bearer synthetic-reviewer-session" },
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("etag"), '"5"');
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.equal(response.headers.get("set-cookie"), null);
    const text = await response.text();
    const dto = JSON.parse(text);
    assert.deepEqual(
      Object.keys(dto).sort(),
      [
        "document_format_declared",
        "files",
        "id",
        "identity_declaration",
        "purpose",
        "revision",
        "state",
      ].sort(),
    );
    assert.equal(dto.id, segments[3]);
    assert.equal(dto.purpose, "identity");
    assert.equal(dto.state, "en_revision");
    assert.equal(dto.revision, 5);
    assert.deepEqual(dto.identity_declaration, {
      first_name: "SyntheticFirst",
      last_name: "SyntheticLast",
      rut: "11.111.111-1",
      date_of_birth: "1980-01-02",
    });
    assert.deepEqual(Object.keys(dto.identity_declaration).sort(), [
      "date_of_birth",
      "first_name",
      "last_name",
      "rut",
    ]);
    assert.deepEqual(dto.files, []);
    for (const forbidden of [
      "11111111-1111-4111-8111-111111111111",
      "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
      "identity_declaration_revision_id",
      "storage_key",
      "object_key",
      "signed_url",
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
