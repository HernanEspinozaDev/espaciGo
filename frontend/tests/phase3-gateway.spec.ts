import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.use({ trace: "off", screenshot: "off", video: "off" });
const enabled = process.env.PHASE3_NATIVE_API_E2E === "1";

interface ConsentItem {
  id: string;
  type: "privacidad" | "terminos";
  synthetic: boolean;
  sha256: string;
}

interface AccountInventory {
  email: string;
  accountId: string;
  status: "verified";
}

function privateTestingPath(filename: string): string {
  return path.resolve(process.cwd(), "../.local/testing", filename);
}

async function loadVerifiedAccountInventory(): Promise<AccountInventory> {
  const file = privateTestingPath("phase3-account-inventory.json");
  try {
    const existing = JSON.parse(
      await readFile(file, "utf8"),
    ) as Partial<AccountInventory>;
    if (
      typeof existing.email !== "string" ||
      typeof existing.accountId !== "string" ||
      existing.status !== "verified"
    ) {
      throw new Error("Verified synthetic account is unavailable");
    }
    return existing as AccountInventory;
  } catch {
    throw new Error("Verified synthetic account inventory is unavailable");
  }
}

async function safeApiErrorCode(response: {
  json(): Promise<unknown>;
}): Promise<string> {
  try {
    const payload = (await response.json()) as {
      error?: { code?: unknown };
    };
    const code = payload.error?.code;
    return typeof code === "string" && /^[a-z0-9_.-]{1,96}$/i.test(code)
      ? code
      : "unknown";
  } catch {
    return "unknown";
  }
}

function responsePath(url: string): string {
  try {
    return new URL(url).pathname;
  } catch {
    return "";
  }
}

function digest(value: string): string {
  return createHash("sha256").update(value).digest("hex");
}

test.skip(!enabled, "Requires the explicitly configured native PostgreSQL API");

