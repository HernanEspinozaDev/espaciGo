import { expect, test } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";
import categoryProfiles from "./fixtures/category-profiles-v1.json";

test.use({ trace: "off", screenshot: "off", video: "off" });

test("@phase7 @controlled no enlaza el borrador parcial al editor tras cambiar de cuenta", async ({
  page,
}) => {
  const bff = createBffFixtureState();
  const originalAccountId = "770e8400-e29b-41d4-a716-446655440000";
  const nextAccountId = "880e8400-e29b-41d4-a716-446655440001";
  const spaceId = "00000000-0000-4000-8000-000000000122";
  const profile = categoryProfiles.profiles.inmuebles;
  let createCount = 0;
  let sessionReadsAfterCreate = 0;
  let availabilityCount = 0;
  let weeklyHoursCount = 0;

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    let result: {
      status: number;
      body?: unknown;
      headers?: Record<string, string>;
    };

    if (path === "/api/backend/auth/login" && request.method() === "POST") {
      result = {
        status: 200,
        body: {
          account_id: originalAccountId,
          roles: [],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600000).toISOString(),
          restricted_mode: false,
        },
      };
    } else if (
      path === "/api/backend/auth/session" &&
      request.method() === "GET"
    ) {
      if (createCount > 0) sessionReadsAfterCreate += 1;
      result = {
        status: 200,
        body: {
          account_id: createCount > 0 ? nextAccountId : originalAccountId,
          roles: [],
          capabilities: [],
          restricted_mode: false,
        },
      };
    } else if (
      path === "/api/backend/spaces/categories" &&
      request.method() === "GET"
    ) {
      result = {
        status: 200,
        body: { items: [{ code: "inmuebles", name: "Inmuebles" }] },
      };
    } else if (
      /^\/api\/backend\/spaces\/categories\/inmuebles\/attributes(?:\/1)?$/.test(
        path,
      ) &&
      request.method() === "GET"
    ) {
      result = { status: 200, body: profile };
    } else if (path === "/api/backend/spaces" && request.method() === "GET") {
      result = { status: 200, body: { items: [] } };
    } else if (path === "/api/backend/spaces" && request.method() === "POST") {
      createCount += 1;
      const payload = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      result = {
        status: 201,
        headers: { etag: '"draft-v1"' },
        body: {
          id: spaceId,
          category_code: "inmuebles",
          category_name: "Inmuebles",
          state: "borrador",
          ...payload,
        },
      };
    } else if (
      path === `/api/backend/spaces/${spaceId}/availability` &&
      request.method() === "PUT"
    ) {
      availabilityCount += 1;
      result = {
        status: 200,
        headers: { etag: '"availability-v2"' },
        body: { space_id: spaceId, time_zone: "America/Santiago" },
      };
    } else if (
      path === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      request.method() === "PUT"
    ) {
      weeklyHoursCount += 1;
      result = {
        status: 503,
        body: {
          error: {
            code: "fixture_unavailable",
            message: "Controlled weekly-hours failure",
          },
        },
      };
    } else {
      result = {
        status: 404,
        body: { error: { code: "not_found", message: "Not found" } },
      };
    }

    const adapted = bff.adapt(request, result);
    if (!adapted) {
      throw new Error(
        `Missing controlled response for ${request.method()} ${path}`,
      );
    }
    await route.fulfill({
      status: adapted.status,
      headers: {
        "content-type": "application/json; charset=utf-8",
        ...(adapted.headers ?? {}),
      },
      body: JSON.stringify(adapted.body ?? {}),
    });
  });

  await page.goto(
    `/acceso?return_to=${encodeURIComponent("/anfitrion/espacios/nuevo")}`,
  );
  await page
    .getByLabel("Correo electrónico")
    .fill("wizard-scope-test@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === "/anfitrion/espacios/nuevo");

  await page.locator("#wizard-category").selectOption("inmuebles");
  await page
    .locator("#wizard-title")
    .fill("Borrador sintético con cambio de cuenta");
  await page
    .locator("#wizard-description")
    .fill(
      "Descripción sintética suficientemente extensa para validar la recuperación segura del borrador cuando cambia el ámbito de cuenta.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.locator("#wizard-region").selectOption("13");
  await page.locator("#wizard-comuna").selectOption("13101");
  await page
    .locator("#wizard-address")
    .fill("Dirección sintética 122, Santiago");
  await page.locator("#wizard-area").fill("40");
  await page.locator("#wizard-capacity").fill("2");
  for (const attribute of profile.attributes.filter((item) => item.required)) {
    const control = page.locator(`#attr-${attribute.code}`);
    if (attribute.type === "enum")
      await control.selectOption(attribute.options![0]!);
    else if (attribute.type === "boolean") await control.check();
    else await control.fill("1");
  }
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.locator("#wizard-price").fill("5001");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.locator("#wizard-timezone")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByRole("heading", { name: /Paso 5:/ })).toBeVisible();
  await page
    .getByRole("button", { name: "Crear espacio y continuar a edición" })
    .click();

  await expect.poll(() => sessionReadsAfterCreate).toBeGreaterThan(0);
  await expect(page).toHaveURL(/\/anfitrion\/espacios\/nuevo$/);
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toContainText(/sesión cambió.*alta/i);
  const safeRecoveryLink = page.getByRole("link", {
    name: "Ir a Mis espacios",
  });
  await expect(safeRecoveryLink).toHaveAttribute("href", "/anfitrion/espacios");
  await expect(
    page.getByRole("link", { name: "Ir al editor del borrador" }),
  ).toHaveCount(0);
  expect(createCount).toBe(1);
  expect(availabilityCount).toBe(0);
  expect(weeklyHoursCount).toBe(0);

  const stepper = page.getByRole("navigation", {
    name: "Pasos del asistente de publicación",
  });
  await page.locator("#wizard-category").selectOption("inmuebles");
  await page
    .locator("#wizard-title")
    .fill("Borrador sintético con cambio de cuenta");
  await page
    .locator("#wizard-description")
    .fill(
      "Descripción sintética suficientemente extensa para validar la recuperación segura del borrador cuando cambia el ámbito de cuenta.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.locator("#wizard-region").selectOption("13");
  await page.locator("#wizard-comuna").selectOption("13101");
  await page
    .locator("#wizard-address")
    .fill("Dirección sintética 122, Santiago");
  await page.locator("#wizard-area").fill("40");
  await page.locator("#wizard-capacity").fill("2");
  for (const attribute of profile.attributes.filter((item) => item.required)) {
    const control = page.locator(`#attr-${attribute.code}`);
    if (attribute.type === "enum")
      await control.selectOption(attribute.options![0]!);
    else if (attribute.type === "boolean") await control.check();
    else await control.fill("1");
  }
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.locator("#wizard-price").fill("5001");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.locator("#wizard-timezone")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  const createButton = page.getByRole("button", {
    name: "Crear espacio y continuar a edición",
  });
  await expect(createButton).toBeDisabled();
  expect(createCount).toBe(1);

  await stepper.getByRole("button", { name: /Tipo y Resumen/ }).click();
  await expect(safeRecoveryLink).toBeVisible();
  await expect(safeRecoveryLink).toHaveAttribute("href", "/anfitrion/espacios");
  await stepper.getByRole("button", { name: /Confirmación/ }).click();
  await expect(createButton).toBeDisabled();
  expect(createCount).toBe(1);

  const accountNavigation = page.getByRole("navigation", {
    name: "Secciones de mi cuenta",
  });
  await accountNavigation.getByRole("link", { name: /Seguridad/ }).click();
  await page.waitForURL((url) => url.pathname === "/mi-cuenta/seguridad");
  await page
    .getByRole("navigation", { name: "Secciones de mi cuenta" })
    .getByRole("link", { name: /Mis espacios/ })
    .click();
  await page.waitForURL((url) => url.pathname === "/anfitrion/espacios");
  await page.getByRole("link", { name: "Publicar un espacio" }).click();
  await page.waitForURL((url) => url.pathname === "/anfitrion/espacios/nuevo");
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Ir a Mis espacios" }),
  ).toHaveCount(0);
  expect(createCount).toBe(1);
});
