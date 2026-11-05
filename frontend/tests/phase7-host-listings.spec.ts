import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";
import categoryProfileFixture from "./fixtures/category-profiles-v1.json";
import type { CategoryProfileSchema } from "../src/features/host-listings/domain/types";

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
    const isBinary =
      Buffer.isBuffer(result.body) || typeof result.body === "string";
    await route.fulfill({
      status: result.status,
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        ...(result.headers ?? {}),
      },
      body:
        result.status === 204
          ? ""
          : isBinary
            ? (result.body as Buffer | string)
            : JSON.stringify(result.body ?? {}),
    });
  });
}

function apiError(
  status: number,
  code: string,
  headers?: Record<string, string>,
): MockResult {
  return {
    status,
    headers,
    body: {
      error: {
        code,
        message: `Synthetic error for code: ${code}`,
      },
    },
  };
}

function syntheticDraftEtag(
  spaceId: string,
  updatedAt: string = "2026-10-28T12:00:00Z",
): string {
  const payload = Buffer.from(`${spaceId}\0${updatedAt}`, "utf8").toString(
    "base64url",
  );
  return `"${payload}"`;
}

const accountId = "770e8400-e29b-41d4-a716-446655440000";

async function loginAndNavigate(
  page: Page,
  targetPath: string = "/anfitrion/espacios",
): Promise<void> {
  await page.goto(`/acceso?return_to=${encodeURIComponent(targetPath)}`);
  await page
    .getByLabel("Correo electrónico")
    .fill("phase7-tester@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("SecurePass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === targetPath);
}

// Mocks comunes de sesión
function handleAuthRequests(
  pathname: string,
  method: string,
): MockResult | null {
  if (pathname === "/api/backend/auth/login" && method === "POST") {
    return {
      status: 200,
      body: {
        account_id: accountId,
        roles: ["arrendatario", "anfitrion"],
        capabilities: [],
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
        restricted_mode: false,
      },
    };
  }
  if (pathname === "/api/backend/auth/session" && method === "GET") {
    return {
      status: 200,
      body: {
        account_id: accountId,
        roles: ["arrendatario", "anfitrion"],
        capabilities: [],
        restricted_mode: false,
      },
    };
  }
  return null;
}

test("@phase7 @controlled /anfitrion/espacios: listado de espacios, filtrado por estado (Todos, Borradores, Publicados, Archivados), badge de estado, paginación y renderizado sin desbordamiento", async ({
  page,
}) => {
  // 8 espacios sintéticos para ejercitar paginación (PAGE_SIZE = 6): 3 borradores, 3 activos, 2 ocultos
  const retireSpaceId = "00000000-0000-4000-8000-000000000021";
  const spaces = [
    {
      id: "00000000-0000-4000-8000-000000000011",
      category_code: "oficina",
      category_name: "Oficinas y Cowork",
      title: "Oficina Providencia A",
      state: "borrador",
      base_price_clp: 20000,
      rate_unit: "hora",
      area_m2: 30,
      capacity: 4,
      region_code: "13",
      comuna_code: "Providencia",
    },
    {
      id: "space-borrador-2",
      category_code: "oficina",
      category_name: "Oficinas y Cowork",
      title: "Oficina Las Condes B",
      state: "borrador",
      base_price_clp: 35000,
      rate_unit: "hora",
      area_m2: 45,
      capacity: 8,
      region_code: "13",
      comuna_code: "Las Condes",
    },
    {
      id: "space-borrador-3",
      category_code: "bodega",
      category_name: "Bodegas y Mini-bodegas",
      title: "Bodega Quilicura C",
      state: "borrador",
      base_price_clp: 150000,
      rate_unit: "mes",
      area_m2: 25,
      capacity: 2,
      region_code: "13",
      comuna_code: "Quilicura",
    },
    {
      id: retireSpaceId,
      category_code: "evento",
      category_name: "Salas de Eventos",
      title: "Sala de Eventos Bellavista D",
      state: "activa",
      base_price_clp: 50000,
      rate_unit: "hora",
      area_m2: 120,
      capacity: 60,
      region_code: "13",
      comuna_code: "Recoleta",
    },
    {
      id: "space-activa-2",
      category_code: "oficina",
      category_name: "Oficinas y Cowork",
      title: "Cowork Santiago Centro E",
      state: "activa",
      base_price_clp: 12000,
      rate_unit: "dia",
      area_m2: 80,
      capacity: 20,
      region_code: "13",
      comuna_code: "Santiago",
    },
    {
      id: "space-activa-3",
      category_code: "estacionamiento",
      category_name: "Estacionamientos",
      title: "Estacionamiento Subterráneo F",
      state: "activa",
      base_price_clp: 3000,
      rate_unit: "hora",
      area_m2: 15,
      capacity: 1,
      region_code: "13",
      comuna_code: "Providencia",
    },
    {
      id: "space-oculta-1",
      category_code: "oficina",
      category_name: "Oficinas y Cowork",
      title: "Oficina Vitacura G (Archivada)",
      state: "oculta",
      base_price_clp: 40000,
      rate_unit: "hora",
      area_m2: 50,
      capacity: 10,
      region_code: "13",
      comuna_code: "Vitacura",
    },
    {
      id: "space-oculta-2",
      category_code: "bodega",
      category_name: "Bodegas y Mini-bodegas",
      title: "Bodega Maipú H (Archivada)",
      state: "oculta",
      base_price_clp: 95000,
      rate_unit: "mes",
      area_m2: 20,
      capacity: 2,
      region_code: "13",
      comuna_code: "Maipú",
    },
  ];

  await mockGateway(page, async (pathname, method) => {
    const authRes = handleAuthRequests(pathname, method);
    if (authRes) return authRes;

    if (pathname === "/api/backend/spaces" && method === "GET") {
      return {
        status: 200,
        body: { items: spaces },
      };
    }

    if (
      pathname === `/api/backend/spaces/${retireSpaceId}` &&
      method === "DELETE"
    ) {
      const spaceId = retireSpaceId;
      const spaceIndex = spaces.findIndex((space) => space.id === spaceId);
      if (spaceIndex >= 0) spaces.splice(spaceIndex, 1);
      return {
        status: 200,
        body: {
          space_id: spaceId,
          state: "eliminada",
          occurred_at: "2026-10-27T12:00:00Z",
          removed: ["catalogo"],
          preserved: ["historicos"],
          files_pending_cleanup: 0,
          reused: false,
        },
      };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios");

  // Verificación de encabezado
  await expect(page.locator("h1")).toHaveText("Mis Espacios");

  // 1. Verificación en pestaña Todos: 6 elementos en página 1
  await expect(page.getByText("Oficina Providencia A")).toBeVisible();
  await expect(page.getByText("Cowork Santiago Centro E")).toBeVisible();
  await expect(page.getByText("Página 1 de 2")).toBeVisible();

  // Paginación: navegar a la página 2
  const nextButton = page.getByRole("button", {
    name: "Ir a página siguiente",
  });
  await expect(nextButton).toBeEnabled();
  await nextButton.click();

  // En página 2 se deben ver los espacios 7 y 8
  await expect(page.getByText("Página 2 de 2")).toBeVisible();
  await expect(page.getByText("Oficina Vitacura G (Archivada)")).toBeVisible();
  await expect(page.getByText("Bodega Maipú H (Archivada)")).toBeVisible();
  await expect(nextButton).toBeDisabled();

  // Volver a página 1
  const prevButton = page.getByRole("button", {
    name: "Ir a página anterior",
  });
  await expect(prevButton).toBeEnabled();
  await prevButton.click();
  await expect(page.getByText("Página 1 de 2")).toBeVisible();

  // 2. Filtrado por Borradores
  await page.getByRole("tab", { name: /Borradores \(3\)/ }).click();
  await expect(page.getByText("Oficina Providencia A")).toBeVisible();
  await expect(page.getByText("Oficina Las Condes B")).toBeVisible();
  await expect(page.getByText("Bodega Quilicura C")).toBeVisible();
  // No debe mostrar activos ni archivados
  await expect(page.getByText("Sala de Eventos Bellavista D")).toBeHidden();
  await expect(page.getByText("Oficina Vitacura G (Archivada)")).toBeHidden();
  // Todos deben lucir el badge 'Borrador'
  const borradorBadges = page.locator(
    "span:has-text('Borrador'):not([role='tab'])",
  );
  expect(await borradorBadges.count()).toBeGreaterThanOrEqual(3);

  // 3. Filtrado por Publicados
  await page.getByRole("tab", { name: /Publicados \(3\)/ }).click();
  await expect(page.getByText("Sala de Eventos Bellavista D")).toBeVisible();
  await expect(page.getByText("Cowork Santiago Centro E")).toBeVisible();
  await expect(page.getByText("Estacionamiento Subterráneo F")).toBeVisible();
  await expect(page.getByText("Oficina Providencia A")).toBeHidden();
  const publicadoBadges = page.locator(
    "span:has-text('Publicado'):not([role='tab'])",
  );
  expect(await publicadoBadges.count()).toBeGreaterThanOrEqual(3);

  // 4. Filtrado por Archivados
  await page.getByRole("tab", { name: /Archivados \(2\)/ }).click();
  await expect(page.getByText("Oficina Vitacura G (Archivada)")).toBeVisible();
  await expect(page.getByText("Bodega Maipú H (Archivada)")).toBeVisible();
  await expect(page.getByText("Oficina Providencia A")).toBeHidden();
  const archivadoBadges = page.locator(
    "span:has-text('Archivado'):not([role='tab'])",
  );
  expect(await archivadoBadges.count()).toBeGreaterThanOrEqual(2);
  await expect(
    page.getByRole("button", {
      name: "Retirar publicación Oficina Vitacura G (Archivada)",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: "Retirar publicación Bodega Maipú H (Archivada)",
    }),
  ).toBeVisible();

  // 5. Verificación de ausencia de desbordamiento horizontal
  const dimensions = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    width: innerWidth,
  }));
  expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.width);

  // Un borrador no se elimina físicamente; la retirada aplica a publicaciones elegibles.
  await page.getByRole("tab", { name: /Borradores \(3\)/ }).click();
  await expect(page.getByText("Oficina Providencia A")).toBeVisible();
  await expect(
    page.getByRole("button", { name: /Eliminar borrador/ }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: /Retirar publicación/ }),
  ).toHaveCount(0);

  await page.getByRole("tab", { name: /Publicados \(3\)/ }).click();
  page.once("dialog", async (dialog) => {
    expect(dialog.message()).toContain("Sala de Eventos Bellavista D");
    expect(dialog.message()).toMatch(/reservas e historiales se conservarán/i);
    await dialog.accept();
  });
  const retireResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname ===
        `/api/backend/spaces/${retireSpaceId}`,
  );
  await page
    .getByRole("button", {
      name: "Retirar publicación Sala de Eventos Bellavista D",
    })
    .click();
  const retireResponse = await retireResponsePromise;
  expect(retireResponse.status()).toBe(200);
  await expect(
    page.getByText("Publicación retirada correctamente."),
  ).toBeVisible();
  await expect(page.getByText("Sala de Eventos Bellavista D")).toHaveCount(0);
});

