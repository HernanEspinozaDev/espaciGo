import { expect, test } from "@playwright/test";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const sessionToken = "synthetic-document-correction-session-token";
const roles = ["identity_front", "identity_back", "identity_selfie"] as const;
type EvidenceRole = (typeof roles)[number];
type EvidenceFile = {
  id: string;
  evidence_role: EvidenceRole;
  state: "pendiente" | "limpio";
  size: number;
  mime: "image/png" | "image/jpeg";
  pages: 1;
};

test.use({ trace: "off", screenshot: "off", video: "off" });

test("owner replaces requested identity evidence and resubmits after a clean scan readback", async ({
  page,
}) => {
  const requestsOutsideLoopback: string[] = [];
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      requestsOutsideLoopback.push(request.url());
    }
  });

  let loggedIn = false;
  let caseState: "requiere_antecedentes" | "en_revision" =
    "requiere_antecedentes";
  let revision = 7;
  let etag = '"7"';
  let uploadCount = 0;
  let submitCount = 0;
  let scanOutcomeAvailable = false;
  let scanTransitionCount = 0;
  const uploadEtags: string[] = [];
  const submitEtags: string[] = [];
  const detailReadEtags: string[] = [];
  const files = new Map<EvidenceRole, EvidenceFile>([
    [
      "identity_front",
      {
        id: "880e8400-e29b-41d4-a716-446655440001",
        evidence_role: "identity_front",
        state: "limpio",
        size: 64,
        mime: "image/png",
        pages: 1,
      },
    ],
    [
      "identity_back",
      {
        id: "880e8400-e29b-41d4-a716-446655440002",
        evidence_role: "identity_back",
        state: "limpio",
        size: 64,
        mime: "image/png",
        pages: 1,
      },
    ],
    [
      "identity_selfie",
      {
        id: "880e8400-e29b-41d4-a716-446655440003",
        evidence_role: "identity_selfie",
        state: "limpio",
        size: 64,
        mime: "image/png",
        pages: 1,
      },
    ],
  ]);
  const originalFrontId = files.get("identity_front")?.id;

  function snapshot() {
    return {
      id: caseId,
      purpose: "identity",
      state: caseState,
      revision,
      document_format_declared: "unsure",
      files: [...files.values()],
    };
  }

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const headers = request.headers();
    const body = request.postDataBuffer()?.toString("latin1") ?? "";
    const role = /name="evidence_role"\r\n\r\n([^\r\n]+)/.exec(body)?.[1] as
      EvidenceRole | undefined;
    const json = (
      status: number,
      value: unknown,
      responseHeaders: Record<string, string> = {},
    ) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
          ...responseHeaders,
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
      if (scanOutcomeAvailable && uploadCount === 1) {
        const pendingFront = files.get("identity_front");
        expect(pendingFront?.state).toBe("pendiente");
        files.set("identity_front", { ...pendingFront!, state: "limpio" });
        revision += 1;
        etag = `"${revision}"`;
        scanOutcomeAvailable = false;
        scanTransitionCount += 1;
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
      expect(uploadCount).toBe(0);
      expect(caseState).toBe("requiere_antecedentes");
      uploadEtags.push(headers["if-match"] ?? "");
      expect(headers["if-match"]).toBe(etag);
      expect(etag).toBe('"7"');
      expect(role).toBe("identity_front");
      expect(body.match(/name="file"/g) ?? []).toHaveLength(1);
      expect(body.match(/name="evidence_role"/g) ?? []).toHaveLength(1);
      expect(body).toContain('filename="synthetic-front-correction.png"');
      expect(body).toContain("synthetic replacement identity front bytes");

      const replacement: EvidenceFile = {
        id: "880e8400-e29b-41d4-a716-446655440004",
        evidence_role: "identity_front",
        state: "pendiente",
        size: Buffer.byteLength("synthetic replacement identity front bytes"),
        mime: "image/png",
        pages: 1,
      };
      expect(replacement.id).not.toBe(originalFrontId);
      files.set("identity_front", replacement);
      uploadCount += 1;
      revision += 1;
      etag = `"${revision}"`;
      return json(201, { revision, file: replacement }, { etag });
    }
    if (
      pathname ===
        `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      submitCount += 1;
      submitEtags.push(headers["if-match"] ?? "");
      expect(uploadCount).toBe(1);
      expect(scanTransitionCount).toBe(1);
      expect(headers["if-match"]).toBe(etag);
      expect(etag).toBe('"9"');
      expect(caseState).toBe("requiere_antecedentes");
      const uploadedFiles = [...files.values()];
      expect(uploadedFiles).toHaveLength(3);
      expect(uploadedFiles.map((file) => file.evidence_role).sort()).toEqual(
        [...roles].sort(),
      );
      expect(uploadedFiles.every((file) => file.state === "limpio")).toBe(true);
      expect(files.get("identity_front")?.id).toBe(
        "880e8400-e29b-41d4-a716-446655440004",
      );
      caseState = "en_revision";
      revision += 1;
      etag = `"${revision}"`;
      return json(200, snapshot(), { etag });
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
    .getByRole("button", {
      name: /Identidad.*revisión 7.*Requiere antecedentes/,
    })
    .click();
  await expect(
    page.getByText("Expediente de identidad · Requiere antecedentes"),
  ).toBeVisible();

  const submitButton = page.getByRole("button", { name: "Enviar a revisión" });
  const frontRoleSummary = page
    .getByText(/Carga un anverso, un reverso y una selfie/)
    .locator("..")
    .getByRole("list")
    .getByRole("listitem")
    .filter({ hasText: "Anverso" });
  await expect(submitButton).toBeEnabled();
  await page.getByLabel("Tipo de evidencia").selectOption("identity_front");
  await page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-front-correction.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic replacement identity front bytes"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText("Evidencia recibida y pendiente de inspección."),
  ).toBeVisible();
  await expect(
    frontRoleSummary.getByText("En inspección", { exact: true }),
  ).toBeVisible();
  await expect(submitButton).toHaveCount(0);
  expect(submitCount).toBe(0);
  expect(files.size).toBe(3);
  expect(
    [...files.values()].filter((file) => file.state === "pendiente"),
  ).toHaveLength(1);

  scanOutcomeAvailable = true;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · Requiere antecedentes"),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toBeEnabled();
  await expect(
    frontRoleSummary.getByText("Validado", { exact: true }),
  ).toBeVisible();
  expect(detailReadEtags.at(-1)).toBe('"9"');
  expect(scanTransitionCount).toBe(1);
  expect(submitCount).toBe(0);

  await page.getByRole("button", { name: "Enviar a revisión" }).click();
  await expect(
    page.getByText("Expediente de identidad · En revisión"),
  ).toBeVisible();

  expect(uploadCount).toBe(1);
  expect(uploadEtags).toEqual(['"7"']);
  expect(submitCount).toBe(1);
  expect(submitEtags).toEqual(['"9"']);
  expect(requestsOutsideLoopback).toEqual([]);
});
