import { expect, test } from "@playwright/test";

test("panel de entregas muestra confirmación explícita y sin republicación automática", async ({
  page,
}) => {
  const expiresAt = new Date(Date.now() + 60_000).toISOString();
  await page.route("**/api/backend/**", (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "not_found" } }),
    }),
  );
  await page.route("**/api/backend/auth/session", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        roles: ["arrendador"],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    }),
  );
  await page.route("**/api/backend/spaces", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: [
          {
            id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
            title: "Espacio de prueba",
            state: "activa",
          },
        ],
      }),
    }),
  );
  await page.route(
    "**/api/backend/spaces/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        headers: { etag: '"1"' },
        body: JSON.stringify({
          id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
          category_code: "oficina",
          category_name: "Oficina",
          title: "Espacio de prueba",
          description: "Prueba",
          area_m2: 10,
          capacity: 2,
          usage_rules: "",
          rate_unit: "hora",
          base_price_clp: 10000,
          address: "Dirección sintética",
          state: "activa",
          attribute_schema_version: 1,
          attributes: {},
        }),
      }),
  );
  await page.route(
    "**/api/backend/local/booking-trial/spaces/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/returns",
    (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            space_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
            items: [
              {
                reservation_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
                state: "finalizada",
                start_at: "2026-01-01T00:00:00Z",
                end_at: "2026-01-02T00:00:00Z",
                check_out_recorded: true,
                vacancy_confirmed: false,
              },
            ],
          },
        }),
      }),
  );
  await page.goto(
    "/anfitrion/espacios/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb/editar",
  );
  await expect(page.getByText("Entregas y desocupación")).toBeVisible();
  await expect(
    page.getByRole("checkbox", { name: /confirmo explícitamente/i }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Confirmar desocupación" }),
  ).toBeDisabled();
  await expect(page.getByRole("button", { name: "Republicar" })).toHaveCount(0);
  await page.getByRole("tab", { name: "Tarifas e historial" }).click();
  const unit = page.getByLabel("Unidad de cobro *", { exact: true });
  expect(
    await unit
      .locator("option")
      .evaluateAll((options) =>
        options.map((option) => (option as HTMLOptionElement).value),
      ),
  ).toEqual(["hora", "dia", "mes"]);
  const amount = page.getByLabel("Precio base (CLP) *", { exact: true });
  await amount.fill("5001");
  expect(
    await amount.evaluate((input) =>
      (input as HTMLInputElement).checkValidity(),
    ),
  ).toBe(true);
  await amount.fill("5000");
  expect(
    await amount.evaluate((input) =>
      (input as HTMLInputElement).checkValidity(),
    ),
  ).toBe(false);
});
