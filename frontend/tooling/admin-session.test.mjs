import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(path.join(os.tmpdir(), "admin-session-test-"));

async function compile(sourcePath, outputName, replacements = []) {
  let source = await readFile(path.join(repoRoot, sourcePath), "utf8");
  for (const [before, after] of replacements) {
    source = source.replaceAll(before, after);
  }
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const destination = path.join(scratch, outputName);
  await writeFile(destination, output, "utf8");
  return destination;
}

await writeFile(
  path.join(scratch, "private-blob-urls.mjs"),
  "export const privateBlobUrls = { clearCount: 0, clear() { this.clearCount++; } };",
  "utf8",
);

await compile("src/features/admin/domain/admin-rules.ts", "admin-rules.mjs");

const facadePath = await compile(
  "src/features/admin/application/admin-facade.ts",
  "admin-facade.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/private-blob-urls", "./private-blob-urls.mjs"],
    ["../domain/admin-rules", "./admin-rules.mjs"],
  ],
);

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

function createDeferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function createMockAuthSource(initialSnapshot) {
  let snapshot = { ...initialSnapshot };
  const listeners = new Set();
  let checkSessionCalls = 0;
  return {
    getSnapshot: () => snapshot,
    subscribe: (fn) => {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    checkSession: async () => {
      checkSessionCalls++;
      return true;
    },
    getCheckSessionCalls: () => checkSessionCalls,
    setSnapshot: (next) => {
      snapshot = { ...next };
      for (const fn of listeners) fn();
    },
  };
}

const authenticatedActorA = Object.freeze({
  status: "authenticated",
  accountId: "actor-a-id",
  roles: Object.freeze(["administrador"]),
  capabilities: Object.freeze(["kpi.read"]),
  restrictedMode: false,
  expiresAt: "2026-10-27T00:00:00Z",
});

const authenticatedActorB = Object.freeze({
  status: "authenticated",
  accountId: "actor-b-id",
  roles: Object.freeze(["administrador"]),
  capabilities: Object.freeze(["kpi.read"]),
  restrictedMode: false,
  expiresAt: "2026-10-27T00:00:00Z",
});

const anonymousAuth = Object.freeze({
  status: "anonymous",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});

test("admin facade: purge borraMe and resets private state and flags", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  let customPurged = 0;
  const mockPort = {
    getGlobalAdminMe: async () => ({
      account_id: "actor-a-id",
      email: "actor@test.com",
      roles: ["administrador"],
      capabilities: ["kpi.read"],
    }),
  };
  const facade = new AdminFacade(mockPort, undefined, () => customPurged++);
  await facade.loadGlobalMe();
  assert.equal(facade.getSnapshot().me?.account_id, "actor-a-id");

  facade.purgePrivateData();
  const s = facade.getSnapshot();
  assert.equal(s.me, null, "me must be purged to null");
  assert.equal(s.overview, null);
  assert.equal(s.loading, false);
  assert.equal(s.actionLoading, false);
  assert.equal(customPurged, 1, "onPrivatePurge callback invoked");
});

test("admin facade: loadGlobalMe tardio tras purge no actorA", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  const deferred = createDeferred();
  const mockPort = {
    getGlobalAdminMe: () => deferred.promise,
  };
  const facade = new AdminFacade(mockPort);
  const promise = facade.loadGlobalMe();
  assert.equal(facade.getSnapshot().loading, true);

  // Purge while in-flight
  facade.purgePrivateData();
  assert.equal(facade.getSnapshot().me, null);
  assert.equal(facade.getSnapshot().loading, false);

  // Deferred resolves late with Actor A
  deferred.resolve({
    account_id: "actor-a-id",
    email: "actor-a@test.com",
    roles: ["administrador"],
    capabilities: ["kpi.read"],
  });
  await promise;

  assert.equal(
    facade.getSnapshot().me,
    null,
    "Actor A data must not be written after purge",
  );
});

