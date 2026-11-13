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
  alt_text: string;
  visible: boolean;
  created_at: string;
  state: "ready";
  position: number;
  cover: boolean;
  mime_type: string;
  size_bytes: number;
  variants: Array<{
    name: "thumbnail" | "display";
    url: string;
    mime_type: "image/webp";
    size_bytes: number;
    sha256: string;
    width: number;
    height: number;
  }>;
};
async function install(
  page: Page,
  upload: (route: Route, request: Request, add: () => Photo) => Promise<void>,
  initialPhotos: readonly Photo[] = [],
) {
  const photos: Photo[] = [...initialPhotos];
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
        const photoId = `00000000-0000-4000-8000-${String(photos.length + 1).padStart(12, "0")}`;
        const photo: Photo = {
          id: photoId,
          space_id: spaceId,
          alt_text: "",
          visible: true,
          created_at: "2026-11-11T12:00:00Z",
          state: "ready",
          position: photos.length + 1,
          cover: false,
          mime_type: "image/webp",
          size_bytes: 42,
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

test("@controlled gallery dropzone queues a synthetic image through the upload path", async ({
  page,
}) => {
  const dropped = file(5);
  const posts: Array<{ key: string; includesOriginalBytes: boolean }> = [];
  await install(page, async (route, request, add) => {
    posts.push({
      key: request.headers()["idempotency-key"] ?? "",
      includesOriginalBytes:
        request.postDataBuffer()?.includes(dropped.buffer) === true,
    });
    await complete(route, add());
  });

  const dropzone = page.getByTestId("gallery-dropzone");
  await expect(dropzone).toBeVisible();
  const dispatchFileEvent = (type: "dragenter" | "dragover" | "drop") =>
    dropzone.evaluate(
      (element, payload) => {
        const dataTransfer = new DataTransfer();
        const bytes = Uint8Array.from(atob(payload.base64), (value) =>
          value.charCodeAt(0),
        );
        dataTransfer.items.add(
          new File([bytes], payload.name, { type: payload.mimeType }),
        );
        const event = new DragEvent(payload.type, {
          bubbles: true,
          cancelable: true,
          dataTransfer,
        });
        element.dispatchEvent(event);
        return event.defaultPrevented;
      },
      {
        type,
        name: dropped.name,
        mimeType: dropped.mimeType,
        base64: dropped.buffer.toString("base64"),
      },
    );

  expect(await dispatchFileEvent("dragenter")).toBe(false);
  await expect(dropzone).toHaveAttribute("data-drag-active", "true");
  expect(await dispatchFileEvent("dragover")).toBe(true);
  expect(await dispatchFileEvent("drop")).toBe(true);
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  await expect(dropzone).toHaveAttribute("data-drag-active", "false");
  expect(posts).toHaveLength(1);
  expect(posts[0]?.key).toMatch(/^[0-9a-f-]{36}$/i);
  expect(posts[0]?.includesOriginalBytes).toBe(true);
});

test("@controlled selecting an image confirms the same gallery photo, renders its WebP and reloads it", async ({
  page,
}) => {
  const selectedImage = {
    name: "espacio-seleccionado.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNQSlv1HwAEEgIyVuBIjQAAAABJRU5ErkJggg==",
      "base64",
    ),
  };
  const photoId = "00000000-0000-4000-8000-000000000001";
  let uploadCount = 0;
  let idempotencyKey = "";
  let originalBytesForwarded = false;
  const galleryURL = `/api/backend/spaces/${spaceId}/gallery`;
  const variantURL = `${galleryURL}/${photoId}/variants/thumbnail`;
  const isGalleryReadback = (response: import("@playwright/test").Response) =>
    response.request().method() === "GET" &&
    new URL(response.url()).pathname === galleryURL &&
    response.status() === 200;
  const isVariantReadback = (response: import("@playwright/test").Response) =>
    response.request().method() === "GET" &&
    new URL(response.url()).pathname === variantURL &&
    response.status() === 200;

  const { photos } = await install(page, async (route, request, add) => {
    uploadCount++;
    idempotencyKey = request.headers()["idempotency-key"] ?? "";
    originalBytesForwarded =
      request.postDataBuffer()?.includes(selectedImage.buffer) === true;
    await complete(route, add());
  });
  await expect(page.getByText("No hay imágenes en la galería")).toBeVisible();

  const uploadReadback = page.waitForResponse(isGalleryReadback);
  const variantReadback = page.waitForResponse(isVariantReadback);
  await page.locator("#gallery-photo-upload").setInputFiles(selectedImage);
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  const uploadReadbackBody = (await uploadReadback).json() as Promise<{
    items: Photo[];
  }>;
  const uploadedItems = (await uploadReadbackBody).items;
  expect(uploadedItems.map((photo) => photo.id)).toContain(photoId);
  expect(photos.map((photo) => photo.id)).toContain(photoId);
  const webpResponse = await variantReadback;
  expect(webpResponse.headers()["content-type"]).toContain("image/webp");
  expect(Number(webpResponse.headers()["content-length"])).toBeGreaterThan(0);
  const image = page
    .locator(`[data-testid="gallery-photo"][data-photo-id="${photoId}"]`)
    .locator("img");
  await expect
    .poll(() =>
      image.evaluate((element) => (element as HTMLImageElement).naturalWidth),
    )
    .toBeGreaterThan(0);
  expect(uploadCount).toBe(1);
  expect(idempotencyKey).toMatch(
    /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
  );
  expect(originalBytesForwarded).toBe(true);

  const card = page.locator(
    `[data-testid="gallery-photo"][data-photo-id="${photoId}"]`,
  );
  await expect(card).toBeVisible();
  await expect(image).toHaveAttribute("src", /^blob:/);
  await expect
    .poll(() =>
      image.evaluate((element) => (element as HTMLImageElement).naturalWidth),
    )
    .toBeGreaterThan(0);

  const reloadReadback = page.waitForResponse(isGalleryReadback);
  await page.reload();
  await page.getByRole("tab", { name: /Fotografías/ }).click();
  const reloadedItems = (
    (await reloadReadback).json() as Promise<{
      items: Photo[];
    }>
  ).then((body) => body.items);
  expect((await reloadedItems).map((photo) => photo.id)).toEqual([photoId]);
  await expect(card).toBeVisible();
  await expect
    .poll(() =>
      image.evaluate((element) => (element as HTMLImageElement).naturalWidth),
    )
    .toBeGreaterThan(0);
  expect(uploadCount).toBe(1);
});

test("@controlled a rejected image can be replaced without an uncertain status lookup", async ({
  page,
}) => {
  const rejectedImage = file(1);
  const replacementImage = {
    name: "imagen-valida.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNQSlv1HwAEEgIyVuBIjQAAAABJRU5ErkJggg==",
      "base64",
    ),
  };
  const statusURL = `${gallery}/upload-status`;
  const idempotencyKeys: string[] = [];
  let uploadCount = 0;
  let statusLookups = 0;
  page.on("request", (request) => {
    if (
      request.method() === "GET" &&
      new URL(request.url()).pathname === statusURL
    )
      statusLookups++;
  });

  const { photos } = await install(page, async (route, request, add) => {
    uploadCount++;
    idempotencyKeys.push(request.headers()["idempotency-key"] ?? "");
    if (uploadCount === 1) {
      await route.fulfill({
        status: 422,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "validation_error" } }),
      });
      return;
    }
    await complete(route, add());
  });

  const input = page.locator("#gallery-photo-upload");
  await input.setInputFiles(rejectedImage);
  const rejectedItem = page.locator('[data-upload-state="rejected"]');
  await expect(rejectedItem).toBeVisible();
  await expect(rejectedItem.getByRole("status")).toHaveText(
    "Imagen rechazada. Selecciona otro archivo.",
  );
  await expect(
    rejectedItem.getByRole("button", { name: "Quitar archivo rechazado" }),
  ).toBeVisible();
  await expect(input).toBeEnabled();
  expect(statusLookups).toBe(0);

  await rejectedItem
    .getByRole("button", { name: "Quitar archivo rechazado" })
    .click();
  await expect(page.getByTestId("gallery-upload-item")).toHaveCount(0);
  await input.setInputFiles(replacementImage);
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  expect(uploadCount).toBe(2);
  expect(new Set(idempotencyKeys).size).toBe(2);
  expect(idempotencyKeys.every((key) => key.length > 0)).toBe(true);
  expect(photos).toHaveLength(1);
  expect(statusLookups).toBe(0);
});

