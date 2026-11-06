import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const fileId = "880e8400-e29b-41d4-a716-446655440000";
const assigned = {
  id: caseId,
  purpose: "identity",
  state: "en_revision",
  revision: 1,
  files: [
    {
      id: fileId,
      state: "limpio",
      size: 15,
      mime: "application/pdf",
      pages: 1,
    },
  ],
};
async function reviewerModel(
  page: Page,
  failure?: "post-404" | "get-500" | "stale",
) {
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const documentExpiresAt = new Date(
    Date.now() + 90 * 86_400_000,
  ).toISOString();
  const secondId = "990e8400-e29b-41d4-a716-446655440000";
  const cases = new Map(
    [caseId, secondId].map((id) => [
      id,
      { ...assigned, id, document_expires_at: documentExpiresAt },
    ]),
  );
  const model = {
    denied: false,
    requests: [] as Array<{ method: string; action: string }>,
    secondId,
    failure,
    refreshAllowed: false,
    authoritativeRefreshed: false,
    writeCount: 0,
  };
  await page.route("**/api/backend/auth/session", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: "550e8400-e29b-41d4-a716-446655440000",
        roles: [],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    }),
  );
  await page.route(
    "**/api/backend/document-verification/reviewer/**",
    async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const suffix = url.pathname.split("/cases")[1] ?? "";
      const parts = suffix.split("/").filter(Boolean);
      const method = request.method(),
        action = parts.at(-1) ?? "list";
      model.requests.push({ method, action });
      if (model.denied)
        return route.fulfill({
          status: 403,
          contentType: "application/json",
          body: JSON.stringify({ error: { code: "forbidden" } }),
        });
      if (
        failure &&
        !model.authoritativeRefreshed &&
        model.writeCount > 0 &&
        method === "GET" &&
        parts.length
      ) {
        if (failure === "get-500")
          return route.fulfill({ status: 500, body: "readback failed" });
        if (failure === "stale")
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({
              ...cases.get(parts[0]),
              state: "en_revision",
              revision: 1,
            }),
          });
      }
      if (!parts.length) {
        if (failure && model.writeCount > 0 && !model.refreshAllowed) {
          if (failure === "get-500")
            return route.fulfill({ status: 500, body: "readback failed" });
          if (failure === "stale")
            return route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({
                items: [
                  { ...cases.get(caseId)!, state: "en_revision", revision: 1 },
                  cases.get(secondId)!,
                ],
              }),
            });
        }
        if (failure && model.writeCount > 0 && model.refreshAllowed) {
          model.refreshAllowed = false;
          model.authoritativeRefreshed = true;
        }
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: [...cases.values()] }),
        });
      }
      const current = cases.get(parts[0]);
      if (!current) return route.fulfill({ status: 404 });
      if (method === "POST") {
        model.writeCount++;
        const body = request.postDataJSON();
        expect(body.expected_revision).toBe(current.revision);
        if (action === "review") {
          expect(body.capability).toBe("review_identity");
          expect(body.decision).toBe("approve");
          current.state = "aprobada";
        } else {
          expect(action).toBe("revoke");
          expect(body.purpose).toBe("identity");
          expect(body.reason_code).toBe("solicitud_titular");
          current.state = "revocada";
        }
        current.revision++;
        if (failure === "post-404" && action === "review")
          return route.fulfill({
            status: 404,
            body: "committed; response unavailable",
          });
      }
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(current),
      });
    },
  );
  return model;
}

test("reviewer decision and revocation reread authoritative metadata with accessible forms", async ({
  page,
}) => {
  const model = await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  const axe = await new AxeBuilder({ page }).analyze();
  expect(axe.violations.map((v) => v.id)).toEqual([]);
  await page
    .getByRole("combobox", { name: "Decisión", exact: true })
    .selectOption("approve");
  const expiry = new Date(Date.now() + 86_400_000).toISOString().slice(0, 16);
  await page.getByLabel("Vigencia hasta").fill(expiry);
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toBeEnabled();
  await page
    .getByRole("button", { name: "Confirmar decisión", exact: true })
    .click();
  await expect(
    page.locator('[data-review-case-state="aprobada"]'),
  ).toBeVisible();
  const approve = model.requests.findIndex(
    (r) => r.method === "POST" && r.action === "review",
  );
  expect(
    model.requests.slice(approve + 1).some((r) => r.method === "GET"),
  ).toBe(true);
  await page
    .getByRole("combobox", { name: "Motivo", exact: true })
    .selectOption("solicitud_titular");
  await page.getByRole("button", { name: "Revocar", exact: true }).click();
  await expect(
    page.locator('[data-review-case-state="revocada"]'),
  ).toBeVisible();
  const revoke = model.requests.findIndex(
    (r) => r.method === "POST" && r.action === "revoke",
  );
  expect(model.requests.slice(revoke + 1).some((r) => r.method === "GET")).toBe(
    true,
  );
});

