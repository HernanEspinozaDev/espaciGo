import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";

test.use({ trace: "off", screenshot: "off", video: "off" });

const requestsOutsideLoopback = new WeakMap<Page, boolean>();
test.beforeEach(async ({ page }) => {
  requestsOutsideLoopback.set(page, false);
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:"))
      requestsOutsideLoopback.set(page, true);
  });
});
test.afterEach(async ({ page }) => {
  expect(requestsOutsideLoopback.get(page)).toBe(false);
});

const termsId = "550e8400-e29b-41d4-a716-446655440000";
const privacyId = "660e8400-e29b-41d4-a716-446655440000";
const actionToken = "synthetic-ui-action-token-7629";
const replacementTokenId = "880e8400-e29b-41d4-a716-446655440000";
const replacementActionToken = "synthetic-ui-action-token-replacement-9041";
const accountId = "770e8400-e29b-41d4-a716-446655440000";

function completeDocuments() {
  return {
    items: [
      {
        id: termsId,
        code: "terms-local-v1",
        type: "terminos",
        sha256: "a".repeat(64),
        published_at: "2026-10-25T12:00:00Z",
        synthetic: true,
        document: {
          id: termsId,
          code: "terms-local-v1",
          type: "terminos",
          title: "Términos sintéticos de ensayo",
          scope: "local_test",
          status: "vigente",
          sections: [
            {
              heading: "Uso de ensayo",
              body: "Contenido sintético visible para la prueba local.",
            },
          ],
        },
      },
      {
        id: privacyId,
        code: "privacy-local-v1",
        type: "privacidad",
        sha256: "b".repeat(64),
        published_at: "2026-10-25T12:00:00Z",
        synthetic: true,
        document: {
          id: privacyId,
          code: "privacy-local-v1",
          type: "privacidad",
          title: "Privacidad sintética de ensayo",
          scope: "local_test",
          status: "vigente",
          sections: [
            {
              heading: "Datos de prueba",
              body: "Contenido sintético visible para la prueba local.",
            },
          ],
        },
      },
    ],
  };
}

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

function termsUnavailable(): MockResult {
  return {
    status: 503,
    body: {
      error: {
        code: "terms_unavailable",
        message: "Unavailable",
        request_id: "phase5-test",
      },
    },
  };
}

function apiError(
  status: number,
  code: string,
  headers: Record<string, string> = {},
): MockResult {
  return {
    status,
    headers,
    body: {
      error: { code, message: "Safe test error", request_id: "phase5-test" },
    },
  };
}

test("@phase5 @controlled registration stays blocked for unavailable and incomplete terms", async ({
  page,
}) => {
  let termsMode = "unavailable";
  let registrationPosts = 0;
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/terms" && method === "GET") {
      return termsMode === "unavailable"
        ? termsUnavailable()
        : {
            status: 200,
            body: {
              items: [
                {
                  ...completeDocuments().items[0],
                  document: {
                    ...completeDocuments().items[0].document,
                    sections: [],
                  },
                },
                completeDocuments().items[1],
              ],
            },
          };
    }
    if (pathname === "/api/backend/auth/register" && method === "POST")
      registrationPosts += 1;
    return apiError(404, "not_found");
  });

  await page.goto("/registro");
  await expect(
    page.getByText(/El servicio no dispone|No pudimos verificar/),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Crear cuenta" }),
  ).toBeDisabled();
  expect(registrationPosts).toBe(0);

  termsMode = "incomplete";
  await page.reload();
  await expect(
    page.getByText(
      "No pudimos verificar documentos completos y vigentes. El registro permanece bloqueado.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Crear cuenta" }),
  ).toBeDisabled();
  expect(registrationPosts).toBe(0);
});

