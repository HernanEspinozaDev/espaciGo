import { expect, test, type Browser, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { chmod, mkdir, open, readFile, rename, stat } from "node:fs/promises";
import path from "node:path";

const enabled = process.env.ESPACIGO_WIZARD_PUBLIC_NATIVE === "1";
const origin = "https://127.0.0.1:4206";
const privateDir = path.resolve(
  process.cwd(),
  "../.local/testing/wizard-improvements",
);
const sourceIntent = path.join(privateDir, "native-wizard-intent-hora.json");
const intentPath = path.join(
  privateDir,
  "native-wizard-publication-intent.json",
);
const inventoryPath = path.join(
  privateDir,
  "wizard-improvements-account-inventory.json",
);
const evidenceDir =
  "/home/nandev/.hermes/cache/scratch/espacigo-wizard-evidence/native-public-final";

test.use({ trace: "off", screenshot: "off", video: "off" });
test.describe.configure({ mode: "serial" });
test.skip(
  !enabled,
  "Explicit opt-in: local native publication/public-readback only",
);

type Account = { email?: unknown; password?: unknown };
type Intent = {
  spaceId: string;
  title: string;
  state: string;
  updatedAt: string;
};
type PublicDTO =
  import("@/lib/api/generated/openapi").components["schemas"]["PublicSpaceDetail"];

async function persistIntent(
  intent: Intent,
  exclusive: boolean,
): Promise<void> {
  const temp = `${intentPath}.${randomUUID()}.tmp`;
  const handle = await open(temp, "wx", 0o600);
  try {
    await handle.writeFile(JSON.stringify(intent));
    await handle.sync();
  } finally {
    await handle.close();
  }
  if (exclusive) {
    try {
      await open(intentPath, "wx", 0o600).then(async (h) => {
        await h.writeFile(JSON.stringify(intent));
        await h.sync();
        await h.close();
      });
    } catch {
      await import("node:fs/promises").then(({ unlink }) => unlink(temp));
      throw new Error(
        "Publication intent already exists; do not replay; reconcile only",
      );
    }
    await import("node:fs/promises").then(({ unlink }) => unlink(temp));
  } else {
    await rename(temp, intentPath);
  }
  const dir = await open(privateDir, "r");
  try {
    await dir.sync();
  } finally {
    await dir.close();
  }
}

async function loadIntent(): Promise<Intent> {
  const raw = JSON.parse(await readFile(intentPath, "utf8")) as Partial<Intent>;
  if (
    !/^[0-9a-f-]{36}$/i.test(raw.spaceId ?? "") ||
    typeof raw.title !== "string"
  )
    throw new Error("Private publication intent is malformed");
  return raw as Intent;
}

async function login(page: Page): Promise<void> {
  const inventory = JSON.parse(
    await readFile(inventoryPath, "utf8"),
  ) as Account[];
  const account = inventory[0];
  if (
    typeof account?.email !== "string" ||
    typeof account.password !== "string"
  )
    throw new Error("Existing private verified-account inventory unavailable");
  await page.goto("/acceso?return_to=%2Fanfitrion%2Fespacios", {
    waitUntil: "domcontentloaded",
  });
  try {
    await page.getByLabel("Correo electrónico").fill(account.email);
    await page.getByLabel("Contraseña", { exact: true }).fill(account.password);
    const responsePromise = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    const response = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      responsePromise,
    ]).then(([, loginResponse]) => loginResponse);
    if (response.status() !== 200)
      throw new Error(`Login failed with HTTP ${response.status()}`);
    await page.waitForURL((url) =>
      url.pathname.startsWith("/anfitrion/espacios"),
    );
  } finally {
    for (const field of [
      page.getByLabel("Correo electrónico"),
      page.getByLabel("Contraseña", { exact: true }),
    ]) {
      if (await field.count())
        await field.fill("", { timeout: 1000 }).catch(() => undefined);
    }
  }
}

function safeKeys(value: Record<string, unknown>, allowed: string[]): void {
  expect(Object.keys(value).every((key) => allowed.includes(key))).toBe(true);
}

