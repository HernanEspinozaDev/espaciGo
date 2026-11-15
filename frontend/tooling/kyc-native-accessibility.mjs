import { lstat, mkdir, open, readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const ORIGIN = "https://127.0.0.1:4219";
const PRIVATE_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../.local/testing/kyc-native",
);
const FIXTURE_PATH = resolve(PRIVATE_DIR, "fixture.json");
const VISUAL_DIR = resolve(PRIVATE_DIR, "visual");
const TIMEOUT = 15_000;
const MUTATION_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);
const DECLARATION_PATH = "/api/backend/verifications/identity-declaration";
const VIEWPORTS = [
  { name: "desktop", width: 1365, height: 900 },
  { name: "mobile", width: 390, height: 844 },
];

function assert(ok, stage) {
  if (!ok) throw Object.assign(new Error(), { stage });
}

function safeError(error, fallback) {
  return { stage: error?.stage ?? fallback, errorName: error?.name ?? "Error" };
}

async function ownPrivatePath(path, mode, label) {
  const stat = await lstat(path);
  assert(stat.isDirectory() && !stat.isSymbolicLink(), `${label}-type`);
  assert(stat.uid === process.getuid(), `${label}-owner`);
  assert((stat.mode & 0o777) === mode, `${label}-mode`);
}

async function loadFixture() {
  await ownPrivatePath(PRIVATE_DIR, 0o700, "fixture-parent");
  const stat = await lstat(FIXTURE_PATH);
  assert(stat.isFile() && !stat.isSymbolicLink(), "fixture-file-type");
  assert(stat.uid === process.getuid(), "fixture-file-owner");
  assert((stat.mode & 0o777) === 0o600, "fixture-file-mode");
  const data = JSON.parse(await readFile(FIXTURE_PATH, "utf8"));
  assert(
    typeof data.email === "string" && data.email.length > 0,
    "fixture-email",
  );
  assert(
    typeof data.password === "string" && data.password.length > 0,
    "fixture-password",
  );
  return data;
}

function forbiddenCopy(text) {
  const normalized = text.normalize("NFKC").toLocaleLowerCase("es");
  // These are concrete misleading assertions, not generic mentions/disclaimers.
  return (
    /verificaci[oó]n\s+(?:kyb|empresarial|de empresa)/i.test(normalized) ||
    /(?:sin|no tener|carecer de)\s+(?:esta\s+)?verificaci[oó]n[^.]{0,100}(?:no podr[aá]s|impide|bloquea)[^.]{0,60}(?:reservar|pagar)/i.test(
      normalized,
    ) ||
    /verificaci[oó]n[^.]{0,100}(?:desbloquea|habilita|permite)[^.]{0,60}(?:reservas|pagos)/i.test(
      normalized,
    )
  );
}

async function saveScreenshot(page, target, fixture) {
  await ownPrivatePath(PRIVATE_DIR, 0o700, "visual-parent");
  try {
    await mkdir(VISUAL_DIR, { mode: 0o700 });
  } catch (error) {
    if (error?.code !== "EEXIST") throw error;
  }
  await ownPrivatePath(VISUAL_DIR, 0o700, "visual-directory");
  const form = page
    .locator("form")
    .filter({ has: page.locator("#id-first-name") })
    .first();
  const safeInputs = await form
    .locator("input")
    .evaluateAll(
      (inputs, secrets) =>
        inputs.every(
          (input) =>
            !(input instanceof HTMLInputElement) ||
            (input.type !== "password" &&
              input.type !== "email" &&
              !secrets.some(
                (secret) => secret && input.value.includes(secret),
              )),
        ),
      [fixture.email, fixture.password],
    );
  assert(safeInputs, "screenshot-content-policy");
  const formText = await form.innerText();
  assert(
    !formText.includes(fixture.email) && !formText.includes(fixture.password),
    "screenshot-text-policy",
  );
  const buffer = await form.screenshot({
    animations: "disabled",
    timeout: TIMEOUT,
  });
  let handle;
  try {
    handle = await open(target, "wx", 0o600);
    await handle.writeFile(buffer);
  } finally {
    await handle?.close();
  }
}

