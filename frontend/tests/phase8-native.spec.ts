import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.use({ trace: "off", screenshot: "off", video: "off" });
test.describe.configure({ mode: "serial" });

const enabled = process.env.PHASE8_NATIVE_API_E2E === "1";

function privatePath(name: string): string {
  return path.resolve(process.cwd(), "../.local/testing", name);
}

test.skip(
  !enabled,
  "Requires the explicitly configured current-HEAD API and native PostgreSQL runtime",
);

test("@phase8 @native authorized synthetic account accesses bookings and host reservations against local Go loopback without console leaks, closing session cleanly", async ({
  page,
}) => {
  test.setTimeout(60_000);
  let policy: {
    account_policy?: {
      never_print_credentials_or_tokens?: boolean;
      password?: string;
    };
  };
  let inventory: { email?: string; accountId?: string; status?: string };
  try {
    policy = JSON.parse(
      await readFile(privatePath("test-account-policy.json"), "utf8"),
    ) as typeof policy;
    inventory = JSON.parse(
      await readFile(privatePath("phase3-account-inventory.json"), "utf8"),
    ) as typeof inventory;
  } catch {
    test.skip(
      true,
      "Authorized private synthetic-account material is unavailable",
    );
    return;
  }
  const credentials = policy.account_policy;
  if (
    credentials?.never_print_credentials_or_tokens !== true ||
    typeof credentials.password !== "string" ||
    typeof inventory.email !== "string" ||
    inventory.status !== "verified"
  ) {
    test.skip(
      true,
      "Private policy does not authorize this existing-account test",
    );
    return;
  }

  const secrets = [inventory.email, credentials.password];
  let sessionConfirmed = false;
  let logoutConfirmed = false;
  let secretInConsole = false;
  let reservationsQueried = false;
  const nonLoopback = { value: false };

  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:"))
      nonLoopback.value = true;
    const urlPath = new URL(request.url()).pathname;
    if (urlPath === "/api/backend/local/booking-trial/reservations") {
      reservationsQueried = true;
    }
  });

  page.on("console", (message) => {
    const text = message.text();
    if (
      secrets.some((secret) => text.includes(secret)) ||
      /\b(?:bearer\s+\S+|access[_ -]?token|authorization)\b/i.test(text)
    ) {
      secretInConsole = true;
    }
  });

  // 1. Login con cuenta sintética autorizada redirigiendo a /reservas
  await page.goto("/acceso?return_to=%2Freservas");
  const email = page.getByLabel("Correo electrónico");
  const password = page.getByLabel("Contraseña", { exact: true });

  try {
    const loginResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login",
      { timeout: 15_000 },
    );
    await email.fill(inventory.email);
    await password.fill(credentials.password);
    const loginResponse = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      loginResponsePromise,
    ]).then(([, response]) => response);
    expect(loginResponse.status()).toBe(200);
    await expect(page).toHaveURL(/\/reservas$/);
    sessionConfirmed = true;
  } catch {
    await email.fill("", { timeout: 500 }).catch(() => undefined);
    await password.fill("", { timeout: 500 }).catch(() => undefined);
    throw new Error(
      "Authorized synthetic login did not reach the allowlisted destination /reservas",
    );
  }
  expect(sessionConfirmed).toBe(true);

  // 2. Consulta real de reservas en /reservas
  await expect(page.locator("h1")).toHaveText("Mis Reservas");
  expect(reservationsQueried).toBe(true);

  // 3. Navegación a bandeja de anfitrión /anfitrion/reservas
  await page.goto("/anfitrion/reservas");
  await expect(page).toHaveURL(/\/anfitrion\/reservas$/);
  await expect(page.locator("h1")).toHaveText(
    "Gestión de Reservas de Anfitrión",
  );

  // 4. Cierre de sesión limpio
  await page.getByRole("link", { name: "Acceso" }).click();
  await expect(page).toHaveURL(/\/acceso$/);
  await expect(page.getByText("Sesión activa")).toBeVisible();

  const logoutPromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/logout",
    { timeout: 15_000 },
  );
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  const logout = await logoutPromise;
  expect(logout.status()).toBe(204);
  logoutConfirmed = true;

  expect(logoutConfirmed).toBe(true);
  expect(nonLoopback.value).toBe(false);
  expect(secretInConsole).toBe(false);
});
