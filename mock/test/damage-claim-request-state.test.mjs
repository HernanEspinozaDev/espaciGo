import test from "node:test";
import assert from "node:assert/strict";
import { createOrRecoverDamageDefenseEvidence, damageClaimRequestIsCurrent, finishDamageClaimAfterEvidence, finishDamageClaimAfterReload } from "../../internal/mockserver/static/damage-claim-request-state.js";

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

test("late defense evidence is discarded after reservation selection changes",async()=>{
  let release;const delayed=new Promise(resolve=>{release=resolve;});
  let current={...original},message="",reloads=0;
  const completion=finishDamageClaimAfterEvidence(original,()=>delayed,async()=>{reloads++;},()=>current,()=>{message="descargo guardado";});
  current={...current,selectionRevision:9,selectionID:"claim-b"};release();
  assert.equal(await completion,false);assert.equal(reloads,0);assert.equal(message,"");
});

test("late defense evidence cannot report success after logout and same-account relogin",async()=>{
  let release;const delayed=new Promise(resolve=>{release=resolve;});
  let current={...original},message="";
  const completion=finishDamageClaimAfterEvidence(original,()=>delayed,async()=>{},()=>current,()=>{message="descargo guardado";});
  current={...current,token:"session-b",generation:6};release();
  assert.equal(await completion,false);assert.equal(message,"");
});

test("after PNG failure and reload, recovery creates evidence without resending the defense",async()=>{
  let current={...original},textSubmissions=1,evidenceCreates=0,reloads=0,message="";
  const failedAttempt=createOrRecoverDamageDefenseEvidence(original,async()=>false,async()=>{evidenceCreates++;throw new Error("PNG generation failed");},async()=>{reloads++;},()=>current,()=>{message="saved";});
  await assert.rejects(failedAttempt,/PNG generation failed/);
  // Reload restored the already-persisted defense; only the recovery action runs.
  const recovered=await createOrRecoverDamageDefenseEvidence(original,async()=>false,async()=>{evidenceCreates++;return {id:"evidence-1"};},async()=>{reloads++;},()=>current,()=>{message="evidence recovered";});
  assert.equal(recovered,true);
  assert.equal(textSubmissions,1);
  assert.equal(evidenceCreates,2);
  assert.equal(reloads,1);
  assert.equal(message,"evidence recovered");
});

test("confirmed descargo evidence is recovered without creating a duplicate",async()=>{
  let current={...original},creates=0,reloads=0,message="";
  const recovered=await createOrRecoverDamageDefenseEvidence(original,async()=>true,async()=>{creates++;},async()=>{reloads++;},()=>current,()=>{message="evidence refreshed";});
  assert.equal(recovered,true);
  assert.equal(creates,0);
  assert.equal(reloads,1);
  assert.equal(message,"evidence refreshed");
});

test("late recovery lookup after selection or session changes cannot create evidence",async()=>{
  for(const replacement of [
    {...original,selectionRevision:9,selectionID:"claim-b"},
    {...original,token:"session-b",generation:6},
  ]){
    let release;const delayed=new Promise(resolve=>{release=resolve;});
    let current={...original},creates=0,reloads=0,message="";
    const completion=createOrRecoverDamageDefenseEvidence(original,()=>delayed,async()=>{creates++;},async()=>{reloads++;},()=>current,()=>{message="success";});
    current=replacement;release(false);
    assert.equal(await completion,false);
    assert.equal(creates,0);
    assert.equal(reloads,0);
    assert.equal(message,"");
  }
});
