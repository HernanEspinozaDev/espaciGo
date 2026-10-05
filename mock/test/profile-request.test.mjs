import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const compiledSource = await readFile(new URL("../../internal/mockserver/static/profile-request.js", import.meta.url), "utf8");
const { ProfileRequestGate, profileMatchesSelection } = await import(`data:text/javascript;base64,${Buffer.from(compiledSource).toString("base64")}`);

test("ignores an earlier category reply that arrives after the selection changed", () => {
  const gate = new ProfileRequestGate();
  const officeRequest = gate.begin();
  const warehouseRequest = gate.begin();

  assert.equal(gate.accepts(officeRequest, "oficina", "bodega"), false);
  assert.equal(gate.accepts(warehouseRequest, "bodega", "bodega"), true);
});

test("blocks saving until the profile belongs to the current selection", () => {
  assert.equal(profileMatchesSelection("oficina", "bodega"), false);
  assert.equal(profileMatchesSelection("bodega", "bodega"), true);
  assert.equal(profileMatchesSelection(undefined, "bodega"), false);
});
