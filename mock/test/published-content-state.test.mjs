import test from "node:test";
import assert from "node:assert/strict";
import { publishedContentChange } from "../../internal/mockserver/static/published-content-state.js";

const loaded = {title:"Cargador",description:"Descripción original de prueba",capacity:2,usage_rules:"No fumar",base_price_clp:8000};

test("published content builds only changed fields and skips an unchanged form", () => {
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"2",base_price_clp:"8000" ,title:"Título nuevo"}), {
    kind: "update", body: { title: "Título nuevo" },
  });
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"2",base_price_clp:"12000"}), {
    kind: "update", body: { base_price_clp: 12000 },
  });
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"2",base_price_clp:"8000",description:"Descripción nueva"}), {
    kind: "update", body: { description: "Descripción nueva" },
  });
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"5",base_price_clp:"8000",usage_rules:"No mascotas"}), {
    kind: "update", body: { capacity: 5, usage_rules: "No mascotas" },
  });
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"2",base_price_clp:"8000"}), { kind: "unchanged" });
});

test("published content refuses CLP values JavaScript cannot represent exactly", () => {
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"2",base_price_clp:"9007199254740992"}), { kind: "unsupported-price" });
  assert.deepEqual(publishedContentChange({...loaded,base_price_clp:Number.MAX_SAFE_INTEGER + 1}, {...loaded,capacity:"2",base_price_clp:"",title:"Título nuevo"}), {
    kind: "update", body: { title: "Título nuevo" },
  });
  assert.deepEqual(publishedContentChange(loaded, {...loaded,capacity:"2147483648",base_price_clp:"8000"}), { kind: "invalid-capacity" });
});