test("admin facade: loadoverview tardio cambio A->B no data", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  const deferred = createDeferred();
  const mockPort = {
    getGlobalAdminOverview: () => deferred.promise,
  };
  const authSource = createMockAuthSource(authenticatedActorA);
  const facade = new AdminFacade(mockPort, authSource);

  const promise = facade.loadOverview();
  assert.equal(facade.getSnapshot().loading, true);

  // Session changes A -> B
  authSource.setSnapshot(authenticatedActorB);
  assert.equal(facade.getSnapshot().loading, false);
  assert.equal(facade.getSnapshot().overview, null);

  // Stale overview for A resolves
  deferred.resolve({
    total_accounts: 99,
    by_state: {},
    by_role: {},
  });
  await promise;

  assert.equal(
    facade.getSnapshot().overview,
    null,
    "Overview for A must not pollute session B",
  );
});

test("admin facade: anonymous no fetch", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  let fetchCalls = 0;
  const mockPort = {
    getGlobalAdminMe: async () => {
      fetchCalls++;
      return { account_id: "x" };
    },
    getGlobalAdminOverview: async () => {
      fetchCalls++;
      return { total_accounts: 1 };
    },
  };
  const authSource = createMockAuthSource(anonymousAuth);
  const facade = new AdminFacade(mockPort, authSource);

  await facade.loadGlobalMe();
  await facade.loadOverview();

  assert.equal(
    fetchCalls,
    0,
    "No fetch should occur when session is anonymous",
  );
  assert.equal(facade.getSnapshot().me, null);
  assert.equal(facade.getSnapshot().overview, null);
});

test("admin facade: 401/403 purga y revalida Go, 503 no asume logout", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  let errorToThrow = null;
  const mockPort = {
    getGlobalAdminOverview: async () => {
      if (errorToThrow) throw errorToThrow;
      return { total_accounts: 5, by_state: {}, by_role: {} };
    },
  };
  const authSource = createMockAuthSource(authenticatedActorA);
  const facade = new AdminFacade(mockPort, authSource);

  // Load initial data
  await facade.loadOverview();
  assert.notEqual(facade.getSnapshot().overview, null);

  // 401 error: purges private data and triggers checkSession (revalidate with Go)
  errorToThrow = { status: 401, code: "unauthorized" };
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().overview, null);
  assert.equal(authSource.getCheckSessionCalls(), 1);

  // 503 error: does NOT trigger checkSession, does NOT purge existing me if present
  errorToThrow = { status: 503, message: "Service Unavailable" };
  await facade.loadOverview();
  assert.equal(
    authSource.getCheckSessionCalls(),
    1,
    "503 must not call checkSession",
  );
  assert.equal(facade.getSnapshot().error, "Service Unavailable");
});

test("admin facade: KPI tardio tras purge no repuebla datos privados", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  const deferred = createDeferred();
  const facade = new AdminFacade({
    getGlobalAdminKPIs: () => deferred.promise,
  });
  const pending = facade.loadKPIs({
    from: "2026-10-01T00:00:00Z",
    until: "2026-10-02T00:00:00Z",
    time_zone: "America/Santiago",
  });
  facade.purgePrivateData();
  deferred.resolve({ synthetic: true });
  await pending;
  assert.equal(facade.getSnapshot().kpis, null);
  assert.equal(facade.getSnapshot().loading, false);
});

test("admin facade: KPI no consulta puertos sin sesion confirmada", async () => {
  const { AdminFacade } = await import(pathToFileURL(facadePath).href);
  let calls = 0;
  const facade = new AdminFacade(
    {
      getGlobalAdminKPIs: async () => {
        calls++;
        return {};
      },
    },
    createMockAuthSource(anonymousAuth),
  );
  await facade.loadKPIs({
    from: "2026-10-01T00:00:00Z",
    until: "2026-10-02T00:00:00Z",
    time_zone: "America/Santiago",
  });
  assert.equal(calls, 0);
});
