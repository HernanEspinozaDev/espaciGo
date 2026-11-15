import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const caseId = "770e8400-e29b-41d4-a716-446655440000";
const sessionToken = "synthetic-document-session-token";
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

test("owner validates identity evidence and submits only three clean roles", async ({
  page,
}) => {
  const requestsOutsideLoopback: string[] = [];
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      requestsOutsideLoopback.push(request.url());
    }
  });
  let loggedIn = false;
  let created = false;
  let caseState = "borrador";
  let revision = 1;
  let etag = '"1"';
  let submitCount = 0;
  let duplicateFile: EvidenceFile | null = null;
  const files = new Map<EvidenceRole, EvidenceFile>();
  const uploadRoles: Array<EvidenceRole | undefined> = [];

  function snapshot() {
    const snapshotFiles = [
      ...files.values(),
      ...(duplicateFile ? [duplicateFile] : []),
    ];
    if (duplicateFile) {
      expect(snapshotFiles).toHaveLength(4);
      expect(new Set(snapshotFiles.map((file) => file.id)).size).toBe(4);
      expect(
        snapshotFiles.filter((file) => file.evidence_role === "identity_back"),
      ).toHaveLength(2);
    }
    return {
      id: caseId,
      purpose: "identity",
      state: caseState,
      revision,
      document_format_declared: "unsure",
      files: snapshotFiles,
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
      for (const [fileRole, file] of files) {
        files.set(fileRole, { ...file, state: "limpio" });
      }
      return json(200, created ? [snapshot()] : []);
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      expect(request.postDataJSON()).toEqual({
        purpose: "identity",
        document_format_declared: "unsure",
        capture_consent_given: true,
        capture_consent_version: "identity-capture-local-v1",
      });
      created = true;
      return json(201, snapshot(), { etag });
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
      uploadRoles.push(role);
      expect(headers["if-match"]).toBe(etag);
      expect(roles).toContain(role);
      expect(body).toContain('name="file"');
      expect(body.match(/name="file"/g) ?? []).toHaveLength(1);
      expect(body.match(/name="evidence_role"/g) ?? []).toHaveLength(1);
      const evidenceRole = role as EvidenceRole;
      const mime =
        evidenceRole === "identity_back" ? "image/jpeg" : "image/png";
      expect(body).toContain(`Content-Type: ${mime}`);
      expect(body).toContain(
        `filename="${evidenceRole}.${mime === "image/jpeg" ? "jpg" : "png"}"`,
      );
      expect(body).toContain(`synthetic ${evidenceRole} evidence bytes`);
      const uploaded: EvidenceFile = {
        id:
          evidenceRole === "identity_front"
            ? "880e8400-e29b-41d4-a716-446655440001"
            : evidenceRole === "identity_back"
              ? "880e8400-e29b-41d4-a716-446655440002"
              : "880e8400-e29b-41d4-a716-446655440003",
        evidence_role: evidenceRole,
        state: "pendiente",
        size: 32,
        mime,
        pages: 1,
      };
      files.set(evidenceRole, uploaded);
      revision += 1;
      etag = `"${revision}"`;
      return json(201, { revision, file: uploaded }, { etag });
    }
    if (
      pathname ===
        `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      submitCount += 1;
      expect(headers["if-match"]).toBe(etag);
      expect([...files.keys()].sort()).toEqual([...roles].sort());
      expect([...files.values()].every((file) => file.state === "limpio")).toBe(
        true,
      );
      caseState = "en_revision";
      revision += 1;
      etag = `"${revision}"`;
      return json(200, snapshot(), { etag });
    }
    return json(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta");
  await page.getByLabel("Correo electrónico").fill("owner@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  await page.getByRole("link", { name: "Documentos", exact: true }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/documentos$/);
  await expect(
    page
      .getByRole("navigation", { name: "Navegación de cuenta" })
      .getByRole("link", { name: "Documentos", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  expect(requestsOutsideLoopback).toEqual([]);
  await page.getByLabel(/Confirmo que usaré solo datos sintéticos/).check();
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();
  await expect(page.getByLabel("Tipo de evidencia")).toBeVisible();
  await expect(
    page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB"),
  ).toHaveAttribute("accept", "image/jpeg,image/png");
  await page.setViewportSize({ width: 1365, height: 900 });
  const desktopAxe = await new AxeBuilder({ page }).analyze();
  expect(desktopAxe.violations.map((violation) => violation.id)).toEqual([]);
  await page.setViewportSize({ width: 390, height: 844 });
  const mobileAxe = await new AxeBuilder({ page }).analyze();
  expect(mobileAxe.violations.map((violation) => violation.id)).toEqual([]);
  await page.getByLabel("Tipo de evidencia").selectOption("identity_front");
  await page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-identity.pdf",
    mimeType: "application/pdf",
    buffer: Buffer.from("synthetic invalid identity evidence"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText(
      "Para identidad usa solo una imagen JPEG o PNG de hasta 10 MiB.",
    ),
  ).toBeVisible();
  expect(uploadRoles).toEqual([]);
  await page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-oversize.png",
    mimeType: "image/png",
    buffer: Buffer.alloc(10 * 1024 * 1024 + 1),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText(
      "Para identidad usa solo una imagen JPEG o PNG de hasta 10 MiB.",
    ),
  ).toBeVisible();
  expect(uploadRoles).toEqual([]);

  for (const role of roles) {
    await page.getByLabel("Tipo de evidencia").selectOption(role);
    const mimeType = role === "identity_back" ? "image/jpeg" : "image/png";
    const filename = `${role}.${mimeType === "image/jpeg" ? "jpg" : "png"}`;
    const payload = `synthetic ${role} evidence bytes`;
    await page.getByLabel("Imagen JPEG o PNG · máximo 10 MiB").setInputFiles({
      name: filename,
      mimeType,
      buffer: Buffer.from(payload),
    });
    await page.getByRole("button", { name: "Cargar evidencia" }).click();
    await expect(
      page.getByText("Evidencia recibida y pendiente de inspección."),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Enviar a revisión" }),
    ).toHaveCount(0);
    await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  }

  await expect(
    page.getByRole("button", { name: "Crear expediente" }),
  ).toBeEnabled();
  const submitButton = page.getByRole("button", { name: "Enviar a revisión" });
  await expect(submitButton).toBeEnabled();
  await page.setViewportSize({ width: 1365, height: 900 });
  const completedDesktopAxe = await new AxeBuilder({ page }).analyze();
  expect(
    completedDesktopAxe.violations.map((violation) => ({
      id: violation.id,
      targets: violation.nodes.map((node) => node.target),
      summaries: violation.nodes.map((node) => node.failureSummary),
    })),
  ).toEqual([]);
  await page.setViewportSize({ width: 390, height: 844 });
  const completedMobileAxe = await new AxeBuilder({ page }).analyze();
  expect(
    completedMobileAxe.violations.map((violation) => ({
      id: violation.id,
      targets: violation.nodes.map((node) => node.target),
      summaries: violation.nodes.map((node) => node.failureSummary),
    })),
  ).toEqual([]);
  const dimensions = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
  await expect(
    page.getByText("Anverso", { exact: true }).first(),
  ).toBeVisible();
  await expect(
    page.getByText("Reverso", { exact: true }).first(),
  ).toBeVisible();
  await expect(page.getByText("Selfie", { exact: true }).first()).toBeVisible();
  const backFile = files.get("identity_back");
  if (!backFile) throw new Error("Synthetic back evidence was not uploaded");
  duplicateFile = {
    ...backFile,
    id: "880e8400-e29b-41d4-a716-446655440004",
  };
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Revisar archivos", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toHaveCount(0);
  duplicateFile = null;
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByRole("button", { name: "Enviar a revisión" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Enviar a revisión" }).click();
  await expect(
    page.getByText(/Expediente de identidad · En revisión/),
  ).toBeVisible();

  expect(uploadRoles).toEqual(roles);
  expect(submitCount).toBe(1);
});
