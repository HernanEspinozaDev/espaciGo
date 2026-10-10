import test from "node:test";
import assert from "node:assert/strict";
import {AdminAuditState} from "../../internal/mockserver/static/admin-audit-state.js";
import {consumeAdminAuditExportResponse} from "../../internal/mockserver/static/admin-audit-export-state.js";

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

test("a 401 from the current export clears the session and administrative view",async()=>{
  const state=new AdminAuditState();state.updateCriteria("filters");state.nextCursor="next";
  const context=state.begin("admin-a","token-a",1);let session={account:"admin-a",token:"token-a",generation:1,roles:["administrador"]},cleared=false;
  const outcome=await consumeAdminAuditExportResponse(new Response(JSON.stringify({error:{code:"session_expired"}}),{status:401}),state,context,c=>state.current(c,session.account,session.token,session.generation),c=>Boolean(c.token)&&c.accountID===session.account&&c.token===session.token&&c.generation===session.generation,()=>{session={account:"admin-a",token:"",generation:2,roles:[]};state.clear();cleared=true;},()=>{});
  assert.equal(outcome.kind,"unauthorized");assert.equal(cleared,true);assert.equal(session.token,"");assert.equal(session.roles.length,0);assert.equal(state.nextCursor,"");
});

test("a delayed old-session export 401 cannot clear a relogged session",async()=>{
  const state=new AdminAuditState();state.updateCriteria("filters");
  const oldContext=state.begin("admin-a","token-old",3);let session={account:"admin-a",token:"token-old",generation:3,roles:["administrador"]},clears=0;
  let resolveBody;const body=new Promise(resolve=>{resolveBody=resolve;});const response=new Response("",{status:401});response.json=()=>body;
  const pending=consumeAdminAuditExportResponse(response,state,oldContext,c=>state.current(c,session.account,session.token,session.generation),c=>Boolean(c.token)&&c.accountID===session.account&&c.token===session.token&&c.generation===session.generation,()=>{clears++;state.clear();session={account:"admin-a",token:"",generation:4,roles:[]};},()=>{});
  state.clear();session={account:"admin-a",token:"token-new",generation:5,roles:["administrador"]};state.updateCriteria("filters");
  resolveBody(JSON.stringify({error:{code:"session_expired"}}));
  const outcome=await pending;
  assert.equal(outcome.kind,"stale");assert.equal(clears,0);assert.equal(session.token,"token-new");assert.deepEqual(session.roles,["administrador"]);
});

test("a 403 clears the administrative view but preserves the session",async()=>{
  const state=new AdminAuditState();state.updateCriteria("filters");state.nextCursor="next";
  const context=state.begin("admin-a","token-a",1);let session={account:"admin-a",token:"token-a",generation:1,roles:["administrador","arrendatario"]},refreshed=false;
  const outcome=await consumeAdminAuditExportResponse(new Response(JSON.stringify({error:{code:"forbidden"}}),{status:403}),state,context,c=>state.current(c,session.account,session.token,session.generation),c=>Boolean(c.token)&&c.accountID===session.account&&c.token===session.token&&c.generation===session.generation,()=>assert.fail("403 must not clear session"),()=>{state.clear();session.roles=["arrendatario"];refreshed=true;});
  assert.equal(outcome.kind,"forbidden");assert.equal(refreshed,true);assert.equal(session.token,"token-a");assert.deepEqual(session.roles,["arrendatario"]);assert.equal(state.nextCursor,"");
});

test("a 401 after filters change clears the same session despite stale results",async()=>{
  const state=new AdminAuditState();state.updateCriteria("filters-a");
  const context=state.begin("admin-a","token-a",1);let session={account:"admin-a",token:"token-a",generation:1},cleared=false;
  state.updateCriteria("filters-b");
  const outcome=await consumeAdminAuditExportResponse(new Response(JSON.stringify({error:{code:"session_expired"}}),{status:401}),state,context,c=>state.current(c,session.account,session.token,session.generation),c=>Boolean(c.token)&&c.accountID===session.account&&c.token===session.token&&c.generation===session.generation,()=>{cleared=true;session={account:"admin-a",token:"",generation:2};state.clear();},()=>{});
  assert.equal(state.current(context,"admin-a","token-a",1),false);assert.equal(outcome.kind,"unauthorized");assert.equal(cleared,true);assert.equal(session.token,"");
});
