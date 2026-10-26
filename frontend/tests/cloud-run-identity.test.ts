import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearCloudRunIdentityCacheForTests,
  getCloudRunIdentityHeaders,
} from "@/lib/api/cloud-run-identity";

describe("getCloudRunIdentityHeaders", () => {
  beforeEach(() => {
    clearCloudRunIdentityCacheForTests();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns empty headers if audience is empty", async () => {
    const headers = await getCloudRunIdentityHeaders("");
    expect(headers).toEqual({});
  });

  it("fetches ID token from GCP metadata server and caches it", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      text: async () => "mock-gcp-token",
    });

    const h1 = await getCloudRunIdentityHeaders(
      "https://api.run.app",
      mockFetch as unknown as typeof fetch,
    );
    expect(h1).toEqual({ Authorization: "Bearer mock-gcp-token" });
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch.mock.calls[0][0]).toContain(
      "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=https%3A%2F%2Fapi.run.app",
    );

    // Second call should hit memory cache
    const h2 = await getCloudRunIdentityHeaders(
      "https://api.run.app",
      mockFetch as unknown as typeof fetch,
    );
    expect(h2).toEqual({ Authorization: "Bearer mock-gcp-token" });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("gracefully falls back to empty headers on metadata server network error", async () => {
    const mockFetch = vi.fn().mockRejectedValue(new Error("Metadata timed out"));

    const h = await getCloudRunIdentityHeaders(
      "https://api.run.app",
      mockFetch as unknown as typeof fetch,
    );
    expect(h).toEqual({});
  });
});
