import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-session-identity-"),
);

async function loadSessionIdentityStore() {
  let source = await readFile(
    path.join(root, "src/features/auth/application/session-identity.ts"),
    "utf8",
  );
  source = source.replace('import "client-only";', "");
  source = source.replace('"use client";', "");

  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  });
  const file = path.join(
    scratch,
    `session-identity-${Date.now()}-${Math.random().toString(36).slice(2)}.mjs`,
  );
  await writeFile(file, outputText);
  const mod = await import(pathToFileURL(file).href);
  return mod.SessionIdentityStore;
}

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

function createFakeAuthSource(initialSnapshot) {
  let snapshot = initialSnapshot;
  const listeners = new Set();
  let checkSessionCalls = 0;

  return {
    getSnapshot: () => snapshot,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    setSnapshot: (next) => {
      snapshot = next;
      for (const l of listeners) l();
    },
    checkSession: async () => {
      checkSessionCalls++;
      return true;
    },
    getCheckSessionCalls: () => checkSessionCalls,
  };
}

function createFakeProfileSource(initialProfile = null) {
  let snapshot = { profile: initialProfile };
  const listeners = new Set();

  return {
    getSnapshot: () => snapshot,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    setProfile: (profile) => {
      snapshot = { profile };
      for (const l of listeners) l();
    },
  };
}

const authenticatedAuthSnapshotA = Object.freeze({
  status: "authenticated",
  accountId: "acc_A",
  roles: Object.freeze(["user"]),
  capabilities: Object.freeze(["rent"]),
  restrictedMode: false,
  expiresAt: "2026-12-31T23:59:59Z",
});

const authenticatedAuthSnapshotB = Object.freeze({
  status: "authenticated",
  accountId: "acc_B",
  roles: Object.freeze(["user"]),
  capabilities: Object.freeze(["rent"]),
  restrictedMode: false,
  expiresAt: "2026-12-31T23:59:59Z",
});

const checkingAuthSnapshot = Object.freeze({
  status: "checking",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});

const anonymousAuthSnapshot = Object.freeze({
  status: "anonymous",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});

test("perfil 404 ID: fallback exact accountId cuando perfil es 404 o no existe", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource(null);

  // 1. getProfile throws 404
  const store1 = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => {
        const err = new Error("Not found");
        err.status = 404;
        throw err;
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store1.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "acc_A",
  });
  store1.dispose();

  // 2. getProfile returns null (OpenAPI 404 mapping)
  const store2 = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => null,
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store2.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "acc_A",
  });
  store2.dispose();
});

test("200 name: actualiza label con displayName de Go (o fallback si vacío)", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource(null);

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => ({
        displayName: "  Hernan Espinoza  ",
        phone: "+56912345678",
        updatedAt: "2026-05-01T12:00:00Z",
      }),
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "Hernan Espinoza",
  });
  store.dispose();

  // Empty displayName fallback to accountId
  const storeEmpty = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => ({
        displayName: "   ",
        phone: null,
        updatedAt: "2026-05-01T12:00:00Z",
      }),
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(storeEmpty.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "acc_A",
  });
  storeEmpty.dispose();
});

test("PUT profileSource cambio nombre: actualiza reactivamente sin recarga", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource(null);

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => ({
        displayName: "Nombre Inicial",
        phone: null,
        updatedAt: "2026-05-01T12:00:00Z",
      }),
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(store.getSnapshot().label, "Nombre Inicial");

  let listenerNotified = 0;
  store.subscribe(() => {
    listenerNotified++;
  });

  // Simular PUT exitoso en accountFacade
  profileSource.setProfile({
    displayName: "Nombre Actualizado",
    phone: null,
    updatedAt: "2026-05-01T13:00:00Z",
  });

  assert.equal(store.getSnapshot().label, "Nombre Actualizado");
  assert.ok(listenerNotified > 0);
  store.dispose();
});

test("late GET no vuelve a nombre viejo tras PUT", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource(null);

  let resolveLateGet;
  const lateGetPromise = new Promise((resolve) => {
    resolveLateGet = resolve;
  });

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: () => lateGetPromise,
    },
    profileSource,
  );

  // Antes de que termine el GET, se guarda el perfil vía PUT
  profileSource.setProfile({
    displayName: "Nombre Guardado PUT",
    phone: null,
    updatedAt: "2026-05-01T14:00:00Z",
  });
  assert.equal(store.getSnapshot().label, "Nombre Guardado PUT");

  // Ahora termina el GET demorado con el nombre viejo
  resolveLateGet({
    displayName: "Nombre Viejo GET",
    phone: null,
    updatedAt: "2026-05-01T10:00:00Z",
  });
  await new Promise((r) => setTimeout(r, 15));

  // No debe revertir al nombre viejo
  assert.equal(store.getSnapshot().label, "Nombre Guardado PUT");
  store.dispose();
});

