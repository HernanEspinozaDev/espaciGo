import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-account-facade-"),
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
      "  clear: () => {",
      "    blobEvents.push({ type: 'clear' });",
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

test("loadOverview tardío tras clear() no debe sobrescribir snapshot limpio", async () => {
  const AccountFacade = await loadFacade();
  let resolveProfile;
  const profilePromise = new Promise((res) => {
    resolveProfile = res;
  });
  let resolvePayout;
  const payoutPromise = new Promise((res) => {
    resolvePayout = res;
  });
  let resolveRights;
  const rightsPromise = new Promise((res) => {
    resolveRights = res;
  });

  const fakePort = {
    getProfile: () => profilePromise,
    getProfilePhoto: async () => null,
    getPayoutAccount: () => payoutPromise,
    listRightsRequests: () => rightsPromise,
  };

  const facade = new AccountFacade(fakePort);
  const pending = facade.loadOverview();
  facade.clear();

  resolveProfile({ id: "user-1", displayName: "Perfil A" });
  resolvePayout({ id: "payout-1", status: "ACTIVE" });
  resolveRights([]);
  await pending;

  assert.equal(facade.getSnapshot().profile, null);
});

test("exportIdentityBlob tardío tras clear() no retorna Blob privado", async () => {
  const AccountFacade = await loadFacade();
  let resolveBlob;
  const blobPromise = new Promise((res) => {
    resolveBlob = res;
  });
  const fakePort = {
    exportIdentityBlob: () => blobPromise,
  };

  const facade = new AccountFacade(fakePort);
  const pending = facade.exportIdentityBlob();
  facade.clear();

  const privateBlob = new Blob(["datos-privados-identidad"]);
  resolveBlob(privateBlob);

  let result;
  let rejectedError = null;
  try {
    result = await pending;
  } catch (err) {
    rejectedError = err;
  }

  if (rejectedError) {
    assert.equal(rejectedError.name, "AbortError");
  } else {
    assert.equal(result, null);
  }
});

test("loadOverview con foto tardía tras clear() no crea Blob URL ni repuebla snapshot", async () => {
  const AccountFacade = await loadFacade();
  AccountFacade.blobEvents.length = 0;

  let resolveProfile;
  const profilePromise = new Promise((res) => {
    resolveProfile = res;
  });
  let resolvePhotoContent;
  const photoContentPromise = new Promise((res) => {
    resolvePhotoContent = res;
  });

  const fakePort = {
    getProfile: () => profilePromise,
    getProfilePhoto: async () => ({ id: "photo-1" }),
    getPayoutAccount: async () => null,
    listRightsRequests: async () => [],
    getProfilePhotoContent: () => photoContentPromise,
  };

  const facade = new AccountFacade(fakePort);
  const pending = facade.loadOverview();

  resolveProfile({ id: "user-1", displayName: "Perfil A" });
  await new Promise((res) => setImmediate(res));

  const eventsAfterClear = AccountFacade.blobEvents.length;
  facade.clear();

  const privatePhotoBlob = new Blob(["foto-binaria"]);
  resolvePhotoContent(privatePhotoBlob);
  await pending;

  assert.equal(facade.getSnapshot().profile, null);
  assert.equal(facade.getSnapshot().photo, null);
  assert.equal(facade.getSnapshot().photoUrl, null);

  const createsAfterClear = AccountFacade.blobEvents
    .slice(eventsAfterClear)
    .filter((e) => e.type === "create");
  assert.equal(createsAfterClear.length, 0);
});

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
    emit: (next) => {
      snapshot = { ...next };
      for (const fn of listeners) fn();
    },
    checkSession: async () => {
      checkSessionCalls++;
      return true;
    },
    getCheckSessionCalls: () => checkSessionCalls,
  };
}

const authenticatedUserA = Object.freeze({
  status: "authenticated",
  accountId: "user-a",
  roles: Object.freeze(["guest", "host"]),
  capabilities: Object.freeze(["booking:create", "listing:manage"]),
  restrictedMode: false,
  expiresAt: "2026-10-26T20:00:00Z",
});

const authenticatedUserB = Object.freeze({
  status: "authenticated",
  accountId: "user-b",
  roles: Object.freeze(["guest"]),
  capabilities: Object.freeze(["booking:create"]),
  restrictedMode: false,
  expiresAt: "2026-10-26T20:00:00Z",
});

const anonymousSnapshot = Object.freeze({
  status: "anonymous",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});

const checkingSnapshot = Object.freeze({
  status: "checking",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});

test("A→B purga snapshot existente y ejecuta onPrivatePurge", async () => {
  const AccountFacade = await loadFacade();
  let purges = 0;
  const authSource = createMockAuthSource(authenticatedUserA);
  const fakePort = {
    getProfile: async () => ({ id: "user-a", displayName: "User A" }),
    getProfilePhoto: async () => null,
    getPayoutAccount: async () => null,
    listRightsRequests: async () => [],
  };

  const facade = new AccountFacade(fakePort, authSource, () => {
    purges++;
  });
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");

  authSource.emit(authenticatedUserB);
  assert.equal(facade.getSnapshot().profile, null);
  assert.ok(purges > 0, "onPrivatePurge debe ser invocado al cambiar A->B");
});

