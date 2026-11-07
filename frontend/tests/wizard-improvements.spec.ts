import { expect, test } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";
import categoryProfiles from "./fixtures/category-profiles-v1.json";

test.use({ trace: "off", screenshot: "off", video: "off" });

test("@phase7 @controlled wizard valida atributos requeridos opcionales sin serializar undefined", async ({
  page,
}) => {
  const bff = createBffFixtureState();
  let createBody: Record<string, unknown> | null = null;
  const profile = categoryProfiles.profiles.inmuebles;
  const required = profile.attributes.filter((attribute) => attribute.required);
  const duration = profile.attributes.find(
    (attribute) => attribute.code === "duracion_minima_unidades",
  );
  if (!duration || required.length === 0)
    throw new Error("Versioned inmuebles fixture is incomplete");

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
    } else if (path === "/api/backend/spaces" && request.method() === "POST") {
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
          title: createBody.title,
          description: createBody.description,
          state: "borrador",
          ...createBody,
        },
      };
    } else if (
      path === "/api/backend/spaces/00000000-0000-4000-8000-000000000121" &&
      request.method() === "GET"
    ) {
      result = {
        status: 200,
        headers: { etag: '"wizard-editor-v1"' },
        body: {
          id: "00000000-0000-4000-8000-000000000121",
          category_code: "inmuebles",
          category_name: "Inmuebles",
          title: createBody?.title,
          description: createBody?.description,
          area_m2: createBody?.area_m2,
          capacity: createBody?.capacity,
          usage_rules: createBody?.usage_rules,
          rate_unit: createBody?.rate_unit,
          base_price_clp: createBody?.base_price_clp,
          address: createBody?.address,
          state: "borrador",
          time_zone: "America/Santiago",
          region_code: createBody?.region_code,
          comuna_code: createBody?.comuna_code,
          attribute_schema_version: createBody?.attribute_schema_version,
          attributes: createBody?.attributes,
        },
      };
    } else if (
      path ===
        "/api/backend/spaces/00000000-0000-4000-8000-000000000121/availability" &&
      request.method() === "PUT"
    ) {
      result = {
        status: 200,
        headers: { etag: '"wizard-time-v2"' },
        body: {
          space_id: "00000000-0000-4000-8000-000000000121",
          time_zone: "America/Santiago",
        },
      };
    } else if (
      path ===
        "/api/backend/spaces/00000000-0000-4000-8000-000000000121/weekly-hours" &&
      request.method() === "PUT"
    ) {
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
  await page.waitForURL((url) => url.pathname === "/anfitrion/espacios/nuevo");

  await page.locator("#wizard-category").selectOption("inmuebles");
  await page.locator("#wizard-title").fill("Casa de prueba sintética");
  await page
    .locator("#wizard-description")
    .fill(
      "Descripción sintética suficientemente extensa para completar la creación del inmueble, con información clara sobre su uso y características principales.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await page.locator("#wizard-region").selectOption("13");
  await expect(page.locator("#wizard-comuna")).toBeEnabled();
  await expect(page.locator("#wizard-comuna option")).toContainText([
    "Selecciona una comuna",
    "Santiago",
    "Cerrillos",
  ]);
  await page.locator("#wizard-comuna").selectOption("13101");
  await page.locator("#wizard-region").selectOption("05");
  await expect(page.locator("#wizard-comuna")).toHaveValue("");
  await page.locator("#wizard-comuna").selectOption("05603");
  await page.locator("#wizard-region").selectOption("13");
  await page.locator("#wizard-comuna").selectOption("13101");
  await page.locator("#wizard-address").fill("Av. de prueba 100, Santiago");
  await page.locator("#wizard-area").fill("40");
  await page.locator("#wizard-capacity").fill("2");

  const durationField = page.locator("#attr-duracion_minima_unidades");
  await durationField.fill("2");
  await durationField.fill("");
  const requiredField = page.locator(`#attr-${required[0]!.code}`);
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(requiredField).toHaveAttribute("aria-invalid", "true");
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toContainText(required[0]!.label);
  await expect(page.getByRole("heading", { name: /Paso 2:/ })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Crear espacio y continuar a edición" }),
  ).toHaveCount(0);
  expect(createBody).toBeNull();

  const requiredEnum = required.find((attribute) => attribute.type === "enum");
  if (!requiredEnum) throw new Error("Required enum missing from fixture");
  const enumControl = page.locator(`#attr-${requiredEnum.code}`);
  await enumControl.selectOption("");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(enumControl).toHaveAttribute("aria-invalid", "true");
  await expect(enumControl).toHaveAttribute(
    "aria-describedby",
    new RegExp(`wizard-attribute-${requiredEnum.code}-error`),
  );
  await expect(
    page.locator(`#wizard-attribute-${requiredEnum.code}-error`),
  ).toContainText(requiredEnum.label);
  await expect(page.getByRole("heading", { name: /Paso 2:/ })).toBeVisible();
  expect(createBody).toBeNull();
  await enumControl.selectOption(requiredEnum.options![0]!);
  await expect(enumControl).not.toHaveAttribute("aria-invalid", "true");

  // Complete actual required fields from the checked-in category fixture.
  for (const attribute of required) {
    const control = page.locator(`#attr-${attribute.code}`);
    if (attribute.type === "enum")
      await control.selectOption(attribute.options![0]!);
    else if (attribute.type === "boolean") await control.check();
    else await control.fill("1");
  }
  await page.locator("#attr-superficie_construida_m2").fill("40");
  await page.locator("#attr-amoblado").check();
  await page.locator("#attr-amoblado").uncheck();
  await page.locator("#attr-dormitorios").fill("0");
  await page.locator("#attr-duracion_minima_unidades").fill("2");
  await page.locator("#attr-duracion_minima_unidades").fill("");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByRole("heading", {
      name: /Paso 3: Tarifas iniciales y condiciones/,
    }),
  ).toBeVisible();

  await page.locator("#wizard-price").fill("5001");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.locator("#wizard-timezone")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByRole("heading", {
      name: /Paso 5: Resumen y confirmación de alta/,
    }),
  ).toBeVisible();

  const stepper = page.getByRole("navigation", {
    name: "Pasos del asistente de publicación",
  });
  await stepper.getByRole("button", { name: /Tipo y Resumen/ }).click();
  await expect(page.locator("#wizard-title")).toHaveValue(
    "Casa de prueba sintética",
  );
  await page.locator("#wizard-title").fill("Título actualizado por teclado");
  await page.keyboard.press("Tab");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await stepper.getByRole("button", { name: /Confirmación/ }).click();
  await expect(page.getByRole("heading", { name: /Paso 5:/ })).toBeVisible();
  await expect(page.locator("main")).toContainText(
    "Título actualizado por teclado",
  );

  // Invalidate only the title, without changing category/profile state.
  await stepper.getByRole("button", { name: /Tipo y Resumen/ }).click();
  await page.locator("#wizard-title").fill("");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await stepper.getByRole("button", { name: /Confirmación/ }).click();
  await expect(page.getByRole("heading", { name: /Paso 1:/ })).toBeVisible();
  await expect(page.locator("#wizard-title")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(page.locator("#wizard-title")).toHaveAttribute(
    "aria-describedby",
    /wizard-title-error/,
  );
  await expect(page.locator("#wizard-title-error")).toContainText(
    "obligatorio",
  );
  expect(createBody).toBeNull();
  await page.locator("#wizard-title").fill("Título actualizado por teclado");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByRole("heading", { name: /Paso 2:/ })).toBeVisible();
  // Restore the final step using only visited checkpoints.
  for (const label of [/Tarifas y Reglas/, /Disponibilidad/, /Confirmación/]) {
    await stepper.getByRole("button", { name: label }).click();
  }
  await expect(page.locator("main")).toContainText(
    "Título actualizado por teclado",
  );

  const summary = await page.locator("main").innerText();
  expect(summary).toContain("Tipo de inmueble");
  expect(summary).toContain("Dormitorios: 0");
  expect(summary).not.toMatch(/duracion_minima_unidades|undefined|true|false/);
  const createButton = page.getByRole("button", {
    name: "Crear espacio y continuar a edición",
  });
  await expect(createButton).toBeEnabled();
  await createButton.click();
  await page.waitForURL(
    /\/anfitrion\/espacios\/00000000-0000-4000-8000-000000000121\/editar/,
  );
  const updatedTitle = "Título actualizado por teclado";
  await expect(
    page.getByRole("heading", { name: updatedTitle, exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("tablist", { name: "Secciones de edición del espacio" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Título del anuncio *" }),
  ).toHaveValue(updatedTitle);

  expect(createBody).not.toBeNull();
  expect(createBody!.title).toBe(updatedTitle);
  expect(createBody).toMatchObject({
    category_code: "inmuebles",
    attribute_schema_version: 1,
    comuna_code: "13101",
    attributes: {
      tipo_inmueble: "Casa",
      tipo_de_espacio: "Inmueble completo",
      superficie_construida_m2: 40,
      dormitorios: 0,
      amoblado: false,
    },
  });
  expect(createBody!.attributes).not.toHaveProperty("duracion_minima_unidades");
});
