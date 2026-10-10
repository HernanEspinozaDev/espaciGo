import test from "node:test";
import assert from "node:assert/strict";
import {AdminAuditState} from "../../internal/mockserver/static/admin-audit-state.js";

test("criteria changes invalidate pages and stale requests",()=>{
  const state=new AdminAuditState();state.updateCriteria("filters-a");
  const old=state.begin("admin","token-a",1);state.nextCursor="cursor-a";
  state.updateCriteria("filters-b");
  assert.equal(state.nextCursor,"");assert.equal(state.current(old,"admin","token-a",1),false);
});
test("logout and relogin as the same account invalidates pending query and export",()=>{
  const state=new AdminAuditState();state.updateCriteria("filters");const pending=state.begin("admin","token",1);
  state.clear();state.updateCriteria("filters");
  assert.equal(state.current(pending,"admin","token",2),false);
  assert.equal(state.current(pending,"admin","new-token",3),false);
});
test("only the current criteria and session may deliver a delayed page or blob",async()=>{
  const state=new AdminAuditState();state.updateCriteria("from=a");const context=state.begin("admin","token-a",4);
  let resolve;const pending=new Promise(r=>resolve=r);let delivered=0;
  const operation=(async()=>{const blob=await pending;if(!state.current(context,"admin","token-a",4))return false;delivered++;return blob;})();
  state.updateCriteria("from=b");resolve(new Blob(["old"]));
  assert.equal(await operation,false);assert.equal(delivered,0);
});