test("@phase5 @controlled registration sends only reviewed version IDs and distinguishes 201 from 202", async ({
  page,
}) => {
  let status = 201;
  const registerBodies: Array<{
    email: string;
    password: string;
    terms_version_ids: string[];
  }> = [];
  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/terms" && method === "GET")
      return { status: 200, body: completeDocuments() };
    if (pathname === "/api/backend/auth/register" && method === "POST") {
      registerBodies.push(
        request.postDataJSON() as {
          email: string;
          password: string;
          terms_version_ids: string[];
        },
      );
      return {
        status,
        body: { account_id: accountId, status: "email_pending" },
      };
    }
    return apiError(404, "not_found");
  });

  await page.goto("/registro");
  const termsLink = page
    .getByRole("link", { name: /términos y condiciones/i })
    .first();
  const privacyLink = page
    .getByRole("link", { name: /política de privacidad/i })
    .first();
  await expect(termsLink).toBeVisible();
  await expect(privacyLink).toBeVisible();
  await expect(termsLink).toHaveAttribute("target", "_blank");
  await expect(privacyLink).toHaveAttribute("target", "_blank");
  const submit = page.getByRole("button", { name: "Crear cuenta" });
  await expect(submit).toBeDisabled();
  await page.getByLabel(/Términos y condiciones/i).check();
  await page.getByLabel(/Política de privacidad/i).check();
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  await expect(submit).toBeEnabled();
  await submit.click();
  await page.waitForURL("**/registro-exitoso");
  await expect(
    page.getByRole("heading", { name: /creada con éxito/i }),
  ).toBeVisible();
  await expect(page.locator("body")).toContainText(
    "phase5-test@example.invalid",
  );
  await expect(page.locator("body")).toContainText(
    "Siguiente paso: verifica tu correo",
  );
  expect(
    (await new AxeBuilder({ page }).analyze()).violations.map(
      (violation) => violation.id,
    ),
  ).toEqual([]);
  expect(Object.keys(registerBodies[0]).sort()).toEqual([
    "email",
    "password",
    "terms_version_ids",
  ]);
  expect(registerBodies[0].terms_version_ids).toEqual([termsId, privacyId]);
  await expect(page.getByText(accountId)).toHaveCount(0);

  status = 202;
  await page.goto("/registro");
  await page.getByLabel(/Términos y condiciones/i).check();
  await page.getByLabel(/Política de privacidad/i).check();
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  await page.getByRole("button", { name: "Crear cuenta" }).click();
  await page.waitForURL("**/registro-exitoso");
  await expect(page.getByRole("heading", { name: /creada/i })).toBeVisible();
  await expect(page.locator("body")).toContainText(
    "no pudimos confirmar el envío",
  );
  await expect(page.locator("body")).not.toContainText("Te enviamos un enlace");
  expect(registerBodies).toHaveLength(2);
});

test("@phase5 @controlled duplicate registration exposes the backend conflict and safe recovery paths", async ({
  page,
}) => {
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/terms" && method === "GET")
      return { status: 200, body: completeDocuments() };
    if (pathname === "/api/backend/auth/register" && method === "POST")
      return apiError(409, "email_registered");
    return apiError(404, "not_found");
  });
  await page.goto("/registro");
  await page.getByLabel(/Términos y condiciones/i).check();
  await page.getByLabel(/Política de privacidad/i).check();
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  await page.getByRole("button", { name: "Crear cuenta" }).click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
    "Este correo ya tiene una cuenta",
  );
  await expect(page.getByRole("link", { name: "Acceder" })).toHaveAttribute(
    "href",
    "/acceso",
  );
  await expect(
    page.getByRole("link", { name: "Recuperar acceso" }),
  ).toHaveAttribute("href", "/recuperar-acceso");
});

test("@phase5 @controlled consent_invalid clears both approvals and reloads the documents", async ({
  page,
}) => {
  let termsLoads = 0;
  let submittedIds: string[] = [];
  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/terms" && method === "GET") {
      termsLoads += 1;
      const set = completeDocuments();
      if (termsLoads > 1) {
        set.items[0].id = "880e8400-e29b-41d4-a716-446655440000";
        set.items[0].document.id = set.items[0].id;
      }
      return { status: 200, body: set };
    }
    if (pathname === "/api/backend/auth/register" && method === "POST") {
      submittedIds = (request.postDataJSON() as { terms_version_ids: string[] })
        .terms_version_ids;
      return {
        status: 422,
        body: {
          error: {
            code: "validation_error",
            message: "Invalid",
            request_id: "phase5-test",
            details: [{ field: "terms_version_ids", code: "consent_invalid" }],
          },
        },
      };
    }
    return apiError(404, "not_found");
  });
  await page.goto("/registro");
  await page.getByLabel(/Términos y condiciones/i).check();
  await page.getByLabel(/Política de privacidad/i).check();
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  await page.getByRole("button", { name: "Crear cuenta" }).click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
    "Los documentos cambiaron",
  );
  await expect(
    page.getByText("Revisa y acepta por separado ambos documentos."),
  ).toBeVisible();
  await expect(page.getByLabel(/Términos y condiciones/i)).not.toBeChecked();
  await expect(page.getByLabel(/Política de privacidad/i)).not.toBeChecked();
  expect(submittedIds).toEqual([termsId, privacyId]);
  expect(termsLoads).toBe(2);
});

