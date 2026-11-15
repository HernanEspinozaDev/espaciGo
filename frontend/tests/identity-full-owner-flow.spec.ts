import { expect, test } from "@playwright/test";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const declarationId = "990e8400-e29b-41d4-a716-446655440000";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const sessionToken = "synthetic-full-owner-flow-token";
const roles = ["identity_front", "identity_back", "identity_selfie"] as const;

test.use({ trace: "off", screenshot: "off", video: "off" });

test("owner saves declaration, explicitly captures identity, refreshes clean evidence, then submits", async ({
  page,
}) => {
  let loggedIn = false;
  let declaration: Record<string, unknown> | null = null;
  let declarationWrites = 0;
  let declarationReadAfterSave = false;
  let created = false;
  let submitted = false;
  let revision = 1;
  let etag = '"1"';
  let explicitRefreshRequested = false;
  let explicitCleanReadback = false;
  const uploadedRoles: string[] = [];
  const uploadResponseStates: string[] = [];
  const detailReadbacks: Array<{
    etag: string;
    state: string;
    revision: number;
    fileStates: string[];
  }> = [];
  const mutations: Array<{ method: string; path: string; ifMatch?: string }> =
    [];
  const outsideLoopback: string[] = [];
  const unexpectedSameOrigin: string[] = [];
  const localOrigin = "http://127.0.0.1:4211";
  const allowedPagePaths = new Set([
    "/",
    "/acceso",
    "/recuperar-acceso",
    "/registro",
    "/saas",
    "/pricing",
    "/terminos-y-condiciones",
    "/politica-de-privacidad",
    "/mi-cuenta",
    "/mi-cuenta/verificaciones",
    "/mi-cuenta/documentos",
    "/mi-cuenta/privacidad",
    "/mi-cuenta/seguridad",
    "/mi-cuenta/revisiones",
  ]);
  const allowedPublicAssets = new Set(["/favicon.ico", "/espacigo-mark.svg"]);
  const syntheticScanner = (() => {
    let state: "pendiente" | "limpio" = "pendiente";
    const transitions: string[] = [];
    return {
      get state() {
        return state;
      },
      get transitions() {
        return [...transitions];
      },
      completeAfterPendingUploads(
        completedRoles: readonly string[],
        responseStates: readonly string[],
      ) {
        expect(completedRoles).toEqual([...roles]);
        expect(responseStates).toEqual(roles.map(() => "pendiente"));
        expect(state).toBe("pendiente");
        state = "limpio";
        transitions.push("completeSyntheticScan");
      },
    };
  })();

  const identityCase = () => ({
    id: caseId,
    purpose: "identity",
    state: submitted ? "enviada" : "borrador",
    revision,
    document_format_declared: "legacy",
    files: uploadedRoles.map((role, index) => ({
      id: `880e8400-e29b-41d4-a716-${String(index + 1).padStart(12, "0")}`,
      evidence_role: role,
      state: syntheticScanner.state,
      size: Buffer.byteLength(`synthetic image fixture ${role}`),
      mime: "image/jpeg",
      pages: 1,
    })),
  });
  const completeSyntheticScan = () => {
    syntheticScanner.completeAfterPendingUploads(
      uploadedRoles,
      uploadResponseStates,
    );
    revision += 1;
    etag = `"${revision}"`;
  };

  page.on("request", (request) => {
    if (new URL(request.url()).origin !== localOrigin) {
      outsideLoopback.push(new URL(request.url()).origin);
    }
  });

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin !== localOrigin) {
      return route.abort();
    }
    const requestHeaders = request.headers();
    const acceptHeader = requestHeaders.accept ?? "";
    const isExpectedPageRequest =
      acceptHeader.includes("text/html") ||
      acceptHeader.includes("text/x-component") ||
      requestHeaders.rsc === "1" ||
      requestHeaders["next-router-prefetch"] === "1";
    const isReadOnlyLocalPageOrAsset =
      ["GET", "HEAD"].includes(request.method()) &&
      ((allowedPagePaths.has(url.pathname) && isExpectedPageRequest) ||
        url.pathname.startsWith("/_next/static/") ||
        allowedPublicAssets.has(url.pathname));
    if (isReadOnlyLocalPageOrAsset) {
      return route.fallback();
    }
    unexpectedSameOrigin.push(`${request.method()} ${url.pathname}`);
    return route.abort();
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const headers = request.headers();
    const json = (status: number, body: unknown, responseHeaders = {}) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
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
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      if (declarationWrites > 0 && declaration !== null) {
        declarationReadAfterSave = true;
      }
      return declaration
        ? json(200, declaration)
        : json(404, { error: { code: "not_found" } });
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "POST"
    ) {
      mutations.push({ method, path: pathname });
      declarationWrites += 1;
      expect(declarationWrites).toBe(1);
      const body = request.postDataJSON() as Record<string, unknown>;
      expect(Object.keys(body).sort()).toEqual(
        [
          "first_name",
          "last_name",
          "date_of_birth",
          "rut",
          "id_country",
          "document_type",
          "consent_given",
          "consent_version",
        ].sort(),
      );
      expect(body).toEqual({
        first_name: "Persona",
        last_name: "Sintética de Prueba",
        date_of_birth: "1990-01-02",
        rut: "11.222.333-9",
        id_country: "CHL",
        document_type: "ID",
        consent_given: true,
        consent_version: expect.stringMatching(/^id-consent-[a-z0-9-]+$/),
      });
      declaration = {
        id: declarationId,
        revision: 1,
        first_name: "Persona",
        last_name: "Sintética de Prueba",
        date_of_birth: "1990-01-02",
        rut: "11222333-9",
        id_country: "CHL",
        document_type: "ID",
        consent_version: body.consent_version as string,
        consent_given_at: "2026-11-13T12:00:00Z",
      };
      return json(201, declaration);
    }
    if (
      (pathname === "/api/backend/verifications" ||
        pathname === "/api/backend/verifications/eligibility") &&
      method === "GET"
    ) {
      return json(200, { items: [] });
    }
    if (pathname === "/api/backend/spaces" && method === "GET")
      return json(200, { items: [] });

    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (syntheticScanner.state === "limpio") {
        expect(uploadedRoles).toEqual([...roles]);
        expect(explicitRefreshRequested).toBe(true);
        explicitCleanReadback = true;
      }
      return json(200, created ? [identityCase()] : []);
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      mutations.push({ method, path: pathname, ifMatch: headers["if-match"] });
      expect(declarationWrites).toBe(1);
      expect(declarationReadAfterSave).toBe(true);
      expect(request.postDataJSON()).toEqual({
        purpose: "identity",
        document_format_declared: "legacy",
        capture_consent_given: true,
        capture_consent_version: "identity-capture-local-v1",
      });
      created = true;
      return json(201, identityCase(), { etag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      const snapshot = identityCase();
      const detailEtag = `"${snapshot.revision}"`;
      expect(etag).toBe(detailEtag);
      if (syntheticScanner.state === "limpio") {
        expect(explicitCleanReadback).toBe(true);
      }
      detailReadbacks.push({
        etag: detailEtag,
        state: snapshot.state,
        revision: snapshot.revision,
        fileStates: snapshot.files.map((file) => file.state),
      });
      return json(200, snapshot, { etag: detailEtag });
    }
    if (
      pathname === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      mutations.push({ method, path: pathname, ifMatch: headers["if-match"] });
      expect(syntheticScanner.state).toBe("pendiente");
      expect(headers["if-match"]).toBe(etag);
      expect(headers["content-type"]).toContain("multipart/form-data");
      const multipart = request.postDataBuffer()?.toString("latin1") ?? "";
      expect(multipart.match(/name="file"/g) ?? []).toHaveLength(1);
      expect(multipart.match(/name="evidence_role"/g) ?? []).toHaveLength(1);
      const partNames = Array.from(
        multipart.matchAll(/Content-Disposition: form-data;\s*name="([^"]+)"/g),
        (match) => match[1],
      );
      expect(partNames.sort()).toEqual(["evidence_role", "file"]);
      const roleField = /name="evidence_role"\r\n\r\n([^\r\n]+)/.exec(
        multipart,
      )?.[1];
      expect(roles).toContain(roleField as (typeof roles)[number]);
      const role = roleField as (typeof roles)[number];
      expect(multipart).toContain(`filename="${role}.jpg"`);
      expect(multipart).toContain("Content-Type: image/jpeg");
      expect(multipart).toContain(`synthetic image fixture ${role}`);
      expect(uploadedRoles).not.toContain(role);
      uploadedRoles.push(role);
      revision += 1;
      etag = `"${revision}"`;
      const uploadResponseEtag = etag;
      const uploadResponseRevision = revision;
      const uploadResponse = {
        revision: uploadResponseRevision,
        file: {
          id: `880e8400-e29b-41d4-a716-${String(uploadedRoles.length).padStart(12, "0")}`,
          evidence_role: role,
          state: "pendiente",
          size: Buffer.byteLength(`synthetic image fixture ${role}`),
          mime: "image/jpeg",
          pages: 1,
        },
      };
      uploadResponseStates.push(uploadResponse.file.state);
      return json(201, uploadResponse, { etag: uploadResponseEtag });
    }
    if (
      pathname ===
        `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      mutations.push({ method, path: pathname, ifMatch: headers["if-match"] });
      expect(uploadedRoles).toEqual([...roles]);
      expect(explicitCleanReadback).toBe(true);
      expect(headers["if-match"]).toBe(etag);
      expect(headers["if-match"]).toBe(detailReadbacks.at(-1)?.etag);
      expect(detailReadbacks.at(-1)?.revision).toBe(revision);
      submitted = true;
      revision += 1;
      etag = `"${revision}"`;
      return json(200, identityCase(), { etag });
    }
    return json(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  const unexpectedLocalApiWasAborted = await page.evaluate(async () => {
    try {
      await fetch("/api/unexpected-local-probe");
      return false;
    } catch {
      return true;
    }
  });
  expect(unexpectedLocalApiWasAborted).toBe(true);
  expect(unexpectedSameOrigin).toEqual(["GET /api/unexpected-local-probe"]);
  unexpectedSameOrigin.length = 0;
  const allowlistedPathApiWasAborted = await page.evaluate(async () => {
    try {
      await fetch("/", { headers: { Accept: "application/json" } });
      return false;
    } catch {
      return true;
    }
  });
  expect(allowlistedPathApiWasAborted).toBe(true);
  expect(unexpectedSameOrigin).toEqual(["GET /"]);
  unexpectedSameOrigin.length = 0;
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  const declarationRegion = page.getByRole("region", {
    name: "Declaración de identidad",
  });
  await declarationRegion.getByLabel("Nombres legales").fill("Persona");
  await declarationRegion
    .getByLabel("Apellidos legales")
    .fill("Sintética de Prueba");
  await declarationRegion.getByLabel("Fecha de nacimiento").fill("1990-01-02");
  await declarationRegion.getByLabel("RUN/RUT").fill("11.222.333-9");
  await declarationRegion
    .getByLabel(/Confirmo que usaré solo datos sintéticos/)
    .check();
  await declarationRegion
    .getByRole("button", { name: "Guardar declaración" })
    .click();
  await expect(
    declarationRegion.getByText("Declaración guardada."),
  ).toBeVisible();
  await expect(
    declarationRegion.getByText("11222333-9", { exact: true }),
  ).toBeVisible();
  expect(declarationWrites).toBe(1);
  expect(declarationReadAfterSave).toBe(true);
  expect(declaration).not.toBeNull();
  expect(created).toBe(false);
  expect(uploadedRoles).toEqual([]);
  await page
    .getByRole("link", { name: "Iniciar verificación de identidad" })
    .click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  await page
    .getByLabel("Formato declarado del documento")
    .selectOption("legacy");
  await page.getByLabel(/Confirmo que usaré solo datos sintéticos/).check();
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText(
      "Expediente creado. Puedes cargar los documentos requeridos.",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();

  for (const role of roles) {
    await page.getByLabel("Tipo de evidencia").selectOption(role);
    const fileBytes = Buffer.from(`synthetic image fixture ${role}`);
    await page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB").setInputFiles({
      name: `${role}.jpg`,
      mimeType: "image/jpeg",
      buffer: fileBytes,
    });
    await page.getByRole("button", { name: "Cargar evidencia" }).click();
    await expect(
      page.getByText("Evidencia recibida y pendiente de inspección."),
    ).toBeVisible();
  }
  expect(uploadedRoles).toEqual([...roles]);
  expect(uploadResponseStates).toEqual(roles.map(() => "pendiente"));
  expect(syntheticScanner.state).toBe("pendiente");
  expect(syntheticScanner.transitions).toEqual([]);
  completeSyntheticScan();
  expect(syntheticScanner.state).toBe("limpio");
  expect(syntheticScanner.transitions).toEqual(["completeSyntheticScan"]);
  expect(explicitCleanReadback).toBe(false);
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toHaveCount(0);
  explicitRefreshRequested = true;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Validado · 1 página", { exact: true }),
  ).toHaveCount(3);
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toBeEnabled();
  expect(detailReadbacks).toHaveLength(1);
  expect(detailReadbacks[0]).toEqual({
    etag: '"5"',
    state: "borrador",
    revision: 5,
    fileStates: ["limpio", "limpio", "limpio"],
  });
  await page.getByRole("button", { name: "Enviar a revisión" }).click();
  await expect(page.getByText("Expediente enviado a revisión.")).toBeVisible();
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · Enviada"),
  ).toBeVisible();
  await expect.poll(() => detailReadbacks.length).toBe(2);
  expect(detailReadbacks[1]).toEqual({
    etag: '"6"',
    state: "enviada",
    revision: 6,
    fileStates: ["limpio", "limpio", "limpio"],
  });

  const creates = mutations.filter(
    (item) => item.method === "POST" && item.path.endsWith("/cases"),
  );
  const uploads = mutations.filter(
    (item) => item.method === "POST" && item.path.endsWith("/files"),
  );
  const submits = mutations.filter(
    (item) => item.method === "POST" && item.path.endsWith("/submit"),
  );
  expect(creates).toHaveLength(1);
  expect(uploads).toHaveLength(3);
  expect(submits).toHaveLength(1);
  expect(uploads.map((item) => item.ifMatch)).toEqual(['"1"', '"2"', '"3"']);
  expect(submits[0]?.ifMatch).toBe(detailReadbacks[0]?.etag);
  expect(outsideLoopback).toEqual([]);
  expect(unexpectedSameOrigin).toEqual([]);
});
