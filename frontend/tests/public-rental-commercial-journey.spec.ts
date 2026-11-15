import { expect, test } from "@playwright/test";
import type { components } from "../src/lib/api/generated/openapi";
import { createBffFixtureState } from "./helpers/bff-fixture";

const spaceId = "00000000-0000-4000-8000-000000000001";
const quoteId = "00000000-0000-4000-8000-000000000101";
const accountId = "770e8400-e29b-41d4-a716-446655440000";
const secondAccountId = "880e8400-e29b-41d4-a716-446655440000";

function localCivilDate(offsetDays: number): string {
  const parts = new Intl.DateTimeFormat("en", {
    timeZone: "America/Santiago",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const year = Number(parts.find((part) => part.type === "year")?.value);
  const month = Number(parts.find((part) => part.type === "month")?.value);
  const day = Number(parts.find((part) => part.type === "day")?.value);
  const date = new Date(Date.UTC(year, month - 1, day + offsetDays));
  return date.toISOString().slice(0, 10);
}

test("public rental journey returns from login, requests a quote and opens its contract preview", async ({
  page,
  context,
}, testInfo) => {
  const bff = createBffFixtureState();
  const today = localCivilDate(0);
  const tomorrow = localCivilDate(1);
  const startAt = `${tomorrow}T12:00:00Z`;
  const endAt = `${tomorrow}T13:00:00Z`;
  const quoteRequests: Array<Record<string, unknown>> = [];
  const previewRequests: string[] = [];
  const browserAuthHeaders: Array<string | undefined> = [];
  let activeAccountId = accountId;

  await context.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const pathname = url.pathname;
    const method = request.method();
    let result: {
      status: number;
      body: unknown;
      headers?: Record<string, string>;
    } = { status: 404, body: { error: { code: "not_found" } } };

    if (pathname === "/api/backend/auth/logout" && method === "POST") {
      result = { status: 204, body: undefined };
    } else if (pathname === "/api/backend/auth/login" && method === "POST") {
      result = {
        status: 200,
        body: {
          account_id: activeAccountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
        },
      };
    } else if (pathname === "/api/backend/auth/session" && method === "GET") {
      result = {
        status: 200,
        body: {
          account_id: activeAccountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    } else if (
      pathname === `/api/backend/spaces/${spaceId}/rental-availability` &&
      method === "GET"
    ) {
      const selectedDate = url.searchParams.get("date");
      if (!selectedDate) throw new Error("Availability request omitted date");
      const availability = {
        space_id: spaceId,
        time_zone: "America/Santiago",
        rate_unit: "hora",
        checked_at: new Date().toISOString(),
        items: [{ start_at: startAt, end_at: endAt }],
      } satisfies components["schemas"]["PublicRentalAvailability"];
      result = { status: 200, body: { data: availability } };
    } else if (pathname === "/api/backend/rental-quotes" && method === "POST") {
      browserAuthHeaders.push(request.headers()["authorization"]);
      const body = request.postDataJSON() as {
        space_id: string;
        start_at: string;
        end_at: string;
      };
      quoteRequests.push(body);
      const quote = {
        id: quoteId,
        space_id: spaceId,
        space_title: "Espacio sintético",
        rate_unit: "hora",
        unit_price_clp: 12000,
        currency: "CLP",
        units: 1,
        subtotal_clp: 12000,
        start_at: body.start_at,
        end_at: body.end_at,
        time_zone: "America/Santiago",
        created_at: new Date().toISOString(),
        expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
      } satisfies components["schemas"]["RentalQuote"];
      result = { status: 201, body: { data: quote } };
    } else if (
      pathname === `/api/backend/rental-quotes/${quoteId}/contract-preview` &&
      method === "POST"
    ) {
      browserAuthHeaders.push(request.headers()["authorization"]);
      previewRequests.push(pathname);
      const preview = {
        state: "preview",
        quote_id: quoteId,
        space_id: spaceId,
        space_title: "Espacio sintético",
        rate_unit: "hora",
        unit_price_clp: 12000,
        units: 1,
        subtotal_clp: 12000,
        currency: "CLP",
        start_at: startAt,
        end_at: endAt,
        time_zone: "America/Santiago",
        parties: [
          {
            role: "arrendador",
            legal_name: "Propietario Sintético",
            rut: "12.345.678-9",
          },
          {
            role: "arrendatario",
            legal_name: "Arrendataria Sintética",
            rut: "98.765.432-1",
          },
        ],
        generated_at: new Date().toISOString(),
      } satisfies components["schemas"]["RentalContractPreview"];
      result = { status: 200, body: { data: preview } };
    }

    const adapted = bff.adapt(request, result);
    if (!adapted)
      throw new Error(`No controlled response for ${method} ${pathname}`);
    await route.fulfill({
      status: adapted.status,
      headers: {
        "cache-control": "no-store",
        ...(adapted.contentType
          ? { "content-type": adapted.contentType }
          : adapted.status !== 204
            ? { "content-type": "application/json; charset=utf-8" }
            : {}),
        ...(adapted.headers ?? {}),
      },
      ...(adapted.status === 204
        ? {}
        : { body: JSON.stringify(adapted.body ?? {}) }),
    });
  });

  await page.goto(`/espacios/${spaceId}`);
  await expect(
    page.getByRole("heading", { name: "Espacio sintético", level: 1 }),
  ).toBeVisible();

  const availability = page
    .getByRole("heading", { name: "Disponibilidad para arriendo" })
    .locator("xpath=ancestor::section[1]");
  const dayNumber = Number(tomorrow.slice(-2));
  if (tomorrow.slice(0, 7) !== today.slice(0, 7)) {
    await availability.getByRole("button", { name: "Mes siguiente" }).click();
  }
  await availability
    .getByRole("button", {
      name: new RegExp(
        `^${dayNumber} (?:Sin información|Disponible|Parcialmente disponible)`,
      ),
    })
    .click();
  await expect(
    availability.getByRole("button", { name: /12:00.*01:00/ }),
  ).toBeVisible();
  await availability.getByRole("button", { name: /12:00.*01:00/ }).click();
  await availability
    .getByRole("link", { name: "Iniciar sesión o registrarse" })
    .click();

  await expect(page).toHaveURL(/\/acceso\?return_to=/);
  const accessUrl = new URL(page.url());
  const returnUrl = new URL(
    accessUrl.searchParams.get("return_to") ?? "",
    accessUrl.origin,
  );
  expect(returnUrl.pathname).toBe(`/espacios/${spaceId}`);
  expect(returnUrl.searchParams.get("rental_date")).toBe(tomorrow);
  expect(returnUrl.searchParams.get("rental_duration")).toBe("1");
  expect(returnUrl.searchParams.get("rental_start_at")).toBe(startAt);
  expect(returnUrl.searchParams.get("rental_end_at")).toBe(endAt);
  await page.getByLabel("Correo electrónico").fill("renter@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === `/espacios/${spaceId}`);

  const authenticatedAvailability = page
    .getByRole("heading", { name: "Disponibilidad para arriendo" })
    .locator("xpath=ancestor::section[1]");
  await authenticatedAvailability
    .getByRole("button", { name: "Solicitar cotización" })
    .waitFor({ state: "visible" });
  await authenticatedAvailability
    .getByRole("button", { name: "Solicitar cotización" })
    .click();
  await expect(
    authenticatedAvailability.getByText(/Cotización creada/i),
  ).toBeVisible();
  await authenticatedAvailability
    .getByRole("button", { name: "Vista previa del contrato" })
    .click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("Propietario Sintético")).toBeVisible();
  await expect(dialog.getByText("Arrendataria Sintética")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("commercial-rental-contract-preview.png"),
    fullPage: false,
  });
  const legalWarning = dialog.getByText(/Este documento es únicamente/);
  await legalWarning.scrollIntoViewIfNeeded();
  await expect(legalWarning).toBeInViewport();
  await page.setViewportSize({ width: 390, height: 844 });
  const closeButton = dialog.getByRole("button", {
    name: "Cerrar",
    exact: true,
  });
  await closeButton.scrollIntoViewIfNeeded();
  await expect(closeButton).toBeInViewport();
  const horizontalOverflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth -
      document.documentElement.clientWidth,
  );
  expect(horizontalOverflow).toBe(0);

  const logoutPage = await context.newPage();
  await logoutPage.goto("/mi-cuenta");
  await expect(
    logoutPage.getByRole("button", { name: "Cerrar sesión" }),
  ).toBeVisible();
  const logoutResponse = logoutPage.waitForResponse(
    (response) =>
      response.url().includes("/api/backend/auth/logout") &&
      response.request().method() === "POST",
  );
  await logoutPage.getByRole("button", { name: "Cerrar sesión" }).click();
  expect((await logoutResponse).status()).toBe(204);
  await expect(logoutPage).toHaveURL(/\/acceso$/);
  await expect(dialog).toBeHidden();
  await expect(
    authenticatedAvailability.getByText(/Cotización creada/i),
  ).toBeHidden();

  activeAccountId = secondAccountId;
  await logoutPage
    .getByLabel("Correo electrónico")
    .fill("second-renter@example.invalid");
  await logoutPage
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticPass123!");
  const accountSwitchLogin = logoutPage.waitForResponse(
    (response) =>
      response.url().includes("/api/backend/auth/login") &&
      response.request().method() === "POST",
  );
  await logoutPage.getByRole("button", { name: "Iniciar sesión" }).click();
  expect((await accountSwitchLogin).status()).toBe(200);
  await expect(page.getByText(secondAccountId, { exact: true })).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(
    authenticatedAvailability.getByText(/Cotización creada/i),
  ).toBeHidden();

  expect(quoteRequests).toHaveLength(1);
  expect(quoteRequests[0]).toEqual({
    space_id: spaceId,
    start_at: startAt,
    end_at: endAt,
  });
  expect(previewRequests).toEqual([
    `/api/backend/rental-quotes/${quoteId}/contract-preview`,
  ]);
  expect(browserAuthHeaders).toEqual([undefined, undefined]);
});
