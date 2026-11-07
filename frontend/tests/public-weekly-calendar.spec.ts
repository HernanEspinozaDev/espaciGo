import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const activeId = "00000000-0000-4000-8000-000000000001";
const missingId = "00000000-0000-4000-8000-000000000002";
const inactiveId = "00000000-0000-4000-8000-000000000003";

test("public schedule is read-only and shows exact weekly periods", async ({
  page,
}) => {
  const writes: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method()))
      writes.push(`${request.method()} ${request.url()}`);
  });
  for (const viewport of [
    { width: 1365, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    await page.goto(`/espacios/${activeId}`);
    await expect(
      page.getByRole("heading", { name: "Horario semanal habitual", level: 3 }),
    ).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    const dayButton = page.getByRole("button", { name: "Martes" });
    const beforePressed = await dayButton.getAttribute("aria-pressed");
    await dayButton.click();
    await expect(dayButton).toHaveAttribute(
      "aria-pressed",
      beforePressed === "true" ? "false" : "true",
    );
    await expect(page.getByLabel("Tramo 1: hora de inicio")).toHaveValue(
      "14:15",
    );
    await expect(
      page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
    ).toHaveValue("16:45");
    await page.evaluate(() => document.fonts.ready);
    await page.screenshot({
      path: test.info().outputPath(`public-calendar-${viewport.width}.png`),
      fullPage: true,
    });
    expect(
      await page
        .locator("[aria-label='Día de la semana'] button")
        .evaluateAll((els) =>
          els.every((el) => {
            const r = el.getBoundingClientRect();
            return r.width >= 44 && r.height >= 44;
          }),
        ),
    ).toBeTruthy();
    await expect(
      page.getByRole("button", { name: /Mañana 09:00/ }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: /14:00–15:00/ }),
    ).toBeDisabled();
    expect(
      await page.evaluate(
        () =>
          document.documentElement.scrollWidth <=
          document.documentElement.clientWidth,
      ),
    ).toBeTruthy();
    const a11y = await new AxeBuilder({ page }).analyze();
    expect(a11y.violations).toEqual([]);
  }
  expect(writes).toEqual([]);
});

test("absent and inactive schedules are presented honestly", async ({
  page,
}) => {
  await page.goto(`/espacios/${missingId}`);
  await expect(page.getByText("Horario no informado.")).toBeVisible();
  await page.goto(`/espacios/${inactiveId}`);
  await expect(page.getByText("Horario inactivo.")).toBeVisible();
  await expect(
    page.getByText(/no confirma disponibilidad para fechas específicas/),
  ).toBeVisible();
});
