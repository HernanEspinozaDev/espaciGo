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
    const raw = await responder(pathname, request.method(), request);
    const statusReply =
      raw.status === 404 &&
      request.method() === "POST" &&
      (pathname === "/api/backend/auth/verification/status" ||
        pathname === "/api/backend/auth/password/recovery/status")
        ? { status: 200, body: { status: "ready" } }
        : raw;
    const result = bff.adapt(request, statusReply);
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
      name: /entrar a espacigo|iniciar sesión|ir a acceso|continuar/i,
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
      page.getByText(
        /solicitud recibida\. revisa tu correo y la carpeta de spam para continuar/i,
      ),
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
      .getByRole("link", {
        name: /entrar a espacigo|iniciar sesión|ir a acceso|continuar/i,
      })
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

  test("login with email_unverified error blocks entry without redirect, displays explicit pending copy and instructions link", async ({
    page,
  }) => {
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return {
          status: 401,
          body: { error: { code: "unauthenticated" } },
        };
      }
      if (pathname === "/api/backend/auth/login" && method === "POST") {
        return {
          status: 403,
          body: {
            error: {
              code: "email_unverified",
              message: "Email is not verified",
            },
          },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto("/acceso");
    await page.getByLabel("Correo electrónico").fill("unverified@ejemplo.test");
    await page.getByLabel("Contraseña", { exact: true }).fill("Password123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();

    // Stays on /acceso without redirect
    await expect(page).toHaveURL(/\/acceso/);
    // Explicit pending copy
    await expect(
      page.getByText(
        "Debes verificar tu correo antes de ingresar. Revisa tu bandeja de entrada y la carpeta de spam.",
      ),
    ).toBeVisible();
    // Accessible link to instructions
    const instructionsLink = page.getByRole("link", {
      name: "Solicitar instrucciones de verificación",
    });
    await expect(instructionsLink).toBeVisible();
    await expect(instructionsLink).toHaveAttribute("href", "/verificar-correo");
  });

  test("correo-verificado preserves existing Go session redirect to /mi-cuenta", async ({
    page,
  }) => {
    const sessionPayload = {
      account_id: VALID_TOKEN_ID,
      roles: ["arrendatario"],
      capabilities: [],
      restricted_mode: false,
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    };
    await mockGateway(page, async (pathname, method) => {
      if (
        pathname === "/api/backend/auth/session" ||
        (pathname === "/api/backend/auth/login" && method === "POST")
      ) {
        return { status: 200, body: sessionPayload };
      }
      if (pathname === "/api/backend/auth/verification" && method === "POST") {
        return { status: 204 };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    // Establish the controlled BFF session through its login contract first.
    await page.goto("/acceso");
    await page
      .getByLabel("Correo electrónico")
      .fill("verified@ejemplo.invalid");
    await page.getByLabel("Contraseña", { exact: true }).fill("Password123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    await expect(page).toHaveURL(/\/mi-cuenta$/);

    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );
    await page.getByRole("button", { name: "Verificar mi correo" }).click();
    await page.waitForURL("**/correo-verificado");

    const sessionCta = page.getByRole("link", {
      name: "Continuar a Mi cuenta",
    });
    await expect(sessionCta).toBeVisible();
    await expect(sessionCta).toHaveAttribute("href", "/mi-cuenta");
  });

  test("verificar-correo invalid_token error shows advice copy and offers 'Comprobar acceso' CTA", async ({
    page,
  }) => {
    let verifyCount = 0;
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return { status: 401, body: { error: { code: "unauthenticated" } } };
      }
      if (pathname === "/api/backend/auth/verification" && method === "POST") {
        verifyCount++;
        return {
          status: 422,
          body: { error: { code: "invalid_token" } },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );
    await page.getByRole("button", { name: "Verificar mi correo" }).click();

    expect(verifyCount).toBe(1);
    await expect(
      page.getByText(
        "Si ya confirmaste tu correo, inicia sesión. Si aún no, solicita un nuevo enlace.",
      ),
    ).toBeVisible();
    const checkAccessCta = page.getByRole("link", {
      name: "Comprobar acceso",
    });
    await expect(checkAccessCta).toBeVisible();
    await expect(checkAccessCta).toHaveAttribute("href", "/acceso");
  });

  test("verificar-correo ambiguous 5xx error shows exactly one alert message, offers 'Comprobar acceso' CTA, and avoids automatic retry", async ({
    page,
  }) => {
    let verifyCount = 0;
    await mockGateway(page, async (pathname, method) => {
      if (pathname === "/api/backend/auth/session") {
        return { status: 401, body: { error: { code: "unauthenticated" } } };
      }
      if (pathname === "/api/backend/auth/verification" && method === "POST") {
        verifyCount++;
        return {
          status: 502,
          body: { error: { code: "bad_gateway" } },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );
    await page.getByRole("button", { name: "Verificar mi correo" }).click();

    expect(verifyCount).toBe(1);
    // Exactly one alert / message
    await expect(
      page.getByText(
        "No pudimos confirmar la respuesta. La verificación puede haberse completado. Intenta iniciar sesión; si tu correo sigue pendiente, solicita un nuevo enlace.",
      ),
    ).toBeVisible();

    // No duplicate alert banner
    await expect(
      page.getByText(/la operación no pudo confirmarse con certeza/i),
    ).not.toBeAttached();

    // Offers 'Comprobar acceso' CTA
    const checkAccessCta = page.getByRole("link", {
      name: "Comprobar acceso",
    });
    await expect(checkAccessCta).toBeVisible();
    await expect(checkAccessCta).toHaveAttribute("href", "/acceso");

    // Wait and verify no automatic retry POST was made
    await page.waitForTimeout(500);
    expect(verifyCount).toBe(1);
  });

  test("used recovery shows H1 'Enlace de recuperación ya utilizado' with CTA to acceso and nuevo enlace, never showing password inputs or submit button", async ({
    page,
  }) => {
    let probeCalled = 0;
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
        pathname === "/api/backend/auth/password/recovery/status" &&
        method === "POST"
      ) {
        probeCalled++;
        return { status: 200, body: { status: "used" } };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/recuperar-acceso#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Enlace de recuperación ya utilizado",
      }),
    ).toBeVisible();
    await expect(
      page.getByText(/la contraseña ya fue cambiada con este enlace/i),
    ).toBeVisible();

    // CTA acceso
    const accesoLink = page.getByRole("link", { name: "Ir a acceso" });
    await expect(accesoLink).toBeVisible();
    await expect(accesoLink).toHaveAttribute("href", "/acceso");

    // CTA nuevo enlace
    const nuevoEnlaceBtn = page.getByRole("button", {
      name: "Solicitar nuevo enlace",
    });
    await expect(nuevoEnlaceBtn).toBeVisible();

    // NEVER password inputs or submit button
    await expect(page.locator("#recovery-new-password")).not.toBeAttached();
    await expect(page.locator("#recovery-confirm-password")).not.toBeAttached();
    await expect(
      page.getByRole("button", { name: "Actualizar contraseña" }),
    ).not.toBeAttached();

    // Clicking nuevo enlace clears outcome and reveals email request form on same pathname
    expect(probeCalled).toBeGreaterThanOrEqual(1);
    await nuevoEnlaceBtn.click();
    await expect(page.locator("#recovery-email")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Solicitar instrucciones" }),
    ).toBeVisible();
  });

  test("used verification shows H1 'Verificación ya realizada' with CTA to acceso and never renders confirm button", async ({
    page,
  }) => {
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
        pathname === "/api/backend/auth/verification/status" &&
        method === "POST"
      ) {
        return { status: 200, body: { status: "used" } };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/verificar-correo#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Verificación ya realizada",
      }),
    ).toBeVisible();
    await expect(
      page.getByText(
        /el correo electrónico ya fue confirmado con este enlace/i,
      ),
    ).toBeVisible();

    const accesoLink = page.getByRole("link", { name: "Ir a acceso" });
    await expect(accesoLink).toBeVisible();
    await expect(accesoLink).toHaveAttribute("href", "/acceso");

    // NEVER confirm button
    await expect(
      page.getByRole("button", { name: "Verificar mi correo" }),
    ).not.toBeAttached();
  });

  test("checking state displays 'Comprobando enlace...' without password fields or buttons, and probe error fails closed", async ({
    page,
  }) => {
    let probeAttempts = 0;
    let consumeAttempts = 0;
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
        pathname === "/api/backend/auth/password/recovery/status" &&
        method === "POST"
      ) {
        probeAttempts++;
        if (probeAttempts === 1) {
          return { status: 500, body: { error: { code: "internal_error" } } };
        }
        return { status: 200, body: { status: "ready" } };
      }
      if (
        pathname === "/api/backend/auth/password/recovery/consume" &&
        method === "POST"
      ) {
        consumeAttempts++;
        return { status: 200, body: {} };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/recuperar-acceso#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    // Fail closed: error message shown with retry button, no password fields
    await expect(
      page.getByText(
        "No fue posible comprobar el estado del enlace. Intenta nuevamente.",
      ),
    ).toBeVisible();
    await expect(page.locator("#recovery-new-password")).not.toBeAttached();
    await expect(
      page.getByRole("button", { name: "Actualizar contraseña" }),
    ).not.toBeAttached();

    const retryBtn = page.getByRole("button", {
      name: "Reintentar comprobación",
    });
    await expect(retryBtn).toBeVisible();

    // Clicking retry runs probe exactly 1 additional time and resolves to ready, never consumes
    await retryBtn.click();
    await expect(page.locator("#recovery-new-password")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Actualizar contraseña" }),
    ).toBeVisible();
    expect(probeAttempts).toBe(2);
    expect(consumeAttempts).toBe(0);
  });

  test("race condition: token consumed elsewhere returns 422 on submit and re-inspection honestly updates UI to used", async ({
    page,
  }) => {
    let statusCalls = 0;
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
        pathname === "/api/backend/auth/password/recovery/status" &&
        method === "POST"
      ) {
        statusCalls++;
        if (statusCalls === 1) {
          return { status: 200, body: { status: "ready" } };
        }
        return { status: 200, body: { status: "used" } };
      }
      if (
        pathname === "/api/backend/auth/password/recovery/consume" &&
        method === "POST"
      ) {
        return {
          status: 422,
          body: {
            error: {
              code: "invalid_token",
              message: "Token has already been consumed",
            },
          },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    await page.goto(
      `/recuperar-acceso#token_id=${VALID_TOKEN_ID}&token=${SYNTHETIC_TOKEN}`,
    );

    await expect(page.locator("#recovery-new-password")).toBeVisible();
    await page.locator("#recovery-new-password").fill("SuperClaveSegura123!");
    await page
      .locator("#recovery-confirm-password")
      .fill("SuperClaveSegura123!");
    await page.getByRole("button", { name: "Actualizar contraseña" }).click();

    // Re-inspect happened and transitioned to used outcome!
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Enlace de recuperación ya utilizado",
      }),
    ).toBeVisible();
    await expect(
      page.getByText(/la contraseña ya fue cambiada con este enlace/i),
    ).toBeVisible();
    await expect(page.locator("#recovery-new-password")).not.toBeAttached();
  });

  test("followup 422 recheck A resolving late after new token B capture does not purge B or mark B as used", async ({
    page,
  }) => {
    const TOKEN_A_ID = VALID_TOKEN_ID;
    const TOKEN_A_SECRET = SYNTHETIC_TOKEN;
    const TOKEN_B_ID = "660e8400-e29b-41d4-a716-446655440001";
    const TOKEN_B_SECRET = "synthetic-action-token-b-456";

    let resolveRecheckA: (() => void) | null = null;
    const recheckAPromise = new Promise<void>((resolve) => {
      resolveRecheckA = resolve;
    });

    let statusCallsA = 0;
    let statusCallsB = 0;
    let consumeCallsA = 0;

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
        pathname === "/api/backend/auth/password/recovery/status" &&
        method === "POST"
      ) {
        const body = req.postDataJSON() as { token_id?: string };
        if (body?.token_id === TOKEN_A_ID) {
          statusCallsA++;
          if (statusCallsA === 1) {
            return { status: 200, body: { status: "ready" } };
          }
          await recheckAPromise;
          return { status: 200, body: { status: "used" } };
        }
        if (body?.token_id === TOKEN_B_ID) {
          statusCallsB++;
          return { status: 200, body: { status: "ready" } };
        }
      }
      if (
        pathname === "/api/backend/auth/password/recovery/consume" &&
        method === "POST"
      ) {
        consumeCallsA++;
        return {
          status: 422,
          body: {
            error: {
              code: "invalid_token",
              message: "Token has already been consumed",
            },
          },
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    });

    // 1. Initial screen with Token A
    await page.goto(
      `/recuperar-acceso#token_id=${TOKEN_A_ID}&token=${TOKEN_A_SECRET}`,
    );
    await expect(page.locator("#recovery-new-password")).toBeVisible();
    await page.locator("#recovery-new-password").fill("SuperClaveSegura123!");
    await page
      .locator("#recovery-confirm-password")
      .fill("SuperClaveSegura123!");

    // 2. Submit consume for A -> 422 triggers inspectRecovery followup which waits on recheckAPromise
    await page.getByRole("button", { name: "Actualizar contraseña" }).click();
    await expect.poll(() => consumeCallsA).toBe(1);

    // 3. While recheck A is pending, user captures new link B via hash
    await page.evaluate(
      ({ id, secret }) => {
        window.location.hash = `token_id=${id}&token=${secret}`;
      },
      { id: TOKEN_B_ID, secret: TOKEN_B_SECRET },
    );

    // 4. Token B probe resolves to ready, user types new password
    await expect(page.locator("#recovery-new-password")).toBeVisible();
    await expect.poll(() => statusCallsB).toBe(1);
    await page.locator("#recovery-new-password").fill("ClaveNuevaTokenB123!");
    await page
      .locator("#recovery-confirm-password")
      .fill("ClaveNuevaTokenB123!");

    // 5. Late resolution of recheck A
    resolveRecheckA!();
    await page.waitForTimeout(100);

    // 6. Token B remains enabled, no 'used' screen, no password purge
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Enlace de recuperación ya utilizado",
      }),
    ).not.toBeAttached();
    await expect(page.locator("#recovery-new-password")).toBeVisible();
    await expect(page.locator("#recovery-new-password")).toHaveValue(
      "ClaveNuevaTokenB123!",
    );
    await expect(page.locator("#recovery-confirm-password")).toHaveValue(
      "ClaveNuevaTokenB123!",
    );
    await expect(
      page.getByRole("button", { name: "Actualizar contraseña" }),
    ).toBeEnabled();
  });
});
