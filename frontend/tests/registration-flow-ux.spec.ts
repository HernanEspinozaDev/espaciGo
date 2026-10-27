import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

test.use({
  trace: "off",
  screenshot: "off",
  video: "off",
});

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
    await route.fulfill({
      status: result.status,
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        ...(result.headers ?? {}),
      },
      body: result.status === 204 ? "" : JSON.stringify(result.body ?? {}),
    });
  });
}

const termsFixtureData = {
  items: [
    {
      id: "550e8400-e29b-41d4-a716-446655440000",
      code: "terms-local-v1",
      type: "terminos",
      sha256: "a".repeat(64),
      published_at: "2026-10-25T12:00:00Z",
      synthetic: true,
      document: {
        id: "550e8400-e29b-41d4-a716-446655440000",
        code: "terms-local-v1",
        type: "terminos",
        title: "Términos y condiciones",
        scope: "local_test",
        status: "vigente",
        sections: [
          {
            heading: "Uso general",
            body: "Contenido de términos y condiciones para validación.",
          },
        ],
      },
    },
    {
      id: "660e8400-e29b-41d4-a716-446655440000",
      code: "privacy-local-v1",
      type: "privacidad",
      sha256: "b".repeat(64),
      published_at: "2026-10-25T12:00:00Z",
      synthetic: true,
      document: {
        id: "660e8400-e29b-41d4-a716-446655440000",
        code: "privacy-local-v1",
        type: "privacidad",
        title: "Política de privacidad",
        scope: "local_test",
        status: "vigente",
        sections: [
          {
            heading: "Protección de datos",
            body: "Contenido de política de privacidad para validación.",
          },
        ],
      },
    },
  ],
};

