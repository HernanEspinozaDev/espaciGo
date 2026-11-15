import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const fileId = "880e8400-e29b-41d4-a716-446655440000";
const backFileId = "aa0e8400-e29b-41d4-a716-446655440000";
const selfieFileId = "bb0e8400-e29b-41d4-a716-446655440000";
const assigned = {
  id: caseId,
  purpose: "identity",
  state: "en_revision",
  revision: 1,
  document_format_declared: "new",
  document_format_observed: undefined as string | undefined,
  document_expires_at_observed: undefined as string | undefined,
  files: [
    {
      id: fileId,
      evidence_role: "identity_front",
      state: "limpio",
      size: 68,
      mime: "image/png",
      pages: 1,
    },
    {
      id: backFileId,
      evidence_role: "identity_back",
      state: "limpio",
      size: 68,
      mime: "image/png",
      pages: 1,
    },
    {
      id: selfieFileId,
      evidence_role: "identity_selfie",
      state: "limpio",
      size: 68,
      mime: "image/png",
      pages: 1,
    },
  ],
};
const declaration = {
  first_name: "Synthetic Review",
  last_name: "Person",
  rut: "11.222.333-9",
  date_of_birth: "1990-02-03",
};
const checklistLabels = [
  "Nombres y apellidos coinciden",
  "RUN/RUT coincide",
  "Fecha de nacimiento y mayoría de edad coinciden",
  "Retrato del documento coincide con la selfie",
  "Las evidencias son legibles",
  "El documento está vigente",
  "Fecha de emisión y formato fueron revisados",
];
async function completeIdentityApproval(page: Page) {
  const documentExpiryDate = new Date(Date.now() + 90 * 86_400_000)
    .toISOString()
    .slice(0, 10);
  const documentExpiry = new Date(`${documentExpiryDate}T23:59:59.999Z`);
  await page.getByLabel("Formato observado").selectOption("new");
  await page
    .getByLabel("Vencimiento del documento observado")
    .fill(documentExpiryDate);
  await page
    .getByLabel("Vigencia hasta")
    .fill(new Date(Date.now() + 86_400_000).toISOString().slice(0, 16));
  for (const label of checklistLabels)
    await page.getByRole("radio", { name: `${label}: Sí` }).check();
  return documentExpiry;
}
async function reviewerModel(
  page: Page,
  failure?: "post-404" | "get-500" | "stale",
) {
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const secondId = "990e8400-e29b-41d4-a716-446655440000";
  const cases = new Map(
    [caseId, secondId].map((id) => [id, { ...assigned, id }]),
  );
  const model = {
    denied: false,
    deniedStatus: 403 as 401 | 403,
    requests: [] as Array<{ method: string; action: string }>,
    deniedResponses: [] as Array<{
      method: string;
      path: string;
      status: number;
    }>,
    secondId,
    failure,
    failNextList: false,
    refreshAllowed: false,
    authoritativeRefreshed: false,
    writeCount: 0,
    reviewPayload: null as Record<string, unknown> | null,
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
      if (method === "GET" && action === "content")
        return route.fulfill({
          status: 200,
          contentType: "image/png",
          body: Buffer.from(
            "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nm8AAAAASUVORK5CYII=",
            "base64",
          ),
        });
      if (model.denied) {
        model.deniedResponses.push({
          method,
          path: url.pathname,
          status: model.deniedStatus,
        });
        return route.fulfill({
          status: model.deniedStatus,
          contentType: "application/json",
          body: JSON.stringify({ error: { code: "forbidden" } }),
        });
      }
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
        if (model.failNextList) {
          model.failNextList = false;
          return route.fulfill({
            status: 503,
            contentType: "application/json",
            body: JSON.stringify({ error: { code: "synthetic_unavailable" } }),
          });
        }
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
          expect(body.document_format_observed).toBe("new");
          expect(body.document_expires_at_observed).toBeTruthy();
          expect(body.verified_until).toBeTruthy();
          expect(body.identity_review_checklist).toEqual({
            names_match: true,
            rut_matches: true,
            birth_date_and_majority_match: true,
            portrait_matches_selfie: true,
            evidence_legible: true,
            document_unexpired: true,
            issue_date_and_format_verified: true,
          });
          model.reviewPayload = body;
          current.document_format_observed = body.document_format_observed;
          current.document_expires_at_observed =
            body.document_expires_at_observed;
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
        body: JSON.stringify(
          method === "GET" &&
            current.state !== "aprobada" &&
            current.state !== "rechazada" &&
            current.state !== "revocada"
            ? { ...current, identity_declaration: declaration }
            : current,
        ),
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
  const documentExpiry = await completeIdentityApproval(page);
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
  expect(model.reviewPayload?.document_expires_at_observed).toBe(
    documentExpiry.toISOString(),
  );
  expect(model.reviewPayload?.verified_until).not.toBe(
    model.reviewPayload?.document_expires_at_observed,
  );
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

test("assigned identity detail shows declaration, checklist and private evidence viewer", async ({
  page,
}) => {
  await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();

  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("radio", { name: "Nombres y apellidos coinciden: Sí" }),
  ).toBeVisible();
  await expect(
    page.getByRole("radio", { name: "RUN/RUT coincide: Sí" }),
  ).toBeVisible();
  await expect(
    page.getByRole("radio", {
      name: "Fecha de nacimiento y mayoría de edad coinciden: Sí",
    }),
  ).toBeVisible();
  await expect(page.getByLabel("Formato observado")).toBeVisible();

  await page.locator(`[data-review-file-id="${fileId}"]`).click();
  await expect(
    page.getByRole("dialog", { name: "Visor privado de evidencia" }),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: "Evidencia privada: anverso" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Cerrar visor" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("refreshing assignments retains selected declaration at the same revision", async ({
  page,
}) => {
  const model = await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toBeVisible();

  await page
    .getByRole("button", { name: "Actualizar expedientes", exact: true })
    .click();
  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.locator(`[data-review-case-state="en_revision"]`),
  ).toContainText("Revisión 1");
  expect(
    model.requests.filter(
      (request) => request.method === "GET" && request.action === "list",
    ).length,
  ).toBe(2);
});

