import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

async function withFacade(run) {
  const directory = await mkdtemp(
    path.join(
      process.env.TMPDIR ?? os.tmpdir(),
      "gallery-concurrent-readback-",
    ),
  );
  try {
    const source = (
      await readFile(
        new URL(
          "../src/features/host-listings/application/host-listings-facade.ts",
          import.meta.url,
        ),
        "utf8",
      )
    ).replace('import "client-only";', "");
    const file = path.join(directory, "facade.mjs");
    await writeFile(
      file,
      ts.transpileModule(source, {
        compilerOptions: {
          target: ts.ScriptTarget.ES2022,
          module: ts.ModuleKind.ESNext,
        },
      }).outputText,
    );
    const { HostListingsFacade } = await import(pathToFileURL(file));
    await run(HostListingsFacade);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

test("overlapping same-selection gallery readbacks both succeed without false uncertainty", async () => {
  await withFacade(async (Facade) => {
    const reads = [];
    let track = false;
    let revision = '"1"';
    const facade = new Facade({
      getListing: async (id) => ({ id, etag: revision }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: () => {
        if (!track) return Promise.resolve([]);
        const read = deferred();
        reads.push(read);
        return read.promise;
      },
    });
    await facade.selectListing("A");
    track = true;
    const first = facade.loadPhotos("A");
    const second = facade.loadPhotos("A");
    reads[0].resolve([{ id: "old", state: "ready" }]);
    await first;
    reads[1].resolve([{ id: "new", state: "ready" }]);
    await second;
    assert.deepEqual(
      facade.getSnapshot().photos.map(({ id }) => id),
      ["new"],
    );
    assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
    assert.equal(facade.getSnapshot().error, null);
    assert.equal(facade.getSnapshot().selectedListing.etag, '"1"');
  });
});

test("A/B/A suppresses stale gallery read failure without waiting for ignored abort", async () => {
  await withFacade(async (Facade) => {
    const oldRead = deferred();
    const calls = new Map();
    let track = false;
    const facade = new Facade({
      getListing: async (id) => ({ id, etag: `"${id}"` }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: (id) => {
        if (!track) return Promise.resolve([]);
        if (id === "A" && !calls.has("A")) {
          calls.set("A", oldRead);
          return oldRead.promise;
        }
        return Promise.resolve([{ id: `${id}-fresh`, state: "ready" }]);
      },
    });
    await facade.selectListing("A");
    track = true;
    const stale = facade.loadPhotos("A");
    await facade.selectListing("B");
    await facade.selectListing("A");
    await facade.loadPhotos("A");
    assert.deepEqual(
      facade.getSnapshot().photos.map(({ id }) => id),
      ["A-fresh"],
    );
    oldRead.reject(new Error("stale read failure"));
    await assert.rejects(stale, { name: "AbortError" });
    assert.deepEqual(
      facade.getSnapshot().photos.map(({ id }) => id),
      ["A-fresh"],
    );
    assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
    assert.equal(facade.getSnapshot().error, null);
  });
});

test("late successful A read cannot restore photos or revision after A/B/A", async () => {
  await withFacade(async (Facade) => {
    const oldRead = deferred();
    let deferNext = false;
    let revision = '"initial"';
    const facade = new Facade({
      getListing: async (id) => ({ id, etag: revision }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async (id) => {
        if (deferNext) {
          deferNext = false;
          return oldRead.promise;
        }
        return [{ id: `${id}-current`, state: "ready" }];
      },
    });
    await facade.selectListing("A");
    deferNext = true;
    const stale = facade.loadPhotos("A");
    await facade.selectListing("B");
    revision = '"fresh-A"';
    await facade.selectListing("A");
    await facade.loadPhotos("A");
    oldRead.resolve([{ id: "A-old-private-photo", state: "ready" }]);
    await assert.rejects(stale, { name: "AbortError" });
    const snapshot = facade.getSnapshot();
    assert.deepEqual(
      snapshot.photos.map(({ id }) => id),
      ["A-current"],
    );
    assert.equal(snapshot.selectedListing.etag, '"fresh-A"');
    assert.equal(snapshot.galleryNeedsRefresh, false);
    assert.equal(snapshot.error, null);
  });
});

test("older concurrent read resolving last cannot replace a newer gallery revision", async () => {
  await withFacade(async (Facade) => {
    const reads = [];
    let track = false;
    let revision = '"initial"';
    const facade = new Facade({
      getListing: async (id) => ({ id, etag: revision }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: () => {
        if (!track) return Promise.resolve([]);
        const read = deferred();
        reads.push(read);
        return read.promise;
      },
    });
    await facade.selectListing("A");
    track = true;
    revision = '"older"';
    const first = facade.loadPhotos("A");
    revision = '"newer"';
    const second = facade.loadPhotos("A");
    reads[1].resolve([{ id: "new", state: "ready" }]);
    await second;
    reads[0].resolve([{ id: "old", state: "ready" }]);
    assert.deepEqual(await first, [{ id: "old", state: "ready" }]);
    const snapshot = facade.getSnapshot();
    assert.deepEqual(
      snapshot.photos.map(({ id }) => id),
      ["new"],
    );
    assert.equal(snapshot.selectedListing.etag, '"newer"');
    assert.equal(snapshot.galleryNeedsRefresh, false);
    assert.equal(snapshot.error, null);
  });
});
