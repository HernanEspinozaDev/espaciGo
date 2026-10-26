import assert from "node:assert/strict";
import test from "node:test";

test("cloud run identity loopback guard returns empty headers without calling metadata even with remote API_SERVICE_AUDIENCE", async () => {
  const {
    getCloudRunIdentityHeadersForBackend,
    clearCloudRunIdentityCacheForTests,
  } = await import("../src/lib/api/cloud-run-identity.ts");
  clearCloudRunIdentityCacheForTests();

  const originalAudience = process.env.API_SERVICE_AUDIENCE;
  try {
    process.env.API_SERVICE_AUDIENCE = "https://remote-audience.run.app";

    let fetchCalls = 0;
    const mockFetch = async () => {
      fetchCalls += 1;
      throw new Error(
        "Metadata fetch must never be called for loopback destinations",
      );
    };

    const loopbackUrls = [
      new URL("http://localhost:8080"),
      new URL("http://127.0.0.1:8080"),
      new URL("http://127.0.0.99:3000"),
      new URL("http://[::1]:8080"),
    ];

    for (const url of loopbackUrls) {
      const headers = await getCloudRunIdentityHeadersForBackend(
        url,
        mockFetch,
      );
      assert.deepEqual(headers, {});
    }

    assert.equal(fetchCalls, 0);
  } finally {
    if (originalAudience === undefined) {
      delete process.env.API_SERVICE_AUDIENCE;
    } else {
      process.env.API_SERVICE_AUDIENCE = originalAudience;
    }
    clearCloudRunIdentityCacheForTests();
  }
});

test("cloud run identity remote backend delegates to IAM metadata and returns serverless identity headers", async () => {
  const {
    getCloudRunIdentityHeadersForBackend,
    clearCloudRunIdentityCacheForTests,
  } = await import("../src/lib/api/cloud-run-identity.ts");
  clearCloudRunIdentityCacheForTests();

  const originalAudience = process.env.API_SERVICE_AUDIENCE;
  try {
    delete process.env.API_SERVICE_AUDIENCE;

    const mockToken = "synthetic-token-".padEnd(120, "y");
    let fetchCalls = 0;
    const mockFetch = async (url, init) => {
      fetchCalls += 1;
      assert.ok(url.includes("audience=https%3A%2F%2Fapi.espacigo.run.app"));
      assert.ok(url.includes("format=full"));
      assert.equal(init?.headers?.["Metadata-Flavor"], "Google");
      return new Response(mockToken, { status: 200 });
    };

    const remoteUrl = new URL("https://api.espacigo.run.app");
    const headers = await getCloudRunIdentityHeadersForBackend(
      remoteUrl,
      mockFetch,
    );

    assert.equal(fetchCalls, 1);
    assert.equal(headers.Authorization, `Bearer ${mockToken}`);
    assert.equal(headers["X-Serverless-Authorization"], `Bearer ${mockToken}`);

    // Second call should be cached
    const cachedHeaders = await getCloudRunIdentityHeadersForBackend(
      remoteUrl,
      mockFetch,
    );
    assert.equal(fetchCalls, 1);
    assert.deepEqual(cachedHeaders, headers);
  } finally {
    if (originalAudience === undefined) {
      delete process.env.API_SERVICE_AUDIENCE;
    } else {
      process.env.API_SERVICE_AUDIENCE = originalAudience;
    }
    clearCloudRunIdentityCacheForTests();
  }
});
