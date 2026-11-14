import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";

const sourcePath = new URL(
  "../src/features/rental-demo/infrastructure/demo-openapi-adapter.ts",
  import.meta.url,
);

async function loadAdapter() {
  const source = await readFile(sourcePath, "utf8");
  const ts = await import("typescript");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const mod = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    createRequire(sourcePath),
    mod,
    mod.exports,
  );
  return mod.exports;
}

test("demo fetch transport rewrites only the generated local path and never forwards product auth", async () => {
  const { createRentalDemoTransport } = await loadAdapter();
  const seen = [];
  const transport = createRentalDemoTransport(async (request) => {
    seen.push(request);
    return new Response("[]", {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  });
  await transport.fetch(
    new Request("http://local/api/local/rental-demo/spaces", {
      headers: { authorization: "Bearer must-not-forward" },
    }),
  );
  await transport.fetch(new Request("http://local/api/local/other"));
  assert.equal(new URL(seen[0].url).pathname, "/api/rental-demo/spaces");
  assert.equal(new URL(seen[1].url).pathname, "/api/local/other");
  assert.equal(seen[0].credentials, "same-origin");
  assert.equal(seen[0].headers.has("authorization"), false);
});

test("adapter performs a typed public-spaces read through its injected fetch seam", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const seen = [];
  const adapter = createRentalDemoAdapter(async (request) => {
    seen.push(request);
    return new Response(JSON.stringify([]), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }, "http://local/api");
  assert.deepEqual(await adapter.spaces(false), []);
  assert.equal(new URL(seen[0].url).pathname, "/api/rental-demo/spaces");
  assert.equal(seen[0].credentials, "same-origin");
  assert.equal(seen[0].redirect, "error");

  assert.deepEqual(await adapter.spaces(true), []);
  assert.equal(new URL(seen[1].url).pathname, "/api/rental-demo/owner/spaces");
});

test("adapter month method routes correctly for public vs owner", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const seen = [];
  const adapter = createRentalDemoAdapter(async (request) => {
    seen.push(request.url);
    return new Response(JSON.stringify({ space_id: "sp-1", days: [] }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }, "http://local/api");
  const spaceId = "00000000-0000-4000-8000-000000000001";

  await adapter.month(spaceId, "2030-01", 1, false);
  assert.equal(
    new URL(seen[0]).pathname,
    `/api/rental-demo/spaces/${spaceId}/month`,
  );
  assert.equal(new URL(seen[0]).search, "?month=2030-01&duration=1");

  await adapter.month(spaceId, "2030-01", 1, true);
  assert.equal(
    new URL(seen[1]).pathname,
    `/api/rental-demo/owner/spaces/${spaceId}/month`,
  );
  assert.equal(new URL(seen[1]).search, "?month=2030-01&duration=1");
});

test("demo transport preserves query and rewrites only the exact prefix", async () => {
  const { createRentalDemoTransport } = await loadAdapter();
  let observed;
  const transport = createRentalDemoTransport(async (request) => {
    observed = request;
    return new Response("{}", { status: 200 });
  });
  await transport.fetch(
    new Request(
      "http://local/api/local/rental-demo/spaces/abc/month?month=2026-11&duration=1",
    ),
  );
  assert.equal(
    observed.url,
    "http://local/api/rental-demo/spaces/abc/month?month=2026-11&duration=1",
  );
});

test("adapter session returns null on 401 unauthenticated and data on 200", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const unauthAdapter = createRentalDemoAdapter(async () => {
    return new Response(
      JSON.stringify({ error: { code: "unauthenticated" } }),
      {
        status: 401,
        headers: { "content-type": "application/json" },
      },
    );
  }, "http://local/api");
  assert.equal(await unauthAdapter.session(), null);

  const authAdapter = createRentalDemoAdapter(async () => {
    return new Response(JSON.stringify({ actor: "renter" }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }, "http://local/api");
  assert.deepEqual(await authAdapter.session(), { actor: "renter" });
});

test("adapter login sends POST with actor payload to session endpoint", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  let seenUrl, seenMethod, seenBody;
  const adapter = createRentalDemoAdapter(async (request) => {
    seenUrl = request.url;
    seenMethod = request.method;
    seenBody = JSON.parse(await request.text());
    return new Response(
      JSON.stringify({ actor: "renter", expires_at: "2030-01-15T20:00:00Z" }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  }, "http://local/api");

  const session = await adapter.login("renter");
  assert.equal(seenMethod, "POST");
  assert.equal(new URL(seenUrl).pathname, "/api/rental-demo/session");
  assert.deepEqual(seenBody, { actor: "renter" });
  assert.equal(session.actor, "renter");
});

test("adapter deleteBlock passes If-Match header with quotes", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  let observedHeader;
  const adapter = createRentalDemoAdapter(async (request) => {
    observedHeader = request.headers.get("if-match");
    return new Response(null, { status: 204 });
  }, "http://local/api");
  const blockId = "00000000-0000-4000-8000-000000000001";
  const spaceId = "00000000-0000-4000-8000-000000000002";
  await adapter.deleteBlock(spaceId, blockId, "rev-5");
  assert.equal(observedHeader, '"rev-5"');
});

test("adapter deleteBlock throws descriptive error when upstream rejects with 409", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const adapter = createRentalDemoAdapter(async () => {
    return new Response(JSON.stringify({ error: { code: "conflict" } }), {
      status: 409,
      headers: { "content-type": "application/json" },
    });
  }, "http://local/api");
  const blockId = "00000000-0000-4000-8000-000000000001";
  const spaceId = "00000000-0000-4000-8000-000000000002";
  await assert.rejects(
    () => adapter.deleteBlock(spaceId, blockId, "stale-rev"),
    { message: "No se pudo retirar el bloqueo." },
  );
});

test("adapter quote and confirm methods construct correct payloads and routes", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  let seenUrl, seenMethod, seenBody;
  const adapter = createRentalDemoAdapter(async (request) => {
    seenUrl = request.url;
    seenMethod = request.method;
    seenBody = await request.json();
    return new Response(
      JSON.stringify({ id: "mock-id", status: "confirmed_demo" }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  }, "http://local/api");

  const quoteId = "00000000-0000-4000-8000-000000000010";
  const intentId = "00000000-0000-4000-8000-000000000020";
  await adapter.confirm(quoteId, {
    approved_contract: true,
    intent_id: intentId,
  });
  assert.equal(seenMethod, "POST");
  assert.equal(
    new URL(seenUrl).pathname,
    `/api/rental-demo/quotes/${quoteId}/confirm`,
  );
  assert.deepEqual(seenBody, { approved_contract: true, intent_id: intentId });
});

test("adapter attempt and logout query correct endpoints", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const calls = [];
  const adapter = createRentalDemoAdapter(async (request) => {
    calls.push({ method: request.method, path: new URL(request.url).pathname });
    if (request.method === "DELETE") {
      return new Response(null, { status: 204 });
    }
    return new Response(
      JSON.stringify({ id: "mock-attempt", status: "confirmed_demo" }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  }, "http://local/api");

  const intentId = "00000000-0000-4000-8000-000000000055";
  const attemptResult = await adapter.attempt(intentId);
  assert.equal(attemptResult.id, "mock-attempt");
  await adapter.logout();

  assert.deepEqual(calls, [
    { method: "GET", path: `/api/rental-demo/attempts/${intentId}` },
    { method: "DELETE", path: "/api/rental-demo/session" },
  ]);
});

test("adapter logout and quote throw descriptive error when upstream rejects", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const adapter = createRentalDemoAdapter(async () => {
    return new Response(JSON.stringify({ error: { code: "upstream_error" } }), {
      status: 502,
      headers: { "content-type": "application/json" },
    });
  }, "http://local/api");

  await assert.rejects(() => adapter.logout(), {
    message: "No se pudo cerrar la sesión de demostración.",
  });

  await assert.rejects(
    () => adapter.quote("sp-1", "2030-01-16T10:00:00Z", "2030-01-16T11:00:00Z"),
    { message: "No se pudo completar la operación de demostración." },
  );

  await assert.rejects(
    () =>
      adapter.confirm("q-1", {
        approved_contract: true,
        intent_id: "00000000-0000-4000-8000-000000000001",
      }),
    { message: "No se pudo completar la operación de demostración." },
  );

  await assert.rejects(
    () => adapter.attempt("00000000-0000-4000-8000-000000000001"),
    { message: "No se pudo completar la operación de demostración." },
  );

  await assert.rejects(
    () =>
      adapter.createBlock("sp-1", {
        expected_revision: "r1",
        start_at: "2030-01-16T10:00:00Z",
        end_at: "2030-01-16T11:00:00Z",
      }),
    { message: "No se pudo completar la operación de demostración." },
  );

  await assert.rejects(
    () => adapter.updateSpace("sp-1", { state: "borrador" }),
    { message: "No se pudo completar la operación de demostración." },
  );
});

test("adapter createBlock and updateSpace invoke correct owner routes with payloads", async () => {
  const { createRentalDemoAdapter } = await loadAdapter();
  const calls = [];
  const adapter = createRentalDemoAdapter(async (request) => {
    const body = request.body ? JSON.parse(await request.text()) : null;
    calls.push({
      method: request.method,
      path: new URL(request.url).pathname,
      body,
    });
    if (request.method === "POST") {
      return new Response(JSON.stringify({ id: "blk-created-123" }), {
        status: 201,
        headers: { "content-type": "application/json" },
      });
    }
    return new Response(
      JSON.stringify({ id: "space-updated-456", state: "activa" }),
      {
        status: 200,
        headers: { "content-type": "application/json" },
      },
    );
  }, "http://local/api");

  const spaceId = "00000000-0000-4000-8000-000000000099";
  const blockReq = {
    expected_revision: "rev-1",
    start_at: "2030-01-16T10:00:00Z",
    end_at: "2030-01-16T12:00:00Z",
  };
  const createdId = await adapter.createBlock(spaceId, blockReq);
  assert.equal(createdId, "blk-created-123");

  const config = { state: "activa" };
  const updated = await adapter.updateSpace(spaceId, config);
  assert.equal(updated.id, "space-updated-456");

  assert.deepEqual(calls, [
    {
      method: "POST",
      path: `/api/rental-demo/owner/spaces/${spaceId}/blocks`,
      body: blockReq,
    },
    {
      method: "PUT",
      path: `/api/rental-demo/owner/spaces/${spaceId}`,
      body: config,
    },
  ]);
});
