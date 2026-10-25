import { test, expect } from "@playwright/test";

test.describe("Phase 10 — Health and Container Readiness", () => {
  test("health endpoint returns status ok with timestamp and uptime", async ({
    request,
  }) => {
    const response = await request.get("/api/health");
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe("ok");
    expect(typeof body.timestamp).toBe("string");
    expect(typeof body.uptime).toBe("number");
  });

  test("healthz alias rewrite resolves to health endpoint", async ({
    request,
  }) => {
    const response = await request.get("/healthz");
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe("ok");
  });

  test("runtime config endpoint returns safe client parameters", async ({
    request,
  }) => {
    const response = await request.get("/api/runtime-config");
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.apiBasePath).toBe("/api/backend");
    expect(body.sessionMode).toBe("memory");
  });
});
