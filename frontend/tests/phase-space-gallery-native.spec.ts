import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { deflateSync } from "node:zlib";

function syntheticPng(seed: number): Buffer {
  const crc = (data: Buffer) => {
    let value = 0xffffffff;
    for (const byte of data) {
      value ^= byte;
      for (let bit = 0; bit < 8; bit++)
        value = (value >>> 1) ^ (value & 1 ? 0xedb88320 : 0);
    }
    return (value ^ 0xffffffff) >>> 0;
  };
  const chunk = (kind: string, body: Buffer) => {
    const data = Buffer.concat([Buffer.from(kind), body]);
    const size = Buffer.alloc(4);
    size.writeUInt32BE(body.length);
    const checksum = Buffer.alloc(4);
    checksum.writeUInt32BE(crc(data));
    return Buffer.concat([size, data, checksum]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(64, 0);
  header.writeUInt32BE(48, 4);
  header[8] = 8;
  header[9] = 2;
  const pixels = Buffer.alloc(48 * (1 + 64 * 3));
  for (let y = 0; y < 48; y++)
    for (let x = 0; x < 64; x++) {
      const offset = y * 193 + 1 + x * 3;
      pixels[offset] = (x * 3 + seed * 41) % 256;
      pixels[offset + 1] = (y * 5 + seed * 23) % 256;
      pixels[offset + 2] = (x + y + seed * 61) % 256;
    }
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk("IHDR", header),
    chunk("IDAT", deflateSync(pixels)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

test.use({ trace: "off", screenshot: "off", video: "off" });
test.describe.configure({ mode: "serial" });

const enabled = process.env.PHASE_SPACE_GALLERY_NATIVE_E2E === "1";
const privateDir = process.env.PHASE6_PRIVATE_TESTING_DIR;
const inventoryPath = process.env.PHASE6_ACCOUNT_INVENTORY_FILE;
const fixtureSuffix = process.env.PHASE_SPACE_GALLERY_FIXTURE_SUFFIX;
if (fixtureSuffix && !/^\d{8}-[a-z0-9]{1,8}$/.test(fixtureSuffix)) {
  throw new Error("Synthetic gallery fixture selector is invalid");
}
const fixtureTitle = `F6 local synthetic gallery fixture${fixtureSuffix ? ` ${fixtureSuffix}` : ""}`;

type Account = {
  slot: string;
  email: string;
  password: string;
  account_id: string;
  registration_http: number;
  verification_http: number;
};
type GalleryPhoto = {
  id: string;
  space_id: string;
  state?: string;
  cover?: boolean;
  position?: number;
  alt_text?: string;
  variants?: Array<{ name: string; url: string; sha256: string }>;
};
type Space = { id: string; title: string };

function accountFile(): string {
  if (!privateDir || !path.isAbsolute(privateDir)) {
    throw new Error("Authorized private test-data directory is unavailable");
  }
  const root = path.resolve(privateDir);
  const selected = inventoryPath
    ? path.resolve(inventoryPath)
    : path.resolve(
        root,
        "f5-gate-20261101-f5a3/f5-a3-accounts-20261103T102133Z.json",
      );
  const relative = path.relative(root, selected);
  if (
    (inventoryPath !== undefined &&
      (!path.isAbsolute(inventoryPath) ||
        path.resolve(inventoryPath) !== inventoryPath)) ||
    relative === "" ||
    relative === ".." ||
    relative.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relative)
  ) {
    throw new Error("Authorized private account inventory path is invalid");
  }
  return selected;
}

async function authorizedAccounts() {
  if (!privateDir || !path.isAbsolute(privateDir)) {
    throw new Error("Authorized private test-data directory is unavailable");
  }
  let policy: {
    database: { host: string; port: number; name: string };
    account_policy: { never_print_credentials_or_tokens: boolean };
  };
  let inventory: { target: Record<string, unknown>; accounts: Account[] };
  try {
    [policy, inventory] = await Promise.all([
      readFile(path.join(privateDir, "test-account-policy.json"), "utf8").then(
        (value) => JSON.parse(value) as typeof policy,
      ),
      readFile(accountFile(), "utf8").then(
        (value) => JSON.parse(value) as typeof inventory,
      ),
    ]);
  } catch {
    throw new Error(
      "Authorized private synthetic-account materials are unavailable",
    );
  }
  expect(policy.account_policy.never_print_credentials_or_tokens).toBe(true);
  expect(policy.database).toMatchObject({
    host: "127.0.0.1",
    port: 15434,
    name: "espacigo_f5_gate_20261101",
  });
  expect(inventory.target).toMatchObject({
    host: "127.0.0.1",
    port: 15434,
    database: "espacigo_f5_gate_20261101",
  });
  const owner = inventory.accounts.find((account) => account.slot === "owner");
  const other = inventory.accounts.find((account) => account.slot !== "owner");
  for (const account of [owner, other]) {
    expect(account?.email).toBeTruthy();
    expect(account?.password).toBeTruthy();
    expect(account?.registration_http).toBe(201);
    expect(account?.verification_http).toBe(204);
  }
  return { owner: owner!, other: other! };
}

async function login(
  page: Page,
  account: Account,
  returnTo = "/anfitrion/espacios",
) {
  await page.goto(`/acceso?return_to=${encodeURIComponent(returnTo)}`);
  const email = page.getByLabel("Correo electrónico");
  const password = page.getByLabel("Contraseña", { exact: true });
  const responsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/login",
    { timeout: 15_000 },
  );
  try {
    await email.fill(account.email);
    await password.fill(account.password);
    const [, response] = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      responsePromise,
    ]);
    expect(response.status()).toBe(200);
    await expect(page).toHaveURL((url) => url.pathname === returnTo);
  } catch {
    await email.fill("", { timeout: 500 }).catch(() => undefined);
    await password.fill("", { timeout: 500 }).catch(() => undefined);
    throw new Error(
      "Authorized synthetic login did not reach its local destination",
    );
  }
}

