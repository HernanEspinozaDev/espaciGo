import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-facade-"),
);

async function loadFacade() {
  let source = await readFile(
    path.join(
      root,
      "src/features/verifications/application/verification-facade.ts",
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
  return (await import(pathToFileURL(file).href)).VerificationFacade;
}

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("loadOverview tardío tras clear() no debe sobrescribir snapshot limpio", async () => {
  const VerificationFacade = await loadFacade();
  let resolveList;
  const listPromise = new Promise((res) => {
    resolveList = res;
  });
  const fakePort = {
    listVerifications: () => listPromise,
    getEligibility: async () => [],
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.loadOverview();
  facade.clear();

  resolveList([{ id: "case-1", status: "PENDING" }]);
  await pending;

  assert.equal(facade.getSnapshot().verifications.length, 0);
});

test("loadCaseDetails tardío tras clear() no debe sobrescribir snapshot limpio ni retornar caso", async () => {
  const VerificationFacade = await loadFacade();
  let resolveCase;
  const casePromise = new Promise((res) => {
    resolveCase = res;
  });
  const fakePort = {
    getVerification: () => casePromise,
    listHistory: async () => [],
    listEvidence: async () => [],
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.loadCaseDetails("case-1");
  facade.clear();

  resolveCase({ id: "case-1", status: "PENDING" });
  const result = await pending;

  assert.equal(result, null);
  assert.equal(facade.getSnapshot().currentCase, null);
  assert.equal(facade.getSnapshot().loading, false);
});

test("getEvidenceContent tardío tras clear() no debe retornar contenido privado", async () => {
  const VerificationFacade = await loadFacade();
  let resolveBlob;
  const blobPromise = new Promise((res) => {
    resolveBlob = res;
  });
  const fakePort = {
    getEvidenceContent: () => blobPromise,
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.getEvidenceContent("case-1", "ev-1");
  facade.clear();

  resolveBlob(new Blob(["private-data"]));
  const result = await pending;

  assert.equal(result, null);
});

test("startVerification tardío tras clear() rechaza con AbortError y no comitea snapshot", async () => {
  const VerificationFacade = await loadFacade();
  let resolveStart;
  const startPromise = new Promise((res) => {
    resolveStart = res;
  });
  const fakePort = {
    startVerification: () => startPromise,
    listVerifications: async () => [],
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.startVerification("key-1", "IDENTITY");
  facade.clear();

  resolveStart({ id: "case-1", status: "PENDING" });
  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().verifications.length, 0);
  assert.equal(facade.getSnapshot().currentCase, null);
  assert.equal(facade.getSnapshot().loading, false);
});

test("uploadEvidence tardío tras clear() rechaza con AbortError y no comitea snapshot", async () => {
  const VerificationFacade = await loadFacade();
  let resolveUpload;
  const uploadPromise = new Promise((res) => {
    resolveUpload = res;
  });
  const fakePort = {
    uploadEvidence: () => uploadPromise,
    listEvidence: async () => [],
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.uploadEvidence("case-1");
  facade.clear();

  resolveUpload({
    id: "ev-1",
    verificationCaseId: "case-1",
    documentType: "PASSPORT",
    storageRef: "ref-1",
    status: "VALID",
  });
  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().evidence.length, 0);
  assert.equal(facade.getSnapshot().loading, false);
});

test("retryVerification tardío tras clear() rechaza con AbortError y no comitea snapshot", async () => {
  const VerificationFacade = await loadFacade();
  let resolveRetry;
  const retryPromise = new Promise((res) => {
    resolveRetry = res;
  });
  const fakePort = {
    retryVerification: () => retryPromise,
    listVerifications: async () => [],
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.retryVerification(
    "case-1",
    "idemp-2",
    "NAME_MISMATCH",
  );
  facade.clear();

  resolveRetry({ id: "case-1", status: "PENDING" });
  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().verifications.length, 0);
  assert.equal(facade.getSnapshot().currentCase, null);
  assert.equal(facade.getSnapshot().loading, false);
});

test("deleteEvidence tardío tras clear() rechaza con AbortError y no comitea snapshot", async () => {
  const VerificationFacade = await loadFacade();
  let resolveDelete;
  const deletePromise = new Promise((res) => {
    resolveDelete = res;
  });
  const fakePort = {
    deleteEvidence: () => deletePromise,
    listEvidence: async () => [],
  };

  const facade = new VerificationFacade(fakePort);
  const pending = facade.deleteEvidence("case-1", "ev-1");
  facade.clear();

  resolveDelete();
  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().evidence.length, 0);
  assert.equal(facade.getSnapshot().loading, false);
});
