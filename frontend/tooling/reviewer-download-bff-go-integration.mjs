import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratchRoot = process.env.TMPDIR;
if (!scratchRoot) throw new Error("TMPDIR must be isolated scratch storage");
const expectedBase64 = process.env.EXPECTED_REVIEWER_DOWNLOAD_BASE64;
if (!expectedBase64)
  throw new Error("expected synthetic bytes were not configured");
const backendBaseURL = new URL(process.env.ESPACIGO_TEST_API_BASE_URL ?? "");
assert.equal(backendBaseURL.protocol, "http:");
assert.equal(backendBaseURL.hostname, "127.0.0.1");
assert.notEqual(backendBaseURL.port, "");
assert.equal(backendBaseURL.pathname, "/api/v1");
assert.equal(backendBaseURL.username, "");
assert.equal(backendBaseURL.password, "");
assert.equal(backendBaseURL.search, "");
assert.equal(backendBaseURL.hash, "");

test("exported Next GET downloads identity evidence from registered Go handler", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-reviewer-download-bff-go-"),
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
      "files",
      "22222222-2222-4222-8222-222222222222",
      "content",
    ];
    const response = await GET(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        headers: { authorization: "Bearer synthetic-reviewer-session" },
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("content-type"), "image/png");
    assert.equal(
      response.headers.get("content-disposition"),
      'attachment; filename="space-document.png"',
    );
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.equal(response.headers.get("set-cookie"), null);
    const body = Buffer.from(await response.arrayBuffer());
    assert.deepEqual(body, Buffer.from(expectedBase64, "base64"));
    const text = new TextDecoder().decode(body);
    for (const forbidden of [
      "storage_key",
      "object_key",
      "envelope",
      "signed_url",
      "11111111-1111-4111-8111-111111111111",
    ])
      assert.equal(text.toLowerCase().includes(forbidden), false);
    const cookieResponse = await GET(
      new Request(`http://127.0.0.1:3000/api/backend/${segments.join("/")}`, {
        headers: {
          authorization: "Bearer synthetic-reviewer-session",
          cookie: "synthetic-browser-session=not-forwardable",
        },
      }),
      { params: Promise.resolve({ path: segments }) },
    );
    assert.equal(cookieResponse.status, 400);
    const cookieError = await cookieResponse.json();
    assert.equal(cookieError.error?.code, "cookies_not_supported");
    assert.equal(
      cookieError.error?.message,
      "La solicitud no pudo completarse.",
    );
    assert.equal(
      cookieResponse.headers.get("cache-control"),
      "private, no-store",
    );
    assert.equal(cookieResponse.headers.get("set-cookie"), null);
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
