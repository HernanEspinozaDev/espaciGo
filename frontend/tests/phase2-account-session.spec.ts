import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

test.use({ trace: "off", screenshot: "off", video: "off" });

const requestsOutsideLoopback = new WeakMap<Page, boolean>();
test.beforeEach(async ({ page }) => {
  requestsOutsideLoopback.set(page, false);
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      requestsOutsideLoopback.set(page, true);
    }
  });
});
test.afterEach(async ({ page }) => {
  expect(requestsOutsideLoopback.get(page)).toBe(false);
});

type MockResult = {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
};

type GatewayResponder = (
  pathname: string,
  method: string,
  request: Request,
) => MockResult | Promise<MockResult>;

async function mockGateway(
  page: Page,
  responder: GatewayResponder,
): Promise<void> {
  const bff = createBffFixtureState();
  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const result = bff.adapt(
      request,
      await responder(pathname, request.method(), request),
    );
    if (!result) throw new Error("Missing controlled BFF reply");
    const isBinary =
      Buffer.isBuffer(result.body) || typeof result.body === "string";
    await route.fulfill({
      status: result.status,
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        ...(result.headers ?? {}),
      },
      body:
        result.status === 204
          ? ""
          : isBinary
            ? (result.body as Buffer | string)
            : JSON.stringify(result.body ?? {}),
    });
  });
}

const fixedExpiry = new Date(Date.now() + 3600_000).toISOString();
const accountIdA = "770e8400-e29b-41d4-a716-446655440001";
const accountIdB = "770e8400-e29b-41d4-a716-446655440002";

async function loginAndNavigate(
  page: Page,
  targetPath: string = "/mi-cuenta/seguridad",
): Promise<void> {
  await page.goto(`/acceso?return_to=${encodeURIComponent(targetPath)}`);
  await page
    .getByLabel("Correo electrónico")
    .fill("phase2-tester@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("SecurePass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === targetPath);
}

test("@phase2 @controlled mutación de sesión limpia campos y mensajes locales en SecurityCard", async ({
  page,
}) => {
  let activeAccountId = accountIdA;

  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountIdA,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: fixedExpiry,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: activeAccountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    }
    return { status: 404 };
  });

  await loginAndNavigate(page, "/mi-cuenta/seguridad");

  await page.locator("#current-password").fill("CurrentSecret123!");
  await page.locator("#new-password").fill("NewSecret123!");
  await page.locator("#confirm-password").fill("NewSecret123!");

  await expect(page.locator("#current-password")).toHaveValue(
    "CurrentSecret123!",
  );
  await expect(page.locator("#new-password")).toHaveValue("NewSecret123!");
  await expect(page.locator("#confirm-password")).toHaveValue("NewSecret123!");

  activeAccountId = accountIdB;

  const sessionResponsePromise = page.waitForResponse(
    (res) =>
      res.url().includes("/api/backend/auth/session") && res.status() === 200,
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const sessionResponse = await sessionResponsePromise;
  const sessionBody = await sessionResponse.json();
  expect(sessionBody.account_id).toBe(accountIdB);

  await expect(page.locator("#current-password")).toHaveValue("");
  await expect(page.locator("#new-password")).toHaveValue("");
  await expect(page.locator("#confirm-password")).toHaveValue("");
  const passwordCard = page.locator(
    '[aria-labelledby="security-password-title"]',
  );
  await expect(passwordCard.locator('[role="alert"]')).toHaveCount(0);
  await expect(passwordCard.locator('[role="status"]')).toHaveCount(0);
});

test("@phase2 @controlled password change pendiente cancelado ante cambio de sesión a B sin éxito ni error tardío", async ({
  page,
}) => {
  let activeAccountId = accountIdA;
  let resolvePasswordChangeA: ((res: MockResult) => void) | null = null;

  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountIdA,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: fixedExpiry,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: activeAccountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/password/change" && method === "POST") {
      return new Promise<MockResult>((resolve) => {
        resolvePasswordChangeA = resolve;
      });
    }
    return { status: 404 };
  });

  await loginAndNavigate(page, "/mi-cuenta/seguridad");

  await page.locator("#current-password").fill("OldValidPass123!");
  await page.locator("#new-password").fill("NewValidPass123!");
  await page.locator("#confirm-password").fill("NewValidPass123!");

  await page.getByRole("button", { name: "Cambiar contraseña" }).click();

  await expect(
    page.getByRole("button", { name: "Cambiar contraseña" }),
  ).toBeDisabled();
  expect(resolvePasswordChangeA).not.toBeNull();

  activeAccountId = accountIdB;
  const sessionResponsePromise = page.waitForResponse(
    (res) =>
      res.url().includes("/api/backend/auth/session") && res.status() === 200,
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const sessionResponse = await sessionResponsePromise;
  const sessionBody = await sessionResponse.json();
  expect(sessionBody.account_id).toBe(accountIdB);

  await expect(page.locator("#current-password")).toHaveValue("");
  await expect(page.locator("#new-password")).toHaveValue("");
  await expect(page.locator("#confirm-password")).toHaveValue("");

  resolvePasswordChangeA!({
    status: 200,
    body: { success: true },
  });

  await page.waitForTimeout(200);
  const passwordCard = page.locator(
    '[aria-labelledby="security-password-title"]',
  );
  await expect(passwordCard.locator('[role="status"]')).toHaveCount(0);
  await expect(passwordCard.locator('[role="alert"]')).toHaveCount(0);
  await expect(page.locator("#current-password")).toHaveValue("");
});