test("ordinary assignment refresh failure purges private detail until explicit recovery", async ({
  page,
}) => {
  const model = await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toBeVisible();

  model.failNextList = true;
  await page
    .getByRole("button", { name: "Actualizar expedientes", exact: true })
    .click();
  await expect(page.locator("[data-review-case-id]")).toHaveCount(0);
  await expect(page.locator("[data-review-case-state]")).toHaveCount(0);
  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Abrir archivo limpio", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("alert").filter({
      hasText: "No fue posible completar la operación.",
    }),
  ).toContainText("Actualiza expedientes antes de continuar.");
  expect(model.writeCount).toBe(0);

  await page
    .getByRole("button", { name: "Actualizar expedientes", exact: true })
    .click();
  await expect(page.locator(`[data-review-case-id="${caseId}"]`)).toBeVisible();
  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toHaveCount(0);
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  await expect(
    page.getByText("Synthetic Review Person", { exact: true }),
  ).toBeVisible();
  expect(model.writeCount).toBe(0);
});

test("identity approval stays disabled until every checklist result is recorded", async ({
  page,
}) => {
  await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  const expiry = new Date(Date.now() + 86_400_000).toISOString().slice(0, 16);
  const documentExpiry = new Date(Date.now() + 90 * 86_400_000)
    .toISOString()
    .slice(0, 10);
  await page
    .getByLabel("Vencimiento del documento observado")
    .fill(documentExpiry);
  await page.getByLabel("Vigencia hasta").fill(expiry);
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toBeDisabled();

  await page.getByLabel("Formato observado").selectOption("new");
  for (const label of [
    "Nombres y apellidos coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad coinciden",
    "Retrato del documento coincide con la selfie",
    "Las evidencias son legibles",
    "El documento está vigente",
    "Fecha de emisión y formato fueron revisados",
  ]) {
    await page.getByRole("radio", { name: `${label}: Sí` }).check();
  }
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toBeEnabled();
  await page.getByLabel("Formato observado").selectOption("undetermined");
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toBeDisabled();
  await page.getByLabel("Formato observado").selectOption("new");
  await page
    .getByRole("radio", { name: "El documento está vigente: No" })
    .check();
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("radio", { name: "El documento está vigente: Sí" })
    .check();
  await expect(
    page.getByRole("button", { name: "Confirmar decisión", exact: true }),
  ).toBeEnabled();
});

