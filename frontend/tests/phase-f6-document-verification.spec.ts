import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const spaceId = "660e8400-e29b-41d4-a716-446655440000";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const secondCaseId = "990e8400-e29b-41d4-a716-446655440000";
const fileId = "880e8400-e29b-41d4-a716-446655440000";

test.use({ trace: "off", screenshot: "off", video: "off" });

test("owner completes the six document operations and downloads only a clean space file", async ({
  page,
}) => {
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
            expires_at: new Date(Date.now() + 60_000).toISOString(),
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
        expires_at: new Date(Date.now() + 60_000).toISOString(),
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

test("identity case supports the private workflow without exposing a download", async ({
  page,
}) => {
  let loggedIn = false;
  let created = false;
  let uploaded = false;
  let caseState = "borrador";
  let fileState: "pendiente" | "limpio" = "pendiente";
  let revision = 1;
  let etag = '"1"';
  let expiresAt = "";
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
      document_expires_at: expiresAt,
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
            expires_at: new Date(Date.now() + 60_000).toISOString(),
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
        expires_at: new Date(Date.now() + 60_000).toISOString(),
      });
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return json(200, { items: [] });
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (uploaded) fileState = "limpio";
      return json(200, created ? [identityCase()] : []);
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as Record<string, unknown>;
      expect(body.purpose).toBe("identity");
      expect(Object.keys(body).sort()).toEqual([
        "document_expires_at",
        "purpose",
      ]);
      expect(typeof body.document_expires_at).toBe("string");
      expiresAt = body.document_expires_at as string;
      expect(Number.isNaN(Date.parse(expiresAt))).toBe(false);
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
      if (headers["if-match"] !== etag) {
        return json(409, {
          error: {
            code: "etag_conflict",
            message: "El expediente cambió.",
            request_id: "synthetic-identity-conflict",
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
  await page.getByLabel("Vencimiento del documento").fill("2030-01-02T00:00");
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText(
      "Expediente creado. Puedes cargar los documentos requeridos.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();

  await page.getByLabel("PDF, JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-identity.pdf",
    mimeType: "application/pdf",
    buffer: Buffer.from("%PDF-1.4 synthetic identity document"),
  });
  await page.getByRole("button", { name: "Cargar documento" }).click();
  await expect(
    page.getByText("Documento recibido y pendiente de inspección."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Validado · 1 página", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Enviar a revisión" }).click();
  await expect(
    page.getByText(/Expediente de identidad · En revisión/),
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
    documentRequests.some(
      ({ method, path }) =>
        method === "GET" &&
        path === `/api/backend/document-verification/cases/${caseId}`,
    ),
  ).toBe(true);
  expect(
    documentRequests.some(
      ({ method, path }) =>
        method === "POST" && path.endsWith(`/cases/${caseId}/files`),
    ),
  ).toBe(true);
  expect(
    documentRequests.some(
      ({ method, path }) =>
        method === "POST" && path.endsWith(`/cases/${caseId}/submit`),
    ),
  ).toBe(true);
  expect(
    documentRequests.every(({ authorization }) => authorization === undefined),
  ).toBe(true);
  expect(
    documentRequests
      .filter(
        ({ method, path }) => method === "POST" && path.endsWith("/files"),
      )
      .map(({ ifMatch }) => ifMatch),
  ).toEqual(['"1"']);
  expect(
    documentRequests
      .filter(
        ({ method, path }) => method === "POST" && path.endsWith("/submit"),
      )
      .map(({ ifMatch }) => ifMatch),
  ).toEqual(['"2"']);
  expect(documentRequests.some(({ path }) => path.endsWith("/content"))).toBe(
    false,
  );
});

test("late superseded case selection does not show a load error", async ({
  page,
}) => {
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
            expires_at: new Date(Date.now() + 60_000).toISOString(),
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
        expires_at: new Date(Date.now() + 60_000).toISOString(),
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
