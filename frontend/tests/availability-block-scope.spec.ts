import { expect, test, type Request, type Route } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

const accountId = "770e8400-e29b-41d4-a716-446655440000";
const firstSpaceId = "00000000-0000-4000-8000-000000000081";
const secondSpaceId = "00000000-0000-4000-8000-000000000082";
const firstBlockId = "10000000-0000-4000-8000-000000000081";
const secondBlockId = "10000000-0000-4000-8000-000000000082";
const timeZone = "America/Santiago";

test("@controlled pending manual-block refresh stays scoped when navigating between same-zone spaces", async ({
  page,
}) => {
  const bff = createBffFixtureState();
  const reads: Array<{
    spaceId: string;
    from: string | null;
    to: string | null;
  }> = [];
  const mutations: string[] = [];
  let releaseFirstSpaceRead!: () => void;
  let signalFirstSpaceRead!: () => void;
  let signalFirstSpaceResponseAttempt!: () => void;
  const firstSpaceReadHeld = new Promise<void>((resolve) => {
    signalFirstSpaceRead = resolve;
  });
  const firstSpaceResponseGate = new Promise<void>((resolve) => {
    releaseFirstSpaceRead = resolve;
  });
  const firstSpaceResponseAttempted = new Promise<void>((resolve) => {
    signalFirstSpaceResponseAttempt = resolve;
  });

  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();
    const spaceId = [firstSpaceId, secondSpaceId].find((id) =>
      path.startsWith(`/api/backend/spaces/${id}`),
    );
    if (
      path.startsWith("/api/backend/spaces/") &&
      method !== "GET" &&
      !path.endsWith("/auth/login")
    ) {
      mutations.push(`${method} ${path}`);
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
    } else if (path === "/api/backend/spaces" && method === "GET") {
      response = {
        status: 200,
        body: {
          items: [firstSpaceId, secondSpaceId].map((id, index) => ({
            id,
            category_code: "oficina",
            category_name: "Oficinas",
            title: `Espacio de calendario ${index === 0 ? "A" : "B"}`,
            state: "borrador",
            base_price_clp: 10000,
            rate_unit: "hora",
            area_m2: 40,
            capacity: 4,
            region_code: "13",
            comuna_code: "13123",
          })),
        },
      };
    } else if (spaceId && path === `/api/backend/spaces/${spaceId}`) {
      const index = spaceId === firstSpaceId ? "A" : "B";
      response = {
        status: 200,
        headers: { etag: `"calendar-${index}-v1"` },
        body: {
          id: spaceId,
          category_code: "oficina",
          category_name: "Oficinas",
          title: `Espacio de calendario ${index}`,
          description: "Fixture local de calendario.",
          area_m2: 40,
          capacity: 4,
          usage_rules: "Fixture sintético",
          rate_unit: "hora",
          base_price_clp: 10000,
          address: "Dirección ficticia 123",
          state: "borrador",
          time_zone: timeZone,
          region_code: "13",
          comuna_code: "13123",
          attribute_schema_version: 1,
          attributes: {},
        },
      };
    } else if (spaceId && path === `/api/backend/spaces/${spaceId}/tariffs`) {
      response = { status: 200, body: { items: [] } };
    } else if (spaceId && path === `/api/backend/spaces/${spaceId}/tariff`) {
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
      spaceId &&
      path === `/api/backend/spaces/${spaceId}/weekly-hours`
    ) {
      response = {
        status: 200,
        headers: {
          etag: `"calendar-${spaceId === firstSpaceId ? "A" : "B"}-v1"`,
        },
        body: {
          data: {
            space_id: spaceId,
            enabled: true,
            time_zone: timeZone,
            days: Array.from({ length: 7 }, (_, index) => ({
              weekday: index + 1,
              periods: [],
            })),
          },
        },
      };
    } else if (spaceId && path === `/api/backend/spaces/${spaceId}/gallery`) {
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
      spaceId &&
      path === `/api/backend/spaces/${spaceId}/availability/blocks` &&
      method === "GET"
    ) {
      reads.push({
        spaceId,
        from: url.searchParams.get("from"),
        to: url.searchParams.get("to"),
      });
      const isFirstSpace = spaceId === firstSpaceId;
      if (
        isFirstSpace &&
        reads.filter((read) => read.spaceId === firstSpaceId).length === 1
      ) {
        signalFirstSpaceRead();
        await firstSpaceResponseGate;
      }
      response = {
        status: 200,
        body: {
          space_id: spaceId,
          time_zone: timeZone,
          items: [
            {
              id: isFirstSpace ? firstBlockId : secondBlockId,
              space_id: spaceId,
              start_at: "2026-12-10T09:00:00Z",
              end_at: "2026-12-10T10:00:00Z",
              time_zone: timeZone,
              reason: `Bloque exclusivo del espacio ${isFirstSpace ? "A" : "B"}`,
              created_at: "2026-11-01T12:00:00Z",
            },
          ],
        },
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
      if (
        spaceId === firstSpaceId &&
        path === `/api/backend/spaces/${firstSpaceId}/availability/blocks` &&
        method === "GET"
      ) {
        signalFirstSpaceResponseAttempt();
      }
      if (!request.failure()?.errorText?.toLowerCase().includes("abort")) {
        throw error;
      }
    }
    if (
      spaceId === firstSpaceId &&
      path === `/api/backend/spaces/${firstSpaceId}/availability/blocks` &&
      method === "GET"
    ) {
      signalFirstSpaceResponseAttempt();
    }
  });

  await page.goto(
    `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${firstSpaceId}/editar`)}`,
  );
  await page
    .getByLabel("Correo electrónico")
    .fill("calendar-scope@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL(`**/anfitrion/espacios/${firstSpaceId}/editar`);
  await expect(
    page.getByRole("heading", { name: "Espacio de calendario A" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await firstSpaceReadHeld;

  await page.getByRole("link", { name: "Mis Espacios", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Mis Espacios" }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "Espacio de calendario B", exact: true })
    .click();
  await page.waitForURL(`**/anfitrion/espacios/${secondSpaceId}/editar`);
  await expect(
    page.getByRole("heading", { name: "Espacio de calendario B" }),
  ).toBeVisible();
  await expect(
    page.getByText(`Zona: ${timeZone}`, { exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Disponibilidad por fecha" }).click();
  await expect(
    page.getByText("Bloque exclusivo del espacio B", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque exclusivo del espacio A", { exact: true }),
  ).toHaveCount(0);

  releaseFirstSpaceRead();
  await firstSpaceResponseAttempted;
  await expect(
    page.getByText("Bloque exclusivo del espacio B", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Bloque exclusivo del espacio A", { exact: true }),
  ).toHaveCount(0);
  expect(reads.map((read) => read.spaceId)).toEqual([
    firstSpaceId,
    secondSpaceId,
  ]);
  expect(reads.every(({ from, to }) => Boolean(from && to))).toBe(true);
  expect(mutations).toEqual([]);
});
