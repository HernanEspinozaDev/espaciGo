import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-account-ops-facade-"),
);

async function loadFacade() {
  const blobFile = path.join(scratch, "private-blob-urls.mjs");
  await writeFile(
    blobFile,
    [
      "export const blobEvents = [];",
      "export const privateBlobUrls = {",
      "  create: (key, blob) => {",
      "    blobEvents.push({ type: 'create', key, blob });",
      "    return { url: 'blob:test' };",
      "  },",
      "  revoke: (key) => {",
      "    blobEvents.push({ type: 'revoke', key });",
      "  },",
      "};",
    ].join("\n"),
    "utf8",
  );

  let source = await readFile(
    path.join(root, "src/features/account/application/account-facade.ts"),
    "utf8",
  );
  source = source.replace('import "client-only";', "");
  source = source.replace(
    'from "@/lib/api/private-blob-urls"',
    'from "./private-blob-urls.mjs"',
  );

  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  });
  const file = path.join(scratch, "account-facade.mjs");
  await writeFile(file, outputText);
  const facadeMod = await import(pathToFileURL(file).href);
  const blobMod = await import(pathToFileURL(blobFile).href);
  facadeMod.AccountFacade.blobEvents = blobMod.blobEvents;
  return facadeMod.AccountFacade;
}

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("updateProfile diferido tras clear() no aplica mutación y rechaza con AbortError", async () => {
  const AccountFacade = await loadFacade();
  let resolveUpdate;
  const updatePromise = new Promise((resolve) => {
    resolveUpdate = resolve;
  });

  const fakePort = {
    updateProfile: () => updatePromise,
  };

  const facade = new AccountFacade(fakePort);

  let result;
  let rejectedError = null;
  const pending = facade.updateProfile({ displayName: "Actor A" }).then(
    (val) => {
      result = val;
    },
    (err) => {
      rejectedError = err;
    },
  );

  facade.clear();

  const syntheticProfile = {
    id: "user-1",
    displayName: "Actor A",
  };
  resolveUpdate(syntheticProfile);
  await pending;

  const snapshot = facade.getSnapshot();
  assert.equal(snapshot.profile, null);
  assert.equal(snapshot.error, null);
  assert.equal(snapshot.loading, false);

  assert.equal(result, undefined);
  assert.ok(rejectedError, "Debe rechazar con AbortError tras clear()");
  assert.equal(rejectedError.name, "AbortError");
});

test("downloadIdentityArchive diferido tras clear() no entrega blob y mantiene snapshot limpio", async () => {
  const AccountFacade = await loadFacade();
  let resolveArchive;
  const archivePromise = new Promise((resolve) => {
    resolveArchive = resolve;
  });

  const fakePort = {
    downloadIdentityArchive: () => archivePromise,
  };

  const facade = new AccountFacade(fakePort);

  let result;
  let rejectedError = null;
  const pending = facade.downloadIdentityArchive().then(
    (val) => {
      result = val;
    },
    (err) => {
      rejectedError = err;
    },
  );

  facade.clear();

  const syntheticZipBlob = new Blob(["datos-archivo-zip"], {
    type: "application/zip",
  });
  resolveArchive(syntheticZipBlob);
  await pending;

  const snapshot = facade.getSnapshot();
  assert.equal(snapshot.profile, null);
  assert.equal(snapshot.photo, null);
  assert.equal(snapshot.photoUrl, null);
  assert.equal(snapshot.payoutAccount, null);
  assert.deepEqual(snapshot.rightsRequests, []);
  assert.equal(snapshot.loading, false);
  assert.equal(snapshot.error, null);

  if (rejectedError) {
    assert.equal(rejectedError.name, "AbortError");
  } else {
    assert.equal(result, null);
  }
});
