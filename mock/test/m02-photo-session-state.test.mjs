import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const source = await readFile(new URL("../../internal/mockserver/static/m02-photo-session-state.js", import.meta.url), "utf8");
const {captureM02PhotoSession, deliverM02PhotoIfCurrent, m02PhotoSessionMatches, applyM02PhotoIfCurrent} =
  await import(`data:text/javascript;base64,${Buffer.from(source).toString("base64")}`);

function deferred() {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return {promise, resolve};
}

test("discards a delayed photo blob after logout and relogin to the same account", async () => {
  const captured = captureM02PhotoSession("token-old", "account-a", 4);
  assert.ok(captured);
  const responseBlob = deferred();
  let current = {token:"token-old", account:"account-a", generation:4};
  let imageOrMetadataUpdates = 0;
  const load = deliverM02PhotoIfCurrent(
    captured,
    () => responseBlob.promise,
    context => m02PhotoSessionMatches(context, current.token, current.account, current.generation),
    () => { imageOrMetadataUpdates++; },
  );

  current = {token:"token-new", account:"account-a", generation:5};
  responseBlob.resolve(new Blob(["stale-photo"]));
  assert.equal(await load, false);
  assert.equal(imageOrMetadataUpdates, 0);
});

test("does not apply photo metadata or messages after loadM02Photo returns to a replaced session", () => {
  const captured = captureM02PhotoSession("token-a", "account-a", 9);
  assert.ok(captured);
  const current = {token:"token-b", account:"account-b", generation:10};
  let metadataOrMessageUpdates = 0;
  const applied = applyM02PhotoIfCurrent(
    captured,
    context => m02PhotoSessionMatches(context, current.token, current.account, current.generation),
    () => { metadataOrMessageUpdates++; },
  );
  assert.equal(applied, false);
  assert.equal(metadataOrMessageUpdates, 0);
});

test("session generation rejects delayed work even if account and token are reused", async () => {
  const captured = captureM02PhotoSession("same-token", "account-a", 2);
  assert.ok(captured);
  const pendingPhotoLoad = deferred();
  const current = {token:"same-token", account:"account-a", generation:3};
  let applied = 0;
  const operation = deliverM02PhotoIfCurrent(
    captured,
    () => pendingPhotoLoad.promise,
    context => m02PhotoSessionMatches(context, current.token, current.account, current.generation),
    () => { applied++; },
  );
  pendingPhotoLoad.resolve({metadata:{synthetic:true}});
  assert.equal(await operation, false);
  assert.equal(applied, 0);
});