test("@phase5 @controlled recovery request has identical generic copy and shows only a real 429 Retry-After", async ({
  page,
}) => {
  const responseStatus = 202;
  let retry = false;
  await mockGateway(page, async (pathname, method) => {
    if (
      pathname === "/api/backend/auth/password/recovery" &&
      method === "POST"
    ) {
      return retry
        ? apiError(429, "rate_limited", { "retry-after": "5" })
        : {
            status: responseStatus,
            body: { status: "accepted", message: "different server text" },
          };
    }
    return apiError(404, "not_found");
  });
  await page.goto("/recuperar-acceso");
  await page
    .getByLabel("Correo electrónico")
    .fill("unknown-phase5@example.invalid");
  await page
    .getByRole("button", { name: "Solicitar instrucciones" })
    .last()
    .click();
  const generic =
    "Solicitud recibida. Revisa tu correo y la carpeta de spam para continuar. Si no recibes el mensaje, comprueba la dirección e inténtalo de nuevo.";
  await expect(page.getByRole("status")).toHaveText(generic);
  await expect(
    page.getByText(/comprueba la dirección e inténtalo de nuevo/),
  ).toBeVisible();

  retry = true;
  await page
    .getByLabel("Correo electrónico")
    .fill("unknown-phase5@example.invalid");
  const retryResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname ===
        "/api/backend/auth/password/recovery" && response.status() === 429,
  );
  await page
    .getByRole("button", { name: "Solicitar instrucciones" })
    .last()
    .click();
  const retryResponse = await retryResponsePromise;
  expect(retryResponse.headers()["retry-after"]).toBe("5");
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
    "Espera antes de volver a intentarlo",
  );
  const requestButton = page
    .getByRole("button", { name: "Solicitar instrucciones" })
    .last();
  await expect(requestButton).toBeDisabled();
  await expect(page.getByText(/aproximadamente \d+ segundos/)).toBeVisible({
    timeout: 1_000,
  });
  await expect(requestButton).toBeEnabled({ timeout: 7_000 });
});

test("@phase5 @controlled fragment credentials are scrubbed, replaceable, and consumed only after explicit action", async ({
  page,
}) => {
  let verificationCalls = 0;
  let consumedCredential: { token?: string; token_id?: string } = {};
  let secretInConsole = false;
  const requestUrls: string[] = [];
  const secrets = [
    actionToken,
    termsId,
    replacementActionToken,
    replacementTokenId,
  ];
  page.on("request", (request) => requestUrls.push(request.url()));
  page.on("console", (message) => {
    if (secrets.some((secret) => message.text().includes(secret))) {
      secretInConsole = true;
    }
  });
  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/verification" && method === "POST") {
      verificationCalls += 1;
      consumedCredential = request.postDataJSON() as {
        token?: string;
        token_id?: string;
      };
      return apiError(422, "invalid_token");
    }
    return apiError(404, "not_found");
  });
  await page.goto(
    `/verificar-correo?unexpected=drop#token=${actionToken}&token_id=${termsId}`,
  );
  await expect(
    page.getByRole("button", { name: "Verificar mi correo" }),
  ).toBeVisible();
  expect(verificationCalls).toBe(0);
  await expect(page).toHaveURL(
    (url) =>
      url.pathname === "/verificar-correo" &&
      url.search === "" &&
      url.hash === "",
  );

  await page.evaluate(
    ({ token, tokenId }) => {
      window.location.hash = `token=${encodeURIComponent(token)}&token_id=${encodeURIComponent(tokenId)}`;
    },
    { token: replacementActionToken, tokenId: replacementTokenId },
  );
  await expect(page).toHaveURL(
    (url) =>
      url.pathname === "/verificar-correo" &&
      url.search === "" &&
      url.hash === "",
  );
  expect(verificationCalls).toBe(0);

  const scrubbed = await page.evaluate(async (sensitiveValues) => {
    const containsSecret = (value: string) =>
      sensitiveValues.some((secret) => value.includes(secret));
    return {
      url: containsSecret(location.href),
      html: containsSecret(document.documentElement.outerHTML),
      localStorage: Object.values(localStorage).some(containsSecret),
      sessionStorage: Object.values(sessionStorage).some(containsSecret),
      cookies: containsSecret(document.cookie),
      historyState: containsSecret(JSON.stringify(history.state ?? {})),
      indexedDbNames: (await indexedDB.databases()).some((database) =>
        sensitiveValues.some((secret) => database.name?.includes(secret)),
      ),
    };
  }, secrets);
  expect(scrubbed).toEqual({
    url: false,
    html: false,
    localStorage: false,
    sessionStorage: false,
    cookies: false,
    historyState: false,
    indexedDbNames: false,
  });
  await page.getByRole("button", { name: "Verificar mi correo" }).click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
    "El enlace no es válido o ya no está disponible",
  );
  expect(verificationCalls).toBe(1);
  expect(consumedCredential).toEqual({
    token: replacementActionToken,
    token_id: replacementTokenId,
  });
  expect(
    requestUrls.every((url) =>
      secrets.every((secret) => !url.includes(secret)),
    ),
  ).toBe(true);
  expect(secretInConsole).toBe(false);
  await expect(page).not.toHaveURL(/#|token=/i);
});

