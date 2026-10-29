import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "espacigo-gallery-position-cas-",
  ),
);
let calls = [];
let responses = [];
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
    "const apiClient = globalThis.__galleryPositionCasApi;",
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
globalThis.__galleryPositionCasApi = {
  PUT: async (...args) => {
    calls.push(args);
    return responses.shift();
  },
};
const { OpenApiHostListingsAdapter } = await import(
  pathToFileURL(path.join(scratch, "adapter.mjs")).href
);
const adapter = new OpenApiHostListingsAdapter();
function response(status, etag) {
  return {
    response: {
      ok: status >= 200 && status < 300,
      status,
      headers: { get: (name) => (name === "ETag" ? etag : null) },
    },
    error: undefined,
  };
}
function setup(list) {
  calls = [];
  responses = list;
}
test.after(async () => rm(scratch, { recursive: true, force: true }));
test("adjacent gallery move sends one position CAS with renewed strong ETag", async () => {
  setup([response(200, '"rev-2"')]);
  assert.equal(
    await adapter.movePhoto("space-1", "photo-7", 2, '"rev-1"'),
    '"rev-2"',
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], "/spaces/{spaceId}/gallery/{photoId}");
  assert.deepEqual(calls[0][1].params.path, {
    spaceId: "space-1",
    photoId: "photo-7",
  });
  assert.deepEqual(calls[0][1].body, { position: 2 });
  assert.equal(calls[0][1].params.header["If-Match"], '"rev-1"');
});
test("stale gallery move propagates 412 without retry", async () => {
  setup([response(412, null), response(200, '"unexpected"')]);
  await assert.rejects(
    () => adapter.movePhoto("space-1", "photo-7", 2, '"stale"'),
    (error) => error.status === 412,
  );
  assert.equal(calls.length, 1);
});
