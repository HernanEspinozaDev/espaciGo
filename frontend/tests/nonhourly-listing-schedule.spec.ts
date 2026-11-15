import { expect, test, type Request, type Route } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

const id = "00000000-0000-4000-8000-000000000042";
const account = "770e8400-e29b-41d4-a716-446655440000";

test.use({ trace: "off", screenshot: "off", video: "off" });

for (const rateUnit of ["dia", "mes"] as const) {
  test(`@phase7 @controlled editor ${rateUnit}: rehidrata zona horaria sin leer ni guardar horario semanal`, async ({
    page,
  }) => {
    const bff = createBffFixtureState();
    const requests: Array<{ method: string; path: string }> = [];
    const writes: Array<{
      path: string;
      body: Record<string, unknown>;
      ifMatch: string | null;
    }> = [];
    let listingEtag = '"nonhourly-listing-v1"';
    let configuredTimeZone = "America/Santiago";

    await page.route("**/api/backend/**", async (route: Route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const method = request.method();
      requests.push({ method, path });
      let response: {
        status: number;
        body?: unknown;
        headers?: Record<string, string>;
      };
      if (path === "/api/backend/auth/login" && method === "POST") {
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
      } else if (path === "/api/backend/auth/session" && method === "GET") {
        response = {
          status: 200,
          body: {
            account_id: account,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          },
        };
      } else if (path === `/api/backend/spaces/${id}` && method === "GET") {
        response = {
          status: 200,
          headers: { etag: listingEtag },
          body: {
            id,
            category_code: "oficina",
            category_name: "Oficinas",
            title: "Espacio de prueba",
            description:
              "Descripción sintética del espacio para comprobar el horario.",
            area_m2: 40,
            capacity: 4,
            usage_rules: "Respetar el espacio",
            rate_unit: rateUnit,
            base_price_clp: 10000,
            address: "Av. Prueba 123",
            state: "borrador",
            time_zone: configuredTimeZone,
            region_code: "13",
            comuna_code: "13101",
            attribute_schema_version: 1,
            attributes: {},
          },
        };
      } else if (
        path === `/api/backend/spaces/${id}/tariffs` &&
        method === "GET"
      ) {
        response = { status: 200, body: { items: [] } };
      } else if (
        path === `/api/backend/spaces/${id}/tariff` &&
        method === "GET"
      ) {
        response = {
          status: 200,
          body: {
            space_id: id,
            version: 1,
            rate_unit: rateUnit,
            base_price: 10000,
            currency: "CLP",
            created_at: "2026-10-01T00:00:00Z",
          },
        };
      } else if (
        path === `/api/backend/spaces/${id}/gallery` &&
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
        path === `/api/backend/spaces/${id}/availability` &&
        method === "PUT"
      ) {
        writes.push({
          path,
          body: (request.postDataJSON?.() ?? {}) as Record<string, unknown>,
          ifMatch: request.headers()["if-match"] ?? null,
        });
        configuredTimeZone = String(writes.at(-1)?.body.time_zone);
        listingEtag = '"nonhourly-listing-v2"';
        response = {
          status: 200,
          headers: { etag: listingEtag },
          body: { space_id: id, time_zone: configuredTimeZone },
        };
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
      `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${id}/editar`)}`,
    );
    await page
      .getByLabel("Correo electrónico")
      .fill("nonhourly-editor@example.invalid");
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("SyntheticPass123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    await page.waitForURL(`**/anfitrion/espacios/${id}/editar`);
    await page.getByRole("tab", { name: /horarios/i }).click();

    await expect(
      page.getByText("El horario semanal solo aplica a tarifas por hora.", {
        exact: false,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Disponibilidad semanal" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Guardar zona horaria" }),
    ).toBeVisible();
    await expect(
      page.getByText("Configura la zona horaria del espacio.", { exact: true }),
    ).toHaveCount(0);
    const timezone = page.getByLabel(/Zona horaria/i);
    await expect(timezone).toHaveValue("America/Santiago");
    await timezone.selectOption("America/Punta_Arenas");
    await page.getByRole("button", { name: /Guardar zona horaria/i }).click();
    await expect(page.getByRole("main").getByRole("status")).toContainText(
      /zona horaria/i,
    );
    expect(writes).toHaveLength(1);
    expect(writes[0]).toMatchObject({
      path: `/api/backend/spaces/${id}/availability`,
      ifMatch: '"nonhourly-listing-v1"',
    });
    expect(writes[0]!.body).toEqual({ time_zone: "America/Punta_Arenas" });
    expect(requests.some(({ path }) => path.endsWith("/weekly-hours"))).toBe(
      false,
    );

    await page.reload();
    await expect(
      page.getByRole("heading", { name: "Espacio de prueba" }),
    ).toBeVisible();
    await page.getByRole("tab", { name: /horarios/i }).click();
    await expect(page.getByLabel(/Zona horaria/i)).toHaveValue(
      "America/Punta_Arenas",
    );
    expect(requests.some(({ path }) => path.endsWith("/weekly-hours"))).toBe(
      false,
    );
    expect(writes).toHaveLength(1);
  });
}
