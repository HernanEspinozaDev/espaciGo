import test from "node:test";
import assert from "node:assert/strict";
import { publicationAction } from "../../internal/mockserver/static/space-publication-state.js";

test("only landlord role receives publication actions for valid local states",()=>{
  assert.equal(publicationAction("borrador",["arrendatario"]),null);
  assert.deepEqual(publicationAction("borrador",["arrendador"]),{label:"Publicar",nextState:"activa"});
  assert.deepEqual(publicationAction("oculta",["arrendador"]),{label:"Publicar",nextState:"activa"});
  assert.deepEqual(publicationAction("activa",["arrendador"]),{label:"Ocultar",nextState:"oculta"});
  assert.equal(publicationAction("desconocida",["arrendador"]),null);
});
