import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import type { components } from "../src/lib/api/generated/openapi";
import { createBffFixtureState } from "./helpers/bff-fixture";

const activeId = "00000000-0000-4000-8000-000000000001";
const missingId = "00000000-0000-4000-8000-000000000002";
const inactiveId = "00000000-0000-4000-8000-000000000003";
const unavailableSpaceId = "00000000-0000-4000-8000-000000000004";
const invalidTimeZoneId = "00000000-0000-4000-8000-000000000005";
const supportedAliasId = "00000000-0000-4000-8000-000000000006";
const fixtureBaseUrl = `http://127.0.0.1:${process.env.PUBLIC_CALENDAR_FIXTURE_PORT ?? 18088}`;

test("public detail hides unknown private fields and shows exact weekly periods", async ({
  page,
}) => {
  const privateMarkers = [
    "PRIVATE_OWNER_ID_MARKER",
    "PRIVATE_ADDRESS_MARKER",
    "PRIVATE_CONTACT_MARKER",
  ];
  const upstream = await page.request.get(
    `${fixtureBaseUrl}/api/v1/public/spaces/${activeId}`,
  );
  expect(upstream.status()).toBe(200);
  const upstreamPayload = await upstream.json();
  for (const marker of privateMarkers)
    expect(JSON.stringify(upstreamPayload)).toContain(marker);

  const writes: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method()))
      writes.push(`${request.method()} ${request.url()}`);
  });
  for (const viewport of [
    { width: 1365, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const response = await page.goto(`/espacios/${activeId}`);
    expect(response?.status()).toBe(200);
    const renderedHtml = (await response?.text()) ?? "";
    for (const marker of privateMarkers)
      expect(renderedHtml).not.toContain(marker);
    await expect(
      page.getByRole("heading", { name: "Horario semanal habitual", level: 3 }),
    ).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    const dayButton = page.getByRole("button", { name: "Martes" });
    const beforePressed = await dayButton.getAttribute("aria-pressed");
    await dayButton.click();
    await expect(dayButton).toHaveAttribute(
      "aria-pressed",
      beforePressed === "true" ? "false" : "true",
    );
    const hydratedHtml = await page.locator("html").innerHTML();
    for (const marker of privateMarkers)
      expect(hydratedHtml).not.toContain(marker);
    await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue(
      "14:15",
    );
    await expect(
      page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
    ).toHaveValue("16:45");
    const partialStartHour = page.getByRole("button", {
      name: /14:00–15:00/,
    });
    await expect(partialStartHour).toHaveAttribute("aria-pressed", "mixed");
    await expect(partialStartHour).toContainText("Parcial");
    await expect(partialStartHour).toHaveAccessibleName(/14:15–16:45/);
    const fullHour = page.getByRole("button", { name: /15:00–16:00/ });
    await expect(fullHour).toHaveAttribute("aria-pressed", "true");
    await expect(fullHour).not.toContainText("Parcial");
    const partialEndHour = page.getByRole("button", {
      name: /16:00–17:00/,
    });
    await expect(partialEndHour).toHaveAttribute("aria-pressed", "mixed");
    await expect(partialEndHour).toContainText("Parcial");
    await expect(partialEndHour).toHaveAccessibleName(/14:15–16:45/);
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({
      path: test.info().outputPath(`public-calendar-${viewport.width}.png`),
      fullPage: true,
    });
    expect(
      await page
        .locator("[aria-label='Día de la semana'] button")
        .evaluateAll((els) =>
          els.every((el) => {
            const r = el.getBoundingClientRect();
            return r.width >= 44 && r.height >= 44;
          }),
        ),
    ).toBeTruthy();
    await expect(
      page.getByRole("button", { name: /Mañana 09:00/ }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: /14:00–15:00/ }),
    ).toBeDisabled();
    expect(
      await page.evaluate(
        () =>
          document.documentElement.scrollWidth <=
          document.documentElement.clientWidth,
      ),
    ).toBeTruthy();
    const a11y = await new AxeBuilder({ page }).analyze();
    expect(a11y.violations).toEqual([]);
  }
  expect(writes).toEqual([]);
});

test("absent and inactive schedules are presented honestly", async ({
  page,
}) => {
  await page.goto(`/espacios/${missingId}`);
  await expect(page.getByText("Horario no informado.")).toBeVisible();
  await page.goto(`/espacios/${inactiveId}`);
  await expect(page.getByText("Horario inactivo.")).toBeVisible();
  await expect(
    page.getByText(/no confirma disponibilidad para fechas específicas/),
  ).toBeVisible();
});

test("an invalid public schedule timezone is omitted in the rendered detail", async ({
  page,
}) => {
  const writes: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method()))
      writes.push(`${request.method()} ${request.url()}`);
  });

  const response = await page.goto(`/espacios/${invalidTimeZoneId}`);

  expect(response?.status()).toBe(200);
  await expect(page.getByText("Horario no informado.")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual" }),
  ).toHaveCount(0);
  await expect(page.getByText("14:15", { exact: true })).toHaveCount(0);
  expect(writes).toEqual([]);
});

