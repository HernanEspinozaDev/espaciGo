import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";
import {
  startSessionUpstream,
  startSessionTlsProxy,
} from "./bff-test-servers.mjs";

const root = fileURLToPath(new URL("../", import.meta.url));
const out = path.resolve(root, "../.local/testing/ui-auth-navbar");
const nextPort = 4343;
const upstreamPort = 18086;
const tlsPort = 18444;
const results = [];
const children = [];
const logs = [];
const sensitive = [];
const networkSurfaces = [];
const browserErrors = [];
let fixture, edge, context;

fs.mkdirSync(out, { recursive: true, mode: 0o700 });
fs.chmodSync(out, 0o700);

const check = (ok, text) => assert.ok(ok, `SAFE: ${text}`);

async function run(name, task) {
  try {
    await task();
    results.push({ name, passed: true });
    console.log(`PASS ${name}`);
  } catch (err) {
    let reason = err.message?.startsWith("SAFE:")
      ? err.message
      : `SAFE: Assertion or element missing (${err.name || "Error"})`;
    for (const secret of sensitive) {
      if (secret && typeof secret === "string") {
        reason = reason.replaceAll(secret, "[REDACTED]");
      }
    }
    results.push({
      name,
      passed: false,
      reason,
    });
    console.log(`FAIL ${name} -> ${reason}`);
  }
}

