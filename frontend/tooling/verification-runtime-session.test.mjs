import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-session-test-"),
);

async function loadBinding() {
  let source = await readFile(
    path.join(
      root,
      "src/features/verifications/application/verification-session.ts",
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
  const file = path.join(scratch, "verification-session.mjs");
  await writeFile(file, outputText);
  return (await import(pathToFileURL(file).href)).bindVerificationSession;
}

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

function createFakeAuth(initial) {
  let current = { ...initial };
  const listeners = new Set();
  const calls = { checkSession: 0, logout: 0 };
  return {
    calls,
    getSnapshot: () => current,
    setSnapshot: (next) => {
      current = { ...next };
      for (const listener of listeners) listener();
    },
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    checkSession: async () => {
      calls.checkSession += 1;
      return current.status === "authenticated";
    },
    logout: async () => {
      calls.logout += 1;
    },
  };
}

function createFakeFacade(overrides = {}) {
  const calls = {
    clear: 0,
    cancel: 0,
    loadOverview: 0,
    loadCaseDetails: 0,
    startVerification: 0,
    retryVerification: 0,
    uploadEvidence: 0,
    deleteEvidence: 0,
    getEvidenceContent: 0,
  };
  return {
    calls,
    getSnapshot: () => ({ verifications: [], loading: false, error: null }),
    getServerSnapshot: () => ({
      verifications: [],
      loading: false,
      error: null,
    }),
    subscribe: () => () => {},
    clear: () => {
      calls.clear += 1;
    },
    cancel: () => {
      calls.cancel += 1;
    },
    loadOverview: async () => {
      calls.loadOverview += 1;
      return { verifications: [] };
    },
    loadCaseDetails: async (id) => {
      calls.loadCaseDetails += 1;
      return { id };
    },
    startVerification: async (key, type) => {
      calls.startVerification += 1;
      return { id: "case-1", type };
    },
    retryVerification: async (id) => {
      calls.retryVerification += 1;
      return { id };
    },
    uploadEvidence: async (id) => {
      calls.uploadEvidence += 1;
      return { id: "ev-1", caseId: id };
    },
    deleteEvidence: async () => {
      calls.deleteEvidence += 1;
    },
    getEvidenceContent: async () => {
      calls.getEvidenceContent += 1;
      return new Blob(["ok"]);
    },
    ...overrides,
  };
}

test("bindVerificationSession: authenticated A -> B triggers facade.clear()", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const facade = createFakeFacade();
  bindVerificationSession(auth, facade);

  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-B",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });

  assert.equal(facade.calls.clear, 1);
});

test("bindVerificationSession: auth -> anonymous triggers facade.clear()", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const facade = createFakeFacade();
  bindVerificationSession(auth, facade);

  auth.setSnapshot({
    status: "anonymous",
    accountId: null,
    roles: [],
    capabilities: [],
    restrictedMode: false,
    expiresAt: null,
  });

  assert.equal(facade.calls.clear, 1);
});

test("bindVerificationSession: roles or capabilities change triggers facade.clear()", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const facade = createFakeFacade();
  bindVerificationSession(auth, facade);

  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user", "host"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });

  assert.equal(facade.calls.clear, 1);
});

test("bindVerificationSession: restrictedMode or expiry change triggers facade.clear()", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const facade = createFakeFacade();
  bindVerificationSession(auth, facade);

  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: true,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  assert.equal(facade.calls.clear, 1);

  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: true,
    expiresAt: "2027-01-01T00:00:00Z",
  });
  assert.equal(facade.calls.clear, 2);
});

test("bindVerificationSession: methods remain bound when detached", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const facade = createFakeFacade();
  const runtime = bindVerificationSession(auth, facade);
  const { subscribe, getSnapshot, getServerSnapshot } = runtime;

  assert.doesNotThrow(() => subscribe(() => {}));
  assert.ok(getSnapshot());
  assert.ok(getServerSnapshot());
});

