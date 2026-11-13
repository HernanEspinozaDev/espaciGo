import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const source = (file) => readFile(path.join(root, file), "utf8");

test("category operations use tokenless facade, port, and adapter with cookie BFF", async () => {
  const port = await source(
    "src/features/host-listings/application/host-listings.port.ts",
  );
  const facade = await source(
    "src/features/host-listings/application/host-listings-facade.ts",
  );
  const hook = await source(
    "src/features/host-listings/ui/use-host-listings.ts",
  );
  const adapter = await source(
    "src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts",
  );
  for (const text of [port, facade, adapter]) {
    assert.match(
      text,
      /(?:listCategories|loadCategories)\(\s*signal\?: AbortSignal/,
    );
    assert.match(text, /(?:getCategoryProfile|loadCategorySchema)\(/);
    assert.doesNotMatch(text, /AccessToken|token:|authHeader\(/);
  }
  assert.match(adapter, /GET\("\/spaces\/categories",\s*\{/);
  assert.match(
    await source("src/lib/api/client.ts"),
    /credentials:\s*"same-origin"/,
  );
  assert.match(facade, /async loadCategories\(/);
  assert.match(facade, /async loadCategorySchema\(/);
  assert.match(hook, /hostListingsFeatureBridge\.loadCategories\(token/);
  assert.match(hook, /hostListingsFeatureBridge\.loadCategorySchema\(\s*token/);
  assert.match(hook, /getAccessToken|AccessToken/);
});

test("createDraft uses cookie transport without a bearer or unsupported idempotency header", async () => {
  const scratch = await mkdtemp(
    path.join(process.env.TMPDIR ?? os.tmpdir(), "host-listings-adapter-"),
  );
  try {
    await writeFile(
      path.join(scratch, "api-client.mjs"),
      `export const apiClient = { POST: async (_path, options) => { globalThis.__capturedCreateDraft = options; return { response: { ok: true, headers: new Headers({ ETag: '"synthetic-revision"' }) }, data: { id: "550e8400-e29b-41d4-a716-446655440000", category_code: "oficina", category_name: "Oficina", title: "Synthetic", description: "Synthetic", area_m2: 20, capacity: 2, usage_rules: "Synthetic", rate_unit: "hora", base_price_clp: 5001, address: "Synthetic", state: "borrador", region_code: "13", comuna_code: "13101", attribute_schema_version: 1 } }; } };`,
    );
    await writeFile(
      path.join(scratch, "errors.mjs"),
      `export class ApiProtocolError extends Error {}\nexport function normalizeApiError() { return new Error("api error"); }`,
    );
    await writeFile(
      path.join(scratch, "exact-json.mjs"),
      `export function serializeExactJson(value) { return JSON.stringify(value); }`,
    );
    await writeFile(
      path.join(scratch, "validation.mjs"),
      `export function convertSlotsToWeeklyDays() { return []; }\nexport function generateIdempotencyKey() { return "generated-key"; }\nexport function isValidUUID(value) { return typeof value === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value.trim()); }`,
    );
    let adapter = await readFile(
      path.join(
        root,
        "src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts",
      ),
      "utf8",
    );
    adapter = adapter
      .replace('import "client-only";', "")
      .replace('"@/lib/api/client"', '"./api-client.mjs"')
      .replace('"@/lib/api/errors"', '"./errors.mjs"')
      .replace('"@/lib/api/exact-json"', '"./exact-json.mjs"')
      .replace('"../domain/validation"', '"./validation.mjs"');
    const compiled = ts.transpileModule(adapter, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.ESNext,
      },
    }).outputText;
    const modulePath = path.join(scratch, "adapter.mjs");
    await writeFile(modulePath, compiled);
    const { OpenApiHostListingsAdapter } = await import(
      pathToFileURL(modulePath)
    );
    const input = {
      category_code: "oficina",
      title: "Synthetic",
      description: "Synthetic",
      area_m2: 20,
      capacity: 2,
      usage_rules: "Synthetic",
      rate_unit: "hora",
      base_price_clp: 5001,
      address: "Synthetic",
      region_code: "13",
      comuna_code: "13101",
      attribute_schema_version: 1,
    };
    await new OpenApiHostListingsAdapter().createDraft(input, "legacy-key");
    const headers = globalThis.__capturedCreateDraft.headers;
    assert.equal(headers, undefined);
    assert.equal(globalThis.__capturedCreateDraft.params, undefined);
    assert.equal(globalThis.__capturedCreateDraft.body.title, input.title);
    assert.equal(
      Object.keys(headers ?? {}).some(
        (name) => name.toLowerCase() === "idempotency-key",
      ),
      false,
    );
  } finally {
    delete globalThis.__capturedCreateDraft;
    await rm(scratch, { recursive: true, force: true });
  }
});

test("wizard and editor effects do not request category data without access token", async () => {
  const wizard = await source(
    "src/features/host-listings/ui/listing-wizard.tsx",
  );
  const editor = await source(
    "src/features/host-listings/ui/listing-editor.tsx",
  );
  assert.match(wizard, /if \(!accessToken\) return/);
  assert.match(wizard, /loadCategories\(accessToken\)/);
  assert.match(wizard, /loadCategorySchema\(accessToken,/);
  assert.match(editor, /loadCategorySchema\(\s*accessToken,/);
});
