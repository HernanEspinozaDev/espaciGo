import test from "node:test";
import assert from "node:assert/strict";

const contractPreviewErrorModule =
  await import("../src/features/public-catalog/domain/contract-preview-error.ts").catch(
    () => null,
  );

test("contract preview errors distinguish identity gate from conflict and expiry", () => {
  const classify = contractPreviewErrorModule?.classifyContractPreviewError;
  assert.equal(
    typeof classify,
    "function",
    "contract preview must classify the documented API error codes",
  );

  assert.equal(
    classify(409, { error: { code: "identity_required" } }),
    "identity_required",
  );
  assert.equal(classify(409, { error: { code: "conflict" } }), "conflict");
  assert.equal(classify(409, null), "conflict");
  assert.equal(classify(404, { error: { code: "not_found" } }), "not_found");
  assert.equal(
    classify(503, { error: { code: "service_unavailable" } }),
    "other",
  );
});