test("bindVerificationSession: anonymous or checking operations reject without calling facade ports", async () => {
  const bindVerificationSession = await loadBinding();
  for (const status of ["anonymous", "checking"]) {
    const auth = createFakeAuth({
      status,
      accountId: null,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: null,
    });
    const facade = createFakeFacade();
    const runtime = bindVerificationSession(auth, facade);

    await assert.rejects(() => runtime.loadOverview());
    await assert.rejects(() => runtime.loadCaseDetails("case-1"));
    await assert.rejects(() => runtime.startVerification("key-1", "identity"));
    await assert.rejects(() =>
      runtime.retryVerification("case-1", "key-2", "new_document"),
    );
    await assert.rejects(() => runtime.uploadEvidence("case-1"));
    await assert.rejects(() => runtime.deleteEvidence("case-1", "ev-1"));
    await assert.rejects(() => runtime.getEvidenceContent("case-1", "ev-1"));

    assert.equal(facade.calls.loadOverview, 0);
    assert.equal(facade.calls.loadCaseDetails, 0);
    assert.equal(facade.calls.startVerification, 0);
    assert.equal(facade.calls.retryVerification, 0);
    assert.equal(facade.calls.uploadEvidence, 0);
    assert.equal(facade.calls.deleteEvidence, 0);
    assert.equal(facade.calls.getEvidenceContent, 0);
  }
});

test("bindVerificationSession: confirmed authenticated delegates to facade", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-confirmed",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const facade = createFakeFacade();
  const runtime = bindVerificationSession(auth, facade);

  const overview = await runtime.loadOverview();
  assert.deepEqual(overview, { verifications: [] });
  assert.equal(facade.calls.loadOverview, 1);

  const created = await runtime.startVerification("k-1", "identity");
  assert.equal(created.id, "case-1");
  assert.equal(facade.calls.startVerification, 1);

  const blob = await runtime.getEvidenceContent("case-1", "ev-1");
  assert.ok(blob);
  assert.equal(facade.calls.getEvidenceContent, 1);
});

test("bindVerificationSession: 403 purges facade & blobs synchronously, calls checkSession without logout, and rethrows 403 when session unchanged", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-confirmed",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  let purgedBlobs = 0;
  let facadeClearAtCheck = null;
  let blobsPurgedAtCheck = null;
  auth.checkSession = async () => {
    auth.calls.checkSession += 1;
    facadeClearAtCheck = facade.calls.clear;
    blobsPurgedAtCheck = purgedBlobs;
    return true;
  };
  const err403 = new Error("Forbidden");
  err403.status = 403;
  const facade = createFakeFacade({
    loadOverview: async () => {
      facade.calls.loadOverview += 1;
      throw err403;
    },
  });
  const runtime = bindVerificationSession(auth, facade, () => {
    purgedBlobs += 1;
  });

  await assert.rejects(
    () => runtime.loadOverview(),
    (err) => err === err403 && err.status === 403,
  );
  assert.equal(facade.calls.clear, 1);
  assert.equal(facadeClearAtCheck, 1);
  assert.equal(purgedBlobs, 1);
  assert.equal(blobsPurgedAtCheck, 1);
  assert.equal(auth.calls.checkSession, 1);
  assert.equal(auth.calls.logout, 0);
});

test("bindVerificationSession: 401 purges facade & blobs, calls checkSession, and throws AbortError when session becomes anonymous", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-confirmed",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  let purgedBlobs = 0;
  auth.checkSession = async () => {
    auth.calls.checkSession += 1;
    auth.setSnapshot({
      status: "anonymous",
      accountId: null,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: null,
    });
    return false;
  };
  const err401 = new Error("Unauthorized");
  err401.status = 401;
  const facade = createFakeFacade({
    uploadEvidence: async () => {
      facade.calls.uploadEvidence += 1;
      throw err401;
    },
  });
  const runtime = bindVerificationSession(auth, facade, () => {
    purgedBlobs += 1;
  });

  await assert.rejects(() => runtime.uploadEvidence("case-1"), {
    name: "AbortError",
  });
  assert.equal(facade.calls.clear >= 1, true);
  assert.equal(purgedBlobs >= 1, true);
  assert.equal(auth.calls.checkSession, 1);
  assert.equal(auth.calls.logout, 0);
});

test("bindVerificationSession: 503 does NOT purge facade or blobs, does NOT call checkSession, and rethrows 503", async () => {
  const bindVerificationSession = await loadBinding();
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-confirmed",
    roles: ["user"],
    capabilities: ["read"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  let purgedBlobs = 0;
  const err503 = new Error("Service Unavailable");
  err503.status = 503;
  const facade = createFakeFacade({
    startVerification: async () => {
      facade.calls.startVerification += 1;
      throw err503;
    },
  });
  const runtime = bindVerificationSession(auth, facade, () => {
    purgedBlobs += 1;
  });

  await assert.rejects(
    () => runtime.startVerification("k-1", "identity"),
    (err) => err === err503 && err.status === 503,
  );
  assert.equal(facade.calls.clear, 0);
  assert.equal(purgedBlobs, 0);
  assert.equal(auth.calls.checkSession, 0);
  assert.equal(auth.calls.logout, 0);
});
