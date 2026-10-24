import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

const routes = ["/", "/panel", "/design-system"] as const;
const viewports = [
  { name: "mobile-390x844", width: 390, height: 844 },
  { name: "desktop-1365x900", width: 1365, height: 900 },
] as const;
const screenshotDirectory = path.resolve(
  process.cwd(),
  "../.local/frontend-phase2",
);

for (const route of routes) {
  test(`@a11y ${route} has no WCAG 2.2 AA axe violations`, async ({ page }) => {
    for (const viewport of viewports) {
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });
      await page.goto(route);
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
        `${route} at ${viewport.name}`,
      ).toEqual([]);
    }
  });
}

test("public and workspace layouts fit the requested viewports and touch targets", async ({
  page,
}) => {
  await mkdir(screenshotDirectory, { recursive: true });

  for (const route of routes) {
    for (const viewport of viewports) {
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });
      await page.goto(route);
      const measurements = await page.evaluate(() => {
        const interactive = Array.from(
          document.querySelectorAll<HTMLElement>(
            "a, button, summary, input, select, [role='button']",
          ),
        ).filter((element) => {
          const style = getComputedStyle(element);
          const rect = element.getBoundingClientRect();
          return (
            style.display !== "none" &&
            style.visibility !== "hidden" &&
            rect.width > 0 &&
            rect.height > 0
          );
        });
        return {
          viewportWidth: window.innerWidth,
          documentWidth: document.documentElement.scrollWidth,
          bodyWidth: document.body.scrollWidth,

          undersizedTargets: interactive
            .map((element) => {
              const rect = element.getBoundingClientRect();
              return {
                label:
                  element.innerText ||
                  element.getAttribute("aria-label") ||
                  element.tagName,
                width: rect.width,
                height: rect.height,
              };
            })
            .filter(({ width, height }) => width < 24 || height < 24),
        };
      });
      expect(
        measurements.documentWidth,
        `${route} document overflow at ${viewport.name}`,
      ).toBeLessThanOrEqual(measurements.viewportWidth);
      expect(
        measurements.bodyWidth,
        `${route} body overflow at ${viewport.name}`,
      ).toBeLessThanOrEqual(measurements.viewportWidth);
      expect(
        measurements.undersizedTargets,
        `${route} touch targets at ${viewport.name}`,
      ).toEqual([]);

      if (route === "/" || route === "/panel") {
        const file = route === "/" ? "marketplace" : "workspace";
        await page.screenshot({
          path: path.join(screenshotDirectory, `${file}-${viewport.name}.png`),
          fullPage: true,
        });
      }
    }
  }
});

test("skip link is first in keyboard order and moves focus to main content", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Omitir al contenido principal" }),
  ).toBeFocused();
  const outline = await page.evaluate(
    () => getComputedStyle(document.activeElement!).outlineStyle,
  );
  expect(outline).toBe("solid");
  await page.keyboard.press("Enter");
  await expect(page.locator("#main-content")).toBeFocused();
});

test("mobile panel navigation opens and works from the keyboard", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/panel");
  const menu = page.locator(".workspace-mobile-menu");
  const trigger = menu.locator("summary");
  await trigger.focus();
  await page.keyboard.press("Enter");
  await expect(menu).toHaveJSProperty("open", true);
  await expect(menu.getByRole("link", { name: "Solicitudes" })).toBeVisible();
  await expect(menu.getByRole("link", { name: "Resumen" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

test("reduced-motion preference disables smooth scrolling", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await expect
    .poll(() =>
      page.evaluate(
        () => getComputedStyle(document.documentElement).scrollBehavior,
      ),
    )
    .toBe("auto");
});

test("brand icon is served as the app favicon", async ({ page }) => {
  await page.goto("/");
  const icon = page.locator('link[rel="icon"]').first();
  await expect(icon).toHaveCount(1);
  const href = await icon.getAttribute("href");
  expect(href).toBeTruthy();
  const response = await page.request.get(
    new URL(href!, page.url()).toString(),
  );
  expect(response.ok()).toBeTruthy();
  expect(response.headers()["content-type"]).toContain("image/svg+xml");
});
