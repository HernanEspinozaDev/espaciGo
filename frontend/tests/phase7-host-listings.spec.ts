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
  const spaces = [
    {
      id: "space-borrador-1",
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
      id: "space-activa-1",
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

  // 5. Verificación de ausencia de desbordamiento horizontal
  const dimensions = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    width: innerWidth,
  }));
  expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.width);
});

test("@phase7 @controlled /anfitrion/espacios/nuevo: asistente en 5 pasos (Paso 1: selección dinámica de categorías desde /spaces/categories, título; Paso 2: dirección y atributos dinámicos con attribute_schema_version; Paso 3: tarifas CLP; Paso 4: disponibilidad semanal; Paso 5: confirmación y creación con Idempotency-Key en POST /spaces redirigiendo a /anfitrion/espacios/{id}/editar)", async ({
  page,
}) => {
  let capturedIdempotencyKey: string | null = null;
  let capturedCreateBody: Record<string, unknown> | null = null;
  const newSpaceId = "space-created-uuid-101";

  const categories = [
    { code: "oficina", name: "Oficinas y Cowork" },
    { code: "bodega", name: "Bodegas y Mini-bodegas" },
  ];

  const categorySchema = {
    category_code: "oficina",
    schema_version: 2,
    attributes: [
      {
        code: "has_wifi",
        label: "Wi-Fi corporativo de alta velocidad",
        type: "boolean",
        required: false,
        order: 1,
        filter_candidate: true,
      },
      {
        code: "desks_count",
        label: "Puestos de trabajo equipados",
        type: "integer",
        required: false,
        order: 2,
        minimum: 1,
        maximum: 50,
        filter_candidate: true,
      },
    ],
  };

  await mockGateway(page, async (pathname, method, request) => {
    const authRes = handleAuthRequests(pathname, method);
    if (authRes) return authRes;

    if (pathname === "/api/backend/spaces/categories" && method === "GET") {
      return {
        status: 200,
        body: { items: categories },
      };
    }

    if (
      (pathname === "/api/backend/spaces/categories/oficina/attributes" ||
        pathname === "/api/backend/spaces/categories/oficina/attributes/2") &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: categorySchema,
      };
    }

    if (pathname === "/api/backend/spaces" && method === "POST") {
      capturedIdempotencyKey = request.headers()["idempotency-key"] ?? null;
      capturedCreateBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      return {
        status: 201,
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
          time_zone: "America/Santiago",
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
      return {
        status: 200,
        body: { space_id: newSpaceId, time_zone: "America/Santiago" },
      };
    }

    if (
      pathname ===
        `/api/backend/local/booking-trial/catalog/${newSpaceId}/weekly-hours` &&
      method === "PUT"
    ) {
      return {
        status: 200,
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
          attribute_schema_version: 2,
          attributes: { has_wifi: true, cancellation_policy: "flexible" },
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
      pathname ===
        `/api/backend/local/booking-trial/catalog/${newSpaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
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
  await page.locator("#wizard-category").selectOption("oficina");
  await page.locator("#wizard-title").fill("Oficina Ejecutiva El Golf");
  await page
    .locator("#wizard-description")
    .fill("Espacio moderno full equipado con luz natural");
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 2: Ubicación y Atributos Dinámicos
  await expect(
    page.getByText("Paso 2: Ubicación y características"),
  ).toBeVisible();
  await page.locator("#wizard-address").fill("Av. Apoquindo 3000, Las Condes");
  await page.locator("#wizard-comuna").fill("Las Condes");
  await page.locator("#wizard-area").fill("35");
  await page.locator("#wizard-capacity").fill("6");

  // Atributo dinámico del esquema v2
  const wifiCheckbox = page.locator("#attr-has_wifi");
  await expect(wifiCheckbox).toBeVisible();
  await wifiCheckbox.check();
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 3: Tarifas y Reglas
  await expect(
    page.getByText("Paso 3: Tarifas iniciales y condiciones"),
  ).toBeVisible();
  await page.locator("#wizard-rate-unit").selectOption("hora");
  await page.locator("#wizard-price").fill("35000");
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 4: Disponibilidad semanal
  await expect(
    page.getByText("Paso 4: Disponibilidad semanal y zona horaria"),
  ).toBeVisible();
  await expect(page.locator("#wizard-timezone")).toHaveValue(
    "America/Santiago",
  );
  await page.getByRole("button", { name: "Siguiente" }).click();

  // PASO 5: Confirmación
  await expect(
    page.getByText("Paso 5: Resumen y confirmación de alta"),
  ).toBeVisible();
  await expect(
    page.locator("h2").filter({ hasText: "Oficina Ejecutiva El Golf" }),
  ).toBeVisible();
  await expect(page.getByText("$35.000 CLP")).toBeVisible();

  // Enviar y verificar llamada con Idempotency-Key UUID
  await page
    .getByRole("button", { name: "Crear espacio y continuar a edición" })
    .click();

  // Redirección confirmada al editor
  await page.waitForURL((url) =>
    url.pathname.endsWith(`/espacios/${newSpaceId}/editar`),
  );
  await expect(page.locator("h1")).toHaveText("Oficina Ejecutiva El Golf");

  // Verificaciones del contrato POST /spaces y clave de idempotencia
  expect(capturedIdempotencyKey).toBeTruthy();
  expect(capturedIdempotencyKey).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(capturedCreateBody).not.toBeNull();
  const createBody = capturedCreateBody!;
  expect(createBody.category_code).toBe("oficina");
  expect(createBody.title).toBe("Oficina Ejecutiva El Golf");
  expect(createBody.base_price_clp).toBe(35000);
  expect(createBody.rate_unit).toBe("hora");
  expect(createBody.attribute_schema_version).toBe(2);
  expect((createBody.attributes as Record<string, unknown>)?.has_wifi).toBe(
    true,
  );
});

test("@phase7 @controlled /anfitrion/espacios/[id]/editar: navegación por pestañas (Información con PUT /spaces/{id}/publication-content, Tarifas e historial con PUT /spaces/{id}/tariff y GET /spaces/{id}/tariffs, Horarios con PUT /spaces/{id}/availability, Fotos con aviso visible permanente de entorno sintético y reordenamiento, y Publicación con PUT /spaces/{id}/publication)", async ({
  page,
}) => {
  const spaceId = "space-edit-42";
  let putContentBody: Record<string, unknown> | null = null;
  let putTariffBody: Record<string, unknown> | null = null;
  let putAvailabilityBody: Record<string, unknown> | null = null;
  let putPublicationBody: Record<string, unknown> | null = null;
  let putCoverBody: Record<string, unknown> | null = null;
  let postGalleryCount = 0;
  let reorderPutCount = 0;

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
    attributes: { has_wifi: true },
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
      return { status: 200, body: currentSpace };
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
              code: "has_wifi",
              label: "Wi-Fi corporativo",
              type: "boolean",
              required: false,
              order: 1,
              filter_candidate: true,
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
      return { status: 200, body: newRate };
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
      if (putContentBody.title)
        currentSpace.title = String(putContentBody.title);
      return { status: 200, body: currentSpace };
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
      return {
        status: 200,
        body: {
          space_id: spaceId,
          time_zone: putAvailabilityBody.time_zone ?? "America/Santiago",
        },
      };
    }
    if (
      pathname ===
        `/api/backend/local/booking-trial/catalog/${spaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
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
      pathname ===
        `/api/backend/local/booking-trial/catalog/${spaceId}/weekly-hours` &&
      method === "PUT"
    ) {
      return {
        status: 200,
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
      return { status: 200, body: { items: photos } };
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "POST"
    ) {
      postGalleryCount += 1;
      const photoId = `photo-${postGalleryCount}`;
      const newPhoto = {
        id: photoId,
        photo_id: photoId,
        space_id: spaceId,
        fixture_code: "synthetic-png-v1",
        mime_type: "image/png",
        sha256: `synthetic-sha256-mock-${postGalleryCount}`,
        size_bytes: 2048,
        position: photos.length + 1,
        cover: photos.length === 0,
        is_cover: photos.length === 0,
        alt_text: `Foto de ensayo #${postGalleryCount}`,
      };
      photos.push(newPhoto);
      if (newPhoto.is_cover) {
        currentSpace.cover_photo_id = photoId;
      }
      return { status: 201, body: { photo: newPhoto } };
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
    if (
      pathname.startsWith(`/api/backend/spaces/${spaceId}/gallery/`) &&
      method === "PUT"
    ) {
      reorderPutCount += 1;
      return { status: 200, body: {} };
    }

    // Publicación / Ocultación
    if (
      pathname === `/api/backend/spaces/${spaceId}/publication` &&
      method === "PUT"
    ) {
      putPublicationBody = JSON.parse(request.postData() ?? "{}") as Record<
        string,
        unknown
      >;
      currentSpace.state = String(putPublicationBody.state);
      return { status: 200, body: currentSpace };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, `/anfitrion/espacios/${spaceId}/editar`);

  // Verificación de carga y estado inicial
  await expect(page.locator("h1")).toHaveText("Oficina Providencia Inicial");
  await expect(page.getByText("Borrador").first()).toBeVisible();

  // 1. Pestaña Información: actualizar título y guardar
  await page.locator("#editor-title").fill("Oficina Providencia Remodelada");
  await page.getByRole("button", { name: "Guardar contenido" }).click();
  await expect(
    page.getByText("Información actualizada exitosamente."),
  ).toBeVisible();
  expect(putContentBody).not.toBeNull();
  expect(putContentBody!.title).toBe("Oficina Providencia Remodelada");

  // 2. Pestaña Tarifas: navegar, actualizar tarifa base y consultar historial
  await page.getByRole("tab", { name: "Tarifas e historial" }).click();
  await expect(page.locator("#editor-price")).toHaveValue("20000");
  await page.locator("#editor-price").fill("28000");
  await page.getByRole("button", { name: "Actualizar tarifa" }).click();
  await expect(
    page.getByText("Tarifa actualizada y registrada en el historial."),
  ).toBeVisible();
  expect(putTariffBody!.base_price).toBe(28000);
  await expect(page.getByText("$28.000 CLP").first()).toBeVisible();

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

  // 4. Pestaña Fotos: verificar aviso visible permanente y agregar fotos de ensayo
  await page.getByRole("tab", { name: /Fotografías/ }).click();
  await expect(
    page.getByText(
      "Aviso permanente: Las fotografías cargadas en este entorno de desarrollo son simuladas y sintéticas",
    ),
  ).toBeVisible();

  // Subir primera foto sintética (quedará como portada)
  await page
    .getByRole("button", { name: "Generar primera fotografía" })
    .click();
  await expect(
    page.getByText("Fotografía sintética de ensayo agregada a la galería."),
  ).toBeVisible();
  await expect(page.getByText("Portada", { exact: true })).toBeVisible();
  expect(postGalleryCount).toBe(1);

  // Subir segunda foto sintética para ejercitar reordenamiento
  await page.getByRole("button", { name: "Agregar foto de ensayo" }).click();
  await expect(page.getByText("#2", { exact: true })).toBeVisible();
  expect(postGalleryCount).toBe(2);

  // Mover foto hacia arriba
  const moveUpButton = page.locator(
    "button[aria-label='Mover foto antes']:not([disabled])",
  );
  await moveUpButton.click();
  await expect(
    page.getByText("Orden de fotografías actualizado."),
  ).toBeVisible();
  expect(reorderPutCount).toBeGreaterThanOrEqual(1);

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
      return { status: 200, body: testSpace };
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
      pathname ===
        `/api/backend/local/booking-trial/catalog/${spaceId}/weekly-hours` &&
      method === "GET"
    ) {
      return {
        status: 200,
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
    schema_version: 2,
    attributes: [
      {
        code: "has_wifi",
        label: "Wi-Fi corporativo de alta velocidad",
        type: "boolean",
        required: false,
        order: 1,
        filter_candidate: true,
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
        pathname === "/api/backend/spaces/categories/oficina/attributes/2") &&
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
