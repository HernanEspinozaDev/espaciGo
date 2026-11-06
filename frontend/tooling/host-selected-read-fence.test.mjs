import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const directory = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "host-read-fence-"),
);
const source = (
  await readFile(
    new URL(
      "../src/features/host-listings/application/host-listings-facade.ts",
      import.meta.url,
    ),
    "utf8",
  )
).replace('import "client-only";', "");
const output = path.join(directory, "facade.mjs");
await writeFile(
  output,
  ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText,
);
const { HostListingsFacade } = await import(pathToFileURL(output));

for (const [method, portMethod, field, data] of [
  ["loadPricing", "getPricing", "activeTariff", { space_id: "A", version: 99 }],
  [
    "loadPricingHistory",
    "listPricingHistory",
    "tariffHistory",
    [{ space_id: "A", version: 99 }],
  ],
  [
    "loadWeeklySchedule",
    "getWeeklySchedule",
    "weeklySchedule",
    { space_id: "A", enabled: true },
  ],
]) {
  for (const outcome of ["success", "error"]) {
    for (const returnToA of [false, true]) {
      test(`${method} rejects late ${outcome} across ${returnToA ? "A/B/A" : "A/B"} without overwriting the current selection`, async () => {
        let hold = false;
        let settle;
        const port = {
          getListing: async (id) => ({ id, etag: '"1"' }),
          getPricing: async () => null,
          getWeeklySchedule: async () => null,
          listPricingHistory: async () => [],
          listPhotos: async () => [],
        };
        const normal = port[portMethod];
        port[portMethod] = (...args) =>
          hold
            ? new Promise((resolve, reject) => {
                settle =
                  outcome === "success"
                    ? () => resolve(data)
                    : () => reject(new Error("stale failure"));
              })
            : normal(...args);
        const facade = new HostListingsFacade(port);
        await facade.selectListing("A");
        hold = true;
        const pending = facade[method]("A");
        hold = false;
        await facade.selectListing("B");
        if (returnToA) await facade.selectListing("A");
        const prior = facade.getSnapshot();
        const rejection = assert.rejects(pending, { name: "AbortError" });
        settle();
        await rejection;
        assert.equal(
          facade.getSnapshot().selectedListing,
          prior.selectedListing,
        );
        assert.deepEqual(facade.getSnapshot()[field], prior[field]);
        assert.equal(facade.getSnapshot().error, prior.error);
      });
    }
  }
}
test.after(async () => {
  await rm(directory, { recursive: true, force: true });
});
