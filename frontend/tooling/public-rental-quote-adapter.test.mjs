import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";

const adapterUrl = new URL(
  "../src/features/public-catalog/infrastructure/rental-quote-openapi-adapter.ts",
  import.meta.url,
);

async function loadAdapter() {
  const source = await readFile(adapterUrl, "utf8");
  const ts = await import("typescript");
  const compiled = ts.transpileModule(
    source.replace('import "client-only";\n', ""),
    {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
      },
    },
  ).outputText;
  const mod = { exports: {} };
  const localRequire = createRequire(adapterUrl);
  localRequire.extensions[".ts"] = (module, filename) => {
    const tsSource = readFileSync(filename, "utf8");
    const output = ts.transpileModule(tsSource, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
      },
    }).outputText;
    module._compile(output, filename);
  };
  new Function("require", "module", "exports", compiled)(
    localRequire,
    mod,
    mod.exports,
  );
  return mod.exports;
}

const request = {
  space_id: "11111111-1111-4111-8111-111111111111",
  start_at: "2026-12-01T00:00:00Z",
  end_at: "2027-01-01T00:00:00Z",
};
const quote = {
  id: "33333333-3333-4333-8333-333333333333",
  ...request,
  space_title: "Bodega El Salto",
  rate_unit: "mes",
  unit_price_clp: "9007199254740993",
  currency: "CLP",
  units: 1,
  subtotal_clp: "9007199254740993",
  time_zone: "America/Santiago",
  created_at: "2026-11-14T05:00:00Z",
  expires_at: "2026-11-15T05:00:00Z",
};

test("rental quote adapter sends exact typed same-origin request and preserves int64 strings", async () => {
  const { createRentalQuoteAdapter } = await loadAdapter();
  const seen = [];
  const controller = new AbortController();
  const adapter = createRentalQuoteAdapter(async (req) => {
    seen.push(req);
    return Response.json({ data: quote }, { status: 201 });
  }, "http://localhost/api/backend");

  const result = await adapter.createQuote(request, controller.signal);
  assert.deepEqual(result, { ok: true, value: quote });
  assert.equal(seen.length, 1);
  assert.equal(seen[0].method, "POST");
  assert.equal(new URL(seen[0].url).pathname, "/api/backend/rental-quotes");
  assert.equal(seen[0].headers.has("authorization"), false);
  assert.equal(seen[0].credentials, "same-origin");
  assert.equal(seen[0].cache, "no-store");
  assert.equal(seen[0].signal.aborted, false);
  assert.deepEqual(await seen[0].clone().json(), request);
  controller.abort();
  assert.equal(seen[0].signal.aborted, true);
});

test("rental quote adapter rejects malformed or mismatched success envelopes", async () => {
  const { createRentalQuoteAdapter } = await loadAdapter();
  const invalid = [
    null,
    {},
    { ...quote, space_id: "22222222-2222-4222-8222-222222222222" },
    { ...quote, start_at: "2026-12-02T00:00:00Z" },
    { ...quote, end_at: "2027-01-02T00:00:00Z" },
    { ...quote, rate_unit: "year" },
    { ...quote, currency: "USD" },
    { ...quote, unit_price_clp: 1.5 },
    { ...quote, unit_price_clp: "9223372036854775808" },
    { ...quote, unit_price_clp: 0 },
    { ...quote, unit_price_clp: "1", subtotal_clp: "1" },
    { ...quote, unit_price_clp: "5000", subtotal_clp: "5000" },
    { ...quote, units: 0 },
    { ...quote, units: 2, subtotal_clp: 12000 },
    { ...quote, subtotal_clp: {} },
    { ...quote, end_at: "2026-11-30T00:00:00Z" },
    { ...quote, time_zone: "../UTC" },
    { ...quote, expires_at: "not-a-date" },
    { ...quote, expires_at: quote.created_at },
  ];
  for (const data of invalid) {
    const adapter = createRentalQuoteAdapter(
      async () => Response.json({ data }, { status: 201 }),
      "http://localhost/api/backend",
    );
    assert.deepEqual(await adapter.createQuote(request), {
      ok: false,
      error: { kind: "other", status: 502 },
    });
  }
  const adapter = createRentalQuoteAdapter(
    async () =>
      Response.json(
        { error: { message: "secret upstream detail" } },
        { status: 201 },
      ),
    "http://localhost/api/backend",
  );
  assert.deepEqual(await adapter.createQuote(request), {
    ok: false,
    error: { kind: "other", status: 502 },
  });
});

test("rental quote adapter maps documented statuses without exposing upstream messages", async () => {
  const { createRentalQuoteAdapter } = await loadAdapter();
  const expected = new Map([
    [400, "invalid"],
    [401, "unauthenticated"],
    [403, "forbidden"],
    [404, "not_found"],
    [409, "conflict"],
    [422, "invalid"],
    [503, "unavailable"],
    [500, "other"],
  ]);
  for (const [status, kind] of expected) {
    const adapter = createRentalQuoteAdapter(
      async () =>
        Response.json(
          {
            error: {
              code: "identity_required",
              message: "private upstream text",
            },
          },
          { status },
        ),
      "http://localhost/api/backend",
    );
    assert.deepEqual(await adapter.createQuote(request), {
      ok: false,
      error: { kind, status },
    });
  }
});