test("@controlled an oversized upload response is rejected without a status lookup", async ({
  page,
}) => {
  let uploadCount = 0;
  let statusLookups = 0;
  const statusURL = `${gallery}/upload-status`;
  page.on("request", (request) => {
    if (
      request.method() === "GET" &&
      new URL(request.url()).pathname === statusURL
    )
      statusLookups++;
  });
  await install(page, async (route) => {
    uploadCount++;
    await route.fulfill({
      status: 413,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "upload_too_large" } }),
    });
  });

  const input = page.locator("#gallery-photo-upload");
  await input.setInputFiles(file(1));
  const rejectedItem = page.locator('[data-upload-state="rejected"]');
  await expect(rejectedItem).toBeVisible();
  await expect(rejectedItem.getByRole("status")).toHaveText(
    "Imagen rechazada. Selecciona otro archivo.",
  );
  await expect(
    rejectedItem.getByRole("button", { name: "Quitar archivo rechazado" }),
  ).toBeVisible();
  await expect(input).toBeEnabled();
  expect(uploadCount).toBe(1);
  expect(statusLookups).toBe(0);
});

test("@controlled gallery dropzone ignores non-file drags and keeps validation feedback", async ({
  page,
}) => {
  const uploads: string[] = [];
  await install(page, async (route, _request, add) => {
    uploads.push("POST");
    await complete(route, add());
  });

  const dropzone = page.getByTestId("gallery-dropzone");
  const uploadRegion = page.getByRole("region", {
    name: "Cola de carga de fotografías",
  });
  await page.locator("#gallery-photo-upload").setInputFiles({
    name: "not-an-image.gif",
    mimeType: "image/gif",
    buffer: Buffer.from("synthetic-invalid-file"),
  });
  const validationAlert = uploadRegion.getByRole("alert");
  await expect(validationAlert).toHaveText(
    "Usa imágenes JPG, PNG o WebP de hasta 10 MiB.",
  );

  const dispatchTextEvent = (
    type: "dragenter" | "dragover" | "dragleave" | "drop",
  ) =>
    dropzone.evaluate((element, eventType) => {
      const dataTransfer = new DataTransfer();
      dataTransfer.setData("text/plain", "synthetic non-file payload");
      const event = new DragEvent(eventType, {
        bubbles: true,
        cancelable: true,
        dataTransfer,
      });
      element.dispatchEvent(event);
      return event.defaultPrevented;
    }, type);

  for (const type of ["dragenter", "dragover", "dragleave"] as const)
    expect(await dispatchTextEvent(type)).toBe(false);
  expect(await dispatchTextEvent("drop")).toBe(true);
  await expect(dropzone).toHaveAttribute("data-drag-active", "false");
  await expect(validationAlert).toHaveText(
    "Usa imágenes JPG, PNG o WebP de hasta 10 MiB.",
  );
  expect(uploads).toEqual([]);
});

