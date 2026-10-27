import AxeBuilder from "@axe-core/playwright";
import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

test.use({ trace: "off", screenshot: "off", video: "off" });

const VALID_TOKEN_ID = "550e8400-e29b-41d4-a716-446655440000";
const SYNTHETIC_TOKEN = "synthetic-action-token-xyz-123";

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

test.describe("Verification and Recovery UX", () => {
  test("verificar-correo with fragment purges hash, hides manual token inputs, requires voluntary confirm, and redirects to /correo-verificado", async ({
    page,
  }) => {
    let verifyConsumed = 0;
    let verifyPayload: unknown = null;

    await mockGateway(page, async (pathname, method, req) => {
      if (pathname === "/api/backend/auth/session") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (pathname === "/api/backend/auth/verification" && method === "POST") {
        verifyConsumed++;
        verifyPayload = req.postDataJSON();
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    // Navigate with fragment carrying token_id and token
    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    // Fragment must be scrubbed immediately
    await expect
      .poll(async () => {
        return page.evaluate(() => window.location.hash);
      })
      .toBe("");

    // Manual token inputs and scanner technical explanation must NOT exist
    await expect(page.locator("#verification-token-id")).not.toBeAttached();
    await expect(page.locator("#verification-token")).not.toBeAttached();
    await expect(
      page.getByText(/escáneres y vistas previas/i),
    ).not.toBeAttached();

    // Must not consume token automatically on load/GET
    expect(verifyConsumed).toBe(0);

    // Must display voluntary single confirmation button
    const confirmBtn = page.getByRole("button", {
      name: "Verificar mi correo",
    });
    await expect(confirmBtn).toBeVisible();

    // Click to confirm voluntarily
    await confirmBtn.click();

    // Verify was called once with payload
    expect(verifyConsumed).toBe(1);
    expect(verifyPayload).toEqual({
      token_id: VALID_TOKEN_ID,
      token: SYNTHETIC_TOKEN,
    });

    // Must navigate to /correo-verificado and show verified confirmation
    await page.waitForURL("**/correo-verificado");
    await expect(
      page.getByRole("heading", { name: "Correo verificado" }),
    ).toBeVisible();
    await expect(
      page.getByText(/tu cuenta de espacigo ya está lista/i),
    ).toBeVisible();
    const continueCta = page.getByRole("link", {
      name: /iniciar sesión|ir a acceso|continuar/i,
    });
    await expect(continueCta).toBeVisible();
  });

  test("verificar-correo without credential shows instruction to open email and reissue form, with no manual token fields", async ({
    page,
  }) => {
    let reissueCalled = 0;
    let reissueEmail = "";

    await mockGateway(page, async (pathname, method, req) => {
      if (pathname === "/api/backend/auth/session") {
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
        reissueCalled++;
        reissueEmail = (req.postDataJSON() as { email?: string })?.email ?? "";
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto("/verificar-correo");

    // Shows instructions to open the email link
    await expect(
      page.getByText(/abre el enlace que enviamos a tu correo/i),
    ).toBeVisible();

    // Manual token fields must NOT exist
    await expect(page.locator("#verification-token-id")).not.toBeAttached();
    await expect(page.locator("#verification-token")).not.toBeAttached();

    // Reissue form is available
    const emailInput = page.getByLabel(/correo electrónico/i);
    await expect(emailInput).toBeVisible();
    await emailInput.fill("persona@ejemplo.test");
    await page
      .getByRole("button", { name: /solicitar instrucciones|reenviar enlace/i })
      .click();

    expect(reissueCalled).toBe(1);
    expect(reissueEmail).toBe("persona@ejemplo.test");
  });

  test("correo-verificado direct visit without in-memory context shows neutral state without claiming verification", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname) => {
      if (pathname === "/api/backend/auth/session") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto("/correo-verificado");

    // Must not show confirmed heading or claim account verified
    await expect(
      page.getByRole("heading", { name: "Correo verificado" }),
    ).not.toBeVisible();
    await expect(
      page.getByText(/no encontramos una confirmación reciente/i),
    ).toBeVisible();
  });

  test("recuperar-acceso without credential has no 'Tengo un enlace' button, no manual token fields, and submits generic request", async ({
    page,
  }) => {
    let requestCalled = 0;

    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (
        pathname === "/api/backend/auth/password/recovery" &&
        method === "POST"
      ) {
        requestCalled++;
        return {
          status: 202,
          body: { status: "accepted", message: "Recovery instructions queued" },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto("/recuperar-acceso");

    // 'Tengo un enlace' button must NOT exist
    await expect(
      page.getByRole("button", { name: /tengo un enlace/i }),
    ).not.toBeAttached();
    await expect(page.locator("#recovery-token-id")).not.toBeAttached();
    await expect(page.locator("#recovery-token")).not.toBeAttached();

    // Submit request
    const emailInput = page.getByLabel(/correo electrónico/i);
    await emailInput.fill("recuperar@ejemplo.test");
    await page
      .getByRole("button", { name: /solicitar instrucciones/i })
      .click();

    expect(requestCalled).toBe(1);
    await expect(
      page.getByText(/si existe una cuenta elegible, recibirás instrucciones/i),
    ).toBeVisible();
  });

  test("recuperar-acceso with credential directly shows reset form, consumes token once, and on success hides forms showing 'Contraseña actualizada'", async ({
    page,
  }) => {
    let consumeCount = 0;
    let consumeBody: unknown = null;

    await mockGateway(page, async (pathname, method, req) => {
      if (pathname === "/api/backend/auth/session") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      if (
        pathname === "/api/backend/auth/password/recovery/consume" &&
        method === "POST"
      ) {
        consumeCount++;
        consumeBody = req.postDataJSON();
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/recuperar-acceso#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    // Fragment scrubbed
    await expect
      .poll(async () => {
        return page.evaluate(() => window.location.hash);
      })
      .toBe("");

    // No 'Tengo un enlace' or manual ID/token inputs
    await expect(
      page.getByRole("button", { name: /tengo un enlace/i }),
    ).not.toBeAttached();
    await expect(page.locator("#recovery-token-id")).not.toBeAttached();
    await expect(page.locator("#recovery-token")).not.toBeAttached();

    // Directly shows password fields
    const newPwdInput = page.getByLabel("Nueva contraseña", { exact: true });
    const confirmPwdInput = page.getByLabel("Confirmar nueva contraseña", {
      exact: true,
    });
    await expect(newPwdInput).toBeVisible();
    await expect(confirmPwdInput).toBeVisible();

    await newPwdInput.fill("ClaveSegura123!");
    await confirmPwdInput.fill("ClaveSegura123!");

    const submitBtn = page.getByRole("button", {
      name: "Actualizar contraseña",
    });
    await submitBtn.click();

    expect(consumeCount).toBe(1);
    expect(consumeBody).toEqual({
      token_id: VALID_TOKEN_ID,
      token: SYNTHETIC_TOKEN,
      new_password: "ClaveSegura123!",
      confirm_password: "ClaveSegura123!",
    });

    // Forms must be hidden! Result must be unequivocal "Contraseña actualizada" + CTA
    await expect(
      page.getByRole("heading", { name: "Contraseña actualizada" }),
    ).toBeVisible();
    await expect(newPwdInput).not.toBeAttached();
    await expect(
      page.getByRole("button", { name: /solicitar instrucciones/i }),
    ).not.toBeAttached();
    const accessCta = page
      .getByRole("link", {
        name: /iniciar sesión|ir a acceso|acceso/i,
      })
      .first();
    await expect(accessCta).toBeVisible();
  });

  test("verificar-correo heading hierarchy is sequential (h2 under h1) and passes axe a11y", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname) => {
      if (pathname === "/api/backend/auth/session") {
        return {
          status: 401,
          body: {
            error: { code: "unauthenticated", message: "No active session" },
          },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto("/verificar-correo");

    // Must use sequential h2 for "Solicitar nuevo enlace"
    const heading = page.getByRole("heading", {
      name: "Solicitar nuevo enlace",
      level: 2,
    });
    await expect(heading).toBeVisible();

    const axe = await new AxeBuilder({ page }).analyze();
    expect(axe.violations.map((v) => v.id)).toEqual([]);
  });

  test("reissue form on /verificar-correo enforces required email input and prevents empty submission", async ({
    page,
  }) => {
    let reissueCalled = 0;
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return { status: 401, body: { error: { code: "unauthenticated" } } };
      }
      if (
        pathname === "/api/backend/auth/verification/reissue" &&
        method === "POST"
      ) {
        reissueCalled++;
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto("/verificar-correo");

    const emailInput = page.getByLabel(/correo electrónico/i);
    await expect(emailInput).toHaveAttribute("required", "");
    await expect(emailInput).toHaveAttribute("type", "email");
    expect(reissueCalled).toBe(0);

    // Check /recuperar-acceso email input as well
    await page.goto("/recuperar-acceso");
    const recoveryEmailInput = page.getByLabel(/correo electrónico/i);
    await expect(recoveryEmailInput).toHaveAttribute("required", "");
    await expect(recoveryEmailInput).toHaveAttribute("type", "email");
  });

  test("recuperar-acceso with credential disables submit button on password policy mismatch or confirmation divergence", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname) => {
      if (pathname === "/api/backend/auth/session") {
        return { status: 401, body: { error: { code: "unauthenticated" } } };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/recuperar-acceso#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    const newPwdInput = page.getByLabel("Nueva contraseña", { exact: true });
    const confirmPwdInput = page.getByLabel("Confirmar nueva contraseña", {
      exact: true,
    });
    const submitBtn = page.getByRole("button", {
      name: "Actualizar contraseña",
    });

    // Initially empty -> submit button must be disabled
    await expect(submitBtn).toBeDisabled();

    // Weak password (< 8 chars)
    await newPwdInput.fill("short");
    await confirmPwdInput.fill("short");
    await expect(submitBtn).toBeDisabled();

    // Missing symbol/punctuation
    await newPwdInput.fill("Password123");
    await confirmPwdInput.fill("Password123");
    await expect(submitBtn).toBeDisabled();

    // Valid policy password, but mismatched confirmation
    await newPwdInput.fill("ClaveSegura123!");
    await confirmPwdInput.fill("ClaveSegura456!");
    await expect(submitBtn).toBeDisabled();

    // Matching valid password -> enabled
    await confirmPwdInput.fill("ClaveSegura123!");
    await expect(submitBtn).toBeEnabled();
  });

  test("correo-verificado one-shot context expires when leaving page, preventing stale success display", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return { status: 401, body: { error: { code: "unauthenticated" } } };
      }
      if (pathname === "/api/backend/auth/verification" && method === "POST") {
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    // 1. Verify successfully
    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );
    await page.getByRole("button", { name: "Verificar mi correo" }).click();
    await page.waitForURL("**/correo-verificado");
    await expect(
      page.getByRole("heading", { name: "Correo verificado" }),
    ).toBeVisible();

    // 2. Leave the page (navigate to /acceso)
    await page
      .getByRole("link", { name: /iniciar sesión|ir a acceso|continuar/i })
      .click();
    await page.waitForURL("**/acceso");

    // 3. Wait a moment for scheduled clear and visit /correo-verificado again directly
    await page.waitForTimeout(1600);
    await page.goto("/correo-verificado");

    // 4. Must NOT show verified heading anymore!
    await expect(
      page.getByRole("heading", { name: "Correo verificado" }),
    ).not.toBeVisible();
    await expect(
      page.getByText(/no encontramos una confirmación reciente/i),
    ).toBeVisible();
  });

  test("fragment/credential change while verify request is in-flight cancels previous operation and ignores late response", async ({
    page,
  }) => {
    let resolveVerify: (() => void) | null = null;
    let verifyCount = 0;

    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return { status: 401, body: { error: { code: "unauthenticated" } } };
      }
      if (pathname === "/api/backend/auth/verification" && method === "POST") {
        verifyCount++;
        await new Promise<void>((resolve) => {
          resolveVerify = resolve;
        });
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    // 1. Navigate with first token
    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    // Click confirm to start verify request
    const confirmBtn = page.getByRole("button", {
      name: "Verificar mi correo",
    });
    await confirmBtn.click();
    expect(verifyCount).toBe(1);

    // 2. While request is in-flight, simulate link change / new fragment
    const SECOND_TOKEN_ID = "660e8400-e29b-41d4-a716-446655440001";
    const SECOND_TOKEN = "second-synthetic-token-abc-456";
    await page.evaluate(
      ({ id, token }) => {
        window.location.hash = `token_id=${id}&token=${token}`;
      },
      { id: SECOND_TOKEN_ID, token: SECOND_TOKEN },
    );

    // 3. Now resolve the first verify request
    if (resolveVerify) {
      (resolveVerify as () => void)();
    }
    await page.waitForTimeout(100);

    // 4. Must NOT have navigated to /correo-verificado for the superseded request!
    expect(page.url()).not.toContain("/correo-verificado");
  });
});
