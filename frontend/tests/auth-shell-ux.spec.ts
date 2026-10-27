import fs from "node:fs";
import path from "node:path";
import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";

test.use({ trace: "off", screenshot: "off", video: "off" });

const OUTPUT_DIR = path.resolve(
  process.cwd(),
  "../.local/testing/registro-ux-fase1",
);

function ensureSecureDir(dir: string): void {
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  try {
    fs.chmodSync(dir, 0o700);
  } catch {}
}

function setSecureFilePermissions(filePath: string): void {
  try {
    fs.chmodSync(filePath, 0o600);
  } catch {}
}

const requestsOutsideLoopback = new WeakMap<Page, boolean>();
test.beforeEach(async ({ page }) => {
  requestsOutsideLoopback.set(page, false);
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      requestsOutsideLoopback.set(page, true);
    }
  });
});
test.afterEach(async ({ page }) => {
  expect(requestsOutsideLoopback.get(page)).toBe(false);
});

type MockResult = {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
};

type GatewayResponder = (
  pathname: string,
  method: string,
  request: Request,
) => MockResult | Promise<MockResult>;

async function mockGateway(
  page: Page,
  responder: GatewayResponder,
): Promise<void> {
  const bff = createBffFixtureState();
  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const result = bff.adapt(
      request,
      await responder(pathname, request.method(), request),
    );
    if (!result) throw new Error("Missing controlled BFF reply");
    await route.fulfill({
      status: result.status,
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        ...(result.headers ?? {}),
      },
      body: result.status === 204 ? "" : JSON.stringify(result.body ?? {}),
    });
  });
}

const termsFixtureData = {
  items: [
    {
      id: "550e8400-e29b-41d4-a716-446655440000",
      code: "terms-local-v1",
      type: "terminos",
      sha256: "a".repeat(64),
      published_at: "2026-10-25T12:00:00Z",
      synthetic: true,
      document: {
        id: "550e8400-e29b-41d4-a716-446655440000",
        code: "terms-local-v1",
        type: "terminos",
        title: "Términos sintéticos",
        scope: "local_test",
        status: "vigente",
        sections: [
          {
            heading: "Uso local",
            body: "Contenido sintético para pruebas locales.",
          },
        ],
      },
    },
    {
      id: "660e8400-e29b-41d4-a716-446655440000",
      code: "privacy-local-v1",
      type: "privacidad",
      sha256: "b".repeat(64),
      published_at: "2026-10-25T12:00:00Z",
      synthetic: true,
      document: {
        id: "660e8400-e29b-41d4-a716-446655440000",
        code: "privacy-local-v1",
        type: "privacidad",
        title: "Privacidad sintética",
        scope: "local_test",
        status: "vigente",
        sections: [
          {
            heading: "Privacidad local",
            body: "Contenido sintético para pruebas locales.",
          },
        ],
      },
    },
  ],
};

async function setupAuthRoutes(page: Page) {
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 401,
        body: {
          error: { code: "unauthenticated", message: "No active session" },
        },
      };
    }
    if (pathname === "/api/backend/auth/terms" && method === "GET") {
      return {
        status: 200,
        body: termsFixtureData,
      };
    }
    return {
      status: 404,
      body: {
        error: { code: "not_found", message: `Unknown path ${pathname}` },
      },
    };
  });
}

const AUTH_ROUTES = [
  { path: "/acceso", name: "acceso", title: "Acceso a tu cuenta" },
  { path: "/registro", name: "registro", title: "Crea tu cuenta en EspaciGo" },
  {
    path: "/verificar-correo",
    name: "verificar-correo",
    title: "Verificar correo electrónico",
  },
  {
    path: "/recuperar-acceso",
    name: "recuperar-acceso",
    title: "Recuperar acceso",
  },
];

