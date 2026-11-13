import { expect, test } from "@playwright/test";

const spaceId = "00000000-0000-4000-8000-000000000091";
const photoIds = [
  "00000000-0000-4000-8000-000000000092",
  "00000000-0000-4000-8000-000000000093",
];
test.use({ trace: "off", screenshot: "off", video: "off" });

test("unconfirmed gallery and stale delete block mutations until authoritative refresh", async ({
  page,
}) => {
  let authenticated = false;
  let coverRequests = 0;
  let deleteRequests = 0;
  const deleteIfMatches: string[] = [];
  let currentEtag = '"1"';
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
      alt_text: "Foto sintética de incertidumbre",
      visible: true,
      created_at: "2026-11-11T12:00:00Z",
      state: "ready",
      position: index + 1,
      cover: id === cover,
      mime_type: "image/webp",
      size_bytes: 42,
      variants: [
        {
          name: "thumbnail",
          url: `/api/v1/spaces/${spaceId}/gallery/${id}/variants/thumbnail`,
          mime_type: "image/webp",
          size_bytes: 42,
          sha256: "a".repeat(64),
          width: 10,
          height: 10,
        },
      ],
    }));
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
  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const reply = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        headers: {
          etag: currentEtag,
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
    if (pathname.endsWith("/auth/logout") && request.method() === "POST") {
      authenticated = false;
      return route.fulfill({ status: 204, body: "" });
    }
    expect(request.headers().authorization).toBeUndefined();
    if (pathname === `/api/backend/spaces/${spaceId}/cover`) {
      coverRequests++;
      expect(request.headers()["if-match"]).toBe('"1"');
      currentEtag = '"2"';
      cover = photoIds[1];
      failReadback = true;
      return route.abort("failed");
    }
    if (
      pathname.startsWith(`/api/backend/spaces/${spaceId}/gallery/`) &&
      request.method() === "DELETE"
    ) {
      deleteRequests++;
      const ifMatch = request.headers()["if-match"] ?? "";
      deleteIfMatches.push(ifMatch);
      if (deleteRequests === 1) currentEtag = '"3"';
      if (ifMatch !== currentEtag)
        return reply({ error: { code: "precondition_failed" } }, 412);
      currentEtag = '"4"';
      return route.fulfill({ status: 204, headers: { etag: currentEtag } });
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

  const firstPhoto = page.getByTestId("gallery-photo").first();
  const firstPhotoId = await firstPhoto.getAttribute("data-photo-id");
  await page.once("dialog", (dialog) => dialog.accept());
  await firstPhoto.getByRole("button", { name: "Eliminar foto" }).click();
  await expect(page.getByTestId("gallery-photo")).toHaveCount(2);
  await expect(
    page.locator(
      `[data-testid="gallery-photo"][data-photo-id="${firstPhotoId}"]`,
    ),
  ).toBeVisible();
  expect(deleteRequests).toBe(1);
  await expect(page.getByText(/actualiza la galería/i)).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Eliminar foto", exact: true }).first(),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Marcar portada", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByLabel("Subir fotografías", { exact: true }),
  ).toBeDisabled();
  for (const label of [
    "Mover foto antes",
    "Mover foto después",
    "Reemplazar fotografía",
    "Eliminar foto",
  ]) {
    const controls =
      label === "Reemplazar fotografía"
        ? page.getByLabel(label, { exact: true })
        : page.getByRole("button", { name: label, exact: true });
    expect(await controls.count()).toBe(2);
    for (const control of await controls.all())
      await expect(control).toBeDisabled();
  }
  await expect(
    page
      .getByRole("button", { name: "Actualizar galería", exact: true })
      .first(),
  ).toBeEnabled();

  await page
    .getByRole("button", { name: "Actualizar galería", exact: true })
    .first()
    .click();
  await expect(page.getByText(/actualiza la galería/i)).toHaveCount(0);
  await expect(page.getByTestId("gallery-photo")).toHaveCount(2);
  await expect(
    page.getByRole("button", { name: "Eliminar foto", exact: true }).first(),
  ).toBeEnabled();
  await expect(
    page.getByLabel("Subir fotografías", { exact: true }),
  ).toBeEnabled();
  const moveBefore = page.getByRole("button", {
    name: "Mover foto antes",
    exact: true,
  });
  await expect(moveBefore.nth(0)).toBeDisabled();
  await expect(moveBefore.nth(1)).toBeEnabled();
  const moveAfter = page.getByRole("button", {
    name: "Mover foto después",
    exact: true,
  });
  await expect(moveAfter.nth(0)).toBeEnabled();
  await expect(moveAfter.nth(1)).toBeDisabled();
  for (const control of await page
    .getByLabel("Reemplazar fotografía", { exact: true })
    .all())
    await expect(control).toBeEnabled();
  for (const control of await page
    .getByRole("button", { name: "Eliminar foto", exact: true })
    .all())
    await expect(control).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Marcar portada", exact: true }),
  ).toBeEnabled();
  expect(deleteRequests).toBe(1);
  expect(deleteIfMatches).toEqual(['"2"']);

  const refreshedPhoto = page.getByTestId("gallery-photo").first();
  await page.once("dialog", (dialog) => dialog.accept());
  await refreshedPhoto.getByRole("button", { name: "Eliminar foto" }).click();
  await expect(page.getByTestId("gallery-photo")).toHaveCount(1);
  await expect(
    page.getByText("Fotografía eliminada.", { exact: true }),
  ).toBeVisible();
  expect(deleteRequests).toBe(2);
  expect(deleteIfMatches).toEqual(['"2"', '"3"']);

  const privateBlobUrls = await page
    .getByTestId("gallery-photo")
    .locator("img")
    .evaluateAll((images) =>
      images
        .map((image) => (image as HTMLImageElement).src)
        .filter((src) => src.startsWith("blob:")),
    );
  expect(privateBlobUrls).toHaveLength(1);
  await page
    .getByRole("button", { name: "Cerrar sesión", exact: true })
    .click();
  await page.waitForURL((url) => url.pathname === "/acceso");
  await expect(page.getByTestId("gallery-photo")).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Galería de incertidumbre sintética" }),
  ).toHaveCount(0);
  const revokedUrls = () =>
    page.evaluate(
      () =>
        Object.getOwnPropertyDescriptor(window, "__privateBlobRevocations")
          ?.value as string[],
    );
  await expect
    .poll(revokedUrls)
    .toEqual(expect.arrayContaining(privateBlobUrls));
  expect(deleteRequests).toBe(2);
});