test("@phase5 @controlled invalid fragment replacement clears the prior action credential", async ({
  page,
}) => {
  let verificationCalls = 0;
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/verification" && method === "POST") {
      verificationCalls += 1;
    }
    return apiError(404, "not_found");
  });
  await page.goto(`/verificar-correo#token=${actionToken}&token_id=${termsId}`);
  await expect(
    page.getByRole("button", { name: "Verificar mi correo" }),
  ).toBeVisible();
  await page.evaluate((token) => {
    window.location.hash = `token=${encodeURIComponent(token)}&token_id=550E8400-E29B-41D4-A716-446655440000`;
  }, replacementActionToken);
  await expect(
    page.getByRole("button", { name: "Verificar mi correo" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Solicitar instrucciones" }),
  ).toBeVisible();
  await expect(page.getByLabel("Correo electrónico")).toBeVisible();
  await expect(page.getByLabel("Correo electrónico")).toHaveValue("");
  await expect(page).not.toHaveURL(/#|token=/i);
  expect(verificationCalls).toBe(0);
});

test("@phase5 @controlled cancelling recovery clears every manually entered secret", async ({
  page,
}) => {
  await mockGateway(page, async () => apiError(404, "not_found"));
  await page.goto(`/recuperar-acceso#token=${actionToken}&token_id=${termsId}`);
  const newPasswordInput = page.getByLabel("Nueva contraseña", { exact: true });
  const confirmPasswordInput = page.getByLabel("Confirmar nueva contraseña", {
    exact: true,
  });
  await expect(newPasswordInput).toBeVisible();
  await expect(confirmPasswordInput).toBeVisible();
  await newPasswordInput.fill("ABC123!!");
  await confirmPasswordInput.fill("ABC123!!");

  await page.evaluate(() => {
    window.location.hash = "#cancel";
  });
  await expect(
    page.getByRole("button", { name: "Solicitar instrucciones" }),
  ).toBeVisible();
  await expect(newPasswordInput).toHaveCount(0);

  await page.evaluate(
    ({ token, tokenId }) => {
      window.location.hash = `token=${encodeURIComponent(token)}&token_id=${encodeURIComponent(tokenId)}`;
    },
    { token: replacementActionToken, tokenId: replacementTokenId },
  );
  await expect(
    page.getByLabel("Nueva contraseña", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Nueva contraseña", { exact: true }),
  ).toHaveValue("");
  await expect(
    page.getByLabel("Confirmar nueva contraseña", { exact: true }),
  ).toHaveValue("");

  const scrubbed = await page.evaluate(
    async (sensitiveValues) => {
      const containsSecret = (value: string) =>
        sensitiveValues.some((secret) => value.includes(secret));
      return {
        url: containsSecret(location.href),
        html: containsSecret(document.documentElement.outerHTML),
        localStorage: Object.values(localStorage).some(containsSecret),
        sessionStorage: Object.values(sessionStorage).some(containsSecret),
        cookies: containsSecret(document.cookie),
        historyState: containsSecret(JSON.stringify(history.state ?? {})),
      };
    },
    [actionToken, termsId, "ABC123!!"],
  );
  expect(scrubbed).toEqual({
    url: false,
    html: false,
    localStorage: false,
    sessionStorage: false,
    cookies: false,
    historyState: false,
  });
});

test("@phase5 @controlled recovery reset requires an explicit click and does not auto-login after 204", async ({
  page,
}) => {
  let consumeCalls = 0;
  let loginCalls = 0;
  await mockGateway(page, async (pathname, method) => {
    if (
      pathname === "/api/backend/auth/password/recovery/consume" &&
      method === "POST"
    ) {
      consumeCalls += 1;
      return { status: 204 };
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loginCalls += 1;
    }
    return apiError(404, "not_found");
  });
  await page.goto(`/recuperar-acceso#token=${actionToken}&token_id=${termsId}`);
  expect(consumeCalls).toBe(0);
  await expect(
    page.getByRole("button", { name: "Mostrar nueva contraseña" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Mostrar confirmar nueva contraseña" }),
  ).toBeVisible();
  await page.getByLabel("Nueva contraseña", { exact: true }).fill("ABC123!!");
  await page
    .getByLabel("Confirmar nueva contraseña", { exact: true })
    .fill("ABC123!!");
  const submitButton = page.getByRole("button", {
    name: "Actualizar contraseña",
  });
  await expect(submitButton).toBeEnabled();
  await submitButton.click();
  await expect(
    page.getByRole("heading", { level: 1, name: "Contraseña actualizada" }),
  ).toBeVisible();
  await expect(page.locator("body")).toContainText(
    "Tu contraseña ha sido actualizada correctamente",
  );
  await expect(page.locator("form")).toHaveCount(0);
  expect(consumeCalls).toBe(1);
  expect(loginCalls).toBe(0);
  await expect(page).toHaveURL(/recuperar-acceso/);
  await expect(
    page.getByLabel("Nueva contraseña", { exact: true }),
  ).toHaveCount(0);
});

test("@phase5 @controlled verification success and reissue 204 remain generic without auto-login", async ({
  page,
}) => {
  let verificationCalls = 0;
  let reissueCalls = 0;
  let loginCalls = 0;
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/verification" && method === "POST") {
      verificationCalls += 1;
      return { status: 204 };
    }
    if (
      pathname === "/api/backend/auth/verification/reissue" &&
      method === "POST"
    ) {
      reissueCalls += 1;
      return { status: 204 };
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loginCalls += 1;
    }
    return apiError(404, "not_found");
  });

  await page.goto(`/verificar-correo#token=${actionToken}&token_id=${termsId}`);
  await page.getByRole("button", { name: "Verificar mi correo" }).click();
  await page.waitForURL("**/correo-verificado");
  await expect(
    page.getByRole("heading", { name: "Correo verificado" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Entrar a EspaciGo" }),
  ).toHaveAttribute("href", "/acceso");
  expect(verificationCalls).toBe(1);
  expect(loginCalls).toBe(0);

  await page.goto("/verificar-correo");
  await page
    .getByLabel("Correo electrónico")
    .fill("unknown-phase5@example.invalid");
  await page
    .getByRole("button", { name: "Solicitar instrucciones" })
    .last()
    .click();
  await expect(page.getByRole("status")).toHaveText(
    "Si la cuenta está pendiente y corresponde, intentaremos enviar nuevas instrucciones. No podemos confirmar la existencia de la cuenta ni la entrega del correo.",
  );
  expect(reissueCalls).toBe(1);
  expect(loginCalls).toBe(0);
  await expect(page.getByText(/aproximadamente \d+ segundos/)).toHaveCount(0);
});

test("@phase5 @controlled recovery ambiguous response does not resubmit a one-time token", async ({
  page,
}) => {
  let consumeCalls = 0;
  await mockGateway(page, async (pathname, method) => {
    if (
      pathname === "/api/backend/auth/password/recovery/consume" &&
      method === "POST"
    ) {
      consumeCalls += 1;
      return apiError(500, "internal_error");
    }
    return apiError(404, "not_found");
  });
  await page.goto(`/recuperar-acceso#token=${actionToken}&token_id=${termsId}`);
  await page.getByLabel("Nueva contraseña", { exact: true }).fill("ABC123!!");
  await page
    .getByLabel("Confirmar nueva contraseña", { exact: true })
    .fill("ABC123!!");
  await page.getByRole("button", { name: "Actualizar contraseña" }).click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
    "No pudimos confirmar el cambio de contraseña. Por seguridad, no lo reintentaremos automáticamente",
  );
  expect(consumeCalls).toBe(1);
  await expect(
    page.getByRole("button", { name: "Actualizar contraseña" }),
  ).toHaveCount(0);
});

test("@phase5 @controlled login 429 honors only the server Retry-After window", async ({
  page,
}) => {
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return apiError(429, "login_blocked", { "retry-after": "2" });
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return apiError(401, "unauthenticated");
    }
    return apiError(404, "not_found");
  });
  await page.goto("/acceso");
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  const submit = page.getByRole("button", { name: "Iniciar sesión" });
  await submit.click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toContainText(
    "El acceso está bloqueado temporalmente",
  );
  await expect(submit).toBeDisabled();
  await expect(page.getByText(/aproximadamente \d+ segundos/)).toBeVisible();
  await expect(submit).toBeEnabled({ timeout: 4_000 });
});

