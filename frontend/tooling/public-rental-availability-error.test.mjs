import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";

const sourcePath = new URL(
  "../src/features/public-catalog/domain/availability-error.ts",
  import.meta.url,
);

async function loadAvailabilityError() {
  let source;
  try {
    source = await readFile(sourcePath, "utf8");
  } catch {
    assert.fail("safe public-availability exception mapping must exist");
  }
  const ts = await import("typescript");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const mod = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    createRequire(sourcePath),
    mod,
    mod.exports,
  );
  return mod.exports;
}

test("unexpected availability failures use a generic message instead of claiming connectivity", async () => {
  const { publicAvailabilityExceptionMessage } = await loadAvailabilityError();
  assert.equal(
    publicAvailabilityExceptionMessage(),
    "No se pudo consultar la disponibilidad en este momento.",
  );
});