test("ambiguous committed review purges rows and recovers only after explicit authoritative refresh", async ({
  page,
}) => {
  for (const failure of ["post-404", "get-500", "stale"] as const) {
    await page.goto("about:blank");
    const model = await reviewerModel(page, failure);
    await page.goto("/mi-cuenta/revision-documental");
    await expect(
      page.locator(`[data-review-case-id="${caseId}"]`),
    ).toBeVisible();
    await page.locator(`[data-review-case-id="${caseId}"]`).click();
    const expiry = new Date(Date.now() + 86_400_000).toISOString().slice(0, 16);
    await page.getByLabel("Vigencia hasta").fill(expiry);
    await page
      .getByRole("button", { name: "Confirmar decisión", exact: true })
      .click();
    await expect(
      page
        .getByRole("alert")
        .filter({ hasText: "no se confirmó el resultado" }),
    ).toBeVisible();
    await expect(page.locator("[data-review-case-id]")).toHaveCount(0);
    await expect(page.locator("[data-review-case-state]")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Confirmar decisión", exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Abrir archivo limpio", exact: true }),
    ).toHaveCount(0);
    expect(model.writeCount).toBe(1);

    model.refreshAllowed = true;
    await page
      .getByRole("button", { name: "Actualizar expedientes", exact: true })
      .click();
    await expect(
      page.locator(`[data-review-case-id="${caseId}"]`),
    ).toBeVisible();
    await expect(
      page.locator(`[data-review-case-id="${model.secondId}"]`),
    ).toBeVisible();
    await page.locator(`[data-review-case-id="${caseId}"]`).click();
    await expect(
      page.locator('[data-review-case-state="aprobada"]'),
    ).toContainText("Revisión 2");
    expect(model.writeCount).toBe(1);
  }
});

test("reviewer case changes clear form values and403 purges private case controls", async ({
  page,
}) => {
  const model = await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  const expiry = new Date(Date.now() + 86_400_000).toISOString().slice(0, 16);
  await page.getByLabel("Vigencia hasta").fill(expiry);
  await expect(page.getByLabel("Vigencia hasta")).toHaveValue(expiry);
  await page.locator(`[data-review-case-id="${model.secondId}"]`).click();
  await expect(page.getByLabel("Vigencia hasta")).toHaveValue("");
  model.denied = true;
  await page
    .getByRole("button", { name: "Abrir archivo limpio", exact: true })
    .click();
  await expect(page.locator("[data-review-case-state]")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toHaveCount(0);
});

test("reviewer lists minimized assignment and downloads only a clean file", async ({
  page,
}) => {
  const requests: string[] = [];
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  await page.route("**/api/backend/auth/session", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: "550e8400-e29b-41d4-a716-446655440000",
        roles: [],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    }),
  );
  await page.route(
    "**/api/backend/document-verification/reviewer/**",
    async (route) => {
      const url = new URL(route.request().url());
      requests.push(`${route.request().method()} ${url.pathname}`);
      if (url.pathname.endsWith("/cases"))
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: [assigned] }),
        });
      if (url.pathname.endsWith("/content"))
        return route.fulfill({
          status: 200,
          contentType: "application/pdf",
          body: "%PDF-1.4 mock",
        });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(assigned),
      });
    },
  );
  await page.goto("/mi-cuenta/revision-documental");
  await expect(
    page.getByRole("heading", { name: "Revisión documental" }),
  ).toBeVisible();
  await expect(page.getByText("Identidad · En revisión")).toBeVisible();
  await page.getByRole("button", { name: "Abrir expediente" }).click();
  await expect(
    page.getByRole("heading", { name: "Expediente · Identidad" }),
  ).toBeVisible();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Abrir archivo limpio" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("documento-verificacion.pdf");
  expect(
    requests.some((x) =>
      x.endsWith(`/cases/${caseId}/files/${fileId}/content`),
    ),
  ).toBeTruthy();
});
