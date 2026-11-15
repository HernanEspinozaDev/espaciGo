import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";

const adapterUrl = new URL(
  "../src/features/public-catalog/infrastructure/contract-preview-openapi-adapter.ts",
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
    const tsCompiled = ts.transpileModule(tsSource, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
      },
    }).outputText;
    module._compile(tsCompiled, filename);
  };
  new Function("require", "module", "exports", compiled)(
    localRequire,
    mod,
    mod.exports,
  );
  return mod.exports;
}

const quoteId = "33333333-3333-4333-8333-333333333333";
const preview = {
  state: "preview",
  quote_id: quoteId,
  space_id: "11111111-1111-4111-8111-111111111111",
  space_title: "Bodega El Salto",
  rate_unit: "mes",
  unit_price_clp: 180000,
  units: 1,
  subtotal_clp: 180000,
  currency: "CLP",
  start_at: "2026-12-01T00:00:00Z",
  end_at: "2027-01-01T00:00:00Z",
  time_zone: "America/Santiago",
  parties: [
    {
      role: "arrendador",
      legal_name: "Carlos Propietario",
      rut: "12.345.678-9",
    },
    {
      role: "arrendatario",
      legal_name: "Ana Arrendataria",
      rut: "98.765.432-1",
    },
  ],
  generated_at: "2026-11-14T05:00:00Z",
};

test("contract preview adapter uses typed POST operation without browser Bearer and preserves no-store", async () => {
  const { createContractPreviewAdapter } = await loadAdapter();
  const seen = [];
  const controller = new AbortController();
  const adapter = createContractPreviewAdapter(async (request) => {
    seen.push(request);
    return Response.json({ data: preview });
  }, "http://localhost/api/backend");

  const result = await adapter.getPreview(quoteId, controller.signal);

  assert.deepEqual(result, { ok: true, value: preview });
  assert.equal(seen.length, 1);
  assert.equal(seen[0].method, "POST");
  assert.equal(
    new URL(seen[0].url).pathname,
    `/api/backend/rental-quotes/${quoteId}/contract-preview`,
  );
  assert.equal(seen[0].headers.has("authorization"), false);
  assert.equal(seen[0].credentials, "same-origin");
  assert.equal(seen[0].cache, "no-store");
  assert.equal(seen[0].signal.aborted, false);
  controller.abort();
  assert.equal(seen[0].signal.aborted, true);
  assert.equal(seen[0].body, null);
});

test("contract preview adapter rejects malformed or cross-quote success payloads", async () => {
  const { createContractPreviewAdapter } = await loadAdapter();
  for (const data of [
    { ...preview, quote_id: "44444444-4444-4444-8444-444444444444" },
    { ...preview, parties: [{ role: "arrendador", legal_name: 7, rut: "x" }] },
    { ...preview, subtotal_clp: {} },
    { ...preview, unit_price_clp: 1.5 },
  ]) {
    const adapter = createContractPreviewAdapter(
      async () => Response.json({ data }),
      "http://localhost/api/backend",
    );
    assert.deepEqual(await adapter.getPreview(quoteId), {
      ok: false,
      error: { kind: "other", status: 502 },
    });
  }
});

test("contract preview adapter preserves documented identity, conflict, and not-found errors", async () => {
  const { createContractPreviewAdapter } = await loadAdapter();
  const cases = [
    [409, { error: { code: "identity_required" } }, "identity_required"],
    [409, { error: { code: "conflict" } }, "conflict"],
    [404, { error: { code: "not_found" } }, "not_found"],
  ];
  for (const [status, body, kind] of cases) {
    const adapter = createContractPreviewAdapter(
      async () => Response.json(body, { status }),
      "http://localhost/api/backend",
    );
    assert.deepEqual(await adapter.getPreview(quoteId), {
      ok: false,
      error: { kind, status },
    });
  }
});
