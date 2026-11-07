import {
  expect,
  test,
  type Page,
  type Route,
  type Request,
} from "@playwright/test";

const spaceId = "00000000-0000-4000-8000-000000000081";
const gallery = `/api/backend/spaces/${spaceId}/gallery`;
const file = (index: number) => ({
  name: `synthetic-${index}.png`,
  mimeType: "image/png",
  buffer: Buffer.from(`synthetic-bytes-${index}`),
});
test.use({ trace: "off", screenshot: "off", video: "off" });

type Photo = {
  id: string;
  space_id: string;
  state: "ready";
  position: number;
  cover: boolean;
  mime_type: string;
  size_bytes: number;
};
async function install(
  page: Page,
  upload: (route: Route, request: Request, add: () => Photo) => Promise<void>,
) {
  const photos: Photo[] = [];
  let authenticated = false;
  const session = {
    account_id: "00000000-0000-4000-8000-000000000080",
    roles: ["anfitrion"],
    capabilities: [],
    restricted_mode: false,
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
  };
  const detail = {
    id: spaceId,
    category_code: "oficina",
    category_name: "Oficina",
    title: "Galería sintética de cola",
    description:
      "Descripción sintética de ensayo para verificar la cola sin servicios externos ni información de personas reales.",
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
    cover_photo_id: null as string | null,
  };
  const reply = (route: Route, body: unknown, status = 200) =>
    route.fulfill({
      status,
      headers: {
        "content-type": "application/json",
        etag: '"1"',
        "cache-control": "no-store",
      },
      body: status === 204 ? "" : JSON.stringify(body),
    });
  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    if (pathname === "/api/backend/auth/login") {
      authenticated = true;
      await reply(route, session);
      return;
    }
    if (pathname === "/api/backend/auth/session") {
      await reply(
        route,
        authenticated ? session : { error: { code: "unauthenticated" } },
        authenticated ? 200 : 401,
      );
      return;
    }
    if (pathname === "/api/backend/auth/logout") {
      authenticated = false;
      await reply(route, {}, 204);
      return;
    }
    expect(request.headers().authorization).toBeUndefined();
    if (pathname === `/api/backend/spaces/${spaceId}`) {
      await reply(route, detail);
      return;
    }
    if (
      pathname === `/api/backend/spaces/${spaceId}/cover` &&
      request.method() === "PUT"
    ) {
      expect(request.headers()["if-match"]).toBe('"1"');
      const { photo_id: id } = request.postDataJSON() as { photo_id: string };
      expect(photos.some((photo) => photo.id === id)).toBe(true);
      detail.cover_photo_id = id;
      photos.forEach((photo) => {
        photo.cover = photo.id === id;
      });
      await reply(route, {
        space_id: spaceId,
        photo_id: id,
        selected: true,
        updated_at: "2026-11-05T12:00:00Z",
      });
      return;
    }
    if (pathname === gallery && request.method() === "GET") {
      await reply(route, { items: photos, max_photos: 30 });
      return;
    }
    if (pathname === gallery && request.method() === "POST") {
      await upload(route, request, () => {
        const photo: Photo = {
          id: `00000000-0000-4000-8000-${String(photos.length + 1).padStart(12, "0")}`,
          space_id: spaceId,
          state: "ready",
          position: photos.length + 1,
          cover: false,
          mime_type: "image/webp",
          size_bytes: 42,
        };
        photos.push(photo);
        return photo;
      });
      return;
    }
    if (pathname.endsWith("/variants/thumbnail")) {
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
    if (pathname.includes("/attributes/")) {
      await reply(route, {
        category_code: "oficina",
        schema_version: 1,
        attributes: [],
      });
      return;
    }
    await reply(route, { error: { code: "not_found" } }, 404);
  });
  await page.goto(
    `/acceso?return_to=${encodeURIComponent(`/anfitrion/espacios/${spaceId}/editar`)}`,
  );
  await page
    .getByLabel("Correo electrónico")
    .fill("queue-fixture@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticOnly123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .locator("h1"),
  ).toHaveText(detail.title);
  await page.getByRole("tab", { name: /Fotografías/ }).click();
  return { photos };
}

