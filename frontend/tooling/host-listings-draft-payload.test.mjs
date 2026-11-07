import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { parse } from "yaml";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
test("draft creation contract does not promise unsupported idempotency", async () => {
  const contract = parse(
    await readFile(path.join(root, "../planning/openapi.yaml"), "utf8"),
  );
  const create = contract.paths["/spaces"].post;
  assert.equal(create.operationId, "createOwnSpaceDraft");
  assert.equal(
    (create.parameters ?? []).some((entry) =>
      entry.$ref?.endsWith("/IdempotencyKey"),
    ),
    false,
  );
  assert.ok(create.responses["201"].headers.ETag);
});
const source = await readFile(
  path.join(root, "src/features/host-listings/ui/listing-wizard.tsx"),
  "utf8",
);
const cleaningSource = await readFile(
  path.join(root, "src/features/host-listings/domain/attribute-formatting.ts"),
  "utf8",
);

function extractCleaner() {
  const start = cleaningSource.indexOf(
    "export function cleanAttributesForPayload(",
  );
  assert.notEqual(start, -1, "production attribute cleaner exists");
  const open = cleaningSource.indexOf("{", start);
  let depth = 0;
  let end = -1;
  for (let index = open; index < cleaningSource.length; index += 1) {
    if (cleaningSource[index] === "{") depth += 1;
    if (cleaningSource[index] === "}" && --depth === 0) {
      end = index + 1;
      break;
    }
  }
  assert.notEqual(end, -1, "cleaner function body is balanced");
  return ts.transpile(
    cleaningSource.slice(start, end).replace("export function", "function"),
    {
      target: ts.ScriptTarget.ES2022,
    },
  );
}

function extractDraftPayload() {
  const start = source.indexOf("const draftPayload: ListingDraftInput = {");
  assert.notEqual(start, -1, "wizard draft payload builder exists");
  const open = source.indexOf("{", start);
  let depth = 0;
  let end = -1;
  for (let index = open; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}" && --depth === 0) {
      end = index + 1;
      break;
    }
  }
  assert.notEqual(end, -1, "draft payload object is balanced");
  return ts.transpile(`return (${source.slice(open, end)});`, {
    target: ts.ScriptTarget.ES2022,
  });
}

test("draft payload uses only dynamic schema attributes and the loaded schema version", () => {
  const build = new Function(
    "categoryCode",
    "title",
    "description",
    "areaM2",
    "capacity",
    "usageRules",
    "rateUnit",
    "basePriceCLP",
    "address",
    "regionCode",
    "comunaCode",
    "currentSchema",
    "dynamicAttributes",
    "cleanAttributes",
    "cancellationPolicy",
    "depositCLP",
    extractDraftPayload(),
  );
  const schema = { schema_version: 7 };
  const attributes = { capacidad_interior: 12 };
  const payload = build(
    "oficina",
    "Espacio",
    "Descripción",
    20,
    2,
    "Normas",
    "hora",
    15000,
    "Dirección",
    "13",
    "13114",
    schema,
    attributes,
    new Function(`${extractCleaner()}; return cleanAttributesForPayload;`)()({
      ...attributes,
      opcional_vacio: undefined,
      disponible: false,
      minimo: 0,
    }),
    "flexible",
    "",
  );
  assert.deepEqual(payload.attributes, {
    ...attributes,
    disponible: false,
    minimo: 0,
  });
  assert.equal(payload.attribute_schema_version, 7);
  assert.equal(
    payload.usage_rules,
    "Política de cancelación: flexible. Normas",
  );
});

test("submission is blocked if the current category schema has not loaded", () => {
  const start = source.indexOf("const handleCreateDraft = async () =>");
  const handler = source.slice(
    start,
    source.indexOf("const selectedCatObj =", start),
  );
  assert.match(handler, /if\s*\(!currentSchema\)/);
  assert.doesNotMatch(
    handler,
    /attribute_schema_version:\s*currentSchema\?\.schema_version\s*\?\?\s*1/,
  );
});

test("unsupported security deposit control is disabled with an accessible explanation", () => {
  const start = source.indexOf('<Label htmlFor="wizard-deposit"');
  const control = source.slice(start, source.indexOf("</div>", start));
  assert.match(control, /disabled/);
  assert.match(control, /no está disponible/i);
  assert.match(control, /htmlFor="wizard-deposit"/);
});
