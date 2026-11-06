import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

async function withFacade(run) {
  const directory = await mkdtemp(
    path.join(process.env.TMPDIR ?? os.tmpdir(), "gallery-mutation-fence-"),
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

function basePort(overrides = {}) {
  return {
    getListing: async (id) => ({
      id,
      etag: '"current"',
      cover_photo_id: "photo",
    }),
    getPricing: async () => null,
    getWeeklySchedule: async () => null,
    listPhotos: async () => [{ id: "photo", state: "ready", is_cover: true }],
    ...overrides,
  };
}

const image = new File(["synthetic image"], "synthetic.png", {
  type: "image/png",
});

const operations = {
  uploadPhoto: (facade, signal) =>
    facade.uploadPhoto("A", { image }, "original-key", signal),
  replacePhoto: (facade, signal) =>
    facade.replacePhoto(
      "A",
      "photo",
      { image },
      "original-key",
      '"original"',
      signal,
    ),
  removePhoto: (facade, signal) =>
    facade.removePhoto("A", "photo", '"original"', signal),
  setCoverPhoto: (facade, signal) => facade.setCoverPhoto("A", "photo", signal),
  movePhoto: (facade, signal) =>
    facade.movePhoto("A", "photo", 1, '"original"', signal),
};

for (const operation of Object.keys(operations)) {
  test(`${operation} ambiguous failure fences subsequent writes until authoritative GET`, async () => {
    await withFacade(async (Facade) => {
      let nextWrites = 0;
      let attempts = 0;
      const nextOperation =
        operation === "removePhoto" ? "uploadPhoto" : "removePhoto";
      const facade = new Facade(
        basePort({
          [operation]: async () => {
            attempts++;
            throw new TypeError("response lost after dispatch");
          },
          [nextOperation]: async () => {
            nextWrites++;
            return nextOperation === "uploadPhoto"
              ? { id: "after", state: "ready" }
              : '"renewed"';
          },
        }),
      );
      await facade.selectListing("A");
      await assert.rejects(operations[operation](facade), /response lost/);
      assert.equal(facade.getSnapshot().galleryNeedsRefresh, true);
      await assert.rejects(
        operations[nextOperation](facade),
        /Actualiza la galería/,
      );
      assert.equal(nextWrites, 0);
      assert.equal(attempts, 1);
      await facade.loadPhotos("A");
      assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
      await operations[nextOperation](facade);
      assert.equal(nextWrites, 1);
      assert.equal(attempts, 1);
    });
  });
}

for (const operation of Object.keys(operations)) {
  test(`${operation} authoritative 422 rejection does not create an ambiguous-write fence`, async () => {
    await withFacade(async (Facade) => {
      const facade = new Facade(
        basePort({
          [operation]: async () => {
            throw Object.assign(new Error("invalid synthetic input"), {
              status: 422,
              code: "validation_error",
            });
          },
        }),
      );
      await facade.selectListing("A");
      await assert.rejects(
        operations[operation](facade),
        (error) => error.status === 422,
      );
      assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
      assert.equal(facade.getSnapshot().selectedListing.id, "A");
    });
  });
  for (const status of [401, 403]) {
    test(`${operation} ${status} purges without restoring error or uncertainty`, async () => {
      await withFacade(async (Facade) => {
        const facade = new Facade(
          basePort({
            [operation]: async () => {
              throw Object.assign(new Error("old private scope"), { status });
            },
          }),
        );
        await facade.selectListing("A");
        await assert.rejects(
          operations[operation](facade),
          (error) => error.status === status,
        );
        const snapshot = facade.getSnapshot();
        assert.equal(snapshot.selectedListing, null);
        assert.deepEqual(snapshot.photos, []);
        assert.equal(snapshot.galleryNeedsRefresh, false);
        assert.equal(snapshot.error, null);
      });
    });
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

for (const operation of Object.keys(operations)) {
  for (const outcome of ["success", "failure"]) {
    test(`${operation} late ${outcome} cannot publish across A/B/A`, async () => {
      await withFacade(async (Facade) => {
        const pending = deferred();
        const facade = new Facade(
          basePort({ [operation]: () => pending.promise }),
        );
        await facade.selectListing("A");
        const result = operations[operation](facade);
        await facade.selectListing("B");
        await facade.selectListing("A");
        const fresh = facade.getSnapshot();
        if (outcome === "failure")
          pending.reject(new TypeError("old failed request"));
        else
          pending.resolve(
            operation === "removePhoto" || operation === "movePhoto"
              ? '"old-revision"'
              : { id: "old-private-photo", state: "ready" },
          );
        await assert.rejects(result, { name: "AbortError" });
        assert.equal(facade.getSnapshot(), fresh);
        assert.equal(fresh.galleryNeedsRefresh, false);
        assert.equal(fresh.error, null);
      });
    });
  }
  test(`${operation} caller abort after dispatch keeps the current gallery uncertain`, async () => {
    await withFacade(async (Facade) => {
      const pending = deferred();
      const controller = new AbortController();
      const facade = new Facade(
        basePort({ [operation]: () => pending.promise }),
      );
      await facade.selectListing("A");
      const result = operations[operation](facade, controller.signal);
      controller.abort();
      pending.reject(
        new DOMException("request aborted after dispatch", "AbortError"),
      );
      await assert.rejects(result, { name: "AbortError" });
      assert.equal(facade.getSnapshot().galleryNeedsRefresh, true);
      assert.equal(facade.getSnapshot().error, null);
      await facade.loadPhotos("A");
      assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
    });
  });
}

for (const operation of ["uploadPhoto", "replacePhoto"]) {
  test(`${operation} terminal lookup reconciles before an explicit same-intent retry`, async () => {
    await withFacade(async (Facade) => {
      const calls = [];
      let reads = 0;
      const facade = new Facade(
        basePort({
          getListing: async (id) => {
            reads++;
            return { id, etag: '\"current\"' };
          },
          [operation]: async (...args) => {
            calls.push(args);
            if (calls.length === 1) throw new TypeError("lost response");
            return { id: "photo", state: "ready" };
          },
          getPhotoUploadStatus: async () => ({ state: "not_found" }),
        }),
      );
      await facade.selectListing("A");
      await assert.rejects(operations[operation](facade), /lost response/);
      const priorReads = reads;
      assert.equal(calls.length, 1);
      assert.deepEqual(await facade.getPhotoUploadStatus("A", "original-key"), {
        state: "not_found",
      });
      assert.equal(calls.length, 1, "lookup must never replay a mutation");
      assert.ok(
        reads > priorReads,
        "terminal lookup must reconcile the selected revision",
      );
      assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
      await operations[operation](facade);
      assert.equal(calls.length, 2);
      const inputIndex = operation === "uploadPhoto" ? 1 : 2;
      const keyIndex = operation === "uploadPhoto" ? 2 : 3;
      assert.equal(calls[0][inputIndex].image, image);
      assert.equal(calls[1][inputIndex].image, image);
      assert.equal(calls[0][keyIndex], calls[1][keyIndex]);
      if (operation === "replacePhoto") assert.equal(calls[0][4], calls[1][4]);
    });
  });
}

test("initial cover intent does not overwrite a cover discovered by authoritative refresh", async () => {
  await withFacade(async (Facade) => {
    let coverWrites = 0;
    const facade = new Facade(
      basePort({
        setCoverPhoto: async () => {
          coverWrites++;
        },
      }),
    );
    await facade.selectListing("A");
    await facade.loadPhotos("A");
    await facade.setCoverPhoto("A", "new-photo", undefined, true);
    assert.equal(coverWrites, 0);
    assert.equal(facade.getSnapshot().selectedListing.cover_photo_id, "photo");
    assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
  });
});

test("pending replacement retains the prior photo objects and revision until terminal lookup", async () => {
  await withFacade(async (Facade) => {
    let readbacksAllowed = true;
    const facade = new Facade(
      basePort({
        listPhotos: async () => {
          assert.equal(
            readbacksAllowed,
            true,
            "pending is not a new confirmed gallery snapshot",
          );
          return [{ id: "prior", state: "ready", is_cover: true }];
        },
        replacePhoto: async () => ({ id: "reserved", state: "pending" }),
      }),
    );
    await facade.selectListing("A");
    const prior = facade.getSnapshot();
    readbacksAllowed = false;
    const result = await facade.replacePhoto(
      "A",
      "prior",
      { image },
      "original-key",
      '\"current\"',
    );
    assert.equal(result.state, "pending");
    assert.equal(facade.getSnapshot().photos, prior.photos);
    assert.equal(facade.getSnapshot().selectedListing, prior.selectedListing);
    assert.equal(facade.getSnapshot().galleryNeedsRefresh, false);
  });
});

for (const status of [401, 403]) {
  test(`cover commit followed by ${status} readback denial purges instead of wrapping authorization`, async () => {
    await withFacade(async (Facade) => {
      let committed = false;
      const facade = new Facade(
        basePort({
          getListing: async (id) => {
            if (committed)
              throw Object.assign(new Error("readback no longer authorized"), {
                status,
              });
            return { id, etag: '"current"', cover_photo_id: "old" };
          },
          setCoverPhoto: async () => {
            committed = true;
          },
        }),
      );
      await facade.selectListing("A");
      await assert.rejects(
        facade.setCoverPhoto("A", "photo"),
        (error) => error.status === status,
      );
      assert.equal(committed, true);
      assert.equal(facade.getSnapshot().selectedListing, null);
      assert.deepEqual(facade.getSnapshot().photos, []);
      assert.equal(facade.getSnapshot().error, null);
    });
  });
}
