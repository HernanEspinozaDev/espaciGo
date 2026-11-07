import { expect, test } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";
import categoryProfiles from "./fixtures/category-profiles-v1.json";

test.use({ trace: "off", screenshot: "off", video: "off" });

for (const rateUnit of ["dia", "mes"] as const) {
  test(`@phase7 @controlled wizard ${rateUnit} omite horario semanal y crea borrador con zona horaria`, async ({
    page,
  }) => {
    const bff = createBffFixtureState();
    let createBody: Record<string, unknown> | null = null;
    let availabilityBody: Record<string, unknown> | null = null;
    const weeklyRequests: string[] = [];
    const profile = categoryProfiles.profiles.inmuebles;

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
            account_id: "770e8400-e29b-41d4-a716-446655440000",
            roles: ["arrendatario", "arrendador"],
            capabilities: [],
            expires_at: new Date(Date.now() + 3600000).toISOString(),
            restricted_mode: false,
          },
        };
      } else if (
        path === "/api/backend/auth/session" &&
        request.method() === "GET"
      ) {
        result = {
          status: 200,
          body: {
            account_id: "770e8400-e29b-41d4-a716-446655440000",
            roles: ["arrendatario", "arrendador"],
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
      } else if (
        path === "/api/backend/spaces" &&
        request.method() === "POST"
      ) {
        createBody = JSON.parse(request.postData() ?? "{}") as Record<
          string,
          unknown
        >;
        result = {
          status: 201,
          headers: { etag: '"wizard-create-v1"' },
          body: {
            id: "00000000-0000-4000-8000-000000000121",
            category_code: "inmuebles",
            category_name: "Inmuebles",
            state: "borrador",
            ...createBody,
          },
        };
      } else if (
        path ===
          "/api/backend/spaces/00000000-0000-4000-8000-000000000121/availability" &&
        request.method() === "PUT"
      ) {
        availabilityBody = JSON.parse(request.postData() ?? "{}") as Record<
          string,
          unknown
        >;
        result = {
          status: 200,
          headers: { etag: '"wizard-time-v2"' },
          body: {
            space_id: "00000000-0000-4000-8000-000000000121",
            time_zone: "America/Santiago",
          },
        };
      } else if (path.endsWith("/weekly-hours")) {
        weeklyRequests.push(`${request.method()} ${path}`);
        result = {
          status: 200,
          headers: { etag: '"wizard-hours-v3"' },
          body: {
            data: {
              space_id: "00000000-0000-4000-8000-000000000121",
              enabled: false,
              time_zone: "America/Santiago",
              days: [],
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
      if (!adapted)
        throw new Error(
          `Missing controlled response for ${request.method()} ${path}`,
        );
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
      .fill("wizard-test@example.invalid");
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("SyntheticPass123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    await page.waitForURL(
      (url) => url.pathname === "/anfitrion/espacios/nuevo",
    );

    await page.locator("#wizard-category").selectOption("inmuebles");
    await page
      .locator("#wizard-title")
      .fill(`Espacio sintético por ${rateUnit}`);
    await page
      .locator("#wizard-description")
      .fill(
        "Descripción sintética suficientemente extensa para completar la creación del inmueble, con información clara sobre su uso y características principales.",
      );
    await page.getByRole("button", { name: "Siguiente" }).click();
    await page.locator("#wizard-region").selectOption("13");
    await page.locator("#wizard-comuna").selectOption("13101");
    await page.locator("#wizard-address").fill("Av. de prueba 100, Santiago");
    await page.locator("#wizard-area").fill("40");
    await page.locator("#wizard-capacity").fill("2");
    for (const attribute of profile.attributes.filter(
      (item) => item.required,
    )) {
      const control = page.locator(`#attr-${attribute.code}`);
      if (attribute.type === "enum")
        await control.selectOption(attribute.options![0]!);
      else if (attribute.type === "boolean") await control.check();
      else if (attribute.type === "enum_list")
        await page.locator(`#attr-${attribute.code}-option-0`).check();
      else await control.fill("1");
    }
    await page.getByRole("button", { name: "Siguiente" }).click();
    await page.locator("#wizard-rate-unit").selectOption(rateUnit);
    await page.locator("#wizard-price").fill("5001");
    await page.getByRole("button", { name: "Siguiente" }).click();
    await expect(page.locator("#wizard-timezone")).toBeVisible();
    await expect(page.locator("#wizard-timezone")).toHaveValue(
      "America/Santiago",
    );
    await expect(
      page.getByLabel("Habilitar calendario de reservas semanal"),
    ).toHaveCount(0);
    await expect(
      page.getByText("Horario semanal no aplicable a tarifas por día o mes", {
        exact: false,
      }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Siguiente" }).click();
    await expect(page.getByRole("heading", { name: /Paso 5:/ })).toBeVisible();
    await expect(page.locator("main")).toContainText(
      "Horario semanal no aplicable a tarifas por día o mes",
    );
    await page
      .getByRole("button", { name: "Crear espacio y continuar a edición" })
      .click();
    await page.waitForURL(
      /\/anfitrion\/espacios\/00000000-0000-4000-8000-000000000121\/editar/,
    );

    expect(createBody).toMatchObject({
      category_code: "inmuebles",
      rate_unit: rateUnit,
      base_price_clp: 5001,
      comuna_code: "13101",
    });
    expect(availabilityBody).toMatchObject({ time_zone: "America/Santiago" });
    expect(JSON.stringify(createBody)).not.toContain("undefined");
    expect(weeklyRequests).toEqual([]);
  });
}
