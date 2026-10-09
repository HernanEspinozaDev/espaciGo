import test from "node:test";
import assert from "node:assert/strict";
import { captureRentalOperationContext, finishRentalOperationAfterReload, rentalOperationControls, rentalOperationResponseIsCurrent, RentalOperationIdempotencyKeys } from "../../internal/mockserver/static/rental-operations-state.js";

const host="aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",renter="bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
test("operation controls follow participant and persisted state",()=>{
  const reservation={host_id:host,renter_id:renter,state:"lista_para_checkin"};
  assert.deepEqual(rentalOperationControls(renter,reservation,[],false,null,true),{canCheckIn:true,canCheckOut:false,canReceive:false,canOpenClaim:false,canDefend:false});
  assert.equal(rentalOperationControls(host,reservation,[],false,null,true).canCheckIn,false);
  assert.equal(rentalOperationControls(renter,{...reservation,state:"en_curso"},[{kind:"checkin"}],false,null,true).canCheckOut,true);
  const finalizada={...reservation,state:"finalizada"},ops=[{kind:"checkout"}];
  assert.equal(rentalOperationControls(host,finalizada,ops,false,null,true).canReceive,true);
  assert.equal(rentalOperationControls(host,finalizada,ops,false,null,true).canOpenClaim,true);
  assert.equal(rentalOperationControls(renter,{...finalizada,state:"en_disputa"},ops,true,{state:"abierto"},true).canDefend,true);
  assert.equal(rentalOperationControls(renter,reservation,[],false,null,false).canCheckIn,false);
});

test("same payload retries retain idempotency key; changed payload starts a distinct intent",()=>{
  const keys=new RentalOperationIdempotencyKeys();let count=0;const generate=()=>`key-${++count}`;
  const first=keys.get("reservation","checkin",'{"comments":"same"}',generate);
  assert.equal(keys.get("reservation","checkin",'{"comments":"same"}',generate),first);
  assert.notEqual(keys.get("reservation","checkin",'{"comments":"changed"}',generate),first);
  keys.clear("reservation","checkin");assert.equal(keys.get("reservation","checkin",'{"comments":"same"}',generate),"key-3");
  keys.clearAll();assert.equal(keys.get("reservation","checkin",'{"comments":"same"}',generate),"key-4");
});

test("late operation response is discarded after selection, logout or relogin",()=>{
  const context=captureRentalOperationContext({revision:4,reservationID:"r1",accountID:host,token:"session-a",generation:2});
  assert.equal(rentalOperationResponseIsCurrent(context,{revision:4,reservationID:"r1",accountID:host,token:"session-a",generation:2}),true);
  assert.equal(rentalOperationResponseIsCurrent(context,{revision:4,reservationID:"r2",accountID:host,token:"session-a",generation:2}),false);
  assert.equal(rentalOperationResponseIsCurrent(context,{revision:4,reservationID:"r1",accountID:"",token:"",generation:3}),false);
  assert.equal(rentalOperationResponseIsCurrent(context,{revision:4,reservationID:"r1",accountID:host,token:"session-b",generation:3}),false);
});

test("last async reload cannot clear fields or show success after reservation changes",async()=>{
  const context=captureRentalOperationContext({revision:4,reservationID:"r1",accountID:host,token:"session-a",generation:2});
  let release;const waiting=new Promise(resolve=>{release=resolve;});let current={...context},cleared=false,success=false;
  const completion=finishRentalOperationAfterReload(context,()=>waiting,()=>current,()=>{cleared=true;success=true;});
  current={...current,revision:9,reservationID:"r2"};release();
  assert.equal(await completion,false);assert.equal(cleared,false);assert.equal(success,false);
});

test("last async reload cannot clear fields or show success after logout and relogin",async()=>{
  const context=captureRentalOperationContext({revision:4,reservationID:"r1",accountID:host,token:"session-a",generation:2});
  let release;const waiting=new Promise(resolve=>{release=resolve;});let current={...context},cleared=false,success=false;
  const completion=finishRentalOperationAfterReload(context,()=>waiting,()=>current,()=>{cleared=true;success=true;});
  current={...current,revision:9,token:"session-b",generation:4};release();
  assert.equal(await completion,false);assert.equal(cleared,false);assert.equal(success,false);
});

test("last async reload can finish for the same reservation and session after its list revision advances",async()=>{
  const context=captureRentalOperationContext({revision:4,reservationID:"r1",accountID:host,token:"session-a",generation:2});
  let current={...context},cleared=false;
  const completed=await finishRentalOperationAfterReload(context,async()=>{current={...current,revision:9};},()=>current,()=>{cleared=true;});
  assert.equal(completed,true);assert.equal(cleared,true);
});
