import { readFile, lstat } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const ORIGIN = "https://127.0.0.1:4219";
const FIXTURE_PATH = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../.local/testing/kyc-native/fixture.json",
);
const TIMEOUT = 15_000;
const MUTATION_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);
const DECLARATION_PATH = "/api/backend/verifications/identity-declaration";

function assert(condition, stage) {
  if (!condition) throw Object.assign(new Error(), { stage });
}

function sanitizedError(error, fallback) {
  return { stage: error?.stage ?? fallback, errorName: error?.name ?? "Error" };
}

async function secureFixture() {
  const parent = dirname(FIXTURE_PATH);
  const parentStat = await lstat(parent);
  assert(
    parentStat.isDirectory() && !parentStat.isSymbolicLink(),
    "fixture-parent",
  );
  assert((parentStat.mode & 0o777) === 0o700, "fixture-parent-mode");
  assert(parentStat.uid === process.getuid(), "fixture-parent-owner");
  const stat = await lstat(FIXTURE_PATH);
  assert(stat.isFile() && !stat.isSymbolicLink(), "fixture-file");
  assert((stat.mode & 0o777) === 0o600, "fixture-file-mode");
  assert(stat.uid === process.getuid(), "fixture-file-owner");
  return JSON.parse(await readFile(FIXTURE_PATH, "utf8"));
}

