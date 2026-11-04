import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const source = (file) => readFile(path.join(root, file), "utf8");

test("category and category-profile operations thread bearer token end to end", async () => {
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
  for (const text of [port, facade, hook, adapter]) {
    assert.match(
      text,
      /(?:listCategories|loadCategories)\(\s*token: AccessToken/,
    );
    assert.match(
      text,
      /(?:getCategoryProfile|loadCategorySchema)\(\s*token: AccessToken/,
    );
  }
  assert.match(
    adapter,
    /GET\("\/spaces\/categories",\s*\{\s*headers:\s*authHeader\(token\)/,
  );
  assert.match(adapter, /headers:\s*authHeader\(token\)/);
  assert.match(hook, /hostListingsFacade\.loadCategories\(\s*token/);
  assert.match(hook, /hostListingsFacade\.loadCategorySchema\(\s*token/);
});

test("createDraft sends only Authorization and ignores legacy idempotency key", async () => {
  const scratch = await mkdtemp(
    path.join(os.tmpdir(), "host-listings-adapter-"),
  );
  try {
    await writeFile(
      path.join(scratch, "api-client.mjs"),
      `export const apiClient = { POST: async (_path, options) => { globalThis.__capturedCreateDraft = options; return { response: { ok: true }, data: { id: "550e8400-e29b-41d4-a716-446655440000", category_code: "oficina", category_name: "Oficina", title: "Synthetic", description: "Synthetic", area_m2: 20, capacity: 2, usage_rules: "Synthetic", rate_unit: "hora", base_price_clp: 5001, address: "Synthetic", state: "borrador", region_code: "RM", comuna_code: "Santiago", attribute_schema_version: 1 } }; } };`,
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
      `export function convertSlotsToWeeklyDays() { return []; }\nexport function generateIdempotencyKey() { return "generated-key"; }`,
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
      region_code: "RM",
      comuna_code: "Santiago",
      attribute_schema_version: 1,
    };
    await new OpenApiHostListingsAdapter().createDraft(
      "synthetic-token",
      input,
      "legacy-key",
    );
    const headers = globalThis.__capturedCreateDraft.headers;
    assert.deepEqual(headers, { Authorization: "Bearer synthetic-token" });
    assert.equal(
      Object.keys(headers).some(
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