test.describe("Registration flow and /registro-exitoso UX", () => {
  test("registration 201 completes compactly, stores ephemeral result, and navigates to /registro-exitoso with contextual message", async ({
    page,
  }) => {
    let registeredEmail = "";
    await mockGateway(page, async (pathname, method, request) => {
      if (pathname === "/api/backend/auth/session" && method === "GET") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (pathname === "/api/backend/auth/terms" && method === "GET") {
        return { status: 200, body: termsFixtureData };
      }
      if (pathname === "/api/backend/auth/register" && method === "POST") {
        const json = (await request.postDataJSON()) as { email: string };
        registeredEmail = json.email;
        return { status: 201, body: {} };
      }
      return {
        status: 404,
        body: {
          error: { code: "not_found", message: `Unknown path ${pathname}` },
        },
      };
    });

    await page.goto("/registro", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");

    // Verify compact form: NO synthetic disclaimer text
    await expect(
      page.locator("text=Documentos sintéticos de ensayo"),
    ).toHaveCount(0);
    await expect(
      page.locator("text=No solicitaremos un rol ni una preferencia"),
    ).toHaveCount(0);

    // Verify two separate compact checkboxes exist with links opening in new tab
    const termsCheckbox = page.locator("input[type='checkbox']").first();
    const privacyCheckbox = page.locator("input[type='checkbox']").nth(1);
    await expect(termsCheckbox).toBeVisible();
    await expect(privacyCheckbox).toBeVisible();

    const termsLink = page
      .getByRole("link", { name: /términos y condiciones/i })
      .first();
    const privacyLink = page
      .getByRole("link", { name: /política de privacidad/i })
      .first();
    await expect(termsLink).toHaveAttribute("target", "_blank");
    await expect(privacyLink).toHaveAttribute("target", "_blank");

    // Fill form with valid credentials
    await page.fill("input[type='email']", "nuevo.usuario@espacigo.com");
    await page.fill("input[type='password']", "ClaveSegura123!");
    await termsCheckbox.check();
    await privacyCheckbox.check();

    const submitBtn = page.getByRole("button", { name: /crear cuenta/i });
    await expect(submitBtn).toBeEnabled();
    await submitBtn.click();

    // Should navigate to /registro-exitoso
    await page.waitForURL("**/registro-exitoso");
    expect(page.url()).not.toContain("email=");
    expect(page.url()).not.toContain("ClaveSegura123!");

    // 201 contextual copy
    await expect(
      page.getByRole("heading", { name: /creada con éxito/i }),
    ).toBeVisible();
    await expect(page.locator("body")).toContainText(
      "nuevo.usuario@espacigo.com",
    );
    await expect(page.locator("body")).toContainText(
      "Siguiente paso: verifica tu correo",
    );
    expect(registeredEmail).toBe("nuevo.usuario@espacigo.com");
  });

  test("registration 202 shows unconfirmed delivery notice without claiming email was sent", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session" && method === "GET") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (pathname === "/api/backend/auth/terms" && method === "GET") {
        return { status: 200, body: termsFixtureData };
      }
      if (pathname === "/api/backend/auth/register" && method === "POST") {
        return { status: 202, body: {} };
      }
      return {
        status: 404,
        body: {
          error: { code: "not_found", message: `Unknown path ${pathname}` },
        },
      };
    });

    await page.goto("/registro", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");

    await page.fill("input[type='email']", "pendiente.envio@espacigo.com");
    await page.fill("input[type='password']", "ClaveSegura123!");
    const checkboxes = page.locator("input[type='checkbox']");
    await checkboxes.first().check();
    await checkboxes.nth(1).check();

    await page.getByRole("button", { name: /crear cuenta/i }).click();

    await page.waitForURL("**/registro-exitoso");
    // Honest 202: account created, but delivery unconfirmed
    await expect(page.locator("body")).toContainText(
      "no pudimos confirmar el envío",
    );
    await expect(page.locator("body")).not.toContainText(
      "Te enviamos un enlace",
    );
  });

  test("lazy-loads secondary registration-help on expand and maintains accessible state", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session" && method === "GET") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (pathname === "/api/backend/auth/terms" && method === "GET") {
        return { status: 200, body: termsFixtureData };
      }
      if (pathname === "/api/backend/auth/register" && method === "POST") {
        return { status: 201, body: {} };
      }
      return {
        status: 404,
        body: {
          error: { code: "not_found", message: `Unknown path ${pathname}` },
        },
      };
    });

    await page.goto("/registro", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");
    await page.fill("input[type='email']", "ayuda.test@espacigo.com");
    await page.fill("input[type='password']", "ClaveSegura123!");
    const checkboxes = page.locator("input[type='checkbox']");
    await checkboxes.first().check();
    await checkboxes.nth(1).check();
    await page.getByRole("button", { name: /crear cuenta/i }).click();

    await page.waitForURL("**/registro-exitoso");

    // Help content should not be present initially
    await expect(page.locator("#registration-help-content")).toHaveCount(0);

    const helpToggle = page.getByRole("button", {
      name: /¿no encuentras el correo\?/i,
    });
    await expect(helpToggle).toBeVisible();
    await expect(helpToggle).toHaveAttribute("aria-expanded", "false");

    // Click to expand
    await helpToggle.click();
    const expandedToggle = page.getByRole("button", {
      name: /ocultar ayuda/i,
    });
    await expect(expandedToggle).toHaveAttribute("aria-expanded", "true");

    // Dynamic component loaded and visible
    const helpContent = page.locator("#registration-help-content");
    await expect(helpContent).toBeVisible();
    await expect(helpContent).toContainText("Spam");
  });

  test("direct visit to /registro-exitoso presents neutral state without false account created assertion", async ({
    page,
  }) => {
    let reissueCalled = false;
    await mockGateway(page, async (pathname, method, request) => {
      if (pathname === "/api/backend/auth/session" && method === "GET") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (
        pathname === "/api/backend/auth/verification/reissue" &&
        method === "POST"
      ) {
        reissueCalled = true;
        const json = (await request.postDataJSON()) as { email: string };
        expect(json.email).toBe("directo@espacigo.com");
        return { status: 204 };
      }
      return {
        status: 404,
        body: {
          error: { code: "not_found", message: `Unknown path ${pathname}` },
        },
      };
    });

    await page.goto("/registro-exitoso", { waitUntil: "domcontentloaded" });

    // Neutral page: must not falsely claim account was just created
    await expect(page.locator("body")).not.toContainText(
      "fue creada con éxito",
    );
    await expect(
      page.getByRole("heading", { name: /verifica tu cuenta/i }),
    ).toBeVisible();

    // Neutral form to request instructions
    const emailInput = page.locator("input[type='email']");
    await expect(emailInput).toBeVisible();
    await emailInput.fill("directo@espacigo.com");
    await page.getByRole("button", { name: /reenviar/i }).click();

    // 204 generic message: does not invent fake delivery confirmation or cooldown
    await expect(page.locator("body")).toContainText(
      "Si corresponde, recibirás las instrucciones",
    );
    expect(reissueCalled).toBe(true);
  });

  test("privacy: navigating to /acceso via Link and navigating back immediately purges PII", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session" && method === "GET") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (pathname === "/api/backend/auth/terms" && method === "GET") {
        return { status: 200, body: termsFixtureData };
      }
      if (pathname === "/api/backend/auth/register" && method === "POST") {
        return { status: 201, body: {} };
      }
      return {
        status: 404,
        body: {
          error: { code: "not_found", message: `Unknown path ${pathname}` },
        },
      };
    });

    await page.goto("/registro", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");

    await page.fill("input[type='email']", "privacidad.rapida@espacigo.com");
    await page.fill("input[type='password']", "ClaveSegura123!");
    const checkboxes = page.locator("input[type='checkbox']");
    await checkboxes.first().check();
    await checkboxes.nth(1).check();
    await page.getByRole("button", { name: /crear cuenta/i }).click();

    await page.waitForURL("**/registro-exitoso");
    await expect(page.locator("body")).toContainText(
      "privacidad.rapida@espacigo.com",
    );

    // Client navigation to /acceso via Link
    const accesoLink = page.getByRole("link", { name: /ir a acceso/i });
    await accesoLink.click();
    await page.waitForURL("**/acceso");

    // Immediately navigate back
    await page.goBack();
    await page.waitForURL("**/registro-exitoso");

    // Must NOT retain PII on quick back navigation (ephemeral memory must be purged)
    await expect(page.locator("body")).not.toContainText(
      "privacidad.rapida@espacigo.com",
    );
    await expect(
      page.getByRole("heading", { name: /verifica tu cuenta/i }),
    ).toBeVisible();
  });
});
