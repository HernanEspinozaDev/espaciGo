import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-local-date-time-"),
);
const source = await readFile(
  path.join(root, "src/features/host-listings/domain/local-date-time.ts"),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
const modulePath = path.join(scratch, "local-date-time.mjs");
await writeFile(modulePath, output);
const { localDateTimeToUtcISOString } = await import(
  pathToFileURL(modulePath).href
);

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("converts an unambiguous Chilean wall time using the IANA zone", () => {
  assert.equal(
    localDateTimeToUtcISOString("2026-12-15T13:00", "America/Santiago"),
    "2026-12-15T16:00:00Z",
  );
});

test("rejects nonexistent wall times at a daylight-saving gap", () => {
  assert.throws(
    () => localDateTimeToUtcISOString("2026-03-08T02:30", "America/New_York"),
    /inexistente o ambigua/i,
  );
});

test("rejects ambiguous wall times at a daylight-saving overlap", () => {
  assert.throws(
    () => localDateTimeToUtcISOString("2026-11-01T01:30", "America/New_York"),
    /inexistente o ambigua/i,
  );
});
