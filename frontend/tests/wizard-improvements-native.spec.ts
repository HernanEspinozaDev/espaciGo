import { expect, test, type Page } from "@playwright/test";
import {
  chmod,
  open,
  readFile,
  rename,
  stat,
  writeFile,
} from "node:fs/promises";
import { randomUUID } from "node:crypto";
import path from "node:path";

const enabled = process.env.ESPACIGO_WIZARD_NATIVE === "1";
const baseURL = "https://127.0.0.1:4206";
const privateDir = path.resolve(
  process.cwd(),
  "../.local/testing/wizard-improvements",
);
const inventoryPath = path.join(
  privateDir,
  "wizard-improvements-account-inventory.json",
);
const receiptPath = path.join(privateDir, "native-wizard-receipt.json");
const titlePrefix = "Ensayo wizard tarifas nativas";
const description =
  "Este espacio sintético local permite comprobar el recorrido completo del asistente, la validación de sus campos y la persistencia real de los datos y tarifas ingresados.";

test.use({ trace: "off", screenshot: "off", video: "off" });
test.describe.configure({ mode: "serial" });
test.skip(!enabled, "Opt-in native local wizard trial only");

type Account = { email?: unknown; password?: unknown };
type Receipt = { titles?: Partial<Record<RateUnit, string>> };
type RateUnit = "hora" | "dia" | "mes";
type Intent = {
  unit: RateUnit;
  title: string;
  status: "prepared" | "created" | "verified";
  createdAt: string;
  spaceId?: string;
};

function intentPath(unit: RateUnit): string {
  return path.join(privateDir, `native-wizard-intent-${unit}.json`);
}

async function writeIntentExclusive(intent: Intent): Promise<void> {
  let handle;
  try {
    handle = await open(intentPath(intent.unit), "wx", 0o600);
  } catch {
    throw new Error(
      "Native creation intent already exists; manual recovery required; no payload recorded",
    );
  }
  try {
    await handle.writeFile(JSON.stringify(intent));
    await handle.sync();
  } finally {
    await handle.close();
  }
}

async function updateIntent(intent: Intent): Promise<void> {
  const target = intentPath(intent.unit);
  const temporary = `${target}.${randomUUID()}.tmp`;
  const handle = await open(temporary, "wx", 0o600);
  try {
    await handle.writeFile(JSON.stringify(intent));
    await handle.sync();
  } finally {
    await handle.close();
  }
  await rename(temporary, target);
  const directory = await open(privateDir, "r");
  try {
    await directory.sync();
  } finally {
    await directory.close();
  }
}

function safePath(value: string): string {
  return new URL(value).pathname;
}

async function clearLogin(page: Page): Promise<void> {
  await page
    .getByLabel("Correo electrónico")
    .fill("")
    .catch(() => undefined);
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("")
    .catch(() => undefined);
}

async function login(page: Page, account: Account): Promise<void> {
  await page.goto("/acceso?return_to=%2Fanfitrion%2Fespacios%2Fnuevo");
  const email = page.getByLabel("Correo electrónico");
  const password = page.getByLabel("Contraseña", { exact: true });
  try {
    const responsePromise = page.waitForResponse(
      (r) =>
        safePath(r.url()) === "/api/backend/auth/login" &&
        r.request().method() === "POST",
      { timeout: 15_000 },
    );
    await email.fill(account.email as string);
    await password.fill(account.password as string);
    const response = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      responsePromise,
    ]).then(([, r]) => r);
    if (response.status() !== 200) throw new Error("login status rejected");
    await page.waitForURL(
      (url) => url.pathname === "/anfitrion/espacios/nuevo",
    );
  } catch {
    await clearLogin(page);
    throw new Error(
      "Native test account login failed; private form values cleared",
    );
  }
}

async function saveReceipt(unit: RateUnit, title: string): Promise<void> {
  await chmod(privateDir, 0o700);
  let receipt: Receipt = {};
  try {
    receipt = JSON.parse(await readFile(receiptPath, "utf8")) as Receipt;
  } catch {
    // First successful readback starts a private receipt.
  }
  receipt.titles = { ...receipt.titles, [unit]: title };
  await writeFile(receiptPath, JSON.stringify(receipt, null, 2), {
    mode: 0o600,
  });
  await chmod(receiptPath, 0o600);
}

test.beforeAll(async () => {
  if (!enabled) return;
  const info = await stat(privateDir);
  if ((info.mode & 0o777) !== 0o700)
    throw new Error("Private trial directory permissions must be 0700");
  const file = await stat(inventoryPath);
  if ((file.mode & 0o777) !== 0o600)
    throw new Error("Private account inventory permissions must be 0600");
});