test("@controlled gallery dropzone rejects unsupported file drops without posting", async ({
  page,
}) => {
  const uploads: string[] = [];
  await install(page, async (route, _request, add) => {
    uploads.push("POST");
    await complete(route, add());
  });

  const dropzone = page.getByTestId("gallery-dropzone");
  await expect(dropzone).toBeVisible();
  const defaultPrevented = await dropzone.evaluate((element) => {
    const dataTransfer = new DataTransfer();
    dataTransfer.items.add(
      new File([new Uint8Array([1, 2, 3])], "unsupported.gif", {
        type: "image/gif",
      }),
    );
    const event = new DragEvent("drop", {
      bubbles: true,
      cancelable: true,
      dataTransfer,
    });
    element.dispatchEvent(event);
    return event.defaultPrevented;
  });

  expect(defaultPrevented).toBe(true);
  await expect(
    page
      .getByRole("region", { name: "Cola de carga de fotografías" })
      .getByRole("alert"),
  ).toHaveText("Usa imágenes JPG, PNG o WebP de hasta 10 MiB.");
  await expect(page.locator("[data-upload-state]")).toHaveCount(0);
  expect(uploads).toEqual([]);
});

test("@controlled gallery accepts exactly 10 MiB and rejects larger or mismatched files", async ({
  page,
}) => {
  let uploads = 0;
  await install(page, async (route, _request, add) => {
    uploads++;
    await complete(route, add());
  });

  const input = page.locator("#gallery-photo-upload");
  const limit = 10 * 1024 * 1024;
  await input.setInputFiles({
    name: "exact-limit.png",
    mimeType: "image/png",
    buffer: Buffer.alloc(limit),
  });
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  expect(uploads).toBe(1);

  await input.setInputFiles({
    name: "over-limit.png",
    mimeType: "image/png",
    buffer: Buffer.alloc(limit + 1),
  });
  await expect(
    page
      .getByRole("region", { name: "Cola de carga de fotografías" })
      .getByRole("alert"),
  ).toHaveText("Usa imágenes JPG, PNG o WebP de hasta 10 MiB.");
  expect(uploads).toBe(1);

  await input.setInputFiles({
    name: "mismatched.jpg",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic-mime-mismatch"),
  });
  await expect(
    page
      .getByRole("region", { name: "Cola de carga de fotografías" })
      .getByRole("alert"),
  ).toHaveText("Usa imágenes JPG, PNG o WebP de hasta 10 MiB.");
  expect(uploads).toBe(1);
});

