import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import path from "node:path";

const enabled = process.env.PHASE_REPUBLICATION_NATIVE_E2E === "1";
test.skip(!enabled, "Requires the authorized native local republication trial");
test.use({ trace: "off", screenshot: "off", video: "off" });

type Account = {
  slot: string;
  email: string;
  password: string;
  account_id: string;
};
type Fixture = {
  route: string;
  space_id: string;
  reservation_id: string;
  database: string;
  host: string;
  port: number;
};
async function login(page: Page, account: Account, destination: string) {
  await page.goto(`/acceso?return_to=${encodeURIComponent(destination)}`);
  try {
    await page.getByLabel("Correo electrónico").fill(account.email);
    await page.getByLabel("Contraseña", { exact: true }).fill(account.password);
    const response = page.waitForResponse(
      (r) => new URL(r.url()).pathname === "/api/backend/auth/login",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    expect((await response).status()).toBe(200);
    await expect(page).toHaveURL((url) => url.pathname === destination);
  } catch {
    await page
      .getByLabel("Correo electrónico")
      .fill("")
      .catch(() => undefined);
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("")
      .catch(() => undefined);
    throw new Error(
      "Authorized synthetic login failed; credential details suppressed",
    );
  }
}

// Fixture setup is separate and labelled: no booking/payment/signature flow is
// claimed here. Check-out/reception/publication use the limited real runtime.
test("@native @republication current owner confirms vacancy and republishes from Mis espacios", async ({
  browser,
}) => {
  const privateRoot = process.env.PHASE6_PRIVATE_TESTING_DIR;
  const inventoryFile = process.env.PHASE6_ACCOUNT_INVENTORY_FILE;
  const fixtureFile = process.env.PHASE_REPUBLICATION_FIXTURE_FILE;
  if (!privateRoot || !inventoryFile || !fixtureFile)
    throw new Error("Private synthetic trial inputs are required");
  for (const selected of [inventoryFile, fixtureFile]) {
    const relative = path.relative(privateRoot, selected);
    if (
      !path.isAbsolute(selected) ||
      relative.startsWith("..") ||
      path.isAbsolute(relative)
    )
      throw new Error("Private material is outside the authorized trial root");
  }
  const policy = JSON.parse(
    await readFile(path.join(privateRoot, "test-account-policy.json"), "utf8"),
  );
  expect(Boolean(policy.republication_authorization_20261104)).toBe(true);
  const inventory = JSON.parse(await readFile(inventoryFile, "utf8")) as {
    accounts: Account[];
  };
  const fixture = JSON.parse(await readFile(fixtureFile, "utf8")) as Fixture;
  expect([fixture.host, fixture.port, fixture.database].join("|")).toBe(
    "127.0.0.1|15434|espacigo_f5_gate_20261101",
  );
  const owner = inventory.accounts.find((account) => account.slot === "owner");
  const renter = inventory.accounts.find((account) => account.slot !== "owner");
  if (!owner || !renter)
    throw new Error("Existing two-account inventory is required");
  const ownerContext = await browser.newContext({
    baseURL: "https://127.0.0.1:4203",
    ignoreHTTPSErrors: true,
  });
  const renterContext = await browser.newContext({
    baseURL: "https://127.0.0.1:4203",
    ignoreHTTPSErrors: true,
  });
  let externalRequest = false;
  for (const context of [ownerContext, renterContext])
    context.on("request", (request) => {
      const url = new URL(request.url());
      if (
        ["http:", "https:"].includes(url.protocol) &&
        url.origin !== "https://127.0.0.1:4203"
      )
        externalRequest = true;
    });
  const page = await ownerContext.newPage();
  const renterPage = await renterContext.newPage();
  const returnsPath = `/api/backend/local/booking-trial/spaces/${fixture.space_id}/returns`;
  const reception = `/api/backend/local/booking-trial/reservations/${fixture.reservation_id}/reception`;
  try {
    await login(page, owner, fixture.route);
    await login(renterPage, renter, "/mi-cuenta");
    const categories = await page.request.get("/api/backend/spaces/categories");
    expect(categories.status()).toBe(200);
    const catalog = (await categories.json()).items as Array<{ code: string }>;
    expect(catalog.length).toBe(9);
    expect(new Set(catalog.map((item) => item.code)).size).toBe(9);
    expect(catalog.some((item) => item.code === "inmuebles")).toBe(true);
    for (const category of catalog) {
      const response = await page.request.get(
        `/api/backend/spaces/categories/${category.code}/attributes`,
      );
      expect(response.status()).toBe(200);
      const profile = await response.json();
      expect(profile.category_code === category.code).toBe(true);
      expect(profile.schema_version > 0 && profile.attributes.length > 0).toBe(
        true,
      );
      if (category.code === "inmuebles") {
        const values = profile.attributes.flatMap(
          (attribute: { options?: string[] }) => attribute.options ?? [],
        );
        for (const kind of [
          "Casa",
          "Departamento",
          "Pieza o habitación",
          "Cabaña",
        ])
          expect(values.includes(kind)).toBe(true);
      }
    }
    expect((await renterPage.request.get(returnsPath)).status()).toBe(404);
    expect(
      (
        await renterPage.request.post(reception, {
          headers: {
            Origin: "https://127.0.0.1:4203",
            "Idempotency-Key": "native-not-owner-receipt",
          },
          data: {},
        })
      ).status(),
    ).toBe(404);
    expect(
      (
        await page.request.post(reception, {
          headers: {
            Origin: "https://127.0.0.1:4203",
            "Idempotency-Key": "native-before-checkout",
          },
          data: {},
        })
      ).status(),
    ).toBe(409);
    const checkout = await renterPage.request.post(
      `/api/backend/local/booking-trial/reservations/${fixture.reservation_id}/check-out`,
      {
        headers: {
          Origin: "https://127.0.0.1:4203",
          "Idempotency-Key": "native-existing-checkout-transition",
        },
        data: {},
      },
    );
    expect(checkout.status()).toBe(201);
    await page.reload();
    await page.getByRole("button", { name: "Archivar", exact: true }).click();
    await expect(
      page.getByText(
        "El espacio ha sido archivado (oculto del catálogo público).",
      ),
    ).toBeVisible();
    const detail = await page.request.get(
      `/api/backend/spaces/${fixture.space_id}`,
    );
    const etag = detail.headers().etag;
    if (!etag) throw new Error("Current publication revision missing");
    const forbiddenPublish = await page.request.put(
      `/api/backend/spaces/${fixture.space_id}/publication`,
      {
        headers: { Origin: "https://127.0.0.1:4203", "If-Match": etag },
        data: { state: "activa" },
      },
    );
    expect(forbiddenPublish.status()).toBe(409);
    // The user-visible action must perform the real reception POST and await
    // authoritative readback; a successful mock or local state is insufficient.
    await page
      .getByRole("checkbox", { name: /confirmo explícitamente/i })
      .check();
    const post = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === reception &&
        r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Confirmar desocupación", exact: true })
      .click();
    expect((await post).status()).toBe(201);
    const confirmed = await page.request.get(returnsPath);
    expect(confirmed.status()).toBe(200);
    const items = (await confirmed.json()).data.items as Array<{
      reservation_id: string;
      vacancy_confirmed: boolean;
    }>;
    expect(
      items.find((item) => item.reservation_id === fixture.reservation_id)
        ?.vacancy_confirmed,
    ).toBe(true);
    const publish = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname ===
          `/api/backend/spaces/${fixture.space_id}/publication` &&
        r.request().method() === "PUT",
    );
    await page.getByRole("button", { name: "Republicar", exact: true }).click();
    expect((await publish).status()).toBe(200);
    await expect(page.getByText("Publicado", { exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByText("Publicado", { exact: true })).toBeVisible();
    for (const viewport of [
      { width: 1440, height: 1000 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      const axe = await new AxeBuilder({ page }).include("main").analyze();
      expect(axe.violations.map((violation) => violation.id)).toEqual([]);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
    }
    expect(externalRequest).toBe(false);
  } finally {
    for (const context of [ownerContext, renterContext]) {
      await context.request
        .post("/api/backend/auth/logout", {
          headers: { Origin: "https://127.0.0.1:4203" },
        })
        .catch(() => undefined);
      await context.close();
    }
  }
});
