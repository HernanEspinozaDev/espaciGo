import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const adapterPath =
  "src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts";
const id = "550e8400-e29b-41d4-a716-446655440000";
const etag = '\"synthetic-strong-revision\"';
const optionalKeys = ["region_code", "comuna_code", "attribute_schema_version"];
const base = {
  category_code: "oficina",
  title: "Synthetic",
  description: "Synthetic",
  area_m2: 20,
  capacity: 2,
  usage_rules: "Synthetic",
  rate_unit: "hora",
  base_price_clp: 5001,
  address: "Synthetic",
  attributes: { enabled: false, count: 0 },
};

async function loadRealAdapter(adapterSource, scratch) {
  await writeFile(
    path.join(scratch, "api-client.mjs"),
    `export const apiClient = {
      POST: async (_path, options) => {
        globalThis.__draftOptions = options;
        globalThis.__draftWire = options.bodySerializer(options.body);
        return { response: { ok: true, status: 200, headers: new Headers({ ETag: '${etag}' }) },
          data: { id: '${id}', category_code: 'oficina', category_name: 'Oficina', title: 'Synthetic', description: 'Synthetic', area_m2: 20, capacity: 2, usage_rules: 'Synthetic', rate_unit: 'hora', base_price_clp: 5001, address: 'Synthetic', state: 'borrador', region_code: '13', comuna_code: '13101', attribute_schema_version: 1, attributes: { enabled: false, count: 0 } } };
      },
      PUT: async (_path, options) => {
        globalThis.__draftOptions = options;
        globalThis.__draftWire = options.bodySerializer(options.body);
        return { response: { ok: true, status: 200, headers: new Headers({ ETag: '${etag}' }) },
          data: { id: '${id}', category_code: 'oficina', category_name: 'Oficina', title: 'Synthetic', description: 'Synthetic', area_m2: 20, capacity: 2, usage_rules: 'Synthetic', rate_unit: 'hora', base_price_clp: 5001, address: 'Synthetic', state: 'borrador', region_code: '13', comuna_code: '13101', attribute_schema_version: 1, attributes: { enabled: false, count: 0 } } };
      }
    };`,
  );
  await writeFile(
    path.join(scratch, "errors.mjs"),
    `export class ApiProtocolError extends Error {}\nexport function normalizeApiError() { return new Error("api error"); }`,
  );
  await writeFile(
    path.join(scratch, "validation.mjs"),
    `export function convertSlotsToWeeklyDays() { return []; }\nexport function generateIdempotencyKey() { return "generated-key"; }`,
  );
  const exactJson = await readFile(
    path.join(root, "src/lib/api/exact-json.ts"),
    "utf8",
  );
  await writeFile(
    path.join(scratch, "exact-json.mjs"),
    ts.transpileModule(exactJson, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText,
  );
  const source = adapterSource
    .replace('import "client-only";', "")
    .replace('"@/lib/api/client"', '"./api-client.mjs"')
    .replace('"@/lib/api/errors"', '"./errors.mjs"')
    .replace('"@/lib/api/exact-json"', '"./exact-json.mjs"')
    .replace('"../domain/validation"', '"./validation.mjs"');
  const modulePath = path.join(scratch, "adapter.mjs");
  await writeFile(
    modulePath,
    ts.transpileModule(source, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText,
  );
  const { OpenApiHostListingsAdapter } = await import(
    pathToFileURL(modulePath)
  );
  return new OpenApiHostListingsAdapter();
}

async function withAdapter(source, run) {
  const scratch = await mkdtemp(
    path.join(process.env.TMPDIR ?? os.tmpdir(), "draft-exact-json-"),
  );
  try {
    await run(await loadRealAdapter(source, scratch));
  } finally {
    delete globalThis.__draftOptions;
    delete globalThis.__draftWire;
    await rm(scratch, { recursive: true, force: true });
  }
}

async function readAdapterSource() {
  if (process.env.BASELINE_DRAFT_ADAPTER_SOURCE) {
    return readFile(process.env.BASELINE_DRAFT_ADAPTER_SOURCE, "utf8");
  }
  return readFile(path.join(root, adapterPath), "utf8");
}

test("createDraft and updateDraft serialize exact optional fields and preserve values and ETags", async () => {
  await withAdapter(await readAdapterSource(), async (adapter) => {
    const absent = {
      ...base,
      region_code: undefined,
      comuna_code: undefined,
      attribute_schema_version: undefined,
    };
    await adapter.createDraft(absent);
    let body = JSON.parse(globalThis.__draftWire);
    for (const key of optionalKeys)
      assert.equal(
        Object.hasOwn(body, key),
        false,
        `createDraft omitted ${key}`,
      );
    assert.deepEqual(body.attributes, { enabled: false, count: 0 });
    assert.equal(body.base_price_clp, 5001);
    assert.equal(
      (
        await adapter.createDraft({
          ...base,
          region_code: "13",
          comuna_code: "13101",
          attribute_schema_version: 1,
        })
      ).etag,
      etag,
    );
    body = JSON.parse(globalThis.__draftWire);
    assert.deepEqual(
      Object.fromEntries(optionalKeys.map((key) => [key, body[key]])),
      {
        region_code: "13",
        comuna_code: "13101",
        attribute_schema_version: 1,
      },
    );

    const updated = await adapter.updateDraft(id, absent, etag);
    assert.equal(globalThis.__draftOptions.params.header["If-Match"], etag);
    body = JSON.parse(globalThis.__draftWire);
    for (const key of optionalKeys)
      assert.equal(
        Object.hasOwn(body, key),
        false,
        `updateDraft omitted ${key}`,
      );
    assert.deepEqual(body.attributes, { enabled: false, count: 0 });
    assert.equal(body.base_price_clp, 5001);
    assert.equal(updated.etag, etag);
    await adapter.updateDraft(
      id,
      {
        ...base,
        region_code: "13",
        comuna_code: "13101",
        attribute_schema_version: 1,
      },
      etag,
    );
    body = JSON.parse(globalThis.__draftWire);
    assert.deepEqual(
      Object.fromEntries(optionalKeys.map((key) => [key, body[key]])),
      {
        region_code: "13",
        comuna_code: "13101",
        attribute_schema_version: 1,
      },
    );
    assert.equal(globalThis.__draftOptions.params.header["If-Match"], etag);
  });
});

test("baseline rejection is reproducible for createDraft and updateDraft", async (t) => {
  if (!process.env.BASELINE_DRAFT_ADAPTER_SOURCE) {
    t.skip(
      "set BASELINE_DRAFT_ADAPTER_SOURCE to a baseline adapter source file",
    );
    return;
  }
  await withAdapter(await readAdapterSource(), async (adapter) => {
    const absent = {
      ...base,
      region_code: undefined,
      comuna_code: undefined,
      attribute_schema_version: undefined,
    };
    await assert.rejects(adapter.createDraft(absent), TypeError);
    await assert.rejects(adapter.updateDraft(id, absent, etag), TypeError);
  });
});
