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
    "espacigo-host-listings-session-",
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

test("loadListings tardío tras clear() no debe sobrescribir snapshot limpio ni entregar payload privado", async () => {
  const HostListingsFacade = await loadFacade();
  let resolveList;
  const listPromise = new Promise((res) => {
    resolveList = res;
  });
  const fakePort = {
    listListings: () => listPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  const pending = facade.loadListings();
  facade.clear();

  const privadoA = [
    {
      id: "listing-privado-a",
      title: "Espacio Privado A",
      category_code: "office",
      category_name: "Oficina",
      state: "published",
      base_price_clp: 50000,
      rate_unit: "day",
      area_m2: 25,
      capacity: 4,
      cover_photo_id: null,
      time_zone: "America/Santiago",
      region_code: "RM",
      comuna_code: "santiago",
    },
  ];

  resolveList(privadoA);

  let result = null;
  let error = null;
  try {
    result = await pending;
  } catch (err) {
    error = err;
  }

  assert.equal(facade.getSnapshot().listings.length, 0);
  if (error) {
    assert.equal(error?.name, "AbortError");
  } else {
    assert.notDeepEqual(result, privadoA);
    assert.ok(
      result === null || (Array.isArray(result) && result.length === 0),
      "No debe entregar payload privado tras clear()",
    );
  }
});

test("tariff success advances selected listing ETag; 412 leaves it unchanged", async () => {
  const HostListingsFacade = await loadFacade();
  let rejectTariff = false;
  const listing = {
    id: "space-1",
    title: "Espacio 1",
    category_code: "office",
    category_name: "Oficina",
    state: "borrador",
    base_price_clp: 8000,
    rate_unit: "hora",
    area_m2: 25,
    capacity: 4,
    cover_photo_id: null,
    time_zone: "America/Santiago",
    region_code: "RM",
    comuna_code: "santiago",
    attribute_schema_version: 1,
    attributes: {},
    etag: '"space-v1"',
  };
  const port = {
    getListing: async () => listing,
    getPricing: async () => null,
    getWeeklySchedule: async () => null,
    listPhotos: async () => [],
    updatePricing: async () => {
      if (rejectTariff)
        throw Object.assign(new Error("stale"), { status: 412 });
      return {
        space_id: "space-1",
        version: 2,
        rate_unit: "hora",
        base_price: 9000,
        currency: "CLP",
        created_at: "2026-10-28T12:00:00Z",
        etag: '"space-v2"',
      };
    },
  };
  const facade = new HostListingsFacade(port);
  await facade.selectListing("space-1");
  await facade.updatePricing(
    "space-1",
    { base_price_clp: 9000, rate_unit: "hora" },
    '"space-v1"',
  );
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v2"');

  rejectTariff = true;
  await assert.rejects(
    facade.updatePricing(
      "space-1",
      { base_price_clp: 10000, rate_unit: "hora" },
      '"space-v2"',
    ),
    (error) => error.status === 412,
  );
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v2"');
});

test("adjacent gallery move advances selected ETag only after success; stale 412 leaves order and revision unchanged", async () => {
  const HostListingsFacade = await loadFacade();
  const listing = {
    id: "space-1",
    title: "Espacio 1",
    category_code: "office",
    category_name: "Oficina",
    state: "borrador",
    base_price_clp: 8000,
    rate_unit: "hora",
    area_m2: 25,
    capacity: 4,
    cover_photo_id: null,
    time_zone: "America/Santiago",
    region_code: "RM",
    comuna_code: "santiago",
    attribute_schema_version: 1,
    attributes: {},
    etag: '"space-v1"',
  };
  let stale = false;
  const moveRequests = [];
  const port = {
    getListing: async () => listing,
    getPricing: async () => null,
    getWeeklySchedule: async () => null,
    listPhotos: async () => [
      { id: "p1", position: 1 },
      { id: "p2", position: 2 },
    ],
    movePhoto: async (listingId, photoId, targetPosition, ifMatch) => {
      moveRequests.push([listingId, photoId, targetPosition, ifMatch]);
      assert.equal(ifMatch, stale ? '"space-v2"' : '"space-v1"');
      if (stale) throw Object.assign(new Error("stale"), { status: 412 });
      return '"space-v2"';
    },
  };
  const facade = new HostListingsFacade(port);
  await facade.selectListing("space-1");
  await facade.loadPhotos("space-1");
  await facade.movePhoto("space-1", "p2", 1, '"space-v1"');
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v2"');
  assert.deepEqual(
    facade.getSnapshot().photos.map((photo) => [photo.id, photo.position]),
    [
      ["p2", 1],
      ["p1", 2],
    ],
  );
  stale = true;
  await assert.rejects(
    facade.movePhoto("space-1", "p1", 1, '"space-v2"'),
    (error) => error.status === 412,
  );
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v2"');
  assert.deepEqual(
    facade.getSnapshot().photos.map((photo) => [photo.id, photo.position]),
    [
      ["p2", 1],
      ["p1", 2],
    ],
  );
  assert.deepEqual(moveRequests, [
    ["space-1", "p2", 1, '"space-v1"'],
    ["space-1", "p1", 1, '"space-v2"'],
  ]);
});

test("timezone renewal feeds weekly CAS and stale 412 leaves facade ETag unchanged", async () => {
  const HostListingsFacade = await loadFacade();
  const zoneRequests = [];
  const weeklyRequests = [];
  let rejectZone = false;
  let rejectWeekly = false;
  const listing = {
    id: "space-1",
    title: "Espacio 1",
    category_code: "office",
    category_name: "Oficina",
    state: "borrador",
    base_price_clp: 8000,
    rate_unit: "hora",
    area_m2: 25,
    capacity: 4,
    cover_photo_id: null,
    time_zone: null,
    region_code: "RM",
    comuna_code: "santiago",
    attribute_schema_version: 1,
    attributes: {},
    etag: '"space-v1"',
  };
  const port = {
    getListing: async () => listing,
    getPricing: async () => null,
    getWeeklySchedule: async () => null,
    listPhotos: async () => [],
    configureTimeZone: async (...args) => {
      zoneRequests.push(args);
      if (rejectZone) throw Object.assign(new Error("stale"), { status: 412 });
      return {
        listingId: "space-1",
        timeZone: "America/Santiago",
        etag: '"space-v2"',
      };
    },
    updateWeeklySchedule: async (...args) => {
      weeklyRequests.push(args);
      if (rejectWeekly)
        throw Object.assign(new Error("stale"), { status: 412 });
      return {
        space_id: "space-1",
        enabled: true,
        time_zone: "America/Santiago",
        days: [],
        etag: '"space-v3"',
      };
    },
  };
  const facade = new HostListingsFacade(port);
  await facade.selectListing("space-1");

  await facade.configureTimeZone(
    "space-1",
    "America/Santiago",
    facade.getSnapshot().selectedListing.etag,
  );
  assert.equal(zoneRequests[0][2], '"space-v1"');
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v2"');

  const scheduleInput = {
    timezone: "America/Santiago",
    enabled: true,
    weekly_slots: [],
  };
  await facade.updateWeeklySchedule(
    "space-1",
    scheduleInput,
    facade.getSnapshot().selectedListing.etag,
  );
  assert.equal(weeklyRequests[0][2], '"space-v2"');
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v3"');

  rejectWeekly = true;
  await assert.rejects(
    facade.updateWeeklySchedule("space-1", scheduleInput, '"space-v3"'),
    (error) => error.status === 412,
  );
  assert.equal(
    weeklyRequests.length,
    2,
    "412 must not be retried automatically",
  );
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v3"');

  rejectZone = true;
  await assert.rejects(
    facade.configureTimeZone("space-1", "America/Santiago", '"space-v3"'),
    (error) => error.status === 412,
  );
  assert.equal(
    zoneRequests.length,
    2,
    "timezone 412 must not be retried automatically",
  );
  assert.equal(facade.getSnapshot().selectedListing.etag, '"space-v3"');
});

test("selectListing tardío tras clear() no debe sobrescribir snapshot limpio ni entregar payload privado", async () => {
  const HostListingsFacade = await loadFacade();
  let resolveListing;
  const listingPromise = new Promise((res) => {
    resolveListing = res;
  });
  let resolvePricing;
  const pricingPromise = new Promise((res) => {
    resolvePricing = res;
  });
  let resolveSchedule;
  const schedulePromise = new Promise((res) => {
    resolveSchedule = res;
  });
  let resolvePhotos;
  const photosPromise = new Promise((res) => {
    resolvePhotos = res;
  });

  const fakePort = {
    getListing: () => listingPromise,
    getPricing: () => pricingPromise,
    getWeeklySchedule: () => schedulePromise,
    listPhotos: () => photosPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  const pending = facade.selectListing("id-fixture");
  facade.clear();

  const privadoA = {
    id: "id-fixture",
    title: "Espacio Privado A",
    category_code: "office",
    category_name: "Oficina",
    state: "published",
    base_price_clp: 50000,
    rate_unit: "day",
    area_m2: 25,
    capacity: 4,
    cover_photo_id: null,
    time_zone: "America/Santiago",
    region_code: "RM",
    comuna_code: "santiago",
  };

  resolveListing(privadoA);
  resolvePricing(null);
  resolveSchedule(null);
  resolvePhotos([]);

  let result = null;
  let error = null;
  try {
    result = await pending;
  } catch (err) {
    error = err;
  }

  assert.equal(facade.getSnapshot().selectedListing, null);
  if (error) {
    assert.equal(error?.name, "AbortError");
  } else {
    assert.notDeepEqual(result, privadoA);
    assert.equal(result, null);
  }
});

function createMockAuthSource(initialSnapshot = {}) {
  let snapshot = {
    status: "authenticated",
    accountId: "user-A",
    roles: ["host"],
    capabilities: ["create_listing"],
    restrictedMode: false,
    expiresAt: "2026-10-26T12:00:00Z",
    ...initialSnapshot,
  };
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
    emit: (next) => {
      snapshot = { ...snapshot, ...next };
      for (const fn of listeners) fn();
    },
    getCheckSessionCalls: () => checkSessionCalls,
  };
}

const sampleListing = {
  id: "listing-1",
  title: "Espacio 1",
  category_code: "office",
  category_name: "Oficina",
  state: "published",
  base_price_clp: 50000,
  rate_unit: "day",
  area_m2: 25,
  capacity: 4,
  cover_photo_id: null,
  time_zone: "America/Santiago",
  region_code: "RM",
  comuna_code: "santiago",
};

test("cambio de identidad (user-A -> user-B) limpia snapshot y purga blobs sincrónicamente", async () => {
  const HostListingsFacade = await loadFacade();
  const authSource = createMockAuthSource();
  let purgeCount = 0;
  const onPrivatePurge = () => {
    purgeCount++;
  };
  const fakePort = {
    listListings: async () => [sampleListing],
  };

  const facade = new HostListingsFacade(fakePort, authSource, onPrivatePurge);
  await facade.loadListings();
  assert.equal(facade.getSnapshot().listings.length, 1);
  assert.equal(purgeCount, 0);

  authSource.emit({ accountId: "user-B" });

  assert.equal(facade.getSnapshot().listings.length, 0);
  assert.equal(purgeCount, 1);
});

test("cambio de roles, restrictedMode o expiry limpia snapshot y purga blobs", async () => {
  const HostListingsFacade = await loadFacade();
  const authSource = createMockAuthSource();
  let purgeCount = 0;
  const fakePort = {
    listListings: async () => [sampleListing],
  };

  const facade = new HostListingsFacade(fakePort, authSource, () => {
    purgeCount++;
  });

  // 1. Roles change
  await facade.loadListings();
  assert.equal(facade.getSnapshot().listings.length, 1);
  authSource.emit({ roles: ["host", "admin"] });
  assert.equal(facade.getSnapshot().listings.length, 0);
  assert.equal(purgeCount, 1);

  // 2. restrictedMode change
  await facade.loadListings();
  assert.equal(facade.getSnapshot().listings.length, 1);
  authSource.emit({ restrictedMode: true });
  assert.equal(facade.getSnapshot().listings.length, 0);
  assert.equal(purgeCount, 2);

  // 3. expiry change
  await facade.loadListings();
  assert.equal(facade.getSnapshot().listings.length, 1);
  authSource.emit({ expiresAt: "2026-10-26T15:00:00Z" });
  assert.equal(facade.getSnapshot().listings.length, 0);
  assert.equal(purgeCount, 3);
});

test("estado anónimo o checking no ejecuta llamadas a puertos en loadListings ni selectListing", async () => {
  const HostListingsFacade = await loadFacade();
  let listCalls = 0;
  let getListingCalls = 0;
  const fakePort = {
    listListings: async () => {
      listCalls++;
      return [sampleListing];
    },
    getListing: async () => {
      getListingCalls++;
      return sampleListing;
    },
    getPricing: async () => {
      getListingCalls++;
      return null;
    },
    getWeeklySchedule: async () => {
      getListingCalls++;
      return null;
    },
    listPhotos: async () => {
      getListingCalls++;
      return [];
    },
  };

  const authSource = createMockAuthSource({ status: "anonymous" });
  const facade = new HostListingsFacade(fakePort, authSource);

  const listings = await facade.loadListings();
  const selected = await facade.selectListing("listing-1");
  assert.deepEqual(listings, []);
  assert.equal(selected, null);
  assert.equal(listCalls, 0);
  assert.equal(getListingCalls, 0);

  authSource.emit({ status: "checking" });
  const listings2 = await facade.loadListings();
  const selected2 = await facade.selectListing("listing-1");
  assert.deepEqual(listings2, []);
  assert.equal(selected2, null);
  assert.equal(listCalls, 0);
  assert.equal(getListingCalls, 0);
});

test("error 401/403 en consulta secundaria (pricing) de selectListing no se traga, purga y revalida sesión", async () => {
  const HostListingsFacade = await loadFacade();
  const authSource = createMockAuthSource();
  let purgeCount = 0;
  const fakePort = {
    getListing: async () => sampleListing,
    getPricing: async () => {
      const err = new Error("Unauthorized");
      err.status = 401;
      throw err;
    },
    getWeeklySchedule: async () => null,
    listPhotos: async () => [],
  };

  const facade = new HostListingsFacade(fakePort, authSource, () => {
    purgeCount++;
  });

  let thrown = null;
  try {
    await facade.selectListing("listing-1");
  } catch (err) {
    thrown = err;
  }

  assert.ok(
    thrown,
    "selectListing debe re-lanzar error 401 de consulta secundaria",
  );
  assert.equal(thrown.status, 401);
  assert.equal(facade.getSnapshot().selectedListing, null);
  assert.equal(purgeCount, 1);
  assert.equal(authSource.getCheckSessionCalls(), 1);
});

test("error 503 en consulta secundaria o principal no purga ni revalida sesión", async () => {
  const HostListingsFacade = await loadFacade();
  const authSource = createMockAuthSource();
  let purgeCount = 0;
  const fakePort = {
    getListing: async () => sampleListing,
    getPricing: async () => {
      const err = new Error("Service Unavailable");
      err.status = 503;
      throw err;
    },
    getWeeklySchedule: async () => null,
    listPhotos: async () => [],
    listListings: async () => {
      const err = new Error("Service Unavailable");
      err.status = 503;
      throw err;
    },
  };

  const facade = new HostListingsFacade(fakePort, authSource, () => {
    purgeCount++;
  });

  // 1. Pricing 503: se conserva rescate existente devolviendo null
  const detail = await facade.selectListing("listing-1");
  assert.equal(detail.id, "listing-1");
  assert.equal(facade.getSnapshot().activeTariff, null);
  assert.equal(purgeCount, 0);
  assert.equal(authSource.getCheckSessionCalls(), 0);

  // 2. listListings 503: falla pero no purga sesión ni llama checkSession
  let listErr = null;
  try {
    await facade.loadListings();
  } catch (err) {
    listErr = err;
  }
  assert.ok(listErr);
  assert.equal(listErr.status, 503);
  assert.equal(purgeCount, 0);
  assert.equal(authSource.getCheckSessionCalls(), 0);
});

test("misma sesión con nuevas instancias de arrays idénticos permanece intacta", async () => {
  const HostListingsFacade = await loadFacade();
  const authSource = createMockAuthSource();
  let purgeCount = 0;
  const fakePort = {
    listListings: async () => [sampleListing],
  };

  const facade = new HostListingsFacade(fakePort, authSource, () => {
    purgeCount++;
  });
  await facade.loadListings();
  assert.equal(facade.getSnapshot().listings.length, 1);
  assert.equal(purgeCount, 0);

  // Emitir snapshot con nueva instancia de array pero valores idénticos
  authSource.emit({
    roles: ["host"],
    capabilities: ["create_listing"],
  });

  assert.equal(facade.getSnapshot().listings.length, 1);
  assert.equal(purgeCount, 0);
});
