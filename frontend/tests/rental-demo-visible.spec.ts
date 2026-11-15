import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const month = "2030-01";
const dayPattern = /\b16\b.*16 de enero de 2030/i;

async function openLocalPage(
  browser: import("@playwright/test").Browser,
  origin: string,
  viewport?: { width: number; height: number },
) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  await page.route("**/*", (route) => {
    if (new URL(route.request().url()).origin !== origin)
      return route.abort("blockedbyclient");
    return route.continue();
  });
  return page;
}

async function selectJanuary16(page: import("@playwright/test").Page) {
  const day = page.getByRole("button", { name: dayPattern });
  await expect(day).toBeEnabled();
  await day.click();
  await expect(page.getByRole("heading", { name: "2030-01-16" })).toBeVisible();
}

test("visible rental demo carries a simulated confirmation across both calendars", async ({
  browser,
}, testInfo) => {
  const origin = new URL(String(testInfo.project.use.baseURL)).origin;
  const hostPage = await openLocalPage(browser, origin);
  await hostPage.goto(`/demo/arriendo?mode=host&month=${month}`);
  await expect(
    hostPage.getByRole("heading", { name: "Recorrido de arriendo (demo)" }),
  ).toBeVisible();
  await hostPage.keyboard.press("Tab");
  const skipLink = hostPage.getByRole("link", {
    name: "Omitir al contenido principal",
  });
  await expect(skipLink).toBeFocused();
  await hostPage.keyboard.press("Enter");
  await expect(hostPage.locator("#main-content")).toBeFocused();
  await hostPage
    .getByRole("button", {
      name: "Entrar como propietario de demostración",
    })
    .first()
    .click();
  await expect(
    hostPage.getByRole("heading", { name: "Configuración y bloqueos" }),
  ).toBeVisible();
  await expect(
    hostPage.getByRole("heading", { name: "enero de 2030", exact: false }),
  ).toBeVisible();

  await hostPage.getByRole("button", { name: "Mes siguiente" }).click();
  await expect(
    hostPage.getByRole("heading", { name: "febrero de 2030", exact: false }),
  ).toBeVisible();
  await hostPage.getByRole("button", { name: "Mes anterior" }).click();
  await expect(
    hostPage.getByRole("heading", { name: "enero de 2030", exact: false }),
  ).toBeVisible();
  await hostPage.getByRole("button", { name: "Mes siguiente" }).click();
  await expect(
    hostPage.getByRole("heading", { name: "febrero de 2030", exact: false }),
  ).toBeVisible();
  await hostPage.getByRole("button", { name: "Hoy" }).click();
  await expect(
    hostPage.getByRole("heading", { name: "enero de 2030", exact: false }),
  ).toBeVisible();

  const desktopDimensions = await hostPage.evaluate(() => ({
    document: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  }));
  expect(desktopDimensions.document).toBeLessThanOrEqual(
    desktopDimensions.viewport,
  );
  const { violations: desktopViolations } = await new AxeBuilder({
    page: hostPage,
  }).analyze();
  expect(desktopViolations).toEqual([]);

  const spaceSelect = hostPage.getByLabel("Espacio de demostración");
  await expect(spaceSelect).toHaveValue(/^[0-9a-f-]{36}$/i);
  const spaceId = await spaceSelect.inputValue();
  await selectJanuary16(hostPage);
  const hostSlots = hostPage.locator("button").filter({
    hasText: /^2030-01-16T/,
  });
  await expect(hostSlots.first()).toBeVisible();
  const slotStart = (await hostSlots.first().innerText()).split(" – ")[0];
  await hostPage.context().close();

  const renterPage = await openLocalPage(browser, origin, {
    width: 390,
    height: 844,
  });
  await renterPage.goto(
    `/demo/arriendo?mode=public&space=${encodeURIComponent(spaceId)}&month=${month}`,
  );
  await expect(
    renterPage.getByRole("heading", { name: "Intervalos disponibles" }),
  ).toBeVisible();
  await renterPage
    .getByRole("button", { name: "Entrar como arrendatario de demostración" })
    .click();
  await expect(renterPage.getByText("Sesión sintética: renter")).toBeVisible();

  const cookies = await renterPage
    .context()
    .cookies(`${renterPage.url().split("/demo/")[0]}/api/rental-demo/session`);
  const sessionCookie = cookies.find(
    (cookie) => cookie.name === "espacigo_rental_demo",
  );
  expect(sessionCookie).toMatchObject({ httpOnly: true, sameSite: "Strict" });

  await selectJanuary16(renterPage);
  const selectedSlot = renterPage.locator("button").filter({
    hasText: slotStart,
  });
  await expect(selectedSlot.first()).toBeVisible();
  await selectedSlot.first().click();
  await renterPage
    .getByRole("button", { name: "Consultar cotización del servidor" })
    .click();
  await expect(
    renterPage.getByRole("heading", { name: "Cotización de demostración" }),
  ).toBeVisible();

  const approve = renterPage.getByRole("checkbox", {
    name: "Aprobar contrato",
  });
  const pay = renterPage.getByRole("button", { name: "Pagar (simulado)" });
  await expect(pay).toBeDisabled();
  await approve.check();
  await expect(pay).toBeEnabled();
  await pay.click();
  await expect(
    renterPage.getByRole("status").filter({ hasText: "Pagado (simulado)" }),
  ).toBeVisible();
  await expect(
    renterPage.getByText("Sin cobros reales ni reserva comercial."),
  ).toBeVisible();
  const dimensions = await renterPage.evaluate(() => ({
    document: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
  const { violations } = await new AxeBuilder({ page: renterPage }).analyze();
  expect(violations).toEqual([]);
  const screenshotPath = testInfo.outputPath(
    "rental-demo-mobile-confirmation.png",
  );
  await renterPage.screenshot({ path: screenshotPath, fullPage: true });
  await testInfo.attach("rental-demo-mobile-confirmation", {
    path: screenshotPath,
    contentType: "image/png",
  });
  await expect(
    renterPage.locator("button").filter({ hasText: slotStart }),
  ).toHaveCount(0);
  await renterPage
    .getByRole("button", { name: "Cerrar sesión de demostración" })
    .click();
  await expect(
    renterPage.getByRole("button", {
      name: "Entrar como arrendatario de demostración",
    }),
  ).toBeVisible();
  await expect(renterPage.getByText("Sesión sintética: renter")).toHaveCount(0);
  const cookiesAfterLogout = await renterPage
    .context()
    .cookies(`${origin}/api/rental-demo/session`);
  expect(
    cookiesAfterLogout.find((cookie) => cookie.name === "espacigo_rental_demo"),
  ).toBeUndefined();
  await renterPage.context().close();

  const ownerAfterPage = await openLocalPage(browser, origin);
  await ownerAfterPage.goto(
    `/demo/arriendo?mode=host&space=${spaceId}&month=${month}`,
  );
  await ownerAfterPage
    .getByRole("button", {
      name: "Entrar como propietario de demostración",
    })
    .first()
    .click();
  await expect(
    ownerAfterPage.getByRole("heading", { name: "Configuración y bloqueos" }),
  ).toBeVisible();
  const ownerSpaceSelect = ownerAfterPage.getByLabel("Espacio de demostración");
  await expect(ownerSpaceSelect.locator("option").nth(2)).toBeAttached();
  await selectJanuary16(ownerAfterPage);
  await expect(
    ownerAfterPage.locator("button").filter({ hasText: slotStart }),
  ).toHaveCount(0);

  await ownerAfterPage.locator("#demo-block-start").fill("2030-01-16T10:00");
  await ownerAfterPage.locator("#demo-block-end").fill("2030-01-16T11:00");
  let releaseCreateBlock!: () => void;
  let notifyCreateBlockForwarded!: () => void;
  const createBlockForwarded = new Promise<void>((resolve) => {
    notifyCreateBlockForwarded = resolve;
  });
  const holdCreateBlockResponse = new Promise<void>((resolve) => {
    releaseCreateBlock = resolve;
  });
  let createBlockStatus = 0;
  await ownerAfterPage.route(
    "**/api/rental-demo/owner/spaces/*/blocks",
    async (route) => {
      if (new URL(route.request().url()).origin !== origin)
        return route.abort("blockedbyclient");
      if (route.request().method() !== "POST") return route.fallback();
      const response = await route.fetch();
      createBlockStatus = response.status();
      notifyCreateBlockForwarded();
      await holdCreateBlockResponse;
      await route.fulfill({ response });
    },
  );
  await ownerAfterPage
    .getByRole("button", { name: "Bloquear rango exacto" })
    .click();
  await createBlockForwarded;
  expect(createBlockStatus).toBe(201);

  const otherSpaceId = await ownerSpaceSelect
    .locator("option")
    .evaluateAll(
      (options, currentSpaceId) =>
        (options as HTMLOptionElement[]).find(
          (option) => option.value && option.value !== currentSpaceId,
        )?.value ?? "",
      spaceId,
    );
  expect(otherSpaceId).not.toBe("");
  const otherSpaceMonth = ownerAfterPage.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.origin === origin &&
      url.pathname === `/api/rental-demo/owner/spaces/${otherSpaceId}/month` &&
      response.status() === 200
    );
  });
  await ownerSpaceSelect.selectOption(otherSpaceId);
  await otherSpaceMonth;

  let staleMonthReadbacks = 0;
  ownerAfterPage.on("request", (request) => {
    const url = new URL(request.url());
    if (
      url.origin === origin &&
      url.pathname === `/api/rental-demo/owner/spaces/${spaceId}/month` &&
      request.method() === "GET"
    ) {
      staleMonthReadbacks++;
    }
  });
  const createBlockResponse = ownerAfterPage.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.origin === origin &&
      url.pathname === `/api/rental-demo/owner/spaces/${spaceId}/blocks` &&
      response.request().method() === "POST" &&
      response.status() === 201
    );
  });
  releaseCreateBlock();
  await createBlockResponse;
  await ownerAfterPage.waitForLoadState("networkidle");
  expect(staleMonthReadbacks).toBe(0);
  const blockText = "2030-01-16T13:00:00Z — 2030-01-16T14:00:00Z";
  await expect(
    ownerAfterPage.getByText(blockText, { exact: true }),
  ).toHaveCount(0);

  const originalSpaceMonth = ownerAfterPage.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.origin === origin &&
      url.pathname === `/api/rental-demo/owner/spaces/${spaceId}/month` &&
      response.status() === 200
    );
  });
  await ownerSpaceSelect.selectOption(spaceId);
  await originalSpaceMonth;
  await expect(
    ownerAfterPage.getByText(blockText, { exact: true }),
  ).toBeVisible();

  let releaseDeleteBlock!: () => void;
  let notifyDeleteBlockForwarded!: () => void;
  const deleteBlockForwarded = new Promise<void>((resolve) => {
    notifyDeleteBlockForwarded = resolve;
  });
  const holdDeleteBlockResponse = new Promise<void>((resolve) => {
    releaseDeleteBlock = resolve;
  });
  await ownerAfterPage.route(
    `**/api/rental-demo/owner/spaces/${spaceId}/blocks/*`,
    async (route) => {
      if (new URL(route.request().url()).origin !== origin)
        return route.abort("blockedbyclient");
      if (route.request().method() !== "DELETE") return route.fallback();
      const response = await route.fetch();
      notifyDeleteBlockForwarded();
      await holdDeleteBlockResponse;
      await route.fulfill({ response });
    },
  );
  staleMonthReadbacks = 0;
  const deleteBlockResponse = ownerAfterPage.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.origin === origin &&
      url.pathname.startsWith(
        `/api/rental-demo/owner/spaces/${spaceId}/blocks/`,
      ) &&
      response.request().method() === "DELETE" &&
      response.ok()
    );
  });
  await ownerAfterPage.getByRole("button", { name: "Retirar bloqueo" }).click();
  await deleteBlockForwarded;
  const otherSpaceAfterDeleteMonth = ownerAfterPage.waitForResponse(
    (response) => {
      const url = new URL(response.url());
      return (
        url.origin === origin &&
        url.pathname ===
          `/api/rental-demo/owner/spaces/${otherSpaceId}/month` &&
        response.status() === 200
      );
    },
  );
  await ownerSpaceSelect.selectOption(otherSpaceId);
  await otherSpaceAfterDeleteMonth;
  releaseDeleteBlock();
  await deleteBlockResponse;
  await ownerAfterPage.waitForLoadState("networkidle");
  expect(staleMonthReadbacks).toBe(0);
  await expect(
    ownerAfterPage.getByText(blockText, { exact: true }),
  ).toHaveCount(0);
  const originalSpaceAfterDeleteMonth = ownerAfterPage.waitForResponse(
    (response) => {
      const url = new URL(response.url());
      return (
        url.origin === origin &&
        url.pathname === `/api/rental-demo/owner/spaces/${spaceId}/month` &&
        response.status() === 200
      );
    },
  );
  await ownerSpaceSelect.selectOption(spaceId);
  await originalSpaceAfterDeleteMonth;
  await selectJanuary16(ownerAfterPage);
  await expect(
    ownerAfterPage.getByText(blockText, { exact: true }),
  ).toHaveCount(0);
  await ownerAfterPage
    .getByRole("button", { name: "Cerrar sesión de demostración" })
    .click();
  await expect(
    ownerAfterPage.getByRole("button", {
      name: "Entrar como propietario de demostración",
    }),
  ).toBeVisible();
  await expect(
    ownerAfterPage.getByRole("heading", { name: "2030-01-16" }),
  ).toHaveCount(0);
  const ownerCookiesAfterLogout = await ownerAfterPage
    .context()
    .cookies(`${origin}/api/rental-demo/session`);
  expect(
    ownerCookiesAfterLogout.find(
      (cookie) => cookie.name === "espacigo_rental_demo",
    ),
  ).toBeUndefined();
  await ownerAfterPage
    .getByRole("button", {
      name: "Entrar como propietario de demostración",
    })
    .first()
    .click();
  await expect(
    ownerSpaceSelect.locator(`option[value="${spaceId}"]`),
  ).toBeAttached();
  await expect(ownerSpaceSelect).toHaveValue(spaceId);
  await expect(
    ownerAfterPage.getByRole("heading", {
      name: "enero de 2030",
      exact: false,
    }),
  ).toBeVisible();

  let releaseConfiguration!: () => void;
  let notifyConfigurationForwarded!: () => void;
  const configurationForwarded = new Promise<void>((resolve) => {
    notifyConfigurationForwarded = resolve;
  });
  const holdConfigurationResponse = new Promise<void>((resolve) => {
    releaseConfiguration = resolve;
  });
  let configurationStatus = 0;
  let ownerSpacesReadbacks = 0;
  ownerAfterPage.on("request", (request) => {
    const url = new URL(request.url());
    if (
      url.origin === origin &&
      url.pathname === "/api/rental-demo/owner/spaces" &&
      request.method() === "GET"
    ) {
      ownerSpacesReadbacks++;
    }
  });
  await ownerAfterPage.route(
    `**/api/rental-demo/owner/spaces/${spaceId}`,
    async (route) => {
      if (new URL(route.request().url()).origin !== origin)
        return route.abort("blockedbyclient");
      if (route.request().method() !== "PUT") return route.fallback();
      const response = await route.fetch();
      configurationStatus = response.status();
      notifyConfigurationForwarded();
      await holdConfigurationResponse;
      await route.fulfill({ response });
    },
  );
  staleMonthReadbacks = 0;
  await ownerAfterPage.locator("#demo-space-price").fill("150001");
  const configurationResponse = ownerAfterPage.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.origin === origin &&
      url.pathname === `/api/rental-demo/owner/spaces/${spaceId}` &&
      response.request().method() === "PUT" &&
      response.ok()
    );
  });
  await ownerAfterPage
    .getByRole("button", { name: "Guardar configuración" })
    .click();
  await configurationForwarded;
  const otherSpaceAfterConfigurationMonth = ownerAfterPage.waitForResponse(
    (response) => {
      const url = new URL(response.url());
      return (
        url.origin === origin &&
        url.pathname ===
          `/api/rental-demo/owner/spaces/${otherSpaceId}/month` &&
        response.status() === 200
      );
    },
  );
  await ownerSpaceSelect.selectOption(otherSpaceId);
  await otherSpaceAfterConfigurationMonth;
  await ownerAfterPage.waitForLoadState("networkidle");
  const spacesReadbacksBeforeRelease = ownerSpacesReadbacks;
  releaseConfiguration();
  await configurationResponse;
  expect(configurationStatus).toBe(200);
  await ownerAfterPage.waitForLoadState("networkidle");
  expect(staleMonthReadbacks).toBe(0);
  expect(ownerSpacesReadbacks).toBe(spacesReadbacksBeforeRelease);

  const spacesReadbacksBeforeReturn = ownerSpacesReadbacks;
  const ownerSpacesRequestAfterReturn = ownerAfterPage.waitForRequest(
    (request) => {
      const url = new URL(request.url());
      return (
        url.origin === origin &&
        url.pathname === "/api/rental-demo/owner/spaces" &&
        request.method() === "GET"
      );
    },
  );
  await ownerSpaceSelect.selectOption(spaceId);
  await expect(ownerSpaceSelect).toHaveValue(spaceId);
  const persistedSpacesRequest = await ownerSpacesRequestAfterReturn;
  const persistedSpacesResponse = await persistedSpacesRequest.response();
  if (!persistedSpacesResponse) {
    throw new Error("owner-spaces readback had no HTTP response");
  }
  expect(persistedSpacesResponse.status()).toBe(200);
  expect(ownerSpacesReadbacks).toBe(spacesReadbacksBeforeReturn + 1);
  const persistedSpaces = (await persistedSpacesResponse.json()) as Array<{
    id: string;
    unit_price_clp: string;
  }>;
  expect(
    persistedSpaces.find((space) => space.id === spaceId)?.unit_price_clp,
  ).toBe("150001");
  await ownerAfterPage.context().close();
});

