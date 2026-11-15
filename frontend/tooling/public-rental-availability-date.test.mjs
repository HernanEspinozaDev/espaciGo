import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";

const sourcePath = new URL(
  "../src/features/public-catalog/domain/local-date.ts",
  import.meta.url,
);

async function loadLocalDate() {
  let source;
  try {
    source = await readFile(sourcePath, "utf8");
  } catch {
    assert.fail("local civil-date formatter must exist");
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

test("local date formatting follows the configured civil timezone across UTC day boundaries", async () => {
  const { localDateISO } = await loadLocalDate();
  assert.equal(
    localDateISO(new Date("2026-11-15T02:30:00Z"), "America/Santiago"),
    "2026-11-14",
  );
  assert.equal(
    localDateISO(new Date("2026-11-01T02:30:00Z"), "America/Santiago"),
    "2026-10-31",
  );
});

test("civil date bounds add whole calendar days without DST drift", async () => {
  const { addCivilDays } = await loadLocalDate();
  assert.equal(addCivilDays("2026-03-07", 1), "2026-03-08");
  assert.equal(addCivilDays("2026-03-08", 90), "2026-06-06");
  assert.equal(addCivilDays("2026-12-31", 1), "2027-01-01");
});