test("A -> B / logout: cambio de actor y logout limpian y aíslan la identidad", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource({
    displayName: "Actor A",
    phone: null,
    updatedAt: "2026-05-01T12:00:00Z",
  });

  let getProfileCalledWith = [];
  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => {
        const accId = authSource.getSnapshot().accountId;
        getProfileCalledWith.push(accId);
        return {
          displayName: `Nombre de ${accId}`,
          phone: null,
          updatedAt: "2026-05-01T12:00:00Z",
        };
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(store.getSnapshot().accountId, "acc_A");
  assert.equal(store.getSnapshot().label, "Nombre de acc_A");

  // Cambio de actor a acc_B
  profileSource.setProfile(null);
  authSource.setSnapshot(authenticatedAuthSnapshotB);
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(store.getSnapshot().accountId, "acc_B");
  assert.equal(store.getSnapshot().label, "Nombre de acc_B");

  // Logout
  authSource.setSnapshot(anonymousAuthSnapshot);
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store.getSnapshot(), {
    status: "anonymous",
    accountId: null,
    label: null,
  });
  store.dispose();
});

test("503 / 401 / 403: manejo de fallos sin logout arbitrario ni exposición privada", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const profileSource = createFakeProfileSource(null);

  // 1. 503 Service Unavailable: fallback ID, no checkSession
  const authSource503 = createFakeAuthSource(authenticatedAuthSnapshotA);
  const store503 = new SessionIdentityStore(
    authSource503,
    {
      getProfile: async () => {
        const err = new Error("Service Unavailable");
        err.status = 503;
        throw err;
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store503.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "acc_A",
  });
  assert.equal(authSource503.getCheckSessionCalls(), 0);
  store503.dispose();

  // 2. 401 Unauthorized: purga/revalida Go best-effort vía checkSession, label fallback ID, sin errores privados en UI
  const authSource401 = createFakeAuthSource(authenticatedAuthSnapshotA);
  const store401 = new SessionIdentityStore(
    authSource401,
    {
      getProfile: async () => {
        const err = new Error("Unauthorized");
        err.status = 401;
        throw err;
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store401.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "acc_A",
  });
  assert.equal(authSource401.getCheckSessionCalls(), 1);
  store401.dispose();

  // 3. 403 Forbidden: purga/revalida Go best-effort vía checkSession, label fallback ID
  const authSource403 = createFakeAuthSource(authenticatedAuthSnapshotA);
  const store403 = new SessionIdentityStore(
    authSource403,
    {
      getProfile: async () => {
        const err = new Error("Forbidden");
        err.status = 403;
        throw err;
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store403.getSnapshot(), {
    status: "authenticated",
    accountId: "acc_A",
    label: "acc_A",
  });
  assert.equal(authSource403.getCheckSessionCalls(), 1);
  store403.dispose();
});

test("checking sin peticiones privadas", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(checkingAuthSnapshot);
  const profileSource = createFakeProfileSource(null);
  let getProfileCalls = 0;

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => {
        getProfileCalls++;
        return null;
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(store.getSnapshot(), {
    status: "checking",
    accountId: null,
    label: null,
  });
  assert.equal(getProfileCalls, 0);
  store.dispose();
});

test("mismo AuthDTO no refetch loop", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource(null);
  let getProfileCalls = 0;

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => {
        getProfileCalls++;
        return {
          displayName: "Test",
          phone: null,
          updatedAt: "2026-05-01T12:00:00Z",
        };
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(getProfileCalls, 1);

  // Emitir el mismo AuthSnapshot varias veces (clones con mismos 6 campos)
  authSource.setSnapshot({ ...authenticatedAuthSnapshotA });
  authSource.setSnapshot({ ...authenticatedAuthSnapshotA });
  authSource.setSnapshot({
    ...authenticatedAuthSnapshotA,
    roles: [...authenticatedAuthSnapshotA.roles],
    capabilities: [...authenticatedAuthSnapshotA.capabilities],
  });
  await new Promise((r) => setTimeout(r, 10));

  // No debe volver a llamar a getProfile
  assert.equal(getProfileCalls, 1);
  store.dispose();
});

test("SSR snapshot: retorna checking estable con auth checking y authenticated (never serialize identidad server)", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const profileSource = createFakeProfileSource(null);

  // 1. Con auth checking
  const authSourceChecking = createFakeAuthSource(checkingAuthSnapshot);
  const storeChecking = new SessionIdentityStore(
    authSourceChecking,
    {
      getProfile: async () => null,
    },
    profileSource,
  );
  assert.deepEqual(storeChecking.getServerSnapshot(), {
    status: "checking",
    accountId: null,
    label: null,
  });
  storeChecking.dispose();

  // 2. Con auth authenticated: nunca serializa ID ni nombre en server
  const authSourceAuth = createFakeAuthSource(authenticatedAuthSnapshotA);
  const storeAuth = new SessionIdentityStore(
    authSourceAuth,
    {
      getProfile: async () => ({
        displayName: "Actor A",
        phone: null,
        updatedAt: "2026-05-01T12:00:00Z",
      }),
    },
    profileSource,
  );
  assert.deepEqual(storeAuth.getServerSnapshot(), {
    status: "checking",
    accountId: null,
    label: null,
  });
  storeAuth.dispose();
});

test("perfil 401/403: si checkSession cambia sincrónicamente a anonymous o B, no revive sesión A", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const profileSource = createFakeProfileSource(null);

  let authSource;
  authSource = {
    getSnapshot: () => currentAuth,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    setSnapshot: (next) => {
      currentAuth = next;
      for (const l of listeners) l();
    },
    checkSession: async () => {
      // Cambia sincrónicamente la fuente antes de resolver
      authSource.setSnapshot(anonymousAuthSnapshot);
      return false;
    },
  };
  let currentAuth = authenticatedAuthSnapshotA;
  const listeners = new Set();

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => {
        const err = new Error("Unauthorized");
        err.status = 401;
        throw err;
      },
    },
    profileSource,
  );

  await new Promise((r) => setTimeout(r, 20));
  // No debe revivir A tras el cambio sincrónico provocado por checkSession
  assert.deepEqual(store.getSnapshot(), {
    status: "anonymous",
    accountId: null,
    label: null,
  });
  store.dispose();
});

