import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.use({ trace: "off", screenshot: "off", video: "off" });
test.describe.configure({ mode: "serial" });

const enabled = process.env.PHASE5_NATIVE_API_E2E === "1";
const fakeTokenId = "550e8400-e29b-41d4-a716-446655440000";
const fakeToken = "synthetic-phase5-invalid-action-token";

function privatePath(name: string): string {
  return path.resolve(process.cwd(), "../.local/testing", name);
}

async function safeCode(response: {
  json(): Promise<unknown>;
}): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: { code?: unknown } };
    const code = payload.error?.code;
    return typeof code === "string" && /^[a-z0-9_.-]{1,96}$/i.test(code)
      ? code
      : "unknown";
  } catch {
    return "unknown";
  }
}

test.skip(
  !enabled,
  "Requires the explicitly configured current-HEAD API and native PostgreSQL runtime",
);

test("@phase5 @native reads actual terms and leaves registration blocked when the server has no complete consent set", async ({
  page,
}) => {
  let completeConsent = false;
  const response = await page.request.get("/api/backend/auth/terms", {
    timeout: 10_000,
  });
  if (response.status() === 503) {
    expect(await safeCode(response)).toBe("terms_unavailable");
  } else {
    expect(response.status()).toBe(200);
    const payload = (await response.json()) as {
      items?: Array<{
        type?: string;
        id?: string;
        sha256?: string;
        synthetic?: boolean;
        document?: {
          id?: string;
          type?: string;
          title?: string;
          scope?: string;
          sections?: Array<{ heading?: string; body?: string }>;
        };
      }>;
    };
    const canonicalId = (id: string | undefined) =>
      typeof id === "string" &&
      /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
        id,
      );
    completeConsent =
      Array.isArray(payload.items) &&
      payload.items.length === 2 &&
      JSON.stringify(payload.items.map((item) => item.type).sort()) ===
        JSON.stringify(["privacidad", "terminos"]) &&
      canonicalId(payload.items[0]?.id) &&
      canonicalId(payload.items[1]?.id) &&
      payload.items[0]?.id !== payload.items[1]?.id &&
      payload.items.every(
        (item) =>
          Boolean(
            item.id &&
            item.sha256 &&
            item.synthetic === true &&
            item.document?.id === item.id &&
            item.document.type === item.type &&
            item.document.title?.trim() &&
            item.document.scope?.trim() &&
            item.document.sections?.length,
          ) &&
          item.document!.sections!.every((section) =>
            Boolean(section.heading?.trim() && section.body?.trim()),
          ),
      );
  }

  let registrationPosts = 0;
  page.on("request", (request) => {
    if (
      new URL(request.url()).pathname === "/api/backend/auth/register" &&
      request.method() === "POST"
    ) {
      registrationPosts += 1;
    }
  });
  await page.goto("/registro");
  await expect(
    page.getByRole("button", { name: "Crear cuenta" }),
  ).toBeDisabled();
  if (!completeConsent) {
    await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
      /documentos vigentes completos|documentos completos y vigentes/,
    );
  }
  expect(registrationPosts).toBe(0);
});

test("@phase5 @native unknown recovery returns the generic 202 contract", async ({
  page,
}) => {
  const response = await page.request.post(
    "/api/backend/auth/password/recovery",
    {
      data: { email: "phase5-unknown-recovery@example.invalid" },
      timeout: 10_000,
    },
  );
  expect(response.status()).toBe(202);
  const payload = (await response.json()) as {
    status?: string;
    message?: string;
  };
  expect(payload.status).toBe("accepted");
  expect(typeof payload.message).toBe("string");
});

test("@phase5 @native unknown verification reissue returns the opaque 204", async ({
  page,
}) => {
  const response = await page.request.post(
    "/api/backend/auth/verification/reissue",
    {
      data: { email: "phase5-unknown-reissue@example.invalid" },
      timeout: 10_000,
    },
  );
  expect(response.status()).toBe(204);
  expect(await response.text()).toBe("");
});

test("@phase5 @native fictitious invalid verification and recovery tokens stay generic", async ({
  page,
}) => {
  const verification = await page.request.post(
    "/api/backend/auth/verification",
    {
      data: { token_id: fakeTokenId, token: fakeToken },
      timeout: 10_000,
    },
  );
  expect(verification.status()).toBe(422);
  expect(await safeCode(verification)).toBe("invalid_token");

  const recovery = await page.request.post(
    "/api/backend/auth/password/recovery/consume",
    {
      data: {
        token_id: fakeTokenId,
        token: fakeToken,
        new_password: "ABC123!!",
        confirm_password: "ABC123!!",
      },
      timeout: 10_000,
    },
  );
  expect(recovery.status()).toBe(422);
  expect(await safeCode(recovery)).toBe("invalid_token");
});

