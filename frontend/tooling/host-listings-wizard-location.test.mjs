import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";

const root = new URL(
  "../src/features/host-listings/domain/wizard-location.ts",
  import.meta.url,
);
const source = await readFile(root, "utf8");
const javascript = ts.transpile(source, { module: ts.ModuleKind.CommonJS });
const cataloguePath = new URL(
  "../src/features/host-listings/domain/chile-dpa.ts",
  import.meta.url,
);
const catalogueSource = await readFile(cataloguePath, "utf8");
const catalogueJs = ts.transpile(catalogueSource, {
  module: ts.ModuleKind.CommonJS,
});
const catalogueModule = { exports: {} };
new Function("module", "exports", catalogueJs)(
  catalogueModule,
  catalogueModule.exports,
);
const loadedModule = { exports: {} };
new Function("module", "exports", "require", javascript)(
  loadedModule,
  loadedModule.exports,
  (specifier) => {
    assert.equal(specifier, "./chile-dpa");
    return catalogueModule.exports;
  },
);
const { validateWizardLocation } = loadedModule.exports;

test("accepts valid region/comuna code pair and optional empty pair", () => {
  assert.deepEqual(validateWizardLocation("13", "13101"), { valid: true });
  assert.deepEqual(validateWizardLocation("", ""), { valid: true });
});

test("rejects incomplete, malformed, and region-mismatched codes", () => {
  for (const [region, comuna] of [
    ["13", ""],
    ["", "13101"],
    ["13", "1310"],
    ["13", "05101"],
    ["1", "13101"],
    ["130", "13001"],
    ["13", "Las Condes"],
  ]) {
    assert.equal(validateWizardLocation(region, comuna).valid, false);
  }
});

test("wizard uses accessible location selects and does not promise idempotency", async () => {
  const ui = await readFile(
    fileURLToPath(
      new URL(
        "../src/features/host-listings/ui/listing-wizard.tsx",
        import.meta.url,
      ),
    ),
    "utf8",
  );
  assert.match(ui, /id="wizard-region"/);
  assert.match(ui, /id="wizard-comuna"/);
  assert.match(ui, /\{\.\.\.fieldFeedback\("comuna"\)\}/);
  assert.doesNotMatch(ui, /clave de idempotencia|idempotencia única UUIDv4/i);
});