test("@native reconciles existing hora draft without mutations", async ({
  page,
}) => {
  test.setTimeout(90_000);
  const intent = JSON.parse(
    await readFile(intentPath("hora"), "utf8"),
  ) as Intent;
  if (intent.unit !== "hora" || intent.status !== "created" || !intent.spaceId)
    throw new Error(
      "Existing hora intent is not eligible for read-only reconciliation",
    );
  const accountList = JSON.parse(
    await readFile(inventoryPath, "utf8"),
  ) as Account[];
  const account = accountList[0];
  if (
    typeof account?.email !== "string" ||
    typeof account.password !== "string"
  )
    throw new Error("Private verified-account inventory is incomplete");

  const spaceMutations: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      url.origin !== new URL(baseURL).origin ||
      url.pathname.startsWith("/api/backend/auth/")
    )
      return;
    if (
      ["POST", "PUT", "PATCH", "DELETE"].includes(request.method()) &&
      url.pathname.startsWith("/api/backend/spaces")
    )
      spaceMutations.push(`${request.method()} ${url.pathname}`);
  });
  await login(page, account);
  await page.goto(`/anfitrion/espacios/${intent.spaceId}/editar`);
  await expect(page.locator("#editor-title")).toHaveValue(intent.title);
  const detailResponse = await page.request.get(
    `/api/backend/spaces/${intent.spaceId}`,
  );
  expect(detailResponse.status()).toBe(200);
  const detail = await detailResponse.json();
  expect(detail).toMatchObject({
    title: intent.title,
    rate_unit: "hora",
    time_zone: "America/Santiago",
  });
  const hoursResponse = await page.request.get(
    `/api/backend/spaces/${intent.spaceId}/weekly-hours`,
  );
  expect(hoursResponse.status()).toBe(200);
  const hours = (await hoursResponse.json()).data;
  expect(hours).toMatchObject({ enabled: true, time_zone: "America/Santiago" });
  expect(hours.days).toHaveLength(7);
  const weekdays = hours.days.filter((day: { weekday: number }) =>
    [1, 2, 3, 4, 5].includes(day.weekday),
  );
  expect(weekdays).toHaveLength(5);
  for (const day of weekdays) {
    expect(day.periods).toEqual([{ open: "09:00", close: "18:00" }]);
  }
  await page.reload();
  await expect(page.locator("#editor-title")).toHaveValue(intent.title);
  expect(spaceMutations).toEqual([]);
  intent.status = "verified";
  await updateIntent(intent);
});

