import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

const accountId = "770e8400-e29b-41d4-a716-446655440000";

async function signIn(page: Page, target: string) {
  await page.goto(`/acceso?return_to=${encodeURIComponent(target)}`);
  await page
    .getByLabel("Correo electrónico")
    .fill("didit-status@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticOnly123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === target.split("?")[0]);
}

async function installMock(
  page: Page,
  handler: (
    path: string,
    method: string,
    request: Request,
  ) => { status: number; body?: unknown; headers?: Record<string, string> },
  getSessionAccountId: () => string = () => accountId,
) {
  const fixture = createBffFixtureState();
  await page.route("**/api/backend/**", async (route: Route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname;
    let response;
    if (path === "/api/backend/auth/login" && req.method() === "POST")
      response = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600000).toISOString(),
          restricted_mode: false,
        },
      };
    else if (path === "/api/backend/auth/session" && req.method() === "GET")
      response = {
        status: 200,
        body: {
          account_id: getSessionAccountId(),
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600000).toISOString(),
          restricted_mode: false,
        },
      };
    else response = handler(path, req.method(), req);
    const adapted = fixture.adapt(req, response);
    await route.fulfill({
      status: adapted?.status ?? 500,
      headers: {
        "content-type": "application/json",
        ...adapted?.headers,
      },
      body: JSON.stringify(adapted?.body ?? {}),
    });
  });
}

