import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-account-remaining-"),
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

test("generateSyntheticPhoto diferido tras clear() no crea Blob URL ni repuebla snapshot y rechaza con AbortError", async () => {
  const AccountFacade = await loadFacade();
  AccountFacade.blobEvents.length = 0;

  let resolveGenerate;
  const generatePromise = new Promise((resolve) => {
    resolveGenerate = resolve;
  });
  let resolvePhotoMeta;
  const photoMetaPromise = new Promise((resolve) => {
    resolvePhotoMeta = resolve;
  });
  let resolvePhotoContent;
  const photoContentPromise = new Promise((resolve) => {
    resolvePhotoContent = resolve;
  });

  const fakePort = {
    generateSyntheticPhoto: () => generatePromise,
    getProfilePhoto: () => photoMetaPromise,
    getProfilePhotoContent: () => photoContentPromise,
  };

  const facade = new AccountFacade(fakePort);
  let result;
  let rejectedError = null;
  const pending = facade.generateSyntheticPhoto("idem-photo-1").then(
    (val) => {
      result = val;
    },
    (err) => {
      rejectedError = err;
    },
  );

  const eventsBeforeClear = AccountFacade.blobEvents.length;
  facade.clear();

  resolveGenerate();
  resolvePhotoMeta({ id: "photo-1", url: "https://example.com/p.jpg" });
  resolvePhotoContent(new Blob(["synthetic-photo-data"]));
  await pending;

  const snapshot = facade.getSnapshot();
  assert.equal(snapshot.photo, null);
  assert.equal(snapshot.photoUrl, null);
  assert.equal(snapshot.loading, false);
  assert.equal(snapshot.error, null);

  const createsAfterClear = AccountFacade.blobEvents
    .slice(eventsBeforeClear)
    .filter((e) => e.type === "create");
  assert.equal(createsAfterClear.length, 0);

  assert.equal(result, undefined);
  assert.ok(rejectedError, "Debe rechazar con AbortError tras clear()");
  assert.equal(rejectedError.name, "AbortError");
});

test("createOrReplacePayoutAccount diferido tras clear() no repuebla snapshot y rechaza con AbortError", async () => {
  const AccountFacade = await loadFacade();
  let resolvePayout;
  const payoutPromise = new Promise((resolve) => {
    resolvePayout = resolve;
  });

  const fakePort = {
    createOrReplacePayoutAccount: () => payoutPromise,
  };

  const facade = new AccountFacade(fakePort);
  let result;
  let rejectedError = null;
  const pending = facade.createOrReplacePayoutAccount("idem-payout-1").then(
    (val) => {
      result = val;
    },
    (err) => {
      rejectedError = err;
    },
  );

  facade.clear();

  resolvePayout({ id: "payout-1", status: "ACTIVE" });
  await pending;

  const snapshot = facade.getSnapshot();
  assert.equal(snapshot.payoutAccount, null);
  assert.equal(snapshot.loading, false);
  assert.equal(snapshot.error, null);

  assert.equal(result, undefined);
  assert.ok(rejectedError, "Debe rechazar con AbortError tras clear()");
  assert.equal(rejectedError.name, "AbortError");
});

test("removeProfilePhoto y revokePayoutAccount diferidos tras clear() rechazan con AbortError y snapshot intacto", async () => {
  const AccountFacade = await loadFacade();
  let resolveRemovePhoto;
  const removePromise = new Promise((resolve) => {
    resolveRemovePhoto = resolve;
  });
  let resolveRevokePayout;
  const revokePromise = new Promise((resolve) => {
    resolveRevokePayout = resolve;
  });

  const fakePort = {
    removeProfilePhoto: () => removePromise,
    revokePayoutAccount: () => revokePromise,
  };

  const facade = new AccountFacade(fakePort);

  let removeErr = null;
  const pendingRemove = facade.removeProfilePhoto("idem-rem-1").catch((e) => {
    removeErr = e;
  });
  let revokeErr = null;
  const pendingRevoke = facade.revokePayoutAccount("idem-rev-1").catch((e) => {
    revokeErr = e;
  });

  facade.clear();
  resolveRemovePhoto();
  resolveRevokePayout();

  await Promise.all([pendingRemove, pendingRevoke]);

  assert.ok(removeErr, "removeProfilePhoto debe fallar con AbortError");
  assert.equal(removeErr.name, "AbortError");
  assert.ok(revokeErr, "revokePayoutAccount debe fallar con AbortError");
  assert.equal(revokeErr.name, "AbortError");

  const snap = facade.getSnapshot();
  assert.equal(snap.photo, null);
  assert.equal(snap.payoutAccount, null);
  assert.equal(snap.loading, false);
});

