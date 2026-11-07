import { expect, test, type Request, type Route } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";

const spaceId = "00000000-0000-4000-8000-000000000042";
const accountId = "770e8400-e29b-41d4-a716-446655440000";
const existingBlockId = "10000000-0000-4000-8000-000000000001";
const createdBlockId = "10000000-0000-4000-8000-000000000002";
const timeZone = "America/Santiago";

test("@controlled host creates a private date availability block", async ({
  page,
}) => {
  const bff = createBffFixtureState();
  const blocks = [
    {
      id: existingBlockId,
      space_id: spaceId,
      start_at: "2026-12-10T09:00:00-03:00",
      end_at: "2026-12-10T11:00:00-03:00",
      time_zone: timeZone,
      reason: "Mantenimiento de prueba",
      created_at: "2026-11-01T12:00:00Z",
    },
  ];
  const writes: Array<{ body: unknown; response: unknown }> = [];
  const attempts: unknown[] = [];
  const deletions: string[] = [];
  let configuredTimeZone: string | null = timeZone;
  let listReads = 0;

  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const body = request.postDataJSON?.() ?? {};
    let response: {
      status: number;
      body?: unknown;
      headers?: Record<string, string>;
    };

    if (path === "/api/backend/auth/login" && method === "POST") {
      response = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600000).toISOString(),
        },
      };
    } else if (path === "/api/backend/auth/session" && method === "GET") {
      response = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    } else if (path === `/api/backend/spaces/${spaceId}` && method === "GET") {
      response = {
        status: 200,
        headers: { etag: '"calendar-space-v1"' },
        body: {
          id: spaceId,
          category_code: "oficina",
          category_name: "Oficinas",
          title: "Espacio sintético para calendario",
          description: "Fixture de prueba local.",
          area_m2: 40,
          capacity: 4,
          usage_rules: "Fixture sintético",
          rate_unit: "hora",
          base_price_clp: 10000,
          address: "Dirección ficticia 123",
          state: "borrador",
          time_zone: configuredTimeZone,
          region_code: "13",
          comuna_code: "13123",
          attribute_schema_version: 1,
          attributes: {},
        },
      };
    } else if (
      path === `/api/backend/spaces/${spaceId}/tariffs` &&
      method === "GET"
    ) {
      response = { status: 200, body: { items: [] } };
    } else if (
      path === `/api/backend/spaces/${spaceId}/tariff` &&
      method === "GET"
    ) {
      response = {
        status: 200,
        body: {
          space_id: spaceId,
          version: 1,
          rate_unit: "hora",
          base_price: 10000,
          currency: "CLP",
          created_at: "2026-10-01T00:00:00Z",
        },
      };
    } else if (
      path === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "GET"
    ) {
      response = { status: 200, body: { items: [] } };
    } else if (
      path === "/api/backend/spaces/categories/oficina/attributes/1" &&
      method === "GET"
    ) {
      response = {
        status: 200,
        body: { category_code: "oficina", schema_version: 1, attributes: [] },
      };
    } else if (
      path === `/api/backend/spaces/${spaceId}/availability/blocks` &&
      method === "GET"
    ) {
      listReads += 1;
      response = {
        status: 200,
        body: { space_id: spaceId, time_zone: timeZone, items: blocks },
      };
    } else if (
      path === `/api/backend/spaces/${spaceId}/availability/blocks` &&
      method === "POST"
    ) {
      const submitted = body as {
        start_at: string;
        end_at: string;
        reason: string;
      };
      attempts.push(body);
      if (submitted.reason === "Intervalo ocupado sintético") {
        response = {
          status: 409,
          body: {
            error: {
              code: "occupancy_conflict",
              message: "Synthetic occupancy conflict",
            },
          },
        };
      } else {
        const created = {
          id: createdBlockId,
          space_id: spaceId,
          start_at: submitted.start_at,
          end_at: submitted.end_at,
          time_zone: timeZone,
          reason: submitted.reason,
          created_at: "2026-11-07T12:00:00Z",
        };
        blocks.push(created);
        response = { status: 201, body: created };
        writes.push({ body, response: created });
      }
    } else if (
      path ===
        `/api/backend/spaces/${spaceId}/availability/blocks/${createdBlockId}` &&
      method === "DELETE"
    ) {
      const index = blocks.findIndex((block) => block.id === createdBlockId);
      if (index < 0) {
        response = {
          status: 404,
          body: { error: { code: "not_found", message: "Missing fixture" } },
        };
      } else {
        blocks.splice(index, 1);
        deletions.push(createdBlockId);
        response = { status: 204 };
      }
    } else {
      response = {
        status: 404,
        body: { error: { code: "not_found", message: "Missing fixture" } },
      };
    }

    const adapted = bff.adapt(request as Request, response);
    if (!adapted)
      throw new Error(`Missing fixture response: ${method} ${path}`);
    await route.fulfill({
      status: adapted.status,
      headers: {
        "content-type": "application/json",
        ...(adapted.headers ?? {}),
      },
      body: adapted.status === 204 ? "" : JSON.stringify(adapted.body ?? {}),
    });
  });

  await page.goto(
    `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${spaceId}/editar`)}`,
  );
  await page.getByLabel("Correo electrónico").fill("calendar@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL(`**/anfitrion/espacios/${spaceId}/editar`);

  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toBeVisible();
  const endDateFilter = page.getByLabel("Hasta", { exact: true });
  const validEndDate = await endDateFilter.inputValue();
  const readsBeforeInvalidRange = listReads;
  await endDateFilter.fill("");
  await page.getByRole("button", { name: "Actualizar fechas" }).click();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("Selecciona un rango de fechas válido.");
  await expect(
    page.getByRole("button", { name: "Actualizar fechas" }),
  ).toBeEnabled();
  expect(listReads).toBe(readsBeforeInvalidRange);
  await endDateFilter.fill(validEndDate);
  for (const viewport of [
    { name: "desktop", width: 1280, height: 800 },
    { name: "mobile", width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const dimensions = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }));
    expect(
      dimensions.scrollWidth,
      `${viewport.name} horizontal overflow`,
    ).toBeLessThanOrEqual(dimensions.clientWidth);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(
      accessibility.violations.map(({ id, impact }) => ({ id, impact })),
      `${viewport.name} accessibility violations`,
    ).toEqual([]);
  }
  await page.setViewportSize({ width: 1280, height: 800 });
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-15T13:00");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-15T15:30");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Intervalo ocupado sintético");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText(
    "El intervalo coincide con una ocupación activa. Elige otro rango.",
  );
  expect(attempts).toHaveLength(1);
  expect(writes).toHaveLength(0);

  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Evento privado sintético");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();

  await expect.poll(() => writes).toHaveLength(1);
  expect(attempts).toHaveLength(2);
  await expect.poll(() => listReads).toBeGreaterThan(1);
  expect(writes[0]).toEqual({
    body: {
      start_at: "2026-12-15T16:00:00Z",
      end_at: "2026-12-15T18:30:00Z",
      reason: "Evento privado sintético",
    },
    response: {
      id: createdBlockId,
      space_id: spaceId,
      start_at: "2026-12-15T16:00:00Z",
      end_at: "2026-12-15T18:30:00Z",
      time_zone: timeZone,
      reason: "Evento privado sintético",
      created_at: "2026-11-07T12:00:00Z",
    },
  });
  await expect(
    page.getByText("Evento privado sintético", { exact: true }),
  ).toBeVisible();
  const readsBeforeReload = listReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBeGreaterThan(readsBeforeReload);
  await expect(
    page.getByText("Evento privado sintético", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Eliminar bloqueo Evento privado sintético" })
    .click();
  await expect.poll(() => deletions).toEqual([createdBlockId]);
  await expect(
    page.getByText("Evento privado sintético", { exact: true }),
  ).toHaveCount(0);
  configuredTimeZone = null;
  const readsWithoutTimeZone = listReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect(
    page.getByText(
      "Guarda una zona horaria IANA antes de consultar o crear bloqueos por fecha.",
    ),
  ).toBeVisible();
  expect(listReads).toBe(readsWithoutTimeZone);
  await page.getByRole("button", { name: "Configurar zona horaria" }).click();
  await expect(
    page.getByRole("tab", { name: "Calendario y horarios" }),
  ).toHaveAttribute("aria-selected", "true");
});
