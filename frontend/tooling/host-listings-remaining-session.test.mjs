import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(
    process.env.TMPDIR ?? os.tmpdir(),
    "espacigo-host-listings-remaining-session-",
  ),
);

async function loadFacade() {
  let source = await readFile(
    path.join(
      root,
      "src/features/host-listings/application/host-listings-facade.ts",
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
  const file = path.join(scratch, "host-listings-facade.mjs");
  await writeFile(file, outputText);
  return (await import(pathToFileURL(file).href)).HostListingsFacade;
}

test.after(async () => {
  await rm(scratch, { recursive: true, force: true });
});

test("RED: updateDraft tardío tras clear() debe abortar y no sobrescribir snapshot limpio", async () => {
  const HostListingsFacade = await loadFacade();
  let resolveDraft;
  const draftPromise = new Promise((res) => {
    resolveDraft = res;
  });
  const fakePort = {
    updateDraft: () => draftPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  let error = null;
  let result = null;
  const pending = facade
    .updateDraft("listing-1", { title: "Nuevo Titulo" })
    .then((r) => {
      result = r;
      return r;
    })
    .catch((e) => {
      error = e;
      return null;
    });

  facade.clear();

  resolveDraft({
    id: "listing-1",
    title: "Nuevo Titulo",
    state: "borrador",
  });
  await pending;

  assert.equal(facade.getSnapshot().listings.length, 0);
  assert.equal(facade.getSnapshot().selectedListing, null);
  assert.equal(error?.name, "AbortError");
  assert.equal(result, null);
});

test("RED: uploadPhoto tardío tras clear() debe abortar y no sobrescribir snapshot limpio", async () => {
  const HostListingsFacade = await loadFacade();
  let resolvePhoto;
  const photoPromise = new Promise((res) => {
    resolvePhoto = res;
  });
  const fakePort = {
    uploadPhoto: () => photoPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  let error = null;
  let result = null;
  const pending = facade
    .uploadPhoto("listing-1", { file: new Blob(["photo"]) })
    .then((r) => {
      result = r;
      return r;
    })
    .catch((e) => {
      error = e;
      return null;
    });

  facade.clear();

  resolvePhoto({
    id: "photo-1",
    url: "blob:synthetic-photo",
    is_cover: false,
    position: 1,
  });
  await pending;

  assert.equal(facade.getSnapshot().photos.length, 0);
  assert.equal(error?.name, "AbortError");
  assert.equal(result, null);
});

test("updatePricing tardío tras clear() debe abortar y no sobrescribir snapshot limpio", async () => {
  const HostListingsFacade = await loadFacade();
  let resolvePricing;
  const pricingPromise = new Promise((res) => {
    resolvePricing = res;
  });
  const fakePort = {
    updatePricing: () => pricingPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  let error = null;
  let result = null;
  const pending = facade
    .updatePricing("listing-1", {
      base_price_clp: 50000,
      rate_unit: "dia",
    })
    .then((r) => {
      result = r;
      return r;
    })
    .catch((e) => {
      error = e;
      return null;
    });

  facade.clear();

  resolvePricing({
    space_id: "listing-1",
    version: 2,
    rate_unit: "dia",
    base_price: 50000,
    currency: "CLP",
    created_at: "2026-10-26T10:00:00.000Z",
  });
  await pending;

  assert.equal(facade.getSnapshot().activeTariff, null);
  assert.equal(facade.getSnapshot().tariffHistory.length, 0);
  assert.equal(error?.name, "AbortError");
  assert.equal(result, null);
});

// 2. Pre-session: authSource unauthenticated guards across method families
const mutationFamilies = [
  { name: "updateDraft", call: (f) => f.updateDraft("l1", { title: "T" }) },
  { name: "deleteListing", call: (f) => f.deleteListing("l1") },
  { name: "updateContent", call: (f) => f.updateContent("l1", { title: "T" }) },
  { name: "publishListing", call: (f) => f.publishListing("l1") },
  { name: "archiveListing", call: (f) => f.archiveListing("l1") },
  {
    name: "updatePricing",
    call: (f) =>
      f.updatePricing("l1", { base_price_clp: 1000, rate_unit: "dia" }),
  },
  {
    name: "configureTimeZone",
    call: (f) => f.configureTimeZone("l1", "America/Santiago"),
  },
  {
    name: "updateWeeklySchedule",
    call: (f) => f.updateWeeklySchedule("l1", { days: [] }),
  },
  { name: "uploadPhoto", call: (f) => f.uploadPhoto("l1") },
  { name: "setCoverPhoto", call: (f) => f.setCoverPhoto("l1", "p1") },
  { name: "removePhoto", call: (f) => f.removePhoto("l1", "p1", '"rev-1"') },
  {
    name: "movePhoto",
    call: (f) => f.movePhoto("l1", "p1", 1, '"rev-1"'),
  },
];

for (const { name, call } of mutationFamilies) {
  test(`Pre-session: mutación ${name} rechaza con Error(unauthenticated) si authSource no autenticado`, async () => {
    const HostListingsFacade = await loadFacade();
    const fakeAuth = {
      getSnapshot: () => ({ status: "unauthenticated", accountId: null }),
      subscribe: () => () => {},
    };
    const facade = new HostListingsFacade({}, fakeAuth);
    await assert.rejects(async () => await call(facade), {
      name: "Error",
      message: "unauthenticated",
    });
  });
}

const queryFamilies = [
  { name: "loadCategories", call: (f) => f.loadCategories(), expected: [] },
  {
    name: "loadCategorySchema",
    call: (f) => f.loadCategorySchema("bodega"),
    expected: null,
  },
  {
    name: "loadPricingHistory",
    call: (f) => f.loadPricingHistory("l1"),
    expected: [],
  },
  {
    name: "loadWeeklySchedule",
    call: (f) => f.loadWeeklySchedule("l1"),
    expected: null,
  },
  { name: "loadPhotos", call: (f) => f.loadPhotos("l1"), expected: [] },
];

for (const { name, call, expected } of queryFamilies) {
  test(`Pre-session: query ${name} retorna ${JSON.stringify(expected)} si authSource no autenticado`, async () => {
    const HostListingsFacade = await loadFacade();
    const fakeAuth = {
      getSnapshot: () => ({ status: "unauthenticated", accountId: null }),
      subscribe: () => () => {},
    };
    const facade = new HostListingsFacade({}, fakeAuth);
    const result = await call(facade);
    assert.deepEqual(result, expected);
  });
}

test("loadCategorySchema retorno temprano desde cache exige sesión autenticada", async () => {
  const HostListingsFacade = await loadFacade();
  let authStatus = "authenticated";
  const fakeAuth = {
    getSnapshot: () => ({ status: authStatus, accountId: "acc-1" }),
    subscribe: () => () => {},
  };
  const fakePort = {
    getCategoryProfile: async (code) => ({
      category_code: code,
      version: 1,
      fields: [],
    }),
  };
  const facade = new HostListingsFacade(fakePort, fakeAuth);
  const schema = await facade.loadCategorySchema("bodega");
  assert.equal(schema?.category_code, "bodega");

  authStatus = "unauthenticated";
  const cached = await facade.loadCategorySchema("bodega");
  assert.equal(cached, null);
});

// 3. 401/403: purga + revalida (checkSession)
const authErrorCases = [
  {
    name: "deleteListing",
    call: (f) => f.deleteListing("l1"),
    portMethod: "deleteListing",
  },
  {
    name: "updateContent",
    call: (f) => f.updateContent("l1", {}),
    portMethod: "updateContent",
  },
  {
    name: "publishListing",
    call: (f) => f.publishListing("l1"),
    portMethod: "publishListing",
  },
  {
    name: "archiveListing",
    call: (f) => f.archiveListing("l1"),
    portMethod: "archiveListing",
  },
  {
    name: "configureTimeZone",
    call: (f) => f.configureTimeZone("l1", "UTC"),
    portMethod: "configureTimeZone",
  },
  {
    name: "updateWeeklySchedule",
    call: (f) => f.updateWeeklySchedule("l1", {}),
    portMethod: "updateWeeklySchedule",
  },
  {
    name: "removePhoto",
    call: (f) => f.removePhoto("l1", "p1", '"rev-1"'),
    portMethod: "removePhoto",
  },
  {
    name: "movePhoto",
    call: (f) => f.movePhoto("l1", "p1", 1, '"rev-1"'),
    portMethod: "movePhoto",
  },
  {
    name: "loadCategories",
    call: (f) => f.loadCategories(),
    portMethod: "listCategories",
  },
  {
    name: "loadPhotos",
    call: (f) => f.loadPhotos("l1"),
    portMethod: "listPhotos",
  },
];

for (const { name, call, portMethod } of authErrorCases) {
  test(`401/403: ${name} ejecuta clear() y llama checkSession()`, async () => {
    const HostListingsFacade = await loadFacade();
    let checkedSession = false;
    let purged = false;
    const fakeAuth = {
      getSnapshot: () => ({ status: "authenticated", accountId: "acc-1" }),
      subscribe: () => () => {},
      checkSession: async () => {
        checkedSession = true;
      },
    };
    const fakePort = {
      [portMethod]: async () => {
        const err = new Error("Unauthorized");
        err.status = 401;
        throw err;
      },
    };
    const facade = new HostListingsFacade(fakePort, fakeAuth, {
      onPrivatePurge: () => {
        purged = true;
      },
    });

    await assert.rejects(async () => await call(facade));
    assert.equal(checkedSession, true);
    assert.equal(purged, true);
  });
}

// 4. 503 error: NO purga ni invalida sesión
test("503 error no purga sesión ni altera listings existentes", async () => {
  const HostListingsFacade = await loadFacade();
  let checkedSession = false;
  let purged = false;
  const fakeAuth = {
    getSnapshot: () => ({ status: "authenticated", accountId: "acc-1" }),
    subscribe: () => () => {},
    checkSession: async () => {
      checkedSession = true;
    },
  };
  const fakePort = {
    updateContent: async () => {
      const err = new Error("Service Unavailable");
      err.status = 503;
      throw err;
    },
  };
  const facade = new HostListingsFacade(fakePort, fakeAuth, {
    onPrivatePurge: () => {
      purged = true;
    },
  });

  await assert.rejects(
    async () => await facade.updateContent("l1", { title: "T" }),
    (err) => err.status === 503,
  );
  assert.equal(checkedSession, false);
  assert.equal(purged, false);
  assert.equal(facade.getSnapshot().error, "Service Unavailable");
});
