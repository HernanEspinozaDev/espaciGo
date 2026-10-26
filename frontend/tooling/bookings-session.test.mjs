import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bookings-session-"),
);

async function loadFacade() {
  let rulesSource = await readFile(
    path.join(root, "src/features/bookings/domain/booking-rules.ts"),
    "utf8",
  );
  const transpiledRules = ts.transpileModule(rulesSource, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  await writeFile(path.join(scratch, "booking-rules.mjs"), transpiledRules);

  let source = await readFile(
    path.join(root, "src/features/bookings/application/bookings-facade.ts"),
    "utf8",
  );
  source = source.replace('import "client-only";', "");
  source = source.replace(
    'from "../domain/booking-rules"',
    'from "./booking-rules.mjs"',
  );
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  });
  const file = path.join(scratch, "bookings-facade.mjs");
  await writeFile(file, outputText);
  return (await import(pathToFileURL(file).href)).BookingsFacade;
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
      return current;
    },
    logout: async () => {
      calls.logout += 1;
    },
  };
}

test("loadReservations demorado -> clear -> resolve no comitea datos ni sobrescribe snapshot", async () => {
  const BookingsFacade = await loadFacade();
  let resolveList;
  const listPromise = new Promise((res) => {
    resolveList = res;
  });
  const fakePort = {
    listReservations: () => listPromise,
  };

  const facade = new BookingsFacade(fakePort);
  const pending = facade.loadReservations();
  facade.clear();

  resolveList([{ id: "res-1", status: "pendiente" }]);
  const result = await pending;

  assert.equal(facade.getSnapshot().reservations.length, 0);
  assert.equal(facade.getSnapshot().loading, false);
  assert.deepEqual(result, []);
});

test("loadReservationDetail demorado -> clear -> resolve no altera currentReservation", async () => {
  const BookingsFacade = await loadFacade();
  let resolveDetail;
  const detailPromise = new Promise((res) => {
    resolveDetail = res;
  });
  const fakePort = {
    getReservation: () => detailPromise,
  };

  const facade = new BookingsFacade(fakePort);
  const pending = facade.loadReservationDetail("res-1");
  facade.clear();

  resolveDetail({ id: "res-1", status: "confirmada" });
  const result = await pending;

  assert.equal(result, null);
  assert.equal(facade.getSnapshot().currentReservation, null);
  assert.equal(facade.getSnapshot().loading, false);
});

test("downloadContractPdf tardío tras clear() retorna null y no persiste error", async () => {
  const BookingsFacade = await loadFacade();
  let resolvePdf;
  const pdfPromise = new Promise((res) => {
    resolvePdf = res;
  });
  const fakePort = {
    downloadContractPdf: () => pdfPromise,
  };

  const facade = new BookingsFacade(fakePort);
  const pending = facade.downloadContractPdf("contract-1");
  facade.clear();

  resolvePdf(new Blob(["fake pdf"]));
  const result = await pending;

  assert.equal(result, null);
  assert.equal(facade.getSnapshot().error, null);
});

test("requestQuote demorada -> clear -> resolve rechaza con AbortError", async () => {
  const BookingsFacade = await loadFacade();
  let resolveQuote;
  const quotePromise = new Promise((res) => {
    resolveQuote = res;
  });
  const fakePort = {
    quote: () => quotePromise,
  };

  const facade = new BookingsFacade(fakePort);
  const pending = facade.requestQuote({
    space_id: "sp-1",
    start_at: "2026-11-01T10:00:00Z",
    end_at: "2026-11-01T12:00:00Z",
  });
  facade.clear();

  resolveQuote({
    quote_id: "q-1",
    space_id: "sp-1",
    start_at: "2026-11-01T10:00:00Z",
    end_at: "2026-11-01T12:00:00Z",
    base_price_clp: 10000,
    commission_clp: 1000,
    tax_clp: 1900,
    total_clp: 12900,
    currency: "CLP",
    expires_at: "2026-11-01T10:15:00Z",
    guarantee_clp: 5000,
  });

  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().quote, null);
  assert.equal(facade.getSnapshot().actionLoading, false);
});

