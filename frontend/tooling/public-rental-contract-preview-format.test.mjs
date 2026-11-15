import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const sourceUrl = new URL(
  "../src/features/public-catalog/domain/contract-preview-format.ts",
  import.meta.url,
);
const source = await readFile(sourceUrl, "utf8").catch(() => "");
const ts = await import("typescript");
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;
const formatModule = { exports: {} };
new Function("module", "exports", compiled)(formatModule, formatModule.exports);

test("contract preview CLP formatter preserves integer precision for string amounts", () => {
  assert.equal(
    formatModule.exports.formatRentalCLP("9007199254740993"),
    "$9.007.199.254.740.993 CLP",
  );
  assert.equal(formatModule.exports.formatRentalCLP(180000), "$180.000 CLP");
});
