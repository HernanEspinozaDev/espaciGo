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
    "espacigo-host-listings-ops-session-",
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

test("createDraft tardío tras clear() no debe sobrescribir snapshot limpio ni entregar payload y debe abortar", async () => {
  const HostListingsFacade = await loadFacade();
  let resolveDraft;
  const draftPromise = new Promise((res) => {
    resolveDraft = res;
  });
  const fakePort = {
    createDraft: () => draftPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  const inputSintetico = {
    category_code: "office",
    title: "Borrador de Prueba",
    description: "Espacio de trabajo para pruebas",
    area_m2: 30,
    capacity: 5,
    usage_rules: "Reglas de uso del espacio",
    rate_unit: "dia",
    base_price_clp: 45000,
    address: "Moneda 456",
  };
  const idempotencyKey = "idempotency-key-001";

  let result = null;
  let error = null;
  const pending = facade
    .createDraft(inputSintetico, idempotencyKey)
    .then((res) => {
      result = res;
      return res;
    })
    .catch((err) => {
      error = err;
      return null;
    });

  facade.clear();

  const syntheticDetail = {
    id: "listing-draft-123",
    category_code: "office",
    category_name: "Oficina",
    title: "Borrador de Prueba",
    description: "Espacio de trabajo para pruebas",
    area_m2: 30,
    capacity: 5,
    usage_rules: "Reglas de uso del espacio",
    rate_unit: "dia",
    base_price_clp: 45000,
    address: "Moneda 456",
    state: "borrador",
    cover_photo_id: null,
    time_zone: "America/Santiago",
    region_code: "RM",
    comuna_code: "santiago",
    attribute_schema_version: 1,
    attributes: {},
  };

  resolveDraft(syntheticDetail);
  await pending;

  assert.equal(facade.getSnapshot().listings.length, 0);
  assert.equal(facade.getSnapshot().selectedListing, null);
  assert.equal(facade.getSnapshot().error, null);
  assert.equal(error?.name, "AbortError");
  assert.equal(result, null);
});

test("loadPricing tardío tras clear() no debe sobrescribir snapshot limpio ni entregar payload", async () => {
  const HostListingsFacade = await loadFacade();
  let resolvePricing;
  const pricingPromise = new Promise((res) => {
    resolvePricing = res;
  });
  const fakePort = {
    getPricing: () => pricingPromise,
  };

  const facade = new HostListingsFacade(fakePort);
  let result = null;
  let error = null;
  const pending = facade
    .loadPricing("listing-a")
    .then((res) => {
      result = res;
      return res;
    })
    .catch((err) => {
      error = err;
      return null;
    });

  facade.clear();

  const syntheticTariff = {
    space_id: "listing-a",
    version: 1,
    rate_unit: "dia",
    base_price: 45000,
    currency: "CLP",
    created_at: "2026-10-26T10:00:00.000Z",
  };

  resolvePricing(syntheticTariff);
  await pending;

  assert.equal(facade.getSnapshot().activeTariff, null);
  assert.equal(facade.getSnapshot().error, null);
  if (error) {
    assert.equal(error?.name, "AbortError");
  } else {
    assert.notDeepEqual(result, syntheticTariff);
    assert.equal(result, null);
  }
});
