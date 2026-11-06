import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

async function withFacade(run) {
  const directory = await mkdtemp(
    path.join(process.env.TMPDIR ?? os.tmpdir(), "gallery-facade-"),
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

test("upload forwards the exact AbortSignal, file input and original idempotency key", async () => {
  await withFacade(async (Facade) => {
    let received;
    const facade = new Facade({
      getListing: async () => ({ id: "A", etag: '"1"' }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async () => [{ id: "photo", state: "ready" }],
      uploadPhoto: async (...args) => {
        received = args;
        return { id: "photo", state: "ready" };
      },
    });
    await facade.selectListing("A");
    const controller = new AbortController();
    const input = {
      image: new File(["synthetic"], "test.png", { type: "image/png" }),
    };
    await facade.uploadPhoto("A", input, "original-key", controller.signal);
    assert.deepEqual(received, ["A", input, "original-key", received[3]]);
    assert.ok(received[3] instanceof AbortSignal);
    assert.equal(received[3].aborted, false);
    controller.abort();
    assert.equal(received[3].aborted, true);
  });
});

for (const operation of [
  "configureTimeZone",
  "updateWeeklySchedule",
  "updatePricing",
]) {
  test(`${operation} forwards the selected revision and refreshes it after commit`, async () => {
    await withFacade(async (Facade) => {
      let revision = '"1"';
      let received;
      const facade = new Facade({
        getListing: async () => ({ id: "synthetic-space", etag: revision }),
        getPricing: async () => null,
        getWeeklySchedule: async () => null,
        listPhotos: async () => [],
        [operation]: async (_space, _input, etag) => {
          received = etag;
          revision = '"2"';
          return {
            listingId: "synthetic-space",
            timeZone: "America/Santiago",
            etag: revision,
          };
        },
      });
      await facade.selectListing("synthetic-space");
      await facade[operation](
        "synthetic-space",
        operation === "configureTimeZone" ? "America/Santiago" : {},
        '"1"',
      );
      assert.equal(received, '"1"');
      assert.equal(facade.getSnapshot().selectedListing.etag, '"2"');
    });
  });
}

test("gallery reconciliation refreshes the selected revision before the next mutation", async () => {
  await withFacade(async (Facade) => {
    let revision = '"1"';
    let receivedRevision;
    const facade = new Facade({
      getListing: async () => ({
        id: "synthetic-space",
        etag: revision,
        cover_photo_id: "synthetic-photo",
      }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async () => [{ id: "synthetic-photo", state: "ready" }],
      setCoverPhoto: async (_space, _photo, _signal, etag) => {
        receivedRevision = etag;
      },
    });
    await facade.selectListing("synthetic-space");
    revision = '"2"';
    await facade.loadPhotos("synthetic-space");
    await facade.setCoverPhoto("synthetic-space", "synthetic-photo");
    assert.equal(receivedRevision, '"2"');
  });
});

test("gallery result lookup preserves the selected owner, space and idempotency key without replay", async () => {
  await withFacade(async (Facade) => {
    const calls = [];
    const facade = new Facade({
      getPhotoUploadStatus: async (...args) => {
        calls.push(args);
        return { state: "pending", photo_id: "synthetic-photo" };
      },
      uploadPhoto: () => {
        throw new Error("must not replay");
      },
      replacePhoto: () => {
        throw new Error("must not replay");
      },
    });
    assert.deepEqual(
      await facade.getPhotoUploadStatus("synthetic-space", "synthetic-key"),
      { state: "pending", photo_id: "synthetic-photo" },
    );
    assert.deepEqual(calls, [
      ["synthetic-space", "synthetic-key", calls[0][2]],
    ]);
  });
});

test("latest selection wins for A/B/A even when the first A resolves last", async () => {
  await withFacade(async (Facade) => {
    const deferred = [];
    const facade = new Facade({
      getListing: (id) =>
        new Promise((resolve) => deferred.push({ id, resolve })),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async () => [],
    });
    const firstA = facade.selectListing("A");
    const b = facade.selectListing("B");
    const lastA = facade.selectListing("A");
    deferred[1].resolve({ id: "B" });
    deferred[2].resolve({ id: "A", etag: '"latest"' });
    await Promise.all([assert.rejects(b, { name: "AbortError" }), lastA]);
    deferred[0].resolve({ id: "A", etag: '"stale"' });
    await assert.rejects(firstA, { name: "AbortError" });
    assert.equal(facade.getSnapshot().selectedListing.etag, '"latest"');
  });
});

test("clear during upload aborts stale completion without publishing or error", async () => {
  await withFacade(async (Facade) => {
    let finish;
    const facade = new Facade({
      uploadPhoto: () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
      getListing: async (id) => ({ id, etag: '"1"' }),
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async () => [],
    });
    await facade.selectListing("A");
    const upload = facade.uploadPhoto("A", {}, "key");
    facade.clear();
    finish({ id: "photo" });
    await assert.rejects(upload, { name: "AbortError" });
    assert.equal(facade.getSnapshot().selectedListing, null);
    assert.equal(facade.getSnapshot().error, null);
  });
});

test("cover readback failure keeps the prior snapshot and reports unconfirmed outcome", async () => {
  await withFacade(async (Facade) => {
    const prior = [{ id: "old", is_cover: true }];
    let failReadback = false;
    const facade = new Facade({
      getListing: async () => {
        if (failReadback) throw new Error("readback unavailable");
        return { id: "A", etag: '"1"' };
      },
      getPricing: async () => null,
      getWeeklySchedule: async () => null,
      listPhotos: async () => prior,
      setCoverPhoto: async () => {
        failReadback = true;
      },
    });
    await facade.selectListing("A");
    await assert.rejects(facade.setCoverPhoto("A", "new"), /sin confirmar/i);
    assert.deepEqual(facade.getSnapshot().photos, prior);
  });
});
