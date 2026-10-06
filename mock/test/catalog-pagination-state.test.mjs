import test from "node:test";
import assert from "node:assert/strict";
import { CatalogPaginationState } from "../../internal/mockserver/static/catalog-pagination-state.js";

test("next is unavailable while loading and after the final page",()=>{
  const state=new CatalogPaginationState();
  const first=state.beginRequest();
  assert.equal(state.canNext,false);
  state.finish(first,"opaque-next");
  assert.equal(state.canNext,true);
  assert.equal(state.beginNext(),"opaque-next");
  assert.equal(state.canNext,false);
  const second=state.beginRequest();
  state.finish(second,"");
  assert.equal(state.canNext,false);
});

test("filter, page and session changes discard stale responses",()=>{
  const state=new CatalogPaginationState();
  const first=state.beginRequest();
  state.invalidate();
  assert.equal(state.accepts(first,"account-a","account-a"),false);
  const second=state.beginRequest();
  assert.equal(state.accepts(second,"account-a","account-b"),false);
  assert.equal(state.accepts(second,"account-a","account-a"),true);
});

test("an older response cannot replace the cursor from a newer request",()=>{
  const state=new CatalogPaginationState();
  const old=state.beginRequest(),current=state.beginRequest();
  state.finish(old,"obsolete");
  assert.equal(state.canNext,false);
  state.finish(current,"current");
  assert.equal(state.beginNext(),"current");
});