test("a runtime-supported timezone alias remains visible in the public detail", async ({
  page,
}) => {
  const writes: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method()))
      writes.push(`${request.method()} ${request.url()}`);
  });

  const response = await page.goto(`/espacios/${supportedAliasId}`);

  expect(response?.status()).toBe(200);
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual", level: 3 }),
  ).toBeVisible();
  const tuesday = page.getByRole("button", { name: "Martes" });
  await expect(tuesday).toBeVisible();
  const beforePressed = await tuesday.getAttribute("aria-pressed");
  await tuesday.click();
  await expect(tuesday).toHaveAttribute(
    "aria-pressed",
    beforePressed === "true" ? "false" : "true",
  );
  await expect(page.getByText(/Zona horaria: EST/)).toBeVisible();
  await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue("14:15");
  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue("16:45");
  await expect(page.getByText("Horario no informado.")).toHaveCount(0);
  expect(writes).toEqual([]);
});

test("an unavailable public detail returns 404 without rendering space content", async ({
  page,
}) => {
  const observationsUrl = `${fixtureBaseUrl}/__fixture/observations`;
  const baselineResponse = await page.request.get(observationsUrl);
  expect(baselineResponse.status()).toBe(200);
  const baseline = await baselineResponse.json();
  expect(baseline.unavailableDetailReads).toBe(0);

  const activeResponse = await page.goto(`/espacios/${activeId}`);
  expect(activeResponse?.status()).toBe(200);
  await expect(
    page.getByRole("heading", { name: "Espacio sintético", level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual", level: 3 }),
  ).toBeVisible();

  const response = await page.goto(`/espacios/${unavailableSpaceId}`);

  expect(response?.status()).toBe(404);
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual" }),
  ).toHaveCount(0);
  await expect(
    page.getByText("Espacio sintético", { exact: true }),
  ).toHaveCount(0);

  const fixtureObservation = await page.request.get(observationsUrl);
  expect(fixtureObservation.status()).toBe(200);
  const observation = await fixtureObservation.json();
  expect(observation.unavailableDetailReads).toBeGreaterThan(
    baseline.unavailableDetailReads,
  );
  expect(observation.unavailableDetailStatus).toBe(404);
  expect(observation.unavailableDetailId).toBe(unavailableSpaceId);
});

test("active owner listing CTA reaches the public SSR detail fixture", async ({
  page,
}) => {
  const bff = createBffFixtureState();
  const observationsUrl = `${fixtureBaseUrl}/__fixture/observations`;
  const baselineResponse = await page.request.get(observationsUrl);
  expect(baselineResponse.status()).toBe(200);
  const baselineObservations = await baselineResponse.json();
  const accountId = "770e8400-e29b-41d4-a716-446655440000";
  const listing = {
    id: activeId,
    category_code: "oficina",
    category_name: "Oficinas y Cowork",
    title: "Espacio sintético",
    description:
      "Espacio sintético de oficina destinado exclusivamente a validar el contrato de listado del anfitrión en un ensayo local.",
    state: "activa",
    base_price_clp: 12000,
    rate_unit: "hora",
    area_m2: 12,
    capacity: 1,
    usage_rules: "Uso sintético de prueba.",
    address: "Avenida Sintética 123",
    region_code: "13",
    comuna_code: "13123",
    attribute_schema_version: 1,
    attributes: {},
  } satisfies components["schemas"]["SpaceDraft"];
  const listingResponse = {
    items: [listing],
  } satisfies components["schemas"]["SpaceDraftList"];
  const apiWrites: string[] = [];
  const publicationWrites: Array<{ state: unknown; ifMatch: string }> = [];
  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    if (
      pathname.startsWith("/api/backend/") &&
      !["GET", "HEAD", "OPTIONS"].includes(method)
    ) {
      apiWrites.push(`${method} ${pathname}`);
    }
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    let result: {
      status: number;
      body: Record<string, unknown>;
      headers?: Record<string, string>;
    } | null = null;

    if (pathname === "/api/backend/auth/login" && request.method() === "POST") {
      result = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["anfitrion"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
        },
      };
    } else if (
      pathname === "/api/backend/auth/session" &&
      request.method() === "GET"
    ) {
      result = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["anfitrion"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    } else if (
      pathname === "/api/backend/spaces" &&
      request.method() === "GET"
    ) {
      result = { status: 200, body: listingResponse };
    } else if (
      pathname === `/api/backend/spaces/${activeId}` &&
      request.method() === "GET"
    ) {
      result = {
        status: 200,
        body: listing,
        headers: { etag: '"active-editor-v1"' },
      };
    } else if (
      pathname === `/api/backend/spaces/${activeId}/publication` &&
      request.method() === "PUT"
    ) {
      const body = request.postDataJSON() as { state?: unknown };
      const ifMatch = request.headers()["if-match"] ?? "";
      publicationWrites.push({ state: body.state, ifMatch });
      const archiveFixture = await page.request.post(
        `${fixtureBaseUrl}/__fixture/archive/${activeId}`,
      );
      if (archiveFixture.status() !== 204)
        throw new Error("Failed to transition the public fixture to archived");
      result = {
        status: 200,
        body: { ...listing, state: "oculta" },
        headers: { etag: '"archived-editor-v2"' },
      };
    } else {
      result = {
        status: 404,
        body: { error: { code: "not_found" } },
      };
    }

    const adapted = bff.adapt(request, result);
    if (!adapted) throw new Error("Missing controlled BFF response");
    await route.fulfill({
      status: adapted.status,
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        ...(adapted.headers ?? {}),
      },
      body: adapted.status === 204 ? "" : JSON.stringify(adapted.body ?? {}),
    });
  });

  const hostListingsPath = "/anfitrion/espacios";
  await page.goto(`/acceso?return_to=${encodeURIComponent(hostListingsPath)}`);
  await page.getByLabel("Correo electrónico").fill("host@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === hostListingsPath);

  await expect(page.getByText("Publicado", { exact: true })).toBeVisible();
  const publicLink = page.getByRole("link", {
    name: `Ver publicación de ${listing.title}`,
  });
  await expect(publicLink).toHaveAttribute("href", `/espacios/${activeId}`);
  await publicLink.click();

  await expect(page).toHaveURL(new RegExp(`/espacios/${activeId}$`));
  await expect(
    page.getByRole("heading", { name: listing.title, level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual", level: 3 }),
  ).toBeVisible();

  await page.goto(`/anfitrion/espacios/${activeId}/editar`);
  const editorPublicLink = page.getByRole("link", {
    name: "Ver publicación",
  });
  await expect(editorPublicLink).toHaveAttribute(
    "href",
    `/espacios/${activeId}`,
  );
  await editorPublicLink.click();
  await expect(page).toHaveURL(new RegExp(`/espacios/${activeId}$`));
  await expect(
    page.getByRole("heading", { name: listing.title, level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual", level: 3 }),
  ).toBeVisible();

  const catalogQuery = "Espacio sintético";
  const activeCatalogBaselineResponse = await page.request.get(observationsUrl);
  expect(activeCatalogBaselineResponse.status()).toBe(200);
  const activeCatalogBaseline = await activeCatalogBaselineResponse.json();
  await page.goto(`/explorar?q=${encodeURIComponent(catalogQuery)}`);
  const catalogListing = page.getByRole("link", {
    name: /Espacio sintético/,
  });
  await expect(catalogListing).toHaveAttribute("href", `/espacios/${activeId}`);
  const activeCatalogObservationResponse =
    await page.request.get(observationsUrl);
  expect(activeCatalogObservationResponse.status()).toBe(200);
  const activeCatalogObservation =
    await activeCatalogObservationResponse.json();
  expect(activeCatalogObservation.publicListReads).toBeGreaterThan(
    activeCatalogBaseline.publicListReads,
  );
  expect(activeCatalogObservation.lastPublicListQuery).toBe(catalogQuery);
  expect(activeCatalogObservation.lastPublicListItems).toBe(1);

  await page.goto(`/anfitrion/espacios/${activeId}/editar`);
  await page.getByRole("button", { name: "Archivar", exact: true }).click();
  await expect(
    page.getByText(
      "El espacio ha sido archivado (oculto del catálogo público).",
      { exact: true },
    ),
  ).toBeVisible();
  expect(publicationWrites).toEqual([
    { state: "oculta", ifMatch: '"active-editor-v1"' },
  ]);
  await expect(page.getByRole("link", { name: "Ver publicación" })).toHaveCount(
    0,
  );

  const archivedResponse = await page.goto(`/espacios/${activeId}`);
  expect(archivedResponse?.status()).toBe(404);
  await expect(
    page.getByRole("heading", { name: "Horario semanal habitual" }),
  ).toHaveCount(0);
  await expect(page.getByText(listing.title, { exact: true })).toHaveCount(0);
  const archiveObservationResponse = await page.request.get(observationsUrl);
  expect(archiveObservationResponse.status()).toBe(200);
  const archiveObservation = await archiveObservationResponse.json();
  expect(archiveObservation.archivedDetailReads).toBeGreaterThan(
    baselineObservations.archivedDetailReads,
  );
  expect(archiveObservation.archivedDetailStatus).toBe(404);
  expect(archiveObservation.archivedDetailId).toBe(activeId);

  await page.goto(`/explorar?q=${encodeURIComponent(catalogQuery)}`);
  await expect(
    page.getByRole("heading", {
      name: `No encontramos publicaciones para “${catalogQuery}”`,
    }),
  ).toBeVisible();
  const catalogObservationResponse = await page.request.get(observationsUrl);
  expect(catalogObservationResponse.status()).toBe(200);
  const catalogObservation = await catalogObservationResponse.json();
  expect(catalogObservation.publicListReads).toBeGreaterThan(
    activeCatalogObservation.publicListReads,
  );
  expect(catalogObservation.lastPublicListQuery).toBe(catalogQuery);
  expect(catalogObservation.lastPublicListItems).toBe(0);

  expect(apiWrites).toEqual([
    "POST /api/backend/auth/login",
    `PUT /api/backend/spaces/${activeId}/publication`,
  ]);
});
