import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import ts from "typescript";
import { createRequire } from "node:module";

const source = fs.readFileSync(
  path.resolve(
    "src/features/host-listings/application/listing-returns-facade.ts",
  ),
  "utf8",
);
const output = ts.transpileModule(source, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.CommonJS,
  },
}).outputText;
const domainSource = fs.readFileSync(
  path.resolve("src/features/host-listings/domain/listing-returns.ts"),
  "utf8",
);
const domainOutput = ts.transpileModule(domainSource, {
  compilerOptions: {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.CommonJS,
  },
}).outputText;
const domainModule = { exports: {} };
new Function("exports", "require", "module", domainOutput)(
  domainModule.exports,
  () => ({}),
  domainModule,
);
const facadeModule = { exports: {} };
new Function("exports", "require", "module", output)(
  facadeModule.exports,
  (name) =>
    name.includes("domain/listing-returns")
      ? domainModule.exports
      : createRequire(import.meta.url)(name),
  facadeModule,
);
const { ListingReturnsFacade } = facadeModule.exports;
const item = {
  reservation_id: "r-1",
  state: "finalizada",
  start_at: "2026-01-01T00:00:00Z",
  end_at: "2026-01-02T00:00:00Z",
  check_out_recorded: true,
  vacancy_confirmed: false,
};
const result = (vacant = false) => ({
  space_id: "s-1",
  items: [{ ...item, vacancy_confirmed: vacant }],
});
function setup(overrides = {}) {
  const calls = { get: 0, post: [], keys: [] };
  const port = {
    async listReturns() {
      calls.get++;
      return overrides.read
        ? overrides.read(calls.get)
        : result(calls.get > 1 && overrides.readbackSuccess);
    },
    async confirmVacancy(_token, _id, key) {
      calls.post.push(key);
      if (overrides.postError) throw Error("network");
    },
  };
  const facade = new ListingReturnsFacade(port);
  return { facade, calls };
}

test("POST exitoso requiere readback con vacancy_confirmed", async () => {
  const { facade, calls } = setup({ readbackSuccess: true });
  await facade.load("t", "s-1", "account-a");
  assert.equal(await facade.confirm("t", "s-1", "account-a", "r-1"), true);
  assert.equal(calls.get, 2);
  assert.equal(facade.getSnapshot().items[0].vacancy_confirmed, true);
});

test("readback pendiente bloquea y conserva key para actualización manual", async () => {
  const { facade, calls } = setup();
  await facade.load("t", "s-1", "account-a");
  assert.equal(await facade.confirm("t", "s-1", "account-a", "r-1"), false);
  assert.equal(facade.getSnapshot().uncertain, true);
  assert.equal(await facade.confirm("t", "s-1", "account-a", "r-1"), false);
  assert.equal(calls.post.length, 1);
  assert.equal(facade.getSnapshot().uncertain, true);
  assert.match(facade.getSnapshot().error, /resultado/i);
});

test("error POST queda incierto y no reintenta automáticamente", async () => {
  const { facade, calls } = setup({ postError: true });
  await facade.load("t", "s-1", "account-a");
  assert.equal(await facade.confirm("t", "s-1", "account-a", "r-1"), false);
  assert.equal(facade.getSnapshot().uncertain, true);
  assert.equal(calls.post.length, 1);
  assert.match(facade.getSnapshot().error, /resultado/i);
});

test("respuesta tardía de sesión anterior no contamina sesión nueva", async () => {
  let resolveOld;
  let calls = 0;
  const port = {
    listReturns: () =>
      ++calls === 1
        ? new Promise((resolve) => {
            resolveOld = resolve;
          })
        : Promise.resolve(result()),
    confirmVacancy: async () => {},
  };
  const facade = new ListingReturnsFacade(port);
  const pending = facade.load("t1", "s-1", "account-a");
  await facade.load("t2", "s-1", "account-b");
  resolveOld(result());
  await pending;
  assert.equal(facade.getSnapshot().sessionScope, "account-b");
});

test("vacancia incierta permite GET refresh y solo la confirmación autoritativa libera", async () => {
  const { facade, calls } = setup({
    read: (count) => (count >= 3 ? result(true) : result()),
  });
  await facade.load("t", "s-1", "account-a");
  await facade.confirm("t", "s-1", "account-a", "r-1");
  assert.equal(facade.getSnapshot().uncertain, true);
  await facade.load("t", "s-1", "account-a");
  assert.equal(facade.getSnapshot().uncertain, false);
  assert.equal(facade.getSnapshot().pendingReservationId, null);
  await facade.confirm("t", "s-1", "account-a", "r-1");
  assert.equal(calls.post.length, 1);
});

test("refresh incierto conserva bloqueo ante GET fallido, vacancy false, ausencia y espacio incorrecto", async () => {
  for (const read of [
    () => {
      throw Error("GET");
    },
    () => result(false),
    () => ({ space_id: "s-1", items: [] }),
    () => ({ space_id: "otro", items: [] }),
  ]) {
    let count = 0;
    const { facade } = setup({
      read: (n) => {
        count = n;
        return n === 1 ? result() : read();
      },
    });
    await facade.load("t", "s-1", "account-a");
    await facade.confirm("t", "s-1", "account-a", "r-1");
    await facade.load("t", "s-1", "account-a");
    assert.equal(count, 3);
    assert.equal(facade.getSnapshot().uncertain, true);
    assert.equal(facade.getSnapshot().pendingReservationId, "r-1");
  }
});

test("clear mientras POST responde invalida resultado tardío", async () => {
  let resolvePost;
  const port = {
    listReturns: async () => result(),
    confirmVacancy: () =>
      new Promise((resolve) => {
        resolvePost = resolve;
      }),
  };
  const facade = new ListingReturnsFacade(port);
  await facade.load("t", "s-1", "account-a");
  const pending = facade.confirm("t", "s-1", "account-a", "r-1");
  facade.clear();
  resolvePost();
  assert.equal(await pending, false);
  assert.equal(facade.getSnapshot().pendingReservationId, null);
  assert.equal(facade.getSnapshot().loading, false);
});

test("GET manual durante POST no invalida generación ni duplica POST", async () => {
  let resolvePost;
  const calls = { post: 0, get: 0 };
  const port = {
    listReturns: async () => {
      calls.get++;
      return result();
    },
    confirmVacancy: () => {
      calls.post++;
      return new Promise((resolve) => {
        resolvePost = resolve;
      });
    },
  };
  const facade = new ListingReturnsFacade(port);
  await facade.load("t", "s-1", "account-a");
  const pending = facade.confirm("t", "s-1", "account-a", "r-1");
  await facade.load("t", "s-1", "account-a");
  assert.equal(calls.get, 1);
  assert.equal(await facade.confirm("t", "s-1", "account-a", "r-1"), false);
  assert.equal(calls.post, 1);
  resolvePost();
  assert.equal(await pending, false);
  assert.equal(facade.getSnapshot().uncertain, true);
  assert.equal(facade.getSnapshot().loading, false);
});
