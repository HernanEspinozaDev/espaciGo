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
const loadedModule = { exports: {} };
new Function("module", "exports", javascript)(
  loadedModule,
  loadedModule.exports,
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

test("wizard copy asks for codes and does not promise draft idempotency", async () => {
  const ui = await readFile(
    fileURLToPath(
      new URL(
        "../src/features/host-listings/ui/listing-wizard.tsx",
        import.meta.url,
      ),
    ),
    "utf8",
  );
  assert.match(ui, /Código de comuna \(5 dígitos\)/);
  assert.match(ui, /id="wizard-comuna"/);
  assert.doesNotMatch(ui, /clave de idempotencia|idempotencia única UUIDv4/i);
});
