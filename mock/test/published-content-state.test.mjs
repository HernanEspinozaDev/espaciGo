import test from "node:test";
import assert from "node:assert/strict";
import { publishedContentChange } from "../../internal/mockserver/static/published-content-state.js";

test("published content builds only changed fields and skips an unchanged form", () => {
  assert.deepEqual(publishedContentChange("Cargador", 8000, "Título nuevo", "8000"), {
    kind: "update", body: { title: "Título nuevo" },
  });
  assert.deepEqual(publishedContentChange("Cargador", 8000, "Cargador", "12000"), {
    kind: "update", body: { base_price_clp: 12000 },
  });
  assert.deepEqual(publishedContentChange("Cargador", 8000, "Cargador", "8000"), { kind: "unchanged" });
});

test("published content refuses CLP values JavaScript cannot represent exactly", () => {
  assert.deepEqual(publishedContentChange("Cargador", 8000, "Cargador", "9007199254740992"), { kind: "unsupported-price" });
  assert.deepEqual(publishedContentChange("Cargador", Number.MAX_SAFE_INTEGER + 1, "Título nuevo", ""), {
    kind: "update", body: { title: "Título nuevo" },
  });
});