test("@controlled gallery dropzone opens the file selector by keyboard", async ({
  page,
}) => {
  const selected = file(6);
  const posts: Array<{ key: string; includesOriginalBytes: boolean }> = [];
  await install(page, async (route, request, add) => {
    posts.push({
      key: request.headers()["idempotency-key"] ?? "",
      includesOriginalBytes:
        request.postDataBuffer()?.includes(selected.buffer) === true,
    });
    await complete(route, add());
  });

  const dropzone = page.getByTestId("gallery-dropzone");
  await expect(dropzone).toHaveRole("button");
  await expect(dropzone).toBeEnabled();
  const chooserPromise = page.waitForEvent("filechooser");
  await dropzone.focus();
  await page.keyboard.press("Enter");
  const chooser = await chooserPromise;
  await chooser.setFiles(selected);

  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  expect(posts).toHaveLength(1);
  expect(posts[0]?.key).toMatch(/^[0-9a-f-]{36}$/i);
  expect(posts[0]?.includesOriginalBytes).toBe(true);
});

test("@controlled gallery dropzone retains a full-width drop target", async ({
  page,
}) => {
  await install(page, async (route, _request, add) => {
    await complete(route, add());
  });

  const dropzone = page.getByTestId("gallery-dropzone");
  const dimensions = await dropzone.evaluate((element) => ({
    target: element.getBoundingClientRect().width,
    container: element.parentElement?.getBoundingClientRect().width ?? 0,
  }));
  expect(dimensions.target).toBeGreaterThanOrEqual(dimensions.container - 1);
});

