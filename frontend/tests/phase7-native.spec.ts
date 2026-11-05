import { expect, test } from "@playwright/test";
import { chmod, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const nativeBaseURL =
  process.env.PHASE7_NATIVE_BASE_URL ??
  `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;

test.use({
  trace: "off",
  screenshot: "off",
  video: "off",
  baseURL: nativeBaseURL,
  ignoreHTTPSErrors: nativeBaseURL.startsWith("https://"),
});
test.describe.configure({ mode: "serial" });

const enabled = process.env.PHASE7_NATIVE_API_E2E === "1";
let nativeSessionNeedsLogout = false;

function privatePath(name: string): string {
  return path.resolve(process.cwd(), "../.local/testing", name);
}

test.skip(
  !enabled,
  "Requires the explicitly configured current-HEAD API and native PostgreSQL runtime",
);

test.afterEach(async ({ page }) => {
  if (!nativeSessionNeedsLogout) return;
  const button = page.getByRole("button", { name: "Cerrar sesión" });
  if (await button.isVisible().catch(() => false)) {
    const responsePromise = page
      .waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/backend/auth/logout",
        { timeout: 5_000 },
      )
      .catch(() => null);
    const [, response] = await Promise.all([
      button.click({ noWaitAfter: true }).catch(() => undefined),
      responsePromise,
    ]);
    if (response?.status() === 204) {
      nativeSessionNeedsLogout = false;
      return;
    }
  }
  const fallbackStatus = await page
    .evaluate(async () => {
      const response = await fetch("/api/backend/auth/logout", {
        method: "POST",
      });
      return response.status;
    })
    .catch(() => 0);
  if (fallbackStatus === 204) nativeSessionNeedsLogout = false;
});

test("@phase7 @native authorized synthetic accounts verify persisted listing CRUD, rehydration, ownership isolation and clean logout against local Go", async ({
  page,
  browser,
}) => {
  test.setTimeout(60_000);
  let policy: {
    account_policy?: {
      never_print_credentials_or_tokens?: boolean;
      password?: string;
    };
  };
  let inventory: {
    accounts?: Array<{
      email?: string;
      accountId?: string;
      password?: string;
      status?: string;
    }>;
  };
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
  const account = inventory.accounts?.[0];
  const otherAccount = inventory.accounts?.[1];
  if (
    credentials?.never_print_credentials_or_tokens !== true ||
    typeof account?.password !== "string" ||
    typeof account.email !== "string" ||
    account.status !== "verified" ||
    !account.email.endsWith("@example.invalid") ||
    typeof otherAccount?.password !== "string" ||
    typeof otherAccount.email !== "string" ||
    otherAccount.status !== "verified" ||
    !otherAccount.email.endsWith("@example.invalid")
  ) {
    test.skip(
      true,
      "Private policy does not authorize this existing-account test",
    );
    return;
  }

  const secrets = [
    account.email,
    account.password,
    otherAccount.email,
    otherAccount.password,
  ];
  let sessionConfirmed = false;
  let logoutConfirmed = false;
  let secretInConsole = false;
  let spacesQueried = false;
  let categoriesQueried = false;
  const nonLoopback = { value: false };

  const checkConsoleForSecrets = (message: { text(): string }) => {
    const text = message.text();
    if (
      secrets.some((secret) => text.includes(secret)) ||
      /\b(?:bearer\s+\S+|access[_ -]?token|authorization)\b/i.test(text)
    ) {
      secretInConsole = true;
    }
  };

  page.on("request", (request) => {
    if (new URL(request.url()).origin !== new URL(nativeBaseURL).origin)
      nonLoopback.value = true;
    const urlPath = new URL(request.url()).pathname;
    if (urlPath === "/api/backend/spaces") {
      spacesQueried = true;
    }
    if (urlPath === "/api/backend/spaces/categories") {
      categoriesQueried = true;
    }
  });

  page.on("console", checkConsoleForSecrets);

  // 1. Login con cuenta sintética autorizada redirigiendo a /anfitrion/espacios
  await page.goto("/acceso?return_to=%2Fanfitrion%2Fespacios");
  const email = page.getByLabel("Correo electrónico");
  const password = page.getByLabel("Contraseña", { exact: true });

  try {
    const loginResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login",
      { timeout: 15_000 },
    );
    await email.fill(account.email);
    await password.fill(account.password);
    const loginResponse = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      loginResponsePromise,
    ]).then(([, response]) => response);
    expect(loginResponse.status()).toBe(200);
    nativeSessionNeedsLogout = true;
    await expect(page).toHaveURL(/\/anfitrion\/espacios$/);
    sessionConfirmed = true;
  } catch {
    await email.fill("", { timeout: 500 }).catch(() => undefined);
    await password.fill("", { timeout: 500 }).catch(() => undefined);
    throw new Error(
      "Authorized synthetic login did not reach the allowlisted destination /anfitrion/espacios",
    );
  }
  expect(sessionConfirmed).toBe(true);

  // 2. Consulta real de espacios en /anfitrion/espacios
  await expect(page.locator("h1")).toHaveText("Mis Espacios");
  expect(spacesQueried).toBe(true);

  // 3. Navegación a /anfitrion/espacios/nuevo y consulta de categorías
  await page.getByRole("link", { name: "Publicar un espacio" }).first().click();
  await expect(page).toHaveURL(/\/anfitrion\/espacios\/nuevo$/);
  await expect(page.locator("h1")).toHaveText("Publicar un nuevo espacio");
  expect(categoriesQueried).toBe(true);
  await expect(page.locator("#wizard-category")).toBeVisible();

  // 4. Crear un borrador con el perfil Go real y confirmar el refresco de rol.
  const title = "Espacio sintético F3 persistido";
  const description =
    "Espacio sintético para comprobar creación, lectura y actualización persistida del flujo F3 en una base local aislada.";
  await page.locator("#wizard-category").selectOption("oficina");
  await page.locator("#wizard-title").fill(title);
  await page.locator("#wizard-description").fill(description);
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();
  await expect(page.locator("#attr-puestos_trabajo")).toBeVisible();
  await page.locator("#wizard-address").fill("Dirección sintética F3 123");
  await page.locator("#wizard-comuna").fill("13123");
  await page.locator("#wizard-region").selectOption("13");
  await page.locator("#wizard-area").fill("35");
  await page.locator("#wizard-capacity").fill("6");
  await page.locator("#attr-puestos_trabajo").fill("6");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 3: Tarifas iniciales y condiciones"),
  ).toBeVisible();
  await page.locator("#wizard-price").fill("15000");
  await page
    .locator("#wizard-usage-rules")
    .fill("Uso sintético para verificar persistencia local.");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 4: Disponibilidad semanal y zona horaria"),
  ).toBeVisible();
  await page.locator("#wizard-timezone").fill("America/Santiago");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 5: Resumen y confirmación de alta"),
  ).toBeVisible();

  const createResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/backend/spaces",
    { timeout: 30_000 },
  );
  const [, createResponse] = await Promise.all([
    page
      .getByRole("button", { name: "Crear espacio y continuar a edición" })
      .click({ noWaitAfter: true }),
    createResponsePromise,
  ]);
  const createPayload = (await createResponse.json().catch(() => null)) as {
    error?: { code?: unknown; details?: unknown };
  } | null;
  const createErrorCode =
    typeof createPayload?.error?.code === "string" &&
    /^[a-z0-9_]{1,48}$/.test(createPayload.error.code)
      ? createPayload.error.code
      : "unknown";
  const createValidationFields = Array.isArray(createPayload?.error?.details)
    ? createPayload.error.details.flatMap((detail) => {
        if (typeof detail !== "object" || detail === null) return [];
        const field = "field" in detail ? detail.field : undefined;
        const code = "code" in detail ? detail.code : undefined;
        return typeof field === "string" &&
          /^[a-z0-9_]{1,64}$/.test(field) &&
          typeof code === "string" &&
          /^[a-z0-9_]{1,64}$/.test(code)
          ? [`${field}:${code}`]
          : [];
      })
    : [];
  expect(
    createResponse.status(),
    `POST /api/backend/spaces returned ${createResponse.status()} (${createErrorCode}; ${createValidationFields.join(",")})`,
  ).toBe(201);
  const idempotencyKey = (await createResponse.request().allHeaders())[
    "idempotency-key"
  ];
  expect(idempotencyKey).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  await page.waitForURL((url) =>
    /\/anfitrion\/espacios\/[0-9a-f-]+\/editar$/i.test(url.pathname),
  );
  const spaceId = new URL(page.url()).pathname.split("/")[3];
  expect(spaceId).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  const privateSpaceIdPath = privatePath("f6-runtime/f3-final-space-id.txt");
  await writeFile(privateSpaceIdPath, spaceId, { mode: 0o600 });
  await chmod(privateSpaceIdPath, 0o600);
  await expect(page.locator("#editor-title")).toHaveValue(title);
  await expect(page.locator("#attr-editor-puestos_trabajo")).toHaveValue("6");

  // 4a. La segunda cuenta autorizada no puede leer ni listar el borrador ajeno.
  const isolationContext = await browser.newContext({
    baseURL: nativeBaseURL,
    ignoreHTTPSErrors: nativeBaseURL.startsWith("https://"),
  });
  const isolationPage = await isolationContext.newPage();
  isolationPage.on("request", (request) => {
    if (new URL(request.url()).origin !== new URL(nativeBaseURL).origin)
      nonLoopback.value = true;
  });
  isolationPage.on("console", checkConsoleForSecrets);
  let isolationLogoutConfirmed = false;
  let isolationSessionNeedsLogout = false;
  try {
    await isolationPage.goto("/acceso?return_to=%2Fanfitrion%2Fespacios");
    const isolationLoginResponsePromise = isolationPage.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login",
      { timeout: 15_000 },
    );
    await isolationPage
      .getByLabel("Correo electrónico")
      .fill(otherAccount.email);
    await isolationPage
      .getByLabel("Contraseña", { exact: true })
      .fill(otherAccount.password);
    const isolationLoginResponse = await Promise.all([
      isolationPage
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      isolationLoginResponsePromise,
    ]).then(([, response]) => response);
    expect(isolationLoginResponse.status()).toBe(200);
    isolationSessionNeedsLogout = true;
    await expect(isolationPage).toHaveURL(/\/anfitrion\/espacios$/);
    await expect(isolationPage.locator("h1")).toHaveText("Mis Espacios");
    const foreignDetailStatus = await isolationPage.evaluate(async (id) => {
      const response = await fetch(`/api/backend/spaces/${id}`, {
        cache: "no-store",
      });
      return response.status;
    }, spaceId);
    expect(foreignDetailStatus).toBe(404);
    await expect(isolationPage.getByText(title, { exact: true })).toHaveCount(
      0,
    );
    const isolationSessionStatus = await isolationPage.evaluate(async () => {
      const response = await fetch("/api/backend/auth/session", {
        cache: "no-store",
      });
      return response.status;
    });
    expect(isolationSessionStatus).toBe(200);

    const [isolationLogout] = await Promise.all([
      isolationPage.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/backend/auth/logout",
        { timeout: 15_000 },
      ),
      isolationPage
        .getByRole("button", { name: "Cerrar sesión" })
        .click({ noWaitAfter: true }),
    ]);
    expect(isolationLogout.status()).toBe(204);
    isolationLogoutConfirmed = true;
    isolationSessionNeedsLogout = false;
  } finally {
    if (!isolationLogoutConfirmed && isolationSessionNeedsLogout) {
      const logoutButton = isolationPage.getByRole("button", {
        name: "Cerrar sesión",
      });
      if (await logoutButton.isVisible().catch(() => false)) {
        const responsePromise = isolationPage
          .waitForResponse(
            (response) =>
              new URL(response.url()).pathname === "/api/backend/auth/logout",
            { timeout: 5_000 },
          )
          .catch(() => null);
        const [, response] = await Promise.all([
          logoutButton.click({ noWaitAfter: true }).catch(() => undefined),
          responsePromise,
        ]);
        isolationLogoutConfirmed = response?.status() === 204;
      }
      if (!isolationLogoutConfirmed) {
        const fallbackStatus = await isolationPage
          .evaluate(async () => {
            const response = await fetch("/api/backend/auth/logout", {
              method: "POST",
            });
            return response.status;
          })
          .catch(() => 0);
        isolationLogoutConfirmed = fallbackStatus === 204;
      }
    }
    await isolationContext.close();
  }
  expect(isolationLogoutConfirmed).toBe(true);

  const refreshedSession = await page.evaluate(async () => {
    const response = await fetch("/api/backend/auth/session", {
      cache: "no-store",
    });
    const payload = (await response.json().catch(() => null)) as {
      roles?: unknown;
    } | null;
    return { status: response.status, roles: payload?.roles };
  });
  expect(refreshedSession.status).toBe(200);
  expect(refreshedSession.roles).toContain("arrendador");

  // 5. Rehidratación después de reload y actualización optimista persistida.
  const detailReloadPromise = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname === `/api/backend/spaces/${spaceId}`,
    { timeout: 30_000 },
  );
  await page.reload();
  const detailReload = await detailReloadPromise;
  expect(detailReload.status()).toBe(200);
  await expect(page.locator("#editor-title")).toHaveValue(title);

  const updatedTitle = "Espacio sintético F3 actualizado";
  await page.locator("#editor-title").fill(updatedTitle);
  await page.locator("#attr-editor-puestos_trabajo").fill("8");
  const staleEtag = detailReload.headers().etag;
  expect(staleEtag).toBeTruthy();
  const contentUpdatePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname === `/api/backend/spaces/${spaceId}`,
    { timeout: 30_000 },
  );
  const contentUpdate = await Promise.all([
    page.getByRole("button", { name: "Guardar contenido" }).click(),
    contentUpdatePromise,
  ]).then(([, response]) => response);
  expect(contentUpdate.status()).toBe(200);
  expect(contentUpdate.headers().etag).toBeTruthy();

  const stalePayload = contentUpdate.request().postData();
  expect(stalePayload).toBeTruthy();
  const staleWrite = await page.evaluate(
    async ({ id, etag, body }) => {
      const response = await fetch(`/api/backend/spaces/${id}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          "If-Match": etag,
        },
        body,
      });
      return response.status;
    },
    { id: spaceId, etag: staleEtag!, body: stalePayload! },
  );
  expect(staleWrite).toBe(412);

  const updatedDetailPromise = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname === `/api/backend/spaces/${spaceId}`,
    { timeout: 30_000 },
  );
  await page.reload();
  const updatedDetail = await updatedDetailPromise;
  expect(updatedDetail.status()).toBe(200);
  await expect(page.locator("#editor-title")).toHaveValue(updatedTitle);
  await expect(page.locator("#attr-editor-puestos_trabajo")).toHaveValue("8");

  await expect(
    page.getByRole("button", { name: "Eliminar", exact: true }),
  ).toHaveCount(0);
  const draftRetirementStatus = await page.evaluate(async (id) => {
    const response = await fetch(`/api/backend/spaces/${id}`, {
      method: "DELETE",
    });
    return response.status;
  }, spaceId);
  expect(draftRetirementStatus).toBe(409);
  const preservedDraft = await page.evaluate(async (id) => {
    const response = await fetch(`/api/backend/spaces/${id}`, {
      cache: "no-store",
    });
    const payload = (await response.json().catch(() => null)) as {
      title?: unknown;
    } | null;
    return { status: response.status, title: payload?.title };
  }, spaceId);
  expect(preservedDraft).toEqual({ status: 200, title: updatedTitle });

  const publishResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname ===
        `/api/backend/spaces/${spaceId}/publication`,
    { timeout: 30_000 },
  );
  const [, publishResponse] = await Promise.all([
    page.getByRole("button", { name: "Publicar", exact: true }).click(),
    publishResponsePromise,
  ]);
  const publishPayload = (await publishResponse.json().catch(() => null)) as {
    state?: unknown;
    error?: { code?: unknown };
  } | null;
  const publishErrorCode =
    typeof publishPayload?.error?.code === "string" &&
    /^[a-z0-9_]{1,48}$/.test(publishPayload.error.code)
      ? publishPayload.error.code
      : "unknown";
  expect(
    publishResponse.status(),
    `PUT /api/backend/spaces/${spaceId}/publication returned ${publishResponse.status()} (${publishErrorCode})`,
  ).toBe(200);
  expect(publishResponse.request().headers()["if-match"]).toBeTruthy();
  expect(publishResponse.headers().etag).toBeTruthy();
  expect(publishPayload?.state).toBe("activa");
  await expect(
    page.getByRole("button", { name: "Retirar publicación", exact: true }),
  ).toBeVisible();

  let retireConfirmation = "";
  page.once("dialog", async (dialog) => {
    retireConfirmation = dialog.message();
    await dialog.accept();
  });
  const retireResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname === `/api/backend/spaces/${spaceId}`,
    { timeout: 30_000 },
  );
  const listReloadPromise = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname === "/api/backend/spaces",
    { timeout: 30_000 },
  );
  const [, retireResponse] = await Promise.all([
    page
      .getByRole("button", { name: "Retirar publicación", exact: true })
      .click(),
    retireResponsePromise,
  ]);
  expect(retireResponse.status()).toBe(200);
  expect(retireConfirmation).toContain(updatedTitle);
  expect(retireConfirmation).toMatch(/reservas e historiales se conservarán/i);
  expect(retireConfirmation).toMatch(/no elimina el espacio físicamente/i);
  const retirementReceipt = (await retireResponse.json()) as {
    space_id?: unknown;
    state?: unknown;
  };
  expect(retirementReceipt.space_id).toBe(spaceId);
  expect(retirementReceipt.state).toBe("eliminada");
  await expect(page).toHaveURL(/\/anfitrion\/espacios$/);
  expect((await listReloadPromise).status()).toBe(200);
  await expect(
    page.locator(`a[href="/anfitrion/espacios/${spaceId}/editar"]`),
  ).toHaveCount(0);
  const retiredDetailStatus = await page.evaluate(async (id) => {
    const response = await fetch(`/api/backend/spaces/${id}`, {
      cache: "no-store",
    });
    return response.status;
  }, spaceId);
  expect(retiredDetailStatus).toBe(404);

  // 6. Cierre de sesión limpio desde el shell autenticado
  const [logout] = await Promise.all([
    page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/logout",
      { timeout: 15_000 },
    ),
    page
      .getByRole("button", { name: "Cerrar sesión" })
      .click({ noWaitAfter: true }),
  ]);
  expect(logout.status()).toBe(204);
  logoutConfirmed = true;
  nativeSessionNeedsLogout = false;
  await expect(page).toHaveURL(/\/acceso$/);
  await expect(page.getByLabel("Correo electrónico")).toBeVisible();

  expect(logoutConfirmed).toBe(true);
  expect(nonLoopback.value).toBe(false);
  expect(secretInConsole).toBe(false);
});