export async function runKycNativeAccessibility() {
  assert(process.env.KYC_NATIVE_ACCESSIBILITY === "1", "opt-in-required");
  assert(process.argv.slice(2).length === 0, "unsupported-arguments");
  const fixture = await loadFixture();
  const report = {
    ok: false,
    viewports: [],
    counters: { externalDenied: 0, mutationAttempts: 0 },
    logout: { status: null, sessionStatus: null, cookiesAbsent: false },
    screenshots: [],
  };
  let browser;
  let context;
  let page;
  let failure = null;
  let stage = "launch";
  let loginSucceeded = false;

  try {
    browser = await chromium.launch({ headless: true });
    context = await browser.newContext({
      ignoreHTTPSErrors: true,
      serviceWorkers: "block",
    });
    await context.route("**/*", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.origin !== ORIGIN) {
        report.counters.externalDenied += 1;
        await route.abort();
        return;
      }
      if (
        MUTATION_METHODS.has(request.method()) &&
        url.pathname.startsWith("/api/backend/verifications/")
      ) {
        report.counters.mutationAttempts += 1;
        await route.abort();
        return;
      }
      await route.continue();
    });
    page = await context.newPage();
    page.setDefaultTimeout(TIMEOUT);
    page.setDefaultNavigationTimeout(TIMEOUT);
    stage = "login-page";
    const loginPage = await page.goto(
      `${ORIGIN}/acceso?return_to=%2Fmi-cuenta`,
      { waitUntil: "domcontentloaded" },
    );
    assert(loginPage?.status() === 200, stage);
    await page.locator('input[name="email"]').fill(fixture.email);
    await page.locator('input[name="password"]').fill(fixture.password);
    const loginWait = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login" &&
        response.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Iniciar sesión", exact: true })
      .click();
    const login = await loginWait;
    assert(login.status() === 200, "login-status");
    loginSucceeded = true;
    stage = "session-status";
    const session = await page.request.get(
      `${ORIGIN}/api/backend/auth/session`,
      { timeout: TIMEOUT },
    );
    assert(session.status() === 200, stage);

    for (const viewport of VIEWPORTS) {
      stage = `${viewport.name}-viewport`;
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });
      const declarationWait = page.waitForResponse(
        (response) =>
          response.request().method() === "GET" &&
          new URL(response.url()).pathname === DECLARATION_PATH,
      );
      await page.goto(`${ORIGIN}/mi-cuenta/verificaciones`, {
        waitUntil: "domcontentloaded",
      });
      await page
        .getByRole("heading", { name: /Verificación de usuario/ })
        .first()
        .waitFor({ state: "visible" });
      const first = page.locator("#id-first-name");
      await first.waitFor({ state: "visible" });
      await page.waitForFunction(() => {
        const input = document.querySelector("#id-first-name");
        return input instanceof HTMLInputElement && !input.disabled;
      });
      const declaration = await declarationWait;
      assert(declaration.status() === 404, `${viewport.name}-declaration-404`);
      await first.fill("Synthetic");
      await page.locator("#id-last-name").fill("Person");
      await page.locator("#id-dob").fill("1990-01-02");
      await page.locator("#id-rut").fill("11.222.333-9");
      assert(
        !(await page.locator('input[name="consent"]').isChecked()),
        `${viewport.name}-consent-unselected`,
      );
      assert(
        (await page.locator('input[name="didit_consent"]').count()) === 0 ||
          !(await page.locator('input[name="didit_consent"]').isChecked()),
        `${viewport.name}-didit-consent-unselected`,
      );

      stage = `${viewport.name}-axe`;
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      const axe = await new AxeBuilder({ page }).analyze();
      const violations = axe.violations.map(({ id, nodes }) => ({
        id,
        count: nodes.length,
      }));
      const dimensions = await page.evaluate(() => ({
        viewportWidth: document.documentElement.clientWidth,
        documentWidth: document.documentElement.scrollWidth,
      }));
      const copy = await page.locator("main").innerText();
      const headings = await page.getByRole("heading").allTextContents();
      const copyViolation =
        forbiddenCopy(copy) ||
        headings.some((text) => /\b(KYC|KYB)\b/i.test(text));
      const overflow = dimensions.documentWidth > dimensions.viewportWidth;
      const item = {
        name: viewport.name,
        dimensions: `${viewport.width}x${viewport.height}`,
        documentWidth: dimensions.documentWidth,
        viewportWidth: dimensions.viewportWidth,
        overflow,
        copyViolation,
        violationCount: violations.reduce(
          (total, violation) => total + violation.count,
          0,
        ),
        violations,
      };
      report.viewports.push(item);
      assert(!overflow, `${viewport.name}-horizontal-overflow`);
      assert(!copyViolation, `${viewport.name}-copy-policy`);
      assert(violations.length === 0, `${viewport.name}-axe-violations`);
      assert(
        report.counters.mutationAttempts === 0,
        `${viewport.name}-mutation-attempt`,
      );
      assert(
        report.counters.externalDenied === 0,
        `${viewport.name}-external-request`,
      );

      stage = `${viewport.name}-screenshot`;
      const basename = `${viewport.name}-form.png`;
      await saveScreenshot(page, resolve(VISUAL_DIR, basename), fixture);
      report.screenshots.push(basename);
    }
  } catch (error) {
    failure = safeError(error, stage);
  } finally {
    if (page && context && loginSucceeded) {
      try {
        stage = "logout";
        const logout = await page.request.post(
          `${ORIGIN}/api/backend/auth/logout`,
          {
            timeout: TIMEOUT,
            headers: { Origin: ORIGIN },
          },
        );
        report.logout.status = logout.status();
        assert(logout.status() === 204, "logout-status");
        const session = await page.request.get(
          `${ORIGIN}/api/backend/auth/session`,
          { timeout: TIMEOUT },
        );
        report.logout.sessionStatus = session.status();
        assert(session.status() === 401, "logout-session");
        const cookies = await context.cookies(ORIGIN);
        report.logout.cookiesAbsent = !cookies.some(
          ({ name }) =>
            name === "__Host-espacigo-session" ||
            name === "__Host-espacigo-session-expiry",
        );
        assert(report.logout.cookiesAbsent, "logout-cookies");
      } catch (error) {
        failure ??= safeError(error, stage);
      }
    }
    await context?.close().catch(() => {
      failure ??= { stage: "context-close", errorName: "Error" };
    });
    await browser?.close().catch(() => {
      failure ??= { stage: "browser-close", errorName: "Error" };
    });
  }
  report.ok =
    !failure &&
    report.viewports.length === VIEWPORTS.length &&
    report.counters.externalDenied === 0 &&
    report.counters.mutationAttempts === 0 &&
    report.logout.status === 204 &&
    report.logout.sessionStatus === 401 &&
    report.logout.cookiesAbsent;
  if (failure) report.failure = failure;
  console.log(JSON.stringify(report));
  if (!report.ok) process.exitCode = 1;
  return report;
}

if (process.env.KYC_NATIVE_ACCESSIBILITY === "1") {
  runKycNativeAccessibility().catch((error) => {
    console.log(
      JSON.stringify({ ok: false, failure: safeError(error, "runner") }),
    );
    process.exitCode = 1;
  });
} else if (process.argv.slice(2).length !== 0) {
  console.error("KYC native accessibility: unsupported arguments");
  process.exitCode = 2;
} else {
  console.error("KYC native accessibility: explicit opt-in required");
  process.exitCode = 2;
}