test("@controlled gallery dropzone preserves validation feedback while disabled", async ({
  page,
}) => {
  const initialPhotos: Photo[] = [1, 2].map((index) => {
    const id = `00000000-0000-4000-8000-${String(index).padStart(12, "0")}`;
    return {
      id,
      space_id: spaceId,
      alt_text: "",
      visible: true,
      created_at: "2026-11-11T12:00:00Z",
      state: "ready",
      position: index,
      cover: false,
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
    };
  });
  const uploadPosts: string[] = [];
  await install(
    page,
    async (route, _request, add) => {
      uploadPosts.push("POST");
      await complete(route, add());
    },
    initialPhotos,
  );

  const uploadRegion = page.getByRole("region", {
    name: "Cola de carga de fotografías",
  });
  await page.locator("#gallery-photo-upload").setInputFiles({
    name: "not-an-image.gif",
    mimeType: "image/gif",
    buffer: Buffer.from("synthetic-invalid-file"),
  });
  const validationAlert = uploadRegion.getByRole("alert");
  await expect(validationAlert).toHaveText(
    "Usa imágenes JPG, PNG o WebP de hasta 10 MiB.",
  );

  await page
    .getByRole("button", { name: "Mover foto después" })
    .first()
    .click();
  const dropzone = page.getByTestId("gallery-dropzone");
  await expect(dropzone).toBeDisabled();
  await dropzone.scrollIntoViewIfNeeded();
  const dropSurface = page.getByTestId("gallery-dropzone-surface");
  await expect(dropSurface).toBeVisible();
  await expect(dropzone).toHaveCSS("pointer-events", "none");
  const buttonBox = await dropzone.boundingBox();
  expect(buttonBox).not.toBeNull();
  const hitTarget = await page.evaluate(
    ({ x, y }) => {
      const element = document.elementFromPoint(x, y);
      return {
        tagName: element?.tagName,
        testId: element?.getAttribute("data-testid"),
        className: element?.getAttribute("class"),
      };
    },
    {
      x: buttonBox!.x + buttonBox!.width / 2,
      y: buttonBox!.y + buttonBox!.height / 2,
    },
  );
  expect(hitTarget).toMatchObject({
    tagName: "DIV",
    testId: "gallery-dropzone-surface",
  });

  const dropped = file(7);
  const dispatchFileEvent = (type: "dragover" | "drop") =>
    dropSurface.evaluate(
      (element, payload) => {
        const dataTransfer = new DataTransfer();
        const bytes = Uint8Array.from(atob(payload.base64), (value) =>
          value.charCodeAt(0),
        );
        dataTransfer.items.add(
          new File([bytes], payload.name, { type: payload.mimeType }),
        );
        const event = new DragEvent(payload.type, {
          bubbles: true,
          cancelable: true,
          dataTransfer,
        });
        element.dispatchEvent(event);
        return {
          defaultPrevented: event.defaultPrevented,
          dropEffect: dataTransfer.dropEffect,
        };
      },
      {
        type,
        name: dropped.name,
        mimeType: dropped.mimeType,
        base64: dropped.buffer.toString("base64"),
      },
    );
  expect(await dispatchFileEvent("dragover")).toEqual({
    defaultPrevented: true,
    dropEffect: "none",
  });
  expect((await dispatchFileEvent("drop")).defaultPrevented).toBe(true);
  await expect(validationAlert).toHaveText(
    "Usa imágenes JPG, PNG o WebP de hasta 10 MiB.",
  );
  expect(uploadPosts).toEqual([]);
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
    if (posts.length === 1) {
      const pendingPhoto = add();
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({
          photo: {
            id: pendingPhoto.id,
            space_id: pendingPhoto.space_id,
            state: "pending",
          },
          reused: false,
        }),
      });
    } else await complete(route, add());
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
  await expect(
    page.getByRole("button", { name: "Descartar carga fallida" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  expect(posts.length).toBe(1);
  expect(lookups[0]).toBe(posts[0]!.key);
  remoteState = "not_found";
  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  await expect(
    page.getByRole("button", { name: "Reintentar carga con el mismo archivo" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Descartar carga fallida" }),
  ).toHaveCount(0);
  expect(posts.length).toBe(1);
  await page
    .getByRole("button", { name: "Reintentar carga con el mismo archivo" })
    .click();
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  expect(posts.length).toBe(2);
  expect(posts[1]!.key).toBe(posts[0]!.key);
  expect(posts.every((post) => post.hasOriginalBytes)).toBe(true);
});

test("@controlled discard authority expires during recheck and returns only for a new terminal failure", async ({
  page,
}) => {
  const uploadKeys: string[] = [];
  const lookups: string[] = [];
  const deleteRequests: string[] = [];
  let signalRecheckStarted!: () => void;
  let releaseRecheck!: () => void;
  const recheckStarted = new Promise<void>((resolve) => {
    signalRecheckStarted = resolve;
  });
  const recheckGate = new Promise<void>((resolve) => {
    releaseRecheck = resolve;
  });
  const { photos } = await install(page, async (route, request) => {
    uploadKeys.push(request.headers()["idempotency-key"] ?? "");
    await route.fulfill({
      status: 422,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "invalid_file" } }),
    });
  });
  page.on("request", (request) => {
    if (request.method() === "DELETE" && request.url().includes("/gallery/"))
      deleteRequests.push(request.url());
  });
  await page.route(`**${gallery}/upload-status`, async (route) => {
    lookups.push(route.request().headers()["idempotency-key"] ?? "");
    if (lookups.length === 2) {
      signalRecheckStarted();
      await recheckGate;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          state: "pending",
          photo_id: "00000000-0000-4000-8000-000000000099",
        }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "failed",
        photo_id: "00000000-0000-4000-8000-000000000099",
      }),
    });
  });

  await page.locator("#gallery-photo-upload").setInputFiles(file(9));
  await expect(page.locator('[data-upload-state="failed"]')).toHaveCount(1);
  const discardButton = page.getByRole("button", {
    name: "Descartar carga fallida",
  });
  await expect(discardButton).toBeVisible();
  expect(uploadKeys).toHaveLength(1);
  expect(lookups).toEqual(uploadKeys);
  expect(photos).toHaveLength(0);

  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  await recheckStarted;
  await expect(page.locator('[data-upload-state="pending"]')).toHaveCount(1);
  await expect(discardButton).toHaveCount(0);
  releaseRecheck();
  await expect(page.locator('[data-upload-state="pending"]')).toHaveCount(1);
  await expect(discardButton).toHaveCount(0);
  expect(uploadKeys).toHaveLength(1);
  expect(lookups).toHaveLength(2);

  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  await expect(page.locator('[data-upload-state="failed"]')).toHaveCount(1);
  await expect(discardButton).toBeVisible();
  expect(uploadKeys).toHaveLength(1);
  expect(lookups).toHaveLength(3);
  expect(deleteRequests).toEqual([]);
  expect(photos).toHaveLength(0);

  await discardButton.click();
  await expect(page.getByTestId("gallery-upload-item")).toHaveCount(0);
  expect(uploadKeys).toHaveLength(1);
  expect(lookups).toHaveLength(3);
  expect(deleteRequests).toEqual([]);
  expect(photos).toHaveLength(0);
});

