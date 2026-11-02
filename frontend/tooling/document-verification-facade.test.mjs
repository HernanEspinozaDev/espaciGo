import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "espacigo-document-verification-",
  ),
);
const source = await readFile(
  path.join(
    root,
    "src/features/document-verification/application/document-verification-facade.ts",
  ),
  "utf8",
);
const errorsSource = await readFile(
  path.join(root, "src/lib/api/errors.ts"),
  "utf8",
);
const errorsCompiled = ts.transpileModule(errorsSource, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
await writeFile(path.join(scratch, "api-errors.mjs"), errorsCompiled);
const compiled = ts.transpileModule(
  source
    .replaceAll('import "client-only";', "")
    .replaceAll('from "@/lib/api/errors";', 'from "./api-errors.mjs";'),
  {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  },
).outputText;
const modulePath = path.join(scratch, "document-verification-facade.mjs");
await writeFile(modulePath, compiled);
const { DocumentVerificationFacade } = await import(pathToFileURL(modulePath));
test.after(async () => rm(scratch, { recursive: true, force: true }));

const firstId = "550e8400-e29b-41d4-a716-446655440000";
const secondId = "660e8400-e29b-41d4-a716-446655440000";
const ownerFile = {
  id: "770e8400-e29b-41d4-a716-446655440000",
  state: "pendiente",
  size: 512,
  mime: "image/png",
  pages: 1,
};

function makeAuth(sessionSignal = new AbortController().signal) {
  let snapshot = {
    status: "authenticated",
    accountId: firstId,
    roles: [],
    capabilities: [],
    restrictedMode: false,
    expiresAt: "2030-01-01T00:00:00Z",
  };
  const listeners = new Set();
  return {
    getSnapshot: () => snapshot,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    runAuthenticatedOperation(operation) {
      return operation("synthetic-session-token", sessionSignal);
    },
    switchAccount(accountId) {
      snapshot = { ...snapshot, accountId };
      for (const listener of listeners) listener();
    },
    updateSession(changes) {
      snapshot = { ...snapshot, ...changes };
      for (const listener of listeners) listener();
    },
  };
}

function ownerCase(id = firstId, revision = 1, files = []) {
  return {
    id,
    purpose: "space",
    space_id: secondId,
    state: "borrador",
    revision,
    files,
  };
}

test("facade preserves server ETags through upload and submit", async () => {
  const calls = [];
  const port = {
    list: async () => [],
    create: async (_token, input) => {
      calls.push(["create", input]);
      return { value: ownerCase(firstId, 1), etag: '"1"' };
    },
    getOwnCase: async () => ({ value: ownerCase(), etag: '"1"' }),
    upload: async (_token, _id, etag, file) => {
      calls.push(["upload", etag, file.name]);
      return { revision: 2, file: ownerFile, etag: '"2"' };
    },
    submit: async (_token, _id, etag) => {
      calls.push(["submit", etag]);
      return {
        value: { ...ownerCase(firstId, 3, [ownerFile]), state: "enviada" },
        etag: '"3"',
      };
    },
    downloadSpaceFile: async () => new Blob(["synthetic"]),
  };
  const facade = new DocumentVerificationFacade(port, makeAuth());
  const created = await facade.create({ purpose: "space", space_id: secondId });
  assert.equal(created.etag, '"1"');

  const file = new File(["synthetic"], "private-original-name.png", {
    type: "image/png",
  });
  const uploaded = await facade.upload(firstId, file);
  assert.equal(uploaded.etag, '"2"');
  assert.equal(facade.getSnapshot().selected?.etag, '"2"');
  assert.deepEqual(facade.getSnapshot().selected?.value.files, [ownerFile]);

  const submitted = await facade.submit(firstId);
  assert.equal(submitted.etag, '"3"');
  assert.deepEqual(calls, [
    ["create", { purpose: "space", space_id: secondId }],
    ["upload", '"1"', "private-original-name.png"],
    ["submit", '"2"'],
  ]);
});

test("facade lists owner cases, loads a versioned case, and downloads a private blob", async () => {
  const calls = [];
  const cleanFile = { ...ownerFile, state: "limpio" };
  const blob = new Blob(["authorized synthetic document"], {
    type: "application/pdf",
  });
  const port = {
    list: async () => [ownerCase()],
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async (_token, id) => {
      calls.push(["getOwnCase", id]);
      return { value: ownerCase(id, 4, [cleanFile]), etag: '"4"' };
    },
    submit: async () => {
      throw new Error("unused");
    },
    upload: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async (_token, caseId, fileId) => {
      calls.push(["downloadSpaceFile", caseId, fileId]);
      return blob;
    },
  };
  const facade = new DocumentVerificationFacade(port, makeAuth());

  assert.deepEqual(await facade.loadCases(), [ownerCase()]);
  const loaded = await facade.loadOwnCase(firstId);
  assert.equal(loaded?.revision, 4);
  assert.equal(facade.getSnapshot().selected?.etag, '"4"');
  assert.equal(facade.getSnapshot().selected?.value.files[0]?.id, cleanFile.id);
  assert.equal(await facade.downloadSpaceFile(firstId, ownerFile.id), blob);
  assert.deepEqual(calls, [
    ["getOwnCase", firstId],
    ["downloadSpaceFile", firstId, cleanFile.id],
  ]);
  facade.dispose();
});

test("facade discards a late private list result after account changes", async () => {
  let finish;
  const auth = makeAuth();
  const port = {
    list: async () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async () => {
      throw new Error("unused");
    },
    upload: async () => {
      throw new Error("unused");
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);
  const pending = facade.loadCases();
  auth.switchAccount(secondId);
  finish([ownerCase(firstId)]);

  await assert.rejects(pending, { name: "AbortError" });
  assert.deepEqual(facade.getSnapshot().cases, []);
  assert.equal(facade.getSnapshot().selected, null);
  facade.dispose();
});

test("facade clears private data when same-account capabilities change", async () => {
  const auth = makeAuth();
  const port = {
    list: async () => [ownerCase()],
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async () => ({ value: ownerCase(), etag: '"1"' }),
    submit: async () => {
      throw new Error("unused");
    },
    upload: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);
  await facade.loadCases();
  await facade.loadOwnCase(firstId);
  assert.equal(facade.getSnapshot().cases.length, 1);
  assert.notEqual(facade.getSnapshot().selected, null);

  auth.updateSession({ capabilities: ["space.documents.read"] });

  assert.deepEqual(facade.getSnapshot().cases, []);
  assert.equal(facade.getSnapshot().selected, null);
  facade.dispose();
});

test("facade keeps the new session's same-action loading state after an old call settles", async () => {
  const pendingSpaces = [];
  const auth = makeAuth();
  const port = {
    listOwnSpaces: () =>
      new Promise((resolve) => {
        pendingSpaces.push(resolve);
      }),
    list: async () => [],
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async () => {
      throw new Error("unused");
    },
    upload: async () => {
      throw new Error("unused");
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);

  const oldSessionLoad = facade.loadOwnSpaces();
  auth.switchAccount(secondId);
  const newSessionLoad = facade.loadOwnSpaces();
  assert.equal(facade.getSnapshot().action, "load-spaces");

  pendingSpaces[0]([]);
  await assert.rejects(oldSessionLoad, { name: "AbortError" });
  assert.equal(facade.getSnapshot().action, "load-spaces");

  pendingSpaces[1]([]);
  await newSessionLoad;
  assert.equal(facade.getSnapshot().action, null);
  facade.dispose();
});

test("facade ignores an earlier case-selection response that settles last", async () => {
  const pendingCases = new Map();
  const auth = makeAuth();
  const port = {
    list: async () => [],
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: (_token, id) =>
      new Promise((resolve) => {
        pendingCases.set(id, resolve);
      }),
    upload: async () => {
      throw new Error("unused");
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);

  const earlierSelection = facade.loadOwnCase(firstId);
  const laterSelection = facade.loadOwnCase(secondId);
  pendingCases.get(secondId)({ value: ownerCase(secondId, 2), etag: '\"2\"' });
  await laterSelection;
  pendingCases.get(firstId)({ value: ownerCase(firstId, 1), etag: '\"1\"' });

  await assert.rejects(earlierSelection, { name: "AbortError" });
  assert.equal(facade.getSnapshot().selected?.value.id, secondId);
  assert.equal(facade.getSnapshot().selected?.etag, '\"2\"');
  facade.dispose();
});

test("facade keeps the newest same-session case list and spaces results", async () => {
  const pendingLists = [];
  const pendingSpaces = [];
  const port = {
    list: () => new Promise((resolve) => pendingLists.push(resolve)),
    listOwnSpaces: () => new Promise((resolve) => pendingSpaces.push(resolve)),
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async () => {
      throw new Error("unused");
    },
    upload: async () => {
      throw new Error("unused");
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, makeAuth());

  const earlierList = facade.loadCases();
  const laterList = facade.loadCases();
  pendingLists[1]([ownerCase(secondId)]);
  await laterList;
  pendingLists[0]([ownerCase(firstId)]);
  await assert.rejects(earlierList, { name: "AbortError" });
  assert.deepEqual(facade.getSnapshot().cases, [ownerCase(secondId)]);

  const earlierSpaces = facade.loadOwnSpaces();
  const laterSpaces = facade.loadOwnSpaces();
  pendingSpaces[1]([{ id: secondId, title: "Newest" }]);
  await laterSpaces;
  pendingSpaces[0]([{ id: firstId, title: "Older" }]);
  await assert.rejects(earlierSpaces, { name: "AbortError" });
  assert.deepEqual(facade.getSnapshot().spaces, [
    { id: secondId, title: "Newest" },
  ]);
  facade.dispose();
});

test("facade suppresses a rejected action from the prior account session", async () => {
  let rejectCreate;
  const auth = makeAuth();
  const port = {
    list: async () => [],
    create: () =>
      new Promise((_resolve, reject) => {
        rejectCreate = reject;
      }),
    getOwnCase: async () => {
      throw new Error("unused");
    },
    upload: async () => {
      throw new Error("unused");
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);
  const pending = facade.create({ purpose: "space", space_id: secondId });

  auth.switchAccount(secondId);
  rejectCreate(new Error("private failure from the previous account"));

  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().error, null);
  assert.equal(facade.getSnapshot().action, null);
  assert.deepEqual(facade.getSnapshot().cases, []);
  assert.equal(facade.getSnapshot().selected, null);
  facade.dispose();
});

test("facade suppresses AbortError by name across error realms", async () => {
  const abort = new Error("superseded request");
  abort.name = "AbortError";
  const port = {
    list: async () => [],
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async () => {
      throw abort;
    },
    upload: async () => {
      throw new Error("unused");
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, makeAuth());

  await assert.rejects(facade.loadOwnCase(firstId), { name: "AbortError" });

  assert.equal(facade.getSnapshot().error, null);
  facade.dispose();
});

test("facade forwards the session AbortSignal to every document port operation", async () => {
  const sessionSignal = new AbortController().signal;
  const seenSignals = [];
  const capture =
    (result) =>
    async (...args) => {
      seenSignals.push(args[args.length - 1]);
      return result;
    };
  const cleanFile = { ...ownerFile, state: "limpio" };
  const cleanCase = { value: ownerCase(firstId, 2, [cleanFile]), etag: '"2"' };
  const port = {
    list: capture([ownerCase()]),
    listOwnSpaces: capture([{ id: secondId, title: "Synthetic space" }]),
    create: capture({ value: ownerCase(firstId), etag: '"1"' }),
    getOwnCase: capture(cleanCase),
    submit: capture({ value: ownerCase(firstId, 3, [cleanFile]), etag: '"3"' }),
    upload: capture({ revision: 3, file: cleanFile, etag: '"3"' }),
    downloadSpaceFile: capture(new Blob(["synthetic document"])),
  };
  const facade = new DocumentVerificationFacade(port, makeAuth(sessionSignal));

  await facade.loadCases();
  await facade.loadOwnSpaces();
  await facade.create({ purpose: "space", space_id: secondId });
  await facade.loadOwnCase(firstId);
  await facade.upload(
    firstId,
    new File(["synthetic"], "document.png", { type: "image/png" }),
  );
  await facade.submit(firstId);
  await facade.loadOwnCase(firstId);
  await facade.downloadSpaceFile(firstId, cleanFile.id);

  assert.equal(seenSignals.length, 8);
  assert.ok(seenSignals.every((signal) => signal === sessionSignal));
  facade.dispose();
});
