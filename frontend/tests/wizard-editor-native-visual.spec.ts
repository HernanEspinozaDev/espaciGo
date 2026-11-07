import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { chmod, mkdir, readFile, stat } from "node:fs/promises";
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
const intentPath = path.join(
  privateDir,
  "native-wizard-editor-schedule-intent-v2.json",
);
const evidenceDir =
  "/home/nandev/.hermes/cache/scratch/espacigo-wizard-evidence/native-editor-visual";

test.skip(
  !enabled,
  "Opt-in native read-only editor visual and full Axe evidence",
);
test.setTimeout(90_000);
test.use({ trace: "off", screenshot: "off", video: "off" });

type Account = { email?: unknown; password?: unknown };
type Receipt = {
  spaceId?: unknown;
  intent?: unknown;
  state?: unknown;
  observedReadback?: { monday?: { open?: unknown; close?: unknown } };
};

async function login(page: Page, account: Account): Promise<void> {
  await page.goto("/acceso", { waitUntil: "domcontentloaded" });
  try {
    const responsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login" &&
        response.request().method() === "POST",
      { timeout: 15_000 },
    );
    await page.getByLabel("Correo electrónico").fill(account.email as string);
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill(account.password as string);
    const [response] = await Promise.all([
      responsePromise,
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
    ]);
    if (response.status() !== 200) throw new Error("login rejected");
  } catch {
    await page
      .getByLabel("Correo electrónico")
      .fill("")
      .catch(() => undefined);
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("")
      .catch(() => undefined);
    throw new Error("Native visual login failed; private form values cleared");
  }
}

test("@native captures read-only persisted schedule and full-page Axe at desktop/mobile", async ({
  page,
}) => {
  const directory = await stat(privateDir);
  if ((directory.mode & 0o777) !== 0o700)
    throw new Error("Private trial directory must be mode 0700");
  const inventoryStat = await stat(inventoryPath);
  if ((inventoryStat.mode & 0o777) !== 0o600)
    throw new Error("Private account inventory must be mode 0600");
  const receipt = JSON.parse(await readFile(intentPath, "utf8")) as Receipt;
  if (
    receipt.intent !== "edit-existing-weekly-hours" ||
    !["saved-needs-reconciliation", "verified"].includes(String(receipt.state))
  )
    throw new Error("Verified saved schedule receipt required");
  if (
    typeof receipt.spaceId !== "string" ||
    !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
      receipt.spaceId,
    )
  )
    throw new Error("Receipt has no valid target ID");
  expect(receipt.observedReadback?.monday).toEqual({
    open: "09:00",
    close: "17:30",
  });
  const accounts = JSON.parse(
    await readFile(inventoryPath, "utf8"),
  ) as Account[];
  const account = accounts[0];
  if (
    typeof account?.email !== "string" ||
    typeof account.password !== "string"
  )
    throw new Error("Private verified-account inventory is incomplete");

  await mkdir(evidenceDir, { recursive: true, mode: 0o700 });
  await chmod(evidenceDir, 0o700);
  const mutations: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      url.origin !== new URL(baseURL).origin ||
      url.pathname.startsWith("/api/backend/auth/")
    )
      return;
    if (
      /^\/api\/backend\/spaces(?:\/|$)/.test(url.pathname) &&
      ["POST", "PUT", "PATCH", "DELETE"].includes(request.method())
    )
      mutations.push(`${request.method()} ${url.pathname}`);
  });

  await login(page, account);
  for (const viewport of [
    { name: "desktop", width: 1440, height: 1000 },
    { name: "mobile", width: 390, height: 844 },
  ]) {
    await page.setViewportSize({
      width: viewport.width,
      height: viewport.height,
    });
    await page.goto(`/anfitrion/espacios/${receipt.spaceId}/editar`, {
      waitUntil: "domcontentloaded",
    });
    await expect(page.locator("#editor-title")).toBeVisible();
    await page.getByRole("tab", { name: /horarios/i }).click();
    await expect(
      page.getByRole("heading", { name: "Disponibilidad semanal" }),
    ).toBeVisible();
    const calendar = page.locator("section[aria-labelledby]").filter({
      has: page.getByRole("heading", { name: "Horario semanal habitual" }),
    });
    await expect(calendar).toBeVisible();
    await expect(
      calendar.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
    ).toHaveValue("17:30");
    await page.evaluate(async () => {
      await document.fonts.ready;
    });
    const violations = (
      await new AxeBuilder({ page }).analyze()
    ).violations.map((violation) => ({
      id: violation.id,
      targets: violation.nodes.map((node) => node.target),
    }));
    expect(violations).toEqual([]);
    const overflow = await page.evaluate(() => ({
      viewport: document.documentElement.clientWidth,
      document: document.documentElement.scrollWidth,
    }));
    expect(overflow.document).toBeLessThanOrEqual(overflow.viewport);
    const imagePath = path.join(
      evidenceDir,
      `native-editor-schedule-${viewport.name}.png`,
    );
    await calendar.screenshot({ path: imagePath, animations: "disabled" });
    await chmod(imagePath, 0o600);
  }
  expect(mutations).toEqual([]);
});
