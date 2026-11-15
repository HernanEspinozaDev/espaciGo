import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const spaceId = "660e8400-e29b-41d4-a716-446655440000";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const secondCaseId = "990e8400-e29b-41d4-a716-446655440000";
const fileId = "880e8400-e29b-41d4-a716-446655440000";
const backFileId = "aa0e8400-e29b-41d4-a716-446655440000";
const selfieFileId = "bb0e8400-e29b-41d4-a716-446655440000";
const duplicateBackFileId = "cc0e8400-e29b-41d4-a716-446655440000";
const identityEvidenceRoles = [
  "identity_front",
  "identity_back",
  "identity_selfie",
] as const;

test.use({ trace: "off", screenshot: "off", video: "off" });

test("owner completes the six document operations and downloads only a clean space file", async ({
  page,
}) => {
  const sessionExpiresAt = new Date(Date.now() + 60_000).toISOString();
  let loggedIn = false;
  let created = false;
  let uploaded = false;
  let failUploadOnce = true;
  let failCaseRefreshOnce = false;
  let cleanAfterRefresh = false;
  let omitCaseFromNextList = false;
  let caseState = "borrador";
  let fileState: "pendiente" | "limpio" = "pendiente";
  let revision = 1;
  let etag = '"1"';
  const observed: Array<{
    path: string;
    method: string;
    authorization?: string;
    ifMatch?: string;
  }> = [];

  function ownerCase() {
    return {
      id: caseId,
      purpose: "space",
      space_id: spaceId,
      state: caseState,
      revision,
      files: uploaded
        ? [
            {
              id: fileId,
              state: fileState,
              size: 32,
              mime: "application/pdf",
              pages: 1,
            },
          ]
        : [],
    };
  }

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const headers = request.headers();
    observed.push({
      path: pathname,
      method,
      authorization: headers["authorization"],
      ifMatch: headers["if-match"],
    });

    const json = (
      status: number,
      body: unknown,
      responseHeaders: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "no-store",
          ...responseHeaders,
        },
        body: JSON.stringify(body),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
            expires_at: sessionExpiresAt,
          })
        : json(401, {
            error: {
              code: "unauthenticated",
              message: "Unauthenticated",
              request_id: "synthetic",
            },
          });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        restricted_mode: false,
        expires_at: sessionExpiresAt,
      });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, {
        items: [{ id: spaceId, title: "Espacio sintético", state: "borrador" }],
      });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (omitCaseFromNextList) {
        omitCaseFromNextList = false;
        return json(200, []);
      }
      if (uploaded && !cleanAfterRefresh) {
        cleanAfterRefresh = true;
        fileState = "limpio";
      }
      return json(200, created ? [ownerCase()] : []);
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as {
        purpose: string;
        space_id: string;
      };
      expect(body).toEqual({ purpose: "space", space_id: spaceId });
      created = true;
      return json(201, ownerCase(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      if (failCaseRefreshOnce) {
        failCaseRefreshOnce = false;
        return json(503, {
          error: {
            code: "upstream_error",
            message: "La solicitud no pudo completarse.",
            request_id: "synthetic-case-refresh-unavailable",
          },
        });
      }
      return json(200, ownerCase(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      expect(headers["if-match"]).toBeDefined();
      expect(headers["content-type"]).toContain("multipart/form-data");
      const multipartBody = request.postDataBuffer()?.toString("latin1") ?? "";
      expect(multipartBody).toContain('name="file"');
      expect(multipartBody).not.toContain('name="evidence_role"');
      expect(multipartBody).toContain('filename="local-private-test.pdf"');
      expect(multipartBody).toContain("%PDF-1.4 synthetic upload");
      if (headers["if-match"] !== etag) {
        return json(409, {
          error: {
            code: "etag_conflict",
            message: "El expediente cambió.",
            request_id: "synthetic-upload-conflict",
          },
        });
      }
      if (failUploadOnce) {
        failUploadOnce = false;
        return json(503, {
          error: {
            code: "upstream_error",
            message: "La solicitud no pudo completarse.",
            request_id: "synthetic-upload-unavailable",
          },
        });
      }
      uploaded = true;
      revision += 1;
      etag = `"${revision}"`;
      return json(
        201,
        {
          revision,
          file: {
            id: fileId,
            state: "pendiente",
            size: 32,
            mime: "application/pdf",
            pages: 1,
          },
        },
        { etag },
      );
    }
    if (
      pathname ===
        `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      if (headers["if-match"] !== etag) {
        return json(409, {
          error: {
            code: "upstream_error",
            message: "La solicitud no pudo completarse.",
            request_id: "synthetic-conflict",
          },
        });
      }
      caseState = "en_revision";
      revision += 1;
      etag = `"${revision}"`;
      return json(200, ownerCase(), { etag });
    }
    if (
      pathname ===
        `/api/backend/document-verification/space-cases/${caseId}/files/${fileId}/content` &&
      method === "GET"
    ) {
      return route.fulfill({
        status: 200,
        headers: {
          "content-type": "application/pdf",
          "content-disposition": 'attachment; filename="space-document.pdf"',
          "cache-control": "private, no-store",
        },
        body: Buffer.from("%PDF-1.4 synthetic private document"),
      });
    }
    if (pathname === "/api/backend/auth/logout" && method === "POST") {
      loggedIn = false;
      return route.fulfill({ status: 204, body: "" });
    }
    return json(404, {
      error: {
        code: "not_found",
        message: "Not found",
        request_id: "synthetic",
      },
    });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fdocumentos");
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  await expect(
    page.getByRole("heading", { name: "Documentos de verificación" }),
  ).toBeVisible();

  await page.getByLabel("Tipo de expediente").selectOption("space");
  const spaceSelect = page.getByLabel("Espacio propio");
  await expect(spaceSelect.locator(`option[value="${spaceId}"]`)).toHaveCount(
    1,
  );
  await spaceSelect.selectOption(spaceId);
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText(
      "Expediente creado. Puedes cargar los documentos requeridos.",
    ),
  ).toBeVisible();
  await page.getByLabel("PDF, JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "local-private-test.pdf",
    mimeType: "application/pdf",
    buffer: Buffer.from("%PDF-1.4 synthetic upload"),
  });
  await page.getByRole("button", { name: "Cargar documento" }).click();
  await expect(
    page.getByText(
      "No se pudo cargar el documento. Actualiza el expediente antes de reintentar.",
    ),
  ).toBeVisible();
  const fileInput = page.getByLabel("PDF, JPEG o PNG · máximo 10 MiB");
  const selectedFileName = await fileInput.evaluate(
    (input) => (input as HTMLInputElement).files?.[0]?.name ?? null,
  );
  expect(selectedFileName).toBe("local-private-test.pdf");
  await expect(
    page.getByRole("button", { name: "Cargar documento" }),
  ).toBeDisabled();
  expect(
    observed.filter(
      ({ method, path }) => method === "POST" && path.endsWith("/files"),
    ),
  ).toHaveLength(1);
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByRole("button", { name: "Cargar documento" }),
  ).toBeEnabled();
  expect(
    await fileInput.evaluate(
      (input) => (input as HTMLInputElement).files?.[0]?.name ?? null,
    ),
  ).toBe("local-private-test.pdf");
  revision = 2;
  etag = '"2"';
  await page.getByRole("button", { name: "Cargar documento" }).click();
  await expect(
    page.getByText(
      "El expediente cambió desde la última consulta. Actualízalo antes de reintentar.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Cargar documento" }),
  ).toBeDisabled();
  expect(
    await fileInput.evaluate(
      (input) => (input as HTMLInputElement).files?.[0]?.name ?? null,
    ),
  ).toBe("local-private-test.pdf");
  expect(
    observed.filter(
      ({ method, path }) => method === "POST" && path.endsWith("/files"),
    ),
  ).toHaveLength(2);
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByRole("button", { name: "Cargar documento" }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "Cargar documento" }).click();
  await expect(
    page.getByText("Documento recibido y pendiente de inspección."),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Descargar" })).toHaveCount(0);
  await expect(page.getByText("local-private-test.pdf")).toHaveCount(0);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Validado · 1 página", { exact: true }),
  ).toBeVisible();

  // Simulate a concurrent update after this view's strong ETag was loaded.
  revision += 1;
  etag = `"${revision}"`;
  const submitButton = page.getByRole("button", { name: "Enviar a revisión" });
  await submitButton.click();
  await expect(submitButton).toBeDisabled();
  await expect(page.getByText(/actualiza el expediente/i)).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Actualizar expedientes" }),
  ).toBeEnabled();
  failCaseRefreshOnce = true;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText(
      "No se pudieron actualizar los expedientes. Intenta nuevamente.",
    ),
  ).toBeVisible();
  await expect(submitButton).toBeDisabled();
  expect(
    observed.filter(
      ({ method, path }) => method === "POST" && path.endsWith("/submit"),
    ),
  ).toHaveLength(1);
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(submitButton).toBeEnabled();
  omitCaseFromNextList = true;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(submitButton).toHaveCount(0);
  await expect(
    page.getByText(
      "El expediente seleccionado ya no aparece en tu lista. Actualízala y vuelve a seleccionarlo.",
    ),
  ).toBeVisible();
  expect(
    observed.filter(
      ({ method, path }) => method === "POST" && path.endsWith("/submit"),
    ),
  ).toHaveLength(1);
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(submitButton).toBeEnabled();
  await submitButton.click();
  await expect(
    page.getByText(/Expediente documental de espacio · En revisión/),
  ).toBeVisible();

  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "Descargar" }).click(),
  ]);
  expect(download.suggestedFilename()).toBe("documento-de-espacio.pdf");
  const downloadPath = await download.path();
  if (downloadPath === null) throw new Error("Download did not produce a file");
  expect(await readFile(downloadPath)).toEqual(
    Buffer.from("%PDF-1.4 synthetic private document"),
  );

  const documentRequests = observed.filter(({ path }) =>
    path.includes("document-verification"),
  );
  expect(
    documentRequests.map(({ method, path }) => `${method} ${path}`),
  ).toEqual([
    "GET /api/backend/document-verification/cases",
    "POST /api/backend/document-verification/cases",
    `POST /api/backend/document-verification/cases/${caseId}/files`,
    "GET /api/backend/document-verification/cases",
    `GET /api/backend/document-verification/cases/${caseId}`,
    `POST /api/backend/document-verification/cases/${caseId}/files`,
    "GET /api/backend/document-verification/cases",
    `GET /api/backend/document-verification/cases/${caseId}`,
    `POST /api/backend/document-verification/cases/${caseId}/files`,
    "GET /api/backend/document-verification/cases",
    `GET /api/backend/document-verification/cases/${caseId}`,
    `POST /api/backend/document-verification/cases/${caseId}/submit`,
    "GET /api/backend/document-verification/cases",
    `GET /api/backend/document-verification/cases/${caseId}`,
    "GET /api/backend/document-verification/cases",
    `GET /api/backend/document-verification/cases/${caseId}`,
    "GET /api/backend/document-verification/cases",
    "GET /api/backend/document-verification/cases",
    `GET /api/backend/document-verification/cases/${caseId}`,
    `POST /api/backend/document-verification/cases/${caseId}/submit`,
    `GET /api/backend/document-verification/space-cases/${caseId}/files/${fileId}/content`,
  ]);
  expect(
    documentRequests.every(({ authorization }) => authorization === undefined),
  ).toBe(true);
  expect(
    documentRequests
      .filter(
        ({ method, path }) => method === "POST" && path.endsWith("/files"),
      )
      .map(({ ifMatch }) => ifMatch),
  ).toEqual(['"1"', '"1"', '"2"']);
  expect(
    documentRequests
      .filter(
        ({ method, path }) => method === "POST" && path.endsWith("/submit"),
      )
      .map(({ ifMatch }) => ifMatch),
  ).toEqual(['"3"', '"4"']);
});

test("identity evidence requires three unique clean roles and replaces a duplicate before submit", async ({
  page,
}) => {
  const appOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`;
  const externalRequests: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.protocol !== "blob:" && url.origin !== appOrigin) {
      externalRequests.push(url.origin);
    }
  });
  const sessionExpiresAt = new Date(Date.now() + 60_000).toISOString();
  let loggedIn = false;
  let created = false;
  let uploadedRoles: (typeof identityEvidenceRoles)[number][] = [];
  const capturedRoles: string[] = [];
  let caseState = "borrador";
  const decisionMessage =
    "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente.";
  let fileState: "pendiente" | "limpio" = "pendiente";
  let revision = 1;
  let etag = '"1"';
  let declaredFormat = "";
  const observed: Array<{
    path: string;
    method: string;
    authorization?: string;
    ifMatch?: string;
  }> = [];

  function identityCase() {
    return {
      id: caseId,
      purpose: "identity",
      state: caseState,
      revision,
      owner_decision_message:
        caseState === "requiere_antecedentes" ? decisionMessage : undefined,
      document_format_declared: declaredFormat,
      files: uploadedRoles.map((role, index) => ({
        id:
          role === "identity_back" && uploadedRoles.indexOf(role) !== index
            ? duplicateBackFileId
            : role === "identity_front"
              ? fileId
              : role === "identity_back"
                ? backFileId
                : selfieFileId,
        evidence_role: role,
        state: fileState,
        size: 32,
        mime: "image/png",
        pages: 1,
      })),
    };
  }

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const headers = request.headers();
    observed.push({
      path: pathname,
      method,
      authorization: headers["authorization"],
      ifMatch: headers["if-match"],
    });

    const json = (
      status: number,
      body: unknown,
      responseHeaders: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "no-store",
          ...responseHeaders,
        },
        body: JSON.stringify(body),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
            expires_at: sessionExpiresAt,
          })
        : json(401, {
            error: {
              code: "unauthenticated",
              message: "Unauthenticated",
              request_id: "synthetic",
            },
          });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        restricted_mode: false,
        expires_at: sessionExpiresAt,
      });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (uploadedRoles.length === identityEvidenceRoles.length + 1) {
        const duplicateCase = identityCase();
        expect(duplicateCase.files).toHaveLength(4);
        expect(new Set(duplicateCase.files.map(({ id }) => id)).size).toBe(4);
        expect(
          duplicateCase.files.filter(
            ({ evidence_role }) => evidence_role === "identity_back",
          ),
        ).toHaveLength(2);
        return json(200, [duplicateCase]);
      }
      if (uploadedRoles.length === identityEvidenceRoles.length) {
        fileState = "limpio";
      }
      return json(200, created ? [identityCase()] : []);
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as Record<string, unknown>;
      expect(body).toEqual({
        capture_consent_given: true,
        capture_consent_version: "identity-capture-local-v1",
        document_format_declared: "unsure",
        purpose: "identity",
      });
      declaredFormat = body.document_format_declared as string;
      created = true;
      return json(201, identityCase(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      return json(200, identityCase(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      const multipart = request.postDataBuffer()?.toString("latin1") ?? "";
      const role = identityEvidenceRoles.find((candidate) =>
        multipart.includes(`\r\n\r\n${candidate}\r\n`),
      );
      expect(multipart).toContain('name="evidence_role"');
      expect(multipart).toContain('name="file"; filename=');
      expect(role).toBeDefined();
      if (!role) return json(400, { error: { code: "missing_evidence_role" } });
      if (headers["if-match"] !== etag) {
        return json(409, {
          error: {
            code: "etag_conflict",
            message: "El expediente cambió.",
            request_id: "synthetic-identity-conflict",
          },
        });
      }
      capturedRoles.push(role);
      uploadedRoles = [...uploadedRoles.filter((item) => item !== role), role];
      fileState = "pendiente";
      revision += 1;
      etag = `"${revision}"`;
      return json(
        201,
        {
          revision,
          file: {
            id:
              role === "identity_front"
                ? fileId
                : role === "identity_back"
                  ? backFileId
                  : selfieFileId,
            evidence_role: role,
            state: "pendiente",
            size: 32,
            mime: "image/png",
            pages: 1,
          },
        },
        { etag },
      );
    }
    if (
      pathname ===
        `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      if (headers["if-match"] !== etag) {
        return json(409, {
          error: {
            code: "etag_conflict",
            message: "El expediente cambió.",
            request_id: "synthetic-identity-submit-conflict",
          },
        });
      }
      caseState = "en_revision";
      revision += 1;
      etag = `"${revision}"`;
      return json(200, identityCase(), { etag });
    }
    return json(404, {
      error: {
        code: "not_found",
        message: "Not found",
        request_id: "synthetic",
      },
    });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fdocumentos");
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  await expect(
    page.getByText(/solo se usan expedientes sintéticos locales/i),
  ).toBeVisible();
  await page
    .getByLabel("Formato declarado de la cédula")
    .selectOption("unsure");
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText(
      "Confirma el aviso de prueba local antes de crear el expediente.",
    ),
  ).toBeVisible();
  expect(
    observed.filter(
      ({ method, path }) =>
        method === "POST" &&
        path === "/api/backend/document-verification/cases",
    ),
  ).toHaveLength(0);
  await page
    .getByLabel(
      "Confirmo el aviso de prueba local para un expediente sintético",
    )
    .check();
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(page.getByRole("status")).toContainText(
    "Expediente sintético creado.",
  );
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();
  await expect(page.getByText(/revisión humana/i)).toBeVisible();
  for (const [index, role] of identityEvidenceRoles.entries()) {
    await page.getByLabel("Evidencia a cargar").selectOption(role);
    await page.getByLabel("JPEG o PNG · máximo 10 MiB").setInputFiles({
      name: `synthetic-${index}.png`,
      mimeType: "image/png",
      buffer: Buffer.from(`synthetic-image-${index}`),
    });
    await page.getByRole("button", { name: "Cargar evidencia" }).click();
    await expect(
      page.getByRole("status").filter({
        hasText: "Evidencia recibida y pendiente de inspección.",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Enviar a revisión" }),
    ).toHaveCount(0);
  }
  expect(capturedRoles).toEqual(identityEvidenceRoles);
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  for (const role of identityEvidenceRoles) {
    const expectedLabel =
      role === "identity_front"
        ? "Anverso de la cédula · Validado"
        : role === "identity_back"
          ? "Reverso de la cédula · Validado"
          : "Selfie · Validado";
    await expect(page.getByText(expectedLabel)).toBeVisible();
  }
  const submitButton = page.getByRole("button", { name: "Enviar a revisión" });
  await expect(submitButton).toBeVisible();

  // A malformed owner response with a duplicate role must fail closed.
  uploadedRoles.push("identity_back");
  revision += 1;
  etag = `"${revision}"`;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Reverso de la cédula · Revisar archivos", { exact: true }),
  ).toBeVisible();
  await expect(submitButton).toHaveCount(0);
  expect(
    observed.filter(
      ({ method, path }) => method === "POST" && path.endsWith("/submit"),
    ),
  ).toHaveLength(0);

  await page.getByLabel("Evidencia a cargar").selectOption("identity_back");
  await page.getByLabel("JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-replacement-back.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic-replacement-image"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText("Reverso de la cédula · En inspección", { exact: true }),
  ).toBeVisible();
  await expect(submitButton).toHaveCount(0);
  expect(capturedRoles).toEqual([...identityEvidenceRoles, "identity_back"]);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Reverso de la cédula · Validado", { exact: true }),
  ).toBeVisible();
  await expect(submitButton).toBeEnabled();
  await submitButton.click();
  await expect(page.getByRole("status")).toContainText(
    "Expediente enviado a revisión.",
  );
  await expect(
    page.getByText("Expediente de identidad · En revisión"),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Descargar" })).toHaveCount(0);

  const documentRequests = observed.filter(({ path }) =>
    path.includes("document-verification"),
  );
  expect(
    documentRequests.some(
      ({ method, path }) =>
        method === "POST" &&
        path === "/api/backend/document-verification/cases",
    ),
  ).toBe(true);
  expect(
    documentRequests
      .filter(
        ({ method, path }) => method === "POST" && path.endsWith("/files"),
      )
      .map(({ ifMatch }) => ifMatch),
  ).toEqual(['"1"', '"2"', '"3"', '"5"']);
  expect(
    documentRequests
      .filter(
        ({ method, path }) => method === "POST" && path.endsWith("/submit"),
      )
      .map(({ ifMatch }) => ifMatch),
  ).toEqual(['"6"']);
  expect(
    documentRequests.every(({ authorization }) => authorization === undefined),
  ).toBe(true);

  expect(documentRequests.some(({ path }) => path.endsWith("/content"))).toBe(
    false,
  );
  expect(externalRequests).toEqual([]);

  caseState = "requiere_antecedentes";
  revision += 1;
  etag = `"${revision}"`;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(page.getByText(decisionMessage, { exact: true })).toBeVisible();

  caseState = "en_revision";
  revision += 1;
  etag = `"${revision}"`;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · En revisión", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText(decisionMessage, { exact: true })).toHaveCount(0);
});

test("late superseded case selection does not show a load error", async ({
  page,
}) => {
  const sessionExpiresAt = new Date(Date.now() + 60_000).toISOString();
  let loggedIn = false;
  let releaseFirstDetail!: () => void;
  let markFirstDetailStarted!: () => void;
  const firstDetailStarted = new Promise<void>((resolve) => {
    markFirstDetailStarted = resolve;
  });
  const firstDetailGate = new Promise<void>((resolve) => {
    releaseFirstDetail = resolve;
  });
  const cases = [
    {
      id: caseId,
      purpose: "space",
      space_id: spaceId,
      state: "borrador",
      revision: 1,
      files: [],
    },
    {
      id: secondCaseId,
      purpose: "space",
      space_id: spaceId,
      state: "borrador",
      revision: 2,
      files: [],
    },
  ];

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (status: number, body: unknown, etag?: string) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          ...(etag ? { etag } : {}),
        },
        body: JSON.stringify(body),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
            expires_at: sessionExpiresAt,
          })
        : json(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        restricted_mode: false,
        expires_at: sessionExpiresAt,
      });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, {
        items: [{ id: spaceId, title: "Espacio sintético", state: "borrador" }],
      });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return json(200, cases);
    }
    if (
      pathname.startsWith("/api/backend/document-verification/cases/") &&
      method === "GET"
    ) {
      const id = pathname.split("/").at(-1);
      const selectedCase = cases.find((item) => item.id === id);
      if (!selectedCase) return json(404, { error: { code: "not_found" } });
      if (id === caseId) {
        markFirstDetailStarted();
        await firstDetailGate;
        return json(200, selectedCase, '\"1\"');
      }
      return json(200, selectedCase, '\"2\"');
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

  const caseButtons = page.getByRole("button", {
    name: /Documentos del espacio/,
  });
  await expect(caseButtons).toHaveCount(2);
  await caseButtons.nth(0).click();
  await firstDetailStarted;
  await caseButtons.nth(1).click();
  await expect(
    page.getByLabel("PDF, JPEG o PNG · máximo 10 MiB"),
  ).toBeVisible();

  const staleResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      new URL(response.url()).pathname.endsWith(`/cases/${caseId}`),
  );
  releaseFirstDetail();
  await staleResponse;
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  await expect(
    page.getByText("No se pudo cargar el expediente seleccionado."),
  ).toHaveCount(0);
  const currentCaseButton = page.locator('button[aria-current="true"]');
  await expect(currentCaseButton).toContainText("revisión 2");
});

test("identity evidence File is isolated when the active account changes", async ({
  page,
}) => {
  const accountA = "550e8400-e29b-41d4-a716-446655440001";
  const accountB = "550e8400-e29b-41d4-a716-446655440002";
  const identityCaseA = "770e8400-e29b-41d4-a716-446655440001";
  const identityCaseB = "770e8400-e29b-41d4-a716-446655440002";
  const sessionExpiresAt = new Date(Date.now() + 60_000).toISOString();
  let activeAccountId = accountA;
  let loggedIn = false;
  const listReads = new Map<string, number>();
  const sessionReads: string[] = [];
  const uploadPosts: string[] = [];
  const detailReads: string[] = [];
  const providerRequests: string[] = [];

  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      url.origin !== `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? "4203"}`
    ) {
      providerRequests.push(url.origin);
    }
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (
      status: number,
      body: unknown,
      headers: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "no-store",
          ...headers,
        },
        body: JSON.stringify(body),
      });
    const session = () => ({
      account_id: activeAccountId,
      roles: ["arrendador"],
      capabilities: [],
      restricted_mode: false,
      expires_at: sessionExpiresAt,
    });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      sessionReads.push(activeAccountId);
      return loggedIn
        ? json(200, session())
        : json(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      activeAccountId = accountA;
      return json(200, session());
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      listReads.set(activeAccountId, (listReads.get(activeAccountId) ?? 0) + 1);
      const id = activeAccountId === accountA ? identityCaseA : identityCaseB;
      return json(200, [
        {
          id,
          purpose: "identity",
          state: "borrador",
          revision: 1,
          document_format_declared: "unsure",
          files: [],
        },
      ]);
    }
    if (
      pathname.startsWith("/api/backend/document-verification/cases/") &&
      method === "GET"
    ) {
      const id = pathname.split("/").at(-1);
      if (id !== identityCaseA && id !== identityCaseB) {
        return json(404, { error: { code: "not_found" } });
      }
      detailReads.push(`${activeAccountId}:${id}`);
      return json(
        200,
        {
          id,
          purpose: "identity",
          state: "borrador",
          revision: 1,
          document_format_declared: "unsure",
          files: [],
        },
        { etag: '\"1\"' },
      );
    }
    if (pathname.endsWith("/files") && method === "POST") {
      uploadPosts.push(pathname);
      return json(500, { error: { code: "unexpected_upload" } });
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

  const rowA = page.getByRole("button", { name: /Identidad.*Borrador/ });
  await expect(rowA).toBeVisible();
  await rowA.click();
  const evidenceInput = page.getByLabel("JPEG o PNG · máximo 10 MiB");
  await evidenceInput.setInputFiles({
    name: "account-a-identity.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic account A identity evidence"),
  });
  await expect(evidenceInput).toHaveValue(/account-a-identity\.png/);
  expect(
    await evidenceInput.evaluate(
      (input) => (input as HTMLInputElement).files?.[0]?.name,
    ),
  ).toBe("account-a-identity.png");
  expect(uploadPosts).toHaveLength(0);

  const sessionReadsBeforeSwitch = sessionReads.length;
  activeAccountId = accountB;
  const sessionForB = page.waitForResponse(async (response) => {
    if (
      response.request().method() !== "GET" ||
      !new URL(response.url()).pathname.endsWith("/auth/session") ||
      response.status() !== 200
    ) {
      return false;
    }
    const body = (await response.json()) as { account_id?: string };
    return body.account_id === accountB;
  });
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  await sessionForB;
  await expect
    .poll(() => sessionReads.length)
    .toBe(sessionReadsBeforeSwitch + 1);
  expect(sessionReads.slice(sessionReadsBeforeSwitch)).toEqual([accountB]);
  await expect.poll(() => listReads.get(accountB) ?? 0).toBe(1);
  await expect(
    page.getByRole("button", { name: /Identidad.*Borrador/ }),
  ).toHaveCount(1);

  await page.getByRole("button", { name: /Identidad.*Borrador/ }).click();
  const accountBInput = page.getByLabel("JPEG o PNG · máximo 10 MiB");
  await expect(accountBInput).toBeVisible();
  await expect
    .poll(() => detailReads.includes(`${accountB}:${identityCaseB}`))
    .toBe(true);
  expect(
    await accountBInput.evaluate(
      (input) => (input as HTMLInputElement).files?.length ?? 0,
    ),
  ).toBe(0);
  await expect(page.getByText("account-a-identity.png")).toHaveCount(0);
  expect(detailReads).toContain(`${accountA}:${identityCaseA}`);
  expect(detailReads).toContain(`${accountB}:${identityCaseB}`);
  expect(detailReads).not.toContain(`${accountB}:${identityCaseA}`);
  expect(listReads.get(accountA)).toBe(1);
  expect(listReads.get(accountB)).toBe(1);
  expect(uploadPosts).toHaveLength(0);
  expect(providerRequests).toEqual([]);
});