test.describe("AuthShell UX, Brand Logo and Legal Footer", () => {
  test("captures updated after screenshots on acceso", async ({ page }) => {
    ensureSecureDir(OUTPUT_DIR);
    await setupAuthRoutes(page);

    const desktopShot = path.join(OUTPUT_DIR, "acceso-desktop-after.png");
    await page.setViewportSize({ width: 1365, height: 900 });
    await page.goto("/acceso", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");
    await page.screenshot({ path: desktopShot, fullPage: false });
    setSecureFilePermissions(desktopShot);

    const mobileShot = path.join(OUTPUT_DIR, "acceso-mobile-after.png");
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/acceso", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");
    await page.screenshot({ path: mobileShot, fullPage: false });
    setSecureFilePermissions(mobileShot);

    expect(fs.existsSync(desktopShot)).toBe(true);
    expect(fs.existsSync(mobileShot)).toBe(true);
  });

  test("acceso displays clear session copy under CTA without technical cookie jargon", async ({
    page,
  }) => {
    await setupAuthRoutes(page);
    await page.goto("/acceso", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");

    const sessionCopy = page.locator("form > p.text-xs");
    await expect(sessionCopy).toBeVisible();
    await expect(sessionCopy).toHaveText(
      "Tu sesión se conserva mientras esté vigente. Puedes cerrarla cuando quieras.",
    );
    await expect(sessionCopy).not.toContainText("cookie");
    await expect(sessionCopy).not.toContainText("protegida");
    await expect(sessionCopy).not.toContainText(
      "servicio hasta su vencimiento",
    );
  });

  test("AuthShell legal footer links meet touch target height >=44px without overflow at 390px", async ({
    page,
  }) => {
    await setupAuthRoutes(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/acceso", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("footer[aria-label='Información legal']");

    const termsLink = page.getByRole("link", {
      name: /términos y condiciones/i,
    });
    const privacyLink = page.getByRole("link", {
      name: /política de privacidad/i,
    });

    await expect(termsLink).toBeVisible();
    await expect(privacyLink).toBeVisible();

    const termsBox = await termsLink.boundingBox();
    const privacyBox = await privacyLink.boundingBox();

    expect(termsBox).not.toBeNull();
    expect(privacyBox).not.toBeNull();
    expect(termsBox!.height).toBeGreaterThanOrEqual(44);
    expect(privacyBox!.height).toBeGreaterThanOrEqual(44);

    const noOverflow = await page.evaluate(() => {
      const footer = document.querySelector(
        "footer[aria-label='Información legal']",
      );
      if (!footer) return false;
      return footer.scrollWidth <= window.innerWidth;
    });
    expect(noOverflow).toBe(true);
  });

  for (const route of AUTH_ROUTES) {
    test(`AuthShell renders real brand logo asset espacigo-mark.svg on ${route.path}`, async ({
      page,
    }) => {
      await setupAuthRoutes(page);
      await page.goto(route.path, { waitUntil: "domcontentloaded" });

      const brandLink = page.getByRole("link", {
        name: /espacigo/i,
      });
      await expect(brandLink).toBeVisible();

      // Real logo SVG asset check (not a Lucide CircleIcon)
      const logoImg = brandLink.locator('img[src*="espacigo-mark.svg"]');
      await expect(logoImg).toBeVisible();

      await expect
        .poll(
          async () =>
            await logoImg.evaluate(
              (img: HTMLImageElement) => img.complete && img.naturalWidth > 0,
            ),
        )
        .toBe(true);

      // Check that CircleIcon is no longer present in brand link
      const circleIcon = brandLink.locator("svg.lucide-circle");
      await expect(circleIcon).toHaveCount(0);
    });

    test(`AuthShell renders accessible legal footer links on ${route.path}`, async ({
      page,
    }) => {
      await setupAuthRoutes(page);
      await page.goto(route.path, { waitUntil: "domcontentloaded" });

      const termsLink = page.getByRole("link", {
        name: /términos y condiciones/i,
      });
      const privacyLink = page.getByRole("link", {
        name: /política de privacidad/i,
      });

      await expect(termsLink).toBeVisible();
      await expect(termsLink).toHaveAttribute(
        "href",
        "/terminos-y-condiciones",
      );

      await expect(privacyLink).toBeVisible();
      await expect(privacyLink).toHaveAttribute(
        "href",
        "/politica-de-privacidad",
      );
    });
  }

  test("AuthShell card layout has no horizontal overflow on desktop and mobile", async ({
    page,
  }) => {
    await setupAuthRoutes(page);
    const viewports = [
      { width: 1365, height: 900 },
      { width: 390, height: 844 },
    ];

    for (const vp of viewports) {
      await page.setViewportSize(vp);
      await page.goto("/acceso", { waitUntil: "domcontentloaded" });
      await page.waitForSelector("form");

      const noOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth <= window.innerWidth;
      });
      expect(noOverflow).toBe(true);

      // Verify logo and form header are above the fold (within viewport height)
      const brandLink = page.getByRole("link", { name: /espacigo/i });
      const brandBox = await brandLink.boundingBox();
      expect(brandBox).not.toBeNull();
      expect(brandBox!.y).toBeGreaterThanOrEqual(0);
      expect(brandBox!.y + brandBox!.height).toBeLessThan(vp.height);

      const form = page.locator("form");
      const formBox = await form.boundingBox();
      expect(formBox).not.toBeNull();
      expect(formBox!.y).toBeLessThan(vp.height);
    }
  });

  test("acceso keyboard navigation order reaches all interactive elements", async ({
    page,
  }) => {
    await setupAuthRoutes(page);
    await page.setViewportSize({ width: 1365, height: 900 });
    await page.goto("/acceso", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("form");

    // Click main content to start keyboard navigation
    await page.locator("#main-content").focus();
    await page.keyboard.press("Tab");

    // Brand link
    const activeAriaLabel = await page.evaluate(() =>
      document.activeElement?.getAttribute("aria-label"),
    );
    expect(activeAriaLabel).toMatch(/espacigo/i);

    // Email input
    await page.keyboard.press("Tab");
    const emailId = await page.evaluate(() => document.activeElement?.id);
    expect(emailId).toBe("email");

    // Password input
    await page.keyboard.press("Tab");
    const passwordId = await page.evaluate(() => document.activeElement?.id);
    expect(passwordId).toBe("password");

    // Toggle password visibility button
    await page.keyboard.press("Tab");
    const isToggle = await page.evaluate(() =>
      document.activeElement
        ?.getAttribute("aria-label")
        ?.includes("contraseña"),
    );
    expect(isToggle).toBe(true);

    // Forgot password link
    await page.keyboard.press("Tab");
    const forgotHref = await page.evaluate(() =>
      document.activeElement?.getAttribute("href"),
    );
    expect(forgotHref).toBe("/recuperar-acceso");

    // Submit button
    await page.keyboard.press("Tab");
    const isSubmit = await page.evaluate(
      () => (document.activeElement as HTMLButtonElement)?.type === "submit",
    );
    expect(isSubmit).toBe(true);

    // Secondary link (Crear una cuenta)
    await page.keyboard.press("Tab");
    const secondaryHref = await page.evaluate(() =>
      document.activeElement?.getAttribute("href"),
    );
    expect(secondaryHref).toBe("/registro");

    // Legal footer links: Terminos and Privacidad
    await page.keyboard.press("Tab");
    const termsHref = await page.evaluate(() =>
      document.activeElement?.getAttribute("href"),
    );
    expect(termsHref).toBe("/terminos-y-condiciones");

    await page.keyboard.press("Tab");
    const privacyHref = await page.evaluate(() =>
      document.activeElement?.getAttribute("href"),
    );
    expect(privacyHref).toBe("/politica-de-privacidad");
  });

  test("passes WCAG accessibility scan on acceso and registro", async ({
    page,
  }) => {
    await setupAuthRoutes(page);
    for (const testPath of ["/acceso", "/registro"]) {
      await page.goto(testPath, { waitUntil: "domcontentloaded" });
      await page.waitForSelector("form");
      const axe = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
        .analyze();
      expect(axe.violations.map((v) => v.id)).toEqual([]);
    }
  });

  test("Workspace layout renders accessible legal footer links on /mi-cuenta", async ({
    page,
  }) => {
    await setupAuthRoutes(page);
    await page.goto("/mi-cuenta", { waitUntil: "domcontentloaded" });

    const legalNav = page.getByRole("navigation", {
      name: "Enlaces legales",
    });
    await expect(legalNav).toBeVisible();

    const termsLink = legalNav.getByRole("link", {
      name: /términos y condiciones/i,
    });
    const privacyLink = legalNav.getByRole("link", {
      name: /política de privacidad/i,
    });

    await expect(termsLink).toBeVisible();
    await expect(termsLink).toHaveAttribute("href", "/terminos-y-condiciones");

    await expect(privacyLink).toBeVisible();
    await expect(privacyLink).toHaveAttribute(
      "href",
      "/politica-de-privacidad",
    );
  });
});