test("context changes cannot re-enable session controls while login is pending", async ({
  browser,
}, testInfo) => {
  const origin = new URL(String(testInfo.project.use.baseURL)).origin;
  const page = await openLocalPage(browser, origin);
  await page.goto(`/demo/arriendo?mode=public&month=${month}`);
  await page
    .getByRole("button", { name: "Entrar como arrendatario de demostración" })
    .click();
  await expect(page.getByText("Sesión sintética: renter")).toBeVisible();

  let releaseLogin!: () => void;
  let notifyLoginCaptured!: () => void;
  const loginCaptured = new Promise<void>((resolve) => {
    notifyLoginCaptured = resolve;
  });
  const holdLogin = new Promise<void>((resolve) => {
    releaseLogin = resolve;
  });
  await page.route("**/api/rental-demo/session", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    notifyLoginCaptured();
    await holdLogin;
    const response = await route.fetch();
    await route.fulfill({ response });
  });
  const pendingLoginResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).origin === origin &&
      new URL(response.url()).pathname === "/api/rental-demo/session" &&
      response.request().method() === "POST" &&
      response.ok(),
  );
  await page
    .getByRole("button", {
      name: "Entrar como segundo arrendatario de demostración",
    })
    .click();
  await loginCaptured;

  await page.getByLabel("Espacio de demostración").selectOption("");
  const logoutButton = page.getByRole("button", {
    name: "Cerrar sesión de demostración",
  });
  const logoutRemainsDisabled = await logoutButton.isDisabled();
  releaseLogin();
  await pendingLoginResponse;
  expect(logoutRemainsDisabled).toBe(true);
  await expect(page.getByText("Sesión sintética: renter2")).toBeVisible();

  const logoutResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).origin === origin &&
      new URL(response.url()).pathname === "/api/rental-demo/session" &&
      response.request().method() === "DELETE" &&
      response.status() === 204,
  );
  await logoutButton.click();
  await logoutResponse;
  await expect(
    page.getByRole("button", {
      name: "Entrar como arrendatario de demostración",
    }),
  ).toBeVisible();

  const cookies = await page
    .context()
    .cookies(`${origin}/api/rental-demo/session`);
  expect(
    cookies.find((cookie) => cookie.name === "espacigo_rental_demo"),
  ).toBeUndefined();
  await expect(page.getByText("Sesión sintética: renter2")).toHaveCount(0);
  await page.context().close();
});
