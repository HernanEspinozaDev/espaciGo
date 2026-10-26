const http = require("node:http");
const fs = require("node:fs");
const path = require("node:path");
const assert = require("node:assert/strict");
const { createRequire } = require("node:module");
const fromFrontend = createRequire(
  path.resolve(__dirname, "../../../frontend/package.json"),
);
const { chromium } = fromFrontend("@playwright/test");
const axePackage = fromFrontend("@axe-core/playwright");
const AxeBuilder = axePackage.default || axePackage.AxeBuilder || axePackage;
const ROOT = path.resolve(__dirname, "../../..");
const OUT = path.join(ROOT, ".local/testing/ui-ux-fase1");
const FIXTURE_ID = "7b2f694e-9d8a-4d92-bfa3-e3a1f81d5a77";
const FIXTURE_NAME = "Camila Demo";

async function main() {
  let server;
  let browser;
  let target = process.env.PROTOTYPE_URL;
  if (target) {
    const parsed = new URL(target);
    assert(
      ["http:", "https:"].includes(parsed.protocol) &&
        parsed.hostname === "127.0.0.1" &&
        !parsed.username &&
        !parsed.password,
      "PROTOTYPE_URL must be an explicit credential-free 127.0.0.1 HTTP(S) URL",
    );
  } else {
    server = http.createServer((req, res) => {
      if (!["/", "/fase1-prototipo.html"].includes(req.url)) {
        res.writeHead(404);
        res.end();
        return;
      }
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(fs.readFileSync(path.join(__dirname, "fase1-prototipo.html")));
    });
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    target = `http://127.0.0.1:${server.address().port}/fase1-prototipo.html`;
  }
  const origin = new URL(target).origin;
  const results = {
    target,
    simulatedOnly: true,
    tests: [],
    viewports: [],
    passed: 0,
    failed: 0,
    skipped: 0,
  };
  fs.mkdirSync(OUT, { recursive: true, mode: 0o700 });
  console.log(`Local prototype only: ${target}`);
  async function check(name, fn) {
    try {
      await fn();
      results.tests.push({ name, status: "PASS" });
      results.passed++;
      console.log(`[PASS] ${name}`);
    } catch (error) {
      results.tests.push({ name, status: "FAIL", detail: error.message });
      results.failed++;
      console.log(`[FAIL] ${name}: ${error.message}`);
    }
  }
  try {
    browser = await chromium.launch({ headless: true });
    for (const vp of [
      { width: 1365, height: 900, name: "desktop" },
      { width: 390, height: 844, name: "mobile" },
    ]) {
      results.viewports.push(vp);
      const context = await browser.newContext({
        viewport: { width: vp.width, height: vp.height },
      });
      const page = await context.newPage();
      const errors = [];
      const blocked = [];
      page.setDefaultTimeout(8000);
      page.on("console", (msg) => {
        if (msg.type() === "error") errors.push(msg.text());
      });
      page.on("pageerror", (error) => errors.push(error.message));
      await page.route("**/*", (route) => {
        const req = route.request();
        if (
          new URL(req.url()).origin !== origin ||
          ["xhr", "fetch"].includes(req.resourceType())
        ) {
          blocked.push(req.url());
          return route.abort();
        }
        return route.continue();
      });
      const test = (name, fn) => check(`${vp.name}: ${name}`, fn);
      async function audit(state) {
        await test(`${state} / overflow`, async () => {
          assert(
            await page.evaluate(
              () => document.documentElement.scrollWidth <= innerWidth,
            ),
            "viewport overflow",
          );
        });
        await test(`${state} / objetivos táctiles 44px`, async () => {
          const small = await page
            .locator("button:visible")
            .evaluateAll((nodes) =>
              nodes
                .filter((node) => !node.disabled)
                .filter((node) => {
                  const rect = node.getBoundingClientRect();
                  return rect.width < 43.9 || rect.height < 43.9;
                })
                .map((node) => node.id || node.textContent.trim()),
            );
          assert.deepEqual(small, []);
        });
        await test(`${state} / Axe incluyendo contraste`, async () => {
          const axe = await new AxeBuilder({ page }).analyze();
          assert.equal(
            axe.violations.length,
            0,
            JSON.stringify(
              axe.violations.map((v) => ({
                id: v.id,
                nodes: v.nodes.map((n) => n.target),
              })),
            ),
          );
        });
      }
      await page.goto(target, { waitUntil: "load" });
      await test("aviso sintético persistente", async () => {
        assert.match(
          await page.locator(".prototype-banner").innerText(),
          /datos sintéticos.*sin conexión a la API/,
        );
      });
      await test("cabecera invitada", async () => {
        assert(await page.locator("#btn-header-login").isVisible());
      });
      await audit("inicio invitado");
      await page.click("#btn-header-login");
      await test("acceso con logo real y sin recoger contraseña", async () => {
        assert(await page.locator("#view-acceso").isVisible());
        assert(await page.locator("#input-mock-password").isDisabled());
        assert(
          (await page.locator("#view-acceso svg.brand-logo-svg path").count()) >
            0,
        );
      });
      await audit("acceso");
      await page.screenshot({
        path: path.join(OUT, `${vp.name}-acceso.png`),
        fullPage: true,
      });
      await page.click("#btn-do-login");
      await test("cuenta usa UUID fixture, sin acceso redundante", async () => {
        assert.equal(
          await page.locator("#header-user-display").innerText(),
          FIXTURE_ID,
        );
        assert.equal(
          await page.locator("#greeting-display-name").innerText(),
          FIXTURE_ID,
        );
        assert(await page.locator("#btn-header-logout").isVisible());
        assert.equal(
          await page
            .locator("header")
            .getByRole("link", { name: "Acceso", exact: true })
            .count(),
          0,
        );
      });
      await audit("cuenta sin nombre");
      await page.screenshot({
        path: path.join(OUT, `${vp.name}-cuenta-id.png`),
        fullPage: true,
      });
      await page.fill("#input-display-name", `  ${FIXTURE_NAME}  `);
      await page.click("#btn-save-profile");
      await test("nombre simulado se actualiza sin reload", async () => {
        assert.equal(
          await page.locator("#header-user-display").innerText(),
          FIXTURE_NAME,
        );
        assert.equal(
          await page.locator("#greeting-display-name").innerText(),
          FIXTURE_NAME,
        );
      });
      await audit("cuenta con nombre");
      await page.screenshot({
        path: path.join(OUT, `${vp.name}-cuenta-nombre.png`),
        fullPage: true,
      });
      await page.click("#brand-logo-btn");
      await test("logo vuelve al inicio manteniendo estado simulado", async () => {
        assert(await page.locator("#view-home").isVisible());
        assert(await page.locator("#btn-header-myaccount").isVisible());
        assert.equal(await page.locator("#btn-header-login").count(), 0);
      });
      await audit("inicio autenticado");
      await page.click("#btn-header-myaccount");
      await test("Mi cuenta directo sin nueva pantalla de acceso", async () =>
        assert(await page.locator("#view-cuenta").isVisible()));
      for (const key of ["seguridad", "privacidad", "verificaciones"]) {
        await page.click(`#tab-${key}`);
        await test(`${key} no presenta funciones ficticias como reales`, async () =>
          assert.match(
            await page.locator(`#panel-${key}`).innerText(),
            /Sección no implementada/,
          ));
      }
      await page.click("#tab-general");
      await page.click("#btn-header-logout");
      await test("logout simulado purga identidad incluso de formulario oculto", async () => {
        assert(await page.locator("#btn-header-login").isVisible());
        assert.equal(await page.locator("#header-user-display").count(), 0);
        assert.equal(
          await page.locator("#input-display-name").inputValue(),
          "",
        );
      });
      await page.click("#sc-checking");
      await test("comprobando no ofrece acceso ni identidad confirmada", async () => {
        assert(await page.locator("#checking-status-indicator").isVisible());
        assert.equal(await page.locator("#btn-header-login").count(), 0);
        assert.equal(await page.locator("#btn-header-myaccount").count(), 0);
      });
      await audit("comprobando");
      await page.click("#sc-expired");
      await test("caducidad simulada vuelve a invitado", async () => {
        assert(await page.locator("#btn-header-login").isVisible());
        assert.match(
          await page.locator("#status-live-region").innerText(),
          /expirado tras 8 horas/,
        );
      });
      await audit("vencida");
      await page.click("#sc-error");
      await test("error 503 explícito y simulado", async () =>
        assert.match(
          await page.locator("#status-live-region").innerText(),
          /503/,
        ));
      await audit("error");
      await test("sin cookies ni almacenamiento navegador", async () => {
        assert.equal((await context.cookies()).length, 0);
        assert(
          await page.evaluate(
            () =>
              localStorage.length === 0 &&
              sessionStorage.length === 0 &&
              document.cookie === "",
          ),
        );
      });
      await test("sin red externa, API ni errores durante todo el flujo", async () => {
        assert.deepEqual(blocked, []);
        assert.deepEqual(errors, []);
      });
      await context.close();
    }
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise((resolve) => server.close(resolve));
    fs.writeFileSync(
      path.join(OUT, "report.json"),
      JSON.stringify(results, null, 2) + "\n",
      { mode: 0o600 },
    );
  }
  console.log(
    `RESULT ${JSON.stringify({ passed: results.passed, failed: results.failed, skipped: results.skipped, report: path.join(OUT, "report.json") })}`,
  );
  process.exitCode = results.failed ? 1 : 0;
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