for (const unit of ["hora", "dia", "mes"] as const) {
  test(`@native creates and reads back a real ${unit} draft`, async ({
    page,
  }) => {
    test.setTimeout(90_000);
    try {
      await stat(intentPath(unit));
      throw new Error(
        "Native creation intent already exists; manual recovery required; no payload recorded",
      );
    } catch (error) {
      if (
        error instanceof Error &&
        error.message.startsWith("Native creation intent")
      )
        throw error;
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
    }
    const accountList = JSON.parse(
      await readFile(inventoryPath, "utf8"),
    ) as Account[];
    const account = accountList[0];
    if (
      typeof account?.email !== "string" ||
      typeof account.password !== "string"
    )
      throw new Error("Private verified-account inventory is incomplete");

    const mutationCounts = new Map<string, number>();
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (url.origin !== new URL(baseURL).origin) return;
      const method = request.method();
      const pathname = url.pathname;
      if (
        ["POST", "PUT"].includes(method) &&
        pathname.startsWith("/api/backend/spaces")
      ) {
        const key = `${method} ${pathname}`;
        mutationCounts.set(key, (mutationCounts.get(key) ?? 0) + 1);
      }
    });

    await login(page, account);
    await expect(
      page.getByRole("heading", { name: "Publicar un nuevo espacio" }),
    ).toBeVisible();
    await page.locator("#wizard-category").selectOption("inmuebles");
    const profileResponse = await page.request.get(
      "/api/backend/spaces/categories/inmuebles/attributes",
    );
    expect(profileResponse.ok()).toBeTruthy();
    const profile = (await profileResponse.json()) as {
      attributes: Array<{
        code: string;
        required?: boolean;
        type: string;
        options?: string[];
        minimum?: number;
        maximum?: number;
      }>;
    };
    const title = `${titlePrefix} ${unit} ${Date.now()}`;
    await page.locator("#wizard-title").fill(title);
    await page.locator("#wizard-description").fill(description);
    await page.getByRole("button", { name: "Siguiente" }).click();

    await page.locator("#wizard-region").selectOption("13");
    await expect(page.locator("#wizard-comuna")).toBeEnabled();
    await expect(
      page.locator("#wizard-comuna option").filter({ hasText: /^Santiago$/ }),
    ).toHaveCount(1);
    await page.locator("#wizard-comuna").selectOption("13101");
    await page
      .locator("#wizard-address")
      .fill("Dirección sintética de ensayo 100");
    await page.locator("#wizard-area").fill("40");
    await page.locator("#wizard-capacity").fill("2");
    for (const attribute of profile.attributes.filter(
      (item) => item.required,
    )) {
      const control = page.locator(`#attr-${attribute.code}`);
      if (attribute.type === "enum") {
        const option = attribute.options?.[0];
        if (!option)
          throw new Error(`Required enum has no option: ${attribute.code}`);
        await control.selectOption(option);
      } else if (attribute.type === "boolean") {
        await control.check();
      } else if (attribute.type === "enum_list") {
        if (!attribute.options?.length)
          throw new Error(
            `Required enum list has no option: ${attribute.code}`,
          );
        await page.locator(`#attr-${attribute.code}-option-0`).check();
      } else {
        const value = Math.max(attribute.minimum ?? 1, 1);
        if (attribute.maximum !== undefined && value > attribute.maximum)
          throw new Error(`No safe synthetic value for ${attribute.code}`);
        await control.fill(String(value));
      }
    }
    await page.getByRole("button", { name: "Siguiente" }).click();

    await page.locator("#wizard-rate-unit").selectOption(unit);
    await page.locator("#wizard-price").fill("5001");
    await page.getByRole("button", { name: "Siguiente" }).click();
    await expect(page.locator("#wizard-timezone")).toHaveValue(
      "America/Santiago",
    );
    if (unit !== "hora") {
      await expect(
        page.getByLabel("Habilitar calendario de reservas semanal"),
      ).toHaveCount(0);
      await expect(
        page.getByText("Horario semanal no aplicable a tarifas por día o mes", {
          exact: false,
        }),
      ).toBeVisible();
    } else {
      await expect(
        page.getByLabel("Habilitar calendario de reservas semanal"),
      ).toBeVisible();
      await page.getByLabel("Habilitar calendario de reservas semanal").check();
    }
    await page.getByRole("button", { name: "Siguiente" }).click();
    await expect(page.getByRole("heading", { name: /Paso 5:/ })).toBeVisible();
    await page
      .getByRole("navigation", { name: "Pasos del asistente de publicación" })
      .getByRole("button", { name: "Disponibilidad", exact: true })
      .click();
    await expect(page.locator("#wizard-timezone")).toHaveValue(
      "America/Santiago",
    );
    await page
      .getByRole("navigation", { name: "Pasos del asistente de publicación" })
      .getByRole("button", { name: /^Confirmación/ })
      .click();
    await expect(page.locator("main")).toContainText(title);
    await expect(page.locator("main")).toContainText("5.001");

    const intent: Intent = {
      unit,
      title,
      status: "prepared",
      createdAt: new Date().toISOString(),
    };
    await writeIntentExclusive(intent);
    const post = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        safePath(r.url()) === "/api/backend/spaces",
      { timeout: 30_000 },
    );
    await page
      .getByRole("button", { name: "Crear espacio y continuar a edición" })
      .click();
    const created = await post;
    expect(created.status()).toBe(201);
    const creationBody = (await created.json().catch(() => null)) as {
      id?: unknown;
    } | null;
    const spaceId =
      typeof creationBody?.id === "string"
        ? creationBody.id
        : created.url().match(/[0-9a-f-]{36}/i)?.[0];
    if (!spaceId || !/^[0-9a-f-]{36}$/i.test(spaceId))
      throw new Error("Creation returned no valid space UUID; intent retained");
    intent.status = "created";
    intent.spaceId = spaceId;
    await updateIntent(intent);
    await page.waitForURL(/\/anfitrion\/espacios\/[0-9a-f-]+\/editar$/i);
    const detailPath = new URL(page.url()).pathname;
    expect(detailPath.split("/").at(-2)).toBe(spaceId);
    const readback = await page.evaluate(async (id) => {
      const response = await fetch(`/api/backend/spaces/${id}`, {
        cache: "no-store",
      });
      const body = (await response.json().catch(() => null)) as Record<
        string,
        unknown
      > | null;
      return {
        status: response.status,
        title: body?.title,
        rate_unit: body?.rate_unit,
        time_zone: body?.time_zone,
        base_price_clp: body?.base_price_clp,
      };
    }, spaceId);
    expect(readback).toMatchObject({
      status: 200,
      title,
      rate_unit: unit,
      time_zone: "America/Santiago",
      base_price_clp: 5001,
    });
    await expect(page.locator("#editor-title")).toHaveValue(title);

    if (unit === "hora") {
      expect(
        [...mutationCounts].some(
          ([key, count]) => key.endsWith("/weekly-hours") && count > 0,
        ),
      ).toBe(true);
    } else {
      expect(
        [...mutationCounts].some(([key]) => key.endsWith("/weekly-hours")),
      ).toBe(false);
    }
    intent.status = "verified";
    await updateIntent(intent);
    expect(
      [...mutationCounts.values()].reduce((sum, n) => sum + n, 0),
    ).toBeGreaterThan(0);
    await saveReceipt(unit, title);
    const logout = page.waitForResponse(
      (r) => safePath(r.url()) === "/api/backend/auth/logout",
      { timeout: 15_000 },
    );
    await page
      .getByRole("button", { name: "Cerrar sesión" })
      .click({ noWaitAfter: true });
    expect((await logout).status()).toBe(204);
  });
}
