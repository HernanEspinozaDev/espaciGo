import assert from "node:assert/strict";
import test from "node:test";

// Test identity headers computation and caching logic
test("cloud run identity returns empty headers if audience is empty", async () => {
  const { getCloudRunIdentityHeaders } = await import(
    "../src/lib/api/cloud-run-identity.ts"
  );
  const headers = await getCloudRunIdentityHeaders("");
  assert.deepEqual(headers, {});
});

test("cloud run identity requests metadata server and formats headers", async () => {
  const { getCloudRunIdentityHeaders, clearCloudRunIdentityCacheForTests } =
    await import("../src/lib/api/cloud-run-identity.ts");
  clearCloudRunIdentityCacheForTests();

  const mockToken = "synthetic-token-".padEnd(120, "x");
  let calls = 0;
  const mockFetch = async (url, init) => {
    calls += 1;
    assert.ok(url.includes("audience=https%3A%2F%2Fapi.test"));
    assert.ok(url.includes("format=full"));
    assert.equal(init?.headers?.["Metadata-Flavor"], "Google");
    return new Response(mockToken, { status: 200 });
  };

  const h1 = await getCloudRunIdentityHeaders(
    "https://api.test",
    mockFetch,
  );
  assert.equal(h1.Authorization, `Bearer ${mockToken}`);
  assert.equal(h1["X-Serverless-Authorization"], `Bearer ${mockToken}`);
  assert.equal(calls, 1);

  // Cached on second call
  const h2 = await getCloudRunIdentityHeaders(
    "https://api.test",
    mockFetch,
  );
  assert.equal(h2.Authorization, `Bearer ${mockToken}`);
  assert.equal(calls, 1);

  clearCloudRunIdentityCacheForTests();
});
