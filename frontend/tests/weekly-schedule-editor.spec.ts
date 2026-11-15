import { expect, test, type Request, type Route } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

const id = "00000000-0000-4000-8000-000000000042";
const account = "770e8400-e29b-41d4-a716-446655440000";
const manualBlockId = "10000000-0000-4000-8000-000000000001";

test("@phase7 @controlled editor saves an IANA zone, preserves fractional slots and does not replay CAS 412", async ({
  page,
}) => {
  const bff = createBffFixtureState();
  const puts: Array<{
    path: string;
    body: Record<string, unknown>;
    ifMatch: string | null;
  }> = [];
  let etag = '"schedule-editor-v1"';
  let scheduleEtag = '"schedule-editor-v1"';
  let configuredTimeZone = "America/Santiago";
  let availabilityPuts = 0;
  let weeklyReads = 0;
  let manualBlockListReads = 0;
  const manualBlockPosts: unknown[] = [];
  const manualBlockDeletes: string[] = [];
  const availabilityResponseEtags: string[] = [];
  const routeResponses: Array<{
    path: string;
    status: number;
    ifMatch: string | null;
  }> = [];
  const revisionEvents: string[] = [];
  let releaseWeeklyPut: (() => void) | undefined;
  const weeklyPutReleaseGate = new Promise<void>((resolve) => {
    releaseWeeklyPut = resolve;
  });
  let signalWeeklyPutHeld: (() => void) | undefined;
  const weeklyPutHeld = new Promise<void>((resolve) => {
    signalWeeklyPutHeld = resolve;
  });
  let savedDays = [
    { weekday: 1, periods: [{ open: "09:30", close: "10:30" }] },
    { weekday: 2, periods: [{ open: "09:00", close: "13:00" }] },
    { weekday: 3, periods: [] },
    { weekday: 4, periods: [] },
    { weekday: 5, periods: [] },
    { weekday: 6, periods: [] },
    { weekday: 7, periods: [] },
  ];
  let scheduleEnabled = true;
  let successfulWeeklySaves = 0;
  let failNextWeeklyRead = false;
  let failNextWeeklyPut = false;
  let failNextWeeklyPutAfterCommit = false;
  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const body = (request.postDataJSON?.() ?? {}) as Record<string, unknown>;
    if (
      path === `/api/backend/spaces/${id}/availability/blocks` &&
      method === "POST"
    ) {
      manualBlockPosts.push(body);
    }
    let response: {
      status: number;
      body?: unknown;
      headers?: Record<string, string>;
    };
    if (path === "/api/backend/auth/login" && method === "POST")
      response = {
        status: 200,
        body: {
          account_id: account,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600000).toISOString(),
        },
      };
    else if (path === "/api/backend/auth/session" && method === "GET")
      response = {
        status: 200,
        body: {
          account_id: account,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    else if (path === `/api/backend/spaces/${id}` && method === "GET")
      response = {
        status: 200,
        headers: { etag },
        body: {
          id,
          category_code: "oficina",
          category_name: "Oficinas",
          title: "Espacio prueba",
          description: "Descripción del espacio de prueba para edición",
          area_m2: 40,
          capacity: 4,
          usage_rules: "Respetar el espacio",
          rate_unit: "hora",
          base_price_clp: 10000,
          address: "Av. Prueba 123",
          state: "borrador",
          time_zone: configuredTimeZone,
          region_code: "13",
          comuna_code: "13123",
          attribute_schema_version: 1,
          attributes: {},
        },
      };
    else if (path === `/api/backend/spaces/${id}/tariffs` && method === "GET")
      response = { status: 200, body: { items: [] } };
    else if (path === `/api/backend/spaces/${id}/tariff` && method === "GET")
      response = {
        status: 200,
        body: {
          space_id: id,
          version: 1,
          rate_unit: "hora",
          base_price: 10000,
          currency: "CLP",
          created_at: "2026-10-01T00:00:00Z",
        },
      };
    else if (
      path === `/api/backend/spaces/${id}/weekly-hours` &&
      method === "GET"
    ) {
      weeklyReads += 1;
      if (failNextWeeklyRead) {
        failNextWeeklyRead = false;
        response = {
          status: 503,
          body: {
            error: { code: "unavailable", message: "Synthetic failure" },
          },
        };
      } else {
        response = {
          status: 200,
          headers: { etag: scheduleEtag },
          body: {
            data: {
              space_id: id,
              enabled: scheduleEnabled,
              time_zone: configuredTimeZone,
              days: savedDays,
            },
          },
        };
      }
    } else if (path === `/api/backend/spaces/${id}/gallery` && method === "GET")
      response = { status: 200, body: { items: [] } };
    else if (
      path === "/api/backend/spaces/categories/oficina/attributes/1" &&
      method === "GET"
    )
      response = {
        status: 200,
        body: { category_code: "oficina", schema_version: 1, attributes: [] },
      };
    else if (
      path === `/api/backend/spaces/${id}/availability` &&
      method === "PUT"
    ) {
      availabilityPuts++;
      const ifMatch = request.headers()["if-match"] ?? null;
      puts.push({ path, body, ifMatch });
      if (ifMatch !== etag) {
        response = {
          status: 412,
          body: { error: { code: "precondition_failed", message: "Stale" } },
        };
      } else {
        const requestedTimeZone = body.time_zone;
        if (
          typeof requestedTimeZone === "string" &&
          requestedTimeZone !== configuredTimeZone
        ) {
          configuredTimeZone = requestedTimeZone;
          etag = '"schedule-timezone-v2"';
          scheduleEtag = etag;
        }
        const responseEtag = etag;
        response = {
          status: 200,
          headers: { etag: responseEtag },
          body: { space_id: id, time_zone: configuredTimeZone },
        };
      }
    } else if (
      path === `/api/backend/spaces/${id}/weekly-hours` &&
      method === "PUT"
    ) {
      const ifMatch = request.headers()["if-match"] ?? null;
      puts.push({ path, body, ifMatch });
      if (availabilityPuts === 2) {
        signalWeeklyPutHeld?.();
        await weeklyPutReleaseGate;
      }
      if (failNextWeeklyPut) {
        failNextWeeklyPut = false;
        response = {
          status: 503,
          body: {
            error: { code: "unavailable", message: "Synthetic failure" },
          },
        };
      } else if (ifMatch !== etag) {
        if (availabilityPuts === 2)
          revisionEvents.push(`weekly-hours-412:${ifMatch}:${etag}`);
        response = {
          status: 412,
          body: { error: { code: "precondition_failed", message: "Stale" } },
        };
      } else if (failNextWeeklyPutAfterCommit) {
        failNextWeeklyPutAfterCommit = false;
        scheduleEnabled = body.enabled === true;
        savedDays = body.days as typeof savedDays;
        successfulWeeklySaves += 1;
        etag = '"schedule-ambiguous-v8"';
        scheduleEtag = etag;
        response = {
          status: 503,
          body: {
            error: {
              code: "unavailable",
              message: "Synthetic ambiguous commit",
            },
          },
        };
      } else {
        scheduleEnabled = body.enabled === true;
        savedDays = body.days as typeof savedDays;
        successfulWeeklySaves += 1;
        etag =
          successfulWeeklySaves === 1
            ? '"schedule-saved-v3"'
            : successfulWeeklySaves === 2
              ? '"schedule-disabled-v5"'
              : '"schedule-boundary-v6"';
        scheduleEtag = etag;
        response = {
          status: 200,
          headers: { etag: scheduleEtag },
          body: {
            data: {
              space_id: id,
              enabled: scheduleEnabled,
              time_zone: configuredTimeZone,
              days: savedDays,
            },
          },
        };
      }
    } else if (
      path.startsWith(`/api/backend/spaces/${id}/availability/blocks/`) &&
      method === "DELETE"
    ) {
      manualBlockDeletes.push(path.split("/").at(-1) ?? "");
      response = { status: 204 };
    } else if (
      path === `/api/backend/spaces/${id}/availability/blocks` &&
      method === "GET"
    ) {
      manualBlockListReads += 1;
      response = {
        status: 200,
        body: {
          space_id: id,
          time_zone: configuredTimeZone,
          items: [
            {
              id: manualBlockId,
              space_id: id,
              start_at: "2026-12-10T09:00:00-03:00",
              end_at: "2026-12-10T11:00:00-03:00",
              time_zone: configuredTimeZone,
              reason: "Bloque fixture existente",
              created_at: "2026-11-01T12:00:00Z",
            },
          ],
        },
      };
    } else
      response = {
        status: 404,
        body: { error: { code: "not_found", message: "Missing fixture" } },
      };
    const adapted = bff.adapt(request as Request, response);
    if (!adapted)
      throw new Error(`Missing fixture response: ${method} ${path}`);
    if (
      path === `/api/backend/spaces/${id}/availability` &&
      method === "PUT" &&
      adapted.status === 200
    ) {
      availabilityResponseEtags.push(adapted.headers?.etag ?? "");
    }
    if (path.endsWith("/weekly-hours") && method === "PUT")
      routeResponses.push({
        path,
        status: adapted.status,
        ifMatch: request.headers()["if-match"] ?? null,
      });
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
    `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${id}/editar`)}`,
  );
  await page
    .getByLabel("Correo electrónico")
    .fill("schedule-editor@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL(`**/anfitrion/espacios/${id}/editar`);
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await page.getByRole("button", { name: "Mañana 09:00–13:00" }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("09:00");
  await page.getByRole("button", { name: "Lunes", exact: true }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("09:30");
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await page.getByRole("button", { name: "19:00–20:00" }).click();
  await page.getByRole("button", { name: "21:00–22:00" }).click();
  await page.locator("#editor-timezone").selectOption("America/Coyhaique");
  const editor = page.getByRole("region", { name: "Horario semanal habitual" });
  await expect(editor.getByRole("alert")).toContainText(
    "Cada día admite como máximo dos tramos horarios.",
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await expect(
    page.getByText("Horario semanal habitual y zona horaria guardados."),
  ).toBeVisible();
  expect(
    puts.find((entry) => entry.path.endsWith("/availability"))?.body,
  ).toEqual({ time_zone: "America/Coyhaique" });
  expect(availabilityResponseEtags[0]).toBe('"schedule-timezone-v2"');
  expect(
    puts.find((entry) => entry.path.endsWith("/weekly-hours"))?.ifMatch,
  ).toBe(availabilityResponseEtags[0]);
  const weeklyReadsBeforeReload = weeklyReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect.poll(() => weeklyReads).toBeGreaterThan(weeklyReadsBeforeReload);
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Coyhaique",
  );
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("09:00");
  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue("13:00");
  await expect(page.getByLabel("Tramo 2: hora de inicio")).toHaveValue("19:00");
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:00");
  await expect(editor.getByRole("alert")).toHaveText("");

  // The second no-op timezone response races with another writer before weekly PUT.
  await page
    .getByLabel("Tramo 2: hora de cierre (24:00 permitido)")
    .fill("20:30");
  const secondAvailabilityResponse = page.waitForResponse((response) => {
    const request = response.request();
    return (
      new URL(response.url()).pathname ===
        `/api/backend/spaces/${id}/availability` &&
      request.method() === "PUT" &&
      response.status() === 200 &&
      response.headers().etag === '"schedule-saved-v3"'
    );
  });
  const staleWeeklyRequest = page.waitForRequest(
    (request) =>
      new URL(request.url()).pathname.endsWith(`/spaces/${id}/weekly-hours`) &&
      request.method() === "PUT",
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  const observedAvailabilityResponse = await secondAvailabilityResponse;
  const observedEtag = observedAvailabilityResponse.headers().etag;
  expect(observedAvailabilityResponse.status()).toBe(200);
  expect(observedEtag).toBe('"schedule-saved-v3"');
  revisionEvents.push(`availability-response-observed:${observedEtag}`);
  const observedWeeklyRequest = await staleWeeklyRequest;
  expect(observedWeeklyRequest.headers()["if-match"]).toBe(observedEtag);
  await weeklyPutHeld;
  await expect(
    page.getByRole("button", { name: "Guardar horarios" }),
  ).toBeDisabled();
  const pendingAvailabilityPanel = page.getByRole("tabpanel", {
    name: "Disponibilidad por fecha",
  });
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => manualBlockListReads).toBe(1);
  const pendingDelete = pendingAvailabilityPanel.getByRole("button", {
    name: "Eliminar bloqueo Bloque fixture existente",
  });
  await expect(pendingDelete).toBeEnabled();
  await expect(
    pendingAvailabilityPanel.getByRole("button", { name: "Crear bloqueo" }),
  ).toBeDisabled();
  expect(manualBlockPosts).toHaveLength(0);
  expect(manualBlockDeletes).toHaveLength(0);
  expect(etag).toBe(observedEtag);
  etag = '"schedule-external-v4"';
  scheduleEtag = etag;
  revisionEvents.push(`external-writer:${etag}`);
  releaseWeeklyPut?.();
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "Este espacio cambió. Actualiza la información antes de volver a guardar.",
    }),
  ).toContainText(
    "Este espacio cambió. Actualiza la información antes de volver a guardar.",
  );
  await expect(pendingDelete).toBeEnabled();
  expect(manualBlockDeletes).toHaveLength(0);
  await page.getByRole("tab", { name: /horarios/i }).click();
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(editor.getByRole("alert")).toHaveText("");
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:30");
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(2);
  const weeklyResponses = routeResponses.filter((entry) =>
    entry.path.endsWith("/weekly-hours"),
  );
  expect(weeklyResponses.map((entry) => entry.status)).toEqual([200, 412]);
  expect(weeklyResponses.filter((entry) => entry.status === 412)).toHaveLength(
    1,
  );
  expect(weeklyResponses[1]?.ifMatch).toBe(availabilityResponseEtags[1]);
  expect(
    puts.filter((entry) => entry.path.endsWith("/availability")),
  ).toHaveLength(2);
  expect(availabilityResponseEtags).toEqual([
    '"schedule-timezone-v2"',
    '"schedule-saved-v3"',
  ]);
  expect(revisionEvents).toEqual([
    'availability-response-observed:"schedule-saved-v3"',
    'external-writer:"schedule-external-v4"',
    'weekly-hours-412:"schedule-saved-v3":"schedule-external-v4"',
  ]);
  const schedulePut = puts.find((entry) =>
    entry.path.endsWith("/weekly-hours"),
  );
  expect(schedulePut?.ifMatch).toBe(availabilityResponseEtags[0]);
  expect(schedulePut?.body).toEqual({
    enabled: true,
    days: [
      { weekday: 1, periods: [{ open: "09:30", close: "10:30" }] },
      {
        weekday: 2,
        periods: [
          { open: "09:00", close: "13:00" },
          { open: "19:00", close: "20:00" },
        ],
      },
      { weekday: 3, periods: [] },
      { weekday: 4, periods: [] },
      { weekday: 5, periods: [] },
      { weekday: 6, periods: [] },
      { weekday: 7, periods: [] },
    ],
  });
  const stalePut = puts.at(-1);
  expect(stalePut?.ifMatch).toBe(availabilityResponseEtags[1]);
  expect(stalePut?.body).toEqual({
    enabled: true,
    days: [
      { weekday: 1, periods: [{ open: "09:30", close: "10:30" }] },
      {
        weekday: 2,
        periods: [
          { open: "09:00", close: "13:00" },
          { open: "19:00", close: "20:30" },
        ],
      },
      { weekday: 3, periods: [] },
      { weekday: 4, periods: [] },
      { weekday: 5, periods: [] },
      { weekday: 6, periods: [] },
      { weekday: 7, periods: [] },
    ],
  });
  const availabilityPanel = page.getByRole("tabpanel", {
    name: "Disponibilidad por fecha",
  });
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => manualBlockListReads).toBe(2);
  await expect(availabilityPanel.getByRole("alert")).toContainText(
    "Este espacio cambió. Recarga la página antes de crear bloqueos manuales.",
  );
  const calendarRefreshButton = availabilityPanel.getByRole("button", {
    name: "Actualizar fechas",
  });
  await expect(calendarRefreshButton).toBeEnabled();
  const manualBlockReadsBeforeStaleRefresh = manualBlockListReads;
  await calendarRefreshButton.click();
  await expect
    .poll(() => manualBlockListReads)
    .toBe(manualBlockReadsBeforeStaleRefresh + 1);
  await expect(calendarRefreshButton).toBeEnabled();
  await expect(availabilityPanel.getByRole("alert")).toContainText(
    "Este espacio cambió. Recarga la página antes de crear bloqueos manuales.",
  );
  await expect(
    availabilityPanel.getByRole("button", { name: "Crear bloqueo" }),
  ).toBeDisabled();
  expect(manualBlockPosts).toHaveLength(0);
  const weeklyReadsBeforeConflictReload = weeklyReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect.poll(() => manualBlockListReads).toBe(4);
  await expect(
    page
      .getByRole("tabpanel", { name: "Disponibilidad por fecha" })
      .getByRole("button", { name: "Crear bloqueo" }),
  ).toBeEnabled();
  expect(manualBlockPosts).toHaveLength(0);
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect
    .poll(() => weeklyReads)
    .toBeGreaterThan(weeklyReadsBeforeConflictReload);
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Coyhaique",
  );
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:00");

  const daysBeforeDisable = structuredClone(savedDays);
  const weeklyWritesBeforeDisable = puts.filter((entry) =>
    entry.path.endsWith("/weekly-hours"),
  ).length;
  const weeklyReadsBeforeDisableReload = weeklyReads;
  await page.getByLabel("Habilitar horario semanal").uncheck();
  const disabledSaveResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith(`/spaces/${id}/weekly-hours`) &&
      response.request().method() === "PUT" &&
      response.status() === 200,
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  const disabledResponse = await disabledSaveResponse;
  expect((await disabledResponse.json()).data).toMatchObject({
    enabled: false,
    days: daysBeforeDisable,
  });
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeDisable + 1);
  expect(puts.at(-1)?.body).toEqual({
    enabled: false,
    days: daysBeforeDisable,
  });
  expect(routeResponses.at(-1)).toMatchObject({ status: 200 });

  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect
    .poll(() => weeklyReads)
    .toBeGreaterThan(weeklyReadsBeforeDisableReload);
  await expect(page.getByLabel("Habilitar horario semanal")).not.toBeChecked();
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("09:00");
  await expect(page.getByLabel("Tramo 2: hora de inicio")).toHaveValue("19:00");
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:00");
  expect(scheduleEnabled).toBe(false);
  expect(savedDays).toEqual(daysBeforeDisable);

  await page.getByLabel("Habilitar horario semanal").check();
  await page.getByRole("button", { name: "Lunes", exact: true }).click();
  await page.getByRole("button", { name: "24 horas 00:00–24:00" }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("00:00");
  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue("24:00");

  const boundarySaveResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith(`/spaces/${id}/weekly-hours`) &&
      response.request().method() === "PUT" &&
      response.status() === 200,
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  const boundaryResponse = await boundarySaveResponse;
  expect(boundaryResponse.headers().etag).toBe('"schedule-boundary-v6"');
  expect(puts.at(-1)?.ifMatch).toBe('"schedule-disabled-v5"');
  expect(puts.at(-1)?.body).toEqual({
    enabled: true,
    days: [
      { weekday: 1, periods: [{ open: "00:00", close: "24:00" }] },
      {
        weekday: 2,
        periods: [
          { open: "09:00", close: "13:00" },
          { open: "19:00", close: "20:00" },
        ],
      },
      { weekday: 3, periods: [] },
      { weekday: 4, periods: [] },
      { weekday: 5, periods: [] },
      { weekday: 6, periods: [] },
      { weekday: 7, periods: [] },
    ],
  });

  const weeklyReadsBeforeBoundaryReload = weeklyReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect
    .poll(() => weeklyReads)
    .toBeGreaterThan(weeklyReadsBeforeBoundaryReload);
  await expect(page.getByLabel("Habilitar horario semanal")).toBeChecked();
  await page.getByRole("button", { name: "Lunes", exact: true }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("00:00");
  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue("24:00");

  // Another writer changes the shared revision before the next save.
  const weeklyWritesBeforeStaleZone = puts.filter((entry) =>
    entry.path.endsWith("/weekly-hours"),
  ).length;
  const previousEtag = etag;
  configuredTimeZone = "America/Punta_Arenas";
  etag = '"schedule-external-v7"';
  scheduleEtag = etag;
  await page.locator("#editor-timezone").selectOption("America/Santiago");
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await page
    .getByLabel("Tramo 2: hora de cierre (24:00 permitido)")
    .fill("20:30");
  const staleTimeZoneResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith(`/spaces/${id}/availability`) &&
      response.request().method() === "PUT" &&
      response.status() === 412,
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  const staleResponse = await staleTimeZoneResponse;
  expect(staleResponse.status()).toBe(412);
  expect(puts.at(-1)?.ifMatch).toBe(previousEtag);
  expect(puts.at(-1)?.body).toEqual({ time_zone: "America/Santiago" });
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "Este espacio cambió. Actualiza la información antes de volver a guardar.",
    }),
  ).toBeVisible();
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeStaleZone);
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:30");

  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Punta_Arenas",
  );
  await page.getByRole("button", { name: "Lunes", exact: true }).click();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("00:00");
  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue("24:00");
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:00");

  await page.locator("#editor-timezone").selectOption("America/Coyhaique");
  await page
    .getByLabel("Tramo 2: hora de cierre (24:00 permitido)")
    .fill("20:45");
  const weeklyWritesBeforeFailure = puts.filter((entry) =>
    entry.path.endsWith("/weekly-hours"),
  ).length;
  const availabilityPutsBeforeFailure = availabilityPuts;
  const weeklyReadsBeforeFailureReload = weeklyReads;
  failNextWeeklyPut = true;
  const failedWeeklyResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith(`/spaces/${id}/weekly-hours`) &&
      response.request().method() === "PUT" &&
      response.status() === 503,
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await failedWeeklyResponse;
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "La zona horaria se guardó, pero no se pudo confirmar el horario semanal. Recarga para verificar su estado antes de volver a guardar.",
    }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:45");
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeFailure + 1);
  expect(availabilityPuts).toBe(availabilityPutsBeforeFailure + 1);
  expect(configuredTimeZone).toBe("America/Coyhaique");
  expect(
    puts.filter((entry) => entry.path.endsWith("/availability")).at(-1)?.body,
  ).toEqual({ time_zone: "America/Coyhaique" });
  expect(routeResponses.at(-1)).toMatchObject({
    status: 503,
    ifMatch: etag,
  });

  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect
    .poll(() => weeklyReads)
    .toBeGreaterThan(weeklyReadsBeforeFailureReload);
  await expect(page.locator("#editor-timezone")).toHaveValue(
    "America/Coyhaique",
  );
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:00");
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeFailure + 1);
  expect(availabilityPuts).toBe(availabilityPutsBeforeFailure + 1);

  const readsBeforeFailedReload = weeklyReads;
  const writesBeforeFailedReload = puts.length;
  failNextWeeklyRead = true;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect.poll(() => weeklyReads).toBe(readsBeforeFailedReload + 1);
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "No se pudo cargar el horario semanal. Recarga la página antes de editarlo.",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Guardar horarios" }),
  ).toBeDisabled();
  await expect(page.locator("#editor-timezone")).toBeDisabled();
  await expect(page.getByLabel("Habilitar horario semanal")).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Martes", exact: true }),
  ).toBeDisabled();
  await page.locator("#panel-schedule form").evaluate((form) => {
    (form as HTMLFormElement).requestSubmit();
  });
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "No se pudo cargar el horario semanal. Recarga la página antes de editarlo.",
    }),
  ).toBeVisible();
  expect(puts).toHaveLength(writesBeforeFailedReload);
  expect(availabilityPuts).toBe(availabilityPutsBeforeFailure + 1);

  const readsBeforeRecoveredReload = weeklyReads;
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect.poll(() => weeklyReads).toBe(readsBeforeRecoveredReload + 1);
  await expect(
    page.getByRole("button", { name: "Guardar horarios" }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:00");
  expect(puts).toHaveLength(writesBeforeFailedReload);
  expect(availabilityPuts).toBe(availabilityPutsBeforeFailure + 1);

  await page
    .getByLabel("Tramo 2: hora de cierre (24:00 permitido)")
    .fill("20:15");
  const weeklyWritesBeforeAmbiguousCommit = puts.filter((entry) =>
    entry.path.endsWith("/weekly-hours"),
  ).length;
  const weeklyReadsBeforeAmbiguousCommit = weeklyReads;
  const ambiguousIfMatch = scheduleEtag;
  failNextWeeklyPutAfterCommit = true;
  const ambiguousWeeklyResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith(`/spaces/${id}/weekly-hours`) &&
      response.request().method() === "PUT" &&
      response.status() === 503,
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await ambiguousWeeklyResponse;
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "La zona horaria se guardó, pero no se pudo confirmar el horario semanal. Recarga para verificar su estado antes de volver a guardar.",
    }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:15");
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeAmbiguousCommit + 1);
  expect(puts.at(-1)?.ifMatch).toBe(ambiguousIfMatch);
  expect(puts.at(-1)?.body).toMatchObject({
    enabled: true,
    days: expect.arrayContaining([
      {
        weekday: 2,
        periods: [
          { open: "09:00", close: "13:00" },
          { open: "19:00", close: "20:15" },
        ],
      },
    ]),
  });
  expect(
    savedDays.find((day) => day.weekday === 2)?.periods.at(-1)?.close,
  ).toBe("20:15");

  const weeklyWritesBeforeStaleRetry = puts.filter((entry) =>
    entry.path.endsWith("/weekly-hours"),
  ).length;
  const availabilityPutsBeforeStaleRetry = availabilityPuts;
  const weeklyResponsesBeforeStaleRetry = routeResponses.length;
  const staleRetryResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith(`/spaces/${id}/availability`) &&
      response.request().method() === "PUT" &&
      response.status() === 412,
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await staleRetryResponse;
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "Este espacio cambió. Actualiza la información antes de volver a guardar.",
    }),
  ).toBeVisible();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:15");
  expect(availabilityPuts).toBe(availabilityPutsBeforeStaleRetry + 1);
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeStaleRetry);
  expect(routeResponses).toHaveLength(weeklyResponsesBeforeStaleRetry);
  expect(puts.at(-1)?.path).toBe(`/api/backend/spaces/${id}/availability`);
  expect(puts.at(-1)?.ifMatch).toBe(ambiguousIfMatch);

  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Espacio prueba" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect
    .poll(() => weeklyReads)
    .toBeGreaterThan(weeklyReadsBeforeAmbiguousCommit);
  await page.getByRole("button", { name: "Martes", exact: true }).click();
  await expect(
    page.getByLabel("Tramo 2: hora de cierre (24:00 permitido)"),
  ).toHaveValue("20:15");
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(weeklyWritesBeforeAmbiguousCommit + 1);
});