test("privacidad tardía: requestDataRight diferido rechaza con AbortError y exportIdentityData diferido retorna null", async () => {
  const AccountFacade = await loadFacade();
  let resolveRequestRight;
  const rightPromise = new Promise((resolve) => {
    resolveRequestRight = resolve;
  });
  let resolveExportData;
  const exportPromise = new Promise((resolve) => {
    resolveExportData = resolve;
  });

  const fakePort = {
    requestDataRight: () => rightPromise,
    listRightsRequests: async () => [{ id: "req-1", kind: "ACCESS" }],
    exportIdentityData: () => exportPromise,
  };

  const facade = new AccountFacade(fakePort);

  let rightErr = null;
  const pendingRight = facade.requestDataRight("ACCESS").catch((e) => {
    rightErr = e;
  });
  let exportResult;
  let exportErr = null;
  const pendingExport = facade.exportIdentityData().then(
    (v) => {
      exportResult = v;
    },
    (e) => {
      exportErr = e;
    },
  );

  facade.clear();
  resolveRequestRight({ id: "req-1", kind: "ACCESS" });
  resolveExportData({ userId: "user-a", profile: { id: "user-a" } });

  await Promise.all([pendingRight, pendingExport]);

  assert.ok(
    rightErr,
    "requestDataRight debe rechazar con AbortError tras clear",
  );
  assert.equal(rightErr.name, "AbortError");
  assert.deepEqual(facade.getSnapshot().rightsRequests, []);

  if (exportErr) {
    assert.equal(exportErr.name, "AbortError");
  } else {
    assert.equal(
      exportResult,
      null,
      "exportIdentityData tardío no debe retornar datos de otra sesión",
    );
  }
});

test("anonymous y checking pre-puerto: no invocan puertos de mutación ni exportación", async () => {
  const AccountFacade = await loadFacade();
  for (const statusSnapshot of [checkingSnapshot, anonymousSnapshot]) {
    let portCalls = 0;
    const fakePort = {
      generateSyntheticPhoto: async () => {
        portCalls++;
      },
      removeProfilePhoto: async () => {
        portCalls++;
      },
      createOrReplacePayoutAccount: async () => {
        portCalls++;
        return { id: "p-1" };
      },
      revokePayoutAccount: async () => {
        portCalls++;
      },
      requestDataRight: async () => {
        portCalls++;
        return { id: "r-1" };
      },
      listRightsRequests: async () => {
        portCalls++;
        return [];
      },
      exportIdentityData: async () => {
        portCalls++;
        return { userId: "u-1" };
      },
    };

    const authSource = createMockAuthSource(statusSnapshot);
    const facade = new AccountFacade(fakePort, authSource);

    await assert.rejects(() => facade.generateSyntheticPhoto("id-1"), {
      name: "AbortError",
    });
    await assert.rejects(() => facade.removeProfilePhoto("id-2"), {
      name: "AbortError",
    });
    await assert.rejects(() => facade.createOrReplacePayoutAccount("id-3"), {
      name: "AbortError",
    });
    await assert.rejects(() => facade.revokePayoutAccount("id-4"), {
      name: "AbortError",
    });
    await assert.rejects(() => facade.requestDataRight("RECTIFICATION"), {
      name: "AbortError",
    });

    const exported = await facade.exportIdentityData();
    assert.equal(exported, null);

    assert.equal(
      portCalls,
      0,
      `No debe invocar puertos en estado ${statusSnapshot.status}`,
    );
  }
});