function utcDateOffset(days) {
  const date = new Date();
  date.setUTCHours(0, 0, 0, 0);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

function fixedCaseResult(name, values) {
  return {
    case: name,
    desktop: Boolean(values.desktop),
    formVisible: Boolean(values.formVisible),
    nativeInvalid: Boolean(values.nativeInvalid),
    expectedAlert: Boolean(values.expectedAlert),
    noSavedSummary: Boolean(values.noSavedSummary),
    mutationAttempts: values.mutationAttempts,
    viewport: values.viewport,
  };
}

export async function runKycNativeValidation() {
  assert(process.env.KYC_NATIVE_VALIDATION === "1", "opt-in-required");
  assert(process.argv.slice(2).length === 0, "unsupported-arguments");

  const fixture = await secureFixture();
  assert(
    typeof fixture.email === "string" && fixture.email.length > 0,
    "fixture-email",
  );
  assert(
    typeof fixture.password === "string" && fixture.password.length > 0,
    "fixture-password",
  );

  let browser;
  let context;
  let page;
  let logoutStatus = null;
  let sessionStatus = null;
  let cookieCheck = false;
  let failure = null;
  let currentStage = "launch";
  const results = [];
  const counters = {
    externalDenied: 0,
    declaration404: 0,
    mutationAttempts: 0,
  };

  try {
    browser = await chromium.launch({ headless: true });
    context = await browser.newContext({
      ignoreHTTPSErrors: true,
      serviceWorkers: "block",
      viewport: { width: 1365, height: 900 },
    });
    await context.route("**/*", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.origin !== ORIGIN) {
        counters.externalDenied += 1;
        await route.abort();
        return;
      }
      if (
        MUTATION_METHODS.has(request.method()) &&
        url.pathname.startsWith("/api/backend/verifications/")
      ) {
        counters.mutationAttempts += 1;
        await route.abort();
        return;
      }
      await route.continue();
    });
    page = await context.newPage();
    page.setDefaultTimeout(TIMEOUT);
    page.setDefaultNavigationTimeout(TIMEOUT);

    currentStage = "login-navigation";
    const loginResponse = await page.goto(
      `${ORIGIN}/acceso?return_to=%2Fmi-cuenta`,
      { waitUntil: "domcontentloaded" },
    );
    assert(loginResponse?.status() === 200, currentStage);
    await page.locator('input[type="email"]').fill(fixture.email);
    await page.locator('input[type="password"]').fill(fixture.password);
    const loginWait = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login" &&
        response.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Iniciar sesión", exact: true })
      .click();
    assert((await loginWait).status() === 200, "login-status");
    await page.waitForURL(
      (url) => url.origin === ORIGIN && url.pathname === "/mi-cuenta",
      { waitUntil: "domcontentloaded" },
    );
    currentStage = "authenticated-session";
    const sessionResponse = await page.request.get(
      `${ORIGIN}/api/backend/auth/session`,
      { timeout: TIMEOUT },
    );
    assert(sessionResponse.status() === 200, currentStage);

    const viewports = [
      { name: "desktop", width: 1365, height: 900 },
      { name: "mobile", width: 390, height: 844 },
    ];
    const cases = [
      "missing-first-name",
      "bad-dv",
      "under-18",
      "future-birth",
      "no-consent",
    ];

    for (const viewport of viewports) {
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });
      for (const name of cases) {
        currentStage = `${viewport.name}-${name}-navigation`;
        const declarationWait = page
          .waitForResponse(
            (response) =>
              response.request().method() === "GET" &&
              new URL(response.url()).pathname === DECLARATION_PATH,
            { timeout: TIMEOUT },
          )
          .catch(() => null);
        await page.goto(`${ORIGIN}/mi-cuenta/verificaciones`, {
          waitUntil: "domcontentloaded",
        });
        await page
          .getByRole("heading", {
            name: "Verificación de usuario",
            exact: false,
          })
          .waitFor({ state: "visible" });
        const first = page.locator("#id-first-name");
        await first.waitFor({ state: "visible" });
        await first.waitFor({ state: "attached" });
        await page.waitForFunction(() => {
          const field = document.querySelector("#id-first-name");
          return field instanceof HTMLInputElement && !field.disabled;
        });
        // The initial declaration GET is expected to return 404 for an unset fixture.
        const declarationResponse = await declarationWait;
        if (declarationResponse?.status() === 404) counters.declaration404 += 1;
        assert(
          declarationResponse?.status() === 404,
          `${viewport.name}-${name}-declaration-404`,
        );

        await page
          .locator("#id-first-name")
          .fill(name === "missing-first-name" ? "" : "Synthetic");
        await page.locator("#id-last-name").fill("Person");
        const dob = page.locator("#id-dob");
        await dob.fill(
          name === "under-18"
            ? utcDateOffset(-365 * 17)
            : name === "future-birth"
              ? utcDateOffset(1)
              : "1990-01-02",
        );
        const rut = page.locator("#id-rut");
        await rut.fill(name === "bad-dv" ? "12.345.678-9" : "11.222.333-9");
        const consent = page.locator('input[name="consent"]');
        if (name !== "no-consent") await consent.check();
        const submit = page.getByRole("button", {
          name: "Guardar datos de identidad",
          exact: true,
        });
        await submit.click();
        await page.evaluate(
          () => new Promise((resolve) => setTimeout(resolve, 0)),
        );

        const nativeInvalid =
          name === "missing-first-name"
            ? await page
                .locator("#id-first-name")
                .evaluate((el) => !el.validity.valid)
            : name === "future-birth"
              ? await dob.evaluate((el) => !el.validity.valid)
              : name === "no-consent"
                ? await consent.evaluate((el) => !el.validity.valid)
                : false;
        if (name === "bad-dv")
          assert(
            (await rut.getAttribute("aria-invalid")) === "true",
            `${viewport.name}-${name}-rut-invalid`,
          );
        const expectedAlert =
          name === "bad-dv" || name === "under-18"
            ? await page
                .getByRole("alert")
                .filter({ hasText: "Revisa el RUN/RUT" })
                .waitFor({ state: "visible" })
                .then(() => true)
                .catch(() => false)
            : false;
        if (name === "bad-dv" || name === "under-18")
          assert(expectedAlert, `${viewport.name}-${name}-alert`);
        const noSavedSummary =
          (await page.getByText(/^Datos guardados \(revisión #/).count()) === 0;
        assert(
          name === "bad-dv" || name === "under-18"
            ? !nativeInvalid
            : nativeInvalid,
          `${viewport.name}-${name}-validity`,
        );
        assert(noSavedSummary, `${viewport.name}-${name}-no-save`);
        assert(
          await first.isVisible(),
          `${viewport.name}-${name}-form-visible`,
        );
        assert(
          counters.mutationAttempts === 0,
          `${viewport.name}-${name}-zero-mutations`,
        );
        results.push(
          fixedCaseResult(name, {
            desktop: viewport.name === "desktop",
            formVisible: await first.isVisible(),
            nativeInvalid,
            expectedAlert,
            noSavedSummary,
            mutationAttempts: counters.mutationAttempts,
            viewport: `${viewport.width}x${viewport.height}`,
          }),
        );
      }
    }
  } catch (error) {
    failure = sanitizedError(error, currentStage);
  } finally {
    if (page && context) {
      try {
        currentStage = "logout";
        const response = await page.request.post(
          `${ORIGIN}/api/backend/auth/logout`,
          {
            timeout: TIMEOUT,
            headers: { Origin: ORIGIN },
          },
        );
        logoutStatus = response.status();
        assert(logoutStatus === 204, currentStage);
        const session = await page.request.get(
          `${ORIGIN}/api/backend/auth/session`,
          {
            timeout: TIMEOUT,
          },
        );
        sessionStatus = session.status();
        assert(sessionStatus === 401, "logout-session-check");
        const cookies = await context.cookies(ORIGIN);
        cookieCheck = !cookies.some(
          ({ name }) =>
            name === "__Host-espacigo-session" ||
            name === "__Host-espacigo-session-expiry",
        );
        assert(cookieCheck, "logout-cookie-check");
      } catch (error) {
        failure ??= sanitizedError(error, currentStage);
      }
    }
    await context?.close().catch(() => {
      failure ??= { stage: "context-close", errorName: "Error" };
    });
    await browser?.close().catch(() => {
      failure ??= { stage: "browser-close", errorName: "Error" };
    });
  }

  const report = {
    ok:
      !failure &&
      results.length === 10 &&
      counters.externalDenied === 0 &&
      counters.mutationAttempts === 0,
    cases: results,
    counters,
    logout: { status: logoutStatus, sessionStatus, cookieCheck },
    ...(failure ? { failure } : {}),
  };
  console.log(JSON.stringify(report));
  if (!report.ok) process.exitCode = 1;
  return report;
}

if (process.env.KYC_NATIVE_VALIDATION === "1") {
  runKycNativeValidation().catch((error) => {
    console.log(
      JSON.stringify({ ok: false, failure: sanitizedError(error, "runner") }),
    );
    process.exitCode = 1;
  });
} else if (process.argv.slice(2).length > 0) {
  console.error("KYC native validation: unsupported arguments");
  process.exitCode = 2;
} else {
  console.error("KYC native validation: explicit opt-in required");
  process.exitCode = 2;
}