test("assigned reviewer opens evidence in a private viewer instead of downloading it", async ({
  page,
}) => {
  await reviewerModel(page);
  await page.goto("/mi-cuenta/revision-documental");
  await page.locator(`[data-review-case-id="${caseId}"]`).click();
  await page.evaluate(() => {
    const target = window as typeof window & { __revokedBlobUrls?: string[] };
    target.__revokedBlobUrls = [];
    const revoke = URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL = (url) => {
      target.__revokedBlobUrls?.push(url);
      revoke(url);
    };
  });
  await page.locator(`[data-review-file-id="${fileId}"]`).click();
  await expect(
    page.getByRole("dialog", { name: "Visor privado de evidencia" }),
  ).toBeVisible();
  const image = page.getByRole("img", { name: "Evidencia privada: anverso" });
  await expect(image).toBeVisible();
  const blobUrl = await image.getAttribute("src");
  expect(blobUrl).toMatch(/^blob:/);
  await page.getByRole("button", { name: "Cerrar visor" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const revoked = await page.evaluate(
    () =>
      (window as typeof window & { __revokedBlobUrls?: string[] })
        .__revokedBlobUrls,
  );
  expect(revoked).toContain(blobUrl);
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
    await completeIdentityApproval(page);
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

for (const status of [401, 403] as const) {
  test(`reviewer case changes clear form values and ${status} purges private case controls`, async ({
    page,
  }) => {
    const model = await reviewerModel(page);
    await page.addInitScript(() => {
      const audit = { created: [] as string[], revoked: [] as string[] };
      const instrumentedWindow = window as typeof window & {
        __privateBlobAudit?: typeof audit;
      };
      instrumentedWindow.__privateBlobAudit = audit;
      const createObjectURL = URL.createObjectURL.bind(URL);
      URL.createObjectURL = (blob) => {
        const url = createObjectURL(blob);
        audit.created.push(url);
        return url;
      };
      const revokeObjectURL = URL.revokeObjectURL.bind(URL);
      URL.revokeObjectURL = (url) => {
        audit.revoked.push(url);
        revokeObjectURL(url);
      };
    });
    await page.goto("/mi-cuenta/revision-documental");
    await page.locator(`[data-review-case-id="${caseId}"]`).click();
    const expiry = new Date(Date.now() + 86_400_000).toISOString().slice(0, 16);
    const documentExpiry = new Date(Date.now() + 90 * 86_400_000)
      .toISOString()
      .slice(0, 10);
    await page.getByLabel("Formato observado").selectOption("new");
    await page
      .getByLabel("Vencimiento del documento observado")
      .fill(documentExpiry);
    await page
      .getByRole("radio", { name: "Nombres y apellidos coinciden: Sí" })
      .check();
    await page.getByLabel("Vigencia hasta").fill(expiry);
    await expect(page.getByLabel("Vigencia hasta")).toHaveValue(expiry);
    await page.locator(`[data-review-case-id="${model.secondId}"]`).click();
    await expect(page.getByLabel("Vigencia hasta")).toHaveValue("");
    await expect(page.getByLabel("Formato observado")).toHaveValue("");
    await expect(
      page.getByLabel("Vencimiento del documento observado"),
    ).toHaveValue("");
    await expect(
      page.getByRole("radio", { name: "Nombres y apellidos coinciden: Sí" }),
    ).not.toBeChecked();
    const fileButton = page
      .getByRole("listitem")
      .filter({ hasText: "Evidencia · anverso" })
      .getByRole("button", { name: "Abrir archivo limpio", exact: true });
    await fileButton.click();
    await expect(
      page.getByRole("dialog", { name: "Visor privado de evidencia" }),
    ).toBeVisible();
    const createdUrls = await page.evaluate(
      () =>
        (
          window as typeof window & {
            __privateBlobAudit?: { created: string[]; revoked: string[] };
          }
        ).__privateBlobAudit?.created ?? [],
    );
    expect(createdUrls).toHaveLength(1);
    model.denied = true;
    model.deniedStatus = status;
    const deniedResponsePromise = page.waitForResponse((response) => {
      const request = response.request();
      return (
        request.method() === "GET" &&
        new URL(response.url()).pathname ===
          `/api/backend/document-verification/reviewer/cases/${model.secondId}`
      );
    });
    await fileButton.click();
    const deniedResponse = await deniedResponsePromise;
    expect(deniedResponse.status()).toBe(status);
    expect(deniedResponse.request().method()).toBe("GET");
    expect(new URL(deniedResponse.url()).pathname).toBe(
      `/api/backend/document-verification/reviewer/cases/${model.secondId}`,
    );
    expect(model.deniedResponses).toEqual([
      {
        method: "GET",
        path: `/api/backend/document-verification/reviewer/cases/${model.secondId}`,
        status,
      },
    ]);
    await expect(page.locator("[data-review-case-state]")).toHaveCount(0);
    await expect(page.locator("[data-review-case-id]")).toHaveCount(0);
    await expect(page.locator("[data-review-file-id]")).toHaveCount(0);
    await expect(page.locator("form")).toHaveCount(0);
    await expect(
      page.getByText("Declaración de identidad guardada", { exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByText("Synthetic Review", { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByText("11.222.333-9", { exact: true })).toHaveCount(
      0,
    );
    await expect(
      page.getByText("Evidencia · anverso", { exact: true }),
    ).toHaveCount(0);
    await expect(page.getByLabel("Decisión", { exact: true })).toHaveCount(0);
    await expect(page.getByLabel("Formato observado")).toHaveCount(0);
    await expect(
      page.getByLabel("Vencimiento del documento observado"),
    ).toHaveCount(0);
    await expect(
      page.getByRole("group", { name: "Checklist de revisión de identidad" }),
    ).toHaveCount(0);
    await expect(page.getByLabel("Vigencia hasta")).toHaveCount(0);
    await expect(
      page.getByRole("dialog", { name: "Visor privado de evidencia" }),
    ).toHaveCount(0);
    if (status === 401) {
      await expect(
        page.getByText(
          "Inicia sesión con una cuenta revisora asignada para continuar.",
          { exact: true },
        ),
      ).toBeVisible();
    } else {
      await expect(
        page.getByRole("alert").filter({
          hasText:
            "No tienes acceso vigente al expediente. Se limpiaron los datos privados.",
        }),
      ).toBeVisible();
    }
    await expect(
      page.getByRole("button", { name: "Confirmar decisión", exact: true }),
    ).toHaveCount(0);
    const blobAudit = await page.evaluate(
      () =>
        (
          window as typeof window & {
            __privateBlobAudit?: { created: string[]; revoked: string[] };
          }
        ).__privateBlobAudit ?? { created: [], revoked: [] },
    );
    expect(blobAudit.created).toHaveLength(1);
    expect(blobAudit.revoked).toContain(blobAudit.created[0]);
    expect(
      model.requests.filter(
        (request) => request.method === "GET" && request.action === "content",
      ),
    ).toHaveLength(1);
  });
}

test("reviewer lists minimized assignment and previews only a clean file", async ({
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
  await page
    .getByRole("listitem")
    .filter({ hasText: "Evidencia · anverso" })
    .getByRole("button", { name: "Abrir archivo limpio", exact: true })
    .click();
  await expect(
    page.getByRole("dialog", { name: "Visor privado de evidencia" }),
  ).toBeVisible();
  expect(
    requests.some((x) =>
      x.endsWith(`/cases/${caseId}/files/${fileId}/content`),
    ),
  ).toBeTruthy();
});
test("session change revokes the prior reviewer private evidence blob", async ({
  page,
}) => {
  const ids = {
    accountA: "150e8400-e29b-41d4-a716-446655440001",
    accountB: "150e8400-e29b-41d4-a716-446655440002",
    caseA: "260e8400-e29b-41d4-a716-446655440001",
    caseB: "260e8400-e29b-41d4-a716-446655440002",
    fileA: "370e8400-e29b-41d4-a716-446655440001",
    fileB: "370e8400-e29b-41d4-a716-446655440002",
  };
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const accountCases = {
    [ids.accountA]: {
      ...assigned,
      id: ids.caseA,
      files: [{ ...assigned.files[0], id: ids.fileA }],
    },
    [ids.accountB]: {
      ...assigned,
      id: ids.caseB,
      files: [{ ...assigned.files[0], id: ids.fileB }],
    },
  };
  const model = {
    active: ids.accountA,
    sessions: [] as string[],
    lists: [] as string[],
    requests: [] as Array<{ account: string; method: string; path: string }>,
  };
  const providerRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== localOrigin)
      providerRequests.push(request.url());
  });
  await page.route("**/api/backend/auth/session", (route) => {
    model.sessions.push(model.active);
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: model.active,
        roles: [],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    });
  });
  await page.route(
    "**/api/backend/document-verification/reviewer/**",
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const caseKey = path.split("/cases/")[1]?.split("/")[0];
      const account = model.active;
      const method = request.method();
      model.requests.push({ account, method, path });
      if (method === "GET" && path.endsWith("/cases")) {
        model.lists.push(account);
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: [accountCases[account]] }),
        });
      }
      if (caseKey && caseKey !== accountCases[account].id)
        return route.fulfill({ status: 404 });
      if (path.endsWith("/content"))
        return route.fulfill({
          status: 200,
          contentType: "image/png",
          body: Buffer.from(
            "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nm8AAAAASUVORK5CYII=",
            "base64",
          ),
        });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...accountCases[account],
          identity_declaration: declaration,
        }),
      });
    },
  );
  await page.goto("/mi-cuenta/revision-documental");
  await expect(
    page.locator(`[data-review-case-id="${ids.caseA}"]`),
  ).toBeVisible();
  await page.locator(`[data-review-case-id="${ids.caseA}"]`).click();
  await expect(
    page.getByRole("heading", { name: "Expediente · Identidad" }),
  ).toBeVisible();
  await page.evaluate(() => {
    const target = window as typeof window & { __revokedBlobs?: string[] };
    target.__revokedBlobs = [];
    const original = URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL = (url) => {
      target.__revokedBlobs!.push(url);
      original(url);
    };
  });
  await page.locator(`[data-review-file-id="${ids.fileA}"]`).click();
  const viewer = page.getByRole("dialog", {
    name: "Visor privado de evidencia",
  });
  await expect(viewer).toBeVisible();
  const image = page.getByRole("img", { name: "Evidencia privada: anverso" });
  await expect(image).toBeVisible();
  const blobUrl = await image.getAttribute("src");
  expect(blobUrl).toMatch(/^blob:/);

  model.active = ids.accountB;
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  await expect.poll(() => model.sessions).toContain(ids.accountB);
  await expect
    .poll(() => model.lists.filter((id) => id === ids.accountB))
    .toHaveLength(1);
  await expect(viewer).toHaveCount(0);
  await expect(image).toHaveCount(0);
  const revoked = await page.evaluate(
    () =>
      (window as typeof window & { __revokedBlobs?: string[] }).__revokedBlobs,
  );
  expect(revoked).toContain(blobUrl);
  await expect(
    page.locator(`[data-review-case-id="${ids.caseA}"]`),
  ).toHaveCount(0);
  const caseBRow = page.locator(`[data-review-case-id="${ids.caseB}"]`);
  await expect(caseBRow).toBeVisible();
  await caseBRow.click();
  await expect(
    page.getByRole("heading", { name: "Expediente · Identidad" }),
  ).toBeVisible();
  await expect(
    page.locator(`[data-review-case-id="${ids.caseB}"]`),
  ).toBeVisible();
  expect(model.lists.filter((id) => id === ids.accountA)).toHaveLength(1);
  expect(model.lists.filter((id) => id === ids.accountB)).toHaveLength(1);
  expect(
    model.requests
      .filter((r) => r.account === ids.accountB)
      .some((r) => r.path.includes(ids.caseA) || r.path.includes(ids.fileA)),
  ).toBe(false);
  expect(model.requests.filter((r) => r.method === "POST")).toHaveLength(0);
  expect(providerRequests).toEqual([]);
});
test("late reviewer detail failure after session change cannot expose prior account data", async ({
  page,
}) => {
  const ids = {
    accountA: "150e8400-e29b-41d4-a716-446655440001",
    accountB: "150e8400-e29b-41d4-a716-446655440002",
    caseA: "260e8400-e29b-41d4-a716-446655440001",
    caseB: "260e8400-e29b-41d4-a716-446655440002",
    fileA: "370e8400-e29b-41d4-a716-446655440001",
    fileB: "370e8400-e29b-41d4-a716-446655440002",
  };
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const accountCases = {
    [ids.accountA]: {
      ...assigned,
      id: ids.caseA,
      files: [{ ...assigned.files[0], id: ids.fileA }],
    },
    [ids.accountB]: {
      ...assigned,
      id: ids.caseB,
      files: [{ ...assigned.files[0], id: ids.fileB }],
    },
  };
  const model = {
    active: ids.accountA,
    sessions: [] as string[],
    lists: [] as string[],
    requests: [] as Array<{ account: string; method: string; path: string }>,
    lateRouteResult: null as string | null,
    detailHold: null as { reject: () => Promise<void> } | null,
    heldDispatch: null as {
      account: string;
      method: string;
      path: string;
    } | null,
  };
  let armDetailHold = false;
  let interceptedADetailResolve!: () => void;
  const interceptedADetail = new Promise<void>((resolve) => {
    interceptedADetailResolve = resolve;
  });
  const providerRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== localOrigin)
      providerRequests.push(request.url());
  });
  await page.route("**/api/backend/auth/session", (route) => {
    model.sessions.push(model.active);
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: model.active,
        roles: [],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    });
  });
  await page.route(
    "**/api/backend/document-verification/reviewer/**",
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const caseKey = path.split("/cases/")[1]?.split("/")[0];
      const account = model.active;
      const method = request.method();
      model.requests.push({ account, method, path });
      if (
        armDetailHold &&
        account === ids.accountA &&
        method === "GET" &&
        path ===
          `/api/backend/document-verification/reviewer/cases/${ids.caseA}`
      ) {
        model.heldDispatch = { account, method, path };
        interceptedADetailResolve();
        await new Promise<void>((resolve) => {
          model.detailHold = {
            reject: async () => {
              resolve();
              await route.fulfill({
                status: 503,
                contentType: "application/json",
                body: JSON.stringify({ error: "synthetic delayed rejection" }),
              });
              model.lateRouteResult = "fulfilled HTTP 503";
            },
          };
        });
        return;
      }
      if (method === "GET" && path.endsWith("/cases")) {
        model.lists.push(account);
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: [accountCases[account]] }),
        });
      }
      if (caseKey && caseKey !== accountCases[account].id)
        return route.fulfill({ status: 404 });
      if (path.endsWith("/content"))
        return route.fulfill({
          status: 200,
          contentType: "image/png",
          body: Buffer.from(
            "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nm8AAAAASUVORK5CYII=",
            "base64",
          ),
        });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...accountCases[account],
          identity_declaration: {
            ...declaration,
            first_name:
              account === ids.accountA ? "Synthetic A" : "Synthetic B",
          },
        }),
      });
    },
  );
  await page.goto("/mi-cuenta/revision-documental");
  await expect(
    page.locator(`[data-review-case-id="${ids.caseA}"]`),
  ).toBeVisible();
  await page.locator(`[data-review-case-id="${ids.caseA}"]`).click();
  await expect(
    page.getByRole("heading", { name: "Expediente · Identidad" }),
  ).toBeVisible();
  await expect(
    page.getByText("Synthetic A Person", { exact: true }),
  ).toBeVisible();
  armDetailHold = true;
  await page.locator(`[data-review-file-id="${ids.fileA}"]`).click();
  await interceptedADetail;
  await expect.poll(() => Boolean(model.detailHold)).toBe(true);
  expect(model.heldDispatch).toEqual({
    account: ids.accountA,
    method: "GET",
    path: `/api/backend/document-verification/reviewer/cases/${ids.caseA}`,
  });

  model.active = ids.accountB;
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  await expect.poll(() => model.sessions).toContain(ids.accountB);
  await expect
    .poll(() => model.lists.filter((id) => id === ids.accountB))
    .toHaveLength(1);
  await expect(
    page.getByText("Synthetic A Person", { exact: true }),
  ).toHaveCount(0);
  await model.detailHold!.reject();
  expect(model.lateRouteResult).toBe("fulfilled HTTP 503");
  await expect(
    page.getByText("Synthetic A Person", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    model.requests.some((request) => request.path.endsWith("/content")),
  ).toBe(false);
  await expect(
    page.locator(`[data-review-case-id="${ids.caseA}"]`),
  ).toHaveCount(0);
  const caseBRow = page.locator(`[data-review-case-id="${ids.caseB}"]`);
  await expect(caseBRow).toBeVisible();
  await caseBRow.click();
  await expect(
    page.getByRole("heading", { name: "Expediente · Identidad" }),
  ).toBeVisible();
  await expect(
    page.getByText("Synthetic B Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.locator(`[data-review-case-id="${ids.caseB}"]`),
  ).toBeVisible();
  expect(model.lists.filter((id) => id === ids.accountA)).toHaveLength(1);
  expect(model.lists.filter((id) => id === ids.accountB)).toHaveLength(1);
  expect(
    model.requests
      .filter((r) => r.account === ids.accountB)
      .some((r) => r.path.includes(ids.caseA) || r.path.includes(ids.fileA)),
  ).toBe(false);
  expect(model.requests.filter((r) => r.method === "POST")).toHaveLength(0);
  expect(providerRequests).toEqual([]);
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toHaveCount(0);
});

