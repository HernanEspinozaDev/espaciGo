import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "host-listings-schedule-"),
);
async function loadFacade() {
  let source = await readFile(
    path.join(
      root,
      "src/features/host-listings/application/host-listings-facade.ts",
    ),
    "utf8",
  );
  source = source.replace('import "client-only";', "");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  });
  const file = path.join(scratch, "facade.mjs");
  await writeFile(file, outputText);
  return (await import(pathToFileURL(file).href)).HostListingsFacade;
}
test.after(() => rm(scratch, { recursive: true, force: true }));

for (const rate_unit of ["dia", "mes", null]) {
  test(`selectListing omite horario con tarifa ${rate_unit}`, async () => {
    const Facade = await loadFacade();
    const calls = { schedule: 0 };
    const facade = new Facade({
      getListing: async () => ({ id: "listing-1", rate_unit }),
      getPricing: async () => null,
      getWeeklySchedule: async () => {
        calls.schedule++;
        return null;
      },
      listPhotos: async () => [],
    });
    await facade.selectListing("listing-1");
    assert.equal(calls.schedule, 0);
  });
}

test("tarifa hora solicita horario una vez", async () => {
  const Facade = await loadFacade();
  let calls = 0;
  const facade = new Facade({
    getListing: async () => ({ id: "listing-1", rate_unit: "hora" }),
    getPricing: async () => null,
    getWeeklySchedule: async () => {
      calls++;
      return null;
    },
    listPhotos: async () => [],
  });
  await facade.selectListing("listing-1");
  assert.equal(calls, 1);
});

test("selector obsoleto no inicia horario diferido ni repuebla snapshot", async () => {
  const Facade = await loadFacade();
  let resolveOld;
  const oldListing = new Promise((resolve) => {
    resolveOld = resolve;
  });
  let scheduleCalls = 0;
  const facade = new Facade({
    getListing: (id) =>
      id === "old" ? oldListing : Promise.resolve({ id, rate_unit: "dia" }),
    getPricing: async () => null,
    getWeeklySchedule: async () => {
      scheduleCalls++;
      return null;
    },
    listPhotos: async () => [],
  });
  const oldSelection = facade.selectListing("old");
  await facade.selectListing("new");
  resolveOld({ id: "old", rate_unit: "hora" });
  await assert.rejects(oldSelection, { name: "AbortError" });
  assert.equal(scheduleCalls, 0);
  assert.equal(facade.getSnapshot().selectedListing.id, "new");
});
