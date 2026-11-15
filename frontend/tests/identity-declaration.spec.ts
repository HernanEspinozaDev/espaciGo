import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

test.use({ trace: "off", screenshot: "off", video: "off" });

const identityDeclarationConsentVersion =
  process.env.NEXT_PUBLIC_IDENTITY_DECLARATION_CONSENT_VERSION?.trim() ||
  "id-consent-20261108";

async function expectAccountPageAccessible(page: Page): Promise<void> {
  for (const viewport of [
    { width: 1365, height: 900 },
    { width: 390, height: 844 },
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

test("identity declaration saves synthetic data and waits for readback", async ({
  page,
}) => {
  const accountId = "550e8400-e29b-41d4-a716-446655440000";
  const declarationId = "990e8400-e29b-41d4-a716-446655440000";
  const sessionToken = "synthetic-identity-declaration-session";
  const requestsOutsideLoopback: string[] = [];
  const declarationRequests: string[] = [];
  const pageErrors: string[] = [];
  let loggedIn = false;
  let savedDeclaration: Record<string, unknown> | null = null;
  let declarationWrites = 0;
  let caseCreates = 0;
  let evidenceUploads = 0;
  page.on("pageerror", (error) => pageErrors.push(error.message));

  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      requestsOutsideLoopback.push(request.url());
    }
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (
      status: number,
      value: unknown,
      responseHeaders: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
          ...responseHeaders,
        },
        body: JSON.stringify(value),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          })
        : json(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        access_token: sessionToken,
        token_type: "Bearer",
        expires_at: new Date(Date.now() + 60_000).toISOString(),
        restricted_mode: false,
      });
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      declarationRequests.push("GET");
      return savedDeclaration
        ? json(200, savedDeclaration)
        : json(404, { error: { code: "not_found" } });
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "POST"
    ) {
      declarationRequests.push("POST");
      declarationWrites += 1;
      expect(request.postDataJSON()).toEqual({
        first_name: "Persona",
        last_name: "Sintética de Prueba",
        date_of_birth: "1990-01-02",
        rut: "11.222.333-9",
        id_country: "CHL",
        document_type: "ID",
        consent_given: true,
        consent_version: identityDeclarationConsentVersion,
      });
      savedDeclaration = {
        id: declarationId,
        revision: 1,
        first_name: "Persona",
        last_name: "Sintética de Prueba",
        date_of_birth: "1990-01-02",
        rut: "11222333-9",
        id_country: "CHL",
        document_type: "ID",
        consent_version: identityDeclarationConsentVersion,
        consent_given_at: "2026-11-13T12:00:00Z",
      };
      return json(201, savedDeclaration);
    }
    if (
      (pathname === "/api/backend/verifications" ||
        pathname === "/api/backend/verifications/eligibility") &&
      method === "GET"
    ) {
      return json(200, { items: [] });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return json(200, []);
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      caseCreates += 1;
    }
    if (
      /^\/api\/backend\/document-verification\/cases\/[^/]+\/files$/.test(
        pathname,
      ) &&
      method === "POST"
    ) {
      evidenceUploads += 1;
    }
    return json(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta");
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  await page.getByRole("link", { name: "Verificaciones", exact: true }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  await expect(page).toHaveTitle("Verificaciones | Mi Cuenta");
  await expect(page.getByText(/\bKYC\b|\bKYB\b/)).toHaveCount(0);

  const declaration = page.getByRole("region", {
    name: "Declaración de identidad",
  });
  await expect(declaration).toBeVisible();
  await expect(
    declaration.getByText(/no inicia una verificación ni carga documentos/),
  ).toBeVisible();
  await expect(
    declaration.getByText(/Solo pruebas locales con datos sintéticos/),
  ).toBeVisible();
  await expect(
    declaration.getByRole("button", { name: "Guardar declaración" }),
  ).toBeDisabled();
  await expectAccountPageAccessible(page);
  await page.setViewportSize({ width: 1365, height: 900 });

  await expect(declaration.getByLabel("Fecha de nacimiento")).toBeVisible();
  await declaration.getByLabel("Nombres legales").fill("Persona");
  await expect(declaration.getByLabel("Nombres legales")).toHaveValue(
    "Persona",
  );
  expect(pageErrors).toEqual([]);
  await declaration.getByLabel("Apellidos legales").fill("Sintética de Prueba");
  await expect(declaration.getByLabel("Apellidos legales")).toHaveValue(
    "Sintética de Prueba",
  );
  expect(pageErrors).toEqual([]);
  await expect(declaration.getByLabel("Fecha de nacimiento")).toBeVisible();
  await declaration.getByLabel("Fecha de nacimiento").fill("1990-01-02");
  await expect(declaration.getByLabel("Fecha de nacimiento")).toHaveValue(
    "1990-01-02",
  );
  await declaration.getByLabel("RUN/RUT").fill("11.222.333-9");
  await expect(declaration.getByLabel("RUN/RUT")).toHaveValue("11.222.333-9");
  expect(pageErrors).toEqual([]);
  await expect(
    declaration.getByRole("button", { name: "Guardar declaración" }),
  ).toBeDisabled();
  await declaration
    .getByLabel(/Confirmo que usaré solo datos sintéticos/)
    .check();
  await expect(
    declaration.getByRole("button", { name: "Guardar declaración" }),
  ).toBeEnabled();
  await declaration
    .getByRole("button", { name: "Guardar declaración" })
    .click();

  await expect(declaration.getByText("Declaración guardada.")).toBeVisible();
  await expectAccountPageAccessible(page);
  await expect(
    declaration.getByText(/no significa que la identidad esté verificada/),
  ).toBeVisible();
  await expect(
    declaration.getByRole("link", {
      name: "Iniciar verificación de identidad",
    }),
  ).toBeVisible();
  expect(declarationRequests).toEqual(["GET", "POST", "GET"]);
  expect(declarationWrites).toBe(1);
  expect(caseCreates).toBe(0);
  expect(evidenceUploads).toBe(0);

  await declaration.getByRole("button", { name: "Consultar estado" }).click();
  await expect(declaration.getByText("Declaración guardada.")).toBeVisible();
  await expect(
    declaration.getByText("11222333-9", { exact: true }),
  ).toBeVisible();
  expect(declarationRequests).toEqual(["GET", "POST", "GET", "GET"]);
  expect(declarationWrites).toBe(1);
  await page
    .getByRole("link", { name: "Iniciar verificación de identidad" })
    .click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  expect(caseCreates).toBe(0);
  expect(evidenceUploads).toBe(0);
  expect(requestsOutsideLoopback).toEqual([]);
});