test("createReservation demorada -> clear -> resolve rechaza con AbortError y no comitea snapshot", async () => {
  const BookingsFacade = await loadFacade();
  let resolveCreate;
  const createPromise = new Promise((res) => {
    resolveCreate = res;
  });
  const fakePort = {
    createReservation: () => createPromise,
  };

  const facade = new BookingsFacade(fakePort);
  const pending = facade.createReservation("q-1");
  facade.clear();

  resolveCreate({
    id: "res-1",
    status: "pendiente",
  });

  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(facade.getSnapshot().reservations.length, 0);
  assert.equal(facade.getSnapshot().actionLoading, false);
});

test("sourceA -> B full purge + onPrivatePurge + no fetch anonimo", async () => {
  const BookingsFacade = await loadFacade();
  let purgedCount = 0;
  const onPrivatePurge = () => {
    purgedCount += 1;
  };
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-A",
    roles: ["user"],
    capabilities: ["book"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  const fakePort = {
    listReservations: async () => [{ id: "res-1" }],
  };
  const facade = new BookingsFacade(fakePort, auth, onPrivatePurge);

  // auth changes accountId to acc-B triggers purge
  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-B",
    roles: ["user"],
    capabilities: ["book"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  assert.equal(purgedCount, 1);

  // roles change triggers purge
  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-B",
    roles: ["admin"],
    capabilities: ["book"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  assert.equal(purgedCount, 2);

  // restrictedMode change triggers purge
  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-B",
    roles: ["admin"],
    capabilities: ["book"],
    restrictedMode: true,
    expiresAt: "2026-12-31T23:59:59Z",
  });
  assert.equal(purgedCount, 3);

  // expiry change triggers purge
  auth.setSnapshot({
    status: "authenticated",
    accountId: "acc-B",
    roles: ["admin"],
    capabilities: ["book"],
    restrictedMode: true,
    expiresAt: "2027-01-01T00:00:00Z",
  });
  assert.equal(purgedCount, 4);

  // when anonymous, actions are rejected with unauthenticated
  auth.setSnapshot({
    status: "anonymous",
    accountId: null,
    roles: [],
    capabilities: [],
    restrictedMode: false,
    expiresAt: null,
  });
  assert.equal(purgedCount, 5);
  await assert.rejects(facade.loadReservations(), /unauthenticated/i);
  await assert.rejects(
    facade.requestQuote({ space_id: "s1", start_at: "", end_at: "" }),
    /unauthenticated/i,
  );
});

test("401/403 triggers clear, onPrivatePurge, and checkSession without auth.logout; 503 does not purge", async () => {
  const BookingsFacade = await loadFacade();
  let purgedCount = 0;
  const onPrivatePurge = () => {
    purgedCount += 1;
  };
  const auth = createFakeAuth({
    status: "authenticated",
    accountId: "acc-1",
    roles: ["user"],
    capabilities: ["book"],
    restrictedMode: false,
    expiresAt: "2026-12-31T23:59:59Z",
  });

  let rejectOp;
  const fakePort = {
    quote: () => rejectOp(),
  };

  const facade = new BookingsFacade(fakePort, auth, onPrivatePurge);

  // 503 Service Unavailable -> error thrown but NO purge, NO checkSession
  rejectOp = () =>
    Promise.reject(
      Object.assign(new Error("Service Unavailable"), { status: 503 }),
    );
  await assert.rejects(
    facade.requestQuote({ space_id: "s1", start_at: "", end_at: "" }),
  );
  assert.equal(purgedCount, 0);
  assert.equal(auth.calls.checkSession, 0);
  assert.equal(auth.calls.logout, 0);

  // 403 Forbidden -> triggers clear, onPrivatePurge, and checkSession; auth.logout is NOT called
  rejectOp = () =>
    Promise.reject(Object.assign(new Error("Forbidden"), { status: 403 }));
  await assert.rejects(
    facade.requestQuote({ space_id: "s1", start_at: "", end_at: "" }),
  );
  assert.equal(purgedCount, 1);
  assert.equal(auth.calls.checkSession, 1);
  assert.equal(auth.calls.logout, 0);

  // 401 Unauthorized -> triggers clear, onPrivatePurge, and checkSession
  rejectOp = () =>
    Promise.reject(Object.assign(new Error("Unauthorized"), { status: 401 }));
  await assert.rejects(
    facade.requestQuote({ space_id: "s1", start_at: "", end_at: "" }),
  );
  assert.equal(purgedCount, 2);
  assert.equal(auth.calls.checkSession, 2);
  assert.equal(auth.calls.logout, 0);
});