test("@phase7 @controlled /anfitrion/espacios/nuevo: asistente en 5 pasos (Paso 1: selección dinámica de categorías desde /spaces/categories, título; Paso 2: dirección y atributos dinámicos con attribute_schema_version; Paso 3: tarifas CLP; Paso 4: disponibilidad semanal; Paso 5: confirmación y creación con Idempotency-Key en POST /spaces redirigiendo a /anfitrion/espacios/{id}/editar)", async ({
  page,
}) => {
  let capturedIdempotencyKey: string | null = null;
  let capturedCreateBody: Record<string, unknown> | null = null;
  let acceptedCreate = false;
  let createPostCount = 0;
  let timezoneIfMatch: string | null = null;
  let weeklyHoursIfMatch: string | null = null;
  const postCreateSessionRequests: string[] = [];
  let currentSyntheticSession = {
    account_id: accountId,
    roles: ["arrendatario"],
    capabilities: [] as string[],
    restricted_mode: false,
    expires_at: new Date(Date.now() + 3600_000).toISOString(),
  };
  const newSpaceId = "00000000-0000-4000-8000-000000000101";

  const categories = [
    { code: "oficina", name: "Oficinas y Cowork" },
    { code: "bodega", name: "Bodegas y Mini-bodegas" },
  ];

  const categorySchema = {
    category_code: "oficina",
    schema_version: 1,
    attributes: [
      {
        code: "puestos_trabajo",
        label: "Puestos de trabajo",
        type: "integer",
        order: 1,
        minimum: 1,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "mobiliario_incluido",
        label: "Mobiliario incluido",
        type: "boolean",
        order: 2,
        filter_candidate: false,
      },
      {
        code: "tipo_uso_oficina",
        label: "Tipo de uso",
        type: "enum",
        order: 3,
        options: ["privada", "compartida"],
        filter_candidate: false,
      },
      {
        code: "escritorios",
        label: "Escritorios",
        type: "integer",
        order: 4,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "sillas_trabajo",
        label: "Sillas de trabajo",
        type: "integer",
        order: 5,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "salas_reunion_disponibles",
        label: "Salas de reunión disponibles",
        type: "integer",
        order: 6,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "impresora_disponible",
        label: "Impresora disponible",
        type: "boolean",
        order: 7,
        filter_candidate: false,
      },
      {
        code: "wifi",
        label: "Wi-Fi",
        type: "boolean",
        order: 8,
        filter_candidate: false,
      },
      {
        code: "energia_electrica",
        label: "Energía eléctrica",
        type: "boolean",
        order: 9,
        filter_candidate: false,
      },
      {
        code: "banos_disponibles",
        label: "Baños disponibles",
        type: "integer",
        order: 10,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "climatizacion",
        label: "Climatización",
        type: "enum_list",
        order: 11,
        options: ["aire_acondicionado", "calefaccion", "ventilacion_mecanica"],
        filter_candidate: false,
      },
    ],
  };

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return { status: 200, body: currentSyntheticSession };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      if (acceptedCreate) postCreateSessionRequests.push(pathname);
      return { status: 200, body: currentSyntheticSession };
    }

    if (pathname === "/api/backend/spaces/categories" && method === "GET") {
      return {
        status: 200,
        body: { items: categories },
      };
    }

    if (
      (pathname === "/api/backend/spaces/categories/oficina/attributes" ||
        pathname === "/api/backend/spaces/categories/oficina/attributes/1") &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: categorySchema,
      };
    }

    if (pathname === "/api/backend/spaces" && method === "POST") {
      createPostCount += 1;
      capturedIdempotencyKey = request.headers()["idempotency-key"] ?? null;
      capturedCreateBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      currentSyntheticSession = {
        ...currentSyntheticSession,
        roles: ["arrendatario", "arrendador"],
      };
      acceptedCreate = true;
      return {
        status: 201,
        headers: { etag: '"rev-create-1"' },
        body: {
          id: newSpaceId,
          category_code: "oficina",
          category_name: "Oficinas y Cowork",
          title: capturedCreateBody.title,
          description: capturedCreateBody.description,
          area_m2: capturedCreateBody.area_m2,
          capacity: capturedCreateBody.capacity,
          usage_rules: capturedCreateBody.usage_rules,
          rate_unit: capturedCreateBody.rate_unit,
          base_price_clp: capturedCreateBody.base_price_clp,
          address: capturedCreateBody.address,
          state: "borrador",
          time_zone: null,
          region_code: capturedCreateBody.region_code,
          comuna_code: capturedCreateBody.comuna_code,
          attribute_schema_version: capturedCreateBody.attribute_schema_version,
          attributes: capturedCreateBody.attributes,
        },
      };
    }

    if (
      pathname === `/api/backend/spaces/${newSpaceId}/availability` &&
      method === "PUT"
    ) {
      timezoneIfMatch = request.headers()["if-match"] ?? null;
      return {
        status: 200,
        headers: { etag: '"rev-timezone-2"' },
        body: { space_id: newSpaceId, time_zone: "America/Santiago" },
      };
    }

    if (
      pathname === `/api/backend/spaces/${newSpaceId}/weekly-hours` &&
      method === "PUT"
    ) {
      weeklyHoursIfMatch = request.headers()["if-match"] ?? null;
      return {
        status: 200,
        headers: { etag: '"rev-weekly-3"' },
        body: {
          data: {
            space_id: newSpaceId,
            enabled: true,
            time_zone: "America/Santiago",
            days: [],
          },
        },
      };
    }

    // Endpoints necesarios para la página de edición tras la redirección
    if (pathname === `/api/backend/spaces/${newSpaceId}` && method === "GET") {
      return {
        status: 200,
        headers: { etag: '"rev-weekly-3"' },
        body: {
          id: newSpaceId,
          category_code: "oficina",
          category_name: "Oficinas y Cowork",
          title: "Oficina Ejecutiva El Golf",
          description: "Espacio moderno full equipado con luz natural",
          area_m2: 35,
          capacity: 6,
          usage_rules:
            "Política de cancelación: flexible. No fumar. Cuidar el orden y limpieza de las instalaciones.",
          rate_unit: "hora",
          base_price_clp: 35000,
          address: "Av. Apoquindo 3000, Las Condes",
          state: "borrador",
          time_zone: "America/Santiago",
          region_code: "13",
          comuna_code: "Las Condes",
          attribute_schema_version:
            capturedCreateBody?.attribute_schema_version,
          attributes: capturedCreateBody?.attributes,
        },
      };
    }

    if (
      pathname === `/api/backend/spaces/${newSpaceId}/tariffs` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === `/api/backend/spaces/${newSpaceId}/tariff` &&
      method === "GET"
    ) {
      return apiError(404, "tariff_not_found");
    }
    if (
      pathname === `/api/backend/spaces/${newSpaceId}/gallery` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === `/api/backend/spaces/${newSpaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
        headers: { etag: '"rev-weekly-3"' },
        body: {
          data: {
            space_id: newSpaceId,
            enabled: true,
            time_zone: "America/Santiago",
            days: [],
          },
        },
      };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await expect(page.locator("h1")).toHaveText("Publicar un nuevo espacio");

  // PASO 1: Tipo y Resumen
  await expect(page.locator("#wizard-category")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  const categoryField = page.locator("#wizard-category");
  await expect(categoryField).toHaveAttribute("aria-invalid", "true");
  const categoryDescription =
    (await categoryField.getAttribute("aria-describedby")) ?? "";
  expect(categoryDescription.split(/\s+/)).toContain("wizard-category-error");
  await expect(page.locator("#wizard-category-error")).toBeVisible();
  await expect(categoryField).toBeFocused();
  expect(createPostCount).toBe(0);
  await categoryField.selectOption("oficina");
  await expect(categoryField).not.toHaveAttribute("aria-invalid", "true");
  const titleField = page.locator("#wizard-title");
  await expect(titleField).not.toHaveAttribute("maxlength", /.+/);
  await titleField.fill("😀".repeat(71));
  await page.locator("#wizard-description").fill("d".repeat(100));
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(titleField).toHaveAttribute("aria-invalid", "true");
  const titleErrorId = await titleField.getAttribute("aria-describedby");
  expect(titleErrorId).toBeTruthy();
  await expect(page.locator(`#${titleErrorId}`)).toContainText(/70/);
  await expect(titleField).toBeFocused();
  expect(createPostCount).toBe(0);

  await titleField.fill("😀".repeat(70));
  await expect(page.getByText("70/70")).toBeVisible();
  await titleField.fill("Oficina Ejecutiva El Golf");
  const descriptionField = page.locator("#wizard-description");
  await descriptionField.fill(
    `${" ".repeat(3)}${"d".repeat(99)}${" ".repeat(2)}`,
  );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(descriptionField).toHaveAttribute("aria-invalid", "true");
  const descriptionErrorId =
    await descriptionField.getAttribute("aria-describedby");
  expect(descriptionErrorId?.split(/\s+/)).toContain(
    "wizard-description-error",
  );
  expect(descriptionErrorId?.split(/\s+/)).toContain(
    "wizard-description-count",
  );
  await expect(page.locator("#wizard-description-count")).toContainText(
    "99/100",
  );
  await expect(page.locator("#wizard-description-error")).toContainText(/100/);
  await expect(descriptionField).toBeFocused();
  expect(createPostCount).toBe(0);
  await page
    .locator("#wizard-description")
    .fill("Espacio moderno full equipado con luz natural");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.locator("#wizard-description-error")).toContainText(
    /100 caracteres/i,
  );
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toHaveCount(0);
  expect(createPostCount).toBe(0);
  await page
    .locator("#wizard-description")
    .fill(
      "Espacio moderno full equipado con luz natural para realizar reuniones y actividades de trabajo colaborativo.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();

  // PASO 2: Ubicación y Atributos Dinámicos
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();
  await expect(page.locator("#wizard-address")).not.toHaveAttribute(
    "maxlength",
    /.+/,
  );
  await page.locator("#wizard-address").fill("Av. Apoquindo 3000, Las Condes");
  await page.locator("#wizard-comuna").fill("Las Condes");
  await page.locator("#wizard-area").fill("0");
  await page.locator("#wizard-capacity").fill("6");
  await page.locator("#wizard-address").fill("😀".repeat(501));
  await page.getByRole("button", { name: "Siguiente" }).click();
  const addressField = page.locator("#wizard-address");
  await expect(addressField).toHaveAttribute("aria-invalid", "true");
  const addressErrorId = await addressField.getAttribute("aria-describedby");
  expect(addressErrorId?.split(/\s+/)).toContain("wizard-address-error");
  expect(addressErrorId?.split(/\s+/)).toContain("wizard-address-count");
  await expect(page.locator("#wizard-address-error")).toContainText(/500/);
  await expect(addressField).toBeFocused();
  expect(createPostCount).toBe(0);
  await expect(
    page.getByText("Paso 3: Tarifas iniciales y condiciones"),
  ).toHaveCount(0);
  await page.locator("#wizard-address").fill("Av. Apoquindo 3000, Las Condes");
  await page.getByRole("button", { name: "Siguiente" }).click();
  const areaField = page.locator("#wizard-area");
  await expect(areaField).toHaveAttribute("aria-invalid", "true");
  const areaErrorId = await areaField.getAttribute("aria-describedby");
  expect(areaErrorId?.split(/\s+/)).toContain("wizard-area-error");
  await expect(page.locator("#wizard-area-error")).toContainText(/superficie/i);
  await expect(areaField).toBeFocused();
  expect(createPostCount).toBe(0);
  await areaField.fill("35");
  await page.locator("#wizard-capacity").fill("6");
  const dynamicNumberField = page.locator("#attr-puestos_trabajo");
  await dynamicNumberField.fill("0");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();
  expect(createPostCount).toBe(0);
  await expect(dynamicNumberField).toHaveAttribute("aria-invalid", "true");
  const dynamicErrorTokens =
    (await dynamicNumberField.getAttribute("aria-describedby"))?.split(
      /\\s+/,
    ) ?? [];
  const dynamicErrorId = dynamicErrorTokens.find((token) =>
    token.includes("puestos_trabajo"),
  );
  expect(dynamicErrorId).toBeTruthy();
  const dynamicError = page.locator(`#${dynamicErrorId}`);
  await expect(dynamicError).toBeVisible();
  await expect(dynamicError).toContainText(/1|válido|mínimo/i);
  await expect(dynamicNumberField).toBeFocused();
  await dynamicNumberField.fill("6");
  await expect(dynamicNumberField).not.toHaveAttribute("aria-invalid", "true");
  await expect(dynamicError).toHaveCount(0);
  await page.locator("#wizard-capacity").fill("0");
  await page.getByRole("button", { name: "Siguiente" }).click();
  const capacityField = page.locator("#wizard-capacity");
  await expect(capacityField).toHaveAttribute("aria-invalid", "true");
  const capacityErrorId = await capacityField.getAttribute("aria-describedby");
  expect(capacityErrorId?.split(/\s+/)).toContain("wizard-capacity-error");
  await expect(page.locator("#wizard-capacity-error")).toContainText(
    /capacidad/i,
  );
  await expect(capacityField).toBeFocused();
  expect(createPostCount).toBe(0);
  await capacityField.fill("6");
  await page.locator("#attr-puestos_trabajo").fill("6");
  const wifiCheckbox = page.locator("#attr-wifi");
  await expect(wifiCheckbox).toBeVisible();
  await wifiCheckbox.check();
  const calefaccion = page.getByLabel("calefaccion", { exact: true });
  await expect(calefaccion).toBeVisible();
  await calefaccion.check();
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 3: Tarifas y Reglas
  await expect(
    page.getByText("Paso 3: Tarifas iniciales y condiciones"),
  ).toBeVisible();
  await expect(page.locator("#wizard-deposit")).toHaveCount(0);
  await page.locator("#wizard-rate-unit").selectOption("hora");
  const priceField = page.locator("#wizard-price");
  await priceField.fill("5000");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 3: Tarifas iniciales y condiciones"),
  ).toBeVisible();
  await expect(
    page.getByRole("alert").filter({ hasText: "El precio base debe ser un" }),
  ).toContainText(/mayor a 5000 CLP/i);
  await expect(priceField).toHaveAttribute("aria-invalid", "true");
  const priceErrorId = await priceField.getAttribute("aria-describedby");
  expect(priceErrorId?.split(/\s+/)).toContain("wizard-price-error");
  await expect(page.locator("#wizard-price-error")).toContainText(
    /mayor a 5000 CLP/i,
  );
  await expect(priceField).toBeFocused();
  expect(createPostCount).toBe(0);
  await priceField.fill("5001");
  await expect(priceField).not.toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#wizard-price-error")).toHaveCount(0);
  await expect(
    page.getByRole("alert").filter({ hasText: "El precio base debe ser un" }),
  ).toHaveCount(0);
  const rulesField = page.locator("#wizard-usage-rules");
  await rulesField.fill("ñ".repeat(250));
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 3: Tarifas iniciales y condiciones"),
  ).toBeVisible();
  await expect(rulesField).toHaveAttribute("aria-invalid", "true");
  const rulesErrorIds = await rulesField.getAttribute("aria-describedby");
  expect(rulesErrorIds?.split(/\s+/)).toContain("wizard-usage-rules-count");
  expect(rulesErrorIds?.split(/\s+/)).toContain("wizard-usage-rules-error");
  await expect(page.locator("#wizard-usage-rules-error")).toContainText(
    /250 caracteres/i,
  );
  await expect(rulesField).toBeFocused();
  expect(createPostCount).toBe(0);
  await rulesField.fill("Mantener el espacio limpio y respetar los horarios.");
  await expect(rulesField).not.toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#wizard-usage-rules-error")).toHaveCount(0);
  await expect(
    page.getByRole("alert").filter({ hasText: /reglas de uso compuestas/i }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 4: Disponibilidad semanal
  await expect(
    page.getByText("Paso 4: Disponibilidad semanal y zona horaria"),
  ).toBeVisible();
  await expect(page.locator("#wizard-timezone")).toHaveValue(
    "America/Santiago",
  );
  const timezoneField = page.locator("#wizard-timezone");
  await timezoneField.fill("Invalid/Timezone");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 4: Disponibilidad semanal y zona horaria"),
  ).toBeVisible();
  await expect(timezoneField).toHaveAttribute("aria-invalid", "true");
  const timezoneErrorIds = await timezoneField.getAttribute("aria-describedby");
  expect(timezoneErrorIds?.split(/\s+/)).toContain("wizard-timezone-error");
  await expect(page.locator("#wizard-timezone-error")).toContainText(
    /identificador IANA válido/i,
  );
  await expect(timezoneField).toBeFocused();
  expect(createPostCount).toBe(0);
  await timezoneField.fill("America/Santiago");
  await expect(timezoneField).not.toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#wizard-timezone-error")).toHaveCount(0);
  await expect(
    page.getByRole("alert").filter({ hasText: /IANA válido/i }),
  ).toHaveCount(0);

  await page.locator("#schedule-enabled").check();
  await page.getByRole("button", { name: "Eliminar tramo de Lunes" }).click();
  await page.getByRole("button", { name: "Agregar tramo" }).first().click();
  const mondayStart = page.getByLabel("Hora inicio Lunes").first();
  const mondayEnd = page.getByLabel("Hora fin Lunes").first();
  await mondayStart.fill("09:00");
  await mondayEnd.fill("08:00");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 4: Disponibilidad semanal y zona horaria"),
  ).toBeVisible();
  const scheduleError = page.locator("#wizard-schedule-error");
  await expect(scheduleError).toBeVisible();
  await expect(scheduleError).toContainText(/lunes|09:00|08:00/i);
  const scheduleGroup = page.locator("#wizard-schedule-group");
  await expect(scheduleGroup).toHaveAttribute(
    "aria-describedby",
    "wizard-schedule-error",
  );
  for (const timeField of [mondayStart, mondayEnd]) {
    await expect(timeField).toHaveAttribute("aria-invalid", "true");
    expect(
      (await timeField.getAttribute("aria-describedby"))?.split(/\\s+/),
    ).toContain("wizard-schedule-error");
  }
  await expect(scheduleGroup).toBeFocused();
  expect(createPostCount).toBe(0);
  await mondayEnd.fill("17:00");
  await expect(scheduleError).toHaveCount(0);
  await expect(
    page.getByRole("alert").filter({ hasText: /tramo|horario/i }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 5: Confirmación
  await expect(
    page.getByText("Paso 5: Resumen y confirmación de alta"),
  ).toBeVisible();
  await expect(
    page.locator("h2").filter({ hasText: "Oficina Ejecutiva El Golf" }),
  ).toBeVisible();
  await expect(page.getByText("$5.001 CLP")).toBeVisible();

  // Enviar y verificar llamada con Idempotency-Key UUID
  const postCreateSessionPromise = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname === "/api/backend/auth/session",
  );
  await page
    .getByRole("button", { name: "Crear espacio y continuar a edición" })
    .click();
  const postCreateSessionResponse = await postCreateSessionPromise;
  expect(postCreateSessionResponse.status()).toBe(200);
  const refreshedSession = (await postCreateSessionResponse.json()) as {
    roles?: string[];
  };
  expect(refreshedSession.roles).toContain("arrendador");
  expect(postCreateSessionRequests).toHaveLength(1);

  // Redirección confirmada al editor
  await page.waitForURL((url) =>
    url.pathname.endsWith(`/espacios/${newSpaceId}/editar`),
  );
  await expect(page.locator("h1")).toHaveText("Oficina Ejecutiva El Golf");

  // Verificaciones del contrato POST /spaces y clave de idempotencia
  expect(capturedIdempotencyKey).toBeTruthy();
  expect(createPostCount).toBe(1);
  expect(timezoneIfMatch).toBe('"rev-create-1"');
  expect(weeklyHoursIfMatch).toBe('"rev-timezone-2"');
  expect(capturedIdempotencyKey).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(capturedCreateBody).not.toBeNull();
  const createBody = capturedCreateBody!;
  expect(createBody.category_code).toBe("oficina");
  expect(createBody.title).toBe("Oficina Ejecutiva El Golf");
  expect(createBody.base_price_clp).toBe(5001);
  expect(createBody.rate_unit).toBe("hora");
  expect(createBody.attribute_schema_version).toBe(1);
  expect(createBody.usage_rules).toEqual(
    expect.stringContaining("Política de cancelación:"),
  );
  expect(createBody.attributes).toMatchObject({
    puestos_trabajo: 6,
    wifi: true,
    climatizacion: ["calefaccion"],
  });
  expect(createBody.attributes).not.toHaveProperty("cancellation_policy");
  expect(createBody.attributes).not.toHaveProperty("security_deposit_clp");

  await page.reload();
  await expect(page.locator("h1")).toHaveText("Oficina Ejecutiva El Golf");
  await expect(page.getByLabel("calefaccion", { exact: true })).toBeChecked();
  expect(createPostCount).toBe(1);
});

