import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-host-extensions-"),
);
const source = (
  await readFile(
    path.join(
      root,
      "src/features/host-listings/application/host-listings-facade.ts",
    ),
    "utf8",
  )
).replace('import "client-only";', "");
const output = path.join(scratch, "facade.mjs");
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
const space = "550e8400-e29b-41d4-a716-446655440000";
const ids = [
  "550e8400-e29b-41d4-a716-446655440001",
  "550e8400-e29b-41d4-a716-446655440002",
  "550e8400-e29b-41d4-a716-446655440003",
];
async function fixture(overrides = {}) {
  let revision = 1;
  let photos = ids.map((id, index) => ({
    id,
    position: index + 1,
    is_cover: index === 0,
  }));
  const requests = [];
  const port = {
    getListing: async () => ({
      id: space,
      category_code: "oficina",
      title: "Synthetic space",
      state: "borrador",
      etag: `"${revision}"`,
    }),
    getPricing: async () => null,
    getWeeklySchedule: async () => null,
    listPhotos: async () => photos,
    movePhoto: async (listingId, photoId, targetPosition, etag, signal) => {
      requests.push({ listingId, photoId, targetPosition, etag, signal });
      const next = [...photos];
      const [photo] = next.splice(
        next.findIndex((item) => item.id === photoId),
        1,
      );
      next.splice(targetPosition - 1, 0, photo);
      photos = next.map((item, index) => ({ ...item, position: index + 1 }));
      revision++;
      return `"${revision}"`;
    },
    ...overrides,
  };
  const core = new HostListingsFacade(port);
  await core.selectListing(space);
  return { core, requests };
}

test("reorder validates the entire permutation before any mutation", async () => {
  const { core, requests } = await fixture();
  await assert.rejects(core.reorderPhotos(space, [ids[0], ids[0], ids[2]]));
  assert.equal(requests.length, 0);
  assert.deepEqual(
    core.getSnapshot().photos.map((item) => item.id),
    ids,
  );
});

test("reorder composes existing position CAS with each renewed revision", async () => {
  const { core, requests } = await fixture();
  await core.reorderPhotos(space, [ids[2], ids[1], ids[0]]);
  assert.deepEqual(
    requests.map(({ photoId, targetPosition, etag }) => ({
      photoId,
      targetPosition,
      etag,
    })),
    [
      { photoId: ids[2], targetPosition: 1, etag: '"1"' },
      { photoId: ids[1], targetPosition: 2, etag: '"2"' },
    ],
  );
  assert.deepEqual(
    core.getSnapshot().photos.map((item) => item.id),
    [ids[2], ids[1], ids[0]],
  );
  assert.equal(core.getSnapshot().selectedListing.etag, '"3"');
});

test("a late private variant is discarded after the session is cleared", async () => {
  let finish;
  let signal;
  const { core } = await fixture({
    getPhotoVariant: async (_listing, _photo, _variant, currentSignal) => {
      signal = currentSignal;
      return new Promise((resolve) => {
        finish = resolve;
      });
    },
  });
  const pending = core.loadPhotoVariant(space, ids[0], "thumbnail");
  core.clear();
  assert.equal(signal.aborted, true);
  finish(new Blob(["synthetic image"], { type: "image/webp" }));
  await assert.rejects(pending, { name: "AbortError" });
});

test("cover sends selected CAS and installs only the authoritative readback", async () => {
  let revision = '"1"';
  let received;
  const { core } = await fixture({
    getListing: async () => ({
      id: space,
      etag: revision,
      cover_photo_id: revision === '"2"' ? ids[1] : ids[0],
    }),
    setCoverPhoto: async (_space, _photo, signal, ifMatch) => {
      received = { signal, ifMatch };
      revision = '"2"';
    },
  });
  await core.setCoverPhoto(space, ids[1]);
  assert.equal(received.ifMatch, '"1"');
  assert.equal(received.signal.aborted, false);
  assert.equal(core.getSnapshot().selectedListing.etag, '"2"');
  assert.equal(core.getSnapshot().selectedListing.cover_photo_id, ids[1]);
});

test("cover readback failure does not replay or replace the prior confirmed data", async () => {
  let failReadback = false;
  let mutations = 0;
  const { core } = await fixture({
    getListing: async () => {
      if (failReadback) throw new Error("Synthetic readback outage");
      return { id: space, etag: '"1"', cover_photo_id: ids[0] };
    },
    setCoverPhoto: async () => {
      mutations++;
      failReadback = true;
    },
  });
  const prior = core.getSnapshot();
  await assert.rejects(core.setCoverPhoto(space, ids[1]), /sin confirmar/i);
  assert.equal(mutations, 1);
  assert.deepEqual(core.getSnapshot().selectedListing, prior.selectedListing);
  assert.deepEqual(core.getSnapshot().photos, prior.photos);
  assert.equal(core.getSnapshot().galleryNeedsRefresh, true);
  await assert.rejects(core.setCoverPhoto(space, ids[1]), /Actualiza/i);
  assert.equal(mutations, 1);
  await assert.rejects(core.loadPhotos(space));
  assert.equal(core.getSnapshot().galleryNeedsRefresh, true);
  failReadback = false;
  await core.loadPhotos(space);
  assert.equal(core.getSnapshot().galleryNeedsRefresh, false);
});

test("initial selection with failed photo read stays unconfirmed until explicit refresh", async () => {
  let fail = true;
  let mutations = 0;
  const core = new HostListingsFacade({
    getListing: async () => ({ id: space, etag: '"1"' }),
    getPricing: async () => null,
    getWeeklySchedule: async () => null,
    listPhotos: async () => {
      if (fail) throw new Error("Synthetic gallery outage");
      return [];
    },
    setCoverPhoto: async () => {
      mutations++;
    },
  });
  await core.selectListing(space);
  assert.equal(core.getSnapshot().galleryNeedsRefresh, true);
  await assert.rejects(core.setCoverPhoto(space, ids[0]), /Actualiza/i);
  assert.equal(mutations, 0);
  fail = false;
  await core.loadPhotos(space);
  assert.equal(core.getSnapshot().galleryNeedsRefresh, false);
});

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});
