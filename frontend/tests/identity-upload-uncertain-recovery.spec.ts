import { expect, test } from "@playwright/test";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const sessionToken = "synthetic-document-session-token";
const evidenceRoles = [
  "identity_front",
  "identity_back",
  "identity_selfie",
] as const;

test.use({ trace: "off", screenshot: "off", video: "off" });

test("uncertain identity upload clears the retained File across a non-editable readback", async ({
  page,
}) => {
  let loggedIn = false;
  let caseState: "borrador" | "en_revision" | "requiere_antecedentes" =
    "borrador";
  let revision = 1;
  let etag = '"1"';
  let uploadCount = 0;
  let postUploadListReads = 0;

  const files = evidenceRoles.map((evidence_role, index) => ({
    id: `880e8400-e29b-41d4-a716-44665544000${index + 1}`,
    evidence_role,
    state: "limpio",
    size: 128,
    mime: "image/png",
    pages: 1,
  }));

  function snapshot() {
    return {
      id: caseId,
      purpose: "identity",
      state: caseState,
      revision,
      document_format_declared: "unsure",
      files,
    };
  }

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (
      status: number,
      value: unknown,
      headers: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
          ...headers,
        },
        body: JSON.stringify(value),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          })
        : json(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        access_token: sessionToken,
        token_type: "Bearer",
        expires_at: new Date(Date.now() + 60_000).toISOString(),
        restricted_mode: false,
      });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (uploadCount > 0) {
        postUploadListReads += 1;
        if (postUploadListReads === 1) {
          caseState = "en_revision";
          revision = 2;
          etag = '"2"';
        } else if (postUploadListReads === 2) {
          caseState = "requiere_antecedentes";
          revision = 3;
          etag = '"3"';
        }
      }
      return json(200, [snapshot()]);
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      return json(200, snapshot(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      uploadCount += 1;
      expect(request.headers()["if-match"]).toBe('"1"');
      return json(409, { error: { code: "etag_conflict" } }, { etag });
    }
    return json(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fdocumentos");
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  await page
    .getByRole("button", { name: /Identidad.*revisión 1.*Borrador/ })
    .click();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();

  await page.getByLabel("Tipo de evidencia").selectOption("identity_front");
  await page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-identity-front.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic identity evidence bytes"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText(
      "El expediente cambió desde la última consulta. Actualízalo antes de reintentar.",
    ),
  ).toBeVisible();
  expect(uploadCount).toBe(1);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · En revisión"),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toHaveCount(0);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · Requiere antecedentes"),
  ).toBeVisible();
  const fileInput = page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB");
  await expect(fileInput).toBeVisible();
  await expect
    .poll(() =>
      fileInput.evaluate((input: HTMLInputElement) => input.files?.length ?? 0),
    )
    .toBe(0);
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeDisabled();
  expect(uploadCount).toBe(1);
});

test("editable identity readback preserves the File until an explicit retry", async ({
  page,
}) => {
  let loggedIn = false;
  let caseState: "borrador" | "requiere_antecedentes" = "borrador";
  let revision = 1;
  let etag = '"1"';
  let uploadCount = 0;
  let postUploadListReads = 0;
  const detailReadEtags: string[] = [];
  const files = evidenceRoles.map((evidence_role, index) => ({
    id: `990e8400-e29b-41d4-a716-44665544000${index + 1}`,
    evidence_role,
    state: "limpio",
    size: 128,
    mime: "image/png",
    pages: 1,
  }));

  function snapshot() {
    return {
      id: caseId,
      purpose: "identity",
      state: caseState,
      revision,
      document_format_declared: "unsure",
      files,
    };
  }

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (
      status: number,
      value: unknown,
      headers: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
          ...headers,
        },
        body: JSON.stringify(value),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          })
        : json(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        access_token: sessionToken,
        token_type: "Bearer",
        expires_at: new Date(Date.now() + 60_000).toISOString(),
        restricted_mode: false,
      });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (uploadCount > 0) {
        postUploadListReads += 1;
        if (postUploadListReads === 1) {
          caseState = "requiere_antecedentes";
          revision = 2;
          etag = '"2"';
        }
      }
      return json(200, [snapshot()]);
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      detailReadEtags.push(etag);
      return json(200, snapshot(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      uploadCount += 1;
      const body = request.postDataBuffer()?.toString("latin1") ?? "";
      expect(body).toContain('name="evidence_role"\r\n\r\nidentity_front');
      expect(body).toContain('filename="synthetic-identity-front.png"');
      expect(body).toContain("synthetic identity evidence bytes");
      if (uploadCount === 1) {
        expect(request.headers()["if-match"]).toBe('"1"');
        return json(409, { error: { code: "etag_conflict" } }, { etag });
      }

      expect(uploadCount).toBe(2);
      expect(request.headers()["if-match"]).toBe('"2"');
      revision = 3;
      etag = '"3"';
      return json(
        201,
        {
          revision,
          file: {
            id: "990e8400-e29b-41d4-a716-446655440004",
            evidence_role: "identity_front",
            state: "pendiente",
            size: Buffer.byteLength("synthetic identity evidence bytes"),
            mime: "image/png",
            pages: 1,
          },
        },
        { etag },
      );
    }
    return json(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fdocumentos");
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  await page
    .getByRole("button", { name: /Identidad.*revisión 1.*Borrador/ })
    .click();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();

  await page.getByLabel("Tipo de evidencia").selectOption("identity_front");
  const fileInput = page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB");
  await fileInput.setInputFiles({
    name: "synthetic-identity-front.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic identity evidence bytes"),
  });
  const fileSelectionBeforeConflict = await fileInput.evaluate(
    async (input: HTMLInputElement) => {
      const file = input.files?.[0];
      if (!file) return null;
      return {
        name: file.name,
        size: file.size,
        type: file.type,
        lastModified: file.lastModified,
        text: await file.text(),
      };
    },
  );
  expect(fileSelectionBeforeConflict).not.toBeNull();
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText(
      "El expediente cambió desde la última consulta. Actualízalo antes de reintentar.",
    ),
  ).toBeVisible();
  expect(uploadCount).toBe(1);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · Requiere antecedentes"),
  ).toBeVisible();
  expect(detailReadEtags.at(-1)).toBe('"2"');
  expect(uploadCount).toBe(1);
  await expect
    .poll(() =>
      fileInput.evaluate(async (input: HTMLInputElement) => {
        const file = input.files?.[0];
        if (!file) return null;
        return {
          name: file.name,
          size: file.size,
          type: file.type,
          lastModified: file.lastModified,
          text: await file.text(),
        };
      }),
    )
    .toEqual(fileSelectionBeforeConflict);
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeEnabled();
  expect(uploadCount).toBe(1);
  const submitButton = page.getByRole("button", { name: "Enviar a revisión" });
  await expect(submitButton).toBeEnabled();

  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText("Evidencia recibida y pendiente de inspección."),
  ).toBeVisible();
  expect(uploadCount).toBe(2);
  await expect(submitButton).toHaveCount(0);
});