test("same-account selection fences a held A preview metadata response", async ({
  page,
}) => {
  const ids = {
    caseA: "260e8400-e29b-41d4-a716-446655440011",
    caseB: "260e8400-e29b-41d4-a716-446655440012",
    fileA: "370e8400-e29b-41d4-a716-446655440011",
    fileB: "370e8400-e29b-41d4-a716-446655440012",
  };
  const account = "150e8400-e29b-41d4-a716-446655440011";
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const cases = [
    {
      ...assigned,
      id: ids.caseA,
      files: [{ ...assigned.files[0], id: ids.fileA }],
    },
    {
      ...assigned,
      id: ids.caseB,
      files: [{ ...assigned.files[0], id: ids.fileB }],
    },
  ];
  const declarations = new Map([
    [ids.caseA, { ...declaration, first_name: "Unique Case A Declaration" }],
    [ids.caseB, { ...declaration, first_name: "Unique Case B Declaration" }],
  ]);
  const model = {
    requests: [] as Array<{ account: string; method: string; path: string }>,
    lists: 0,
    heldDispatch: null as {
      account: string;
      method: string;
      path: string;
    } | null,
    releaseA: null as (() => Promise<void>) | null,
    fulfilled503: false,
  };
  let armed = false;
  let interceptedResolve!: () => void;
  const intercepted = new Promise<void>((resolve) => {
    interceptedResolve = resolve;
  });
  const external: string[] = [];
  page.on("request", (req) => {
    if (new URL(req.url()).origin !== localOrigin) external.push(req.url());
  });
  await page.route("**/api/backend/auth/session", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: account,
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
      const req = route.request();
      const path = new URL(req.url()).pathname;
      const method = req.method();
      model.requests.push({ account, method, path });
      const caseId = path.split("/cases/")[1]?.split("/")[0];
      if (
        armed &&
        caseId === ids.caseA &&
        method === "GET" &&
        path ===
          `/api/backend/document-verification/reviewer/cases/${ids.caseA}`
      ) {
        model.heldDispatch = { account, method, path };
        interceptedResolve();
        await new Promise<void>((resolve) => {
          model.releaseA = async () => {
            resolve();
            await route.fulfill({
              status: 503,
              contentType: "application/json",
              body: JSON.stringify({ error: "synthetic delayed rejection" }),
            });
            model.fulfilled503 = true;
          };
        });
        return;
      }
      if (method === "GET" && path.endsWith("/cases")) {
        model.lists++;
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: cases }),
        });
      }
      const current = cases.find((item) => item.id === caseId);
      if (!current) return route.fulfill({ status: 404 });
      if (path.endsWith("/content")) return route.fulfill({ status: 500 });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...current,
          identity_declaration: declarations.get(caseId),
        }),
      });
    },
  );

  await page.goto("/mi-cuenta/revision-documental");
  await expect(
    page.locator(`[data-review-case-id="${ids.caseA}"]`),
  ).toBeVisible();
  await expect(
    page.locator(`[data-review-case-id="${ids.caseB}"]`),
  ).toBeVisible();
  await page.locator(`[data-review-case-id="${ids.caseA}"]`).click();
  await expect(
    page.getByText("Unique Case A Declaration Person", { exact: true }),
  ).toBeVisible();

  armed = true;
  await page.locator(`[data-review-file-id="${ids.fileA}"]`).click();
  await intercepted;
  expect(model.heldDispatch).toEqual({
    account,
    method: "GET",
    path: `/api/backend/document-verification/reviewer/cases/${ids.caseA}`,
  });
  expect(
    model.requests.filter(
      (r) => r.method === "GET" && r.path.endsWith("/cases"),
    ).length,
  ).toBe(1);

  const caseBRow = page.locator(`[data-review-case-id="${ids.caseB}"]`);
  await expect(caseBRow).toBeEnabled();
  await caseBRow.click();
  await expect(
    page.getByText("Unique Case B Declaration Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Unique Case A Declaration Person", { exact: true }),
  ).toHaveCount(0);
  const aDetailPath = `/api/backend/document-verification/reviewer/cases/${ids.caseA}`;
  const heldAResponse = page.waitForResponse((response) => {
    const responseRequest = response.request();
    return (
      new URL(response.url()).pathname === aDetailPath &&
      responseRequest.method() === "GET" &&
      response.status() === 503
    );
  });
  await model.releaseA!();
  const aResponse = await heldAResponse;
  await aResponse.finished();
  expect(aResponse.request().method()).toBe("GET");
  expect(new URL(aResponse.url()).pathname).toBe(aDetailPath);
  expect(aResponse.status()).toBe(503);
  expect(model.fulfilled503).toBe(true);
  await page.waitForTimeout(0);
  await expect(
    page.getByText("Unique Case A Declaration Person", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText("Unique Case B Declaration Person", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toHaveCount(0);
  expect(
    model.requests.filter(
      (r) => r.method === "GET" && /\/cases\/[^/]+$/.test(r.path),
    ),
  ).toHaveLength(3);
  expect(model.requests.some((r) => r.path.endsWith("/content"))).toBe(false);
  expect(model.requests.filter((r) => r.method === "POST")).toHaveLength(0);
  expect(external).toEqual([]);
});

test("late successful A preview metadata cannot replace B within the same reviewer account", async ({
  page,
}) => {
  const ids = {
    account: "150e8400-e29b-41d4-a716-446655440021",
    caseA: "260e8400-e29b-41d4-a716-446655440021",
    caseB: "260e8400-e29b-41d4-a716-446655440022",
    fileA: "370e8400-e29b-41d4-a716-446655440021",
    fileB: "370e8400-e29b-41d4-a716-446655440022",
  };
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const cases = [
    {
      ...assigned,
      id: ids.caseA,
      files: [{ ...assigned.files[0], id: ids.fileA }],
    },
    {
      ...assigned,
      id: ids.caseB,
      files: [{ ...assigned.files[0], id: ids.fileB }],
    },
  ];
  const names = new Map([
    [ids.caseA, "Late Success A Declaration"],
    [ids.caseB, "Late Success B Declaration"],
  ]);
  const aDetailPath = `/api/backend/document-verification/reviewer/cases/${ids.caseA}`;
  const model = {
    requests: [] as Array<{ account: string; method: string; path: string }>,
    lists: 0,
    heldDispatch: null as {
      account: string;
      method: string;
      path: string;
    } | null,
    releaseA: null as (() => Promise<void>) | null,
    routeFulfilled: false,
  };
  let armed = false;
  let interceptedResolve!: () => void;
  const intercepted = new Promise<void>((resolve) => {
    interceptedResolve = resolve;
  });
  const external: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== localOrigin)
      external.push(request.url());
  });
  await page.route("**/api/backend/auth/session", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: ids.account,
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
      const path = new URL(request.url()).pathname;
      const method = request.method();
      model.requests.push({ account: ids.account, method, path });
      if (armed && method === "GET" && path === aDetailPath) {
        model.heldDispatch = { account: ids.account, method, path };
        interceptedResolve();
        await new Promise<void>((resolve) => {
          model.releaseA = async () => {
            resolve();
            await route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({
                ...cases[0],
                identity_declaration: {
                  ...declaration,
                  first_name: names.get(ids.caseA),
                },
              }),
            });
            model.routeFulfilled = true;
          };
        });
        return;
      }
      if (method === "GET" && path.endsWith("/cases")) {
        model.lists++;
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: cases }),
        });
      }
      const caseId = path.split("/cases/")[1]?.split("/")[0];
      const current = cases.find((item) => item.id === caseId);
      if (!current) return route.fulfill({ status: 404 });
      if (path.endsWith("/content")) return route.fulfill({ status: 500 });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...current,
          identity_declaration: {
            ...declaration,
            first_name: names.get(caseId),
          },
        }),
      });
    },
  );

  await page.goto("/mi-cuenta/revision-documental");
  await expect(
    page.locator(`[data-review-case-id="${ids.caseA}"]`),
  ).toBeVisible();
  await expect(
    page.locator(`[data-review-case-id="${ids.caseB}"]`),
  ).toBeVisible();
  await page.locator(`[data-review-case-id="${ids.caseA}"]`).click();
  await expect(
    page.getByText("Late Success A Declaration Person", { exact: true }),
  ).toBeVisible();

  armed = true;
  await page.locator(`[data-review-file-id="${ids.fileA}"]`).click();
  await intercepted;
  expect(model.heldDispatch).toEqual({
    account: ids.account,
    method: "GET",
    path: aDetailPath,
  });
  expect(
    model.requests.filter(
      (item) => item.method === "GET" && item.path.endsWith("/cases"),
    ),
  ).toHaveLength(1);

  const rowB = page.locator(`[data-review-case-id="${ids.caseB}"]`);
  await expect(rowB).toBeEnabled();
  await rowB.click();
  await expect(
    page.getByText("Late Success B Declaration Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Late Success A Declaration Person", { exact: true }),
  ).toHaveCount(0);

  const aResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === aDetailPath &&
      response.request().method() === "GET" &&
      response.status() === 200,
  );
  await model.releaseA!();
  const aResponse = await aResponsePromise;
  await aResponse.finished();
  expect(aResponse.request().method()).toBe("GET");
  expect(new URL(aResponse.url()).pathname).toBe(aDetailPath);
  expect(aResponse.status()).toBe(200);
  expect(model.routeFulfilled).toBe(true);
  await page.waitForTimeout(0);

  await expect(
    page.getByText("Late Success A Declaration Person", { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText("Late Success B Declaration Person", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toHaveCount(0);
  expect(model.lists).toBe(1);
  expect(
    model.requests.filter(
      (item) => item.method === "GET" && /\/cases\/[^/]+$/.test(item.path),
    ),
  ).toHaveLength(3);
  expect(model.requests.some((item) => item.path.endsWith("/content"))).toBe(
    false,
  );
  expect(model.requests.filter((item) => item.method === "POST")).toHaveLength(
    0,
  );
  expect(model.requests.every((item) => item.account === ids.account)).toBe(
    true,
  );
  expect(external).toEqual([]);
});

test("late successful A preview metadata after account switch cannot expose A in B", async ({
  page,
}) => {
  const ids = {
    accountA: "150e8400-e29b-41d4-a716-446655440031",
    accountB: "150e8400-e29b-41d4-a716-446655440032",
    caseA: "260e8400-e29b-41d4-a716-446655440031",
    caseB: "260e8400-e29b-41d4-a716-446655440032",
    fileA: "370e8400-e29b-41d4-a716-446655440031",
    fileB: "370e8400-e29b-41d4-a716-446655440032",
  };
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const fixtures = {
    [ids.accountA]: {
      ...assigned,
      id: ids.caseA,
      files: [{ ...assigned.files[0], id: ids.fileA }],
    },
    [ids.accountB]: {
      ...assigned,
      id: ids.caseB,
      files: [{ ...assigned.files[0], id: ids.fileB }],
    },
  };
  const declarations = {
    [ids.accountA]: { ...declaration, first_name: "Unique Cross Account A" },
    [ids.accountB]: { ...declaration, first_name: "Unique Cross Account B" },
  };
  const aPath = `/api/backend/document-verification/reviewer/cases/${ids.caseA}`;
  const model = {
    active: ids.accountA,
    sessions: [] as string[],
    lists: [] as string[],
    requests: [] as Array<{ account: string; method: string; path: string }>,
    held: null as { account: string; method: string; path: string } | null,
    release: null as (() => Promise<void>) | null,
    completed: false,
  };
  let armed = false;
  let interceptedResolve!: () => void;
  const intercepted = new Promise<void>((resolve) => {
    interceptedResolve = resolve;
  });
  const external: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== localOrigin)
      external.push(request.url());
  });
  await page.route("**/api/backend/auth/session", (route) => {
    model.sessions.push(model.active);
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: model.active,
        roles: [],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    });
  });
  await page.route(
    "**/api/backend/document-verification/reviewer/**",
    async (route) => {
      const request = route.request(),
        path = new URL(request.url()).pathname,
        method = request.method();
      const account = model.active;
      model.requests.push({ account, method, path });
      if (method === "GET" && path.endsWith("/cases")) {
        model.lists.push(account);
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: [fixtures[account]] }),
        });
      }
      if (
        armed &&
        account === ids.accountA &&
        method === "GET" &&
        path === aPath
      ) {
        model.held = { account, method, path };
        interceptedResolve();
        await new Promise<void>((resolve) => {
          model.release = async () => {
            resolve();
            await route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({
                ...fixtures[ids.accountA],
                identity_declaration: declarations[ids.accountA],
              }),
            });
            model.completed = true;
          };
        });
        return;
      }
      const current = fixtures[account],
        caseId = path.split("/cases/")[1]?.split("/")[0];
      if (caseId && caseId !== current.id)
        return route.fulfill({ status: 404 });
      if (path.endsWith("/content")) return route.fulfill({ status: 500 });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...current,
          identity_declaration: declarations[account],
        }),
      });
    },
  );

  await page.goto("/mi-cuenta/revision-documental");
  const rowA = page.locator(`[data-review-case-id="${ids.caseA}"]`);
  await expect(rowA).toBeVisible();
  await rowA.click();
  await expect(
    page.getByText("Unique Cross Account A Person", { exact: true }),
  ).toBeVisible();
  armed = true;
  await page.locator(`[data-review-file-id="${ids.fileA}"]`).click();
  await intercepted;
  expect(model.held).toEqual({
    account: ids.accountA,
    method: "GET",
    path: aPath,
  });

  model.active = ids.accountB;
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  await expect.poll(() => model.sessions).toContain(ids.accountB);
  await expect
    .poll(() => model.lists.filter((id) => id === ids.accountB))
    .toHaveLength(1);
  await expect(rowA).toHaveCount(0);
  const rowB = page.locator(`[data-review-case-id="${ids.caseB}"]`);
  await expect(rowB).toBeVisible();
  await rowB.click();
  await expect(
    page.getByText("Unique Cross Account B Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Unique Cross Account A Person", { exact: true }),
  ).toHaveCount(0);

  const responsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === aPath &&
      response.request().method() === "GET" &&
      response.status() === 200,
  );
  await model.release!();
  const response = await responsePromise;
  await response.finished();
  expect(response.request().method()).toBe("GET");
  expect(new URL(response.url()).pathname).toBe(aPath);
  expect(response.status()).toBe(200);
  expect(model.completed).toBe(true);
  await page.waitForTimeout(0);

  await expect(rowB).toBeVisible();
  await expect(rowA).toHaveCount(0);
  await expect(
    page.getByText("Unique Cross Account B Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Unique Cross Account A Person", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toHaveCount(0);
  expect(model.lists.filter((id) => id === ids.accountA)).toHaveLength(1);
  expect(model.lists.filter((id) => id === ids.accountB)).toHaveLength(1);
  const details = model.requests.filter(
    (item) => item.method === "GET" && /\/cases\/[^/]+$/.test(item.path),
  );
  expect(
    details.filter(
      (item) => item.account === ids.accountA && item.path === aPath,
    ),
  ).toHaveLength(2);
  expect(
    details.filter(
      (item) => item.account === ids.accountB && item.path.endsWith(ids.caseB),
    ),
  ).toHaveLength(1);
  expect(details).toHaveLength(3);
  expect(
    model.requests.filter(
      (item) =>
        item.account === ids.accountB &&
        (item.path.includes(ids.caseA) || item.path.includes(ids.fileA)),
    ),
  ).toHaveLength(0);
  expect(model.requests.some((item) => item.path.endsWith("/content"))).toBe(
    false,
  );
  expect(model.requests.filter((item) => item.method === "POST")).toHaveLength(
    0,
  );
  expect(external).toEqual([]);
});

