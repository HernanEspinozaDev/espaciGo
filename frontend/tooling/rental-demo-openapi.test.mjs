import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { parse } from "yaml";

const api = parse(
  await readFile(
    new URL("../../planning/openapi.yaml", import.meta.url),
    "utf8",
  ),
);
test("demo month, quote and confirmation contracts remain separate from commercial booking", () => {
  const root = "/local/rental-demo";
  for (const [route, method] of [
    ["/session", "post"],
    ["/spaces", "get"],
    ["/owner/spaces", "get"],
    ["/spaces/{spaceID}/month", "get"],
    ["/owner/spaces/{spaceID}/month", "get"],
    ["/owner/spaces/{spaceID}", "put"],
    ["/owner/spaces/{spaceID}/blocks", "post"],
    ["/owner/spaces/{spaceID}/blocks/{blockID}", "delete"],
    ["/quotes", "post"],
    ["/quotes/{quoteID}/confirm", "post"],
    ["/attempts/{intentID}", "get"],
  ]) {
    const op = api.paths[root + route]?.[method];
    assert.ok(op, `${method} ${root + route}`);
    assert.match(op.description, /memoria|reiniciable/);
    const success = Object.entries(op.responses).find(([status]) =>
      /^2/.test(status),
    )?.[1];
    assert.equal(success.headers["Cache-Control"].schema.const, "no-store");
  }
  assert.deepEqual(
    api.paths[root + "/spaces"].get.security,
    [],
  );
  assert.deepEqual(
    api.paths[root + "/spaces/{spaceID}/month"].get.security,
    [],
  );
  for (const [p, m] of [
    ["/owner/spaces", "get"],
    ["/owner/spaces/{spaceID}/month", "get"],
    ["/owner/spaces/{spaceID}", "put"],
    ["/owner/spaces/{spaceID}/blocks", "post"],
    ["/owner/spaces/{spaceID}/blocks/{blockID}", "delete"],
    ["/quotes", "post"],
    ["/quotes/{quoteID}/confirm", "post"],
    ["/attempts/{intentID}", "get"],
  ]) {
    assert.deepEqual(
      api.paths[root + p][m].security,
      [{ RentalDemoSession: [] }],
      `Endpoint ${m.toUpperCase()} ${root + p} must enforce RentalDemoSession security`,
    );
  }
  assert.deepEqual(
    Object.keys(api.components.schemas.DemoMonth.properties).sort(),
    [
      "space_id",
      "title",
      "time_zone",
      "rate_unit",
      "unit_price_clp",
      "month",
      "today",
      "days",
    ].sort(),
  );
  assert.equal(api.components.schemas.DemoMonth.additionalProperties, false);
  assert.equal(
    api.components.schemas.DemoQuote.properties.subtotal_clp.type,
    "string",
  );
  assert.equal(
    api.components.schemas.DemoConfirmRequest.properties.approved_contract
      .const,
    true,
  );
  assert.equal(
    api.components.schemas.DemoConfirmation.properties.payment_status.const,
    "paid_simulated",
  );
  assert.equal(api.components.schemas.DemoQuote.additionalProperties, false);
  assert.equal(api.components.schemas.DemoConfirmRequest.additionalProperties, false);
  assert.equal(api.components.schemas.DemoBlockRequest.additionalProperties, false);
  assert.equal(api.components.schemas.DemoSpace.additionalProperties, false);
  assert.equal(api.components.schemas.DemoDay.additionalProperties, false);
  assert.equal(api.components.schemas.DemoInterval.additionalProperties, false);
  assert.equal(api.components.schemas.DemoSession.additionalProperties, false);
  assert.deepEqual(api.components.schemas.DemoSession.properties.actor.enum, ["host", "renter", "renter2"]);
  assert.equal(api.components.schemas.DemoLogin.additionalProperties, false);
  assert.deepEqual(api.components.schemas.DemoLogin.properties.actor.enum, ["host", "renter", "renter2"]);

  const demoPaths = Object.keys(api.paths).filter((p) => p.startsWith("/local/rental-demo"));
  assert.equal(demoPaths.length, 11, "Must maintain exactly 11 isolated demo endpoints");
});