test("roles, capabilities, restrictedMode y expiry distintos purgan la sesión", async () => {
  const AccountFacade = await loadFacade();
  const authSource = createMockAuthSource(authenticatedUserA);
  const fakePort = {
    getProfile: async () => ({ id: "user-a", displayName: "User A" }),
    getProfilePhoto: async () => null,
    getPayoutAccount: async () => null,
    listRightsRequests: async () => [],
  };
  const facade = new AccountFacade(fakePort, authSource);

  // 1. Roles distintos
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  authSource.emit({ ...authenticatedUserA, roles: ["admin"] });
  assert.equal(
    facade.getSnapshot().profile,
    null,
    "Roles distintos deben purgar",
  );

  // 2. Capabilities distintas
  authSource.emit(authenticatedUserA);
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  authSource.emit({ ...authenticatedUserA, capabilities: ["audit:all"] });
  assert.equal(
    facade.getSnapshot().profile,
    null,
    "Capabilities distintas deben purgar",
  );

  // 3. restrictedMode distinto
  authSource.emit(authenticatedUserA);
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  authSource.emit({ ...authenticatedUserA, restrictedMode: true });
  assert.equal(
    facade.getSnapshot().profile,
    null,
    "restrictedMode distinto debe purgar",
  );

  // 4. expiry distinto
  authSource.emit(authenticatedUserA);
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  authSource.emit({ ...authenticatedUserA, expiresAt: "2026-11-01T00:00:00Z" });
  assert.equal(
    facade.getSnapshot().profile,
    null,
    "expiry distinto debe purgar",
  );

  // 5. logout / anonymous
  authSource.emit(authenticatedUserA);
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  authSource.emit(anonymousSnapshot);
  assert.equal(
    facade.getSnapshot().profile,
    null,
    "logout / anonymous debe purgar",
  );
});

test("emisión de mismo contenido de sesión NO purga el snapshot", async () => {
  const AccountFacade = await loadFacade();
  let purges = 0;
  const authSource = createMockAuthSource(authenticatedUserA);
  const fakePort = {
    getProfile: async () => ({ id: "user-a", displayName: "User A" }),
    getProfilePhoto: async () => null,
    getPayoutAccount: async () => null,
    listRightsRequests: async () => [],
  };
  const facade = new AccountFacade(fakePort, authSource, () => {
    purges++;
  });

  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");

  // Emit same snapshot content again
  authSource.emit({ ...authenticatedUserA });
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  assert.equal(purges, 0, "Mismo contenido no debe disparar purge");
});

test("checking y anonymous nofetch: no invocan puertos de loadOverview ni exportIdentityBlob", async () => {
  const AccountFacade = await loadFacade();
  for (const statusSnapshot of [checkingSnapshot, anonymousSnapshot]) {
    let fetchCalls = 0;
    const fakePort = {
      getProfile: async () => {
        fetchCalls++;
        return null;
      },
      getProfilePhoto: async () => {
        fetchCalls++;
        return null;
      },
      getPayoutAccount: async () => {
        fetchCalls++;
        return null;
      },
      listRightsRequests: async () => {
        fetchCalls++;
        return [];
      },
      exportIdentityBlob: async () => {
        fetchCalls++;
        return null;
      },
    };

    const authSource = createMockAuthSource(statusSnapshot);
    const facade = new AccountFacade(fakePort, authSource);

    const overview = await facade.loadOverview();
    const blob = await facade.exportIdentityBlob();

    assert.equal(
      fetchCalls,
      0,
      `No debe invocar puertos en estado ${statusSnapshot.status}`,
    );
    assert.equal(overview.profile, null);
    assert.equal(facade.getSnapshot().profile, null);
    assert.equal(blob, null);
  }
});

test("401/403 purga síncrono y revalida con checkSession sin logout, 503 no invalida ni revalida", async () => {
  const AccountFacade = await loadFacade();
  let errorToThrow = null;
  let fetchCalls = 0;
  const fakePort = {
    getProfile: async () => {
      fetchCalls++;
      if (errorToThrow) throw errorToThrow;
      return { id: "user-a", displayName: "User A" };
    },
    getProfilePhoto: async () => null,
    getPayoutAccount: async () => null,
    listRightsRequests: async () => [],
  };

  const authSource = createMockAuthSource(authenticatedUserA);
  let purges = 0;
  const facade = new AccountFacade(fakePort, authSource, () => {
    purges++;
  });

  // 1. Carga inicial exitosa
  await facade.loadOverview();
  assert.equal(fetchCalls, 1);
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  assert.equal(authSource.getCheckSessionCalls(), 0);

  // 2. Error 401: purga síncrona y checkSession, pero sin logout de authSource
  errorToThrow = { status: 401, message: "Unauthorized" };
  try {
    await facade.loadOverview();
  } catch {
    // capturamos error
  }
  assert.equal(facade.getSnapshot().profile, null, "401 debe purgar snapshot");
  assert.equal(
    authSource.getCheckSessionCalls(),
    1,
    "401 debe invocar checkSession",
  );
  assert.equal(
    authSource.getSnapshot().status,
    "authenticated",
    "401 no debe provocar logout directo en authSource",
  );
  assert.ok(purges > 0, "401 debe invocar onPrivatePurge");

  // 3. Error 503: NO llama a checkSession ni purga
  authSource.emit(authenticatedUserA);
  errorToThrow = null;
  await facade.loadOverview();
  assert.equal(facade.getSnapshot().profile?.id, "user-a");
  const checkCallsBefore503 = authSource.getCheckSessionCalls();
  const purgesBefore503 = purges;

  errorToThrow = { status: 503, message: "Service Unavailable" };
  try {
    await facade.loadOverview();
  } catch {
    // capturamos error
  }
  assert.equal(
    authSource.getCheckSessionCalls(),
    checkCallsBefore503,
    "503 no debe invocar checkSession",
  );
  assert.equal(purges, purgesBefore503, "503 no debe purgar");
  assert.equal(facade.getSnapshot().error, "Service Unavailable");
});