test("successful review decision POST dispatched under A cannot replace B after session change", async ({
  page,
}) => {
  // Mock/UI-only characterization; this does not assert backend authorization.
  const ids = {
    accountA: "150e8400-e29b-41d4-a716-446655440041",
    accountB: "150e8400-e29b-41d4-a716-446655440042",
    caseA: "260e8400-e29b-41d4-a716-446655440041",
    caseB: "260e8400-e29b-41d4-a716-446655440042",
    fileA: "370e8400-e29b-41d4-a716-446655440041",
    fileB: "370e8400-e29b-41d4-a716-446655440042",
  };
  const expiresAt = new Date(Date.now() + 600_000).toISOString();
  const localOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const cases = {
    [ids.accountA]: {
      ...assigned,
      id: ids.caseA,
      files: [{ ...assigned.files[0], id: ids.fileA }],
    },
    [ids.accountB]: {
      ...assigned,
      id: ids.caseB,
      files: [{ ...assigned.files[0], id: ids.fileB }],
    },
  };
  const declarations = {
    [ids.accountA]: {
      ...declaration,
      first_name: "Declaration Account Forty One",
    },
    [ids.accountB]: {
      ...declaration,
      first_name: "Declaration Account Forty Two",
    },
  };
  const reviewPath = `/api/backend/document-verification/reviewer/cases/${ids.caseA}/review`;
  const model = {
    active: ids.accountA,
    sessions: [] as string[],
    lists: [] as string[],
    requests: [] as Array<{ account: string; method: string; path: string }>,
    heldDispatch: null as {
      account: string;
      method: string;
      path: string;
    } | null,
    payload: null as Record<string, unknown> | null,
    release: null as (() => Promise<void>) | null,
    completed: false,
  };
  let armed = false;
  let enteredResolve!: () => void;
  const entered = new Promise<void>((resolve) => {
    enteredResolve = resolve;
  });
  const external: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== localOrigin)
      external.push(request.url());
  });
  await page.route("**/api/backend/auth/session", (route) => {
    model.sessions.push(model.active);
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        account_id: model.active,
        roles: [],
        capabilities: [],
        restricted_mode: false,
        expires_at: expiresAt,
      }),
    });
  });
  await page.route(
    "**/api/backend/document-verification/reviewer/**",
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const method = request.method();
      const account = model.active;
      model.requests.push({ account, method, path });
      if (method === "GET" && path.endsWith("/cases")) {
        model.lists.push(account);
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ items: [cases[account]] }),
        });
      }
      if (
        armed &&
        account === ids.accountA &&
        method === "POST" &&
        path === reviewPath
      ) {
        const body = request.postDataJSON() as Record<string, unknown>;
        expect(model.heldDispatch).toBeNull();
        model.heldDispatch = { account, method, path };
        expect(body.expected_revision).toBe(cases[ids.accountA].revision);
        expect(body.capability).toBe("review_identity");
        expect(body.decision).toBe("approve");
        expect(body.document_format_observed).toBe("new");
        expect(body.document_expires_at_observed).toBeTruthy();
        expect(body.verified_until).toBeTruthy();
        expect(body.identity_review_checklist).toEqual({
          names_match: true,
          rut_matches: true,
          birth_date_and_majority_match: true,
          portrait_matches_selfie: true,
          evidence_legible: true,
          document_unexpired: true,
          issue_date_and_format_verified: true,
        });
        model.payload = body;
        enteredResolve();
        await new Promise<void>((resolve) => {
          model.release = async () => {
            resolve();
            await route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({
                ...cases[ids.accountA],
                state: "aprobada",
                revision: 2,
                document_format_observed: body.document_format_observed,
                document_expires_at_observed: body.document_expires_at_observed,
                identity_declaration: declarations[ids.accountA],
              }),
            });
            model.completed = true;
          };
        });
        return;
      }
      const caseId = path.split("/cases/")[1]?.split("/")[0];
      const current = cases[account];
      if (caseId && caseId !== current.id)
        return route.fulfill({ status: 404 });
      if (path.endsWith("/content")) return route.fulfill({ status: 500 });
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...current,
          identity_declaration: declarations[account],
        }),
      });
    },
  );

  await page.goto("/mi-cuenta/revision-documental");
  const rowA = page.locator(`[data-review-case-id="${ids.caseA}"]`);
  await expect(rowA).toBeVisible();
  expect(
    model.lists.filter((account) => account === ids.accountA),
  ).toHaveLength(1);
  await rowA.click();
  await expect(
    page.getByText("Declaration Account Forty One Person", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("combobox", { name: "Decisión", exact: true })
    .selectOption("approve");
  await completeIdentityApproval(page);
  armed = true;
  await page
    .getByRole("button", { name: "Confirmar decisión", exact: true })
    .click();
  await entered;
  expect(model.heldDispatch).toEqual({
    account: ids.accountA,
    method: "POST",
    path: reviewPath,
  });
  expect(model.payload).not.toBeNull();

  model.active = ids.accountB;
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  await expect.poll(() => model.sessions).toContain(ids.accountB);
  await expect
    .poll(() => model.lists.filter((account) => account === ids.accountB))
    .toHaveLength(1);
  await expect(rowA).toHaveCount(0);
  const rowB = page.locator(`[data-review-case-id="${ids.caseB}"]`);
  await expect(rowB).toBeVisible();
  await rowB.click();
  await expect(
    page.getByText("Declaration Account Forty Two Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Declaration Account Forty One Person", { exact: true }),
  ).toHaveCount(0);

  const responsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === reviewPath &&
      response.request().method() === "POST" &&
      response.status() === 200,
  );
  await model.release!();
  const response = await responsePromise;
  await response.finished();
  expect(response.request().method()).toBe("POST");
  expect(new URL(response.url()).pathname).toBe(reviewPath);
  expect(response.status()).toBe(200);
  expect(model.completed).toBe(true);
  await page.waitForTimeout(0);
  await expect(rowB).toBeVisible();
  await expect(rowB).toBeEnabled();
  await expect(
    page.getByText("Declaration Account Forty Two Person", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Declaration Account Forty One Person", { exact: true }),
  ).toHaveCount(0);
  await expect(rowA).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page
      .getByRole("region", { name: "Contenido de la sección de cuenta" })
      .getByRole("alert"),
  ).toHaveCount(0);
  expect(
    model.lists.filter((account) => account === ids.accountA),
  ).toHaveLength(1);
  expect(
    model.lists.filter((account) => account === ids.accountB),
  ).toHaveLength(1);
  const details = model.requests.filter(
    (item) => item.method === "GET" && /\/cases\/[^/]+$/.test(item.path),
  );
  expect(
    details.filter(
      (item) => item.account === ids.accountA && item.path.endsWith(ids.caseA),
    ),
  ).toHaveLength(1);
  expect(
    details.filter(
      (item) => item.account === ids.accountB && item.path.endsWith(ids.caseB),
    ),
  ).toHaveLength(1);
  expect(details).toHaveLength(2);
  expect(
    model.requests.filter(
      (item) =>
        item.account === ids.accountB &&
        (item.path.includes(ids.caseA) || item.path.includes(ids.fileA)),
    ),
  ).toHaveLength(0);
  expect(model.requests.some((item) => item.path.endsWith("/content"))).toBe(
    false,
  );
  expect(model.requests.filter((item) => item.method === "POST")).toEqual([
    { account: ids.accountA, method: "POST", path: reviewPath },
  ]);
  expect(external).toEqual([]);
});
