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

test("Next BFF forwards identity multipart to the real Go owner upload handler", async () => {
  const scratch = await mkdtemp(
    path.join(scratchRoot, "espacigo-identity-bff-go-"),
  );
  try {
    const serverConfigPath = path.join(scratch, "server-config.mjs");
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
      serverConfigPath,
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

    const evidenceBytes = new Uint8Array([
      0x73, 0x79, 0x6e, 0x74, 0x68, 0x65, 0x74, 0x69, 0x63, 0x2d, 0x69, 0x64,
      0x65, 0x6e, 0x74, 0x69, 0x74, 0x79,
    ]);
    const form = new FormData();
    form.append(
      "file",
      new File([evidenceBytes], "identity-front-synthetic.png", {
        type: "image/png",
      }),
    );
    form.append("evidence_role", "identity_front");

    const caseID = "99999999-9999-4999-8999-999999999999";
    const pathSegments = ["document-verification", "cases", caseID, "files"];
    const request = new Request(
      `http://127.0.0.1:3000/api/backend/${pathSegments.join("/")}`,
      {
        method: "POST",
        headers: {
          authorization: "Bearer synthetic-session",
          "if-match": '"3"',
        },
        body: form,
      },
    );
    const response = await POST(request, {
      params: Promise.resolve({ path: pathSegments }),
    });

    assert.equal(response.status, 201);
    assert.equal(response.headers.get("etag"), '"4"');
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.match(
      response.headers.get("content-type") ?? "",
      /^application\/json(?:;|$)/,
    );
    assert.equal(response.headers.get("set-cookie"), null);
    const responseText = await response.text();
    assert.equal(responseText.includes("identity-front-synthetic.png"), false);
    assert.equal(responseText.includes("object_key"), false);
    const payload = JSON.parse(responseText);
    assert.equal(payload.revision, 4);
    assert.equal(payload.file.evidence_role, "identity_front");
    assert.equal(payload.file.mime, "image/png");
    assert.equal(payload.file.size, evidenceBytes.byteLength);
  } finally {
    await rm(scratch, { recursive: true, force: true });
  }
});
