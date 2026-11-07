import { expect, test, type Request, type Route } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

const id = "00000000-0000-4000-8000-000000000042";
const account = "770e8400-e29b-41d4-a716-446655440000";

test("@phase7 @controlled editor schedule preserves fractional slots and does not replay CAS 412", async ({
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
  let schedulePuts = 0;
  let savedDays = [
    { weekday: 1, periods: [{ open: "09:30", close: "10:30" }] },
    { weekday: 2, periods: [{ open: "09:00", close: "13:00" }] },
    { weekday: 3, periods: [] },
    { weekday: 4, periods: [] },
    { weekday: 5, periods: [] },
    { weekday: 6, periods: [] },
    { weekday: 7, periods: [] },
  ];
  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const body = (request.postDataJSON?.() ?? {}) as Record<string, unknown>;
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
          time_zone: "America/Santiago",
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
    )
      response = {
        status: 200,
        headers: { etag: scheduleEtag },
        body: {
          data: {
            space_id: id,
            enabled: true,
            time_zone: "America/Santiago",
            days: savedDays,
          },
        },
      };
    else if (path === `/api/backend/spaces/${id}/gallery` && method === "GET")
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
      puts.push({ path, body, ifMatch: request.headers()["if-match"] ?? null });
      etag = '"schedule-editor-v2"';
      response = {
        status: 200,
        headers: { etag },
        body: { space_id: id, time_zone: "America/Santiago" },
      };
    } else if (
      path === `/api/backend/spaces/${id}/weekly-hours` &&
      method === "PUT"
    ) {
      schedulePuts++;
      puts.push({ path, body, ifMatch: request.headers()["if-match"] ?? null });
      if (schedulePuts === 1) {
        savedDays = body.days as typeof savedDays;
        scheduleEtag = '"schedule-saved-v2"';
        response = {
          status: 200,
          headers: { etag: scheduleEtag },
          body: {
            data: {
              space_id: id,
              enabled: true,
              time_zone: "America/Santiago",
              days: savedDays,
            },
          },
        };
      } else
        response = {
          status: 412,
          body: { error: { code: "precondition_failed", message: "Stale" } },
        };
    } else
      response = {
        status: 404,
        body: { error: { code: "not_found", message: "Missing fixture" } },
      };
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
  const editor = page.getByRole("region", { name: "Horario semanal habitual" });
  await expect(editor.getByRole("alert")).toContainText(
    "Cada día admite como máximo dos tramos horarios.",
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await expect(
    page.getByText("Horario semanal habitual y zona horaria guardados."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  await expect(
    page.getByRole("main").getByRole("alert").filter({
      hasText:
        "Este espacio cambió. Actualiza la información antes de volver a guardar.",
    }),
  ).toContainText(
    "Este espacio cambió. Actualiza la información antes de volver a guardar.",
  );
  await expect(editor.getByRole("alert")).toContainText(
    "Cada día admite como máximo dos tramos horarios.",
  );
  expect(
    puts.filter((entry) => entry.path.endsWith("/weekly-hours")),
  ).toHaveLength(2);
  const schedulePut = puts.find((entry) =>
    entry.path.endsWith("/weekly-hours"),
  );
  expect(schedulePut?.ifMatch).toBe('"schedule-editor-v2"');
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
});