test("A -> B: si profileSource conserva cacheA, no muestra nombre de actor A en actor B", async () => {
  const SessionIdentityStore = await loadSessionIdentityStore();
  const authSource = createFakeAuthSource(authenticatedAuthSnapshotA);
  const profileSource = createFakeProfileSource({
    displayName: "Actor A",
    phone: null,
    updatedAt: "2026-05-01T12:00:00Z",
  });

  let resolveProfileB;
  const profileBPromise = new Promise((resolve) => {
    resolveProfileB = resolve;
  });

  const store = new SessionIdentityStore(
    authSource,
    {
      getProfile: async () => {
        const accId = authSource.getSnapshot().accountId;
        if (accId === "acc_B") {
          return profileBPromise;
        }
        return {
          displayName: "Actor A",
          phone: null,
          updatedAt: "2026-05-01T12:00:00Z",
        };
      },
    },
    profileSource,
  );
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(store.getSnapshot().label, "Actor A");

  // Cambio de actor a B, pero profileSource AÚN retiene el perfil de A
  authSource.setSnapshot(authenticatedAuthSnapshotB);

  // Inmediatamente tras cambio de actor, no debe exhibir el nombre de A para B
  assert.equal(store.getSnapshot().accountId, "acc_B");
  assert.notEqual(store.getSnapshot().label, "Actor A");
  assert.equal(store.getSnapshot().label, "acc_B");

  // Notificación de profileSource sin cambio real (retiene cacheA) tampoco debe filtrar Actor A
  profileSource.setProfile({
    displayName: "Actor A",
    phone: null,
    updatedAt: "2026-05-01T12:00:00Z",
  });
  assert.equal(store.getSnapshot().label, "acc_B");

  // Cuando Go responde con el perfil de B, se actualiza a Actor B
  resolveProfileB({
    displayName: "Actor B",
    phone: null,
    updatedAt: "2026-05-01T14:00:00Z",
  });
  await new Promise((r) => setTimeout(r, 15));
  assert.equal(store.getSnapshot().label, "Actor B");
  store.dispose();
});