async function logout(page: Page) {
  if (new URL(page.url()).pathname !== "/acceso") {
    const link = page.getByRole("link", { name: "Acceso" });
    if (await link.count()) await link.first().click();
  }
  await expect(page).toHaveURL(/\/acceso$/);
  await expect(page.getByText("Sesión activa")).toBeVisible();
  const responsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/logout",
    { timeout: 15_000 },
  );
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  expect((await responsePromise).status()).toBe(204);
  await page
    .getByLabel("Correo electrónico")
    .fill("", { timeout: 500 })
    .catch(() => undefined);
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("", { timeout: 500 })
    .catch(() => undefined);
}

test.skip(
  !enabled,
  "Requires the selected native Go API and isolated local database",
);

test("@space-gallery @native owner creates, edits and independently rehydrates real gallery media", async ({
  page,
}) => {
  test.setTimeout(300_000);
  const { owner, other } = await authorizedAccounts();
  const baseURL = test.info().project.use.baseURL;
  expect(typeof baseURL).toBe("string");
  const origin = new URL(baseURL as string).origin;
  expect(new URL(origin).hostname).toBe("127.0.0.1");
  expect(new URL(origin).protocol).toBe("https:");
  const observed: Array<{
    path: string;
    method: string;
    status?: number;
    bearer: boolean;
    cookie: boolean;
  }> = [];
  const secrets = [owner.email, owner.password, other.email, other.password];
  let nonLoopback = false;
  let crossOrigin = false;
  let secretInConsole = false;
  let mayHaveSession = false;
  let currentSpaceID = "";
  let galleryPath = "";

  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      ["http:", "https:"].includes(url.protocol) &&
      url.hostname !== "127.0.0.1"
    )
      nonLoopback = true;
    if (
      !url.pathname.startsWith("/api/backend/spaces/") ||
      !url.pathname.includes("/gallery")
    )
      return;
    if (url.origin !== origin) crossOrigin = true;
    const headers = request.headers();
    observed.push({
      path: url.pathname,
      method: request.method(),
      bearer: headers.authorization?.startsWith("Bearer ") ?? false,
      cookie: "cookie" in headers,
    });
  });
  page.on("response", (response) => {
    const item = observed.findLast(
      (request) =>
        request.path === new URL(response.url()).pathname &&
        request.method === response.request().method() &&
        request.status === undefined,
    );
    if (item) item.status = response.status();
  });
  page.on("console", (message) => {
    const text = message.text();
    if (
      secrets.some((secret) => text.includes(secret)) ||
      /\b(?:bearer\s+\S+|access[_ -]?token|authorization)\b/i.test(text)
    )
      secretInConsole = true;
  });

  try {
    mayHaveSession = true;
    await login(page, owner);
    const listPromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/spaces" &&
        response.request().method() === "GET",
    );
    await page.goto("/anfitrion/espacios");
    const listResponse = await listPromise;
    expect(listResponse.status()).toBe(200);
    const spaces = (await listResponse.json()) as { items: Space[] };
    let space = spaces.items.find((item) => item.title === fixtureTitle);
    if (space && fixtureSuffix) {
      throw new Error(
        "This native creation gate requires a fresh synthetic fixture selector",
      );
    }
    if (!space) {
      await page
        .getByRole("link", { name: "Publicar un espacio" })
        .first()
        .click();
      await expect(
        page.locator('#wizard-category option[value="oficina"]'),
      ).toHaveCount(1, { timeout: 15_000 });
      await page.locator("#wizard-category").selectOption("oficina");
      await page.locator("#wizard-title").fill(fixtureTitle);
      await page
        .locator("#wizard-description")
        .fill(
          "Espacio sintético local de ensayo para verificar el recorrido de galería y publicación. No corresponde a una oferta real ni contiene información de personas reales.",
        );
      await page.getByRole("button", { name: "Siguiente" }).click();
      await page
        .locator("#wizard-address")
        .fill("Dirección sintética local de ensayo");
      await page.locator("#wizard-region").selectOption("13");
      await page.locator("#wizard-comuna").fill("13101");
      await page.locator("#wizard-area").fill("35");
      await page.locator("#wizard-capacity").fill("2");
      await page.getByRole("checkbox", { name: "Wi-Fi", exact: true }).check();
      await page.getByRole("button", { name: "Siguiente" }).click();
      await page.locator("#wizard-rate-unit").selectOption("hora");
      await page.locator("#wizard-price").fill("6000");
      await page.getByRole("button", { name: "Siguiente" }).click();
      await page.getByRole("button", { name: "Siguiente" }).click();
      await expect(page.getByText(/clave de idempotencia/i)).toHaveCount(0);
      const creation = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/backend/spaces" &&
          response.request().method() === "POST",
      );
      const configuredTimeZone = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname.endsWith("/availability") &&
          response.request().method() === "PUT",
      );
      await page
        .getByRole("button", { name: "Crear espacio y continuar a edición" })
        .click();
      const created = await creation;
      expect(created.status()).toBe(201);
      expect(created.request().headers()["idempotency-key"]).toBeUndefined();
      const data = (await created.json()) as { id: string };
      expect((await configuredTimeZone).status()).toBe(200);
      await expect(page).toHaveURL(
        (url) => url.pathname === `/anfitrion/espacios/${data.id}/editar`,
      );
      space = { id: data.id, title: fixtureTitle };
    }
    currentSpaceID = space!.id;
    galleryPath = `/api/backend/spaces/${encodeURIComponent(currentSpaceID)}/gallery`;

    await page.goto(`/anfitrion/espacios/${currentSpaceID}/editar`);
    await expect(page.locator("h1")).toContainText(fixtureTitle);
    await page.reload();
    await expect(page.locator("h1")).toContainText(fixtureTitle);
    const editedTitle = `${fixtureTitle} editado`;
    await page.locator("#editor-title").fill(editedTitle);
    const editResponse = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname ===
          `/api/backend/spaces/${currentSpaceID}` &&
        response.request().method() === "PUT",
    );
    await page.getByRole("button", { name: "Guardar contenido" }).click();
    expect((await editResponse).status()).toBe(200);
    await page.reload();
    await expect(page.locator("h1")).toContainText(editedTitle);
    await page.getByRole("tab", { name: /Fotografías/ }).click();
    await expect(
      page.getByRole("tabpanel", { name: /Fotografías/ }),
    ).toBeVisible();

    const galleryRead = async () => {
      const response = await page.request.get(`${origin}${galleryPath}`);
      expect(response.status()).toBe(200);
      return (await response.json()) as {
        items: GalleryPhoto[];
        max_photos: number;
      };
    };
    let gallery = await galleryRead();
    expect(gallery.max_photos).toBe(30);

    const uploadThroughUI = async (seed: number) => {
      await expect(page.locator("#gallery-photo-upload")).toBeEnabled();
      const uploadResponsePromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === galleryPath &&
          response.request().method() === "POST",
      );
      await page.locator("#gallery-photo-upload").setInputFiles({
        name: `synthetic-${seed}.png`,
        mimeType: "image/png",
        buffer: syntheticPng(seed),
      });
      const uploaded = await uploadResponsePromise;
      expect([201, 200].includes(uploaded.status())).toBe(true);
      const uploadedBody = (await uploaded.json()) as {
        item?: GalleryPhoto;
        photo?: GalleryPhoto;
      };
      const id = uploadedBody.item?.id ?? uploadedBody.photo?.id;
      expect(/^[0-9a-f-]{36}$/i.test(id ?? "")).toBe(true);
      await expect(
        page.locator(`[data-testid="gallery-photo"][data-photo-id="${id}"]`),
      ).toBeVisible();
      await expect
        .poll(async () =>
          (await galleryRead()).items.some((photo) => photo.id === id),
        )
        .toBe(true);
      return id!;
    };
    await uploadThroughUI(1);
    const createdPhotoID = await uploadThroughUI(2);
    await uploadThroughUI(3);

    gallery = await galleryRead();
    expect(gallery.items.some((photo) => photo.id === createdPhotoID)).toBe(
      true,
    );
    expect(gallery.items.length).toBeGreaterThanOrEqual(1);
    expect(
      gallery.items.every(
        (photo, index) =>
          photo.position === undefined || photo.position === index + 1,
      ),
    ).toBe(true);
    const card = (id: string) =>
      page.locator(`[data-testid="gallery-photo"][data-photo-id="${id}"]`);
    const coverResponse = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname ===
          `/api/backend/spaces/${currentSpaceID}/cover` &&
        response.request().method() === "PUT",
    );
    await card(createdPhotoID)
      .getByRole("button", { name: "Marcar portada" })
      .click();
    expect((await coverResponse).status()).toBe(200);
    await expect
      .poll(
        async () =>
          (await galleryRead()).items.find(
            (photo) => photo.id === createdPhotoID,
          )?.cover,
      )
      .toBe(true);
    const cover = await galleryRead();
    expect(cover.items.filter((photo) => photo.cover).length).toBe(1);

    await card(createdPhotoID)
      .getByRole("button", { name: "Mover foto antes" })
      .click();
    await expect
      .poll(
        async () =>
          (await galleryRead()).items.find(
            (photo) => photo.id === createdPhotoID,
          )?.position,
      )
      .toBe(1);
    const replacementPath = `${galleryPath}/${createdPhotoID}/content`;
    const priorPhoto = (await galleryRead()).items.find(
      (photo) => photo.id === createdPhotoID,
    )!;
    await expect(
      card(createdPhotoID).getByLabel("Reemplazar fotografía"),
    ).toBeEnabled();
    const failedReplace = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === replacementPath &&
        response.request().method() === "PUT",
    );
    await card(createdPhotoID)
      .getByLabel("Reemplazar fotografía")
      .setInputFiles({
        name: "invalid.png",
        mimeType: "image/png",
        buffer: Buffer.from("not a valid image"),
      });
    const rejected = await failedReplace;
    expect(rejected.status()).toBeGreaterThanOrEqual(400);
    const afterFailure = await galleryRead();
    expect(
      JSON.stringify(
        afterFailure.items.find((photo) => photo.id === createdPhotoID),
      ) === JSON.stringify(priorPhoto),
    ).toBe(true);
    await expect(
      card(createdPhotoID).getByLabel("Reemplazar fotografía"),
    ).toBeEnabled();
    expect(
      afterFailure.items.some(
        (photo) =>
          photo.id === createdPhotoID && photo.cover && photo.position === 1,
      ),
    ).toBe(true);
    const successReplace = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === replacementPath &&
        response.request().method() === "PUT",
    );
    await card(createdPhotoID)
      .getByLabel("Reemplazar fotografía")
      .setInputFiles({
        name: "replacement.png",
        mimeType: "image/png",
        buffer: syntheticPng(4),
      });
    const replaced = await successReplace;
    expect(replaced.status()).toBe(201);
    const newPhoto = ((await replaced.json()) as { photo: GalleryPhoto }).photo;
    await expect
      .poll(async () =>
        (await galleryRead()).items.some(
          (photo) =>
            photo.id === newPhoto.id && photo.cover && photo.position === 1,
        ),
      )
      .toBe(true);
    expect(
      (await galleryRead()).items.some((photo) => photo.id === createdPhotoID),
    ).toBe(false);
    page.once("dialog", async (dialog) => {
      expect(dialog.type()).toBe("confirm");
      await dialog.accept();
    });
    const removed = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === `${galleryPath}/${newPhoto.id}` &&
        response.request().method() === "DELETE",
    );
    await card(newPhoto.id)
      .getByRole("button", { name: "Eliminar foto" })
      .click();
    const removalResponse = await removed;
    expect(removalResponse.status()).toBe(200);
    const removal = (await removalResponse.json()) as {
      removed: boolean;
      photo_id: string;
    };
    expect(removal.removed).toBe(true);
    expect(removal.photo_id === newPhoto.id).toBe(true);
    await expect.poll(async () => (await galleryRead()).items.length).toBe(2);

    // Removing the cover clears it by contract; explicitly choose a surviving photo.
    const survivingPhoto = (await galleryRead()).items[0]!;
    await card(survivingPhoto.id)
      .getByRole("button", { name: "Marcar portada" })
      .click();
    await expect
      .poll(
        async () => (await galleryRead()).items.filter((p) => p.cover).length,
      )
      .toBe(1);

    // Repetitive cap setup uses the real same-origin BFF, not mocks or SQL.
    for (let seed = 10; (await galleryRead()).items.length < 30; seed++) {
      const response = await page.request.post(`${origin}${galleryPath}`, {
        headers: { Origin: origin, "Idempotency-Key": crypto.randomUUID() },
        multipart: {
          image: {
            name: `capacity-${seed}.png`,
            mimeType: "image/png",
            buffer: syntheticPng(seed),
          },
        },
      });
      expect(response.status()).toBe(201);
    }
    const overflow = await page.request.post(`${origin}${galleryPath}`, {
      headers: { Origin: origin, "Idempotency-Key": crypto.randomUUID() },
      multipart: {
        image: {
          name: "overflow.png",
          mimeType: "image/png",
          buffer: syntheticPng(99),
        },
      },
    });
    expect(overflow.status()).toBe(409);
    expect((await galleryRead()).items.length).toBe(30);

    // Reload forces independent server hydration instead of trusting optimistic UI state.
    await page.reload();
    await page.getByRole("tab", { name: /Fotografías/ }).click();
    gallery = await galleryRead();
    expect(gallery.items.length).toBe(30);
    await expect(page.getByTestId("gallery-photo")).toHaveCount(30);
    await expect(
      page.getByTestId("gallery-photo").first().getByRole("img"),
    ).toBeVisible();
    await expect
      .poll(async () =>
        page
          .getByTestId("gallery-photo")
          .getByRole("img")
          .evaluateAll((images) =>
            images.every(
              (image) =>
                image instanceof HTMLImageElement &&
                image.complete &&
                image.naturalWidth > 0,
            ),
          ),
      )
      .toBe(true);
    for (const viewport of [
      { width: 1365, height: 900 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      const accessibility = await new AxeBuilder({ page })
        .include('[role="tabpanel"]')
        .analyze();
      expect(accessibility.violations.map((violation) => violation.id)).toEqual(
        [],
      );
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
    }
    await page.setViewportSize({ width: 1365, height: 900 });
    await expect(
      page.getByRole("button", { name: "Publicar", exact: true }),
    ).toBeEnabled();
    const publication = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname ===
          `/api/backend/spaces/${currentSpaceID}/publication` &&
        response.request().method() === "PUT",
    );
    await page.getByRole("button", { name: "Publicar", exact: true }).click();
    expect((await publication).status()).toBe(200);
    const persisted = await page.request.get(
      `${origin}/api/backend/spaces/${currentSpaceID}`,
    );
    expect(persisted.status()).toBe(200);
    expect(((await persisted.json()) as { state: string }).state).toBe(
      "activa",
    );

    for (const request of observed) {
      expect(
        request.bearer,
        "Browser requests must not carry an opaque bearer",
      ).toBe(false);
    }
    expect(nonLoopback).toBe(false);
    expect(crossOrigin).toBe(false);
    expect(secretInConsole).toBe(false);
    await logout(page);
    mayHaveSession = false;

    // A different authorized account must not read another owner's gallery.
    mayHaveSession = true;
    await login(page, other, `/anfitrion/espacios/${currentSpaceID}/editar`);
    const denied = await page.request.get(`${origin}${galleryPath}`);
    expect(denied.status()).toBe(403);
    const denial = (await denied.json()) as {
      error?: { code?: string };
      items?: unknown;
    };
    expect(denial.error?.code).toBe("forbidden");
    expect(denial.items).toBeUndefined();
    await logout(page);
    mayHaveSession = false;
  } finally {
    if (mayHaveSession) await logout(page).catch(() => undefined);
  }
});