test("identity declaration stays isolated across account changes and logout", async ({
  page,
}) => {
  const requestsOutsideLoopback: string[] = [];
  const accounts = {
    "owner-a@example.invalid": "550e8400-e29b-41d4-a716-446655440000",
    "owner-b@example.invalid": "660e8400-e29b-41d4-a716-446655440000",
  } as const;
  const declarations: Record<string, Record<string, unknown>> = {
    [accounts["owner-a@example.invalid"]]: {
      id: "990e8400-e29b-41d4-a716-446655440000",
      revision: 1,
      first_name: "Synthetic Account A",
      last_name: "Synthetic A Last",
      date_of_birth: "1990-01-02",
      rut: "11222333-9",
      id_country: "CHL",
      document_type: "ID",
      consent_version: identityDeclarationConsentVersion,
      consent_given_at: "2026-11-13T12:00:00Z",
    },
    [accounts["owner-b@example.invalid"]]: {
      id: "880e8400-e29b-41d4-a716-446655440000",
      revision: 1,
      first_name: "Synthetic Account B",
      last_name: "Synthetic B Last",
      date_of_birth: "1991-02-03",
      rut: "76543210-K",
      id_country: "CHL",
      document_type: "ID",
      consent_version: identityDeclarationConsentVersion,
      consent_given_at: "2026-11-13T12:00:00Z",
    },
  };
  let currentAccountId: string | null = null;
  let currentEmail: string | null = null;
  const declarationReads: Array<string | null> = [];

  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      requestsOutsideLoopback.push(request.url());
    }
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (status: number, value: unknown) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
        },
        body: JSON.stringify(value),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return currentAccountId
        ? json(200, {
            account_id: currentAccountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          })
        : json(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      const body = request.postDataJSON() as { email?: string };
      currentEmail = body.email ?? null;
      currentAccountId =
        currentEmail && currentEmail in accounts
          ? accounts[currentEmail as keyof typeof accounts]
          : null;
      if (!currentAccountId) {
        return json(401, { error: { code: "unauthenticated" } });
      }
      return json(200, {
        account_id: currentAccountId,
        roles: ["arrendador"],
        capabilities: [],
        access_token: `synthetic-token-${currentEmail}`,
        token_type: "Bearer",
        expires_at: new Date(Date.now() + 60_000).toISOString(),
        restricted_mode: false,
      });
    }
    if (pathname === "/api/backend/auth/logout" && method === "POST") {
      currentAccountId = null;
      currentEmail = null;
      return route.fulfill({ status: 204 });
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      // Record every GET, including anonymous reads after logout.
      declarationReads.push(currentAccountId);
      if (!currentAccountId) {
        return json(401, { error: { code: "unauthenticated" } });
      }
      return json(200, declarations[currentAccountId]);
    }
    if (
      (pathname === "/api/backend/verifications" ||
        pathname === "/api/backend/verifications/eligibility") &&
      method === "GET"
    ) {
      return json(200, { items: [] });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return json(200, []);
    }
    return json(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  await page.getByLabel("Correo electrónico").fill("owner-a@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  const declaration = page.getByRole("region", {
    name: "Declaración de identidad",
  });
  await expect(declaration.getByText("Synthetic Account A")).toBeVisible();
  await expect(declaration.getByText("Synthetic A Last")).toBeVisible();
  await expect(declaration.getByText("1990-01-02")).toBeVisible();
  await expect(declaration.getByText("11222333-9")).toBeVisible();
  expect(declarationReads).toEqual([accounts["owner-a@example.invalid"]]);
  await declaration.getByRole("button", { name: "Editar declaración" }).click();
  await expect(declaration.getByLabel("Nombres legales")).toHaveValue(
    "Synthetic Account A",
  );
  await declaration.getByLabel("Nombres legales").fill("Draft A only");
  await declaration.getByLabel("Apellidos legales").fill("Unsubmitted A only");
  await expect(declaration.getByLabel("Nombres legales")).toHaveValue(
    "Draft A only",
  );
  await expect(declaration.getByLabel("Apellidos legales")).toHaveValue(
    "Unsubmitted A only",
  );

  await expect(page.getByRole("link", { name: "Acceso" })).toBeVisible();
  await page.getByRole("link", { name: "Acceso" }).click();
  await expect(page).toHaveURL(/\/acceso$/);
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  await page.getByLabel("Correo electrónico").fill("owner-b@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  await expect(declaration.getByText("Synthetic Account B")).toBeVisible();
  await expect(declaration.getByText("Synthetic B Last")).toBeVisible();
  await expect(declaration.getByText("1991-02-03")).toBeVisible();
  await expect(declaration.getByText("76543210-K")).toBeVisible();
  await expect(declaration.getByText("Synthetic Account A")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic A Last")).toHaveCount(0);
  await expect(declaration.getByText("1990-01-02")).toHaveCount(0);
  await expect(declaration.getByText("11222333-9")).toHaveCount(0);
  expect(declarationReads).toEqual([
    accounts["owner-a@example.invalid"],
    accounts["owner-b@example.invalid"],
  ]);
  await declaration.getByRole("button", { name: "Editar declaración" }).click();
  await expect(declaration.getByLabel("Nombres legales")).toHaveValue(
    "Synthetic Account B",
  );
  await expect(declaration.getByLabel("Apellidos legales")).toHaveValue(
    "Synthetic B Last",
  );
  await expect(declaration.getByLabel("Fecha de nacimiento")).toHaveValue(
    "1991-02-03",
  );
  await expect(declaration.getByLabel("RUN/RUT")).toHaveValue("76543210-K");
  await expect(declaration.getByLabel("Nombres legales")).not.toHaveValue(
    "Draft A only",
  );
  await expect(declaration.getByLabel("Apellidos legales")).not.toHaveValue(
    "Unsubmitted A only",
  );
  await declaration.getByLabel("Nombres legales").fill("Draft B only");
  await declaration.getByLabel("Apellidos legales").fill("Unsubmitted B only");
  await expect(declaration.getByLabel("Nombres legales")).toHaveValue(
    "Draft B only",
  );
  await expect(declaration.getByLabel("Apellidos legales")).toHaveValue(
    "Unsubmitted B only",
  );
  expect(declarationReads).toEqual([
    accounts["owner-a@example.invalid"],
    accounts["owner-b@example.invalid"],
  ]);

  await page.getByRole("link", { name: "Acceso" }).click();
  await expect(page).toHaveURL(/\/acceso$/);
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  await page.getByLabel("Correo electrónico").fill("owner-a@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  await expect(declaration.getByText("Synthetic Account A")).toBeVisible();
  await expect(declaration.getByText("Synthetic A Last")).toBeVisible();
  await expect(declaration.getByText("1990-01-02")).toBeVisible();
  await expect(declaration.getByText("11222333-9")).toBeVisible();
  await expect(declaration.getByText("Synthetic Account B")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic B Last")).toHaveCount(0);
  await expect(declaration.getByText("1991-02-03")).toHaveCount(0);
  await expect(declaration.getByText("76543210-K")).toHaveCount(0);
  expect(declarationReads).toEqual([
    accounts["owner-a@example.invalid"],
    accounts["owner-b@example.invalid"],
    accounts["owner-a@example.invalid"],
  ]);
  await declaration.getByRole("button", { name: "Editar declaración" }).click();
  await expect(declaration.getByLabel("Nombres legales")).toHaveValue(
    "Synthetic Account A",
  );
  await expect(declaration.getByLabel("Apellidos legales")).toHaveValue(
    "Synthetic A Last",
  );
  await expect(declaration.getByLabel("Fecha de nacimiento")).toHaveValue(
    "1990-01-02",
  );
  await expect(declaration.getByLabel("RUN/RUT")).toHaveValue("11222333-9");
  await expect(declaration.getByLabel("Nombres legales")).not.toHaveValue(
    "Draft A only",
  );
  await expect(declaration.getByLabel("Apellidos legales")).not.toHaveValue(
    "Unsubmitted A only",
  );
  await expect(declaration.getByLabel("Nombres legales")).not.toHaveValue(
    "Draft B only",
  );
  await expect(declaration.getByLabel("Apellidos legales")).not.toHaveValue(
    "Unsubmitted B only",
  );

  await page.getByRole("link", { name: "Acceso" }).click();
  await expect(page).toHaveURL(/\/acceso$/);
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.goto("/mi-cuenta/verificaciones");
  await expect(declaration).toBeVisible();
  await expect(
    declaration.getByText(
      "Inicia sesión para consultar o guardar tu declaración.",
    ),
  ).toBeVisible();
  await expect(declaration.getByText("Synthetic Account A")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic Account B")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic A Last")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic B Last")).toHaveCount(0);
  await expect(declaration.getByText("1990-01-02")).toHaveCount(0);
  await expect(declaration.getByText("1991-02-03")).toHaveCount(0);
  await expect(declaration.getByText("11222333-9")).toHaveCount(0);
  await expect(declaration.getByText("76543210-K")).toHaveCount(0);
  await expect(declaration.getByLabel("Nombres legales")).toHaveCount(0);
  await expect(declaration.getByLabel("Apellidos legales")).toHaveCount(0);
  await expect(declaration.getByLabel("Fecha de nacimiento")).toHaveCount(0);
  await expect(declaration.getByLabel("RUN/RUT")).toHaveCount(0);
  await expect(
    declaration.getByLabel(/Confirmo que usaré solo datos sintéticos/),
  ).toHaveCount(0);
  await expect(declaration.locator("form")).toHaveCount(0);
  await expect(
    declaration.getByRole("button", { name: "Editar declaración" }),
  ).toHaveCount(0);
  await expect(
    declaration.getByRole("button", { name: "Guardar declaración" }),
  ).toHaveCount(0);
  await expect(
    declaration.getByRole("button", { name: "Consultar estado" }),
  ).toHaveCount(0);
  await expect(declaration.getByText("Declaración guardada.")).toHaveCount(0);
  await expect(declaration.getByText("Consultando declaración...")).toHaveCount(
    0,
  );
  expect(declarationReads).toEqual([
    accounts["owner-a@example.invalid"],
    accounts["owner-b@example.invalid"],
    accounts["owner-a@example.invalid"],
  ]);
  expect(requestsOutsideLoopback).toEqual([]);
});
