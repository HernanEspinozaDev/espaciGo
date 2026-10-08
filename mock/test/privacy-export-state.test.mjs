import test from "node:test";
import assert from "node:assert/strict";
import {
  capturePrivacyExportContext,
  deliverPrivacyExportIfCurrent,
  privacyExportSessionMatches,
} from "../../internal/mockserver/static/privacy-export-state.js";

test("does not deliver an export response after logout and login to the same account", async () => {
  const context = capturePrivacyExportContext("account-a", "token-a", 7);
  assert.ok(context);
  let resolveRequest;
  const pending = new Promise(resolve => { resolveRequest = resolve; });
  let current = {accountID:"account-a", sessionToken:"token-a", generation:7};
  let delivered = 0;
  const operation = deliverPrivacyExportIfCurrent(
    context,
    () => pending,
    captured => privacyExportSessionMatches(captured, current.accountID, current.sessionToken, current.generation),
    () => { delivered++; },
  );

  // Even if an account or session credential were reused, a later login has
  // a new generation and cannot receive data requested by the previous login.
  current = {accountID:"account-a", sessionToken:"token-a", generation:8};
  resolveRequest({account:{email:"account-a@example.invalid"}});

  assert.equal(await operation, false);
  assert.equal(delivered, 0);
});

test("does not surface a stale export failure after the session changes", async () => {
  const context = capturePrivacyExportContext("account-a", "token-a", 2);
  assert.ok(context);
  let rejectRequest;
  const pending = new Promise((_resolve, reject) => { rejectRequest = reject; });
  let current = {accountID:"account-a", sessionToken:"token-a", generation:2};
  let delivered = 0;
  const operation = deliverPrivacyExportIfCurrent(
    context,
    () => pending,
    captured => privacyExportSessionMatches(captured, current.accountID, current.sessionToken, current.generation),
    () => { delivered++; },
  );
  current = {accountID:"account-a", sessionToken:"token-b", generation:3};
  rejectRequest(new Error("old session failed"));
  assert.equal(await operation, false);
  assert.equal(delivered, 0);
});

test("delivers ZIP bytes only to the session that requested the archive", async () => {
  const context = capturePrivacyExportContext("account-a", "token-a", 11);
  assert.ok(context);
  const zipBytes = new Uint8Array([0x50, 0x4b, 0x03, 0x04, 0x01]);
  let current = {accountID:"account-a", sessionToken:"token-a", generation:11};
  let delivered = null;
  const result = await deliverPrivacyExportIfCurrent(
    context,
    async () => new Blob([zipBytes], {type:"application/zip"}),
    captured => privacyExportSessionMatches(captured,current.accountID,current.sessionToken,current.generation),
    value => { delivered = value; },
  );
  assert.equal(result,true);
  assert.equal(delivered.type,"application/zip");
  assert.deepEqual([...new Uint8Array(await delivered.arrayBuffer())],[...zipBytes]);

  current = {accountID:"account-b",sessionToken:"token-b",generation:12};
  let staleDelivery = 0;
  const stale = await deliverPrivacyExportIfCurrent(
    context,
    async () => new Blob([zipBytes],{type:"application/zip"}),
    captured => privacyExportSessionMatches(captured,current.accountID,current.sessionToken,current.generation),
    () => { staleDelivery++; },
  );
  assert.equal(stale,false);
  assert.equal(staleDelivery,0);
});
