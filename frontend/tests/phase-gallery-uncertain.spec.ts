import { expect, test } from "@playwright/test";

const spaceId = "00000000-0000-4000-8000-000000000091";
const photoIds = [
  "00000000-0000-4000-8000-000000000092",
  "00000000-0000-4000-8000-000000000093",
];
test.use({ trace: "off", screenshot: "off", video: "off" });

test("unconfirmed cover blocks every mutation until an explicit successful authoritative refresh", async ({
  page,
}) => {
  let authenticated = false;
  let coverRequests = 0;
  let failReadback = false;
  let failInitialGallery = true;
  let cover = photoIds[0];
  const session = {
    account_id: "00000000-0000-4000-8000-000000000090",
    roles: ["anfitrion"],
    capabilities: [],
    restricted_mode: false,
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
  };
  const detail = () => ({
    id: spaceId,
    category_code: "oficina",
    category_name: "Oficina",
    title: "Galería de incertidumbre sintética",
    description:
      "Descripción sintética suficientemente extensa para probar la conservación de las fotografías ante una respuesta incierta.",
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
    cover_photo_id: cover,
  });
  const photos = () =>
    photoIds.map((id, index) => ({
      id,
      space_id: spaceId,
      state: "ready",
      position: index + 1,
      cover: id === cover,
      mime_type: "image/webp",
      size_bytes: 42,
      variants: [],
    }));
  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const reply = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        headers: {
          etag: coverRequests ? '"2"' : '"1"',
          "cache-control": "no-store",
        },
        body: JSON.stringify(body),
      });
    if (pathname.endsWith("/auth/login")) {
      authenticated = true;
      return reply(session);
    }
    if (pathname.endsWith("/auth/session"))
      return reply(
        authenticated ? session : { error: { code: "unauthenticated" } },
        authenticated ? 200 : 401,
      );
    expect(request.headers().authorization).toBeUndefined();
    if (pathname === `/api/backend/spaces/${spaceId}/cover`) {
      coverRequests++;
      expect(request.headers()["if-match"]).toBe('"1"');
      cover = photoIds[1];
      failReadback = true;
      return route.abort("failed");
    }
    if (pathname === `/api/backend/spaces/${spaceId}`)
      return reply(
        failReadback ? { error: { code: "unavailable" } } : detail(),
        failReadback ? 503 : 200,
      );
    if (pathname === `/api/backend/spaces/${spaceId}/gallery`)
      return reply(
        failInitialGallery
          ? { error: { code: "unavailable" } }
          : { items: photos(), max_photos: 30 },
        failInitialGallery ? 503 : 200,
      );
    if (pathname.endsWith("/variants/thumbnail"))
      return route.fulfill({
        contentType: "image/webp",
        body: Buffer.from(
          "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
          "base64",
        ),
      });
    if (pathname.includes("/attributes/"))
      return reply({
        category_code: "oficina",
        schema_version: 1,
        attributes: [],
      });
    if (pathname.endsWith("/tariffs")) return reply({ items: [] });
    return reply({ error: { code: "not_found" } }, 404);
  });
  await page.goto(
    `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${spaceId}/editar`)}`,
  );
  await page.getByLabel("Correo electrónico").fill("uncertain@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("Synthetic1!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname.endsWith(`/${spaceId}/editar`));
  await page.getByRole("tab", { name: /Fotografías/ }).click();
  await expect(page.getByText(/La galería no está confirmada/)).toBeVisible();
  await expect(
    page.getByLabel("Subir fotografías", { exact: true }),
  ).toBeDisabled();
  expect(coverRequests).toBe(0);
  failInitialGallery = false;
  await page
    .getByRole("button", { name: "Actualizar galería", exact: true })
    .first()
    .click();
  await expect(page.getByText(/La galería no está confirmada/)).toHaveCount(0);
  const markCover = page.getByRole("button", {
    name: "Marcar portada",
    exact: true,
  });
  await markCover.click();
  await expect(page.getByText(/La galería no está confirmada/)).toBeVisible();
  expect(coverRequests).toBe(1);
  await expect(markCover).toBeDisabled();
  await expect(
    page.getByLabel("Subir fotografías", { exact: true }),
  ).toBeDisabled();
  for (const label of [
    "Mover foto antes",
    "Mover foto después",
    "Eliminar foto",
  ]) {
    const buttons = page.getByRole("button", { name: label, exact: true });
    expect(await buttons.count()).toBeGreaterThan(0);
    for (const button of await buttons.all())
      await expect(button).toBeDisabled();
  }
  const replacements = page.getByLabel("Reemplazar fotografía", {
    exact: true,
  });
  expect(await replacements.count()).toBe(2);
  for (const button of await replacements.all())
    await expect(button).toBeDisabled();
  await page
    .getByRole("button", { name: "Actualizar galería", exact: true })
    .first()
    .click();
  await expect(page.getByText(/La galería no está confirmada/)).toBeVisible();
  await expect(markCover).toBeDisabled();
  expect(coverRequests).toBe(1);
  failReadback = false;
  await page
    .getByRole("button", { name: "Actualizar galería", exact: true })
    .first()
    .click();
  await expect(page.getByText(/La galería no está confirmada/)).toHaveCount(0);
  await expect(
    page.getByLabel("Subir fotografías", { exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Marcar portada", exact: true }),
  ).toBeEnabled();
  expect(coverRequests).toBe(1);
});
