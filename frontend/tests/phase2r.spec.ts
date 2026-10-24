import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

const routes = [
  { name: "home", path: "/" },
  { name: "products", path: "/products" },
  { name: "search", path: "/search?q=estudio" },
  { name: "space-detail", path: "/product/sala-luz-providencia" },
  { name: "saas-landing", path: "/saas" },
  { name: "login", path: "/login" },
  { name: "dashboard", path: "/dashboard" },
  { name: "dashboard-general", path: "/dashboard/general" },
  { name: "dashboard-security", path: "/dashboard/security" },
  { name: "dashboard-activity", path: "/dashboard/activity" },
  { name: "pricing-demo", path: "/pricing" },
] as const;
const viewports = [
  { name: "390x844", width: 390, height: 844 },
  { name: "1365x900", width: 1365, height: 900 },
] as const;
const screenshotDirectory = path.resolve(
  process.cwd(),
  "../.local/frontend-phase2r/captures",
);
const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? 4203}`;

test.beforeAll(async () => {
  await mkdir(screenshotDirectory, { recursive: true });
});

for (const route of routes) {
  for (const viewport of viewports) {
    test(`@a11y ${route.name} ${viewport.name} renders locally, fits and passes axe`, async ({
      page,
    }) => {
      test.setTimeout(90_000);
      const externalRequests: string[] = [];
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.route("**/*", async (request) => {
        const requestUrl = new URL(request.request().url());
        if (requestUrl.origin !== localOrigin) {
          externalRequests.push(requestUrl.origin);
          await request.abort();
          return;
        }
        await request.continue();
      });

      const response = await page.goto(route.path, {
        waitUntil: "networkidle",
      });
      expect(response?.status(), route.path).toBe(200);
      await expect(page.getByRole("main")).toBeVisible();
      await expect(page.locator("main h1").first()).toBeVisible();
      if (route.path === "/saas") {
        const terminal = page.locator("main .font-mono");
        await expect(terminal.getByText("pnpm test:e2e")).toHaveCSS(
          "opacity",
          "1",
        );
        await expect(terminal.locator(".opacity-0")).toHaveCount(0);
      }

      const dimensions = await page.evaluate(() => ({
        viewportWidth: document.documentElement.clientWidth,
        documentWidth: document.documentElement.scrollWidth,
        bodyWidth: document.body.scrollWidth,
      }));
      expect(
        dimensions.documentWidth,
        `${route.path} horizontal overflow`,
      ).toBeLessThanOrEqual(dimensions.viewportWidth);
      expect(
        dimensions.bodyWidth,
        `${route.path} body overflow`,
      ).toBeLessThanOrEqual(dimensions.viewportWidth);
      expect(
        externalRequests,
        `${route.path} attempted non-local network access`,
      ).toEqual([]);

      const screenshot = path.join(
        screenshotDirectory,
        `${viewport.name}__${route.name}.png`,
      );
      await page.screenshot({
        path: screenshot,
        fullPage: true,
        animations: "disabled",
      });

      const results = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
        .analyze();
      expect(
        results.violations.map(({ id, impact, description, nodes }) => ({
          id,
          impact,
          description,
          elements: nodes.map(({ target }) => target),
        })),
        `${route.path} at ${viewport.name}`,
      ).toEqual([]);
    });
  }
}

test("marketplace detail gallery changes images using the original navigation controls", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1365, height: 900 });
  await page.goto("/product/sala-luz-providencia");
  await expect(page.getByText("1 / 2")).toBeVisible();
  await page.getByRole("button", { name: "Siguiente imagen" }).click();
  await expect(page.getByText("2 / 2")).toBeVisible();
});

test("workspace account, security and pricing stay previews while login is enabled", async ({
  page,
}) => {
  await page.goto("/dashboard/general");
  await expect(page.getByLabel("Nombre")).toBeDisabled();
  await expect(page.getByLabel("Correo electrónico")).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Guardar cambios" }),
  ).toBeDisabled();

  await page.goto("/dashboard/security");
  await expect(page.getByLabel("Contraseña actual")).toBeDisabled();
  await expect(
    page.getByLabel("Nueva contraseña", { exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Actualizar contraseña" }),
  ).toBeDisabled();
  await expect(page.getByLabel("Confirmar contraseña")).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Eliminar cuenta" }),
  ).toBeDisabled();

  await page.goto("/login");
  await expect(page.getByLabel("Correo electrónico")).toBeEnabled();
  await expect(page.getByLabel("Contraseña")).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "Iniciar sesión" }),
  ).toBeEnabled();

  await page.goto("/pricing");
  await expect(
    page.getByText("no hay precios, suscripciones, checkout ni pagos."),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Solo vista previa" }).first(),
  ).toBeDisabled();
});

test("skip link is first in keyboard order and moves focus to main content", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.keyboard.press("Tab");
  const skipLink = page.getByRole("link", {
    name: "Omitir al contenido principal",
  });
  await expect(skipLink).toBeFocused();
  const ring = await page.evaluate(
    () => getComputedStyle(document.activeElement!).boxShadow,
  );
  expect(ring).not.toBe("none");
  await page.keyboard.press("Enter");
  await expect(page.locator("#main-content")).toBeFocused();
});

test("search suggestions support keyboard selection using local fixtures", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/search");
  const input = page.getByRole("combobox", { name: "Buscar espacios" });
  await input.fill("estudio");
  const firstSuggestion = page.getByRole("option").first();
  await expect(firstSuggestion).toBeVisible();
  await input.press("ArrowDown");
  await input.press("Enter");
  await expect(page).toHaveURL(/\/product\//);
});

test("mobile marketplace navigation opens and follows a template route", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByRole("button", { name: "Abrir menú" }).click();
  const spacesLink = page.getByRole("link", { name: "Espacios", exact: true });
  await expect(spacesLink).toBeVisible();
  await spacesLink.click();
  await expect(page).toHaveURL(/\/products$/);
  await expect(
    page.getByRole("heading", { name: "Explorar espacios" }),
  ).toBeVisible();
});

test("mobile workspace navigation opens and follows a dashboard route", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/dashboard");
  await page.getByRole("button", { name: "Abrir navegación" }).click();
  const securityLink = page.getByRole("link", { name: "Seguridad" });
  await expect(securityLink).toBeVisible();
  await securityLink.click();
  await expect(page).toHaveURL(/\/dashboard\/security$/);
  await expect(page.getByRole("heading", { name: "Seguridad" })).toBeVisible();
});

test("mobile catalog filters preserve the listing view and update its route", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/products");
  await page.getByRole("button", { name: "Filtros" }).click();
  await expect(
    page.getByRole("dialog").getByRole("heading", { name: "Filtros" }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "Cowork" }).click();
  await expect(page).toHaveURL(/category=cowork/);
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("heading", { name: "Explorar espacios" }),
  ).toBeVisible();
});