test("@phase5 @controlled login 401 is generic and confirmed login honors only an allowlisted return_to", async ({
  page,
}) => {
  let allowLogin = false;
  let loginRequests = 0;
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loginRequests++;
      if (!allowLogin) return apiError(401, "unauthenticated");
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 60_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    }
    return apiError(404, "not_found");
  });

  await page.goto("/acceso?return_to=%2Fdashboard%2Fsecurity");
  await page
    .getByLabel("Correo electrónico")
    .fill("unknown-phase5@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("Bad123!!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toHaveText(
    "El correo o la contraseña no son válidos.",
  );

  allowLogin = true;
  await page.goto("/acceso?return_to=%2Fdashboard%2Fsecurity");
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/dashboard\/security$/);

  const loginRequestsBefore = loginRequests;
  await page.goto("/acceso?return_to=https%3A%2F%2Foutside.invalid%2Fpath");
  await expect(page).toHaveURL(
    // main already uses Mi cuenta as its safe fallback; keep the external URL rejected.
    (url) => url.pathname === "/mi-cuenta" && url.hostname === "127.0.0.1",
  );
  await expect(page.getByLabel("Correo electrónico")).not.toBeVisible();
  expect(loginRequests).toBe(loginRequestsBefore);
});

test("@phase5 @a11y identity routes fit both viewports and have no Axe violations", async ({
  page,
}) => {
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/terms" && method === "GET")
      return termsUnavailable();
    return apiError(401, "unauthenticated");
  });
  for (const route of [
    "/acceso",
    "/login",
    "/registro",
    "/verificar-correo",
    "/recuperar-acceso",
  ]) {
    await page.goto(route);
    await expect(page.locator("h1")).toHaveCount(1);
    for (const viewport of [
      { width: 390, height: 844 },
      { width: 1365, height: 900 },
    ]) {
      await page.setViewportSize(viewport);
      const dimensions = await page.evaluate(() => ({
        width: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.width);
      const results = await new AxeBuilder({ page }).analyze();
      expect(
        results.violations.map((violation) => violation.id),
        JSON.stringify(
          results.violations.map((violation) => ({
            id: violation.id,
            nodes: violation.nodes.map((node) => ({
              target: node.target,
              failureSummary: node.failureSummary,
            })),
          })),
        ),
      ).toEqual([]);
    }
  }
});

test("@phase5 @controlled return_to parser rejects malformed query through the UI", async ({
  page,
}) => {
  let loginRequest = 0;
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loginRequest += 1;
      return apiError(401, "unauthenticated");
    }
    return apiError(404, "not_found");
  });
  await page.goto("/acceso?return_to=%2Fdashboard%3Funknown%3D1");
  await expect(
    page.getByRole("link", { name: "EspaciGo, volver al marketplace" }),
  ).toHaveAttribute("href", "/");
  await page
    .getByLabel("Correo electrónico")
    .fill("phase5-test@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("ABC123!!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page.locator('[role="alert"][tabindex="-1"]')).toHaveText(
    "El correo o la contraseña no son válidos.",
  );
  expect(loginRequest).toBe(1);
});