test("anonymous login page passes Axe at desktop and mobile sizes", async ({
  page,
}) => {
  await page.goto("/login");
  for (const viewport of [
    { width: 1280, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(accessibility.violations.map((violation) => violation.id)).toEqual(
      [],
    );
  }
});

test("runtime config exposes only the same-origin browser contract", async ({
  request,
}) => {
  const response = await request.get("/api/runtime-config");
  expect(response.status()).toBe(200);
  expect(await response.json()).toEqual({
    apiBasePath: "/api/backend",
    sessionMode: "memory",
  });
});

test("gateway blocks operations, query keys, and headers outside its allowlist", async ({
  request,
}) => {
  const unknown = await request.get("/api/backend/not-allowlisted");
  expect(unknown.status()).toBe(404);

  const unsupportedMethod = await request.post("/api/backend/auth/session");
  expect(unsupportedMethod.status()).toBe(405);
  expect(unsupportedMethod.headers().allow).toBe("GET");

  const unknownQuery = await request.get(
    "/api/backend/auth/terms?unexpected=1",
  );
  expect(unknownQuery.status()).toBe(400);

  const unknownHeader = await request.get("/api/backend/auth/terms", {
    headers: { "x-not-in-contract": "synthetic" },
  });
  expect(unknownHeader.status()).toBe(400);
});

test("gateway returns consent versions read from the configured native API", async ({
  request,
}) => {
  const response = await request.get("/api/backend/auth/terms", {
    timeout: 10_000,
  });
  if (response.status() === 503) {
    const unavailable = (await response.json()) as {
      error?: { code?: string };
    };
    if (unavailable.error?.code === "terms_unavailable") {
      test.skip(
        true,
        "native DB does not expose usable synthetic consent versions",
      );
    }
  }
  expect(response.status()).toBe(200);
  expect(response.headers()["cache-control"]).toContain("no-store");
  expect(response.headers()["x-request-id"]).toMatch(/^[A-Za-z0-9._-]{1,128}$/);

  const payload = (await response.json()) as { items: ConsentItem[] };
  expect(Array.isArray(payload.items)).toBe(true);
  expect(payload.items).toHaveLength(2);
  expect(payload.items.map((item) => item.type).sort()).toEqual([
    "privacidad",
    "terminos",
  ]);
  for (const item of payload.items) {
    expect(item.synthetic).toBe(true);
    expect(item.id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    expect(item.sha256).toMatch(/^[0-9a-f]{64}$/);
  }
});

test("gateway preserves the native API's real unauthenticated session response", async ({
  request,
}) => {
  const response = await request.get("/api/backend/auth/session");
  expect(response.status()).toBe(401);
  expect(response.headers()["content-type"]).toContain("application/json");
  const payload = (await response.json()) as {
    error?: { code?: string; request_id?: string };
  };
  expect(payload.error?.code).toBe("unauthenticated");
  expect(payload.error?.request_id).toMatch(/^[A-Za-z0-9._-]{1,128}$/);
});

test("gateway forwards bodyless logout requests to the native API", async ({
  request,
}) => {
  const port = process.env.PLAYWRIGHT_PORT ?? "4203";
  const response = await request.post("/api/backend/auth/logout", {
    headers: {
      host: `127.0.0.1:${port}`,
      origin: `http://127.0.0.1:${port}`,
    },
  });
  expect(response.status()).toBe(401);
  const payload = (await response.json()) as {
    error?: { code?: string };
  };
  expect(payload.error?.code).toBe("unauthenticated");
});

test("gateway ignores framework forwarding headers before allowlist checks", async ({
  request,
}) => {
  const port = process.env.PLAYWRIGHT_PORT ?? "4203";
  const sameOrigin = `http://127.0.0.1:${port}`;
  const response = await request.get("/api/backend/auth/session", {
    headers: {
      host: `127.0.0.1:${port}`,
      origin: sameOrigin,
      "x-forwarded-for": "198.51.100.23",
      "x-forwarded-host": "untrusted.invalid",
      "x-forwarded-port": "443",
      "x-forwarded-proto": "https",
    },
  });
  expect(response.status()).toBe(401);
  const payload = (await response.json()) as {
    error?: { code?: string };
  };
  expect(payload.error?.code).toBe("unauthenticated");
});

test("gateway rejects a request Origin that differs from Host", async ({
  request,
}) => {
  const port = process.env.PLAYWRIGHT_PORT ?? "4203";
  const response = await request.get("/api/backend/auth/session", {
    headers: {
      host: `127.0.0.1:${port}`,
      origin: `http://untrusted.invalid:${port}`,
    },
  });
  expect(response.status()).toBe(403);
  const payload = (await response.json()) as {
    error?: { code?: string };
  };
  expect(payload.error?.code).toBe("origin_not_allowed");
});

test("authorized existing synthetic account completes session and logout", async ({
  page,
}) => {
  const policy = JSON.parse(
    await readFile(privateTestingPath("test-account-policy.json"), "utf8"),
  );
  const accountPolicy = policy.account_policy;
  if (
    accountPolicy.never_print_credentials_or_tokens !== true ||
    typeof accountPolicy.password !== "string"
  ) {
    throw new Error(
      "Private policy does not authorize the existing synthetic account test",
    );
  }

  const inventory = await loadVerifiedAccountInventory();
  let accessToken: string | null = null;
  let sessionRequestDigest: string | null = null;
  let logoutRequestDigest: string | null = null;
  let sessionResult = "response_not_observed";
  let sessionResponseObserved = false;
  let sessionClaims: {
    roles: unknown;
    capabilities: unknown;
    restricted_mode: unknown;
  } | null = null;
  let unsafeConsoleOutput = false;
  let observingPostLogoutSession = false;
  let postLogoutSessionAuthorization: boolean | null = null;

  page.on("request", (request) => {
    const pathname = responsePath(request.url());
    const authorization = request.headers().authorization;
    if (
      observingPostLogoutSession &&
      pathname === "/api/backend/auth/session" &&
      request.method() === "GET"
    ) {
      postLogoutSessionAuthorization = authorization !== undefined;
    }
    if (!authorization?.startsWith("Bearer ")) return;
    if (
      pathname === "/api/backend/auth/session" &&
      request.method() === "GET"
    ) {
      sessionRequestDigest = digest(authorization);
    } else if (
      pathname === "/api/backend/auth/logout" &&
      request.method() === "POST"
    ) {
      logoutRequestDigest = digest(authorization);
    }
  });

  page.on("response", (response) => {
    if (
      responsePath(response.url()) !== "/api/backend/auth/session" ||
      !response.request().headers().authorization?.startsWith("Bearer ") ||
      sessionResponseObserved
    ) {
      return;
    }
    sessionResponseObserved = true;
    void (async () => {
      if (!response.ok()) {
        sessionResult = `${response.status()}/${await safeApiErrorCode(response)}`;
        return;
      }
      sessionResult = `${response.status()}/ok`;
      try {
        const payload = (await response.json()) as {
          roles?: unknown;
          capabilities?: unknown;
          restricted_mode?: unknown;
        };
        sessionClaims = {
          roles: payload.roles,
          capabilities: payload.capabilities,
          restricted_mode: payload.restricted_mode,
        };
      } catch {
        sessionResult = `${response.status()}/invalid_response`;
      }
    })().catch(() => {
      sessionResult = `${response.status()}/invalid_response`;
    });
  });

  page.on("console", (message) => {
    const text = message.text();
    if (
      text.includes(inventory.email) ||
      text.includes(inventory.accountId) ||
      text.includes(accountPolicy.password) ||
      /\b(?:bearer\s+\S+|access[_ -]?token|authorization|password|token)\b/i.test(
        text,
      ) ||
      (accessToken !== null && text.includes(accessToken))
    ) {
      unsafeConsoleOutput = true;
    }
  });

  await page.goto("/login");
  const emailField = page.getByLabel("Correo electrónico");
  const passwordField = page.getByLabel("Contraseña");
  const clearCredentials = async () => {
    for (const field of [emailField, passwordField]) {
      try {
        if (await field.count()) await field.fill("");
      } catch {
        // Keep credentials out of failure artifacts even if the form detached.
      }
    }
  };

  let loginResult = "response_not_observed";
  let loginClaims: {
    roles: unknown;
    capabilities: unknown;
    restricted_mode: unknown;
  } | null = null;
  try {
    const loginResponsePromise = page.waitForResponse(
      (response) => responsePath(response.url()) === "/api/backend/auth/login",
      { timeout: 15_000 },
    );
    await emailField.fill(inventory.email);
    await passwordField.fill(accountPolicy.password);
    const [, loginResponse] = await Promise.all([
      page.getByRole("button", { name: "Iniciar sesión" }).click(),
      loginResponsePromise,
    ]);
    loginResult = `${loginResponse.status()}/${loginResponse.ok() ? "ok" : await safeApiErrorCode(loginResponse)}`;
    if (!loginResponse.ok()) throw new Error("login_not_accepted");
    const payload = (await loginResponse.json()) as {
      access_token?: unknown;
      token_type?: unknown;
      roles?: unknown;
      capabilities?: unknown;
      restricted_mode?: unknown;
    };
    if (
      typeof payload.access_token !== "string" ||
      payload.access_token.length === 0 ||
      payload.token_type !== "Bearer" ||
      !Array.isArray(payload.roles) ||
      payload.roles.length === 0 ||
      !Array.isArray(payload.capabilities) ||
      typeof payload.restricted_mode !== "boolean"
    ) {
      loginResult = `${loginResponse.status()}/invalid_response`;
      throw new Error("login_response_invalid");
    }
    accessToken = payload.access_token;
    loginClaims = {
      roles: payload.roles,
      capabilities: payload.capabilities,
      restricted_mode: payload.restricted_mode,
    };
    await expect(page.getByText("Sesión activa")).toBeVisible({
      timeout: 15_000,
    });
    await expect
      .poll(() => sessionClaims !== null, { timeout: 5_000 })
      .toBe(true);
    await clearCredentials();
  } catch {
    await clearCredentials();
    throw new Error(
      `Synthetic account did not reach authenticated UI state (login=${loginResult}; session=${sessionResult})`,
    );
  }

  const sessionToken = accessToken;
  const authenticatedLoginClaims = loginClaims;
  const authenticatedSessionClaims = sessionClaims as {
    roles: unknown;
    capabilities: unknown;
    restricted_mode: unknown;
  } | null;
  if (
    sessionToken === null ||
    authenticatedLoginClaims === null ||
    authenticatedSessionClaims === null
  ) {
    throw new Error("Authenticated session proof was unavailable");
  }
  expect(sessionResult).toBe("200/ok");
  expect(Array.isArray(authenticatedSessionClaims.roles)).toBe(true);
  expect(Array.isArray(authenticatedSessionClaims.capabilities)).toBe(true);
  expect(typeof authenticatedSessionClaims.restricted_mode).toBe("boolean");
  expect(
    (authenticatedSessionClaims.roles as unknown[]).length,
  ).toBeGreaterThan(0);
  expect(authenticatedSessionClaims.restricted_mode).toBe(false);
  expect(authenticatedSessionClaims.roles).toEqual(
    authenticatedLoginClaims.roles,
  );
  expect(authenticatedSessionClaims.capabilities).toEqual(
    authenticatedLoginClaims.capabilities,
  );
  expect(authenticatedSessionClaims.restricted_mode).toBe(
    authenticatedLoginClaims.restricted_mode,
  );
  expect(sessionRequestDigest === digest(`Bearer ${sessionToken}`)).toBe(true);

  const persisted = await page.evaluate(
    async (sensitiveValues: string[]) => {
      const containsSensitive = (value: string) =>
        sensitiveValues.some(
          (sensitive) => sensitive.length > 0 && value.includes(sensitive),
        );
      const containsInStorage = (storage: Storage) =>
        Array.from({ length: storage.length }, (_, index) => {
          const key = storage.key(index) ?? "";
          return (
            containsSensitive(key) ||
            containsSensitive(storage.getItem(key) ?? "")
          );
        }).some(Boolean);
      const contains = (
        value: unknown,
        seen = new WeakSet<object>(),
      ): boolean => {
        if (typeof value === "string") return containsSensitive(value);
        if (value !== null && typeof value === "object") {
          if (seen.has(value)) return false;
          seen.add(value);
          if (value instanceof ArrayBuffer) {
            return containsSensitive(new TextDecoder().decode(value));
          }
          if (ArrayBuffer.isView(value)) {
            const view = value as ArrayBufferView;
            const bytes = new Uint8Array(
              view.buffer as ArrayBuffer,
              view.byteOffset,
              view.byteLength,
            );
            return containsSensitive(new TextDecoder().decode(bytes));
          }
          if (Array.isArray(value))
            return value.some((item) => contains(item, seen));
          return Object.entries(value as Record<string, unknown>).some(
            ([key, item]) => containsSensitive(key) || contains(item, seen),
          );
        }
        return false;
      };
      const hasSensitiveInDatabase = async (name: string, version?: number) => {
        const database = await new Promise<IDBDatabase>((resolve, reject) => {
          const opening = indexedDB.open(name, version);
          opening.onsuccess = () => resolve(opening.result);
          opening.onerror = () => reject(opening.error);
        });
        try {
          for (const storeName of Array.from(database.objectStoreNames)) {
            const records = await new Promise<unknown[]>((resolve, reject) => {
              const reading = database
                .transaction(storeName, "readonly")
                .objectStore(storeName)
                .getAll();
              reading.onsuccess = () => resolve(reading.result as unknown[]);
              reading.onerror = () => reject(reading.error);
            });
            if (contains(records)) return true;
          }
          return false;
        } finally {
          database.close();
        }
      };
      let indexedDBContainsSensitive = false;
      for (const database of await indexedDB.databases()) {
        if (
          database.name &&
          (await hasSensitiveInDatabase(database.name, database.version))
        ) {
          indexedDBContainsSensitive = true;
          break;
        }
      }
      const html = document.documentElement.outerHTML;
      return {
        localStorage: containsInStorage(window.localStorage),
        sessionStorage: containsInStorage(window.sessionStorage),
        cookies: containsSensitive(document.cookie),
        indexedDB: indexedDBContainsSensitive,
        url: containsSensitive(window.location.href),
        html: sensitiveValues.some(
          (value) => value.length > 0 && html.includes(value),
        ),
      };
    },
    [
      inventory.email,
      inventory.accountId,
      accountPolicy.password,
      sessionToken,
    ],
  );
  const cookies = await page.context().cookies();
  expect(persisted).toEqual({
    localStorage: false,
    sessionStorage: false,
    cookies: false,
    indexedDB: false,
    url: false,
    html: false,
  });
  expect(
    cookies.some((cookie) =>
      [
        inventory.email,
        inventory.accountId,
        accountPolicy.password,
        sessionToken,
      ].some((value) => value.length > 0 && cookie.value.includes(value)),
    ),
  ).toBe(false);
  expect(unsafeConsoleOutput).toBe(false);
  await expect(page.getByText(/Modo restringido/)).toHaveCount(0);

  for (const viewport of [
    { width: 1280, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(accessibility.violations.map((violation) => violation.id)).toEqual(
      [],
    );
  }

  const logoutResponsePromise = page.waitForResponse(
    (response) => responsePath(response.url()) === "/api/backend/auth/logout",
    { timeout: 15_000 },
  );
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  const logoutResponse = await logoutResponsePromise;
  expect(logoutResponse.status()).toBe(204);
  expect(logoutRequestDigest === digest(`Bearer ${sessionToken}`)).toBe(true);
  await expect(
    page.getByRole("button", { name: "Iniciar sesión" }),
  ).toBeVisible({ timeout: 15_000 });
  await expect(page.getByRole("status")).toHaveText("Sesión cerrada.");
  observingPostLogoutSession = true;
  const postLogoutSession = await page.evaluate(async () => {
    const response = await fetch("/api/backend/auth/session", {
      cache: "no-store",
    });
    let code = "unknown";
    try {
      const payload = (await response.json()) as {
        error?: { code?: unknown };
      };
      if (
        typeof payload.error?.code === "string" &&
        /^[a-z0-9_.-]{1,96}$/i.test(payload.error.code)
      ) {
        code = payload.error.code;
      }
    } catch {
      code = "unknown";
    }
    return { status: response.status, code };
  });
  observingPostLogoutSession = false;
  expect(postLogoutSession).toEqual({
    status: 401,
    code: "unauthenticated",
  });
  expect(postLogoutSessionAuthorization).toBe(false);
  const postLogoutPrivacy = await page.evaluate(
    (sensitiveValues: string[]) => {
      const html = document.documentElement.outerHTML;
      const email =
        document.querySelector<HTMLInputElement>("#email")?.value ?? "";
      const password =
        document.querySelector<HTMLInputElement>("#password")?.value ?? "";
      return {
        emailFieldFilled: email.length > 0,
        passwordFieldFilled: password.length > 0,
        sensitiveInHTML: sensitiveValues.some(
          (value) => value.length > 0 && html.includes(value),
        ),
      };
    },
    [
      inventory.email,
      inventory.accountId,
      accountPolicy.password,
      sessionToken,
    ],
  );
  expect(postLogoutPrivacy).toEqual({
    emailFieldFilled: false,
    passwordFieldFilled: false,
    sensitiveInHTML: false,
  });
  accessToken = null;
  expect(unsafeConsoleOutput).toBe(false);
});
