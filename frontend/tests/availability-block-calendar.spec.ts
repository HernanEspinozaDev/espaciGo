import { expect, test, type Request, type Route } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";

const spaceId = "00000000-0000-4000-8000-000000000042";
const accountId = "770e8400-e29b-41d4-a716-446655440000";
const existingBlockId = "10000000-0000-4000-8000-000000000001";
const createdBlockId = "10000000-0000-4000-8000-000000000002";
const externallyAddedBlockId = "10000000-0000-4000-8000-000000000003";
const createdKathmanduBlockId = "10000000-0000-4000-8000-000000000004";
const createdDstBlockId = "10000000-0000-4000-8000-000000000005";
const createdEasterBlockId = "10000000-0000-4000-8000-000000000006";
const createdUnsavedZoneBlockId = "10000000-0000-4000-8000-000000000007";
const createdStaleZoneRecoveryBlockId = "10000000-0000-4000-8000-000000000008";
const ambiguousBlockId = "10000000-0000-4000-8000-000000000009";
const timeZone = "America/Santiago";
const maximumLengthReason = "😀".repeat(500);

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
  const listRanges: Array<{ from: string | null; to: string | null }> = [];
  let configuredTimeZone: string | null = timeZone;
  let listingEtag = '"calendar-space-v1"';
  let weeklyEtag = '"calendar-schedule-v1"';
  const zoneConfigurationWrites: Array<{
    body: unknown;
    ifMatch: string | null;
  }> = [];
  const manualBlockMutationPreconditions: Array<{
    method: "POST" | "DELETE";
    ifMatch: string | null;
  }> = [];
  let listReads = 0;
  let failNextBlockListRead = false;
  let failNextBlockCreateAfterCommit = false;
  let failNextBlockDeleteAfterCommit = false;
  let holdNextBlockListRead = false;
  const releaseHeldBlockListReads: Array<() => void> = [];
  let scheduleMutationAttempts = 0;

  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const body = request.postDataJSON?.() ?? {};
    if (
      (method === "POST" &&
        path === `/api/backend/spaces/${spaceId}/availability/blocks`) ||
      (method === "DELETE" &&
        path.startsWith(`/api/backend/spaces/${spaceId}/availability/blocks/`))
    ) {
      manualBlockMutationPreconditions.push({
        method,
        ifMatch: request.headers()["if-match"] ?? null,
      });
    }
    if (
      method === "PUT" &&
      (path === `/api/backend/spaces/${spaceId}/availability` ||
        path === `/api/backend/spaces/${spaceId}/weekly-hours`)
    ) {
      scheduleMutationAttempts += 1;
    }
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
        headers: { etag: listingEtag },
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
      path === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      method === "GET"
    ) {
      response = {
        status: 200,
        headers: { etag: weeklyEtag },
        body: {
          data: {
            space_id: spaceId,
            enabled: true,
            time_zone: configuredTimeZone,
            days: Array.from({ length: 7 }, (_, index) => ({
              weekday: index + 1,
              periods: [],
            })),
          },
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
      path === `/api/backend/spaces/${spaceId}/availability` &&
      method === "PUT"
    ) {
      const ifMatch = request.headers()["if-match"] ?? null;
      zoneConfigurationWrites.push({ body, ifMatch });
      if (ifMatch !== listingEtag) {
        response = {
          status: 412,
          body: { error: { code: "precondition_failed", message: "Stale" } },
        };
      } else {
        configuredTimeZone = (body as { time_zone: string }).time_zone;
        listingEtag = '"calendar-space-v2"';
        weeklyEtag = listingEtag;
        response = {
          status: 200,
          headers: { etag: listingEtag },
          body: { space_id: spaceId, time_zone: configuredTimeZone },
        };
      }
    } else if (
      path === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      method === "PUT"
    ) {
      const ifMatch = request.headers()["if-match"] ?? null;
      if (ifMatch !== listingEtag) {
        response = {
          status: 412,
          body: { error: { code: "precondition_failed", message: "Stale" } },
        };
      } else {
        const submitted = body as { enabled?: boolean; days?: unknown };
        listingEtag = '"calendar-space-v3"';
        weeklyEtag = listingEtag;
        response = {
          status: 200,
          headers: { etag: weeklyEtag },
          body: {
            data: {
              space_id: spaceId,
              enabled: submitted.enabled === true,
              time_zone: configuredTimeZone,
              days: submitted.days,
            },
          },
        };
      }
    } else if (
      path === `/api/backend/spaces/${spaceId}/availability/blocks` &&
      method === "GET"
    ) {
      listReads += 1;
      const range = new URL(request.url()).searchParams;
      listRanges.push({ from: range.get("from"), to: range.get("to") });
      const rangeFrom = Date.parse(range.get("from") ?? "");
      const rangeTo = Date.parse(range.get("to") ?? "");
      const responseItems = blocks
        .filter(
          (block) =>
            Date.parse(block.start_at) < rangeTo &&
            Date.parse(block.end_at) > rangeFrom,
        )
        .map((block) => ({ ...block }));
      if (failNextBlockListRead) {
        failNextBlockListRead = false;
        response = {
          status: 503,
          body: {
            error: {
              code: "service_unavailable",
              message: "Synthetic temporary outage",
            },
          },
        };
      } else {
        if (holdNextBlockListRead) {
          holdNextBlockListRead = false;
          await new Promise<void>((resolve) => {
            releaseHeldBlockListReads.push(resolve);
          });
        }
        response = {
          status: 200,
          body: {
            space_id: spaceId,
            time_zone: configuredTimeZone ?? timeZone,
            items: responseItems,
          },
        };
      }
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
      if (
        submitted.reason === "Intervalo ocupado sintético" ||
        submitted.reason === maximumLengthReason
      ) {
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
        const createdId = failNextBlockCreateAfterCommit
          ? ambiguousBlockId
          : [
              createdBlockId,
              createdKathmanduBlockId,
              createdDstBlockId,
              createdEasterBlockId,
              createdUnsavedZoneBlockId,
              createdStaleZoneRecoveryBlockId,
            ][writes.length];
        if (!createdId) throw new Error("Synthetic block IDs exhausted");
        const created = {
          id: createdId,
          space_id: spaceId,
          start_at: submitted.start_at,
          end_at: submitted.end_at,
          time_zone: configuredTimeZone ?? timeZone,
          reason: submitted.reason,
          created_at: "2026-11-07T12:00:00Z",
        };
        blocks.push(created);
        if (failNextBlockCreateAfterCommit) {
          failNextBlockCreateAfterCommit = false;
          response = {
            status: 503,
            body: {
              error: {
                code: "service_unavailable",
                message: "Synthetic ambiguous create result",
              },
            },
          };
        } else {
          response = { status: 201, body: created };
          writes.push({ body, response: created });
        }
      }
    } else if (
      method === "DELETE" &&
      path.startsWith(`/api/backend/spaces/${spaceId}/availability/blocks/`)
    ) {
      const blockId = path.split("/").at(-1);
      const index = blocks.findIndex((block) => block.id === blockId);
      if (index < 0) {
        response = {
          status: 404,
          body: { error: { code: "not_found", message: "Missing fixture" } },
        };
      } else {
        blocks.splice(index, 1);
        deletions.push(blockId!);
        if (failNextBlockDeleteAfterCommit) {
          failNextBlockDeleteAfterCommit = false;
          response = {
            status: 503,
            body: {
              error: {
                code: "service_unavailable",
                message: "Synthetic ambiguous delete result",
              },
            },
          };
        } else {
          response = { status: 204 };
        }
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
    try {
      await route.fulfill({
        status: adapted.status,
        headers: {
          "content-type": "application/json",
          ...(adapted.headers ?? {}),
        },
        body: adapted.status === 204 ? "" : JSON.stringify(adapted.body ?? {}),
      });
    } catch (error) {
      if (!request.failure()?.errorText?.toLowerCase().includes("abort")) {
        throw error;
      }
    }
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
  const readsBeforeSingleDayRange = listReads;
  await page.getByLabel("Desde", { exact: true }).fill("2026-12-10");
  await endDateFilter.fill("2026-12-10");
  await page.getByRole("button", { name: "Actualizar fechas" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeSingleDayRange + 1);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-09T00:00:00.000Z",
    to: "2026-12-12T00:00:00.000Z",
  });
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toBeVisible();
  expect(writes).toHaveLength(0);
  expect(attempts).toHaveLength(0);
  expect(deletions).toHaveLength(0);

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

  const writesBeforeMissingStartRange = writes.length;
  const attemptsBeforeMissingStartRange = attempts.length;
  const deletionsBeforeMissingStartRange = deletions.length;
  const scheduleWritesBeforeMissingStartRange = scheduleMutationAttempts;
  await endDateFilter.fill("2026-12-10");
  await page.getByLabel("Desde", { exact: true }).fill("");
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
  expect(writes).toHaveLength(writesBeforeMissingStartRange);
  expect(attempts).toHaveLength(attemptsBeforeMissingStartRange);
  expect(deletions).toHaveLength(deletionsBeforeMissingStartRange);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeMissingStartRange);

  await page.getByLabel("Desde", { exact: true }).fill("2026-12-20");
  await endDateFilter.fill("2026-12-19");
  await page.getByRole("button", { name: "Actualizar fechas" }).click();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("Selecciona un rango de fechas válido.");
  expect(listReads).toBe(readsBeforeInvalidRange);

  await page.getByLabel("Desde", { exact: true }).fill("2026-12-10");
  await endDateFilter.fill("2026-12-20");
  await page.getByRole("button", { name: "Actualizar fechas" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeInvalidRange + 1);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-09T00:00:00.000Z",
    to: "2026-12-22T00:00:00.000Z",
  });

  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-15T14:00");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-15T13:00");
  await page.getByLabel("Motivo *", { exact: true }).fill("Rango inválido");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("La fecha y hora de término debe ser posterior al inicio.");
  expect(attempts).toHaveLength(0);
  expect(scheduleMutationAttempts).toBe(0);

  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-15T14:00");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-15T14:00");
  await page.getByLabel("Motivo *", { exact: true }).fill("Rango vacío");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("La fecha y hora de término debe ser posterior al inicio.");
  expect(attempts).toHaveLength(0);
  expect(scheduleMutationAttempts).toBe(0);

  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-15T13:00");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-15T14:00");
  await page.getByLabel("Motivo *", { exact: true }).fill("á".repeat(501));
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("El motivo es obligatorio y no puede superar 500 caracteres.");
  expect(attempts).toHaveLength(0);
  expect(scheduleMutationAttempts).toBe(0);

  for (const invalidWallTime of [
    {
      start: "2026-09-06T00:30",
      end: "2026-09-06T02:00",
    },
    {
      start: "2026-04-04T23:30",
      end: "2026-04-05T01:30",
    },
  ]) {
    await page
      .getByLabel("Fecha y hora de inicio *", { exact: true })
      .fill(invalidWallTime.start);
    await page
      .getByLabel("Fecha y hora de término *", { exact: true })
      .fill(invalidWallTime.end);
    await page
      .getByLabel("Motivo *", { exact: true })
      .fill("Prueba DST sintética");
    await page
      .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
      .click();
    await expect(
      page
        .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
        .getByRole("alert"),
    ).toHaveText(
      "La hora local no existe o se repite por un cambio horario. Elige otra hora.",
    );
    expect(attempts).toHaveLength(0);
  }

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
  await page.getByLabel("Motivo *", { exact: true }).fill(maximumLengthReason);
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
  expect(attempts[0]).toEqual({
    start_at: "2026-12-15T16:00:00Z",
    end_at: "2026-12-15T18:30:00Z",
    reason: maximumLengthReason,
  });
  expect(writes).toHaveLength(0);

  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Evento privado sintético");
  const readsBeforeCreateReadbackFailure = listReads;
  failNextBlockListRead = true;
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();

  await expect.poll(() => writes).toHaveLength(1);
  expect(attempts).toHaveLength(2);
  await expect.poll(() => listReads).toBe(readsBeforeCreateReadbackFailure + 1);
  await expect(
    page.getByText("Bloqueo manual guardado.", { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("No fue posible completar la operación. Intenta nuevamente.");
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
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeCreateReadbackFailure + 2);
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveCount(0);
  await expect(
    page.getByText("Evento privado sintético", { exact: true }),
  ).toBeVisible();
  expect(writes).toHaveLength(1);
  expect(attempts).toHaveLength(2);
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
  const readsBeforeDeleteReadbackFailure = listReads;
  failNextBlockListRead = true;
  await page
    .getByRole("button", { name: "Eliminar bloqueo Evento privado sintético" })
    .click();
  await expect.poll(() => deletions).toEqual([createdBlockId]);
  await expect.poll(() => listReads).toBe(readsBeforeDeleteReadbackFailure + 1);
  await expect(
    page.getByText("Bloqueo manual eliminado.", { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("No fue posible completar la operación. Intenta nuevamente.");
  await expect(
    page.getByText("Evento privado sintético", { exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeDeleteReadbackFailure + 2);
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveCount(0);
  await expect(
    page.getByText("Evento privado sintético", { exact: true }),
  ).toHaveCount(0);
  expect(deletions).toEqual([createdBlockId]);
  configuredTimeZone = null;
  const readsWithoutTimeZone = listReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect(
    page.getByText(
      "Guarda una zona horaria IANA válida antes de consultar o crear bloqueos por fecha.",
    ),
  ).toBeVisible();
  expect(listReads).toBe(readsWithoutTimeZone);
  await page.getByRole("button", { name: "Configurar zona horaria" }).click();
  await expect(
    page.getByRole("tab", { name: "Calendario y horarios" }),
  ).toHaveAttribute("aria-selected", "true");

  configuredTimeZone = "Invalid/Zone";
  const scheduleWritesBeforeInvalidZone = scheduleMutationAttempts;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await expect(page.locator("#editor-timezone")).toHaveValue("Invalid/Zone");
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeInvalidZone);
  await expect(
    page.getByRole("alert").filter({
      hasText:
        "La zona horaria debe ser un identificador IANA válido (ej: America/Santiago).",
    }),
  ).toBeVisible();

  const readsBeforeInvalidStoredZone = listReads;
  const blockAttemptsBeforeInvalidStoredZone = attempts.length;
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect(
    page.getByText(
      "Guarda una zona horaria IANA válida antes de consultar o crear bloqueos por fecha.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Actualizar fechas", exact: true }),
  ).toBeDisabled();
  expect(listReads).toBe(readsBeforeInvalidStoredZone);
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-22T09:00");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-22T10:00");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Zona inválida sintética");
  await expect(
    page.getByRole("button", { name: "Crear bloqueo", exact: true }),
  ).toBeDisabled();
  expect(attempts).toHaveLength(blockAttemptsBeforeInvalidStoredZone);
  await expect(
    page.getByText(
      "Guarda una zona horaria IANA válida antes de consultar o crear bloqueos por fecha.",
    ),
  ).toBeVisible();

  const readsBeforeValidZoneReload = listReads;
  const scheduleWritesBeforeValidZoneReload = scheduleMutationAttempts;
  configuredTimeZone = timeZone;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeValidZoneReload + 1);
  await expect(
    page.getByRole("button", { name: "Actualizar fechas", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Crear bloqueo", exact: true }),
  ).toBeEnabled();
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveCount(0);
  expect(attempts).toHaveLength(blockAttemptsBeforeInvalidStoredZone);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  await page.getByLabel("Desde", { exact: true }).fill("2026-12-10");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-20");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeValidZoneReload + 2);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-09T00:00:00.000Z",
    to: "2026-12-22T00:00:00.000Z",
  });

  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toHaveCount(0);
  const readsBeforeStaleRangeRace = listReads;
  const writesBeforeStaleRangeRace = writes.length;
  const attemptsBeforeStaleRangeRace = attempts.length;
  const deletionsBeforeStaleRangeRace = deletions.length;
  holdNextBlockListRead = true;
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeStaleRangeRace + 1);
  expect(releaseHeldBlockListReads).toHaveLength(1);
  blocks.push({
    id: externallyAddedBlockId,
    space_id: spaceId,
    start_at: "2026-12-18T09:00:00-03:00",
    end_at: "2026-12-18T10:00:00-03:00",
    time_zone: timeZone,
    reason: "Bloque externo sintético",
    created_at: "2026-11-11T12:00:00Z",
  });
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeStaleRangeRace + 2);
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toBeVisible();
  releaseHeldBlockListReads.shift()?.();
  expect(releaseHeldBlockListReads).toHaveLength(0);
  expect(writes).toHaveLength(writesBeforeStaleRangeRace);
  expect(attempts).toHaveLength(attemptsBeforeStaleRangeRace);
  expect(deletions).toHaveLength(deletionsBeforeStaleRangeRace);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toBeVisible();

  const readsBeforeChangedRangeRace = listReads;
  const writesBeforeChangedRangeRace = writes.length;
  const attemptsBeforeChangedRangeRace = attempts.length;
  const deletionsBeforeChangedRangeRace = deletions.length;
  holdNextBlockListRead = true;
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeChangedRangeRace + 1);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-09T00:00:00.000Z",
    to: "2026-12-22T00:00:00.000Z",
  });
  await expect(page.getByLabel("Desde", { exact: true })).toBeDisabled();
  await expect(page.getByLabel("Hasta", { exact: true })).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Actualizando…", exact: true }),
  ).toBeDisabled();
  releaseHeldBlockListReads.shift()?.();
  await expect(page.getByLabel("Desde", { exact: true })).toBeEnabled();
  await page.getByLabel("Desde", { exact: true }).fill("2026-12-24");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-26");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeChangedRangeRace + 2);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-23T00:00:00.000Z",
    to: "2026-12-28T00:00:00.000Z",
  });
  await expect(
    page.getByText("No hay bloqueos manuales para mostrar en este período.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText("No hay bloqueos manuales para mostrar en este período.", {
      exact: true,
    }),
  ).toBeVisible();
  expect(releaseHeldBlockListReads).toHaveLength(0);
  expect(writes).toHaveLength(writesBeforeChangedRangeRace);
  expect(attempts).toHaveLength(attemptsBeforeChangedRangeRace);
  expect(deletions).toHaveLength(deletionsBeforeChangedRangeRace);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  await page.getByLabel("Desde", { exact: true }).fill("2026-12-10");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-20");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeChangedRangeRace + 3);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-09T00:00:00.000Z",
    to: "2026-12-22T00:00:00.000Z",
  });
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toBeVisible();

  const readsBeforeRefreshFailure = listReads;
  const writesBeforeRefreshFailure = writes.length;
  const attemptsBeforeRefreshFailure = attempts.length;
  const deletionsBeforeRefreshFailure = deletions.length;
  const rangeBeforeRefreshFailure = listRanges.at(-1);
  failNextBlockListRead = true;
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeRefreshFailure + 1);
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveText("No fue posible completar la operación. Intenta nuevamente.");
  await expect(
    page.getByRole("button", { name: "Actualizar fechas", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toBeVisible();
  expect(listRanges.at(-1)).toEqual(rangeBeforeRefreshFailure);
  expect(writes).toHaveLength(writesBeforeRefreshFailure);
  expect(attempts).toHaveLength(attemptsBeforeRefreshFailure);
  expect(deletions).toHaveLength(deletionsBeforeRefreshFailure);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeRefreshFailure + 2);
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("alert"),
  ).toHaveCount(0);
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toBeVisible();
  expect(listRanges.at(-1)).toEqual(rangeBeforeRefreshFailure);
  expect(writes).toHaveLength(writesBeforeRefreshFailure);
  expect(attempts).toHaveLength(attemptsBeforeRefreshFailure);
  expect(deletions).toHaveLength(deletionsBeforeRefreshFailure);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  const readsBeforeEmptyValidRange = listReads;
  await page.getByLabel("Desde", { exact: true }).fill("2026-12-24");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-26");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeEmptyValidRange + 1);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-23T00:00:00.000Z",
    to: "2026-12-28T00:00:00.000Z",
  });
  await expect(
    page.getByText("No hay bloqueos manuales para mostrar en este período.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toHaveCount(0);
  expect(writes).toHaveLength(writesBeforeRefreshFailure);
  expect(attempts).toHaveLength(attemptsBeforeRefreshFailure);
  expect(deletions).toHaveLength(deletionsBeforeRefreshFailure);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  const readsBeforeFinalReload = listReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeFinalReload + 1);
  await expect(
    page.getByText("Mantenimiento de prueba", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque externo sintético", { exact: true }),
  ).toBeVisible();
  expect(writes).toHaveLength(writesBeforeRefreshFailure);
  expect(attempts).toHaveLength(attemptsBeforeRefreshFailure);
  expect(deletions).toHaveLength(deletionsBeforeRefreshFailure);
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  const readsBeforeKathmanduReload = listReads;
  configuredTimeZone = "Asia/Kathmandu";
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await expect(page.locator("#editor-timezone")).toHaveValue("Asia/Kathmandu");
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeKathmanduReload + 1);

  const attemptsBeforeKathmanduBlock = attempts.length;
  const writesBeforeKathmanduBlock = writes.length;
  const readsBeforeKathmanduBlock = listReads;
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-15T09:15");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-15T11:45");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloqueo con offset IANA fraccionario");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect
    .poll(() => attempts.length)
    .toBe(attemptsBeforeKathmanduBlock + 1);
  await expect.poll(() => writes.length).toBe(writesBeforeKathmanduBlock + 1);
  await expect.poll(() => listReads).toBe(readsBeforeKathmanduBlock + 1);
  const kathmanduPayload = {
    start_at: "2026-12-15T03:30:00Z",
    end_at: "2026-12-15T06:00:00Z",
    reason: "Bloqueo con offset IANA fraccionario",
  };
  expect(attempts.at(-1)).toEqual(kathmanduPayload);
  expect(writes.at(-1)).toEqual({
    body: kathmanduPayload,
    response: {
      id: createdKathmanduBlockId,
      space_id: spaceId,
      ...kathmanduPayload,
      time_zone: "Asia/Kathmandu",
      created_at: "2026-11-07T12:00:00Z",
    },
  });
  await expect(
    page.getByText("Bloqueo con offset IANA fraccionario", { exact: true }),
  ).toBeVisible();
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  const readsBeforeDstReload = listReads;
  configuredTimeZone = timeZone;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await expect(page.locator("#editor-timezone")).toHaveValue(timeZone);
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeDstReload + 1);
  await page.getByLabel("Desde", { exact: true }).fill("2026-09-05");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-09-06");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeDstReload + 2);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-09-04T00:00:00.000Z",
    to: "2026-09-08T00:00:00.000Z",
  });

  const attemptsBeforeDstBlock = attempts.length;
  const writesBeforeDstBlock = writes.length;
  const readsBeforeDstBlock = listReads;
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-09-05T23:30");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-09-06T01:30");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloqueo que cruza el cambio DST");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect.poll(() => attempts.length).toBe(attemptsBeforeDstBlock + 1);
  await expect.poll(() => writes.length).toBe(writesBeforeDstBlock + 1);
  await expect.poll(() => listReads).toBe(readsBeforeDstBlock + 1);
  const dstPayload = {
    start_at: "2026-09-06T03:30:00Z",
    end_at: "2026-09-06T04:30:00Z",
    reason: "Bloqueo que cruza el cambio DST",
  };
  expect(attempts.at(-1)).toEqual(dstPayload);
  expect(writes.at(-1)).toEqual({
    body: dstPayload,
    response: {
      id: createdDstBlockId,
      space_id: spaceId,
      ...dstPayload,
      time_zone: timeZone,
      created_at: "2026-11-07T12:00:00Z",
    },
  });
  await expect(
    page.getByText("Bloqueo que cruza el cambio DST", { exact: true }),
  ).toBeVisible();
  expect(scheduleMutationAttempts).toBe(scheduleWritesBeforeValidZoneReload);

  const attemptsBeforeAmbiguousDstBlock = attempts.length;
  const writesBeforeAmbiguousDstBlock = writes.length;
  const preconditionsBeforeAmbiguousDstBlock =
    manualBlockMutationPreconditions.length;
  const readsBeforeAmbiguousDstBlock = listReads;
  const scheduleMutationsBeforeAmbiguousDstBlock = scheduleMutationAttempts;
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-04-04T23:30");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-04-05T00:30");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloqueo en hora ambigua");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "La hora local no existe o se repite por un cambio horario. Elige otra hora.",
    }),
  ).toBeVisible();
  expect(attempts).toHaveLength(attemptsBeforeAmbiguousDstBlock);
  expect(writes).toHaveLength(writesBeforeAmbiguousDstBlock);
  expect(manualBlockMutationPreconditions).toHaveLength(
    preconditionsBeforeAmbiguousDstBlock,
  );
  expect(listReads).toBe(readsBeforeAmbiguousDstBlock);
  expect(scheduleMutationAttempts).toBe(
    scheduleMutationsBeforeAmbiguousDstBlock,
  );

  const attemptsBeforeNonexistentDstBlock = attempts.length;
  const writesBeforeNonexistentDstBlock = writes.length;
  const preconditionsBeforeNonexistentDstBlock =
    manualBlockMutationPreconditions.length;
  const readsBeforeNonexistentDstBlock = listReads;
  const scheduleMutationsBeforeNonexistentDstBlock = scheduleMutationAttempts;
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-09-06T01:30");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-09-06T01:00");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloqueo de prueba de rango");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText: "La fecha y hora de término debe ser posterior al inicio.",
    }),
  ).toBeVisible();
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-09-06T00:30");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-09-06T01:30");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloqueo en hora inexistente");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "La hora local no existe o se repite por un cambio horario. Elige otra hora.",
    }),
  ).toBeVisible();
  expect(attempts).toHaveLength(attemptsBeforeNonexistentDstBlock);
  expect(writes).toHaveLength(writesBeforeNonexistentDstBlock);
  expect(manualBlockMutationPreconditions).toHaveLength(
    preconditionsBeforeNonexistentDstBlock,
  );
  expect(listReads).toBe(readsBeforeNonexistentDstBlock);
  expect(scheduleMutationAttempts).toBe(
    scheduleMutationsBeforeNonexistentDstBlock,
  );

  const scheduleMutationsBeforeSavedZone = scheduleMutationAttempts;
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await page.locator("#editor-timezone").selectOption("Pacific/Easter");
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await expect(
    page.getByText("Horario semanal habitual y zona horaria guardados."),
  ).toBeVisible();
  expect(zoneConfigurationWrites.at(-1)).toEqual({
    body: { time_zone: "Pacific/Easter" },
    ifMatch: '"calendar-space-v1"',
  });
  await expect(page.locator("#editor-timezone")).toHaveValue("Pacific/Easter");

  const readsBeforeSavedZoneBlock = listReads;
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeSavedZoneBlock + 1);
  await page.getByLabel("Desde", { exact: true }).fill("2026-12-24");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-26");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeSavedZoneBlock + 2);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-23T00:00:00.000Z",
    to: "2026-12-28T00:00:00.000Z",
  });

  const attemptsBeforeSavedZoneBlock = attempts.length;
  const writesBeforeSavedZoneBlock = writes.length;
  const readsBeforeSavedZoneCreate = listReads;
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-25T09:15");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-25T10:45");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloque tras guardar zona");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect
    .poll(() => attempts.length)
    .toBe(attemptsBeforeSavedZoneBlock + 1);
  await expect.poll(() => writes.length).toBe(writesBeforeSavedZoneBlock + 1);
  await expect.poll(() => listReads).toBe(readsBeforeSavedZoneCreate + 1);
  const savedZonePayload = {
    start_at: "2026-12-25T14:15:00Z",
    end_at: "2026-12-25T15:45:00Z",
    reason: "Bloque tras guardar zona",
  };
  expect(attempts.at(-1)).toEqual(savedZonePayload);
  expect(writes.at(-1)).toEqual({
    body: savedZonePayload,
    response: {
      id: createdEasterBlockId,
      space_id: spaceId,
      ...savedZonePayload,
      time_zone: "Pacific/Easter",
      created_at: "2026-11-07T12:00:00Z",
    },
  });
  await expect(
    page.getByText("Bloque tras guardar zona", { exact: true }),
  ).toBeVisible();
  expect(scheduleMutationAttempts).toBe(scheduleMutationsBeforeSavedZone + 2);

  const writesBeforeUnsavedZoneBlock = writes.length;
  const attemptsBeforeUnsavedZoneBlock = attempts.length;
  const zoneWritesBeforeUnsavedZoneBlock = zoneConfigurationWrites.length;
  const scheduleMutationsBeforeUnsavedZoneBlock = scheduleMutationAttempts;
  const readsBeforeUnsavedZoneTab = listReads;
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await page.locator("#editor-timezone").selectOption("America/Santiago");
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Santiago",
  );
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeUnsavedZoneTab + 1);
  await page.getByLabel("Desde", { exact: true }).fill("2026-12-26");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-26");
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeUnsavedZoneTab + 2);
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-26T10:15");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-26T11:45");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloque con zona sin guardar");
  await page
    .getByRole("button", { name: /Guardar bloqueo|Crear bloqueo/i })
    .click();
  await expect
    .poll(() => attempts.length)
    .toBe(attemptsBeforeUnsavedZoneBlock + 1);
  await expect.poll(() => writes.length).toBe(writesBeforeUnsavedZoneBlock + 1);
  await expect.poll(() => listReads).toBe(readsBeforeUnsavedZoneTab + 3);
  const unsavedZonePayload = {
    start_at: "2026-12-26T15:15:00Z",
    end_at: "2026-12-26T16:45:00Z",
    reason: "Bloque con zona sin guardar",
  };
  expect(attempts.at(-1)).toEqual(unsavedZonePayload);
  expect(writes.at(-1)).toEqual({
    body: unsavedZonePayload,
    response: {
      id: createdUnsavedZoneBlockId,
      space_id: spaceId,
      ...unsavedZonePayload,
      time_zone: "Pacific/Easter",
      created_at: "2026-11-07T12:00:00Z",
    },
  });
  await expect(
    page.getByText("Bloque con zona sin guardar", { exact: true }),
  ).toBeVisible();
  expect(zoneConfigurationWrites).toHaveLength(
    zoneWritesBeforeUnsavedZoneBlock,
  );
  expect(scheduleMutationAttempts).toBe(
    scheduleMutationsBeforeUnsavedZoneBlock,
  );
  expect(manualBlockMutationPreconditions).toHaveLength(
    attempts.length + deletions.length,
  );
  expect(
    manualBlockMutationPreconditions.every(({ ifMatch }) => ifMatch === null),
  ).toBe(true);

  const attemptsBeforeStaleZoneConflict = attempts.length;
  const scheduleMutationsBeforeStaleZoneConflict = scheduleMutationAttempts;
  configuredTimeZone = "America/Santiago";
  listingEtag = '"calendar-space-external-v4"';
  weeklyEtag = listingEtag;
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "Este espacio cambió. Actualiza la información antes de volver a guardar.",
    }),
  ).toBeVisible();
  expect(zoneConfigurationWrites.at(-1)?.ifMatch).not.toBe(listingEtag);
  expect(scheduleMutationAttempts).toBe(
    scheduleMutationsBeforeStaleZoneConflict + 1,
  );

  const availabilityPanel = page.getByRole("tabpanel", {
    name: "Disponibilidad por fecha",
  });
  const readsBeforeStaleZonePanel = listReads;
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeStaleZonePanel + 1);
  await expect(availabilityPanel.getByRole("alert")).toHaveText(
    "Este espacio cambió. Recarga la página antes de crear bloqueos manuales.",
  );
  await expect(
    availabilityPanel.getByRole("button", { name: "Crear bloqueo" }),
  ).toBeDisabled();
  expect(attempts).toHaveLength(attemptsBeforeStaleZoneConflict);

  const staleZoneBlockReason = "Bloque con zona sin guardar";
  const staleZoneBlockDelete = availabilityPanel.getByRole("button", {
    name: `Eliminar bloqueo ${staleZoneBlockReason}`,
  });
  await expect(
    availabilityPanel.getByText(staleZoneBlockReason, { exact: true }),
  ).toBeVisible();
  await expect(staleZoneBlockDelete).toBeEnabled();
  const deletionsBeforeStaleZoneDelete = deletions.length;
  const preconditionsBeforeStaleZoneDelete =
    manualBlockMutationPreconditions.length;
  await staleZoneBlockDelete.click();
  await expect
    .poll(() => deletions.length)
    .toBe(deletionsBeforeStaleZoneDelete + 1);
  await expect(
    availabilityPanel.getByText(staleZoneBlockReason, { exact: true }),
  ).toHaveCount(0);
  expect(deletions.at(-1)).toBe(createdUnsavedZoneBlockId);
  expect(manualBlockMutationPreconditions).toHaveLength(
    preconditionsBeforeStaleZoneDelete + 1,
  );
  expect(manualBlockMutationPreconditions.at(-1)).toEqual({
    method: "DELETE",
    ifMatch: null,
  });
  expect(attempts).toHaveLength(attemptsBeforeStaleZoneConflict);

  const readsBeforeStaleZoneReload = listReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio sintético para calendario" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Calendario y horarios" }).click();
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Santiago",
  );
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => listReads).toBe(readsBeforeStaleZoneReload + 1);
  await expect(
    availabilityPanel.getByRole("button", { name: "Crear bloqueo" }),
  ).toBeEnabled();
  await expect(availabilityPanel.getByRole("alert")).toHaveCount(0);
  await page.getByLabel("Desde", { exact: true }).fill("2026-12-26");
  await page.getByLabel("Hasta", { exact: true }).fill("2026-12-26");
  const readsBeforeReloadedZoneRange = listReads;
  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeReloadedZoneRange + 1);
  expect(listRanges.at(-1)).toEqual({
    from: "2026-12-25T00:00:00.000Z",
    to: "2026-12-28T00:00:00.000Z",
  });
  await expect(
    page.getByRole("button", { name: "Actualizar fechas" }),
  ).toBeEnabled();
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-26T10:15");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-26T11:15");
  await page
    .getByLabel("Motivo *", { exact: true })
    .fill("Bloque después de recargar zona vigente");
  const attemptsBeforeReloadedZoneBlock = attempts.length;
  const writesBeforeReloadedZoneBlock = writes.length;
  const readsBeforeReloadedZoneBlock = listReads;
  await page.getByRole("button", { name: "Crear bloqueo" }).click();
  await expect
    .poll(() => attempts.length)
    .toBe(attemptsBeforeReloadedZoneBlock + 1);
  await expect
    .poll(() => writes.length)
    .toBe(writesBeforeReloadedZoneBlock + 1);
  await expect.poll(() => listReads).toBe(readsBeforeReloadedZoneBlock + 1);
  await expect(
    page.getByRole("button", { name: "Crear bloqueo" }),
  ).toBeEnabled();
  const reloadedZonePayload = {
    start_at: "2026-12-26T13:15:00Z",
    end_at: "2026-12-26T14:15:00Z",
    reason: "Bloque después de recargar zona vigente",
  };
  expect(attempts.at(-1)).toEqual(reloadedZonePayload);
  expect(writes.at(-1)).toEqual({
    body: reloadedZonePayload,
    response: {
      id: createdStaleZoneRecoveryBlockId,
      space_id: spaceId,
      ...reloadedZonePayload,
      time_zone: "America/Santiago",
      created_at: "2026-11-07T12:00:00Z",
    },
  });

  const ambiguousBlockReason = "Resultado incierto sintético";
  const attemptsBeforeAmbiguousCreate = attempts.length;
  const writesBeforeAmbiguousCreate = writes.length;
  const preconditionsBeforeAmbiguousCreate =
    manualBlockMutationPreconditions.length;
  const readsBeforeAmbiguousCreate = listReads;
  failNextBlockCreateAfterCommit = true;
  await page
    .getByLabel("Fecha y hora de inicio *", { exact: true })
    .fill("2026-12-26T15:30");
  await page
    .getByLabel("Fecha y hora de término *", { exact: true })
    .fill("2026-12-26T16:30");
  await page.getByLabel("Motivo *", { exact: true }).fill(ambiguousBlockReason);
  await page.getByRole("button", { name: "Crear bloqueo" }).click();
  await expect
    .poll(() => attempts.length)
    .toBe(attemptsBeforeAmbiguousCreate + 1);
  await expect(availabilityPanel.getByRole("alert")).toBeVisible();
  await expect.poll(() => listReads).toBe(readsBeforeAmbiguousCreate);
  await expect(
    availabilityPanel.getByText(ambiguousBlockReason, { exact: true }),
  ).toHaveCount(0);
  expect(writes).toHaveLength(writesBeforeAmbiguousCreate);

  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeAmbiguousCreate + 1);
  await expect(
    availabilityPanel.getByText(ambiguousBlockReason, { exact: true }),
  ).toBeVisible();
  expect(attempts).toHaveLength(attemptsBeforeAmbiguousCreate + 1);
  expect(writes).toHaveLength(writesBeforeAmbiguousCreate);
  expect(manualBlockMutationPreconditions).toHaveLength(
    preconditionsBeforeAmbiguousCreate + 1,
  );
  expect(manualBlockMutationPreconditions.at(-1)).toEqual({
    method: "POST",
    ifMatch: null,
  });
  expect(blocks.some((block) => block.id === ambiguousBlockId)).toBe(true);

  const preconditionsBeforeAmbiguousDelete =
    manualBlockMutationPreconditions.length;
  const deletionsBeforeAmbiguousDelete = deletions.length;
  const readsBeforeAmbiguousDelete = listReads;
  failNextBlockDeleteAfterCommit = true;
  await availabilityPanel
    .getByRole("button", { name: `Eliminar bloqueo ${ambiguousBlockReason}` })
    .click();
  await expect
    .poll(() => manualBlockMutationPreconditions.length)
    .toBe(preconditionsBeforeAmbiguousDelete + 1);
  await expect(availabilityPanel.getByRole("alert")).toBeVisible();
  await expect(
    availabilityPanel.getByText(ambiguousBlockReason, { exact: true }),
  ).toBeVisible();
  await expect.poll(() => listReads).toBe(readsBeforeAmbiguousDelete);
  expect(deletions).toHaveLength(deletionsBeforeAmbiguousDelete + 1);
  expect(blocks.some((block) => block.id === ambiguousBlockId)).toBe(false);

  await page
    .getByRole("button", { name: "Actualizar fechas", exact: true })
    .click();
  await expect.poll(() => listReads).toBe(readsBeforeAmbiguousDelete + 1);
  await expect(
    availabilityPanel.getByText(ambiguousBlockReason, { exact: true }),
  ).toHaveCount(0);
  expect(manualBlockMutationPreconditions).toHaveLength(
    preconditionsBeforeAmbiguousDelete + 1,
  );
  expect(manualBlockMutationPreconditions.at(-1)).toEqual({
    method: "DELETE",
    ifMatch: null,
  });
  expect(deletions).toHaveLength(deletionsBeforeAmbiguousDelete + 1);
});