test("@controlled an unavailable upload-status lookup stays pending until explicit recovery", async ({
  page,
}) => {
  let posts = 0;
  let lookups = 0;
  const deleteRequests: string[] = [];
  const { photos } = await install(page, async (route) => {
    posts++;
    await route.fulfill({
      status: 422,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "invalid_file" } }),
    });
  });
  page.on("request", (request) => {
    if (request.method() === "DELETE" && request.url().includes("/gallery/"))
      deleteRequests.push(request.url());
  });
  await page.route(`**${gallery}/upload-status`, async (route) => {
    lookups++;
    if (lookups === 1) {
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "temporarily_unavailable" } }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "failed",
        photo_id: "00000000-0000-4000-8000-000000000098",
      }),
    });
  });

  await page.locator("#gallery-photo-upload").setInputFiles(file(10));
  await expect(page.locator('[data-upload-state="pending"]')).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "Consultar estado de carga" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Reintentar carga con el mismo archivo" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Descartar carga fallida" }),
  ).toHaveCount(0);
  expect(posts).toBe(1);
  expect(lookups).toBe(1);
  expect(photos).toHaveLength(0);

  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  await expect(page.locator('[data-upload-state="failed"]')).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "Descartar carga fallida" }),
  ).toBeVisible();
  expect(posts).toBe(1);
  expect(lookups).toBe(2);
  expect(deleteRequests).toEqual([]);
  expect(photos).toHaveLength(0);
});

