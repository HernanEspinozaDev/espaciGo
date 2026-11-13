import { expect, test, type Page } from "@playwright/test";
import { isAllowedLocalResource } from "./helpers/local-resource-origin";

const spaceId = "00000000-0000-4000-8000-000000000141";
const photoId = "00000000-0000-4000-8000-000000000142";
const privateTitle = "Espacio sintético con foto de contrato inválido";
const malformedReadyPhoto = {
  id: photoId,
  space_id: spaceId,
  alt_text: "Foto sintética",
  visible: true,
  cover: false,
  position: 1,
  state: "ready",
  created_at: "2026-11-11T12:00:00Z",
  // SpaceMediaPhoto requires variants; without fixture_code this is not synthetic.
};
const readyPhotoMissingCreatedAt = {
  id: photoId,
  space_id: spaceId,
  alt_text: "Foto sintética",
  visible: true,
  cover: false,
  position: 1,
  state: "ready",
  variants: [
    {
      name: "thumbnail",
      url: `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/thumbnail`,
      mime_type: "image/webp",
      size_bytes: 42,
      sha256: "a".repeat(64),
      width: 10,
      height: 10,
    },
  ],
};
const readyPhotoInvalidCreatedAt = {
  ...readyPhotoMissingCreatedAt,
  created_at: "2026-02-30T12:00:00Z",
};
const invalidGalleryPhotos = [
  {
    name: "a ready photo without contract variants is not treated as synthetic",
    photo: malformedReadyPhoto,
  },
  {
    name: "a ready photo missing required created_at is rejected, not assigned a fabricated date",
    photo: readyPhotoMissingCreatedAt,
  },
  {
    name: "a ready photo with invalid created_at is rejected as a protocol error",
    photo: readyPhotoInvalidCreatedAt,
  },
];

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

for (const { name, photo } of invalidGalleryPhotos) {
  test(`@controlled ${name}`, async ({ page }) => {
    let authenticated = false;
    let galleryReads = 0;
    let variantReads = 0;
    const session = {
      account_id: "00000000-0000-4000-8000-000000000140",
      roles: ["arrendador"],
      capabilities: [],
      restricted_mode: false,
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
    };
    const detail = {
      id: spaceId,
      category_code: "oficina",
      category_name: "Oficina",
      title: privateTitle,
      description:
        "Descripción sintética suficientemente extensa para probar el contrato de lectura de la galería.",
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
      cover_photo_id: null,
    };
    await page.route("**/api/backend/**", async (route) => {
      const request = route.request();
      const pathname = new URL(request.url()).pathname;
      const method = request.method();
      const reply = (body: unknown, status = 200) =>
        route.fulfill({
          status,
          contentType: "application/json",
          headers: {
            etag: '"gallery-contract-v1"',
            "cache-control": "no-store",
          },
          body: JSON.stringify(body),
        });

      if (pathname === "/api/backend/auth/login" && method === "POST") {
        authenticated = true;
        return reply(session);
      }
      if (pathname === "/api/backend/auth/session" && method === "GET")
        return reply(
          authenticated ? session : { error: { code: "unauthenticated" } },
          authenticated ? 200 : 401,
        );

      expect(request.headers().authorization).toBeUndefined();
      if (pathname === `/api/backend/spaces/${spaceId}` && method === "GET")
        return reply(detail);
      if (
        pathname === `/api/backend/spaces/${spaceId}/gallery` &&
        method === "GET"
      ) {
        galleryReads++;
        return reply({ items: [photo], max_photos: 30 });
      }
      if (
        pathname ===
          `/api/backend/spaces/${spaceId}/gallery/${photoId}/variants/thumbnail` &&
        method === "GET"
      ) {
        variantReads++;
        return route.fulfill({
          status: 200,
          contentType: "image/webp",
          body: Buffer.from(
            "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
            "base64",
          ),
        });
      }
      if (
        pathname === `/api/backend/spaces/categories/oficina/attributes/1` &&
        method === "GET"
      )
        return reply({
          category_code: "oficina",
          schema_version: 1,
          attributes: [],
        });
      if (
        pathname === `/api/backend/spaces/${spaceId}/tariffs` &&
        method === "GET"
      )
        return reply({ items: [] });
      return reply({ error: { code: "not_found" } }, 404);
    });

    await page.goto(
      `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${spaceId}/editar`)}`,
    );
    await page
      .getByLabel("Correo electrónico")
      .fill("gallery-contract@example.invalid");
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("SyntheticOnly123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    await page.waitForURL(`**/anfitrion/espacios/${spaceId}/editar`);
    await page.getByRole("tab", { name: /Fotografías/ }).click();

    await expect(
      page.getByRole("heading", { name: "Galería de imágenes" }),
    ).toBeVisible();
    await expect.poll(() => galleryReads).toBeGreaterThan(0);
    await expect(page.getByTestId("gallery-photo")).toHaveCount(0);
    await expect(page.getByText(/La galería no está confirmada/)).toBeVisible();
    expect(variantReads).toBe(0);
  });
}