test("@phase7 @controlled /anfitrion/espacios/[id]/editar: navegación por pestañas (Información con PUT /spaces/{id}/publication-content, Tarifas e historial con PUT /spaces/{id}/tariff y GET /spaces/{id}/tariffs, Horarios con PUT /spaces/{id}/availability, Fotos con selección de imágenes, portada y reordenamiento, y Publicación con PUT /spaces/{id}/publication)", async ({
  page,
}) => {
  const spaceId = "00000000-0000-4000-8000-000000000042";
  const firstPhotoId = "00000000-0000-4000-8000-000000000001";
  let putContentBody: Record<string, unknown> | null = null;
  let rejectNextDraftUpdate: Array<{ field: string; code: string }> | null =
    null;
  let draftIfMatchHeader: string | undefined;
  const contentIfMatchHeaders: string[] = [];
  const contentResponseEtags = [
    '"published-content-v1"',
    '"published-content-v2"',
    '"published-content-v3"',
  ];
  let publishedContentSaveCount = 0;
  type ReplacementAttempt = {
    targetPhotoId: string;
    ifMatch: string | null;
    idempotencyKey: string | null;
    contentType: string;
    body: string;
  };
  let replacementAttempt: ReplacementAttempt | null = null;
  const replacementAttempts: ReplacementAttempt[] = [];
  let replacementStatusLookupCount = 0;
  let replacementStatusState:
    "pending" | "not_found" | "deleting" | "ready" | "error" = "pending";
  let putTariffBody: Record<string, unknown> | null = null;
  let tariffIfMatchHeader: string | undefined;
  let putAvailabilityBody: Record<string, unknown> | null = null;
  let availabilityIfMatchHeader: string | undefined;
  let weeklyHoursIfMatchHeader: string | undefined;
  let putPublicationBody: Record<string, unknown> | null = null;
  let putCoverBody: Record<string, unknown> | null = null;
  let postGalleryCount = 0;
  let activeGalleryPosts = 0;
  let maxActiveGalleryPosts = 0;
  let deferGalleryPosts = false;
  const releaseGalleryPosts: Array<() => void> = [];
  let thumbnailGetCount = 0;
  let secondPhotoThumbnailGetCount = 0;
  let reorderPutCount = 0;
  const reorderIfMatchHeaders: string[] = [];
  const reorderRequests: Array<{
    photoId: string;
    position: number | undefined;
    ifMatch: string;
  }> = [];
  let rejectNextReorderAsStale = false;
  let deletedPhotoId: string | null = null;
  const galleryDeleteIfMatchHeaders: string[] = [];
  const galleryDeleteResponseEtags = [
    '"gallery-delete-cas-v1"',
    '"gallery-delete-cas-v2"',
    '"gallery-delete-cas-v3"',
  ];
  const galleryUploads: Array<{
    contentType: string;
    idempotencyKey: string | null;
    body: string;
    imagePayload: string | null;
  }> = [];
  let retryTargetFailed = false;
  let pendingTargetResponded = false;
  let pendingReplacementResponded = false;
  let deletingReplacementResponded = false;
  let pendingStatusCheckCount = 0;
  let cancelledUploadKey: string | null = null;
  let cancelledUploadPhotoId: string | null = null;
  let cancelledUploadState: "pending" | "ready" | "not_found" = "pending";
  let cancelledUploadStatusLookupCount = 0;
  const galleryStatusLookups: string[] = [];
  const replacementStatusLookups: string[] = [];
  const detailReloads: Array<{ etag: string; cover: unknown }> = [];
  const galleryReloads: unknown[][] = [];
  let currentSpaceEtag = syntheticDraftEtag(spaceId);
  const replacementPhotoId = "00000000-0000-4000-8000-000000000099";
  const readyReplacementPhotoId = "00000000-0000-4000-8000-000000000091";
  const pendingReplacementPhotoId = "00000000-0000-4000-8000-000000000100";

  const currentSpace = {
    id: spaceId,
    category_code: "oficina",
    category_name: "Oficinas y Cowork",
    title: "Oficina Providencia Inicial",
    description: "Espacio con mobiliario completo y sala de reuniones",
    area_m2: 50,
    capacity: 10,
    usage_rules: "No fumar. Mantener silencio.",
    rate_unit: "hora",
    base_price_clp: 20000,
    address: "Av. Providencia 1234",
    state: "borrador",
    time_zone: "America/Santiago",
    region_code: "13",
    comuna_code: "Providencia",
    attribute_schema_version: 1,
    attributes: { wifi: true, climatizacion: ["calefaccion"] },
    cover_photo_id: null as string | null,
  };

  const tariffHistory = [
    {
      space_id: spaceId,
      version: 1,
      rate_unit: "hora",
      base_price: 20000,
      currency: "CLP",
      created_at: "2026-10-25T10:00:00Z",
    },
  ];

  const photos: Array<Record<string, unknown>> = [];

  await mockGateway(page, async (pathname, method, request) => {
    const authRes = handleAuthRequests(pathname, method);
    if (authRes) return authRes;

    // Espacio actual
    if (pathname === `/api/backend/spaces/${spaceId}` && method === "GET") {
      detailReloads.push({
        etag: currentSpaceEtag,
        cover: currentSpace.cover_photo_id,
      });
      return {
        status: 200,
        headers: { etag: currentSpaceEtag },
        body: currentSpace,
      };
    }

    // Atributos de categoría
    if (
      pathname === "/api/backend/spaces/categories/oficina/attributes/1" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          category_code: "oficina",
          schema_version: 1,
          attributes: [
            {
              code: "puestos_trabajo",
              label: "Puestos de trabajo",
              type: "integer",
              required: false,
              order: 1,
              minimum: 1,
              maximum: 2147483647,
              filter_candidate: false,
            },
            {
              code: "tipo_uso_oficina",
              label: "Tipo de uso",
              type: "enum",
              required: false,
              order: 3,
              options: ["privada", "compartida"],
              filter_candidate: false,
            },
            {
              code: "wifi",
              label: "Wi-Fi",
              type: "boolean",
              required: false,
              order: 1,
              filter_candidate: false,
            },
            {
              code: "climatizacion",
              label: "Climatización",
              type: "enum_list",
              required: false,
              order: 2,
              options: [
                "aire_acondicionado",
                "calefaccion",
                "ventilacion_mecanica",
              ],
              filter_candidate: false,
            },
          ],
        },
      };
    }

    // Tarifas
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariff` &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: tariffHistory[tariffHistory.length - 1],
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariffs` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: tariffHistory } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariff` &&
      method === "PUT"
    ) {
      putTariffBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      tariffIfMatchHeader = request.headers()["if-match"];
      const newRate = {
        space_id: spaceId,
        version: tariffHistory.length + 1,
        rate_unit: putTariffBody.rate_unit as "hora",
        base_price: Number(putTariffBody.base_price),
        currency: "CLP" as const,
        created_at: new Date().toISOString(),
      };
      tariffHistory.unshift(newRate);
      currentSpace.base_price_clp = Number(putTariffBody.base_price);
      currentSpaceEtag = syntheticDraftEtag(
        spaceId,
        "2026-10-28T12:00:01.500Z",
      );
      return {
        status: 200,
        headers: { etag: currentSpaceEtag },
        body: newRate,
      };
    }

    // Actualización de contenido / publicación
    if (
      (pathname === `/api/backend/spaces/${spaceId}/publication-content` ||
        pathname === `/api/backend/spaces/${spaceId}`) &&
      method === "PUT"
    ) {
      putContentBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      if (pathname === `/api/backend/spaces/${spaceId}`) {
        draftIfMatchHeader = request.headers()["if-match"];
      } else if (currentSpace.state !== "borrador") {
        contentIfMatchHeaders.push(request.headers()["if-match"] ?? "");
        publishedContentSaveCount += 1;
        if (publishedContentSaveCount === 3)
          return apiError(412, "precondition_failed");
        currentSpaceEtag = contentResponseEtags[publishedContentSaveCount - 1]!;
      }
      if (
        rejectNextDraftUpdate &&
        pathname === `/api/backend/spaces/${spaceId}`
      ) {
        const details = rejectNextDraftUpdate;
        rejectNextDraftUpdate = null;
        return {
          status: 422,
          body: {
            error: {
              code: "validation_error",
              message: "Revisa los campos indicados.",
              details,
            },
          },
        };
      }
      if (putContentBody.title)
        currentSpace.title = String(putContentBody.title);
      currentSpace.attributes =
        putContentBody.attributes as typeof currentSpace.attributes;
      currentSpace.attribute_schema_version = Number(
        putContentBody.attribute_schema_version,
      );
      if (currentSpace.state === "borrador") {
        currentSpaceEtag = syntheticDraftEtag(spaceId, "2026-10-28T12:00:01Z");
      }
      return {
        status: 200,
        headers: {
          etag: currentSpaceEtag,
        },
        body: currentSpace,
      };
    }

    // Horarios y disponibilidad
    if (
      pathname === `/api/backend/spaces/${spaceId}/availability` &&
      method === "PUT"
    ) {
      putAvailabilityBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      availabilityIfMatchHeader = request.headers()["if-match"];
      return {
        status: 200,
        headers: { etag: currentSpaceEtag },
        body: {
          space_id: spaceId,
          time_zone: putAvailabilityBody.time_zone ?? "America/Santiago",
        },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
        headers: { etag: syntheticDraftEtag(spaceId) },
        body: {
          data: {
            space_id: spaceId,
            enabled: true,
            time_zone: "America/Santiago",
            days: [
              {
                weekday: 1,
                periods: [{ open: "09:00", close: "18:00" }],
              },
            ],
          },
        },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      method === "PUT"
    ) {
      weeklyHoursIfMatchHeader = request.headers()["if-match"];
      return {
        status: 200,
        headers: { etag: currentSpaceEtag },
        body: {
          data: {
            space_id: spaceId,
            enabled: true,
            time_zone: "America/Santiago",
            days: [
              {
                weekday: 1,
                periods: [{ open: "09:00", close: "18:00" }],
              },
            ],
          },
        },
      };
    }

    // Galería y Fotos
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "GET"
    ) {
      galleryReloads.push(photos.map((photo) => photo.id));
      return { status: 200, body: { items: photos } };
    }
    if (
      (pathname ===
        `/api/backend/spaces/${spaceId}/gallery/${firstPhotoId}/variants/thumbnail` ||
        pathname ===
          `/api/backend/spaces/${spaceId}/gallery/${replacementPhotoId}/variants/thumbnail` ||
        pathname ===
          `/api/backend/spaces/${spaceId}/gallery/${pendingReplacementPhotoId}/variants/thumbnail` ||
        pathname ===
          `/api/backend/spaces/${spaceId}/gallery/${readyReplacementPhotoId}/variants/thumbnail`) &&
      method === "GET"
    ) {
      thumbnailGetCount += 1;
      return {
        status: 200,
        headers: { "content-type": "image/webp" },
        body: Buffer.from([]),
      };
    }
    const secondPhotoId = "00000000-0000-4000-8000-000000000002";
    if (
      pathname ===
        `/api/backend/spaces/${spaceId}/gallery/${secondPhotoId}/variants/thumbnail` &&
      method === "GET"
    ) {
      secondPhotoThumbnailGetCount += 1;
      return {
        status: 200,
        headers: { "content-type": "image/webp" },
        body: Buffer.from([]),
      };
    }
    const galleryReplacementMatch = pathname.match(
      new RegExp(`^/api/backend/spaces/${spaceId}/gallery/([^/]+)/content$`),
    );
    if (galleryReplacementMatch && method === "PUT") {
      const targetPhotoId = galleryReplacementMatch[1]!;
      const targetIndex = photos.findIndex(
        (photo) => String(photo.id) === targetPhotoId,
      );
      if (targetIndex < 0) return apiError(404, "not_found");
      const headers = request.headers();
      const body = request.postData() ?? "";
      replacementAttempt = {
        targetPhotoId,
        ifMatch: headers["if-match"] ?? null,
        idempotencyKey: headers["idempotency-key"] ?? null,
        contentType: headers["content-type"] ?? "",
        body,
      };
      replacementAttempts.push(replacementAttempt);
      const previous = photos[targetIndex]!;
      if (
        body.includes("foto-reemplazo-pending.png") &&
        !pendingReplacementResponded
      ) {
        pendingReplacementResponded = true;
        return {
          status: 202,
          headers: { "retry-after": "2" },
          body: {
            photo: {
              id: targetPhotoId,
              space_id: spaceId,
              state: "pending",
            },
            reused: true,
          },
        };
      }
      if (
        body.includes("foto-reemplazo-deleting.png") &&
        !deletingReplacementResponded
      ) {
        deletingReplacementResponded = true;
        return {
          status: 202,
          headers: { "retry-after": "2" },
          body: {
            photo: {
              id: targetPhotoId,
              space_id: spaceId,
              state: "pending",
            },
            reused: true,
          },
        };
      }
      const wasCover =
        Boolean(previous.cover) ||
        currentSpace.cover_photo_id === targetPhotoId;
      const nextReplacementPhotoId = body.includes("foto-reemplazo-pending.png")
        ? pendingReplacementPhotoId
        : replacementPhotoId;
      const variants = Array.isArray(previous.variants)
        ? (previous.variants as Array<Record<string, unknown>>)
        : [];
      const replacementPhoto = {
        ...previous,
        id: nextReplacementPhotoId,
        cover: wasCover,
        variants: variants.map((variant) => ({
          ...variant,
          url: `/api/v1/spaces/${spaceId}/gallery/${nextReplacementPhotoId}/variants/${String(variant.name)}`,
        })),
      };
      photos.splice(targetIndex, 1, replacementPhoto);
      if (wasCover) currentSpace.cover_photo_id = replacementPhotoId;
      currentSpaceEtag = syntheticDraftEtag(spaceId, "2026-10-28T12:00:02Z");
      return {
        status: 201,
        body: { photo: replacementPhoto, reused: false },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery/upload-status` &&
      method === "GET"
    ) {
      const idempotencyKey = request.headers()["idempotency-key"] ?? "";
      if (
        idempotencyKey === replacementAttempt?.idempotencyKey &&
        (replacementAttempt.body.includes("foto-reemplazo-pending.png") ||
          replacementAttempt.body.includes("foto-reemplazo-deleting.png"))
      ) {
        replacementStatusLookups.push(idempotencyKey);
        replacementStatusLookupCount += 1;
        if (replacementStatusState === "not_found") {
          return apiError(404, "not_found");
        }
        if (replacementStatusState === "error") {
          return apiError(503, "unavailable");
        }
        if (replacementStatusState === "ready") {
          const currentVariants = Array.isArray(photos[0]?.variants)
            ? (photos[0].variants as Array<Record<string, unknown>>)
            : [];
          photos.splice(0, photos.length, {
            ...photos[0],
            id: readyReplacementPhotoId,
            cover: true,
            variants: currentVariants.map((variant) => ({
              ...variant,
              url: `/api/v1/spaces/${spaceId}/gallery/${readyReplacementPhotoId}/variants/${String(variant.name)}`,
            })),
          });
          currentSpace.cover_photo_id = readyReplacementPhotoId;
          currentSpaceEtag = syntheticDraftEtag(
            spaceId,
            "2026-10-28T12:00:03Z",
          );
        }
        return {
          status: 200,
          body: {
            photo_id:
              replacementStatusState === "ready"
                ? readyReplacementPhotoId
                : pendingReplacementPhotoId,
            state: replacementStatusState,
          },
        };
      }
      galleryStatusLookups.push(idempotencyKey);
      if (idempotencyKey === cancelledUploadKey) {
        cancelledUploadStatusLookupCount += 1;
        if (cancelledUploadState === "not_found") {
          return apiError(404, "not_found");
        }
        return {
          status: 200,
          body: {
            photo_id:
              cancelledUploadPhotoId ?? "00000000-0000-4000-8000-000000000097",
            state: cancelledUploadState,
          },
        };
      }
      const pendingUpload = galleryUploads.find((upload) =>
        upload.body.includes("pending-me.png"),
      );
      if (!pendingUpload || idempotencyKey !== pendingUpload.idempotencyKey) {
        return apiError(404, "not_found");
      }
      pendingStatusCheckCount += 1;
      if (pendingStatusCheckCount === 1) {
        return {
          status: 200,
          body: {
            photo_id: "00000000-0000-4000-8000-000000000098",
            state: "pending",
          },
        };
      }
      const photoId = "00000000-0000-4000-8000-000000000098";
      if (!photos.some((photo) => String(photo.id) === photoId)) {
        photos.push({
          id: photoId,
          space_id: spaceId,
          alt_text: "",
          visible: true,
          cover: false,
          position: 30,
          state: "ready",
          created_at: "2026-10-28T12:00:00Z",
          variants: ["thumbnail", "display"].map((name) => ({
            name,
            url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
            mime_type: "image/webp",
            size_bytes: 128,
            sha256: "b".repeat(64),
            width: 32,
            height: 32,
          })),
        });
      }
      return { status: 200, body: { photo_id: photoId, state: "ready" } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "POST"
    ) {
      postGalleryCount += 1;
      const uploadNumber = postGalleryCount;
      activeGalleryPosts += 1;
      maxActiveGalleryPosts = Math.max(
        maxActiveGalleryPosts,
        activeGalleryPosts,
      );
      const headers = request.headers();
      const body = request.postData() ?? "";
      const rawBody = request.postDataBuffer();
      const imageOffset = rawBody?.indexOf(tinyPng) ?? -1;
      const idempotencyKey = headers["idempotency-key"] ?? null;
      galleryUploads.push({
        contentType: headers["content-type"] ?? "",
        idempotencyKey,
        body,
        imagePayload:
          imageOffset >= 0
            ? rawBody!
                .subarray(imageOffset, imageOffset + tinyPng.length)
                .toString("base64")
            : null,
      });
      if (
        body.includes("cancel-in-flight.png") ||
        body.includes("cancel-not-found.png")
      ) {
        cancelledUploadKey = idempotencyKey;
      }
      if (deferGalleryPosts) {
        await new Promise<void>((resolve) => releaseGalleryPosts.push(resolve));
      }
      activeGalleryPosts -= 1;
      if (body.includes("cancel-in-flight.png")) {
        cancelledUploadPhotoId = `00000000-0000-4000-8000-${String(uploadNumber).padStart(12, "0")}`;
      }
      if (body.includes("cancel-not-found.png")) {
        return apiError(404, "not_found");
      }
      if (body.includes("pending-me.png") && !pendingTargetResponded) {
        pendingTargetResponded = true;
        return {
          status: 202,
          headers: { "retry-after": "2" },
          body: {
            photo: {
              id: "00000000-0000-4000-8000-000000000098",
              space_id: spaceId,
              state: "pending",
            },
            reused: true,
          },
        };
      }
      if (body.includes("retry-me.png") && !retryTargetFailed) {
        retryTargetFailed = true;
        return apiError(500, "temporary_upload_failure");
      }
      const photoId = `00000000-0000-4000-8000-${String(uploadNumber).padStart(12, "0")}`;
      const newPhoto = {
        id: photoId,
        space_id: spaceId,
        alt_text: "",
        visible: true,
        cover: photos.length === 0,
        position: photos.length + 1,
        state: "ready",
        created_at: "2026-10-27T12:00:00Z",
        variants: (uploadNumber === 1
          ? ["thumbnail", "display"]
          : ["display"]
        ).map((name) => ({
          name,
          url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
          mime_type: "image/webp",
          size_bytes: 128,
          sha256: `a${String(uploadNumber).repeat(63)}`,
          width: 32,
          height: 32,
        })),
      };
      photos.push(newPhoto);
      if (newPhoto.cover) currentSpace.cover_photo_id = photoId;
      return { status: 201, body: { photo: newPhoto, reused: false } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/cover` &&
      method === "PUT"
    ) {
      putCoverBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      currentSpace.cover_photo_id = String(putCoverBody.photo_id);
      for (const p of photos) {
        p.is_cover = p.id === putCoverBody.photo_id;
      }
      return { status: 204 };
    }
    const galleryDeleteMatch = pathname.match(
      new RegExp(`^/api/backend/spaces/${spaceId}/gallery/([^/]+)$`),
    );
    if (galleryDeleteMatch && method === "DELETE") {
      const ifMatchHeader = request.headers()["if-match"] ?? "";
      galleryDeleteIfMatchHeaders.push(ifMatchHeader);
      if (ifMatchHeader !== currentSpaceEtag) {
        return apiError(412, "precondition_failed");
      }
      deletedPhotoId = galleryDeleteMatch[1];
      const deletedIndex = photos.findIndex(
        (photo) => String(photo.id) === deletedPhotoId,
      );
      if (deletedIndex >= 0) {
        const [deletedPhoto] = photos.splice(deletedIndex, 1);
        if (deletedPhoto.cover) {
          currentSpace.cover_photo_id = photos[0] ? String(photos[0].id) : null;
          if (photos[0]) photos[0].cover = true;
        }
        photos.forEach((photo, index) => {
          photo.position = index + 1;
        });
      }
      const responseEtag =
        galleryDeleteResponseEtags[galleryDeleteIfMatchHeaders.length - 1];
      if (!responseEtag) return apiError(412, "precondition_failed");
      currentSpaceEtag = responseEtag;
      return {
        status: 200,
        headers: { etag: responseEtag },
        body: { photo_id: deletedPhotoId, removed: true },
      };
    }
    if (
      pathname.startsWith(`/api/backend/spaces/${spaceId}/gallery/`) &&
      method === "PUT"
    ) {
      const positionMatch = pathname.match(
        new RegExp(
          `^/api/backend/spaces/${spaceId}/gallery/([^/]+)(?:/position)?$`,
        ),
      );
      if (!positionMatch) return apiError(404, "not_found");
      const body = JSON.parse(request.postData() ?? "{}") as {
        position?: number;
      };
      const ifMatch = request.headers()["if-match"] ?? "";
      reorderPutCount += 1;
      reorderIfMatchHeaders.push(ifMatch);
      reorderRequests.push({
        photoId: positionMatch[1]!,
        position: body.position,
        ifMatch,
      });
      if (rejectNextReorderAsStale) {
        rejectNextReorderAsStale = false;
        return apiError(412, "precondition_failed");
      }
      const targetIndex = photos.findIndex(
        (photo) => String(photo.id) === positionMatch[1],
      );
      if (targetIndex < 0) return apiError(404, "not_found");
      const [moved] = photos.splice(targetIndex, 1);
      photos.splice(Math.max(0, (body.position ?? 1) - 1), 0, moved!);
      photos.forEach((photo, index) => {
        photo.position = index + 1;
      });
      currentSpaceEtag = `"gallery-position-rev-${reorderPutCount + 1}"`;
      return { status: 200, headers: { etag: currentSpaceEtag }, body: {} };
    }

    // Publicación / Ocultación
    if (
      pathname === `/api/backend/spaces/${spaceId}/publication` &&
      method === "PUT"
    ) {
      expect(request.headers()["if-match"]).toBe(currentSpaceEtag);
      putPublicationBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      currentSpace.state = String(putPublicationBody.state);
      currentSpaceEtag = syntheticDraftEtag(spaceId, "2026-10-28T12:00:04Z");
      return {
        status: 200,
        headers: { etag: currentSpaceEtag },
        body: currentSpace,
      };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, `/anfitrion/espacios/${spaceId}/editar`);

  // Verificación de carga y estado inicial
  await expect(page.locator("h1")).toHaveText("Oficina Providencia Inicial");
  await expect(page.getByText("Borrador").first()).toBeVisible();

  // 1. Pestaña Información: validación común antes de guardar contenido
  const calefaccion = page.getByLabel("calefaccion", { exact: true });
  await expect(calefaccion).toBeVisible();
  await expect(calefaccion).toBeChecked();
  await page.getByLabel("ventilacion_mecanica", { exact: true }).check();
  await page.locator("#editor-title").fill(`${"😀".repeat(70)} `);
  await page.locator("#editor-desc").fill("Descripción breve no válida");
  const contentPutCountBeforeInvalid = putContentBody;
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(page.locator("#editor-title")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(page.locator("#editor-title").locator("xpath=..")).toContainText(
    /70/,
  );
  await expect(page.locator("#editor-desc")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(page.locator("#editor-desc").locator("xpath=..")).toContainText(
    /100/,
  );
  await expect(page.locator("#editor-title")).toBeFocused();
  expect(putContentBody).toBe(contentPutCountBeforeInvalid);

  await page.locator("#editor-title").fill("Título válido");
  await expect(page.locator("#editor-title")).not.toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(page.locator("#editor-title-error")).toHaveCount(0);
  await page.locator("#editor-desc").fill("d".repeat(100));
  await page.locator("#editor-address").fill(`${"😀".repeat(500)} `);
  await page.locator("#editor-area").fill("0");
  await page.locator("#editor-capacity").fill("0");
  await page.locator("#editor-rules").fill(`${"r".repeat(250)} `);
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  for (const [id, pattern] of [
    ["#editor-address", /500/],
    ["#editor-area", /superficie|área|area/i],
    ["#editor-capacity", /capacidad/i],
    ["#editor-rules", /250/],
  ] as const) {
    await expect(page.locator(id)).toHaveAttribute("aria-invalid", "true");
    const errorId = await page.locator(id).getAttribute("aria-describedby");
    expect(errorId).toBeTruthy();
    await expect(page.locator(`#${errorId}`)).toContainText(pattern);
  }
  expect(putContentBody).toBe(contentPutCountBeforeInvalid);
  await page.locator("#editor-address").fill("Av. Providencia 1234, Santiago");
  await page.locator("#editor-area").fill("35.5");
  await page.locator("#editor-capacity").fill("6");
  await page.locator("#editor-rules").fill("No fumar ni realizar fiestas.");
  await page.locator("#editor-title").fill("Oficina Providencia Remodelada");
  const validDescription =
    "Oficina luminosa y equipada, ideal para reuniones de trabajo colaborativo y jornadas productivas, con mobiliario cómodo, conectividad estable y espacios preparados para recibir equipos profesionales.";
  await page.locator("#editor-desc").fill(validDescription);
  rejectNextDraftUpdate = [
    { field: "region_code", code: "location_pair" },
    { field: "comuna_code", code: "location_pair" },
    { field: "attributes.wifi", code: "invalid_value" },
    { field: "attributes.puestos_trabajo", code: "invalid_value" },
    { field: "attributes.tipo_uso_oficina", code: "invalid_value" },
    { field: "attributes.climatizacion", code: "invalid_value" },
    { field: "attributes.not_in_schema", code: "invalid_value" },
  ];
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(
    page.getByText(
      "No se pudo guardar. Corrige los campos indicados y vuelve a intentar.",
      { exact: true },
    ),
  ).toBeVisible();
  for (const [selector, errorID] of [
    ["#editor-region", "editor-region-error"],
    ["#editor-comuna", "editor-comuna-error"],
  ] as const) {
    const field = page.locator(selector);
    await expect(field).toHaveAttribute("aria-invalid", "true");
    const describedBy = await field.getAttribute("aria-describedby");
    expect(describedBy?.split(/\s+/)).toContain(errorID);
    await expect(page.locator(`#${errorID}`)).toHaveText(
      "Selecciona una región y comuna compatibles.",
    );
  }
  const wifiAttribute = page.locator("#attr-editor-wifi");
  await expect(wifiAttribute).toHaveAttribute("aria-invalid", "true");
  const wifiErrorDescription =
    await wifiAttribute.getAttribute("aria-describedby");
  expect(wifiErrorDescription?.split(/\s+/)).toContain(
    "attr-editor-wifi-error",
  );
  await expect(page.locator("#attr-editor-wifi-error")).toHaveText(
    "Ingresa un valor válido para este atributo.",
  );
  const dynamicErrorControls = [
    {
      code: "puestos_trabajo",
      selector: "#attr-editor-puestos_trabajo",
    },
    {
      code: "tipo_uso_oficina",
      selector: "#attr-editor-tipo_uso_oficina",
    },
    {
      code: "climatizacion",
      selector: "#attr-editor-climatizacion-option-0",
    },
  ];
  for (const { code, selector } of dynamicErrorControls) {
    const field = page.locator(selector);
    const errorId = `attr-editor-${code}-error`;
    await expect(field).toHaveAttribute("aria-invalid", "true");
    const describedBy = await field.getAttribute("aria-describedby");
    expect(describedBy?.split(/\s+/)).toContain(errorId);
    await expect(page.locator(`#${errorId}`)).toHaveText(
      "Ingresa un valor válido para este atributo.",
    );
  }
  await page.locator("#attr-editor-puestos_trabajo").fill("4");
  await page.locator("#attr-editor-tipo_uso_oficina").selectOption("privada");
  await page.locator("#attr-editor-climatizacion-option-0").click();
  for (const { code, selector } of dynamicErrorControls) {
    await expect(page.locator(selector)).not.toHaveAttribute(
      "aria-invalid",
      "true",
    );
    await expect(page.locator(`#attr-editor-${code}-error`)).toHaveCount(0);
  }
  await wifiAttribute.uncheck();
  await expect(wifiAttribute).not.toHaveAttribute("aria-invalid", "true");
  await wifiAttribute.check();
  await page.locator("#editor-region").selectOption("05");
  await page.locator("#editor-region").selectOption("13");
  await page.locator("#editor-comuna").fill("Santiago");
  await expect(page.locator("#editor-region")).not.toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(page.locator("#editor-comuna")).not.toHaveAttribute(
    "aria-invalid",
    "true",
  );

  rejectNextDraftUpdate = [
    { field: "attributes.wifi", code: "unrecognized_validation_code" },
    { field: "attributes.not_in_schema", code: "invalid_value" },
  ];
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(
    page.getByText(
      "No se pudo guardar. Corrige los campos indicados y vuelve a intentar.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(wifiAttribute).not.toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#attr-editor-wifi-error")).toHaveCount(0);
  await expect(
    page.getByText(/unrecognized_validation_code|not_in_schema/, {
      exact: false,
    }),
  ).toHaveCount(0);

  rejectNextDraftUpdate = [
    { field: "attributes.sillas_disponibles", code: "required" },
  ];
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(
    page.getByText(
      "No se pudo guardar. Corrige los campos indicados y vuelve a intentar.",
      { exact: true },
    ),
  ).toBeVisible();

  rejectNextDraftUpdate = [{ field: "title", code: "max_length" }];
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(page.locator("#editor-title")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  const serverTitleErrorId = await page
    .locator("#editor-title")
    .getAttribute("aria-describedby");
  expect(serverTitleErrorId).toBe("editor-title-error");
  await expect(page.locator(`#${serverTitleErrorId}`)).toHaveText(
    "El título no puede superar 70 caracteres.",
  );
  expect(putContentBody!.title).toBe("Oficina Providencia Remodelada");
  await page.locator("#editor-title").fill("Oficina Providencia Verificada");
  await expect(page.locator("#editor-title")).not.toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await page.locator("#editor-title").fill("Oficina Providencia Remodelada");
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(
    page.getByText("Información actualizada exitosamente."),
  ).toBeVisible();
  expect(draftIfMatchHeader).toBe(syntheticDraftEtag(spaceId));
  expect(putContentBody).not.toBeNull();
  expect(putContentBody!.title).toBe("Oficina Providencia Remodelada");
  expect(putContentBody!.description).toBe(validDescription);
  expect(putContentBody!.area_m2).toBe(35.5);
  expect(putContentBody!.capacity).toBe(6);
  expect(putContentBody!.usage_rules).toBe("No fumar ni realizar fiestas.");
  expect(putContentBody!.base_price_clp).toBe(20000);
  expect(putContentBody!.rate_unit).toBe("hora");
  expect(putContentBody!.attribute_schema_version).toBe(1);
  expect(putContentBody!.attributes).toEqual({
    wifi: true,
    puestos_trabajo: 4,
    tipo_uso_oficina: "privada",
    climatizacion: [
      "aire_acondicionado",
      "calefaccion",
      "ventilacion_mecanica",
    ],
  });

  // 2. Pestaña Tarifas: navegar, actualizar tarifa base y consultar historial
  await page.getByRole("tab", { name: "Tarifas e historial" }).click();
  await expect(page.locator("#editor-price")).toHaveValue("20000");
  const tariffBeforeInvalid = putTariffBody;
  await page.locator("#editor-price").fill("5000");
  await page.getByRole("button", { name: "Actualizar tarifa" }).click();
  await expect(page.locator("#editor-price")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  const priceErrorId = await page
    .locator("#editor-price")
    .getAttribute("aria-describedby");
  expect(priceErrorId).toBeTruthy();
  await expect(page.locator(`#${priceErrorId}`)).toContainText(
    /entero mayor.*5\.000.*CLP.*hora.*día.*mes/i,
  );
  expect(putTariffBody).toBe(tariffBeforeInvalid);
  await page.locator("#editor-price").fill("5001");
  await expect(page.locator("#editor-price")).not.toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await expect(page.locator(`#${priceErrorId}`)).toHaveCount(0);
  await page.getByRole("button", { name: "Actualizar tarifa" }).click();
  await expect(
    page.getByText("Tarifa actualizada y registrada en el historial."),
  ).toBeVisible();
  expect(putTariffBody!.base_price).toBe(5001);
  expect(tariffIfMatchHeader).toBe(
    syntheticDraftEtag(spaceId, "2026-10-28T12:00:01Z"),
  );
  await expect(page.getByText("$5.001 CLP").first()).toBeVisible();

  // 3. Pestaña Horarios: navegar y guardar horarios
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Santiago",
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await expect(
    page.getByText(
      "Disponibilidad semanal y zona horaria guardadas con éxito.",
    ),
  ).toBeVisible();
  expect(putAvailabilityBody!.time_zone).toBe("America/Santiago");
  const scheduleExpectedEtag = syntheticDraftEtag(
    spaceId,
    "2026-10-28T12:00:01.500Z",
  );
  expect(availabilityIfMatchHeader).toBe(scheduleExpectedEtag);
  expect(weeklyHoursIfMatchHeader).toBe(scheduleExpectedEtag);

  // 4. Pestaña Fotografías: selección de archivos, carga y reordenamiento
  await page.getByRole("tab", { name: /Fotografías/ }).click();
  await expect(
    page.getByText(/entorno sintético|simuladas y sintéticas/i),
  ).toHaveCount(0);
  const imageInput = page.getByLabel("Subir imágenes", { exact: true });
  await expect(imageInput).toHaveAttribute(
    "accept",
    "image/jpeg,image/png,image/webp",
  );
  await expect(imageInput).toHaveAttribute("multiple", "");
  await expect(page.locator("#gallery-upload-help")).toContainText(
    "30 por espacio",
  );
  const tinyPng = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNQSlv1HwAEEgIyVuBIjQAAAABJRU5ErkJggg==",
    "base64",
  );
  await imageInput.setInputFiles({
    name: "foto-1.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect.poll(() => postGalleryCount).toBe(1);
  await expect(
    page.getByText("Lote de galería finalizado: 1 correcta.", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Portada", { exact: true })).toBeVisible();
  const firstPhotoImage = page.getByRole("img", {
    name: "Imagen 1 del espacio",
    exact: true,
  });
  await expect.poll(() => thumbnailGetCount).toBeGreaterThan(0);
  await expect(firstPhotoImage).toHaveAttribute("src", /^blob:/);
  expect(postGalleryCount).toBe(1);
  expect(galleryUploads[0].contentType).toMatch(
    /^multipart\/form-data; boundary=/i,
  );
  expect(galleryUploads[0].body).toContain('name="image"');
  expect(galleryUploads[0].body).toContain("foto-1.png");
  expect(galleryUploads[0].idempotencyKey).toMatch(/^[0-9a-f-]{36}$/i);

  const etagBeforeReplacement = currentSpaceEtag;
  await page.getByRole("button", { name: "Reemplazar imagen 1" }).click();
  await page.locator("#gallery-replace-input").setInputFiles({
    name: "foto-reemplazo.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect.poll(() => replacementAttempt).not.toBeNull();
  await expect(page.getByText("Foto reemplazada correctamente.")).toBeVisible();
  expect(replacementAttempt!.ifMatch).toBe(etagBeforeReplacement);
  expect(replacementAttempt!.idempotencyKey).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(replacementAttempt!.contentType).toMatch(
    /^multipart\/form-data; boundary=/i,
  );
  expect(replacementAttempt!.body).toContain('name="image"');
  expect(replacementAttempt!.body).toContain("foto-reemplazo.png");
  await page.getByRole("button", { name: "Reemplazar imagen 1" }).click();
  await page.locator("#gallery-replace-input").setInputFiles({
    name: "foto-reemplazo-pending.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText(
      "El reemplazo está pendiente de confirmación; la foto actual se conserva.",
      { exact: true },
    ),
  ).toBeVisible();
  expect(photos).toHaveLength(1);
  expect(photos[0]?.id).toBe(replacementPhotoId);
  const pendingReplacementKey = replacementAttempt!.idempotencyKey;
  expect(pendingReplacementKey).toMatch(/^[0-9a-f-]{36}$/i);
  expect(replacementAttempts).toHaveLength(2);
  await page
    .getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-pending.png",
    })
    .click();
  await expect(
    page.getByText(
      "El reemplazo continúa pendiente; la foto actual se conserva.",
      {
        exact: true,
      },
    ),
  ).toBeVisible();
  expect(replacementStatusLookupCount).toBe(1);
  expect(replacementStatusLookups.at(-1)).toBe(pendingReplacementKey);
  expect(replacementAttempts).toHaveLength(2);
  expect(photos[0]?.id).toBe(replacementPhotoId);
  await expect(
    page.getByRole("img", { name: "Imagen 1 del espacio", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Portada", { exact: true })).toBeVisible();

  replacementStatusState = "not_found";
  await page
    .getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-pending.png",
    })
    .click();
  await expect(
    page.getByRole("button", {
      name: "Reintentar reemplazo foto-reemplazo-pending.png",
    }),
  ).toBeVisible();
  expect(replacementStatusLookupCount).toBe(2);
  expect(replacementStatusLookups).toEqual([
    pendingReplacementKey,
    pendingReplacementKey,
  ]);
  expect(replacementAttempts).toHaveLength(2);
  expect(photos[0]?.id).toBe(replacementPhotoId);
  await page
    .getByRole("button", {
      name: "Reintentar reemplazo foto-reemplazo-pending.png",
    })
    .click();
  await expect(page.getByText("Foto reemplazada correctamente.")).toBeVisible();
  expect(replacementAttempts).toHaveLength(3);
  expect(replacementAttempts[2]?.idempotencyKey).toBe(pendingReplacementKey);
  expect(replacementAttempts[2]?.ifMatch).toBe(replacementAttempts[1]?.ifMatch);
  expect(replacementAttempts[2]?.body).toContain("foto-reemplazo-pending.png");
  expect(photos).toHaveLength(1);
  expect(photos[0]?.id).toBe(pendingReplacementPhotoId);
  await expect.poll(() => thumbnailGetCount).toBeGreaterThan(1);

  // A fresh pending replacement exercises lookup ready separately from retry.
  pendingReplacementResponded = false;
  replacementStatusState = "pending";
  const readyLookupStart = replacementStatusLookupCount;
  const readyAttemptStart = replacementAttempts.length;
  const blobBeforeReady = await firstPhotoImage.getAttribute("src");
  await page.getByRole("button", { name: "Reemplazar imagen 1" }).click();
  await page.locator("#gallery-replace-input").setInputFiles({
    name: "foto-reemplazo-pending.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText(
      "El reemplazo está pendiente de confirmación; la foto actual se conserva.",
      { exact: true },
    ),
  ).toBeVisible();
  const readyReplacementKey = replacementAttempt!.idempotencyKey;
  expect(readyReplacementKey).not.toBe(pendingReplacementKey);
  expect(replacementAttempts).toHaveLength(readyAttemptStart + 1);
  replacementStatusState = "error";
  await page
    .getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-pending.png",
    })
    .click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "API request failed (503: unavailable)" }),
  ).toBeVisible();
  expect(replacementStatusLookupCount).toBe(readyLookupStart + 1);
  expect(replacementStatusLookups.at(-1)).toBe(readyReplacementKey);
  expect(replacementAttempts).toHaveLength(readyAttemptStart + 1);
  await expect(firstPhotoImage).toHaveAttribute("src", blobBeforeReady!);
  await expect(page.getByText("Portada", { exact: true })).toBeVisible();
  const detailReadsBeforeReady = detailReloads.length;
  const galleryReadsBeforeReady = galleryReloads.length;
  await expect(
    page.getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-pending.png",
    }),
  ).toBeEnabled();
  replacementStatusState = "ready";
  await page
    .getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-pending.png",
    })
    .click();
  await expect(
    page.getByText("Foto reemplazada correctamente.", { exact: true }),
  ).toBeVisible();
  expect(replacementStatusLookupCount).toBe(readyLookupStart + 2);
  expect(replacementStatusLookups.slice(-2)).toEqual([
    readyReplacementKey,
    readyReplacementKey,
  ]);
  expect(replacementAttempts).toHaveLength(readyAttemptStart + 1);
  expect(detailReloads).toHaveLength(detailReadsBeforeReady + 1);
  // Lookup reconciles photos in the facade; selectListing then reloads detail and photos.
  expect(galleryReloads).toHaveLength(galleryReadsBeforeReady + 2);
  expect(detailReloads.at(-1)).toEqual({
    etag: currentSpaceEtag,
    cover: readyReplacementPhotoId,
  });
  expect(galleryReloads.at(-1)).toEqual([readyReplacementPhotoId]);
  expect(photos[0]?.id).toBe(readyReplacementPhotoId);
  expect(currentSpace.cover_photo_id).toBe(readyReplacementPhotoId);
  await expect(firstPhotoImage).toBeVisible();
  await expect(firstPhotoImage).not.toHaveAttribute("src", blobBeforeReady!);
  await expect(page.getByText("Portada", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-pending.png",
    }),
  ).toHaveCount(0);

  const oldPhotoDuringCleanupId = photos[0]?.id;
  const replacementCountBeforeCleanup = replacementAttempts.length;
  await page.getByRole("button", { name: "Reemplazar imagen 1" }).click();
  await page.locator("#gallery-replace-input").setInputFiles({
    name: "foto-reemplazo-deleting.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText(
      "El reemplazo está pendiente de confirmación; la foto actual se conserva.",
      { exact: true },
    ),
  ).toBeVisible();
  const deletingReplacementKey = replacementAttempts.at(-1)!.idempotencyKey;
  expect(replacementAttempts).toHaveLength(replacementCountBeforeCleanup + 1);
  expect(replacementAttempts.at(-1)?.targetPhotoId).toBe(
    oldPhotoDuringCleanupId,
  );
  replacementStatusState = "deleting";
  await page
    .getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-deleting.png",
    })
    .click();
  await expect(
    page.getByText(
      "El reemplazo no se completó; el archivo reservado está en limpieza. La foto actual se conserva. Elige nuevamente una imagen para iniciar otro intento.",
      { exact: true },
    ),
  ).toBeVisible();
  expect(replacementStatusLookups.at(-1)).toBe(deletingReplacementKey);
  expect(replacementAttempts).toHaveLength(replacementCountBeforeCleanup + 1);
  expect(photos[0]?.id).toBe(oldPhotoDuringCleanupId);
  await expect(
    page.getByRole("button", {
      name: "Revisar estado del reemplazo foto-reemplazo-deleting.png",
    }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Reemplazar imagen 1" }),
  ).toBeEnabled();

  const refreshedEtag = currentSpaceEtag;
  await page.getByRole("tab", { name: "Información" }).click();
  await page.locator("#editor-title").fill("Oficina con foto renovada");
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(
    page.getByText("Información actualizada exitosamente."),
  ).toBeVisible();
  expect(draftIfMatchHeader).toBe(refreshedEtag);
  await page.getByRole("tab", { name: /Fotografías/ }).click();

  await imageInput.setInputFiles({
    name: "retry-me.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: /Error al subir retry-me\.png/ }),
  ).toBeVisible();
  const retryButton = page.getByRole("button", {
    name: "Reintentar retry-me.png",
  });
  await expect(retryButton).toBeVisible();
  expect(postGalleryCount).toBe(2);
  expect(photos).toHaveLength(1);
  const firstFailedAttempt = galleryUploads[1]!;
  expect(firstFailedAttempt.idempotencyKey).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(firstFailedAttempt.body).toContain('name="image"');
  expect(firstFailedAttempt.body).toContain("retry-me.png");
  expect(firstFailedAttempt.imagePayload).toBeTruthy();
  await retryButton.click();
  await expect(
    page.getByText("Carga completada: retry-me.png", { exact: true }),
  ).toBeVisible();
  await expect.poll(() => postGalleryCount).toBe(3);
  expect(photos).toHaveLength(2);
  const retriedAttempt = galleryUploads[2]!;
  expect(retriedAttempt.idempotencyKey).toBe(firstFailedAttempt.idempotencyKey);
  expect(retriedAttempt.body).toContain('name="image"');
  expect(retriedAttempt.body).toContain("retry-me.png");
  expect(retriedAttempt.imagePayload).toBe(firstFailedAttempt.imagePayload);

  await imageInput.setInputFiles({
    name: "foto-2.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(page.getByText("#2", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("img", { name: "Imagen 2 del espacio", exact: true }),
  ).toHaveCount(0);
  expect(secondPhotoThumbnailGetCount).toBe(0);
  expect(postGalleryCount).toBe(4);
  expect(galleryUploads[3].contentType).toMatch(
    /^multipart\/form-data; boundary=/i,
  );
  expect(galleryUploads[3].body).toContain('name="image"');
  expect(galleryUploads[3].idempotencyKey).toMatch(/^[0-9a-f-]{36}$/i);

  // Cola acotada: dos solicitudes activas y el resto en espera.
  deferGalleryPosts = true;
  await imageInput.setInputFiles(
    Array.from({ length: 3 }, (_, index) => ({
      name: `cola-${index + 1}.png`,
      mimeType: "image/png",
      buffer: tinyPng,
    })),
  );
  await expect(
    page.getByText("Subiendo cola-1.png", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("En cola cola-3.png", { exact: true }),
  ).toBeVisible();
  await expect.poll(() => activeGalleryPosts).toBe(2);
  expect(maxActiveGalleryPosts).toBe(2);
  const cancelQueuedUpload = page.getByRole("button", {
    name: "Cancelar carga cola-3.png",
  });
  await expect(cancelQueuedUpload).toBeVisible();
  await cancelQueuedUpload.click();
  await expect(
    page.getByText("Carga cancelada: cola-3.png", { exact: true }),
  ).toBeVisible();
  expect(postGalleryCount).toBe(6);
  for (let released = 0; released < 2; released += 1) {
    await expect.poll(() => releaseGalleryPosts.length).toBeGreaterThan(0);
    releaseGalleryPosts.shift()!();
  }
  await expect(
    page.getByText("Carga completada: cola-1.png", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Carga completada: cola-2.png", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Lote de galería finalizado: 2 correctas.", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Carga completada: cola-3.png", { exact: true }),
  ).toHaveCount(0);
  expect(postGalleryCount).toBe(6);
  deferGalleryPosts = false;

  // Cancelar una carga ya enviada debe parar el fetch local y reconciliar con el estado autoritativo.
  deferGalleryPosts = true;
  await imageInput.setInputFiles({
    name: "cancel-in-flight.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText("Subiendo cancel-in-flight.png", { exact: true }),
  ).toBeVisible();
  await expect.poll(() => activeGalleryPosts).toBe(1);
  const cancelInFlightButton = page.getByRole("button", {
    name: "Cancelar carga cancel-in-flight.png",
  });
  await expect(cancelInFlightButton).toBeVisible();
  const photosBeforeCancel = photos.length;
  await cancelInFlightButton.click();
  await expect(
    page.getByText("Carga pendiente de confirmación: cancel-in-flight.png", {
      exact: true,
    }),
  ).toBeVisible();
  await expect.poll(() => cancelledUploadStatusLookupCount).toBe(1);
  expect(postGalleryCount).toBe(7);
  await expect.poll(() => releaseGalleryPosts.length).toBe(1);
  cancelledUploadState = "ready";
  releaseGalleryPosts.shift()!();
  await expect.poll(() => activeGalleryPosts).toBe(0);
  await expect.poll(() => photos.length).toBe(photosBeforeCancel + 1);
  await page
    .getByRole("button", { name: "Revisar estado cancel-in-flight.png" })
    .click();
  await expect(
    page.getByText("Carga completada: cancel-in-flight.png", { exact: true }),
  ).toBeVisible();
  await expect.poll(() => cancelledUploadStatusLookupCount).toBe(2);
  expect(postGalleryCount).toBe(7);
  const firstDeleteIfMatch = currentSpaceEtag;
  await page.getByRole("button", { name: "Eliminar foto" }).last().click();
  await expect(page.getByText("Foto eliminada de la galería.")).toBeVisible();
  expect(galleryDeleteIfMatchHeaders[0]).toBe(firstDeleteIfMatch);
  expect(currentSpaceEtag).toBe(galleryDeleteResponseEtags[0]);
  expect(deletedPhotoId).toBe(cancelledUploadPhotoId);
  await expect.poll(() => photos.length).toBe(photosBeforeCancel);

  const firstCancelledUploadKey = cancelledUploadKey;
  expect(firstCancelledUploadKey).toMatch(/^[0-9a-f-]{36}$/i);
  // Si la solicitud abortada aún no tiene reserva, revisar estado no debe repetir el POST.
  cancelledUploadState = "pending";
  await imageInput.setInputFiles({
    name: "cancel-not-found.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText("Subiendo cancel-not-found.png", { exact: true }),
  ).toBeVisible();
  await expect.poll(() => activeGalleryPosts).toBe(1);
  await page
    .getByRole("button", { name: "Cancelar carga cancel-not-found.png" })
    .click();
  await expect(
    page.getByText("Carga pendiente de confirmación: cancel-not-found.png", {
      exact: true,
    }),
  ).toBeVisible();
  await expect.poll(() => cancelledUploadStatusLookupCount).toBe(3);
  const notFoundCancelledUploadKey = cancelledUploadKey;
  expect(notFoundCancelledUploadKey).toMatch(/^[0-9a-f-]{36}$/i);
  cancelledUploadState = "not_found";
  await page
    .getByRole("button", { name: "Revisar estado cancel-not-found.png" })
    .click();
  await expect(
    page.getByRole("button", { name: "Reintentar cancel-not-found.png" }),
  ).toBeVisible();
  await expect.poll(() => cancelledUploadStatusLookupCount).toBe(4);
  expect(postGalleryCount).toBe(8);
  await expect.poll(() => releaseGalleryPosts.length).toBe(1);
  releaseGalleryPosts.shift()!();
  await expect.poll(() => activeGalleryPosts).toBe(0);
  expect(postGalleryCount).toBe(8);
  deferGalleryPosts = false;

  const canceledNotFoundRetry = page.getByRole("button", {
    name: "Reintentar cancel-not-found.png",
  });
  await canceledNotFoundRetry.click();
  await expect(canceledNotFoundRetry).toBeVisible();
  await expect.poll(() => postGalleryCount).toBe(9);
  const canceledNotFoundAttempts = galleryUploads.filter((upload) =>
    upload.body.includes("cancel-not-found.png"),
  );
  expect(canceledNotFoundAttempts).toHaveLength(2);
  expect(
    canceledNotFoundAttempts.map((upload) => upload.idempotencyKey),
  ).toEqual([notFoundCancelledUploadKey, notFoundCancelledUploadKey]);

  const postCountBeforeMismatchedExtension = postGalleryCount;
  await imageInput.setInputFiles({
    name: "foto-mal-etiquetada.jpg",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText("Usa imágenes JPG, PNG o WebP de hasta 10 MiB cada una.", {
      exact: true,
    }),
  ).toBeVisible();
  expect(postGalleryCount).toBe(postCountBeforeMismatchedExtension);

  page.on("response", (response) => {
    const match = response
      .url()
      .match(new RegExp(`/api/backend/spaces/${spaceId}/gallery/([^/?]+)$`));
    if (
      response.request().method() === "DELETE" &&
      response.status() < 400 &&
      match
    ) {
      deletedPhotoId = match[1];
    }
  });

  // Derive the moved photo and target from the rendered gallery order.
  const visiblePhotoIds = async () => {
    const visibleCards = await page
      .getByRole("button", { name: "Eliminar foto" })
      .count();
    return photos.slice(0, visibleCards).map((photo) => String(photo.id));
  };
  const moveUpButton = page.locator(
    "button[aria-label='Mover foto antes']:not([disabled])",
  );
  const orderBeforeReorder = await visiblePhotoIds();
  expect(orderBeforeReorder.length).toBeGreaterThan(1);
  const movedPhotoId = orderBeforeReorder[1]!;
  const expectedTargetPosition = 1;
  const reorderStart = reorderPutCount;
  const revisionBeforeReorder = currentSpaceEtag;
  await moveUpButton.first().click();
  await expect(
    page.getByText("Orden de fotografías actualizado."),
  ).toBeVisible();
  expect(reorderPutCount).toBe(reorderStart + 1);
  expect(reorderRequests[reorderStart]).toEqual({
    photoId: movedPhotoId,
    position: expectedTargetPosition,
    ifMatch: revisionBeforeReorder,
  });
  await expect
    .poll(visiblePhotoIds)
    .toEqual([
      movedPhotoId,
      ...orderBeforeReorder.filter((id) => id !== movedPhotoId),
    ]);
  const revisionAfterReorder = currentSpaceEtag;
  const requestsBeforeStaleReorder = reorderPutCount;
  const orderBeforeStaleReorder = await visiblePhotoIds();
  rejectNextReorderAsStale = true;
  await moveUpButton.first().click();
  await expect(page.getByRole("alert").last()).toBeVisible();
  expect(reorderPutCount).toBe(requestsBeforeStaleReorder + 1);
  expect(reorderRequests[requestsBeforeStaleReorder]?.ifMatch).toBe(
    revisionAfterReorder,
  );
  expect(currentSpaceEtag).toBe(revisionAfterReorder);
  await expect.poll(visiblePhotoIds).toEqual(orderBeforeStaleReorder);

  // Quitar la foto real que quedó primera tras el reordenamiento
  await page.getByRole("button", { name: "Eliminar foto" }).first().click();
  await expect(page.getByText("Foto eliminada de la galería.")).toBeVisible();
  expect(galleryDeleteIfMatchHeaders[1]).toBe(revisionAfterReorder);
  expect(currentSpaceEtag).toBe(galleryDeleteResponseEtags[1]);
  await expect(page.getByRole("button", { name: "Eliminar foto" })).toHaveCount(
    4,
  );
  expect(deletedPhotoId).toBe("00000000-0000-4000-8000-000000000003");

  // Límite actual: el exceso se rechaza antes de enviar POST a la galería.
  const galleryLimitError = page.getByText(
    "El límite actual es de 30 imágenes por espacio.",
    { exact: true },
  );
  const postCountBeforeOverflow = postGalleryCount;
  await imageInput.setInputFiles(
    Array.from({ length: 27 }, (_, index) => ({
      name: `overflow-${index + 1}.png`,
      mimeType: "image/png",
      buffer: tinyPng,
    })),
  );
  await expect(galleryLimitError).toBeVisible();
  expect(postGalleryCount).toBe(postCountBeforeOverflow);

  await imageInput.setInputFiles(
    Array.from({ length: 26 }, (_, index) => ({
      name: `limite-${index + 1}.png`,
      mimeType: "image/png",
      buffer: tinyPng,
    })),
  );
  await expect(
    page.getByText("Lote de galería finalizado: 26 correctas.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect.poll(() => postGalleryCount).toBe(postCountBeforeOverflow + 26);
  await expect(page.getByRole("button", { name: "Eliminar foto" })).toHaveCount(
    30,
  );
  await expect(imageInput).toBeDisabled();

  // Un reintento idempotente pendiente no cuenta como completado ni ocupa una ranura.
  const postCountBeforePending = postGalleryCount;
  await page.getByRole("button", { name: "Eliminar foto" }).first().click();
  await expect(page.getByText("Foto eliminada de la galería.")).toBeVisible();
  expect(galleryDeleteIfMatchHeaders[2]).toBe(galleryDeleteResponseEtags[1]);
  expect(currentSpaceEtag).toBe(galleryDeleteResponseEtags[2]);
  await expect(page.getByRole("button", { name: "Eliminar foto" })).toHaveCount(
    29,
  );
  await expect(imageInput).toBeEnabled();
  await imageInput.setInputFiles({
    name: "pending-me.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText("Carga pendiente de confirmación: pending-me.png", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "Lote de galería finalizado: 0 correctas; 1 pendiente de confirmación.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect.poll(() => postGalleryCount).toBe(postCountBeforePending + 1);
  expect(photos).toHaveLength(29);
  const pendingUpload = galleryUploads.at(-1)!;
  const pendingRetryButton = page.getByRole("button", {
    name: "Revisar estado pending-me.png",
  });
  await expect(pendingRetryButton).toBeVisible();
  await pendingRetryButton.click();
  await expect(
    page.getByText("Carga pendiente de confirmación: pending-me.png", {
      exact: true,
    }),
  ).toBeVisible();
  expect(postGalleryCount).toBe(postCountBeforePending + 1);
  expect(galleryStatusLookups).toEqual([
    firstCancelledUploadKey,
    firstCancelledUploadKey,
    cancelledUploadKey,
    cancelledUploadKey,
    pendingUpload.idempotencyKey,
  ]);
  expect(photos).toHaveLength(29);
  await pendingRetryButton.click();
  const completedStatus = page.getByText("Carga completada: pending-me.png", {
    exact: true,
  });
  await expect(completedStatus).toHaveCount(1);
  await expect(completedStatus).toBeVisible();
  expect(postGalleryCount).toBe(postCountBeforePending + 1);
  expect(galleryStatusLookups).toEqual([
    firstCancelledUploadKey,
    firstCancelledUploadKey,
    cancelledUploadKey,
    cancelledUploadKey,
    pendingUpload.idempotencyKey,
    pendingUpload.idempotencyKey,
  ]);
  expect(photos).toHaveLength(30);
  await expect(imageInput).toBeDisabled();

  // 5. Publicación global del espacio con PUT /spaces/{id}/publication
  const publishButton = page.getByRole("button", { name: "Publicar" });
  await expect(publishButton).toBeEnabled();
  await publishButton.click();

  await expect(
    page.getByText(
      "¡El espacio ha sido publicado exitosamente y está activo en el catálogo!",
    ),
  ).toBeVisible();
  expect(putPublicationBody!.state).toBe("activa");
  await expect(page.getByText("Publicado").first()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Retirar publicación" }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Información" }).click();
  const postPublishEtag = currentSpaceEtag;
  const contentSaveButton = page.getByRole("button", {
    name: "Guardar contenido",
  });
  await page.locator("#editor-title").fill("Contenido publicado uno");
  await contentSaveButton.click();
  await expect(
    page.getByText("Información actualizada exitosamente."),
  ).toBeVisible();
  expect(contentIfMatchHeaders[0]).toBe(postPublishEtag);
  expect(publishedContentSaveCount).toBe(1);

  await page.locator("#editor-title").fill("Contenido publicado dos");
  await contentSaveButton.click();
  await expect(
    page.getByText("Información actualizada exitosamente."),
  ).toBeVisible();
  expect(contentIfMatchHeaders[1]).toBe(contentResponseEtags[0]);
  expect(publishedContentSaveCount).toBe(2);

  await page.locator("#editor-title").fill("Contenido con conflicto");
  await contentSaveButton.click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "API request failed (412: precondition_failed)" }),
  ).toBeVisible();
  expect(publishedContentSaveCount).toBe(3);
  expect(contentIfMatchHeaders).toHaveLength(3);
});

test("@phase7 @controlled una carga inicial pendiente conserva la intención de portada al quedar lista", async ({
  page,
}) => {
  const spaceId = "00000000-0000-4000-8000-000000000143";
  const photoId = "00000000-0000-4000-8000-000000000144";
  const tinyPng = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNQSlv1HwAEEgIyVuBIjQAAAABJRU5ErkJggg==",
    "base64",
  );
  const currentSpace = {
    id: spaceId,
    category_code: "oficina",
    category_name: "Oficinas y Cowork",
    title: "Espacio sintético de carga",
    description:
      "Espacio local de prueba para reconciliar una carga pendiente.",
    area_m2: 25,
    capacity: 4,
    usage_rules: "Mantener el espacio limpio y ordenado.",
    rate_unit: "hora",
    base_price_clp: 15000,
    address: "Avenida Sintética 100",
    state: "borrador",
    time_zone: "America/Santiago",
    region_code: "13",
    comuna_code: "Providencia",
    attribute_schema_version: 1,
    attributes: {},
    cover_photo_id: null as string | null,
  };
  const tariff = {
    space_id: spaceId,
    version: 1,
    rate_unit: "hora",
    base_price: 15000,
    currency: "CLP",
    created_at: "2026-10-28T12:00:00Z",
  };
  const photos: Array<Record<string, unknown>> = [];
  let uploadCount = 0;
  let uploadKey: string | null = null;
  let statusLookupCount = 0;
  let coverPhotoID: string | null = null;

  await mockGateway(page, async (pathname, method, request) => {
    const authRes = handleAuthRequests(pathname, method);
    if (authRes) return authRes;

    if (pathname === `/api/backend/spaces/${spaceId}` && method === "GET") {
      return {
        status: 200,
        headers: { etag: syntheticDraftEtag(spaceId) },
        body: currentSpace,
      };
    }
    if (
      pathname === "/api/backend/spaces/categories/oficina/attributes/1" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          category_code: "oficina",
          schema_version: 1,
          attributes: [],
        },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariff` &&
      method === "GET"
    ) {
      return { status: 200, body: tariff };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariffs` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: [tariff] } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
        headers: { etag: syntheticDraftEtag(spaceId) },
        body: {
          data: {
            space_id: spaceId,
            enabled: true,
            time_zone: "America/Santiago",
            days: [],
          },
        },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: photos } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "POST"
    ) {
      uploadCount += 1;
      uploadKey = request.headers()["idempotency-key"] ?? null;
      return {
        status: 202,
        headers: { "retry-after": "2" },
        body: {
          photo: { id: photoId, space_id: spaceId, state: "pending" },
          reused: false,
        },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery/upload-status` &&
      method === "GET"
    ) {
      if (request.headers()["idempotency-key"] !== uploadKey) {
        return apiError(404, "not_found");
      }
      statusLookupCount += 1;
      if (statusLookupCount === 1) {
        return {
          status: 200,
          body: { photo_id: photoId, state: "pending" },
        };
      }
      photos.splice(0, photos.length, {
        id: photoId,
        space_id: spaceId,
        mime_type: "image/webp",
        sha256: "b".repeat(64),
        size_bytes: 128,
        created_at: "2026-10-28T12:00:00Z",
        is_cover: false,
        visible: true,
        position: 1,
        variants: ["thumbnail", "display"].map((name) => ({
          name,
          url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
          mime_type: "image/webp",
          size_bytes: 128,
          sha256: "b".repeat(64),
          width: 32,
          height: 32,
        })),
      });
      return { status: 200, body: { photo_id: photoId, state: "ready" } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/cover` &&
      method === "PUT"
    ) {
      const body = JSON.parse(request.postData() ?? "{}") as {
        photo_id?: string;
      };
      coverPhotoID = body.photo_id ?? null;
      currentSpace.cover_photo_id = coverPhotoID;
      return { status: 204 };
    }
    if (pathname.endsWith("/variants/thumbnail") && method === "GET") {
      return {
        status: 200,
        headers: { "content-type": "image/webp" },
        body: Buffer.from([]),
      };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, `/anfitrion/espacios/${spaceId}/editar`);
  await page.getByRole("tab", { name: /Fotografías/ }).click();
  const imageInput = page.getByLabel("Subir imágenes", { exact: true });
  await imageInput.setInputFiles({
    name: "pending-cover.png",
    mimeType: "image/png",
    buffer: tinyPng,
  });
  await expect(
    page.getByText("Carga pendiente de confirmación: pending-cover.png", {
      exact: true,
    }),
  ).toBeVisible();
  expect(uploadCount).toBe(1);
  const reviewButton = page.getByRole("button", {
    name: "Revisar estado pending-cover.png",
  });
  await reviewButton.click();
  await expect(
    page.getByText("Carga pendiente de confirmación: pending-cover.png", {
      exact: true,
    }),
  ).toBeVisible();
  expect(uploadCount).toBe(1);
  await reviewButton.click();
  await expect(
    page.getByText("Carga completada: pending-cover.png", { exact: true }),
  ).toBeVisible();
  expect(statusLookupCount).toBe(2);
  expect(uploadCount).toBe(1);
  expect(coverPhotoID).toBe(photoId);
  expect(currentSpace.cover_photo_id).toBe(photoId);
});

test("@phase7 @a11y auditoría de accesibilidad Axe y verificación estricta de no desbordamiento horizontal en desktop (1365x900) y mobile (390x844)", async ({
  page,
}) => {
  const spaceId = "test-space-a11y";
  const testSpace = {
    id: spaceId,
    category_code: "oficina",
    category_name: "Oficinas y Cowork",
    title: "Espacio Accesible de Ensayo",
    description: "Oficina de prueba para validación a11y completa sin barreras",
    area_m2: 45,
    capacity: 8,
    usage_rules: "Reglas de convivencia claras y accesibles.",
    rate_unit: "hora",
    base_price_clp: 18000,
    address: "Avenida Libertador Bernardo O'Higgins 1000",
    state: "borrador",
    time_zone: "America/Santiago",
    region_code: "13",
    comuna_code: "Santiago",
    attribute_schema_version: 1,
    attributes: { has_wifi: true },
    cover_photo_id: null,
  };

  await mockGateway(page, async (pathname, method) => {
    const authRes = handleAuthRequests(pathname, method);
    if (authRes) return authRes;

    if (pathname === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [testSpace] } };
    }
    if (pathname === "/api/backend/spaces/categories" && method === "GET") {
      return {
        status: 200,
        body: { items: [{ code: "oficina", name: "Oficinas y Cowork" }] },
      };
    }
    if (
      pathname.startsWith(
        "/api/backend/spaces/categories/oficina/attributes",
      ) &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          category_code: "oficina",
          schema_version: 1,
          attributes: [
            {
              code: "has_wifi",
              label: "Wi-Fi disponible",
              type: "boolean",
              required: false,
              order: 1,
              filter_candidate: true,
            },
          ],
        },
      };
    }
    if (pathname === `/api/backend/spaces/${spaceId}` && method === "GET") {
      return {
        status: 200,
        headers: { etag: syntheticDraftEtag(spaceId) },
        body: testSpace,
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariff` &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          space_id: spaceId,
          version: 1,
          rate_unit: "hora",
          base_price: 18000,
          currency: "CLP",
          created_at: "2026-10-25T10:00:00Z",
        },
      };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/tariffs` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "GET"
    ) {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
        headers: { etag: syntheticDraftEtag(spaceId) },
        body: {
          data: {
            space_id: spaceId,
            enabled: true,
            time_zone: "America/Santiago",
            days: [],
          },
        },
      };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios");

  const targetRoutes = [
    "/anfitrion/espacios",
    "/anfitrion/espacios/nuevo",
    `/anfitrion/espacios/${spaceId}/editar`,
  ];

  for (const route of targetRoutes) {
    await page.goto(route);
    await page.waitForLoadState("networkidle");

    // Verificación de encabezado principal único h1
    await expect(page.locator("h1")).toHaveCount(1);

    for (const viewport of [
      { width: 1365, height: 900 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      const dimensions = await page.evaluate(() => ({
        width: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      // Verificación estricta de ausencia de desbordamiento horizontal
      expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.width);

      // Auditoría Axe estricta
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

test("@phase7 @controlled session-boundary: el formulario de nuevo espacio aísla el estado local y se reinicia cuando la sesión cambia de Actor A a Actor B", async ({
  page,
}) => {
  const accountIdA = "770e8400-e29b-41d4-a716-44665544000a";
  const accountIdB = "770e8400-e29b-41d4-a716-44665544000b";
  const fixedExpiry = new Date(Date.now() + 3600_000).toISOString();

  let currentActorSession = {
    account_id: accountIdA,
    roles: ["arrendatario", "anfitrion"],
    capabilities: [] as string[],
    restricted_mode: false,
    expires_at: fixedExpiry,
  };

  const categories = [{ code: "oficina", name: "Oficinas y Cowork" }];
  const categorySchema = {
    category_code: "oficina",
    schema_version: 1,
    attributes: [
      {
        code: "wifi",
        label: "Wi-Fi",
        type: "boolean",
        order: 1,
      },
    ],
  };

  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: currentActorSession,
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: currentActorSession,
      };
    }
    if (pathname === "/api/backend/spaces/categories" && method === "GET") {
      return {
        status: 200,
        body: { items: categories },
      };
    }
    if (
      (pathname === "/api/backend/spaces/categories/oficina/attributes" ||
        pathname === "/api/backend/spaces/categories/oficina/attributes/1") &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: categorySchema,
      };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await expect(page.locator("h1")).toHaveText("Publicar un nuevo espacio");

  // Llenar campos en Paso 1 como Actor A sin crear el espacio
  await expect(page.locator("#wizard-title")).toBeVisible();
  await page.locator("#wizard-title").fill("Oficina Confidencial de Actor A");
  await page
    .locator("#wizard-description")
    .fill("Descripción privada anfitrión A");
  await expect(page.locator("#wizard-title")).toHaveValue(
    "Oficina Confidencial de Actor A",
  );

  // Cambiar sesión a Actor B con DTO de 5 campos y expiry coherente
  currentActorSession = {
    account_id: accountIdB,
    roles: ["arrendatario", "anfitrion"],
    capabilities: [],
    restricted_mode: false,
    expires_at: fixedExpiry,
  };

  // Disparar evento de focus para forzar verificación de sesión
  const sessionCheckPromise = page.waitForResponse(
    (res) =>
      res.url().includes("/api/backend/auth/session") &&
      res.request().method() === "GET",
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const sessionRes = await sessionCheckPromise;
  expect(sessionRes.status()).toBe(200);
  const sessionBody = (await sessionRes.json()) as Record<string, unknown>;
  expect(sessionBody.account_id).toBe(accountIdB);

  // El estado local del formulario de borrador debe vaciarse sin dejar rastros de Actor A
  await expect(page.locator("#wizard-title")).toHaveValue("");
  await expect(page.locator("#wizard-description")).toHaveValue("");
});

test("@phase7 @controlled quincho sin parrilla: pone solo el contador en cero y preserva los demás atributos", async ({
  page,
}) => {
  const requests: string[] = [];
  const categories = [{ code: "quincho", name: "Quinchos" }];
  const categorySchema = {
    category_code: "quincho",
    schema_version: 1,
    attributes: [
      {
        code: "tipo_parrilla",
        label: "Tipo de parrilla",
        type: "enum",
        order: 1,
        options: ["carbon", "gas", "electrica", "lena", "sin_parrilla"],
        filter_candidate: false,
      },
      {
        code: "techado",
        label: "Techado",
        type: "boolean",
        order: 2,
        filter_candidate: false,
      },
      {
        code: "asientos_disponibles",
        label: "Asientos disponibles",
        type: "integer",
        order: 3,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "parrillas_disponibles",
        label: "Parrillas disponibles",
        type: "integer",
        order: 4,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "mesas_disponibles",
        label: "Mesas disponibles",
        type: "integer",
        order: 5,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
      {
        code: "lavaplatos",
        label: "Lavaplatos",
        type: "boolean",
        order: 6,
        filter_candidate: false,
      },
      {
        code: "refrigerador",
        label: "Refrigerador",
        type: "boolean",
        order: 7,
        filter_candidate: false,
      },
      {
        code: "utensilios_incluidos",
        label: "Utensilios incluidos",
        type: "boolean",
        order: 8,
        filter_candidate: false,
      },
      {
        code: "agua_potable",
        label: "Agua potable",
        type: "boolean",
        order: 9,
        filter_candidate: false,
      },
      {
        code: "energia_electrica",
        label: "Energía eléctrica",
        type: "boolean",
        order: 10,
        filter_candidate: false,
      },
      {
        code: "banos_disponibles",
        label: "Baños disponibles",
        type: "integer",
        order: 11,
        minimum: 0,
        maximum: 2147483647,
        filter_candidate: false,
      },
    ],
  };

  await mockGateway(page, async (pathname, method) => {
    requests.push(`${method} ${pathname}`);
    const auth = handleAuthRequests(pathname, method);
    if (auth) return auth;
    if (pathname === "/api/backend/spaces/categories" && method === "GET")
      return { status: 200, body: { items: categories } };
    if (
      pathname === "/api/backend/spaces/categories/quincho/attributes" &&
      method === "GET"
    )
      return { status: 200, body: categorySchema };
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await page.locator("#wizard-category").selectOption("quincho");
  await page.locator("#wizard-title").fill("Quincho de prueba");
  await page
    .locator("#wizard-description")
    .fill(
      "Quincho amplio para reuniones familiares y celebraciones, con instalaciones cómodas, buena iluminación y un espacio agradable para compartir durante todo el día.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();

  const techado = page.getByLabel("Techado");
  await techado.check();
  await page.getByLabel("Tipo de parrilla").selectOption("carbon");
  const parrillas = page.getByLabel("Parrillas disponibles");
  await parrillas.fill("2");
  await page.getByLabel("Tipo de parrilla").selectOption("sin_parrilla");

  await expect(parrillas).toHaveValue("0");
  await expect(techado).toBeChecked();
  expect(requests).not.toContain("POST /api/backend/spaces");
});

test("@phase7 @controlled estacionamiento v1: renderiza el perfil local sin crear un espacio", async ({
  page,
}) => {
  const requests: string[] = [];
  const categories = [{ code: "estacionamiento", name: "Estacionamiento" }];
  const categorySchema = {
    category_code: "estacionamiento",
    schema_version: 1,
    attributes: [
      {
        code: "largo_plaza_m",
        label: "Largo de plaza",
        type: "number",
        order: 1,
        minimum: 0.01,
        maximum: 99999999.99,
        step: 0.01,
        unit: "m",
        filter_candidate: false,
      },
      {
        code: "ancho_plaza_m",
        label: "Ancho de plaza",
        type: "number",
        order: 2,
        minimum: 0.01,
        maximum: 99999999.99,
        step: 0.01,
        unit: "m",
        filter_candidate: false,
      },
      {
        code: "vehiculos_admitidos",
        label: "Vehículos admitidos",
        type: "enum_list",
        order: 3,
        options: ["motocicleta", "automovil", "suv", "furgon"],
        filter_candidate: false,
      },
      {
        code: "altura_maxima_m",
        label: "Altura máxima",
        type: "number",
        order: 4,
        minimum: 0.01,
        maximum: 99999999.99,
        step: 0.01,
        unit: "m",
        filter_candidate: false,
      },
      {
        code: "techado",
        label: "Techado",
        type: "boolean",
        order: 5,
        filter_candidate: false,
      },
      {
        code: "tipo_acceso",
        label: "Tipo de acceso",
        type: "enum",
        order: 6,
        options: ["abierto", "porton_manual", "porton_automatico"],
        filter_candidate: false,
      },
      {
        code: "carga_electrica",
        label: "Carga eléctrica",
        type: "boolean",
        order: 7,
        filter_candidate: false,
      },
    ],
  };

  await mockGateway(page, async (pathname, method) => {
    requests.push(`${method} ${pathname}`);
    const auth = handleAuthRequests(pathname, method);
    if (auth) return auth;
    if (pathname === "/api/backend/spaces/categories" && method === "GET")
      return { status: 200, body: { items: categories } };
    if (
      pathname ===
        "/api/backend/spaces/categories/estacionamiento/attributes" &&
      method === "GET"
    )
      return { status: 200, body: categorySchema };
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await page.locator("#wizard-category").selectOption("estacionamiento");
  await page.locator("#wizard-title").fill("Estacionamiento de prueba");
  await page
    .locator("#wizard-description")
    .fill(
      "Estacionamiento amplio y seguro, con acceso cómodo y espacio bien iluminado. Este perfil permite revisar las características dinámicas del lugar antes de completar la publicación del anuncio.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();

  const length = page.getByLabel("Largo de plaza (m)");
  const width = page.getByLabel("Ancho de plaza (m)");
  const height = page.getByLabel("Altura máxima (m)");
  for (const input of [length, width, height]) {
    await expect(input).toHaveAttribute("type", "number");
    await expect(input).toHaveAttribute("min", "0.01");
    await expect(input).toHaveAttribute("max", "99999999.99");
    await expect(input).toHaveAttribute("step", "0.01");
  }
  const vehicleGroup = page.getByRole("group", { name: "Vehículos admitidos" });
  for (const option of ["motocicleta", "automovil", "suv", "furgon"])
    await expect(vehicleGroup.getByLabel(option)).toBeVisible();
  const access = page.getByLabel("Tipo de acceso");
  for (const option of ["abierto", "porton_manual", "porton_automatico"])
    await expect(access.locator(`option[value="${option}"]`)).toHaveCount(1);
  await expect(page.getByLabel("Techado")).toBeVisible();
  await expect(page.getByLabel("Techado")).toHaveAttribute("type", "checkbox");
  await expect(page.getByLabel("Carga eléctrica")).toHaveAttribute(
    "type",
    "checkbox",
  );
  expect(requests).not.toContain("POST /api/backend/spaces");
});

const profileFixtureCategories = [
  { code: "oficina", name: "Oficina" },
  { code: "sala_multiproposito", name: "Sala o espacio multipropósito" },
  { code: "bodega", name: "Bodega" },
  { code: "estacionamiento", name: "Estacionamiento" },
  { code: "local_flexible", name: "Local flexible" },
  { code: "stand", name: "Stand" },
  { code: "quincho", name: "Quincho" },
  { code: "parcela_eventos", name: "Parcela o espacio para eventos" },
  { code: "inmuebles", name: "Inmuebles" },
];
const profileFixtureByCode = categoryProfileFixture.profiles as Record<
  string,
  CategoryProfileSchema
>;

for (const category of profileFixtureCategories) {
  test(`@phase7 @controlled perfil ${category.code} v1: renderiza todos sus atributos sin mutar`, async ({
    page,
  }) => {
    const requests: string[] = [];
    const schema = profileFixtureByCode[category.code];
    expect(schema, `${category.code}: missing v1 fixture`).toBeDefined();
    if (!schema) throw new Error(`${category.code}: missing v1 fixture`);
    expect(schema.category_code).toBe(category.code);
    expect(schema.schema_version).toBe(1);

    await mockGateway(page, async (pathname, method) => {
      requests.push(`${method} ${pathname}`);
      const auth = handleAuthRequests(pathname, method);
      if (auth) return auth;
      if (pathname === "/api/backend/spaces/categories" && method === "GET")
        return { status: 200, body: { items: profileFixtureCategories } };
      if (
        pathname ===
          `/api/backend/spaces/categories/${category.code}/attributes` &&
        method === "GET"
      )
        return { status: 200, body: schema };
      return apiError(404, "not_found");
    });

    await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
    await page.locator("#wizard-category").selectOption(category.code);
    await page.locator("#wizard-title").fill(`${category.name} sintético`);
    await page
      .locator("#wizard-description")
      .fill(
        "Espacio sintético usado para verificar que cada perfil vigente presenta todos sus controles, opciones y límites declarados sin crear ni modificar una publicación.",
      );
    await page.getByRole("button", { name: "Siguiente" }).click();
    await expect(
      page.getByText("Paso 2: Ubicación y características"),
    ).toBeVisible();

    for (const attribute of schema.attributes) {
      const numeric =
        attribute.type === "integer" || attribute.type === "number";
      const accessibleLabel =
        numeric && attribute.unit
          ? `${attribute.label} (${attribute.unit})`
          : attribute.label;
      if (attribute.type === "boolean") {
        await expect(page.getByLabel(accessibleLabel)).toHaveAttribute(
          "type",
          "checkbox",
        );
      } else if (numeric) {
        const control = page.getByLabel(accessibleLabel);
        await expect(control).toHaveAttribute("type", "number");
        if (attribute.minimum !== undefined)
          await expect(control).toHaveAttribute(
            "min",
            String(attribute.minimum),
          );
        if (attribute.maximum !== undefined)
          await expect(control).toHaveAttribute(
            "max",
            String(attribute.maximum),
          );
        await expect(control).toHaveAttribute(
          "step",
          String(attribute.step ?? (attribute.type === "integer" ? 1 : 0.1)),
        );
      } else if (attribute.type === "enum") {
        const control = page.getByLabel(accessibleLabel);
        await expect(control.locator("option")).toHaveCount(
          (attribute.options?.length ?? 0) + 1,
        );
        for (const option of attribute.options ?? [])
          await expect(
            control.getByRole("option", { name: option, exact: true }),
          ).toHaveCount(1);
      } else {
        expect(attribute.type, `${category.code}: ${attribute.code}`).toBe(
          "enum_list",
        );
        const group = page.getByRole("group", {
          name: attribute.label,
        });
        await expect(group).toBeVisible();
        for (const option of attribute.options ?? [])
          await expect(
            group.getByRole("checkbox", { name: option, exact: true }),
          ).toBeVisible();
      }
      if (attribute.required) {
        expect(attribute.type, `${category.code}: ${attribute.code}`).toBe(
          "enum",
        );
        await expect(page.getByLabel(attribute.label)).toHaveAttribute(
          "aria-required",
          "true",
        );
      }
    }

    expect(requests).toContain(
      `GET /api/backend/spaces/categories/${category.code}/attributes`,
    );
    expect(requests).not.toContain("POST /api/backend/spaces");
  });
}

test("@phase7 @controlled perfil sala_multiproposito v1: bloquea el avance y asocia el error al mínimo del contrato", async ({
  page,
}) => {
  const requests: string[] = [];
  const categoryCode = "sala_multiproposito";
  const schema = profileFixtureByCode[categoryCode];

  await mockGateway(page, async (pathname, method) => {
    requests.push(`${method} ${pathname}`);
    const auth = handleAuthRequests(pathname, method);
    if (auth) return auth;
    if (pathname === "/api/backend/spaces/categories" && method === "GET")
      return { status: 200, body: { items: profileFixtureCategories } };
    if (
      pathname ===
        `/api/backend/spaces/categories/${categoryCode}/attributes` &&
      method === "GET"
    )
      return { status: 200, body: schema };
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await page.locator("#wizard-category").selectOption(categoryCode);
  await page.locator("#wizard-title").fill("Sala multipropósito sintética");
  await page
    .locator("#wizard-description")
    .fill(
      "Espacio sintético para verificar la validación de atributos contra el perfil versionado sin crear ni modificar un anuncio real.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();

  await page.locator("#wizard-address").fill("Avenida sintética 123");
  await page.locator("#wizard-area").fill("50");
  await page.locator("#wizard-capacity").fill("4");
  const chairs = page.getByLabel("Sillas disponibles");
  await chairs.fill("-1");
  await page.getByRole("button", { name: "Siguiente" }).click();

  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();
  await expect(chairs).toHaveAttribute("aria-invalid", "true");
  const describedBy = await chairs.getAttribute("aria-describedby");
  expect(describedBy?.split(/\s+/)).toContain(
    "wizard-attribute-sillas_disponibles-error",
  );
  await expect(
    page.locator("#wizard-attribute-sillas_disponibles-error"),
  ).toHaveText("El atributo 'Sillas disponibles' debe ser al menos 0.");
  expect(requests).not.toContain("POST /api/backend/spaces");
});

test("@phase7 @controlled el asistente espera el perfil dinámico antes de validar el paso de atributos", async ({
  page,
}) => {
  const categoryCode = "sala_multiproposito";
  const schema = profileFixtureByCode[categoryCode];
  let markSchemaRequested!: () => void;
  const schemaRequested = new Promise<void>((resolve) => {
    markSchemaRequested = resolve;
  });
  let releaseSchema!: () => void;
  const schemaGate = new Promise<void>((resolve) => {
    releaseSchema = resolve;
  });

  await mockGateway(page, async (pathname, method) => {
    const auth = handleAuthRequests(pathname, method);
    if (auth) return auth;
    if (pathname === "/api/backend/spaces/categories" && method === "GET")
      return { status: 200, body: { items: profileFixtureCategories } };
    if (
      pathname ===
        `/api/backend/spaces/categories/${categoryCode}/attributes` &&
      method === "GET"
    ) {
      markSchemaRequested();
      await schemaGate;
      return { status: 200, body: schema };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await page.locator("#wizard-category").selectOption(categoryCode);
  await page.locator("#wizard-title").fill("Sala sintética");
  await page
    .locator("#wizard-description")
    .fill(
      "Espacio sintético para verificar que el asistente espera el perfil dinámico antes de validar los atributos específicos y permitir continuar.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await schemaRequested;
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();
  const nextButton = page.getByRole("button", { name: "Siguiente" });
  await expect(nextButton).toBeDisabled();

  releaseSchema();
  await expect(page.getByLabel("Sillas disponibles")).toBeVisible();
  await expect(nextButton).toBeEnabled();
});

test("@phase7 @controlled si falla el perfil dinámico, el wizard permite reintentar sin guardar", async ({
  page,
}) => {
  const categoryCode = "sala_multiproposito";
  const schema = profileFixtureByCode[categoryCode];
  let profileRequests = 0;
  const mutations: string[] = [];

  await mockGateway(page, async (pathname, method) => {
    const auth = handleAuthRequests(pathname, method);
    if (auth) return auth;
    if (pathname === "/api/backend/spaces/categories" && method === "GET")
      return { status: 200, body: { items: profileFixtureCategories } };
    if (
      pathname ===
        `/api/backend/spaces/categories/${categoryCode}/attributes` &&
      method === "GET"
    ) {
      profileRequests += 1;
      return profileRequests === 1
        ? apiError(503, "unavailable")
        : { status: 200, body: schema };
    }
    if (method !== "GET") mutations.push(`${method} ${pathname}`);
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await page.locator("#wizard-category").selectOption(categoryCode);
  await page.locator("#wizard-title").fill("Sala sintética");
  await page
    .locator("#wizard-description")
    .fill(
      "Espacio sintético para verificar el mensaje y la recuperación cuando el servicio de perfiles no está disponible temporalmente.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "No se pudo cargar el perfil de esta categoría." }),
  ).toBeVisible();
  const nextButton = page.getByRole("button", { name: "Siguiente" });
  await expect(nextButton).toBeDisabled();

  await page.getByRole("button", { name: "Reintentar perfil" }).click();
  await expect(page.getByLabel("Sillas disponibles")).toBeVisible();
  await expect(nextButton).toBeEnabled();
  expect(profileRequests).toBe(2);
  expect(mutations).toEqual([]);
});

test("@phase7 @controlled cambiar categoría elimina atributos dinámicos anteriores del payload", async ({
  page,
}) => {
  const oldCategory = "oficina";
  const newCategory = "estacionamiento";
  const oldSchema = profileFixtureByCode[oldCategory];
  const newSchema = profileFixtureByCode[newCategory];
  const wifi = oldSchema.attributes.find(
    (attribute) => attribute.code === "wifi",
  );
  const access = newSchema.attributes.find(
    (attribute) => attribute.code === "tipo_acceso",
  );
  expect(wifi, "oficina v1 debe declarar wifi").toBeDefined();
  expect(access, "estacionamiento v1 debe declarar tipo_acceso").toBeDefined();

  const createBodies: Record<string, unknown>[] = [];
  const newSpaceId = "00000000-0000-4000-8000-000000000102";

  await mockGateway(page, async (pathname, method, request) => {
    const auth = handleAuthRequests(pathname, method);
    if (auth) return auth;
    if (pathname === "/api/backend/spaces/categories" && method === "GET")
      return { status: 200, body: { items: profileFixtureCategories } };

    const profileMatch = pathname.match(
      /^\/api\/backend\/spaces\/categories\/([^/]+)\/attributes$/,
    );
    if (profileMatch && method === "GET") {
      const schema = profileFixtureByCode[profileMatch[1]!];
      return schema
        ? { status: 200, body: schema }
        : apiError(404, "not_found");
    }

    if (pathname === "/api/backend/spaces" && method === "POST") {
      const createBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      createBodies.push(createBody);
      return {
        status: 201,
        body: {
          id: newSpaceId,
          category_code: createBody.category_code,
          category_name:
            profileFixtureCategories.find(
              (category) => category.code === createBody?.category_code,
            )?.name ?? newCategory,
          title: createBody.title,
          description: createBody.description,
          area_m2: createBody.area_m2,
          capacity: createBody.capacity,
          usage_rules: createBody.usage_rules,
          rate_unit: createBody.rate_unit,
          base_price_clp: createBody.base_price_clp,
          address: createBody.address,
          state: "borrador",
          time_zone: "America/Santiago",
          region_code: createBody.region_code,
          comuna_code: createBody.comuna_code,
          attribute_schema_version: createBody.attribute_schema_version,
          attributes: createBody.attributes,
        },
      };
    }

    if (
      pathname === `/api/backend/spaces/${newSpaceId}/availability` &&
      method === "PUT"
    )
      return {
        status: 200,
        body: { space_id: newSpaceId, time_zone: "America/Santiago" },
      };
    if (
      pathname === `/api/backend/spaces/${newSpaceId}/weekly-hours` &&
      method === "PUT"
    )
      return {
        status: 200,
        body: { data: { space_id: newSpaceId, days: [] } },
      };
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await page.locator("#wizard-category").selectOption(oldCategory);
  await page.locator("#wizard-title").fill("Estacionamiento de prueba");
  await page
    .locator("#wizard-description")
    .fill(
      "Descripción sintética para probar que al cambiar de una oficina con Wi-Fi a estacionamiento el atributo anterior no se filtre en el borrador creado.",
    );
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByLabel(wifi!.label)).toBeVisible();
  await page.getByLabel(wifi!.label).check();

  await page.getByRole("button", { name: "Atrás" }).click();
  await page.locator("#wizard-category").selectOption(newCategory);
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByLabel(access!.label)).toBeVisible();
  await page.locator("#wizard-address").fill("Dirección sintética 123");
  await page.locator("#wizard-comuna").fill("Las Condes");
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByText("Paso 3: Tarifas")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(page.getByText("Paso 4: Disponibilidad semanal")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente" }).click();
  await expect(
    page.getByText("Paso 5: Resumen y confirmación de alta"),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Crear espacio y continuar a edición" })
    .click();

  await expect.poll(() => createBodies).toHaveLength(1);
  const createdBody = createBodies[0]!;
  expect(createdBody.category_code).toBe(newCategory);
  expect(createdBody.attributes).not.toHaveProperty("wifi");
});

test("@phase7 @controlled session-boundary: el formulario de nuevo espacio aísla el estado local ante cambios de permisos del mismo actor", async ({
  page,
}) => {
  const actorId = "770e8400-e29b-41d4-a716-44665544000c";
  const fixedExpiry = new Date(Date.now() + 3600_000).toISOString();

  let currentActorSession = {
    account_id: actorId,
    roles: ["arrendatario", "anfitrion"],
    capabilities: [] as string[],
    restricted_mode: false,
    expires_at: fixedExpiry,
  };

  const categories = [{ code: "oficina", name: "Oficinas y Cowork" }];

  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: currentActorSession,
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: currentActorSession,
      };
    }
    if (pathname === "/api/backend/spaces/categories" && method === "GET") {
      return {
        status: 200,
        body: { items: categories },
      };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/anfitrion/espacios/nuevo");
  await expect(page.locator("h1")).toHaveText("Publicar un nuevo espacio");

  await expect(page.locator("#wizard-title")).toBeVisible();
  await page.locator("#wizard-title").fill("Borrador con permisos previos");
  await expect(page.locator("#wizard-title")).toHaveValue(
    "Borrador con permisos previos",
  );

  // Mismo actor recibe actualización de capabilities en backend
  currentActorSession = {
    account_id: actorId,
    roles: ["arrendatario", "anfitrion"],
    capabilities: ["spaces:restricted_compliance"],
    restricted_mode: false,
    expires_at: fixedExpiry,
  };

  const sessionCheckPromise = page.waitForResponse(
    (res) =>
      res.url().includes("/api/backend/auth/session") &&
      res.request().method() === "GET",
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const sessionRes = await sessionCheckPromise;
  expect(sessionRes.status()).toBe(200);

  // El límite de sesión debe reiniciar el árbol reactivo al detectar cambio en la tupla de sesión
  await expect(page.locator("#wizard-title")).toHaveValue("");
});
