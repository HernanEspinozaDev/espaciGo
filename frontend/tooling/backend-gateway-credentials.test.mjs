import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-gateway-"),
);
const serverConfigPath = path.join(scratch, "server-config.mjs");
await writeFile(
  path.join(scratch, "cloud-run-identity.mjs"),
  `export async function getCloudRunIdentityHeadersForBackend() { return {}; }`,
);
const sessionSource = (
  await readFile(path.join(root, "src/lib/api/server-session.ts"), "utf8")
).replace('import "server-only";', "");
await writeFile(
  path.join(scratch, "server-session.mjs"),
  ts.transpileModule(sessionSource, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
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
await writeFile(path.join(scratch, "gateway-operations.mjs"), operationsModule);
await writeFile(
  serverConfigPath,
  `export function getApiBackendBaseUrl() {
    return new URL("http://127.0.0.1:18084/api/v1");
  }`,
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
  )
  .replace(
    'import { getCloudRunIdentityHeadersForBackend } from "@/lib/api/cloud-run-identity";',
    'import { getCloudRunIdentityHeadersForBackend } from "./cloud-run-identity.mjs";',
  )
  .replace('from "@/lib/api/server-session";', 'from "./server-session.mjs";')
  .replace('import "server-only";', "");
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const routePath = path.join(scratch, "route.mjs");
await writeFile(routePath, compiled);
const { GET, POST } = await import(pathToFileURL(routePath));
const originalFetch = globalThis.fetch;
const sessionCookie = `__Host-espacigo-session=synthetic-opaque-token; __Host-espacigo-session-expiry=${new Date(Date.now() + 3600000).toISOString()}`;
function requestWithSession(url, init = {}) {
  const headers = new Headers(init.headers);
  headers.set("cookie", sessionCookie);
  headers.set("host", "127.0.0.1:3000");
  headers.set("origin", "https://127.0.0.1:3000");
  return new Request(url, { ...init, headers });
}
test.after(async () => {
  globalThis.fetch = originalFetch;
  await rm(scratch, { recursive: true, force: true });
});

test("all six document operations forward Bearer without browser credentials", async () => {
  const caseID = "11111111-1111-4111-8111-111111111111";
  const fileID = "22222222-2222-4222-8222-222222222222";
  const pdfBytes = new Uint8Array([37, 80, 68, 70, 45, 49, 46, 55]);
  const scenarios = [
    {
      method: "GET",
      path: ["document-verification", "cases"],
      contentType: "application/json",
    },
    {
      method: "POST",
      path: ["document-verification", "cases"],
      contentType: "application/json",
      requestBody: JSON.stringify({ purpose: "identity" }),
      requestContentType: "application/json",
    },
    {
      method: "GET",
      path: ["document-verification", "cases", caseID],
      contentType: "application/json",
    },
    {
      method: "POST",
      path: ["document-verification", "cases", caseID, "submit"],
      contentType: "application/json",
      ifMatch: '"8"',
    },
    {
      method: "POST",
      path: ["document-verification", "cases", caseID, "files"],
      contentType: "application/json",
      requestBody: (() => {
        const form = new FormData();
        form.set(
          "file",
          new File([pdfBytes], "synthetic.pdf", { type: "application/pdf" }),
        );
        return form;
      })(),
      ifMatch: '"8"',
    },
    {
      method: "GET",
      path: [
        "document-verification",
        "space-cases",
        caseID,
        "files",
        fileID,
        "content",
      ],
      contentType: "application/pdf",
      responseBody: pdfBytes,
    },
  ];

  for (const scenario of scenarios) {
    let upstreamURL;
    let upstreamOptions;
    globalThis.fetch = async (url, options) => {
      upstreamURL = url;
      upstreamOptions = options;
      return new Response(scenario.responseBody ?? "{}", {
        status: 200,
        headers: { "content-type": scenario.contentType },
      });
    };

    const headers = new Headers();
    if (scenario.ifMatch) headers.set("if-match", scenario.ifMatch);
    if (scenario.requestContentType) {
      headers.set("content-type", scenario.requestContentType);
    }
    const init = { method: scenario.method, headers };
    if (scenario.requestBody !== undefined) init.body = scenario.requestBody;
    const request = requestWithSession(
      `http://127.0.0.1:3000/api/backend/${scenario.path.join("/")}`,
      init,
    );
    const handler = scenario.method === "GET" ? GET : POST;
    const response = await handler(request, {
      params: Promise.resolve({ path: scenario.path }),
    });

    assert.equal(response.status, 200, scenario.path.join("/"));
    assert.equal(upstreamOptions.credentials, "omit");
    assert.equal(
      upstreamOptions.headers.get("authorization"),
      "Bearer synthetic-opaque-token",
    );
    assert.equal(upstreamOptions.headers.has("cookie"), false);
    assert.equal(upstreamOptions.cache, "no-store");
    assert.equal(upstreamOptions.redirect, "manual");
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.equal(
      new URL(upstreamURL).pathname,
      `/api/v1/${scenario.path.join("/")}`,
    );
    if (scenario.ifMatch) {
      assert.equal(upstreamOptions.headers.get("if-match"), scenario.ifMatch);
    }
    if (scenario.method === "POST" && scenario.path.at(-1) === "files") {
      assert.match(
        upstreamOptions.headers.get("content-type"),
        /^multipart\/form-data; boundary=/,
      );
      assert.ok(upstreamOptions.body instanceof Uint8Array);
    }
    if (scenario.responseBody === pdfBytes) {
      assert.deepEqual(new Uint8Array(await response.arrayBuffer()), pdfBytes);
    }
  }
});

test("BFF preserves upstream auth and CAS statuses without exposing text errors", async () => {
  const caseID = "11111111-1111-4111-8111-111111111111";
  const scenarios = [
    { method: "GET", path: ["document-verification", "cases"], status: 401 },
    {
      method: "POST",
      path: ["document-verification", "cases", caseID, "submit"],
      status: 409,
      ifMatch: '"1"',
    },
    {
      method: "POST",
      path: ["document-verification", "cases", caseID, "submit"],
      status: 428,
    },
  ];

  for (const scenario of scenarios) {
    globalThis.fetch = async () =>
      new Response("private backend detail that must not escape", {
        status: scenario.status,
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    const headers = new Headers();
    if (scenario.ifMatch) headers.set("if-match", scenario.ifMatch);
    const handler = scenario.method === "GET" ? GET : POST;
    const response = await handler(
      requestWithSession(
        `http://127.0.0.1:3000/api/backend/${scenario.path.join("/")}`,
        { method: scenario.method, headers },
      ),
      { params: Promise.resolve({ path: scenario.path }) },
    );

    assert.equal(response.status, scenario.status);
    assert.match(
      response.headers.get("content-type") ?? "",
      /application\/json/,
    );
    const body = await response.text();
    assert.match(body, /"code":"upstream_error"/);
    assert.doesNotMatch(body, /private backend detail/);
  }
});

test("BFF sanitizes disallowed-media errors at status boundaries and cancels bodies", async () => {
  const caseID = "11111111-1111-4111-8111-111111111111";
  const scenarios = [
    { status: 399, contentType: "text/plain", wantStatus: 502 },
    { status: 400, contentType: "text/plain", wantStatus: 400 },
    { status: 499, contentType: "text/plain", wantStatus: 499 },
    { status: 500, contentType: "text/plain", wantStatus: 500 },
    { status: 599, contentType: "text/plain", wantStatus: 599 },
    { status: 600, contentType: "text/plain", wantStatus: 502 },
    { status: 401, contentType: null, wantStatus: 401 },
    { status: 500, contentType: "not a media type", wantStatus: 500 },
  ];

  for (const scenario of scenarios) {
    let cancelCalls = 0;
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(new TextEncoder().encode("private backend detail"));
      },
      cancel() {
        cancelCalls += 1;
        throw new Error("synthetic cancellation failure");
      },
    });
    const headers = new Headers();
    if (scenario.contentType) {
      headers.set("content-type", scenario.contentType);
    }
    const upstream =
      scenario.status === 600
        ? { status: scenario.status, headers, body }
        : new Response(body, { status: scenario.status, headers });
    globalThis.fetch = async () => upstream;

    const response = await GET(
      requestWithSession(
        `http://127.0.0.1:3000/api/backend/document-verification/cases/${caseID}`,
      ),
      {
        params: Promise.resolve({
          path: ["document-verification", "cases", caseID],
        }),
      },
    );

    assert.equal(response.status, scenario.wantStatus);
    assert.equal(cancelCalls, 1, `status ${scenario.status}`);
    if (scenario.wantStatus === 502) {
      assert.match(await response.text(), /upstream_content_type_not_allowed/);
    } else {
      const responseBody = await response.text();
      assert.match(responseBody, /upstream_error/);
      assert.doesNotMatch(responseBody, /private backend detail/);
    }
  }
});

test("BFF passes contract JSON errors through with their original status", async () => {
  const caseID = "11111111-1111-4111-8111-111111111111";
  const payload = {
    error: { code: "conflict", message: "Conflicto de estado." },
  };
  globalThis.fetch = async () =>
    new Response(JSON.stringify(payload), {
      status: 409,
      headers: { "content-type": "application/json; charset=utf-8" },
    });

  const response = await GET(
    requestWithSession(
      `http://127.0.0.1:3000/api/backend/document-verification/cases/${caseID}`,
    ),
    {
      params: Promise.resolve({
        path: ["document-verification", "cases", caseID],
      }),
    },
  );

  assert.equal(response.status, 409);
  assert.deepEqual(await response.json(), payload);
});
