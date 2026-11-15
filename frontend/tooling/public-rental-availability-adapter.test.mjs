import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";

const sourcePath = new URL(
  "../src/features/public-catalog/infrastructure/rental-availability-openapi-adapter.ts",
  import.meta.url,
);

async function loadAdapter() {
  let source;
  try {
    source = await readFile(sourcePath, "utf8");
  } catch {
    assert.fail("commercial availability adapter must exist before use");
  }
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
  new Function("require", "module", "exports", compiled)(
    createRequire(sourcePath),
    mod,
    mod.exports,
  );
  return mod.exports;
}

test("public availability adapter calls the generated operation with exact query and no browser Bearer", async () => {
  const { createRentalAvailabilityAdapter } = await loadAdapter();
  const seen = [];
  const controller = new AbortController();
  const signal = controller.signal;
  const adapter = createRentalAvailabilityAdapter(async (request) => {
    seen.push(request);
    return Response.json({
      data: {
        space_id: "11111111-1111-4111-8111-111111111111",
        time_zone: "America/Santiago",
        rate_unit: "hora",
        checked_at: "2026-11-15T12:00:00Z",
        items: [
          {
            start_at: "2026-12-01T14:00:00Z",
            end_at: "2026-12-01T16:00:00Z",
          },
        ],
      },
    });
  }, "http://localhost/api/backend");

  const result = await adapter.getAvailability(
    "11111111-1111-4111-8111-111111111111",
    "2026-12-01",
    2,
    signal,
  );

  assert.equal(result.ok, true);
  assert.equal(result.value.items[0].start_at, "2026-12-01T14:00:00Z");
  assert.equal(seen.length, 1);
  assert.equal(seen[0].method, "GET");
  const url = new URL(seen[0].url);
  assert.equal(
    url.pathname,
    "/api/backend/spaces/11111111-1111-4111-8111-111111111111/rental-availability",
  );
  assert.equal(url.searchParams.get("date"), "2026-12-01");
  assert.equal(url.searchParams.get("duration"), "2");
  assert.equal(seen[0].headers.has("authorization"), false);
  assert.equal(seen[0].credentials, "same-origin");
  assert.equal(seen[0].signal.aborted, false);
  controller.abort();
  assert.equal(seen[0].signal.aborted, true);
});

test("public availability adapter maps missing publication and malformed success safely", async () => {
  const { createRentalAvailabilityAdapter } = await loadAdapter();
  const notFound = createRentalAvailabilityAdapter(
    async () =>
      Response.json({ error: { code: "not_found" } }, { status: 404 }),
    "http://localhost/api/backend",
  );
  assert.deepEqual(
    await notFound.getAvailability(
      "11111111-1111-4111-8111-111111111111",
      "2026-12-01",
      1,
    ),
    { ok: false, error: { kind: "not-found", status: 404, code: "not_found" } },
  );

  const malformed = createRentalAvailabilityAdapter(
    async () => Response.json({ data: { items: "not-an-array" } }),
    "http://localhost/api/backend",
  );
  assert.deepEqual(
    await malformed.getAvailability(
      "11111111-1111-4111-8111-111111111111",
      "2026-12-01",
      1,
    ),
    { ok: false, error: { kind: "unavailable", status: 502 } },
  );
});
