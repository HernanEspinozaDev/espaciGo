import test from "node:test";
import assert from "node:assert/strict";
import { CatalogPaginationState } from "../../internal/mockserver/static/catalog-pagination-state.js";
import { actionWithButtonState } from "../../internal/mockserver/static/action-button-state.js";

test("next is unavailable while loading and after the final page",()=>{
  const state=new CatalogPaginationState();
  const first=state.beginRequest();
  assert.equal(state.canNext,false);
  state.finish(first,"opaque-next");
  assert.equal(state.canNext,true);
  assert.equal(state.beginNext(),"opaque-next");
  assert.equal(state.canNext,true,"choosing Next does not enter loading until action submits the form");
  const second=state.beginRequest();
  assert.equal(state.canNext,false);
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

test("a new form attempt invalidates a pending page before local validation",()=>{
  const state=new CatalogPaginationState();
  const pending=state.beginRequest();
  state.invalidate();
  assert.equal(state.canNext,false);
  assert.equal(state.accepts(pending,"account-a","account-a"),false);
  assert.equal(state.loading,false,"invalid local form data cannot leave the button stuck in loading");
});

test("an older response cannot replace the cursor from a newer request",()=>{
  const state=new CatalogPaginationState();
  const old=state.beginRequest(),current=state.beginRequest();
  state.finish(old,"obsolete");
  assert.equal(state.canNext,false);
  state.finish(current,"current");
  assert.equal(state.beginNext(),"current");
});

test("action restores other buttons, then recalculates Next from pagination state",async()=>{
  const state=new CatalogPaginationState();
  const button={disabled:false};
  const page=state.beginRequest();
  await actionWithButtonState([button],async()=>{
    assert.equal(button.disabled,true);
    state.finish(page,"page-two");
  },()=>{button.disabled=state.loading||!state.canNext;});
  assert.equal(button.disabled,false,"a cursor should enable Next after action restores buttons");

  const final=state.beginRequest();
  await actionWithButtonState([button],async()=>state.finish(final,""),()=>{button.disabled=state.loading||!state.canNext;});
  assert.equal(button.disabled,true,"the final page has no cursor");
});