test("@phase5 @native one unknown login remains a generic 401 without provoking a lockout", async ({
  page,
}) => {
  const response = await page.request.post("/api/backend/auth/login", {
    data: {
      email: "phase5-unknown-login@example.invalid",
      password: "ABC123!!",
    },
    timeout: 10_000,
  });
  expect(response.status()).toBe(401);
  expect(["unauthenticated", "invalid_credentials"]).toContain(
    await safeCode(response),
  );
});

test("@phase5 @native existing authorized synthetic account confirms session, contextual return, and logout", async ({
  page,
}) => {
  test.setTimeout(60_000);
  let policy: {
    account_policy?: {
      never_print_credentials_or_tokens?: boolean;
      password?: string;
    };
  };
  let inventory: { email?: string; accountId?: string; status?: string };
  try {
    policy = JSON.parse(
      await readFile(privatePath("test-account-policy.json"), "utf8"),
    ) as typeof policy;
    inventory = JSON.parse(
      await readFile(privatePath("phase3-account-inventory.json"), "utf8"),
    ) as typeof inventory;
  } catch {
    test.skip(
      true,
      "Authorized private synthetic-account material is unavailable",
    );
    return;
  }
  const credentials = policy.account_policy;
  if (
    credentials?.never_print_credentials_or_tokens !== true ||
    typeof credentials.password !== "string" ||
    typeof inventory.email !== "string" ||
    inventory.status !== "verified"
  ) {
    test.skip(
      true,
      "Private policy does not authorize this existing-account test",
    );
    return;
  }

  const secrets = [inventory.email, credentials.password];
  let tokenDigest: string | null = null;
  let sessionConfirmed = false;
  let logoutConfirmed = false;
  let secretInConsole = false;
  const nonLoopback = { value: false };
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:"))
      nonLoopback.value = true;
    const authorization = request.headers().authorization;
    if (
      new URL(request.url()).pathname === "/api/backend/auth/session" &&
      authorization?.startsWith("Bearer ")
    ) {
      tokenDigest = `${authorization.length}`;
    }
  });
  page.on("console", (message) => {
    const text = message.text();
    if (
      secrets.some((secret) => text.includes(secret)) ||
      /\b(?:bearer\s+\S+|access[_ -]?token|authorization)\b/i.test(text)
    ) {
      secretInConsole = true;
    }
  });

  await page.goto("/acceso?return_to=%2Fdashboard%2Fsecurity");
  const email = page.getByLabel("Correo electrónico");
  const password = page.getByLabel("Contraseña", { exact: true });
  try {
    const loginResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login",
      { timeout: 15_000 },
    );
    await email.fill(inventory.email);
    await password.fill(credentials.password);
    const loginResponse = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      loginResponsePromise,
    ]).then(([, response]) => response);
    expect(loginResponse.status()).toBe(200);
    await expect(page).toHaveURL(/\/dashboard\/security$/);
    sessionConfirmed = true;
  } catch {
    await email.fill("", { timeout: 500 }).catch(() => undefined);
    await password.fill("", { timeout: 500 }).catch(() => undefined);
    throw new Error(
      "Authorized synthetic login did not reach the allowlisted destination",
    );
  }
  expect(sessionConfirmed).toBe(true);
  expect(tokenDigest).not.toBeNull();

  await page.getByRole("link", { name: "Acceso" }).click();
  await expect(page).toHaveURL(/\/acceso$/);
  await expect(page.getByText("Sesión activa")).toBeVisible();
  for (const viewport of [
    { width: 1365, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const axe = await new AxeBuilder({ page }).analyze();
    expect(axe.violations.map((violation) => violation.id)).toEqual([]);
  }
  const logoutPromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/logout",
    { timeout: 15_000 },
  );
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  const logout = await logoutPromise;
  expect(logout.status()).toBe(204);
  logoutConfirmed = true;
  expect(logoutConfirmed).toBe(true);
  expect(nonLoopback.value).toBe(false);
  expect(secretInConsole).toBe(false);
});
