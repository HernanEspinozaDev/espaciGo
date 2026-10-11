import test from "node:test";
import assert from "node:assert/strict";
import { CatalogPaginationState } from "../../internal/mockserver/static/catalog-pagination-state.js";
import { clearCatalogFilters } from "../../internal/mockserver/static/catalog-filter-actions.js";
import { SyntheticMapViewport } from "../../internal/mockserver/static/synthetic-map-state.js";

test("limpiar filtros invalida respuestas, cursores, selección y cotización antes de reiniciar", () => {
  const pagination = new CatalogPaginationState();
  const pendingRequest = pagination.beginRequest();
  pagination.finish(pendingRequest, "cursor-next");
  const next = pagination.beginNext();
  assert.equal(next, "cursor-next");
  const oldRequest = pagination.beginRequest();
  const values = { q: "oficina", minArea: "40", category: "oficina", cursor: "old", selected: "space-a", quote: "quote-a" };
  const order = [];
  clearCatalogFilters({
    invalidatePending: () => { order.push("invalidate"); pagination.invalidate(); },
    resetControls: () => { order.push("reset"); Object.assign(values, { q: "", minArea: "", category: "" }); },
    clearProfile: () => { order.push("profile"); },
    clearCursors: () => { order.push("cursor"); values.cursor = ""; },
    clearSelection: () => { order.push("selection"); values.selected = ""; values.quote = ""; },
    resetMap: () => { order.push("map"); },
  });
  assert.deepEqual(order, ["invalidate", "reset", "profile", "cursor", "selection", "map"]);
  assert.deepEqual(values, { q: "", minArea: "", category: "", cursor: "", selected: "", quote: "" });
  assert.equal(pagination.accepts(oldRequest, "same-session", "same-session"), false);
  assert.equal(pagination.canNext, false);
});

test("el plano sintético permite acercar, desplazar, filtrar pines fuera de vista y reiniciar", () => {
  const map = new SyntheticMapViewport();
  const west = { x: 0, y: 50 }, east = { x: 100, y: 50 };
  assert.equal(map.isVisible(west), true);
  map.zoomBy(2);
  assert.equal(map.isVisible(east), false);
  map.panBy(-50, 0);
  assert.equal(map.isVisible(east), true);
  map.reset();
  assert.deepEqual(map.project(west), west);
  assert.equal(map.zoom, 1);
});

test("acercar, desplazar y alejar vuelve a mostrar los pines al reducir el zoom", () => {
  const map = new SyntheticMapViewport();
  const west = { x: 0, y: 50 }, east = { x: 100, y: 50 };
  map.zoomBy(2);
  map.panBy(-50, 0);
  assert.equal(map.zoom, 2);
  assert.equal(map.panX, -50);
  assert.equal(map.isVisible(west), false);
  assert.equal(map.isVisible(east), true);

  map.zoomBy(0.5);
  assert.equal(map.zoom, 1);
  assert.equal(map.panX, 0, "pan is clamped to the zero extent at zoom 1");
  assert.equal(map.isVisible(west), true);
  assert.equal(map.isVisible(east), true, "the pins return without a manual reset");
});