async function ready(port) {
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${port}/api/health`);
      if (r.ok) return;
    } catch {
      /* starting */
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("SAFE: owned Next process did not become ready");
}

async function startNext(upstream) {
  check(
    fs.existsSync(path.join(root, ".next/standalone/server.js")),
    "build standalone first",
  );
  fs.cpSync(
    path.join(root, "public"),
    path.join(root, ".next/standalone/public"),
    { recursive: true },
  );
  fs.cpSync(
    path.join(root, ".next/static"),
    path.join(root, ".next/standalone/.next/static"),
    { recursive: true },
  );

  await new Promise((resolve, reject) => {
    const reservation = net.createServer();
    reservation.once("error", reject);
    reservation.listen(nextPort, "127.0.0.1", () => reservation.close(resolve));
  });

  const nodeBin = "/home/nandev/.nvm/versions/node/v24.21.0/bin";
  const env = {
    PATH: `${nodeBin}:${process.env.PATH || ""}`,
    HOME: process.env.HOME,
    TMPDIR: process.env.TMPDIR,
    NODE_ENV: "production",
    HOSTNAME: "127.0.0.1",
    PORT: String(nextPort),
    API_BACKEND_BASE_URL: upstream,
  };
  const child = spawn(
    process.execPath,
    [path.join(root, ".next/standalone/server.js")],
    { cwd: root, env, stdio: ["ignore", "pipe", "pipe"] },
  );
  children.push(child);
  child.stdout.on("data", (chunk) => logs.push(chunk.toString()));
  child.stderr.on("data", (chunk) => logs.push(chunk.toString()));
  await ready(nextPort);
}

async function main() {
  const email = "bff-test@example.invalid";
  const password = "SyntheticBff!123";
  sensitive.push(email, password);

  fixture = await startSessionUpstream(upstreamPort);
  await startNext(fixture.url);

  const certFile =
    process.env.SESSION_TLS_CERT ||
    "/home/nandev/.hermes/cache/scratch/espacigo-ui-auth-tls-b30qqa5q/cert.pem";
  const keyFile =
    process.env.SESSION_TLS_KEY ||
    "/home/nandev/.hermes/cache/scratch/espacigo-ui-auth-tls-b30qqa5q/key.pem";

  check(
    fs.existsSync(certFile) && fs.existsSync(keyFile),
    "private localhost TLS files are required",
  );

  edge = await startSessionTlsProxy({
    port: tlsPort,
    targetPort: nextPort,
    certFile,
    keyFile,
  });

  const scratchBase =
    process.env.TMPDIR || "/home/nandev/.hermes/cache/scratch";
  const profile = fs.mkdtempSync(path.join(scratchBase, "ui-navbar-browser-"));
  fs.chmodSync(profile, 0o700);

  const launch = async () => {
    const ctx = await chromium.launchPersistentContext(profile, {
      headless: true,
      ignoreHTTPSErrors: true,
      viewport: { width: 1365, height: 900 },
    });
    await ctx.route("**/*", (route) =>
      new URL(route.request().url()).origin === edge.url
        ? route.continue()
        : route.abort(),
    );
    ctx.setDefaultTimeout(6000);
    ctx.setDefaultNavigationTimeout(10000);
    for (const existing of ctx.pages()) {
      existing.on("console", (msg) => {
        if (msg.type() === "error") browserErrors.push(msg.text());
      });
    }
    ctx.on("page", (p) =>
      p.on("console", (msg) => {
        if (msg.type() === "error") browserErrors.push(msg.text());
      }),
    );
    ctx.on("request", (req) => {
      networkSurfaces.push(req.url());
    });
    ctx.on("response", (response) => {
      if (
        /javascript|json|text\/html/.test(
          response.headers()["content-type"] || "",
        )
      ) {
        response
          .text()
          .then((text) => networkSurfaces.push(text))
          .catch(() => {});
      }
    });
    return ctx;
  };

  context = await launch();
  let page = context.pages()[0] || (await context.newPage());

  const loginViaUI = async () => {
    await page.goto(
      `${edge.url}/acceso?return_to=${encodeURIComponent("/mi-cuenta")}`,
    );
    await page.getByLabel("Correo electrónico").fill(email);
    await page.getByLabel("Contraseña", { exact: true }).fill(password);
    const responsePromise = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === "/api/backend/auth/login" &&
        r.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    const response = await responsePromise;
    check(response.status() === 200, "login must succeed");
    await page.waitForURL((url) => url.pathname === "/mi-cuenta");
    await page.locator("#main-content").waitFor();
  };

  // 1. Después de login en MiCuenta: navbar muestra nombre o ID + Cerrar sesión, sin Acceso
  await run(
    "después de login en MiCuenta navbar muestra nombre o ID + Cerrar sesión, sin Acceso",
    async () => {
      await loginViaUI();
      const header = page.locator("header").first();
      await header.waitFor();
      const logoutBtn = header.getByRole("button", { name: /Cerrar sesión/i });
      await logoutBtn.waitFor({ state: "visible" });
      const navText = await header.innerText();
      check(
        !/\bAcceso\b/i.test(navText),
        "navbar de MiCuenta autenticado no debe mostrar Acceso",
      );
      check(
        navText.includes("Cuenta sintética") || navText.includes("770e8400"),
        "navbar de MiCuenta debe mostrar nombre de perfil o ID de cuenta",
      );
      check(
        navText.includes("Cerrar sesión"),
        "navbar de MiCuenta debe mostrar botón o enlace de Cerrar sesión",
      );
    },
  );

  // 2. Volver a Inicio público: navbar muestra nombre o ID + MiCuenta, sin Ingresar
  await run(
    "volver a Inicio público muestra nombre o ID + MiCuenta, sin Ingresar",
    async () => {
      await page.goto(`${edge.url}/`);
      const header = page.locator("header").first();
      await header.waitFor();
      const miCuentaLink = header.getByRole("link", { name: /Mi cuenta/i });
      await miCuentaLink.waitFor({ state: "visible" });
      const navText = await header.innerText();
      check(
        !/Ingresar/i.test(navText),
        "navbar público no debe mostrar Ingresar para usuario con sesión activa",
      );
      check(
        navText.includes("Cuenta sintética") || navText.includes("770e8400"),
        "navbar público debe mostrar nombre o ID del usuario autenticado",
      );
      check(
        await miCuentaLink.isVisible(),
        "navbar público debe mostrar enlace visible a Mi cuenta",
      );
    },
  );

  // 3. Clic en MiCuenta desde navbar público navega directo sin pedir login
  await run(
    "clic en MiCuenta desde navbar público navega directo sin pedir login",
    async () => {
      const header = page.locator("header").first();
      const miCuentaLink = header.getByRole("link", { name: /Mi cuenta/i });
      await miCuentaLink.waitFor({ state: "visible" });
      check(
        await miCuentaLink.isVisible(),
        "enlace Mi cuenta debe existir en el navbar",
      );
      await miCuentaLink.click();
      await page.waitForURL((url) => url.pathname === "/mi-cuenta");
      check(
        page.url().includes("/mi-cuenta"),
        "debe navegar directamente a /mi-cuenta",
      );
      await page.getByLabel("Nombre visible").waitFor({ timeout: 4000 });
    },
  );

  // 4. Hard reload con profile 404 hace fallback a ID sin desautenticar
  await run(
    "hard reload con profile 404 hace fallback a ID sin desautenticar",
    async () => {
      fixture.state.profile404 = true;
      try {
        await page.reload();
        const header = page.locator("header").first();
        await header.waitFor();
        const logoutBtn = header.getByRole("button", {
          name: /Cerrar sesión/i,
        });
        await logoutBtn.waitFor({ state: "visible" });
        await header
          .getByText("770e8400", { exact: false })
          .waitFor({ state: "visible" });
        const navText = await header.innerText();
        check(
          !/\bAcceso\b/i.test(navText),
          "navbar con profile 404 no debe caer a estado de invitado (Acceso)",
        );
        check(
          navText.includes("770e8400"),
          "navbar con profile 404 debe mostrar fallback con ID de cuenta",
        );
        check(
          navText.includes("Cerrar sesión"),
          "navbar con profile 404 debe conservar control de Cerrar sesión",
        );
      } finally {
        fixture.state.profile404 = false;
      }
    },
  );

  // 5. Guardar perfil actualiza reactivamente el nombre en navbar sin recargar
  await run(
    "guardar perfil actualiza reactivamente el nombre en navbar sin recargar",
    async () => {
      await page.goto(`${edge.url}/mi-cuenta`);
      const nameInput = page.getByLabel("Nombre visible");
      await nameInput.waitFor();
      await nameInput.fill("Nombre Reactivo Actualizado");
      const saveBtn = page.getByRole("button", { name: /Guardar cambios/i });
      await saveBtn.click();
      await page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === "/api/backend/profile" &&
          r.request().method() === "PUT" &&
          r.status() === 200,
      );
      const header = page.locator("header").first();
      await header
        .getByText("Nombre Reactivo Actualizado", { exact: false })
        .waitFor({ state: "visible" });
      const navText = await header.innerText();
      check(
        navText.includes("Nombre Reactivo Actualizado"),
        "navbar debe reflejar reactivamente el nuevo nombre sin recargar la página",
      );
    },
  );

  // 6. Logout transiciona navbar a estado invitado (Acceso / Ingresar)
  await run("logout transiciona navbar a estado invitado", async () => {
    const header = page.locator("header").first();
    const logoutBtn = header.getByRole("button", { name: /Cerrar sesión/i });
    check(
      await logoutBtn.isVisible(),
      "botón Cerrar sesión debe estar visible en el navbar",
    );
    await logoutBtn.click();
    await page.waitForFunction(
      () =>
        !document.querySelector("header")?.innerText.includes("Cerrar sesión"),
      { timeout: 6000 },
    );
    const guestNavText = await page.locator("header").first().innerText();
    check(
      /\bAcceso\b|Ingresar/i.test(guestNavText),
      "navbar tras logout debe mostrar Acceso o Ingresar",
    );
    check(
      !guestNavText.includes("Cuenta sintética") &&
        !guestNavText.includes("Nombre Reactivo Actualizado") &&
        !guestNavText.includes("770e8400"),
      "navbar tras logout no debe retener identidad anterior",
    );
  });

  // 7. Purga entre 2 pestañas sincroniza estado invitado sin credenciales residuales
  await run(
    "purga entre 2 pestañas sincroniza estado invitado sin credenciales residuales",
    async () => {
      // Volver a loguear en pestaña principal
      await loginViaUI();
      const tab2 = await context.newPage();
      await tab2.goto(`${edge.url}/mi-cuenta`);
      await tab2.locator("#main-content").waitFor();

      // Logout en pestaña 1
      const header1 = page.locator("header").first();
      const logoutBtn1 = header1.getByRole("button", {
        name: /Cerrar sesión/i,
      });
      await logoutBtn1.waitFor({ state: "visible" });
      check(
        await logoutBtn1.isVisible(),
        "botón Cerrar sesión visible en pestaña 1",
      );
      await logoutBtn1.click();
      await page.waitForFunction(
        () =>
          !document
            .querySelector("header")
            ?.innerText.includes("Cerrar sesión"),
        { timeout: 6000 },
      );

      // En pestaña 2, al enfocar o recargar, debe purgarse la sesión
      await tab2.bringToFront();
      await tab2.evaluate(() => window.dispatchEvent(new Event("focus")));
      await tab2.waitForFunction(
        () =>
          !document
            .querySelector("header")
            ?.innerText.includes("Cerrar sesión"),
        { timeout: 6000 },
      );
      const tab2NavText = await tab2.locator("header").first().innerText();
      check(
        !tab2NavText.includes("Cuenta sintética") &&
          !tab2NavText.includes("Nombre Reactivo Actualizado") &&
          !tab2NavText.includes("770e8400"),
        "pestaña 2 debe haber purgado nombre e ID tras logout en pestaña 1",
      );
      await tab2.close();
    },
  );

  // 8. SSR/checking: no parpadea CTA invitado con GET 200 retardado y confirma sesión
  await run(
    "SSR/checking: no parpadea CTA invitado con GET 200 retardado y confirma sesión",
    async () => {
      await loginViaUI();
      fixture.state.sessionDelayMs = 400;
      try {
        await page.goto(`${edge.url}/`);
        const header = page.locator("header").first();
        await header.waitFor();

        // Mientras sesión está demorada, estado discreto de checking sin Ingresar ni Acceso
        const interimText = await header.innerText();
        check(
          !/Ingresar/i.test(interimText),
          "mientras sesión está en verificación no debe parpadear Ingresar",
        );
        check(
          !/\bAcceso\b/i.test(interimText),
          "mientras sesión está en verificación no debe parpadear Acceso",
        );

        // Al completarse el GET 200 retardado:
        const miCuentaLink = header.getByRole("link", { name: /Mi cuenta/i });
        await miCuentaLink.waitFor({ state: "visible" });
        const confirmedText = await header.innerText();
        check(
          confirmedText.includes("Cuenta sintética") ||
            confirmedText.includes("Nombre Reactivo Actualizado") ||
            confirmedText.includes("770e8400"),
          "tras responder GET 200 retardado debe confirmar identidad",
        );
        check(
          await miCuentaLink.isVisible(),
          "enlace a Mi cuenta debe confirmarse visible",
        );
      } finally {
        fixture.state.sessionDelayMs = 0;
      }
    },
  );

  // 9. Smoke desktop (1365x900): identidad larga y 404 sin overflow y exactos 2 controles auth
  await run(
    "smoke desktop 1365x900: identidad larga y 404 sin overflow y exactos 2 controles auth",
    async () => {
      await page.setViewportSize({ width: 1365, height: 900 });
      const longName =
        "Nombre Extraordinariamente Largo Para Validacion Rigurosa De Truncamiento y Overflow En Navbar 80+";
      check(
        longName.length >= 80,
        "nombre largo de prueba debe tener al menos 80 caracteres",
      );
      fixture.state.profileName = longName;

      try {
        // Workspace con nombre largo
        await page.goto(`${edge.url}/mi-cuenta`);
        const wsHeader = page.locator("header").first();
        await wsHeader.waitFor();
        const logoutBtn = wsHeader.getByRole("button", {
          name: /Cerrar sesión/i,
        });
        await logoutBtn.waitFor({ state: "visible" });
        await page.evaluate(() => document.fonts.ready);

        // Nav exactos 2 controles auth en workspace
        const wsAuthControls = page
          .locator(
            'header nav[aria-label="Navegación del panel"] button:has-text("Cerrar sesión")',
          )
          .locator("xpath=..");
        const wsCount = await wsAuthControls.locator("> *").count();
        check(
          wsCount === 2,
          `navbar workspace desktop debe tener exactamente 2 controles auth (tiene ${wsCount})`,
        );

        // No overflow horizontal
        const wsOverflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth,
        );
        check(
          !wsOverflow,
          "no debe haber overflow horizontal en workspace desktop con nombre largo",
        );

        // Public con nombre largo
        await page.goto(`${edge.url}/`);
        const pubHeader = page.locator("header").first();
        await pubHeader.waitFor();
        const miCuentaLink = pubHeader.getByRole("link", {
          name: /Mi cuenta/i,
        });
        await miCuentaLink.waitFor({ state: "visible" });
        await page.evaluate(() => document.fonts.ready);

        // Nav exactos 2 controles auth en public
        const pubAuthControls = page
          .locator("header a[href='/mi-cuenta']")
          .locator("xpath=..");
        const pubCount = await pubAuthControls.locator("> *").count();
        check(
          pubCount === 2,
          `navbar público desktop debe tener exactamente 2 controles auth (tiene ${pubCount})`,
        );

        // No overflow horizontal
        const pubOverflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth,
        );
        check(
          !pubOverflow,
          "no debe haber overflow horizontal en public desktop con nombre largo",
        );

        // Workspace con ID largo (fallback 404)
        fixture.state.profile404 = true;
        await page.goto(`${edge.url}/mi-cuenta`);
        await page.locator("header").first().waitFor();
        await page
          .getByText("770e8400", { exact: false })
          .waitFor({ state: "visible" });
        await page.evaluate(() => document.fonts.ready);
        const idOverflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth,
        );
        check(
          !idOverflow,
          "no debe haber overflow horizontal en workspace desktop con fallback a ID largo",
        );
      } finally {
        fixture.state.profile404 = false;
        fixture.state.profileName = "Cuenta sintética";
      }
    },
  );

  // 10. Smoke mobile (390x844): identidad larga y 404 sin overflow y exactos 2 controles auth
  await run(
    "smoke mobile 390x844: identidad larga y 404 sin overflow y exactos 2 controles auth",
    async () => {
      await page.setViewportSize({ width: 390, height: 844 });
      const longName =
        "Nombre Extraordinariamente Largo Para Validacion Rigurosa De Truncamiento y Overflow En Navbar 80+";
      fixture.state.profileName = longName;

      try {
        // Workspace con nombre largo
        await page.goto(`${edge.url}/mi-cuenta`);
        const wsHeader = page.locator("header").first();
        await wsHeader.waitFor();
        const logoutBtn = wsHeader.getByRole("button", {
          name: /Cerrar sesión/i,
        });
        await logoutBtn.waitFor({ state: "visible" });
        await page.evaluate(() => document.fonts.ready);

        // Nav exactos 2 controles auth en workspace
        const wsAuthControls = page
          .locator(
            'header nav[aria-label="Navegación del panel"] button:has-text("Cerrar sesión")',
          )
          .locator("xpath=..");
        const wsCount = await wsAuthControls.locator("> *").count();
        check(
          wsCount === 2,
          `navbar workspace mobile debe tener exactamente 2 controles auth (tiene ${wsCount})`,
        );

        // No overflow horizontal
        const wsOverflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth,
        );
        check(
          !wsOverflow,
          "no debe haber overflow horizontal en workspace mobile con nombre largo",
        );

        // Public con nombre largo
        await page.goto(`${edge.url}/`);
        const pubHeader = page.locator("header").first();
        await pubHeader.waitFor();
        const miCuentaLink = pubHeader.getByRole("link", {
          name: /Mi cuenta/i,
        });
        await miCuentaLink.waitFor({ state: "visible" });
        await page.evaluate(() => document.fonts.ready);

        // Nav exactos 2 controles auth en public
        const pubAuthControls = page
          .locator("header a[href='/mi-cuenta']")
          .locator("xpath=..");
        const pubCount = await pubAuthControls.locator("> *").count();
        check(
          pubCount === 2,
          `navbar público mobile debe tener exactamente 2 controles auth (tiene ${pubCount})`,
        );

        // No overflow horizontal
        const pubOverflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth,
        );
        check(
          !pubOverflow,
          "no debe haber overflow horizontal en public mobile con nombre largo",
        );

        // Workspace con ID largo (fallback 404)
        fixture.state.profile404 = true;
        await page.goto(`${edge.url}/mi-cuenta`);
        await page.locator("header").first().waitFor();
        await page
          .getByText("770e8400", { exact: false })
          .waitFor({ state: "visible" });
        await page.evaluate(() => document.fonts.ready);
        const idOverflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth,
        );
        check(
          !idOverflow,
          "no debe haber overflow horizontal en workspace mobile con fallback a ID largo",
        );
      } finally {
        fixture.state.profile404 = false;
        fixture.state.profileName = "Cuenta sintética";
        await page.setViewportSize({ width: 1365, height: 900 });
      }
    },
  );

  // 11. Auditoría de seguridad: cero credenciales o secretos en logs o superficies DOM
  await run(
    "seguridad: ningún secreto o credencial privada expuesta en logs o DOM",
    async () => {
      const text = [...logs, ...browserErrors].join("\n");
      check(
        !sensitive.some((v) => v && text.includes(v)),
        "logs del servidor y browser nunca deben contener credenciales",
      );
      const network = networkSurfaces.join("\n");
      const credentials = [password, ...(fixture?.secrets() || [])];
      check(
        !credentials.some((v) => v && network.includes(v)),
        "superficies de respuesta no deben exponer contraseñas ni secrets de upstream",
      );
    },
  );
}

async function cleanup() {
  await context?.close().catch(() => {});
  await edge?.close().catch(() => {});
  await fixture?.close().catch(() => {});
  await Promise.all(
    children.map(
      (child) =>
        new Promise((resolve) => {
          if (child.exitCode !== null) return resolve();
          child.once("exit", resolve);
          child.kill("SIGTERM");
          const timer = setTimeout(() => {
            child.kill("SIGKILL");
            resolve();
          }, 5000);
          timer.unref();
        }),
    ),
  );

  const passed = results.filter((r) => r.passed).length;
  const report = {
    mode: "controlled-upstream-real-next-bff",
    passed,
    failed: results.length - passed,
    results,
    setupFailed: Boolean(process.exitCode),
  };

  fs.writeFileSync(
    path.join(out, "controlled-report.json"),
    `${JSON.stringify(report, null, 2)}\n`,
    { mode: 0o600 },
  );

  console.log(
    JSON.stringify({
      mode: report.mode,
      passed: report.passed,
      failed: report.failed,
      setupFailed: report.setupFailed,
    }),
  );

  if (!results.length || report.failed || report.setupFailed) {
    process.exitCode = 1;
  }
}

main()
  .catch((err) => {
    console.error(
      "Navbar browser suite failed unexpectedly; private details withheld",
      err.message,
    );
    process.exitCode = 1;
  })
  .finally(cleanup);
