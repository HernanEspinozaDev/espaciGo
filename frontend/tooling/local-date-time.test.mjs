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

test("converts valid Chilean wall times on both sides of the spring DST gap", () => {
  assert.equal(
    localDateTimeToUtcISOString("2026-09-05T23:30", "America/Santiago"),
    "2026-09-06T03:30:00Z",
  );
  assert.equal(
    localDateTimeToUtcISOString("2026-09-06T01:30", "America/Santiago"),
    "2026-09-06T04:30:00Z",
  );
});

test("converts unique Chilean wall times on both sides of the fall DST overlap", () => {
  assert.equal(
    localDateTimeToUtcISOString("2026-04-04T22:30", "America/Santiago"),
    "2026-04-05T01:30:00Z",
  );
  assert.equal(
    localDateTimeToUtcISOString("2026-04-05T00:30", "America/Santiago"),
    "2026-04-05T04:30:00Z",
  );
});

test("converts a positive 45-minute IANA offset across the UTC date boundary", () => {
  assert.equal(
    localDateTimeToUtcISOString("2026-01-15T00:15", "Asia/Kathmandu"),
    "2026-01-14T18:30:00Z",
  );
});

test("converts a negative 30-minute IANA offset", () => {
  assert.equal(
    localDateTimeToUtcISOString("2026-01-15T00:15", "America/St_Johns"),
    "2026-01-15T03:45:00Z",
  );
});

test("rejects calendar dates that JavaScript would otherwise normalize", () => {
  assert.throws(
    () => localDateTimeToUtcISOString("2026-02-30T10:00", "America/Santiago"),
    /fecha y hora válidas/i,
  );
});

test("rejects wall-clock hours outside the datetime-local range", () => {
  assert.throws(
    () => localDateTimeToUtcISOString("2026-02-28T24:00", "America/Santiago"),
    /fecha y hora válidas/i,
  );
});

test("handles Lord Howe's half-hour daylight-saving gap and overlap", () => {
  assert.equal(
    localDateTimeToUtcISOString("2026-10-04T01:45", "Australia/Lord_Howe"),
    "2026-10-03T15:15:00Z",
  );
  assert.throws(
    () =>
      localDateTimeToUtcISOString("2026-10-04T02:15", "Australia/Lord_Howe"),
    /inexistente o ambigua/i,
  );
  assert.throws(
    () =>
      localDateTimeToUtcISOString("2026-04-05T01:45", "Australia/Lord_Howe"),
    /inexistente o ambigua/i,
  );
  assert.equal(
    localDateTimeToUtcISOString("2026-04-05T02:15", "Australia/Lord_Howe"),
    "2026-04-04T15:45:00Z",
  );
});
