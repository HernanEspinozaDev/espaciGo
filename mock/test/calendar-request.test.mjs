import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

const compiledSource = await readFile(new URL("../../internal/mockserver/static/calendar-request.js", import.meta.url), "utf8");
const { CalendarRequestState } = await import(`data:text/javascript;base64,${Buffer.from(compiledSource).toString("base64")}`);

test("requires the API-confirmed zone and blocks use when the field has unsaved edits", () => {
  const state = new CalendarRequestState();
  const selected = state.beginSelection();
  assert.equal(state.confirm("draft-a", "America/Santiago", selected, "draft-a"), true);
  assert.equal(state.zoneFor("draft-a", "America/Santiago"), "America/Santiago");
  assert.throws(() => state.zoneFor("draft-a", "Pacific/Auckland"), /Guarda la zona horaria/);
});

test("discards an out-of-order zone reply from the previously selected draft", () => {
  const state = new CalendarRequestState();
  const firstSelection = state.beginSelection();
  const secondSelection = state.beginSelection();

  assert.equal(state.confirm("draft-a", "America/Santiago", firstSelection, "draft-b"), false);
  assert.equal(state.confirm("draft-b", "Pacific/Auckland", secondSelection, "draft-b"), true);
  assert.throws(() => state.zoneFor("draft-a", "America/Santiago"), /Consulta la zona/);
  assert.equal(state.zoneFor("draft-b", "Pacific/Auckland"), "Pacific/Auckland");
});