test("Mi cuenta obtiene el estado propio y ofrece iniciar cuando no hay expediente", async ({
  page,
}) => {
  let providerRequests = 0;
  let caseListReads = 0;
  await installMock(page, (path, method) => {
    if (path.includes("/verifications/didit/")) providerRequests++;
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      caseListReads += 1;
      return { status: 200, body: [] };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta");
  await expect(
    page.getByRole("heading", { name: "Verificación de la cuenta" }),
  ).toBeVisible();
  await expect(
    page.getByText("Aún no has iniciado una revisión documental propia."),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Iniciar verificación documental" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  await expect(
    page.getByRole("link", { name: "Ver declaración" }),
  ).toHaveAttribute("href", "/mi-cuenta/verificaciones");
  expect(caseListReads).toBeGreaterThan(0);
  expect(providerRequests).toBe(0);
});

test("Mi cuenta distingue el expediente de identidad de los de espacios y muestra su estado", async ({
  page,
}) => {
  const identityCaseId = "990e8400-e29b-41d4-a716-446655440001";
  const spaceCaseId = "990e8400-e29b-41d4-a716-446655440002";
  let providerRequests = 0;
  await installMock(page, (path, method) => {
    if (path.includes("/verifications/didit/")) providerRequests++;
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: [
          {
            id: spaceCaseId,
            purpose: "space",
            state: "borrador",
            revision: 1,
            files: [],
          },
          {
            id: identityCaseId,
            purpose: "identity",
            state: "en_revision",
            revision: 4,
            document_format_declared: "new",
            files: [
              {
                id: "990e8400-e29b-41d4-a716-446655440003",
                evidence_role: "identity_front",
                state: "limpio",
                mime: "image/jpeg",
                size: 128,
                pages: 0,
              },
            ],
          },
        ],
      };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta");
  await expect(
    page.getByText("Tu solicitud está en revisión documental."),
  ).toBeVisible();
  await expect(
    page.getByText("Aún no has iniciado una revisión documental propia."),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Ver estado de solicitud" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  await expect(page.getByText(identityCaseId)).toHaveCount(0);
  await expect(page.getByText("identity_front")).toHaveCount(0);
  expect(providerRequests).toBe(0);
});

test("Mi cuenta muestra la decisión aprobada sin inferir vigencia en el cliente", async ({
  page,
}) => {
  let providerRequests = 0;
  await installMock(page, (path, method) => {
    if (path.includes("/verifications/didit/")) providerRequests++;
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: [
          {
            id: "990e8400-e29b-41d4-a716-446655440004",
            purpose: "identity",
            state: "aprobada",
            revision: 5,
            verified_at: "2025-03-10T12:00:00Z",
            verified_until: "2025-03-10T11:59:59Z",
            files: [],
          },
        ],
      };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta");
  await expect(
    page.getByText(
      "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Cuenta verificada", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText(/vigente hasta/i)).toHaveCount(0);
  expect(providerRequests).toBe(0);
});

test("Mi cuenta ofrece corregir documentos cuando la revisión requiere antecedentes", async ({
  page,
}) => {
  const identityCaseId = "990e8400-e29b-41d4-a716-446655440006";
  const correctionCase = {
    id: identityCaseId,
    purpose: "identity",
    state: "requiere_antecedentes",
    revision: 6,
    files: [],
  };
  await installMock(page, (path, method) => {
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return { status: 200, body: [correctionCase] };
    }
    if (path === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      path === `/api/backend/document-verification/cases/${identityCaseId}` &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: correctionCase,
        headers: { etag: '"6"' },
      };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page, "/mi-cuenta");

  await expect(
    page.getByText("Necesitamos que corrijas o completes tus documentos."),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Corregir documentos" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  await page.getByRole("link", { name: "Corregir documentos" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos/);
  const correctionCaseButton = page.getByRole("button", {
    name: /Identidad.*revisión 6/,
  });
  await expect(correctionCaseButton).toBeVisible();
  await correctionCaseButton.click();
  await expect(
    page.getByRole("heading", { name: "Agregar evidencia sintética" }),
  ).toBeVisible();
});

test("Mi cuenta permite reponer evidencia bloqueada y solo reenviar tras validación sintética", async ({
  page,
}) => {
  const identityCaseId = "990e8400-e29b-41d4-a716-446655440010";
  let revision = 7;
  let etag = '"7"';
  let caseState = "requiere_antecedentes";
  let replacementUploaded = false;
  let replacementScanned = false;
  const uploadEtags: string[] = [];
  const uploadRoles: string[] = [];
  const submitEtags: string[] = [];
  const ownerCase = () => ({
    id: identityCaseId,
    purpose: "identity",
    state: caseState,
    revision,
    files: [
      {
        id: "990e8400-e29b-41d4-a716-446655440011",
        evidence_role: "identity_front",
        state: "limpio",
        mime: "image/png",
        size: 128,
        pages: 0,
      },
      {
        id: replacementUploaded
          ? "990e8400-e29b-41d4-a716-446655440013"
          : "990e8400-e29b-41d4-a716-446655440012",
        evidence_role: "identity_back",
        state: replacementUploaded
          ? replacementScanned
            ? "limpio"
            : "pendiente"
          : "bloqueado",
        mime: "image/png",
        size: 128,
        pages: 0,
      },
      {
        id: "990e8400-e29b-41d4-a716-446655440014",
        evidence_role: "identity_selfie",
        state: "limpio",
        mime: "image/png",
        size: 128,
        pages: 0,
      },
    ],
  });

  await installMock(page, (path, method, request) => {
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (replacementUploaded) replacementScanned = true;
      return { status: 200, body: [ownerCase()] };
    }
    if (path === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      path === `/api/backend/document-verification/cases/${identityCaseId}` &&
      method === "GET"
    ) {
      return { status: 200, body: ownerCase(), headers: { etag } };
    }
    if (
      path ===
        `/api/backend/document-verification/cases/${identityCaseId}/files` &&
      method === "POST"
    ) {
      const headers = request.headers();
      uploadEtags.push(headers["if-match"] ?? "");
      const body = request.postDataBuffer()?.toString("utf8") ?? "";
      const roleParts = body.match(/name="evidence_role"/g) ?? [];
      const fileParts = body.match(/name="file"/g) ?? [];
      const expectedFileParts =
        body.match(
          /name="file"; filename="identity-back-synthetic\.png"\r\nContent-Type: image\/png\r\n\r\nsynthetic identity evidence\r\n/g,
        ) ?? [];
      const role = body.match(
        /name="evidence_role"\r\n\r\n([^\r\n]+)\r\n/,
      )?.[1];
      const contentType = headers["content-type"] ?? "";
      if (headers["if-match"] !== etag) {
        return { status: 409, body: { error: { code: "etag_conflict" } } };
      }
      if (
        etag !== '"7"' ||
        !contentType.startsWith("multipart/form-data; boundary=") ||
        roleParts.length !== 1 ||
        role !== "identity_back" ||
        fileParts.length !== 1 ||
        expectedFileParts.length !== 1
      ) {
        return { status: 400, body: { error: { code: "invalid_upload" } } };
      }
      uploadRoles.push(role);
      replacementUploaded = true;
      revision += 1;
      etag = `"${revision}"`;
      return {
        status: 201,
        headers: { etag },
        body: {
          revision,
          file: {
            id: "990e8400-e29b-41d4-a716-446655440013",
            evidence_role: "identity_back",
            state: "pendiente",
            mime: "image/png",
            size: 128,
            pages: 0,
          },
        },
      };
    }
    if (
      path ===
        `/api/backend/document-verification/cases/${identityCaseId}/submit` &&
      method === "POST"
    ) {
      const requestEtag = request.headers()["if-match"] ?? "";
      submitEtags.push(requestEtag);
      if (requestEtag !== etag || etag !== '"8"') {
        return { status: 409, body: { error: { code: "etag_conflict" } } };
      }
      if (!replacementUploaded || !replacementScanned) {
        return { status: 409, body: { error: { code: "not_ready" } } };
      }
      caseState = "en_revision";
      revision += 1;
      etag = `"${revision}"`;
      return {
        status: 200,
        body: ownerCase(),
        headers: { etag },
      };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page, "/mi-cuenta/documentos");
  await page
    .getByRole("button", { name: /Identidad.*Requiere antecedentes/ })
    .click();
  await expect(
    page.getByText("Reverso de la cédula · No aceptado", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toHaveCount(0);

  await page.getByLabel("Evidencia a cargar").selectOption("identity_back");
  await page.locator('input[type="file"]').setInputFiles({
    name: "identity-back-synthetic.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic identity evidence"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText("Reverso de la cédula · En inspección", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toHaveCount(0);
  expect(uploadRoles).toEqual(["identity_back"]);
  expect(uploadEtags).toEqual(['"7"']);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Reverso de la cédula · Validado", { exact: true }),
  ).toBeVisible();
  const submitButton = page.getByRole("button", {
    name: "Enviar a revisión",
  });
  await expect(submitButton).toBeEnabled();
  await submitButton.click();
  await expect(
    page.getByText("Expediente de identidad · En revisión", { exact: true }),
  ).toBeVisible();
  expect(submitEtags).toEqual(['"8"']);
});

test("Mi cuenta presenta una revisión revocada sin afirmar vigencia ni verificación", async ({
  page,
}) => {
  await installMock(page, (path, method) => {
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: [
          {
            id: "990e8400-e29b-41d4-a716-446655440007",
            purpose: "identity",
            state: "revocada",
            revision: 7,
            verified_at: "2025-03-10T12:00:00Z",
            verified_until: "2035-03-10T12:00:00Z",
            files: [],
          },
        ],
      };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page, "/mi-cuenta");

  await expect(
    page.getByText(
      "La revisión fue revocada. La vigencia efectiva la determina el servidor.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Cuenta verificada", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText(/vigente hasta/i)).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Ver estado de solicitud" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
});

test("Mi cuenta, Verificaciones y Documentos presentan el mismo estado vencido", async ({
  page,
}) => {
  const identityCaseId = "990e8400-e29b-41d4-a716-446655440010";
  const expiredCase = {
    id: identityCaseId,
    purpose: "identity",
    state: "vencida",
    revision: 8,
    files: [],
  };
  const declaration = {
    id: "990e8400-e29b-41d4-a716-446655440011",
    revision: 3,
    first_name: "Persona",
    last_name: "Sintética",
    date_of_birth: "1990-01-01",
    rut: "000000000",
    id_country: "CHL",
    document_type: "ID",
    consent_version: "synthetic-local-fixture",
    consent_given_at: "2026-11-11T12:00:00Z",
  };
  let providerRequests = 0;
  let caseListReads = 0;

  await installMock(page, (path, method) => {
    if (path.includes("/verifications/didit/")) providerRequests += 1;
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      caseListReads += 1;
      return { status: 200, body: [expiredCase] };
    }
    if (
      path === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      return { status: 200, body: declaration };
    }
    if (path === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      path === `/api/backend/document-verification/cases/${identityCaseId}` &&
      method === "GET"
    ) {
      return { status: 200, body: expiredCase, headers: { etag: '"8"' } };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page, "/mi-cuenta");
  await expect(
    page.getByText("La solicitud venció. Consulta los pasos para continuar."),
  ).toBeVisible();
  const accountCaseReads = caseListReads;
  expect(accountCaseReads).toBe(1);
  await expect(
    page.getByText(
      "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
    ),
  ).toHaveCount(0);
  await expect(
    page.getByText("Cuenta verificada", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText(/vigente hasta/i)).toHaveCount(0);

  await page.goto("/mi-cuenta/verificaciones");
  await expect(
    page.getByRole("heading", {
      name: "Verificación de usuario (Identidad personal)",
    }),
  ).toBeVisible();
  await expect(page.getByText("Datos guardados (revisión #3)")).toBeVisible();
  await expect(
    page.getByText("La solicitud venció. Consulta los pasos para continuar."),
  ).toBeVisible();
  await expect.poll(() => caseListReads).toBe(accountCaseReads + 1);
  const verificationCaseReads = caseListReads;
  await expect(
    page.getByRole("link", { name: "Ver estado de solicitud" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  await expect(page.getByRole("link", { name: "Ver declaración" })).toHaveCount(
    0,
  );
  await expect(
    page.getByText(
      "La revisión documental propia estará disponible próximamente",
    ),
  ).toHaveCount(0);

  await page.goto("/mi-cuenta/documentos");
  await expect(
    page.getByRole("heading", { name: "Documentos de verificación" }),
  ).toBeVisible();
  await expect.poll(() => caseListReads).toBe(verificationCaseReads + 1);
  const expiredRow = page.getByRole("button", {
    name: /Identidad.*Vencida/,
  });
  await expect(expiredRow).toBeVisible();
  await expiredRow.click();
  await expect(
    page.getByText("Expediente de identidad · Vencida", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Expediente de identidad · Aprobada", { exact: true }),
  ).toHaveCount(0);
  expect(providerRequests).toBe(0);
});

test("Verificaciones actualiza el estado de identidad al recuperar el foco", async ({
  page,
}) => {
  const identityCaseId = "990e8400-e29b-41d4-a716-446655440012";
  const ownerDecisionMessage =
    "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente.";
  let identityState: "en_revision" | "requiere_antecedentes" = "en_revision";
  let caseListReads = 0;
  let providerRequests = 0;
  const declaration = {
    id: "990e8400-e29b-41d4-a716-446655440013",
    revision: 4,
    first_name: "Cuenta",
    last_name: "Sintética",
    date_of_birth: "1990-01-01",
    rut: "000000000",
    id_country: "CHL",
    document_type: "ID",
    consent_version: "synthetic-local-fixture",
    consent_given_at: "2026-11-11T12:00:00Z",
  };

  await installMock(page, (path, method) => {
    if (path.includes("/verifications/didit/")) providerRequests += 1;
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      caseListReads += 1;
      return {
        status: 200,
        body: [
          {
            id: identityCaseId,
            purpose: "identity",
            state: identityState,
            revision: caseListReads,
            owner_decision_message:
              identityState === "requiere_antecedentes"
                ? ownerDecisionMessage
                : undefined,
            files: [],
          },
        ],
      };
    }
    if (
      path === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      return { status: 200, body: declaration };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page, "/mi-cuenta/verificaciones");
  await expect(page.getByText("Datos guardados (revisión #4)")).toBeVisible();
  await expect(
    page.getByText("Tu solicitud está en revisión documental."),
  ).toBeVisible();
  expect(caseListReads).toBe(1);

  identityState = "requiere_antecedentes";
  const refreshResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname ===
        "/api/backend/document-verification/cases" &&
      response.request().method() === "GET" &&
      response.status() === 200,
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await refreshResponse;

  await expect(
    page.getByText(ownerDecisionMessage, { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Tu solicitud está en revisión documental."),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Corregir documentos" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  expect(caseListReads).toBe(2);

  identityState = "en_revision";
  const secondRefreshResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname ===
        "/api/backend/document-verification/cases" &&
      response.request().method() === "GET" &&
      response.status() === 200,
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await secondRefreshResponse;

  await expect(
    page.getByText("Tu solicitud está en revisión documental."),
  ).toBeVisible();
  await expect(
    page.getByText(ownerDecisionMessage, { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "Ver estado de solicitud" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  expect(caseListReads).toBe(3);
  expect(providerRequests).toBe(0);
});

test("Mi cuenta reemplaza el resumen de A tras un cambio de sesión a B", async ({
  page,
}) => {
  const accountB = "880e8400-e29b-41d4-a716-446655440008";
  let activeAccountId = accountId;
  let caseListReads = 0;
  await installMock(
    page,
    (path, method) => {
      if (
        path === "/api/backend/document-verification/cases" &&
        method === "GET"
      ) {
        caseListReads += 1;
        return {
          status: 200,
          body: [
            {
              id:
                activeAccountId === accountId
                  ? "990e8400-e29b-41d4-a716-446655440008"
                  : "990e8400-e29b-41d4-a716-446655440009",
              purpose: "identity",
              state:
                activeAccountId === accountId
                  ? "aprobada"
                  : "requiere_antecedentes",
              revision: activeAccountId === accountId ? 8 : 9,
              files: [],
            },
          ],
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    },
    () => activeAccountId,
  );

  await signIn(page, "/mi-cuenta");
  const approvalMessage = page.getByText(
    "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
  );
  await expect(approvalMessage).toBeVisible();
  const readsBeforeSwitch = caseListReads;

  activeAccountId = accountB;
  const sessionResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes("/api/backend/auth/session") &&
      response.status() === 200,
  );
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  const sessionResponse = await sessionResponsePromise;
  expect((await sessionResponse.json()).account_id).toBe(accountB);

  await expect.poll(() => caseListReads).toBeGreaterThan(readsBeforeSwitch);
  await expect(
    page.getByText("Necesitamos que corrijas o completes tus documentos."),
  ).toBeVisible();
  await expect(approvalMessage).toHaveCount(0);
});

test("Mi cuenta vuelve a cargar el resumen propio tras un ciclo A→B→A", async ({
  page,
}) => {
  const accountB = "880e8400-e29b-41d4-a716-446655440012";
  let activeAccountId = accountId;
  let caseListReads = 0;
  await installMock(
    page,
    (path, method) => {
      if (
        path === "/api/backend/document-verification/cases" &&
        method === "GET"
      ) {
        caseListReads += 1;
        return {
          status: 200,
          body: [
            {
              id:
                activeAccountId === accountId
                  ? "990e8400-e29b-41d4-a716-446655440012"
                  : "990e8400-e29b-41d4-a716-446655440013",
              purpose: "identity",
              state:
                activeAccountId === accountId
                  ? "aprobada"
                  : "requiere_antecedentes",
              revision: activeAccountId === accountId ? 12 : 13,
              files: [],
            },
          ],
        };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    },
    () => activeAccountId,
  );

  await signIn(page, "/mi-cuenta");
  const approvalMessage = page.getByText(
    "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
  );
  const correctionMessage = page.getByText(
    "Necesitamos que corrijas o completes tus documentos.",
  );
  await expect(approvalMessage).toBeVisible();

  const switchSession = async (nextAccountId: string) => {
    const readsBeforeSwitch = caseListReads;
    activeAccountId = nextAccountId;
    const sessionResponsePromise = page.waitForResponse(async (response) => {
      if (
        !response.url().includes("/api/backend/auth/session") ||
        response.status() !== 200
      ) {
        return false;
      }
      return (await response.json()).account_id === nextAccountId;
    });
    await page.evaluate(() => {
      const channel = new BroadcastChannel("espacigo_auth");
      channel.postMessage({ type: "session_changed" });
      channel.close();
    });
    const sessionResponse = await sessionResponsePromise;
    expect((await sessionResponse.json()).account_id).toBe(nextAccountId);
    await expect.poll(() => caseListReads).toBeGreaterThan(readsBeforeSwitch);
  };

  await switchSession(accountB);
  await expect(correctionMessage).toBeVisible();
  await expect(approvalMessage).toHaveCount(0);

  await switchSession(accountId);
  await expect(approvalMessage).toBeVisible();
  await expect(correctionMessage).toHaveCount(0);
  expect(caseListReads).toBeGreaterThanOrEqual(3);
});

test("Mi cuenta conserva el resumen de B si la respuesta pendiente de A se libera después del cambio", async ({
  page,
}) => {
  const accountB = "880e8400-e29b-41d4-a716-446655440010";
  const casePath = "/api/backend/document-verification/cases";
  let activeAccountId = accountId;
  let caseListReads = 0;
  let holdNextCaseList = false;
  const pendingAControl: {
    release?: () => void;
    markStarted?: () => void;
    markRouteFinished?: () => void;
  } = {};
  const pendingAStarted = new Promise<void>((resolve) => {
    pendingAControl.markStarted = resolve;
  });
  const pendingARouteFinished = new Promise<void>((resolve) => {
    pendingAControl.markRouteFinished = resolve;
  });
  const caseFor = (owner: string) => ({
    id:
      owner === accountId
        ? "990e8400-e29b-41d4-a716-446655440010"
        : "990e8400-e29b-41d4-a716-446655440011",
    purpose: "identity",
    state: owner === accountId ? "aprobada" : "requiere_antecedentes",
    revision: owner === accountId ? 10 : 11,
    files: [],
  });

  await installMock(
    page,
    () => ({ status: 404, body: { error: { code: "not_found" } } }),
    () => activeAccountId,
  );
  await page.route(
    "**/api/backend/document-verification/cases",
    async (route: Route) => {
      const request = route.request();
      if (request.method() !== "GET") return route.fallback();

      caseListReads += 1;
      if (holdNextCaseList) {
        holdNextCaseList = false;
        const requestOwner = activeAccountId;
        expect(requestOwner).toBe(accountId);
        pendingAControl.markStarted?.();
        await new Promise<void>((resolve) => {
          pendingAControl.release = resolve;
        });
        try {
          await route.fulfill({
            status: 200,
            headers: { "content-type": "application/json" },
            body: JSON.stringify([caseFor(requestOwner)]),
          });
        } catch (error) {
          if (!request.failure()) throw error;
        } finally {
          pendingAControl.markRouteFinished?.();
        }
        return;
      }

      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify([caseFor(activeAccountId)]),
      });
    },
  );

  await signIn(page, "/mi-cuenta");
  const approvalMessage = page.getByText(
    "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
  );
  await expect(approvalMessage).toBeVisible();
  const readsBeforePendingA = caseListReads;

  holdNextCaseList = true;
  const pendingRequestPromise = page.waitForRequest(
    (request) =>
      new URL(request.url()).pathname === casePath &&
      request.method() === "GET",
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const pendingRequest = await pendingRequestPromise;
  await pendingAStarted;
  await expect.poll(() => caseListReads).toBeGreaterThan(readsBeforePendingA);

  activeAccountId = accountB;
  const sessionResponsePromise = page.waitForResponse(async (response) => {
    if (
      !response.url().includes("/api/backend/auth/session") ||
      response.status() !== 200
    ) {
      return false;
    }
    return (await response.json()).account_id === accountB;
  });
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  const sessionResponse = await sessionResponsePromise;
  expect((await sessionResponse.json()).account_id).toBe(accountB);

  await expect
    .poll(() => caseListReads)
    .toBeGreaterThan(readsBeforePendingA + 1);
  const correctionMessage = page.getByText(
    "Necesitamos que corrijas o completes tus documentos.",
  );
  await expect(correctionMessage).toBeVisible();

  pendingAControl.release?.();
  await pendingARouteFinished;
  const lateResponse = await pendingRequest.response();
  expect(lateResponse?.status()).toBe(200);
  await expect(correctionMessage).toBeVisible();
  await expect(approvalMessage).toHaveCount(0);
  await expect(
    page.getByText("No pudimos consultar el estado de tu revisión documental."),
  ).toHaveCount(0);
});

test("Verificaciones no restaura la identidad ni el estado de A tras una respuesta de casos demorada", async ({
  page,
}) => {
  const accountB = "880e8400-e29b-41d4-a716-446655440014";
  const casePath = "/api/backend/document-verification/cases";
  const declarationPath = "/api/backend/verifications/identity-declaration";
  let activeAccountId = accountId;
  let holdNextCaseList = false;
  const caseReadsByOwner = new Map<string, number>();
  let providerRequests = 0;
  let releaseHeldResponse: (() => void) | undefined;
  let markHeldRequestStarted: (() => void) | undefined;
  let markHeldRouteFinished: (() => void) | undefined;
  const heldRequestStarted = new Promise<void>((resolve) => {
    markHeldRequestStarted = resolve;
  });
  const heldRouteFinished = new Promise<void>((resolve) => {
    markHeldRouteFinished = resolve;
  });
  const declarationFor = (owner: string) => ({
    id:
      owner === accountId
        ? "990e8400-e29b-41d4-a716-446655440020"
        : "990e8400-e29b-41d4-a716-446655440021",
    revision: owner === accountId ? 20 : 21,
    first_name: owner === accountId ? "Cuenta A" : "Cuenta B",
    last_name: "Sintética",
    date_of_birth: "1990-01-01",
    rut: "000000000",
    id_country: "CHL",
    document_type: "ID",
    consent_version: "synthetic-local-fixture",
    consent_given_at: "2026-11-11T12:00:00Z",
  });
  const caseFor = (owner: string) => ({
    id:
      owner === accountId
        ? "990e8400-e29b-41d4-a716-446655440022"
        : "990e8400-e29b-41d4-a716-446655440023",
    purpose: "identity",
    state: owner === accountId ? "en_revision" : "requiere_antecedentes",
    revision: owner === accountId ? 22 : 23,
    files: [],
  });

  await installMock(
    page,
    (path, method) => {
      if (path.includes("/verifications/didit/")) providerRequests += 1;
      if (path === declarationPath && method === "GET") {
        return { status: 200, body: declarationFor(activeAccountId) };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    },
    () => activeAccountId,
  );
  await page.route(`**${casePath}`, async (route: Route) => {
    const request = route.request();
    if (request.method() !== "GET") return route.fallback();

    // The mock session is authoritative for the owner at dispatch time.
    const requestOwner = activeAccountId;
    const ownerReads = (caseReadsByOwner.get(requestOwner) ?? 0) + 1;
    caseReadsByOwner.set(requestOwner, ownerReads);
    if (holdNextCaseList) {
      holdNextCaseList = false;
      expect(requestOwner).toBe(accountId);
      markHeldRequestStarted?.();
      await new Promise<void>((resolve) => {
        releaseHeldResponse = resolve;
      });
      try {
        await route.fulfill({
          status: 200,
          headers: { "content-type": "application/json" },
          body: JSON.stringify([caseFor(requestOwner)]),
        });
      } catch (error) {
        if (!request.failure()) throw error;
      } finally {
        markHeldRouteFinished?.();
      }
      return;
    }
    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify([caseFor(requestOwner)]),
    });
  });

  await signIn(page, "/mi-cuenta/verificaciones");
  const aDeclaration = page.getByText("Datos guardados (revisión #20)");
  const aStatus = page.getByText("Tu solicitud está en revisión documental.");
  await expect(aDeclaration).toBeVisible();
  await expect(aStatus).toBeVisible();
  expect(caseReadsByOwner.get(accountId)).toBe(1);

  holdNextCaseList = true;
  const pendingCaseRequestPromise = page.waitForRequest(
    (request) =>
      new URL(request.url()).pathname === casePath &&
      request.method() === "GET",
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const pendingCaseRequest = await pendingCaseRequestPromise;
  await heldRequestStarted;
  expect(caseReadsByOwner.get(accountId)).toBe(2);

  activeAccountId = accountB;
  const bSessionResponsePromise = page.waitForResponse(async (response) => {
    if (
      !response.url().includes("/api/backend/auth/session") ||
      response.status() !== 200
    )
      return false;
    return (await response.json()).account_id === accountB;
  });
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  const bSessionResponse = await bSessionResponsePromise;
  expect((await bSessionResponse.json()).account_id).toBe(accountB);

  const bDeclaration = page.getByText("Datos guardados (revisión #21)");
  const bStatus = page.getByText(
    "Necesitamos que corrijas o completes tus documentos.",
  );
  await expect(bDeclaration).toBeVisible();
  await expect(bStatus).toBeVisible();
  await expect(aDeclaration).toHaveCount(0);
  await expect(aStatus).toHaveCount(0);
  expect(caseReadsByOwner.get(accountId)).toBe(2);
  expect(caseReadsByOwner.get(accountB)).toBe(1);

  releaseHeldResponse?.();
  await heldRouteFinished;
  expect((await pendingCaseRequest.response())?.status()).toBe(200);
  await expect(bDeclaration).toBeVisible();
  await expect(bStatus).toBeVisible();
  await expect(aDeclaration).toHaveCount(0);
  await expect(aStatus).toHaveCount(0);
  expect(caseReadsByOwner.get(accountId)).toBe(2);
  expect(caseReadsByOwner.get(accountB)).toBe(1);
  expect(providerRequests).toBe(0);
});

test("Mi cuenta no infiere ausencia ante un error y permite consultar nuevamente", async ({
  page,
}) => {
  let caseListReads = 0;
  await installMock(page, (path, method) => {
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      caseListReads += 1;
      return caseListReads === 1
        ? {
            status: 503,
            body: {
              error: {
                code: "upstream_error",
                message: "Synthetic failure",
                request_id: "vi04-status-retry",
              },
            },
          }
        : { status: 200, body: [] };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta");
  const queryError = page.getByText(
    "No pudimos consultar el estado de tu revisión documental.",
    { exact: true },
  );
  await expect(queryError).toBeVisible();
  await expect(queryError).toHaveAttribute("role", "alert");
  await expect(
    page.getByText("Aún no has iniciado una revisión documental propia."),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Reintentar consulta" }).click();
  await expect(
    page.getByText("Aún no has iniciado una revisión documental propia."),
  ).toBeVisible();
  expect(caseListReads).toBe(2);
});

test("Mi cuenta actualiza el expediente cuando la ventana recupera el foco", async ({
  page,
}) => {
  let caseListReads = 0;
  let refreshState = false;
  await installMock(page, (path, method) => {
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      caseListReads += 1;
      return {
        status: 200,
        body: [
          {
            id: "990e8400-e29b-41d4-a716-446655440005",
            purpose: "identity",
            state: refreshState ? "aprobada" : "en_revision",
            revision: caseListReads,
            files: [],
          },
        ],
      };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta");
  await expect(
    page.getByText("Tu solicitud está en revisión documental."),
  ).toBeVisible();
  const readsBeforeFocus = caseListReads;
  expect(readsBeforeFocus).toBeGreaterThan(0);

  refreshState = true;
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  await expect.poll(() => caseListReads).toBeGreaterThan(readsBeforeFocus);

  await expect(
    page.getByText(
      "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Tu solicitud está en revisión documental."),
  ).toHaveCount(0);
});

test("Mi cuenta conserva el estado fresco si el GET de foco anterior falla tarde", async ({
  page,
}) => {
  const casePath = "/api/backend/document-verification/cases";
  let caseListReads = 0;
  let refreshState = false;
  let holdNextCaseList = false;
  let releaseHeldResponse: (() => void) | undefined;
  let markHeldRequestStarted: (() => void) | undefined;
  let markHeldRouteFinished: (() => void) | undefined;
  let providerRequests = 0;
  const heldRequestStarted = new Promise<void>((resolve) => {
    markHeldRequestStarted = resolve;
  });
  const heldRouteFinished = new Promise<void>((resolve) => {
    markHeldRouteFinished = resolve;
  });

  await installMock(page, (path) => {
    if (path.includes("/verifications/didit/")) providerRequests += 1;
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await page.route(`**${casePath}`, async (route: Route) => {
    const request = route.request();
    if (request.method() !== "GET") return route.fallback();

    caseListReads += 1;
    if (holdNextCaseList) {
      holdNextCaseList = false;
      markHeldRequestStarted?.();
      await new Promise<void>((resolve) => {
        releaseHeldResponse = resolve;
      });
      try {
        await route.fulfill({
          status: 503,
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ error: { code: "synthetic_unavailable" } }),
        });
      } catch (error) {
        if (!request.failure()) throw error;
      } finally {
        markHeldRouteFinished?.();
      }
      return;
    }

    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify([
        {
          id: "990e8400-e29b-41d4-a716-446655440034",
          purpose: "identity",
          state: refreshState ? "requiere_antecedentes" : "en_revision",
          revision: caseListReads,
          files: [],
        },
      ]),
    });
  });

  await signIn(page, "/mi-cuenta");
  const pendingStatus = page.getByText(
    "Tu solicitud está en revisión documental.",
  );
  const refreshedStatus = page.getByText(
    "Necesitamos que corrijas o completes tus documentos.",
  );
  const queryError = page.getByText(
    "No pudimos consultar el estado de tu revisión documental.",
    { exact: true },
  );
  await expect(pendingStatus).toBeVisible();
  expect(caseListReads).toBe(1);

  holdNextCaseList = true;
  const heldRequestPromise = page.waitForRequest(
    (request) =>
      new URL(request.url()).pathname === casePath &&
      request.method() === "GET",
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const heldRequest = await heldRequestPromise;
  await heldRequestStarted;
  expect(heldRequest.method()).toBe("GET");
  expect(new URL(heldRequest.url()).pathname).toBe(casePath);
  expect(caseListReads).toBe(2);

  refreshState = true;
  const refreshedResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === casePath &&
      response.request().method() === "GET" &&
      response.status() === 200,
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const refreshedResponse = await refreshedResponsePromise;
  expect(await refreshedResponse.json()).toMatchObject([
    { state: "requiere_antecedentes", revision: 3 },
  ]);
  await expect(refreshedStatus).toBeVisible();
  await expect(pendingStatus).toHaveCount(0);
  expect(caseListReads).toBe(3);

  const lateFailurePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === casePath &&
      response.request().method() === "GET" &&
      response.status() === 503,
  );
  releaseHeldResponse?.();
  const lateFailure = await lateFailurePromise;
  await heldRouteFinished;
  expect(lateFailure.status()).toBe(503);
  expect(lateFailure.request().method()).toBe("GET");
  expect(new URL(lateFailure.url()).pathname).toBe(casePath);
  await expect(refreshedStatus).toBeVisible();
  await expect(pendingStatus).toHaveCount(0);
  await expect(queryError).toHaveCount(0);
  expect(caseListReads).toBe(3);
  expect(providerRequests).toBe(0);
});

test("legacy return URL strips query parameters without reading provider status", async ({
  page,
}) => {
  let providerRequests = 0;
  await installMock(page, (path) => {
    if (path.includes("/verifications/didit/")) providerRequests++;
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta");
  await page.goto(
    "/verificaciones/retorno?attempt_id=770e8400-e29b-41d4-a716-446655440011&token=synthetic",
  );
  await expect(
    page.getByRole("heading", { name: "Este retorno ya no está activo" }),
  ).toBeVisible();
  await expect(page.getByRole("status")).toContainText(
    "La revisión documental propia aún no está habilitada",
  );
  await expect(page).toHaveURL(/\/verificaciones\/retorno$/);
  expect(providerRequests).toBe(0);
});

test("identity declaration screen has no provider CTA or provider request", async ({
  page,
}) => {
  let providerRequests = 0;
  await installMock(page, (path) => {
    if (path.includes("/verifications/didit/")) providerRequests++;
    return { status: 404, body: { error: { code: "not_found" } } };
  });
  await signIn(page, "/mi-cuenta/verificaciones");
  await expect(
    page.getByRole("button", { name: "Guardar datos de identidad" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Verificar identidad con Didit" }),
  ).toHaveCount(0);
  await expect(
    page.getByText(
      /La declaración legal y la revisión documental son pasos separados/,
    ),
  ).toBeVisible();
  expect(providerRequests).toBe(0);
});

test("Verificaciones conserva la declaración de B si el GET inicial demorado de A se libera después del cambio de sesión", async ({
  page,
}) => {
  const accountB = "880e8400-e29b-41d4-a716-446655440016";
  const declarationPath = "/api/backend/verifications/identity-declaration";
  let activeAccountId = accountId;
  let providerRequests = 0;
  const declarationReads = new Map<string, number>();
  let releaseAResponse: (() => void) | undefined;
  let markAStarted: (() => void) | undefined;
  const aRequestStarted = new Promise<void>((resolve) => {
    markAStarted = resolve;
  });
  const declarationFor = (owner: string) => ({
    id:
      owner === accountId
        ? "990e8400-e29b-41d4-a716-446655440030"
        : "990e8400-e29b-41d4-a716-446655440031",
    revision: owner === accountId ? 30 : 31,
    first_name: owner === accountId ? "Alicia A" : "Beatriz B",
    last_name: owner === accountId ? "Apellido A" : "Apellido B",
    date_of_birth: "1990-01-01",
    rut: "000000000",
    id_country: "CHL",
    document_type: "ID",
    consent_version: "synthetic-local-fixture",
    consent_given_at: "2026-11-11T12:00:00Z",
  });

  await installMock(
    page,
    (path) => {
      if (path.includes("/verifications/didit/")) providerRequests += 1;
      return { status: 404, body: { error: { code: "not_found" } } };
    },
    () => activeAccountId,
  );
  await page.route(`**${declarationPath}`, async (route: Route) => {
    const request = route.request();
    if (request.method() !== "GET") return route.fallback();

    const requestOwner = activeAccountId;
    declarationReads.set(
      requestOwner,
      (declarationReads.get(requestOwner) ?? 0) + 1,
    );
    if (requestOwner === accountId) {
      markAStarted?.();
      await new Promise<void>((resolve) => {
        releaseAResponse = resolve;
      });
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify(declarationFor(requestOwner)),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify(declarationFor(requestOwner)),
    });
  });

  await signIn(page, "/mi-cuenta/verificaciones");
  const pendingAResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === declarationPath &&
      response.request().method() === "GET",
  );
  await aRequestStarted;
  expect(declarationReads.get(accountId)).toBe(1);

  activeAccountId = accountB;
  const bSessionResponsePromise = page.waitForResponse(async (response) => {
    if (
      !response.url().includes("/api/backend/auth/session") ||
      response.status() !== 200
    )
      return false;
    return (await response.json()).account_id === accountB;
  });
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  const bSessionResponse = await bSessionResponsePromise;
  expect((await bSessionResponse.json()).account_id).toBe(accountB);

  const bRevision = page.getByText("Datos guardados (revisión #31)");
  await expect(bRevision).toBeVisible();
  await expect(page.getByText("Beatriz B", { exact: true })).toBeVisible();
  await expect(page.getByText("Apellido B", { exact: true })).toBeVisible();
  expect(declarationReads.get(accountB)).toBe(1);
  expect(declarationReads.get(accountId)).toBe(1);

  // B must be rendered before releasing A's held initial declaration response.
  releaseAResponse?.();
  const pendingAResponse = await pendingAResponsePromise;
  expect(pendingAResponse.status()).toBe(200);
  await expect(bRevision).toBeVisible();
  await expect(page.getByText("Beatriz B", { exact: true })).toBeVisible();
  await expect(page.getByText("Apellido B", { exact: true })).toBeVisible();
  await expect(page.getByText("Datos guardados (revisión #30)")).toHaveCount(0);
  await expect(page.getByText("Alicia A", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Apellido A", { exact: true })).toHaveCount(0);
  expect(declarationReads.get(accountId)).toBe(1);
  expect(declarationReads.get(accountB)).toBe(1);
  expect(providerRequests).toBe(0);
});

test("Mi cuenta ignora el error tardío de casos de A después del cambio a B", async ({
  page,
}) => {
  const accountB = "880e8400-e29b-41d4-a716-446655440015";
  const casePath = "/api/backend/document-verification/cases";
  const declarationPath = "/api/backend/verifications/identity-declaration";
  let activeAccountId = accountId;
  let holdNextCaseList = false;
  const caseReadsByOwner = new Map<string, number>();
  let providerRequests = 0;
  let releaseHeldResponse: (() => void) | undefined;
  let markHeldRequestStarted: (() => void) | undefined;
  let markHeldRouteFinished: (() => void) | undefined;
  const heldRequestStarted = new Promise<void>((resolve) => {
    markHeldRequestStarted = resolve;
  });
  const heldRouteFinished = new Promise<void>((resolve) => {
    markHeldRouteFinished = resolve;
  });
  const declarationFor = (owner: string) => ({
    id:
      owner === accountId
        ? "990e8400-e29b-41d4-a716-446655440030"
        : "990e8400-e29b-41d4-a716-446655440031",
    revision: owner === accountId ? 30 : 31,
    first_name: owner === accountId ? "Alicia A" : "Beatriz B",
    last_name: owner === accountId ? "Apellido A" : "Apellido B",
    date_of_birth: owner === accountId ? "1990-01-01" : "1991-02-02",
    rut: "000000000",
    id_country: "CHL",
    document_type: "ID",
    consent_version: "synthetic-local-fixture",
    consent_given_at: "2026-11-11T12:00:00Z",
  });
  const caseFor = (owner: string) => ({
    id:
      owner === accountId
        ? "990e8400-e29b-41d4-a716-446655440032"
        : "990e8400-e29b-41d4-a716-446655440033",
    purpose: "identity",
    state: owner === accountId ? "en_revision" : "requiere_antecedentes",
    revision: owner === accountId ? 32 : 33,
    files: [],
  });

  await installMock(
    page,
    (path, method) => {
      if (path.includes("/verifications/didit/")) providerRequests += 1;
      if (path === declarationPath && method === "GET") {
        return { status: 200, body: declarationFor(activeAccountId) };
      }
      return { status: 404, body: { error: { code: "not_found" } } };
    },
    () => activeAccountId,
  );
  await page.route(`**${casePath}`, async (route: Route) => {
    const request = route.request();
    if (request.method() !== "GET") return route.fallback();

    const requestOwner = activeAccountId;
    caseReadsByOwner.set(
      requestOwner,
      (caseReadsByOwner.get(requestOwner) ?? 0) + 1,
    );
    if (holdNextCaseList) {
      holdNextCaseList = false;
      expect(requestOwner).toBe(accountId);
      markHeldRequestStarted?.();
      await new Promise<void>((resolve) => {
        releaseHeldResponse = resolve;
      });
      try {
        await route.fulfill({
          status: 503,
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ error: { code: "synthetic_unavailable" } }),
        });
      } catch (error) {
        if (!request.failure()) throw error;
      } finally {
        markHeldRouteFinished?.();
      }
      return;
    }
    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify([caseFor(requestOwner)]),
    });
  });

  await signIn(page, "/mi-cuenta/verificaciones");
  const aDeclaration = page.getByText("Datos guardados (revisión #30)");
  const aStatus = page.getByText("Tu solicitud está en revisión documental.");
  await expect(aDeclaration).toBeVisible();
  await expect(aStatus).toBeVisible();
  expect(caseReadsByOwner.get(accountId)).toBe(1);

  holdNextCaseList = true;
  const heldRequestPromise = page.waitForRequest(
    (request) =>
      new URL(request.url()).pathname === casePath &&
      request.method() === "GET",
  );
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  const heldRequest = await heldRequestPromise;
  await heldRequestStarted;
  expect(heldRequest.method()).toBe("GET");
  expect(new URL(heldRequest.url()).pathname).toBe(casePath);
  expect(caseReadsByOwner.get(accountId)).toBe(2);

  activeAccountId = accountB;
  const bSessionResponsePromise = page.waitForResponse(async (response) => {
    if (
      !response.url().includes("/api/backend/auth/session") ||
      response.status() !== 200
    )
      return false;
    return (await response.json()).account_id === accountB;
  });
  await page.evaluate(() => {
    const channel = new BroadcastChannel("espacigo_auth");
    channel.postMessage({ type: "session_changed" });
    channel.close();
  });
  const bSessionResponse = await bSessionResponsePromise;
  expect((await bSessionResponse.json()).account_id).toBe(accountB);

  const bDeclaration = page.getByText("Datos guardados (revisión #31)");
  const bStatus = page.getByText(
    "Necesitamos que corrijas o completes tus documentos.",
  );
  await expect(bDeclaration).toBeVisible();
  await expect(bStatus).toBeVisible();
  await expect(aDeclaration).toHaveCount(0);
  await expect(aStatus).toHaveCount(0);
  expect(caseReadsByOwner.get(accountId)).toBe(2);
  expect(caseReadsByOwner.get(accountB)).toBe(1);

  const lateFailurePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === casePath &&
      response.request().method() === "GET" &&
      response.status() === 503,
  );
  releaseHeldResponse?.();
  const lateFailure = await lateFailurePromise;
  await heldRouteFinished;
  expect(lateFailure.status()).toBe(503);
  expect(lateFailure.request().method()).toBe("GET");
  expect(new URL(lateFailure.url()).pathname).toBe(casePath);
  await expect(bDeclaration).toBeVisible();
  await expect(bStatus).toBeVisible();
  await expect(aDeclaration).toHaveCount(0);
  await expect(aStatus).toHaveCount(0);
  await expect(
    page.getByText("No pudimos consultar el estado de tu revisión documental."),
  ).toHaveCount(0);
  expect(caseReadsByOwner.get(accountId)).toBe(2);
  expect(caseReadsByOwner.get(accountB)).toBe(1);
  expect(providerRequests).toBe(0);
});

test("the running Next BFF returns route_not_allowed for retired provider endpoints", async ({
  request,
}) => {
  const cases = [
    { method: "get", path: "/api/backend/verifications/didit/intent" },
    { method: "post", path: "/api/backend/verifications/didit/intent" },
    {
      method: "get",
      path: "/api/backend/verifications/didit/intent/770e8400-e29b-41d4-a716-446655440011/refresh",
    },
    { method: "post", path: "/api/backend/verifications/didit/webhook" },
    { method: "post", path: "/api/backend/webhooks/didit" },
  ] as const;

  for (const { method, path } of cases) {
    const response = await request[method](path);
    expect(response.status()).toBe(404);
    expect(await response.json()).toMatchObject({
      error: { code: "route_not_allowed" },
    });
  }
});
