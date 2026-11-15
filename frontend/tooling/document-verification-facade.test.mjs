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
const domainSource = await readFile(
  path.join(
    root,
    "src/features/document-verification/domain/document-verification.ts",
  ),
  "utf8",
);
const errorsCompiled = ts.transpileModule(errorsSource, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
await writeFile(path.join(scratch, "api-errors.mjs"), errorsCompiled);
const domainCompiled = ts.transpileModule(domainSource, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
  },
}).outputText;
await writeFile(
  path.join(scratch, "domain-document-verification.mjs"),
  domainCompiled,
);
const compiled = ts.transpileModule(
  source
    .replaceAll('import "client-only";', "")
    .replaceAll('from "@/lib/api/errors";', 'from "./api-errors.mjs";')
    .replaceAll(
      'from "../domain/document-verification";',
      'from "./domain-document-verification.mjs";',
    ),
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

test("identity upload forwards its role and replaces only the same role", async () => {
  const oldFront = {
    ...ownerFile,
    id: "front-old",
    evidence_role: "identity_front",
  };
  const back = { ...ownerFile, id: "back", evidence_role: "identity_back" };
  const selfie = {
    ...ownerFile,
    id: "selfie",
    evidence_role: "identity_selfie",
  };
  const replacement = {
    ...ownerFile,
    id: "front-new",
    evidence_role: "identity_front",
  };
  let uploadedRole;
  const port = {
    list: async () => [],
    create: async () => ({
      value: {
        id: firstId,
        purpose: "identity",
        state: "borrador",
        revision: 4,
        files: [oldFront, back, selfie],
      },
      etag: '"4"',
    }),
    getOwnCase: async () => {
      throw new Error("unused");
    },
    upload: async (...args) => {
      uploadedRole = args[4];
      return { revision: 5, file: replacement, etag: '"5"' };
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, makeAuth());
  await facade.create({ purpose: "identity" });
  const file = new File(["synthetic"], "front.png", { type: "image/png" });

  await facade.upload(firstId, file, "identity_front");

  assert.equal(uploadedRole, "identity_front");
  assert.deepEqual(
    facade.getSnapshot().selected?.value.files.map((item) => item.id),
    ["back", "selfie", "front-new"],
  );
  assert.equal(facade.getSnapshot().selected?.etag, '"5"');
  facade.dispose();
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

test("successful identity upload from a previous account cannot repopulate the current snapshot", async () => {
  let resolveUpload;
  let signalStarted;
  const started = new Promise((resolve) => {
    signalStarted = resolve;
  });
  const calls = [];
  const auth = makeAuth();
  const createdCase = {
    id: firstId,
    purpose: "identity",
    state: "borrador",
    revision: 7,
    files: [],
  };
  const returnedFile = {
    ...ownerFile,
    id: "880e8400-e29b-41d4-a716-446655440000",
    state: "pendiente",
    evidence_role: "identity_front",
  };
  const port = {
    list: async () => {
      calls.push(["list"]);
      return [createdCase];
    },
    create: async (_token, input) => {
      calls.push(["create", input]);
      return { value: createdCase, etag: '\"7\"' };
    },
    getOwnCase: async (_token, id) => {
      calls.push(["read", id]);
      return { value: createdCase, etag: '\"7\"' };
    },
    upload: (_token, id, etag, file, role) => {
      calls.push([
        "upload",
        auth.getSnapshot().accountId,
        id,
        etag,
        role,
        file,
      ]);
      signalStarted();
      return new Promise((resolve) => {
        resolveUpload = resolve;
      });
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);
  await facade.create({
    purpose: "identity",
    document_format_declared: "unsure",
    capture_consent_given: true,
    capture_consent_version: "identity-capture-local-v1",
  });
  await facade.loadCases();
  await facade.loadOwnCase(firstId);
  const file = new File(["synthetic identity image"], "front.png", {
    type: "image/png",
  });

  const pendingUpload = facade.upload(firstId, file, "identity_front");
  await started;
  assert.equal(auth.getSnapshot().accountId, firstId);
  assert.deepEqual(calls.at(-1), [
    "upload",
    firstId,
    firstId,
    '\"7\"',
    "identity_front",
    file,
  ]);

  auth.switchAccount(secondId);
  assert.deepEqual(facade.getSnapshot().cases, []);
  assert.deepEqual(facade.getSnapshot().spaces, []);
  assert.equal(facade.getSnapshot().selected, null);
  assert.equal(facade.getSnapshot().action, null);
  assert.equal(facade.getSnapshot().error, null);

  resolveUpload({ revision: 8, file: returnedFile, etag: '\"8\"' });
  await assert.rejects(pendingUpload, { name: "AbortError" });
  assert.deepEqual(facade.getSnapshot().cases, []);
  assert.deepEqual(facade.getSnapshot().spaces, []);
  assert.equal(facade.getSnapshot().selected, null);
  assert.equal(facade.getSnapshot().action, null);
  assert.equal(facade.getSnapshot().error, null);
  assert.deepEqual(
    calls.map(([kind]) => kind),
    ["create", "list", "read", "upload"],
  );
  facade.dispose();
});

test("successful identity submit cannot replace a newer selection in the same account", async () => {
  const accountA = "550e8400-e29b-41d4-a716-446655440000";
  const caseAId = "880e8400-e29b-41d4-a716-446655440001";
  const caseBId = "880e8400-e29b-41d4-a716-446655440002";
  const caseAFiles = [
    {
      id: "990e8400-e29b-41d4-a716-446655440001",
      state: "limpio",
      size: 512,
      mime: "image/jpeg",
      pages: 1,
      evidence_role: "identity_front",
    },
    {
      id: "990e8400-e29b-41d4-a716-446655440002",
      state: "limpio",
      size: 512,
      mime: "image/jpeg",
      pages: 1,
      evidence_role: "identity_back",
    },
    {
      id: "990e8400-e29b-41d4-a716-446655440003",
      state: "limpio",
      size: 512,
      mime: "image/jpeg",
      pages: 1,
      evidence_role: "identity_selfie",
    },
  ];
  const caseA = {
    id: caseAId,
    purpose: "identity",
    state: "borrador",
    revision: 4,
    files: caseAFiles,
  };
  const caseB = {
    id: caseBId,
    purpose: "identity",
    state: "borrador",
    revision: 9,
    files: [],
  };
  const submittedA = { ...caseA, state: "enviada", revision: 5 };
  let resolveSubmit;
  let signalSubmitStarted;
  const submitStarted = new Promise((resolve) => {
    signalSubmitStarted = resolve;
  });
  const calls = [];
  const auth = makeAuth();
  const port = {
    list: async () => {
      calls.push(["list"]);
      return [];
    },
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async (_token, id) => {
      calls.push(["read", auth.getSnapshot().accountId, id]);
      return {
        value: id === caseAId ? caseA : caseB,
        etag: id === caseAId ? '\"4\"' : '\"9\"',
      };
    },
    upload: async () => {
      throw new Error("unused");
    },
    submit: (_token, id, etag) => {
      calls.push(["submit", auth.getSnapshot().accountId, id, etag]);
      signalSubmitStarted();
      return new Promise((resolve) => {
        resolveSubmit = resolve;
      });
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);

  await facade.loadOwnCase(caseAId);
  const pendingSubmit = facade.submit(caseAId);
  await submitStarted;
  assert.deepEqual(calls.at(-1), ["submit", accountA, caseAId, '\"4\"']);

  await facade.loadOwnCase(caseBId);
  const selectedB = facade.getSnapshot().selected;
  assert.deepEqual(selectedB, { value: caseB, etag: '\"9\"' });

  resolveSubmit({ value: submittedA, etag: '\"5\"' });
  await assert.rejects(pendingSubmit, { name: "AbortError" });

  const snapshot = facade.getSnapshot();
  assert.deepEqual(snapshot.selected, selectedB);
  assert.equal(snapshot.selected.value.id, caseBId);
  assert.equal(snapshot.selected.value.state, "borrador");
  assert.notEqual(snapshot.selected.value.state, submittedA.state);
  const caseAInCases = snapshot.cases.find(({ id }) => id === caseAId);
  assert.deepEqual(caseAInCases, caseA);
  assert.equal(caseAInCases.state, "borrador");
  assert.equal(caseAInCases.revision, caseA.revision);
  assert.equal(snapshot.error, null);
  assert.equal(snapshot.action, null);
  assert.deepEqual(calls, [
    ["read", accountA, caseAId],
    ["submit", accountA, caseAId, '\"4\"'],
    ["read", accountA, caseBId],
  ]);
  facade.dispose();
});

test("late identity upload cannot replace a newer same-account case selection", async () => {
  const accountId = firstId;
  const caseAId = "880e8400-e29b-41d4-a716-446655440011";
  const caseBId = "880e8400-e29b-41d4-a716-446655440012";
  const filesA = [
    {
      id: "990e8400-e29b-41d4-a716-446655440011",
      state: "limpio",
      size: 512,
      mime: "image/jpeg",
      pages: 1,
      evidence_role: "identity_front",
    },
    {
      id: "990e8400-e29b-41d4-a716-446655440012",
      state: "limpio",
      size: 512,
      mime: "image/jpeg",
      pages: 1,
      evidence_role: "identity_back",
    },
    {
      id: "990e8400-e29b-41d4-a716-446655440013",
      state: "limpio",
      size: 512,
      mime: "image/jpeg",
      pages: 1,
      evidence_role: "identity_selfie",
    },
  ];
  const caseA = {
    id: caseAId,
    purpose: "identity",
    state: "borrador",
    revision: 4,
    files: filesA,
  };
  const caseB = {
    id: caseBId,
    purpose: "identity",
    state: "borrador",
    revision: 9,
    files: [],
  };
  const uploadedFile = {
    id: "990e8400-e29b-41d4-a716-446655440014",
    state: "pendiente",
    size: 512,
    mime: "image/jpeg",
    pages: 1,
    evidence_role: "identity_front",
  };
  let resolveUpload;
  let signalUploadStarted;
  const uploadStarted = new Promise((resolve) => {
    signalUploadStarted = resolve;
  });
  const calls = [];
  const auth = makeAuth();
  const port = {
    list: async () => {
      throw new Error("unused");
    },
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async (_token, id) => {
      calls.push(["read", auth.getSnapshot().accountId, id]);
      return {
        value: id === caseAId ? caseA : caseB,
        etag: id === caseAId ? '\"4\"' : '\"9\"',
      };
    },
    upload: (_token, id, etag, file, role) => {
      calls.push([
        "upload",
        auth.getSnapshot().accountId,
        id,
        etag,
        role,
        file,
      ]);
      signalUploadStarted();
      return new Promise((resolve) => {
        resolveUpload = resolve;
      });
    },
    submit: async () => {
      throw new Error("unused");
    },
    downloadSpaceFile: async () => {
      throw new Error("unused");
    },
  };
  const facade = new DocumentVerificationFacade(port, auth);
  await facade.loadOwnCase(caseAId);
  const file = new File(["synthetic identity image"], "front.jpg", {
    type: "image/jpeg",
  });
  const pendingUpload = facade.upload(caseAId, file, "identity_front");
  await uploadStarted;
  assert.deepEqual(calls.at(-1), [
    "upload",
    accountId,
    caseAId,
    '\"4\"',
    "identity_front",
    file,
  ]);

  await facade.loadOwnCase(caseBId);
  const selectedB = facade.getSnapshot().selected;
  assert.deepEqual(selectedB, { value: caseB, etag: '\"9\"' });
  resolveUpload({ revision: 5, file: uploadedFile, etag: '\"5\"' });
  await assert.rejects(pendingUpload, { name: "AbortError" });

  const snapshot = facade.getSnapshot();
  assert.deepEqual(snapshot.selected, selectedB);
  assert.equal(snapshot.selected.value.id, caseBId);
  assert.deepEqual(
    snapshot.cases.find(({ id }) => id === caseBId),
    caseB,
  );
  assert.deepEqual(
    snapshot.cases.find(({ id }) => id === caseAId),
    caseA,
  );
  assert.equal(snapshot.error, null);
  assert.equal(snapshot.action, null);
  assert.deepEqual(calls, [
    ["read", accountId, caseAId],
    ["upload", accountId, caseAId, '\"4\"', "identity_front", file],
    ["read", accountId, caseBId],
  ]);
  facade.dispose();
});

test("superseded owner case list does not leave the facade loading", async () => {
  const caseBId = "880e8400-e29b-41d4-a716-446655440020";
  const caseB = {
    id: caseBId,
    purpose: "identity",
    state: "borrador",
    revision: 2,
    files: [],
  };
  let resolveList;
  const calls = [];
  const port = {
    list: () => {
      calls.push(["list"]);
      return new Promise((resolve) => {
        resolveList = resolve;
      });
    },
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async (_token, id) => {
      calls.push(["read", id]);
      return { value: caseB, etag: '\"2\"' };
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

  const pendingList = facade.loadCases();
  assert.equal(facade.getSnapshot().loading, true);
  assert.deepEqual(await facade.loadOwnCase(caseBId), caseB);
  assert.equal(facade.getSnapshot().action, null);

  resolveList([ownerCase(firstId)]);
  await assert.rejects(pendingList, { name: "AbortError" });

  assert.equal(facade.getSnapshot().loading, false);
  assert.deepEqual(facade.getSnapshot().selected, {
    value: caseB,
    etag: '\"2\"',
  });
  assert.deepEqual(facade.getSnapshot().cases, [caseB]);
  assert.equal(facade.getSnapshot().error, null);
  assert.deepEqual(calls, [["list"], ["read", caseBId]]);
  facade.dispose();
});

test("stale case detail cannot clear a newer list's loading state", async () => {
  const caseBId = "880e8400-e29b-41d4-a716-446655440021";
  const caseB = {
    id: caseBId,
    purpose: "identity",
    state: "borrador",
    revision: 2,
    files: [],
  };
  let resolveRead;
  let resolveList;
  const calls = [];
  const port = {
    list: () => {
      calls.push(["list"]);
      return new Promise((resolve) => {
        resolveList = resolve;
      });
    },
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: (_token, id) => {
      calls.push(["read", id]);
      return new Promise((resolve) => {
        resolveRead = resolve;
      });
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

  const pendingRead = facade.loadOwnCase(caseBId);
  const pendingList = facade.loadCases();
  assert.equal(facade.getSnapshot().loading, true);

  resolveRead({ value: caseB, etag: '\"2\"' });
  await assert.rejects(pendingRead, { name: "AbortError" });
  assert.equal(facade.getSnapshot().loading, true);

  resolveList([caseB]);
  await pendingList;
  assert.equal(facade.getSnapshot().loading, false);
  assert.deepEqual(facade.getSnapshot().cases, [caseB]);
  assert.equal(facade.getSnapshot().selected, null);
  assert.deepEqual(calls, [["read", caseBId], ["list"]]);
  facade.dispose();
});

test("failed current case read clears loading from a superseded list", async () => {
  const caseBId = "880e8400-e29b-41d4-a716-446655440022";
  let resolveList;
  const calls = [];
  const port = {
    list: () => {
      calls.push(["list"]);
      return new Promise((resolve) => {
        resolveList = resolve;
      });
    },
    create: async () => {
      throw new Error("unused");
    },
    getOwnCase: async (_token, id) => {
      calls.push(["read", id]);
      throw new Error("synthetic case read failure");
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

  const pendingList = facade.loadCases();
  await assert.rejects(facade.loadOwnCase(caseBId), {
    message: "synthetic case read failure",
  });
  assert.equal(facade.getSnapshot().loading, false);
  assert.equal(facade.getSnapshot().error, "synthetic case read failure");
  assert.equal(facade.getSnapshot().action, null);

  resolveList([ownerCase(firstId)]);
  await assert.rejects(pendingList, { name: "AbortError" });
  assert.equal(facade.getSnapshot().loading, false);
  assert.equal(facade.getSnapshot().error, "synthetic case read failure");
  assert.deepEqual(facade.getSnapshot().cases, []);
  assert.equal(facade.getSnapshot().selected, null);
  assert.deepEqual(calls, [["list"], ["read", caseBId]]);
  facade.dispose();
});

test("successful create, submit, and upload supersede an in-flight case list", async (t) => {
  const caseId = "880e8400-e29b-41d4-a716-446655440030";
  const front = {
    ...ownerFile,
    id: "770e8400-e29b-41d4-a716-446655440032",
    state: "limpio",
    evidence_role: "identity_front",
  };
  const back = {
    ...ownerFile,
    id: "770e8400-e29b-41d4-a716-446655440033",
    state: "limpio",
    evidence_role: "identity_back",
  };
  const selfie = {
    ...ownerFile,
    id: "770e8400-e29b-41d4-a716-446655440034",
    state: "limpio",
    evidence_role: "identity_selfie",
  };
  const initial = {
    id: caseId,
    purpose: "identity",
    state: "borrador",
    revision: 1,
    files: [front, back, selfie],
  };
  const created = { ...initial, revision: 1, files: [] };
  const submitted = { ...initial, state: "enviada", revision: 2 };
  const uploadedFile = {
    ...ownerFile,
    id: "770e8400-e29b-41d4-a716-446655440031",
    evidence_role: "identity_front",
  };
  const uploaded = { revision: 2, etag: '"2"', file: uploadedFile };
  const expectedUpload = {
    ...initial,
    revision: 2,
    files: [back, selfie, uploadedFile],
  };
  const file = new File(["synthetic identity image"], "front.png", {
    type: "image/png",
  });
  const scenarios = [
    {
      name: "create",
      result: { value: created, etag: '"1"' },
      invoke: (facade) => facade.create({ purpose: "identity" }),
      expectedCalls: [["list"], ["create", { purpose: "identity" }]],
      expectedValue: created,
    },
    {
      name: "submit",
      result: { value: submitted, etag: '"2"' },
      invoke: (facade) => facade.submit(caseId),
      expectedCalls: [["read", caseId], ["list"], ["submit", caseId, '"1"']],
      expectedValue: submitted,
    },
    {
      name: "upload",
      result: uploaded,
      invoke: (facade) => facade.upload(caseId, file, "identity_front"),
      expectedCalls: [
        ["read", caseId],
        ["list"],
        ["upload", caseId, '"1"', file, "identity_front"],
      ],
      expectedValue: expectedUpload,
    },
  ];

  for (const scenario of scenarios) {
    await t.test(scenario.name, async () => {
      let resolveList;
      let resolveMutation;
      let signalMutationEntered;
      const mutationEntered = new Promise((resolve) => {
        signalMutationEntered = resolve;
      });
      const calls = [];
      const port = {
        list: () => {
          calls.push(["list"]);
          return new Promise((resolve) => {
            resolveList = resolve;
          });
        },
        create: (_token, input) => {
          calls.push(["create", input]);
          signalMutationEntered();
          return new Promise((resolve) => {
            resolveMutation = resolve;
          });
        },
        getOwnCase: async (_token, id) => {
          calls.push(["read", id]);
          return { value: initial, etag: '"1"' };
        },
        submit: (_token, id, etag) => {
          calls.push(["submit", id, etag]);
          signalMutationEntered();
          return new Promise((resolve) => {
            resolveMutation = resolve;
          });
        },
        upload: (_token, id, etag, sentFile, role) => {
          calls.push(["upload", id, etag, sentFile, role]);
          signalMutationEntered();
          return new Promise((resolve) => {
            resolveMutation = resolve;
          });
        },
        downloadSpaceFile: async () => {
          throw new Error("unused");
        },
      };
      const facade = new DocumentVerificationFacade(port, makeAuth());

      if (scenario.name !== "create") await facade.loadOwnCase(caseId);
      const pendingList = facade.loadCases();
      const pendingMutation = scenario.invoke(facade);
      await mutationEntered;
      assert.deepEqual(calls, scenario.expectedCalls);

      resolveMutation(scenario.result);
      await pendingMutation;
      assert.equal(facade.getSnapshot().loading, false);
      assert.equal(facade.getSnapshot().action, null);
      assert.deepEqual(facade.getSnapshot().cases, [scenario.expectedValue]);

      resolveList([ownerCase(secondId, 99)]);
      await assert.rejects(pendingList, { name: "AbortError" });
      assert.equal(facade.getSnapshot().loading, false);
      assert.deepEqual(facade.getSnapshot().cases, [scenario.expectedValue]);
      facade.dispose();
    });
  }
});