test("@controlled explicit ready lookup recovers a prior unavailable check without replay", async ({
  page,
}) => {
  let posts = 0;
  let lookups = 0;
  let galleryReads = 0;
  let addUploadedPhoto: (() => Photo) | undefined;
  let confirmedPhotoId = "";
  const deleteRequests: string[] = [];
  const initialPhotoId = "00000000-0000-4000-8000-000000000010";
  const initialPhoto: Photo = {
    id: initialPhotoId,
    space_id: spaceId,
    alt_text: "Foto sintética inicial",
    visible: true,
    created_at: "2026-11-11T12:00:00Z",
    state: "ready",
    position: 1,
    cover: true,
    mime_type: "image/webp",
    size_bytes: 42,
    variants: [
      {
        name: "thumbnail",
        url: `/api/v1/spaces/${spaceId}/gallery/${initialPhotoId}/variants/thumbnail`,
        mime_type: "image/webp",
        size_bytes: 42,
        sha256: "a".repeat(64),
        width: 10,
        height: 10,
      },
    ],
  };
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (request.method() === "GET" && url.pathname === gallery) galleryReads++;
    if (request.method() === "DELETE" && url.pathname.includes("/gallery/"))
      deleteRequests.push(request.url());
  });
  await install(
    page,
    async (route, _request, add) => {
      posts++;
      addUploadedPhoto = add;
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "internal_error" } }),
      });
    },
    [initialPhoto],
  );
  await expect(
    page.locator(
      `[data-testid="gallery-photo"][data-photo-id="${initialPhotoId}"]`,
    ),
  ).toBeVisible();
  const initialGalleryReads = galleryReads;
  await page.route(`**${gallery}/upload-status`, async (route) => {
    lookups++;
    if (lookups === 1) {
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: { code: "temporarily_unavailable" } }),
      });
      return;
    }
    const add = addUploadedPhoto;
    expect(add).toBeDefined();
    const photo = add!();
    confirmedPhotoId = photo.id;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ state: "ready", photo_id: photo.id }),
    });
  });

  await page.locator("#gallery-photo-upload").setInputFiles(file(11));
  await expect(page.locator('[data-upload-state="pending"]')).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "Reintentar carga con el mismo archivo" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Descartar carga fallida" }),
  ).toHaveCount(0);
  expect(posts).toBe(1);
  expect(lookups).toBe(1);

  await page.getByRole("button", { name: "Consultar estado de carga" }).click();
  await expect(page.locator('[data-upload-state="ready"]')).toHaveCount(1);
  await expect.poll(() => galleryReads).toBeGreaterThan(initialGalleryReads);
  expect(confirmedPhotoId).toMatch(/^[0-9a-f-]{36}$/i);
  await expect(
    page.locator(
      `[data-testid="gallery-photo"][data-photo-id="${confirmedPhotoId}"]`,
    ),
  ).toBeVisible();
  expect(posts).toBe(1);
  expect(lookups).toBe(2);
  expect(deleteRequests).toEqual([]);
});
