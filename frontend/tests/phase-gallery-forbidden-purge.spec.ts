import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Route } from "@playwright/test";
import { isAllowedLocalResource } from "./helpers/local-resource-origin";

test.use({ trace: "off", screenshot: "off", video: "off" });

const requestsOutsideLoopback = new WeakMap<Page, boolean>();
test.beforeEach(async ({ page }) => {
  requestsOutsideLoopback.set(page, false);
  const baseUrl = test.info().project.use.baseURL;
  if (typeof baseUrl !== "string")
    throw new Error("Playwright baseURL is required");
  page.on("request", (request) => {
    if (!isAllowedLocalResource(request.url(), baseUrl))
      requestsOutsideLoopback.set(page, true);
  });
});
test.afterEach(async ({ page }) => {
  expect(requestsOutsideLoopback.get(page)).toBe(false);
});

const spaceId = "00000000-0000-4000-8000-000000000131";
const photoIds = [
  "00000000-0000-4000-8000-000000000132",
  "00000000-0000-4000-8000-000000000133",
];
const privateTitle = "Espacio de galería protegida sintética";

test("@controlled a forbidden private thumbnail purges the visible gallery", async ({
  page,
}) => {
  let authenticated = false;
  const mutationRequests: string[] = [];
  const forbiddenThumbnailRequests: string[] = [];
  let releaseForbiddenThumbnail: (() => void) | undefined;
  let resolveForbiddenThumbnailRequest: (() => void) | undefined;
  const forbiddenThumbnailGate = new Promise<void>((resolve) => {
    releaseForbiddenThumbnail = resolve;
  });
  const forbiddenThumbnailRequested = new Promise<void>((resolve) => {
    resolveForbiddenThumbnailRequest = resolve;
  });
  const expiresAt = new Date(Date.now() + 3_600_000).toISOString();
  const session = {
    account_id: "770e8400-e29b-41d4-a716-446655440131",
    roles: ["arrendador"],
    capabilities: [],
    restricted_mode: false,
    expires_at: expiresAt,
  };
  const detail = {
    id: spaceId,
    category_code: "oficina",
    category_name: "Oficinas",
    title: privateTitle,
    description:
      "Descripción sintética suficientemente larga para una ficha privada de prueba.",
    area_m2: 40,
    capacity: 2,
    usage_rules: "Reglas sintéticas",
    rate_unit: "hora",
    base_price_clp: 6000,
    address: "Dirección sintética",
    state: "borrador",
    time_zone: "America/Santiago",
    region_code: "13",
    comuna_code: "13101",
    attribute_schema_version: 1,
    attributes: {},
    cover_photo_id: photoIds[0],
  };
  const photos = photoIds.map((id, index) => ({
    id,
    space_id: spaceId,
    alt_text: "Foto sintética protegida",
    visible: true,
    created_at: "2026-11-11T12:00:00Z",
    state: "ready",
    position: index + 1,
    cover: index === 0,
    mime_type: "image/webp",
    size_bytes: 42,
    variants: ["thumbnail", "display"].map((name) => ({
      name,
      url: `/api/v1/spaces/${spaceId}/gallery/${id}/variants/${name}`,
      mime_type: "image/webp",
      size_bytes: 42,
      sha256: "a".repeat(64),
      width: 10,
      height: 10,
    })),
  }));

  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      authenticated = true;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(session),
      });
      return;
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      await route.fulfill({
        status: authenticated ? 200 : 401,
        contentType: "application/json",
        body: JSON.stringify(
          authenticated ? session : { error: { code: "unauthenticated" } },
        ),
      });
      return;
    }

    expect(request.headers().authorization).toBeUndefined();
    if (
      method === "POST" ||
      method === "PUT" ||
      method === "PATCH" ||
      method === "DELETE"
    ) {
      mutationRequests.push(`${method} ${pathname}`);
      await route.fulfill({
        status: 403,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "forbidden" } }),
      });
      return;
    }
    if (pathname === `/api/backend/spaces/${spaceId}` && method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        headers: { etag: '"gallery-forbidden-v1"' },
        body: JSON.stringify(detail),
      });
      return;
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/gallery` &&
      method === "GET"
    ) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        headers: { etag: '"gallery-forbidden-v1"' },
        body: JSON.stringify({ items: photos, max_photos: 30 }),
      });
      return;
    }
    if (
      pathname ===
        `/api/backend/spaces/${spaceId}/gallery/${photoIds[1]}/variants/thumbnail` &&
      method === "GET"
    ) {
      forbiddenThumbnailRequests.push(`${method} ${pathname}`);
      resolveForbiddenThumbnailRequest?.();
      await forbiddenThumbnailGate;
      await route.fulfill({
        status: 403,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "forbidden" } }),
      });
      return;
    }
    if (
      pathname ===
      `/api/backend/spaces/${spaceId}/gallery/${photoIds[0]}/variants/thumbnail`
    ) {
      await route.fulfill({
        status: 200,
        contentType: "image/webp",
        body: Buffer.from(
          "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
          "base64",
        ),
      });
      return;
    }
    if (pathname === "/api/backend/spaces/categories/oficina/attributes/1") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          category_code: "oficina",
          schema_version: 1,
          attributes: [],
        }),
      });
      return;
    }
    if (pathname === `/api/backend/spaces/${spaceId}/tariffs`) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ items: [] }),
      });
      return;
    }
    await route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "not_found" } }),
    });
  });

  await page.addInitScript(() => {
    const revokedUrls: string[] = [];
    Object.defineProperty(window, "__privateBlobRevocations", {
      value: revokedUrls,
    });
    const revoke = URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL = (url) => {
      revokedUrls.push(url);
      revoke(url);
    };
  });

  await page.goto(
    `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${spaceId}/editar`)}`,
  );
  await page.getByLabel("Correo electrónico").fill("gallery@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticOnly123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL(`**/anfitrion/espacios/${spaceId}/editar`);
  await page.getByRole("tab", { name: /Fotografías/ }).click();

  await expect(page.getByTestId("gallery-photo")).toHaveCount(2);
  for (const viewport of [
    { width: 1280, height: 800 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const accessibility = await new AxeBuilder({ page }).analyze();
    expect(
      accessibility.violations.map(({ id, impact }) => ({ id, impact })),
      `gallery accessibility at ${viewport.width}x${viewport.height}`,
    ).toEqual([]);
  }
  await page.setViewportSize({ width: 1280, height: 800 });
  const firstPhotoImage = page.locator(`[data-photo-id="${photoIds[0]}"] img`);
  await expect(firstPhotoImage).toHaveAttribute("src", /^blob:/);
  const firstPhotoBlobUrl = await firstPhotoImage.getAttribute("src");
  expect(firstPhotoBlobUrl).toMatch(/^blob:/);
  await forbiddenThumbnailRequested;
  await expect(page.getByRole("heading", { name: privateTitle })).toBeVisible();

  const forbiddenResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname ===
        `/api/backend/spaces/${spaceId}/gallery/${photoIds[1]}/variants/thumbnail` &&
      response.status() === 403,
  );
  releaseForbiddenThumbnail?.();
  expect((await forbiddenResponse).status()).toBe(403);

  await expect(
    page.getByRole("heading", { name: "Espacio no encontrado" }),
  ).toBeVisible();
  await expect(page.getByTestId("gallery-photo")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: privateTitle })).toHaveCount(
    0,
  );
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          Object.getOwnPropertyDescriptor(window, "__privateBlobRevocations")
            ?.value as string[],
      ),
    )
    .toContain(firstPhotoBlobUrl);
  expect(forbiddenThumbnailRequests).toEqual([
    `GET /api/backend/spaces/${spaceId}/gallery/${photoIds[1]}/variants/thumbnail`,
  ]);
  expect(mutationRequests).toEqual([]);
});
