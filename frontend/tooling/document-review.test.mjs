import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
const root = new URL("../src/features/document-review/", import.meta.url);
test("review adapter uses generated reviewer operations and direct DTOs", async () => {
  const adapter = await readFile(
    new URL("infrastructure/review-openapi-adapter.ts", root),
    "utf8",
  );
  for (const path of [
    "/document-verification/reviewer/cases",
    "/document-verification/reviewer/cases/{caseID}",
    "/document-verification/reviewer/cases/{caseID}/files/{fileID}/content",
    "/document-verification/reviewer/cases/{caseID}/review",
    "/document-verification/reviewer/cases/{caseID}/revoke",
  ])
    assert.ok(adapter.includes(path), path);
  assert.match(adapter, /parseAs: "blob"/);
  assert.match(adapter, /bodySerializer: serializeExactJson/);
  assert.match(
    adapter,
    /Object\.entries\(value\)\.filter\(\(\[, v\]\) => v !== undefined\)/,
  );
});
test("facade fences account changes, rereads after writes, and tracks private downloads", async () => {
  const facade = await readFile(
    new URL("application/review-facade.ts", root),
    "utf8",
  );
  assert.match(
    facade,
    /setSession\(account: string \| null, permissionsKey = ""\)/,
  );
  assert.match(facade, /privateBlobUrls\.revoke\("document-review"\)/);
  assert.match(facade, /#current\(g,\s*account\)/);
  // Post-write freshness and no-replay are exercised behaviorally in
  // document-review-facade.test.mjs, not inferred from a local variable name.
  assert.match(facade, /x\.id === fileId && x\.state === "limpio"/);
  assert.match(facade, /privateBlobUrls\.create\("document-review",\s*blob\)/);
  assert.doesNotMatch(facade, /localStorage|sessionStorage/);
});
test("review domain requires current revision and bounds expiry", async () => {
  const domain = await readFile(new URL("domain/review.ts", root), "utf8");
  assert.match(domain, /expected_revision: number/);
  assert.match(domain, /maxMonths = 24/);
  assert.match(domain, /solicitud_titular/);
  const facade = await readFile(
    new URL("application/review-facade.ts", root),
    "utf8",
  );
  assert.match(facade, /x\.id === fileId && x\.state === "limpio"/);
});