async function verifyAnonymous(
  browser: Browser,
  intent: Intent,
): Promise<void> {
  const context = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  const weeklyReads: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.origin !== origin) throw new Error("Non-loopback browser request");
    if (url.pathname.includes("weekly-hours") && request.method() === "GET")
      weeklyReads.push(url.pathname);
    if (
      ["POST", "PUT", "PATCH", "DELETE"].includes(request.method()) &&
      url.pathname.startsWith("/api/backend/public/")
    )
      throw new Error("Public page attempted a mutation");
  });
  const direct = await context.request.get(
    `/api/backend/public/spaces/${intent.spaceId}`,
    { headers: { accept: "application/json" } },
  );
  expect(direct.status()).toBe(200);
  const envelope =
    (await direct.json()) as import("@/lib/api/generated/openapi").components["schemas"]["PublicSpaceDetailEnvelope"];
  expect(envelope.data).toBeTruthy();
  const dto: PublicDTO = envelope.data!;
  safeKeys(dto as unknown as Record<string, unknown>, [
    "space_id",
    "category_code",
    "category_name",
    "title",
    "description",
    "area_m2",
    "rate_unit",
    "base_price_clp",
    "currency",
    "region_code",
    "region_name",
    "comuna_code",
    "comuna_name",
    "estimated_total_clp",
    "usage_rules",
    "features",
    "images",
    "weekly_schedule",
    "verification",
    "cover_image_id",
  ]);
  for (const key of ["space_id", "title", "weekly_schedule"])
    expect(dto).toHaveProperty(key);
  expect(dto).toMatchObject({ space_id: intent.spaceId, title: intent.title });
  const schedule = dto.weekly_schedule;
  expect(schedule).toBeTruthy();
  safeKeys(schedule as unknown as Record<string, unknown>, [
    "enabled",
    "time_zone",
    "days",
  ]);
  expect(schedule).toMatchObject({
    enabled: true,
    time_zone: "America/Santiago",
  });
  expect(schedule!.days.map((d) => d.weekday)).toEqual([1, 2, 3, 4, 5, 6, 7]);
  for (const day of schedule!.days)
    safeKeys(day as unknown as Record<string, unknown>, ["weekday", "periods"]);
  for (const day of schedule!.days)
    for (const period of day.periods)
      safeKeys(period as unknown as Record<string, unknown>, ["open", "close"]);
  expect(schedule!.days.find((d) => d.weekday === 1)?.periods).toEqual([
    { open: "09:00", close: "17:30" },
  ]);
  for (const day of schedule!.days.filter(
    (d) => d.weekday >= 2 && d.weekday <= 5,
  ))
    expect(day.periods).toEqual([{ open: "09:00", close: "18:00" }]);
  for (const day of schedule!.days.filter(
    (d) => d.weekday === 6 || d.weekday === 7,
  ))
    expect(day.periods).toEqual([]);

  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    await page.goto(`/espacios/${intent.spaceId}`, {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByRole("heading", { name: "Horario semanal habitual" }),
    ).toBeVisible();
    const calendar = page.getByRole("region", {
      name: "Horario semanal habitual",
      exact: true,
    });
    await expect(calendar).toBeVisible();
    const inputs = calendar.locator("input");
    await expect(inputs).toHaveCount(2);
    await expect(inputs.nth(0)).toBeDisabled();
    await expect(inputs.nth(0)).toHaveValue("09:00");
    await expect(inputs.nth(1)).toBeDisabled();
    await expect(inputs.nth(1)).toHaveValue("17:30");
    await expect(
      calendar.getByRole("button", { name: /^17:00–18:00/ }),
    ).toBeDisabled();
    await expect(
      calendar.getByRole("button", { name: "Lunes", exact: true }),
    ).toBeEnabled();
    await expect(calendar.locator("[aria-pressed='mixed']")).toHaveCount(1);
    await page.evaluate(async () => {
      await document.fonts.ready;
    });
    const overflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth <=
        document.documentElement.clientWidth,
    );
    expect(overflow).toBe(true);
    const axe = await import("@axe-core/playwright");
    const results = await new axe.default({ page }).analyze();
    expect(results.violations.map((v) => v.id)).toEqual([]);
    await mkdir(evidenceDir, { recursive: true, mode: 0o700 });
    await chmod(evidenceDir, 0o700);
    const screenshotPath = path.join(
      evidenceDir,
      viewport.width === 390 ? "mobile.png" : "desktop.png",
    );
    const stickyHeaderBottom = await page.evaluate(() => {
      let bottom = 0;
      for (const element of document.querySelectorAll<HTMLElement>("body *")) {
        const style = getComputedStyle(element);
        if (style.position !== "sticky" && style.position !== "fixed") continue;
        const rect = element.getBoundingClientRect();
        if (rect.top <= 0 && rect.bottom > 0)
          bottom = Math.max(bottom, rect.bottom);
      }
      return bottom;
    });
    await calendar.evaluate((element, headerBottom) => {
      const top = element.getBoundingClientRect().top;
      window.scrollTo(0, Math.max(0, window.scrollY + top - headerBottom - 24));
    }, stickyHeaderBottom);
    await page.evaluate(
      () =>
        new Promise<void>((resolve) => requestAnimationFrame(() => resolve())),
    );
    await expect(calendar).toBeInViewport({ ratio: 1 });
    const calendarTop = await calendar.evaluate(
      (element) => element.getBoundingClientRect().top,
    );
    expect(calendarTop).toBeGreaterThanOrEqual(stickyHeaderBottom);
    await calendar.screenshot({
      path: screenshotPath,
      animations: "disabled",
    });
    await chmod(screenshotPath, 0o600);
  }
  expect(weeklyReads).toEqual([]);
  await context.close();
}