const complete = (route: Route, photo: Photo) =>
  route.fulfill({
    status: 201,
    contentType: "application/json",
    headers: { etag: '"1"' },
    body: JSON.stringify({ photo }),
  });

test("@controlled queue renders two active uploads, cancels queued work without POST and counts only confirmed success", async ({
  page,
}) => {
  const releases: Array<() => Promise<void>> = [];
  let active = 0;
  let maximum = 0;
  await install(page, async (route, _request, add) => {
    active++;
    maximum = Math.max(maximum, active);
    await new Promise<void>((resolve) =>
      releases.push(async () => {
        active--;
        await complete(route, add());
        resolve();
      }),
    );
  });
  await page
    .locator("#gallery-photo-upload")
    .setInputFiles([file(1), file(2), file(3), file(4)]);
  await expect.poll(() => releases.length).toBe(2);
  await expect(page.locator('[data-upload-state="uploading"]')).toHaveCount(2);
  await expect(page.locator('[data-upload-state="queued"]')).toHaveCount(2);
  await expect(
    page.getByRole("button", { name: "Publicar", exact: true }),
  ).toBeDisabled();
  await page
    .getByTestId("gallery-upload-item")
    .nth(3)
    .getByRole("button", { name: "Cancelar en cola" })
    .click();
  await releases[0]!();
  await expect.poll(() => releases.length).toBe(3);
  await releases[1]!();
  await releases[2]!();
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(3);
  await expect(page.locator('[data-upload-state="cancelled"]')).toHaveCount(1);
  await expect(
    page.getByText("3 fotografías agregadas a la galería.", { exact: true }),
  ).toBeVisible();
  expect(releases.length).toBe(3);
  expect(maximum).toBe(2);
  await expect(
    page.getByRole("button", { name: "Publicar", exact: true }),
  ).toBeEnabled();
  await page
    .locator("#gallery-photo-upload")
    .setInputFiles(Array.from({ length: 28 }, (_, i) => file(i + 10)));
  await expect(
    page
      .getByRole("region", { name: "Cola de carga de fotografías" })
      .getByRole("alert"),
  ).toContainText("hasta 30 fotografías");
  expect(releases.length).toBe(3);
});

test("@controlled accepted upload stays unresolved, looks up the same key and retries only explicitly with the original bytes", async ({
  page,
}) => {
  const posts: Array<{ key: string; hasOriginalBytes: boolean }> = [];
  const lookups: string[] = [];
  let remoteState = "pending";
  const input = file(1);
  await install(page, async (route, request, add) => {
    posts.push({
      key: request.headers()["idempotency-key"]!,
      hasOriginalBytes:
        request.postDataBuffer()?.includes(input.buffer) === true,
    });
    if (posts.length === 1)
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({ photo: { ...add(), state: "pending" } }),
      });
    else await complete(route, add());
  });
  await page.route(`**${gallery}/upload-status`, async (route) => {
    lookups.push(route.request().headers()["idempotency-key"]!);
    await route.fulfill({
      status: remoteState === "not_found" ? 404 : 200,
      contentType: "application/json",
      body: JSON.stringify(
        remoteState === "not_found"
          ? { error: { code: "not_found" } }
          : {
              state: remoteState,
              photo_id: "00000000-0000-4000-8000-000000000001",
            },
      ),
    });
  });
  await page.locator("#gallery-photo-upload").setInputFiles(input);
  await expect(page.locator('[data-upload-state="pending"]')).toHaveCount(1);
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(0);
  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  expect(posts.length).toBe(1);
  expect(lookups[0]).toBe(posts[0]!.key);
  remoteState = "not_found";
  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  await expect(
    page.getByRole("button", { name: "Reintentar carga con el mismo archivo" }),
  ).toBeVisible();
  expect(posts.length).toBe(1);
  await page
    .getByRole("button", { name: "Reintentar carga con el mismo archivo" })
    .click();
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  expect(posts.length).toBe(2);
  expect(posts[1]!.key).toBe(posts[0]!.key);
  expect(posts.every((post) => post.hasOriginalBytes)).toBe(true);
});
