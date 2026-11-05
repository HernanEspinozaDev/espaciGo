import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-publication-cas-"),
);
let calls = [];
let result;
let thrown;
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
    "const apiClient = globalThis.__casApi;",
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
globalThis.__casApi = {
  PUT: async (...args) => {
    calls.push(args);
    if (thrown) throw thrown;
    return result;
  },
};
const { OpenApiHostListingsAdapter } = await import(
  pathToFileURL(path.join(scratch, "adapter.mjs")).href
);
const adapter = new OpenApiHostListingsAdapter();
const input = {
  title: "T",
  description: "D",
  capacity: 1,
  usage_rules: "R",
  base_price_clp: 6000,
  region_code: "R",
  comuna_code: "C",
};
const detail = {
  id: "space-1",
  category_code: "c",
  category_name: "C",
  title: "T",
  description: "D",
  area_m2: 5,
  capacity: 1,
  usage_rules: "R",
  rate_unit: "day",
  base_price_clp: 6000,
  address: "A",
  state: "activa",
  region_code: "R",
  comuna_code: "C",
  attribute_schema_version: 1,
  attributes: {},
};
function response(status = 200, etag = '"rev-2"') {
  return {
    response: {
      ok: status >= 200 && status < 300,
      status,
      headers: { get: (name) => (name === "ETag" ? etag : null) },
    },
    data: detail,
  };
}
function reset(next = response()) {
  calls = [];
  result = next;
  thrown = null;
}
function invoke(method, revision) {
  reset(response());
  return method === "updateContent"
    ? adapter[method]("space-1", input, revision)
    : adapter[method]("space-1", revision);
}
test.after(async () => rm(scratch, { recursive: true, force: true }));
test("tariff update sends the space If-Match and returns the renewed strong ETag", async () => {
  reset(response(200, '"rev-2"'));
  const mapped = await adapter.updatePricing(
    "space-1",
    { base_price_clp: 8000, rate_unit: "dia" },
    '"rev-1"',
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], "/spaces/{spaceId}/tariff");
  assert.equal(calls[0][1].params.header["If-Match"], '"rev-1"');
  assert.equal(mapped.etag, '"rev-2"');
});

test("tariff update rejects missing or malformed ETag before request", async () => {
  for (const revision of [undefined, "", 'W/"weak"', "rev-1", '"a", "b"']) {
    reset();
    await assert.rejects(() =>
      adapter.updatePricing(
        "space-1",
        { base_price_clp: 8000, rate_unit: "dia" },
        revision,
      ),
    );
    assert.equal(calls.length, 0);
  }
});

test("tariff update fails closed without response ETag and does not retry 412", async () => {
  reset(response(200, null));
  await assert.rejects(() =>
    adapter.updatePricing(
      "space-1",
      { base_price_clp: 8000, rate_unit: "dia" },
      '"rev-1"',
    ),
  );
  assert.equal(calls.length, 1);
  reset(response(412));
  await assert.rejects(
    () =>
      adapter.updatePricing(
        "space-1",
        { base_price_clp: 8000, rate_unit: "dia" },
        '"rev-1"',
      ),
    (error) => error.status === 412,
  );
  assert.equal(calls.length, 1);
});

test("content update sends If-Match and maps the refreshed strong ETag", async () => {
  reset();
  const mapped = await adapter.updateContent("space-1", input, '"rev-1"');
  assert.equal(calls.length, 1);
  assert.equal(calls[0][1].params.header["If-Match"], '"rev-1"');
  assert.equal(mapped.etag, '"rev-2"');
});
test("publication mutations reject missing and malformed revisions before request", async () => {
  for (const method of ["updateContent", "publishListing", "archiveListing"])
    for (const revision of [
      undefined,
      'W/"weak"',
      "rev-1",
      '"bad',
      '"a", "b"',
      '"bad tag"',
      '"bad\u007ftag"',
    ]) {
      reset();
      await assert.rejects(() => invoke(method, revision));
      assert.equal(calls.length, 0);
    }
});
test("publication mutations fail closed when success ETag is missing or invalid", async () => {
  for (const method of ["updateContent", "publishListing", "archiveListing"])
    for (const etag of [null, 'W/"weak"', "not-etag"]) {
      reset(response(200, etag));
      await assert.rejects(() =>
        method === "updateContent"
          ? adapter[method]("space-1", input, '"rev-1"')
          : adapter[method]("space-1", '"rev-1"'),
      );
      assert.equal(calls.length, 1);
    }
});
test("412 propagates without retry for content, publish, and archive", async () => {
  for (const method of ["updateContent", "publishListing", "archiveListing"]) {
    reset(response(412));
    await assert.rejects(
      () =>
        method === "updateContent"
          ? adapter[method]("space-1", input, '"rev-1"')
          : adapter[method]("space-1", '"rev-1"'),
      (error) => error.status === 412,
    );
    assert.equal(calls.length, 1);
  }
});