test("@native-public publishes the verified hora draft once with fresh CAS then anonymously reads it", async ({
  browser,
  page,
}) => {
  const prior = await stat(intentPath)
    .then(() => true)
    .catch(() => false);
  if (prior)
    throw new Error(
      "Publication intent exists; fresh publish forbidden, run reconciliation case",
    );
  const privateMetadata = await stat(privateDir);
  expect(privateMetadata.isDirectory()).toBe(true);
  expect(privateMetadata.mode & 0o777).toBe(0o700);
  for (const file of [sourceIntent, inventoryPath]) {
    const metadata = await stat(file);
    expect(metadata.isFile()).toBe(true);
    expect(metadata.mode & 0o777).toBe(0o600);
  }
  const draft = JSON.parse(await readFile(sourceIntent, "utf8")) as {
    status: string;
    spaceId?: string;
    title: string;
  };
  if (draft.status !== "verified" || !draft.spaceId)
    throw new Error("Existing hora draft receipt is not verified");
  await login(page);
  await page.goto(`/anfitrion/espacios/${draft.spaceId}/editar`, {
    waitUntil: "domcontentloaded",
  });
  await expect(page.locator("#editor-title")).toHaveValue(draft.title);
  const publishButton = page.getByRole("button", {
    name: "Publicar",
    exact: true,
  });
  await expect(publishButton).toBeVisible();
  await expect(publishButton).toBeEnabled();
  const ownerGet = await page.request.get(
    `/api/backend/spaces/${draft.spaceId}`,
  );
  expect(ownerGet.status()).toBe(200);
  const owner = (await ownerGet.json()) as { state?: string };
  expect(owner.state).toBe("borrador");
  const etag = ownerGet.headers()["etag"];
  expect(etag).toBeTruthy();
  const intent: Intent = {
    spaceId: draft.spaceId,
    title: draft.title,
    state: "prepared",
    updatedAt: new Date().toISOString(),
  };
  // Exclusive durable intent precedes the one and only UI publication click.
  await persistIntent(intent, true);
  let publicationWrites = 0;
  page.on("request", (request) => {
    if (
      request.method() === "PUT" &&
      new URL(request.url()).pathname ===
        `/api/backend/spaces/${draft.spaceId}/publication`
    )
      publicationWrites++;
  });
  const pending = page.waitForResponse(
    (r) =>
      r.request().method() === "PUT" &&
      new URL(r.url()).pathname ===
        `/api/backend/spaces/${draft.spaceId}/publication`,
  );
  await page.getByRole("button", { name: "Publicar", exact: true }).click();
  const response = await pending;
  const body = (await response.json()) as { state?: string };
  expect(response.status()).toBe(200);
  expect(publicationWrites).toBe(1);
  expect(response.request().headers()["if-match"]).toBe(etag);
  expect(body.state).toBe("activa");
  const updated: Intent = {
    ...intent,
    state: "published-needs-public-readback",
    updatedAt: new Date().toISOString(),
  };
  await persistIntent(updated, false);
  await expect(
    page.getByRole("button", { name: "Archivar", exact: true }),
  ).toBeVisible();
  const independent = await page.request.get(
    `/api/backend/spaces/${draft.spaceId}`,
  );
  expect(independent.status()).toBe(200);
  expect((await independent.json()).state).toBe("activa");
  await verifyAnonymous(browser, updated);
  expect(publicationWrites).toBe(1);
  await persistIntent(
    { ...updated, state: "verified", updatedAt: new Date().toISOString() },
    false,
  );
});

test("@native-public reconciles an existing publication receipt without PUT", async ({
  browser,
  page,
}) => {
  const intent = await loadIntent();
  if (
    intent.state === "prepared" ||
    intent.state === "published-needs-public-readback"
  ) {
    // An uncertain prior run is reconciled only by authoritative reads; never click Publish again.
  } else if (intent.state !== "verified")
    throw new Error("Receipt state requires manual review");
  await login(page);
  const ownerResponse = await page.request.get(
    `/api/backend/spaces/${intent.spaceId}`,
  );
  expect(ownerResponse.status()).toBe(200);
  expect((await ownerResponse.json()).state).toBe("activa");
  let writes = 0;
  page.on("request", (r) => {
    if (
      r.method() === "PUT" &&
      new URL(r.url()).pathname.endsWith("/publication")
    )
      writes++;
  });
  await verifyAnonymous(browser, intent);
  expect(writes).toBe(0);
  await persistIntent(
    { ...intent, state: "verified", updatedAt: new Date().toISOString() },
    false,
  );
});
