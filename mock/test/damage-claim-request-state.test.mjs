import test from "node:test";
import assert from "node:assert/strict";
import { damageClaimRequestIsCurrent, finishDamageClaimAfterReload } from "../../internal/mockserver/static/damage-claim-request-state.js";

const host="aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const original={requestRevision:3,selectionRevision:8,selectionID:"claim-a",accountID:host,token:"session-a",generation:4};

test("late admin refresh does not restore the previous claim after selection changes",async()=>{
  let release;const delayed=new Promise(resolve=>{release=resolve;});
  let current={...original},rendered="claim-b",success=false;
  const completion=finishDamageClaimAfterReload(original,()=>delayed,()=>current,()=>{rendered="claim-a";success=true;});
  current={...current,selectionRevision:9,selectionID:"claim-b"};
  release();
  assert.equal(await completion,false);
  assert.equal(rendered,"claim-b");
  assert.equal(success,false);
});

test("late claim response cannot show success after logout and relogin to the same account",async()=>{
  let release;const delayed=new Promise(resolve=>{release=resolve;});
  let current={...original},message="";
  const completion=finishDamageClaimAfterReload(original,()=>delayed,()=>current,()=>{message="guardado";});
  current={...current,token:"session-b",generation:6};
  release();
  assert.equal(await completion,false);
  assert.equal(message,"");
  assert.equal(damageClaimRequestIsCurrent(original,current),false);
});

test("current session and claim selection may show the completed result",async()=>{
  let current={...original},message="";
  const completion=finishDamageClaimAfterReload(original,async()=>{current={...current};},()=>current,()=>{message="resuelto";});
  assert.equal(await completion,true);
  assert.equal(message,"resuelto");
});