test("401/403 purga síncrono y revalida con checkSession sin logout en operaciones restantes, 503 no purga", async () => {
  const AccountFacade = await loadFacade();
  const authSource = createMockAuthSource(authenticatedUserA);
  let purges = 0;

  let errorToThrow = null;
  const fakePort = {
    generateSyntheticPhoto: async () => {
      if (errorToThrow) throw errorToThrow;
    },
    getProfilePhoto: async () => null,
    createOrReplacePayoutAccount: async () => {
      if (errorToThrow) throw errorToThrow;
      return { id: "payout-1", status: "ACTIVE" };
    },
    requestDataRight: async () => {
      if (errorToThrow) throw errorToThrow;
      return { id: "req-1", kind: "OPPOSITION" };
    },
    listRightsRequests: async () => [],
    exportIdentityData: async () => {
      if (errorToThrow) throw errorToThrow;
      return { userId: "user-a" };
    },
  };

  const facade = new AccountFacade(fakePort, authSource, () => {
    purges++;
  });

  // 1. Operación exitosa
  await facade.createOrReplacePayoutAccount("idem-1");
  assert.equal(facade.getSnapshot().payoutAccount?.id, "payout-1");
  assert.equal(authSource.getCheckSessionCalls(), 0);

  // 2. Error 401 en createOrReplacePayoutAccount
  errorToThrow = { status: 401, message: "Unauthorized" };
  await assert.rejects(
    () => facade.createOrReplacePayoutAccount("idem-2"),
    (err) => err.status === 401,
  );
  assert.equal(
    facade.getSnapshot().payoutAccount,
    null,
    "401 debe purgar snapshot",
  );
  assert.equal(
    authSource.getCheckSessionCalls(),
    1,
    "401 debe disparar checkSession",
  );
  assert.equal(
    authSource.getSnapshot().status,
    "authenticated",
    "401 no desloguea authSource",
  );
  assert.ok(purges > 0, "401 debe disparar onPrivatePurge");

  // 3. Error 403 en requestDataRight
  authSource.emit(authenticatedUserA);
  errorToThrow = { status: 403, message: "Forbidden" };
  const purgesBefore403 = purges;
  const checkCallsBefore403 = authSource.getCheckSessionCalls();
  await assert.rejects(
    () => facade.requestDataRight("OPPOSITION"),
    (err) => err.status === 403,
  );
  assert.equal(authSource.getCheckSessionCalls(), checkCallsBefore403 + 1);
  assert.ok(purges > purgesBefore403);

  // 4. Error 503 en generateSyntheticPhoto: NO purga ni checkSession
  authSource.emit(authenticatedUserA);
  errorToThrow = null;
  await facade.createOrReplacePayoutAccount("idem-3");
  assert.equal(facade.getSnapshot().payoutAccount?.id, "payout-1");

  const checkCallsBefore503 = authSource.getCheckSessionCalls();
  const purgesBefore503 = purges;
  errorToThrow = { status: 503, message: "Service Unavailable" };
  await assert.rejects(
    () => facade.generateSyntheticPhoto("idem-photo-503"),
    (err) => err.status === 503,
  );
  assert.equal(
    authSource.getCheckSessionCalls(),
    checkCallsBefore503,
    "503 no llama checkSession",
  );
  assert.equal(purges, purgesBefore503, "503 no purga");
  assert.equal(
    facade.getSnapshot().payoutAccount?.id,
    "payout-1",
    "503 mantiene snapshot existente",
  );
  assert.equal(facade.getSnapshot().error, "Service Unavailable");
});

test("AbortSignal respetado: señal abortada cancela llamadas y no actualiza snapshot", async () => {
  const AccountFacade = await loadFacade();
  const authSource = createMockAuthSource(authenticatedUserA);

  let portCalled = false;
  const fakePort = {
    createOrReplacePayoutAccount: async (_key, signal) => {
      portCalled = true;
      if (signal?.aborted) {
        throw new DOMException("The operation was aborted", "AbortError");
      }
      return { id: "payout-abort", status: "ACTIVE" };
    },
  };

  const facade = new AccountFacade(fakePort, authSource);
  const controller = new AbortController();
  controller.abort();

  await assert.rejects(
    () => facade.createOrReplacePayoutAccount("idem-abort", controller.signal),
    { name: "AbortError" },
  );

  assert.equal(portCalled, false, "Señal abortada no debe invocar puerto");
  assert.equal(facade.getSnapshot().payoutAccount, null);
});
