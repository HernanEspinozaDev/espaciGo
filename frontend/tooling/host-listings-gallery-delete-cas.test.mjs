import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-gallery-delete-cas-"),
);
let calls = [];
let result;
const source = await readFile(
  path.join(
    root,
    "src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts",
  ),
  "utf8",
);
const transformed = source
  .replace('import "client-only";', "")
  .replace(
    'import { apiClient } from "@/lib/api/client";',
    "const apiClient = globalThis.__galleryDeleteCasApi;",
  )
  .replace(
    'import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";',
    'const ApiProtocolError = class extends Error {}; const normalizeApiError = (response) => Object.assign(new Error("api error"), { status: response.status });',
  )
  .replace(
    'import { serializeExactJson } from "@/lib/api/exact-json";',
    "const serializeExactJson = JSON.stringify;",
  )
  .replace(
    /import type \{[\s\S]*?\} from "\.\.\/application\/host-listings\.port";/,
    "",
  )
  .replace(/import type \{[\s\S]*?\} from "\.\.\/domain\/types";/, "")
  .replace(/import \{[\s\S]*?\} from "\.\.\/domain\/validation";/, "");
await writeFile(
  path.join(scratch, "adapter.mjs"),
  ts.transpileModule(transformed, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
globalThis.__galleryDeleteCasApi = {
  DELETE: async (...args) => {
    calls.push(args);
    return result;
  },
};
const { OpenApiHostListingsAdapter } = await import(
  pathToFileURL(path.join(scratch, "adapter.mjs")).href
);
const adapter = new OpenApiHostListingsAdapter();
function setResponse(status = 204, etag = '"rev-2"') {
  calls = [];
  result = {
    response: {
      ok: status >= 200 && status < 300,
      status,
      headers: { get: (name) => (name === "ETag" ? etag : null) },
    },
    error: undefined,
  };
}
test.after(async () => rm(scratch, { recursive: true, force: true }));
test("gallery DELETE sends strong If-Match and returns the renewed strong ETag", async () => {
  setResponse();
  assert.equal(
    await adapter.removePhoto("space-1", "photo-1", '"rev-1"'),
    '"rev-2"',
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0][1].params.header["If-Match"], '"rev-1"');
});
test("gallery DELETE rejects malformed input before request", async () => {
  for (const etag of [undefined, "", 'W/"weak"', "rev-1", '"a", "b"']) {
    setResponse();
    await assert.rejects(() => adapter.removePhoto("space-1", "photo-1", etag));
    assert.equal(calls.length, 0);
  }
});
test("gallery DELETE requires renewed strong ETag and never retries stale 412", async () => {
  for (const etag of [null, 'W/"weak"', "malformed"]) {
    setResponse(204, etag);
    await assert.rejects(() =>
      adapter.removePhoto("space-1", "photo-1", '"rev-1"'),
    );
    assert.equal(calls.length, 1);
  }
  setResponse(412, null);
  await assert.rejects(
    () => adapter.removePhoto("space-1", "photo-1", '"rev-1"'),
    (error) => error.status === 412,
  );
  assert.equal(calls.length, 1);
});
