import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";
import { identityDeclarationConsentVersion } from "../src/features/identity-declaration/domain/identity-declaration";

test.use({ trace: "off", screenshot: "off", video: "off" });

const requestsOutsideLoopback = new WeakMap<Page, boolean>();
test.beforeEach(async ({ page }) => {
  requestsOutsideLoopback.set(page, false);
  page.on("request", (request) => {
    const url = request.url();
    if (!url.startsWith("http://127.0.0.1:"))
      requestsOutsideLoopback.set(page, true);
  });
});
test.afterEach(async ({ page }) => {
  expect(requestsOutsideLoopback.get(page)).toBe(false);
});

type MockResult = {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
};

type GatewayResponder = (
  pathname: string,
  method: string,
  request: Request,
) => MockResult | Promise<MockResult>;

async function mockGateway(
  page: Page,
  responder: GatewayResponder,
): Promise<void> {
  const bff = createBffFixtureState();
  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const result = bff.adapt(
      request,
      await responder(pathname, request.method(), request),
    );
    if (!result) throw new Error("Missing controlled BFF reply");
    const isBinary =
      Buffer.isBuffer(result.body) || typeof result.body === "string";
    await route.fulfill({
      status: result.status,
      headers: {
        "cache-control": "no-store",
        "content-type": "application/json; charset=utf-8",
        ...(result.headers ?? {}),
      },
      body:
        result.status === 204
          ? ""
          : isBinary
            ? (result.body as Buffer | string)
            : JSON.stringify(result.body ?? {}),
    });
  });
}

function apiError(
  status: number,
  code: string,
  headers?: Record<string, string>,
): MockResult {
  return {
    status,
    headers,
    body: {
      error: {
        code,
        message: `Synthetic error for code: ${code}`,
      },
    },
  };
}

const accountId = "770e8400-e29b-41d4-a716-446655440000";
const samplePngBuffer = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
  "base64",
);
const sampleZipBuffer = Buffer.from(
  "UEsFBgAAAAAAAAAAAAAAAAAAAAAAAA==",
  "base64",
);

async function loginAndNavigate(
  page: Page,
  targetPath: string = "/mi-cuenta",
): Promise<void> {
  await page.goto(`/acceso?return_to=${encodeURIComponent(targetPath)}`);
  await page
    .getByLabel("Correo electrónico")
    .fill("phase6-tester@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("SecurePass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === targetPath);
}

test("@phase6 @controlled /mi-cuenta: 404 inicial muestra estado sin configurar y permite crear perfil con PUT; actualizar perfil envía PUT /profile con nombre y teléfono de 9 dígitos", async ({
  page,
}) => {
  let currentProfile: {
    display_name: string;
    phone?: string | null;
    updated_at: string;
  } | null = null;
  let putProfileCount = 0;
  let diditRequestCount = 0;
  let lastPutBody: Record<string, unknown> | null = null;

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname.includes("/verifications/didit/")) {
      diditRequestCount += 1;
      return apiError(404, "not_found");
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return { status: 200, body: [] };
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      if (!currentProfile) return apiError(404, "profile_not_found");
      return { status: 200, body: currentProfile };
    }
    if (pathname === "/api/backend/profile" && method === "PUT") {
      putProfileCount += 1;
      const body = JSON.parse(request.postData() ?? "{}") as {
        display_name: string;
        phone?: string;
      };
      lastPutBody = body;
      currentProfile = {
        display_name: body.display_name,
        phone: body.phone ?? null,
        updated_at: new Date().toISOString(),
      };
      return { status: 200, body: currentProfile };
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET") {
      return apiError(404, "photo_not_found");
    }
    if (pathname === "/api/backend/payout-account" && method === "GET") {
      return apiError(404, "payout_not_found");
    }
    if (pathname === "/api/backend/rights-requests" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");

  await expect(
    page.getByRole("heading", { name: "Verificación de la cuenta" }),
  ).toBeVisible();
  await expect(
    page.getByText("Aún no has iniciado una revisión documental propia."),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Ver declaración" }),
  ).toHaveAttribute("href", "/mi-cuenta/verificaciones");
  expect(diditRequestCount).toBe(0);

  // 1. Estado inicial sin configurar (404)
  await expect(
    page.getByText(
      "Tu perfil aún no está configurado. Completa tus datos para crearlo.",
    ),
  ).toBeVisible();
  const createButton = page.getByRole("button", { name: "Crear perfil" });
  await expect(createButton).toBeVisible();

  // 2. Crear perfil con PUT
  const nameInput = page.getByLabel("Nombre visible");
  const phoneInput = page.getByLabel(/Teléfono móvil/i);
  await nameInput.fill("Carlos Santana");
  await phoneInput.fill("912345678");
  await createButton.click();

  await expect(page.getByText("Perfil guardado correctamente.")).toBeVisible();
  expect(putProfileCount).toBe(1);
  expect(lastPutBody).toEqual({
    display_name: "Carlos Santana",
    phone: "912345678",
  });
  await expect(
    page.getByRole("button", { name: "Guardar cambios" }),
  ).toBeVisible();

  // 3. Actualizar perfil enviando nuevo nombre y teléfono de 9 dígitos
  await nameInput.fill("Carlos Santana Modificado");
  await phoneInput.fill("+56 9 8765 4321");
  await page.getByRole("button", { name: "Guardar cambios" }).click();

  await expect(page.getByText("Perfil guardado correctamente.")).toBeVisible();
  expect(putProfileCount).toBe(2);
  expect(lastPutBody).toEqual({
    display_name: "Carlos Santana Modificado",
    phone: "987654321",
  });
  expect(diditRequestCount).toBe(0);
});

test("@phase6 @controlled foto sintética: generar foto (PUT /profile/photo) carga contenido Blob autenticado (/profile/photo/content), muestra preview con aviso sintético visible y retirar (DELETE /profile/photo) limpia el recurso y revoca Blob URL", async ({
  page,
}) => {
  let hasPhoto = false;
  let putPhotoCount = 0;
  let deletePhotoCount = 0;
  let getContentCount = 0;
  let lastIdempotencyKey: string | null = null;

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: "Elena Morales",
          phone: "912345678",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET") {
      if (!hasPhoto) return apiError(404, "photo_not_found");
      return {
        status: 200,
        body: {
          id: "550e8400-e29b-41d4-a716-446655440020",
          fixture_code: "synthetic-png-v1",
          mime_type: "image/png",
          sha256: "e".repeat(64),
          size_bytes: samplePngBuffer.length,
          created_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "PUT") {
      putPhotoCount += 1;
      lastIdempotencyKey = request.headers()["idempotency-key"] ?? null;
      hasPhoto = true;
      return { status: 204 };
    }
    if (pathname === "/api/backend/profile/photo/content" && method === "GET") {
      getContentCount += 1;
      expect(request.headers()["authorization"]).toBeUndefined();
      return {
        status: 200,
        headers: { "content-type": "image/png" },
        body: samplePngBuffer,
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "DELETE") {
      deletePhotoCount += 1;
      hasPhoto = false;
      return { status: 204 };
    }
    if (pathname === "/api/backend/payout-account" && method === "GET") {
      return apiError(404, "payout_not_found");
    }
    if (pathname === "/api/backend/rights-requests" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");

  // Aviso sintético visible permanentemente
  await expect(
    page.getByText(
      /La foto se descarga como objeto Blob privado autenticado mediante encabezados estrictos/,
    ),
  ).toBeVisible();

  const generateButton = page.getByRole("button", {
    name: "Generar foto sintética",
  });
  await expect(generateButton).toBeVisible();
  const photoPreview = page.getByAltText(
    "Foto de perfil sintética del usuario",
  );
  await expect(photoPreview).not.toBeVisible();

  // Generar foto sintética
  await generateButton.click();
  await expect(
    page.getByText("Foto sintética de ensayo generada con éxito."),
  ).toBeVisible();
  expect(putPhotoCount).toBe(1);
  expect(lastIdempotencyKey).toBeTruthy();
  expect(getContentCount).toBeGreaterThanOrEqual(1);
  await expect(photoPreview).toBeVisible();

  const deleteButton = page.getByRole("button", { name: "Retirar foto" });
  await expect(deleteButton).toBeVisible();

  // Retirar foto
  await deleteButton.click();
  await expect(page.getByText("Foto de perfil eliminada.")).toBeVisible();
  expect(deletePhotoCount).toBe(1);
  await expect(photoPreview).not.toBeVisible();
  await expect(deleteButton).not.toBeVisible();
});

test("@phase6 @controlled cuenta de cobro fake: PUT /payout-account crea referencia sintética con aviso de ensayo visible permanente, DELETE /payout-account la revoca", async ({
  page,
}) => {
  let payoutActive = false;
  let putPayoutCount = 0;
  let deletePayoutCount = 0;

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["anfitrion"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["anfitrion"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: "Marcos Valdés",
          phone: "912345678",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET") {
      return apiError(404, "photo_not_found");
    }
    if (pathname === "/api/backend/payout-account" && method === "GET") {
      if (!payoutActive) return apiError(404, "payout_not_found");
      return {
        status: 200,
        body: {
          reference_id: "payout-synth-ref-4048",
          status: "activa_fake",
          notice: "Referencia local ficticia; no contiene datos bancarios.",
        },
      };
    }
    if (pathname === "/api/backend/payout-account" && method === "PUT") {
      putPayoutCount += 1;
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      payoutActive = true;
      return {
        status: 200,
        body: {
          reference_id: "payout-synth-ref-4048",
          status: "activa_fake",
          notice: "Referencia local ficticia; no contiene datos bancarios.",
        },
      };
    }
    if (pathname === "/api/backend/payout-account" && method === "DELETE") {
      deletePayoutCount += 1;
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      payoutActive = false;
      return { status: 204 };
    }
    if (pathname === "/api/backend/rights-requests" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");

  // Aviso de ensayo visible permanente en tarjeta
  await expect(
    page.getByText(
      /Entorno de ensayo: No se utilizan ni requieren cuentas bancarias reales/,
    ),
  ).toBeVisible();
  await expect(
    page.getByText("No tienes ninguna cuenta de cobro configurada."),
  ).toBeVisible();

  const createButton = page.getByRole("button", {
    name: "Crear cuenta de cobro sintética",
  });
  await expect(createButton).toBeVisible();

  // Crear cuenta fake con PUT
  await createButton.click();
  await expect(
    page.getByText("Cuenta de cobro sintética configurada para ensayos."),
  ).toBeVisible();
  expect(putPayoutCount).toBe(1);

  // Verificar detalles y aviso de ensayo de referencia
  await expect(page.getByText("payout-synth-ref-4048")).toBeVisible();
  await expect(page.getByText("activa_fake")).toBeVisible();
  await expect(
    page.getByText("Referencia local ficticia; no contiene datos bancarios."),
  ).toBeVisible();

  const revokeButton = page.getByRole("button", {
    name: "Revocar cuenta de cobro",
  });
  await expect(revokeButton).toBeVisible();

  // Revocar con DELETE
  await revokeButton.click();
  await expect(
    page.getByText("Cuenta de cobro sintética revocada."),
  ).toBeVisible();
  expect(deletePayoutCount).toBe(1);
  await expect(
    page.getByText("No tienes ninguna cuenta de cobro configurada."),
  ).toBeVisible();
  await expect(createButton).toBeVisible();
});

test("@phase6 @controlled /mi-cuenta/privacidad: registrar solicitud ARCO (POST /rights-requests), listar solicitudes vigentes (GET /rights-requests), y descargar exportaciones JSON (/privacy/export) y ZIP (/privacy/export/archive) verificando que se invoque la descarga Blob con cleanup", async ({
  page,
}) => {
  const registeredRequests: Array<{
    id: string;
    kind: string;
    state: string;
    created_at: string;
  }> = [];
  let jsonExportCount = 0;
  let zipExportCount = 0;

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: "Ana Lucía",
          phone: "912345678",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/rights-requests" && method === "GET") {
      return { status: 200, body: { items: registeredRequests } };
    }
    if (pathname === "/api/backend/rights-requests" && method === "POST") {
      const body = JSON.parse(request.postData() ?? "{}") as { type: string };
      const created = {
        id: `req-${Date.now()}`,
        kind: body.type,
        state: "registrada",
        created_at: new Date().toISOString(),
      };
      registeredRequests.push(created);
      return { status: 201, body: created };
    }
    if (pathname === "/api/backend/privacy/export" && method === "GET") {
      jsonExportCount += 1;
      expect(request.headers()["authorization"]).toBeUndefined();
      return {
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          account_id: accountId,
          profile: { display_name: "Ana Lucía" },
          synthetic: true,
        }),
      };
    }
    if (
      pathname === "/api/backend/privacy/export/archive" &&
      method === "GET"
    ) {
      zipExportCount += 1;
      expect(request.headers()["authorization"]).toBeUndefined();
      return {
        status: 200,
        headers: { "content-type": "application/zip" },
        body: sampleZipBuffer,
      };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");
  await page
    .getByRole("navigation", { name: "Secciones de mi cuenta" })
    .getByRole("link", { name: "Privacidad" })
    .click();
  await page.waitForURL((url) => url.pathname === "/mi-cuenta/privacidad");

  // 1. Registrar derecho de acceso
  await page
    .getByRole("button", { name: "Solicitar derecho de acceso" })
    .click();
  await expect(
    page.getByText("Solicitud de acceso a datos registrada exitosamente."),
  ).toBeVisible();
  expect(registeredRequests.some((r) => r.kind === "acceso")).toBe(true);

  // 2. Registrar derecho de supresión
  await page
    .getByRole("button", { name: "Solicitar derecho de supresión" })
    .click();
  await expect(
    page.getByText("Solicitud de supresión de datos registrada exitosamente."),
  ).toBeVisible();
  expect(registeredRequests.some((r) => r.kind === "supresion")).toBe(true);

  // 3. Comprobar listado de solicitudes vigentes con badges
  await expect(page.getByText("Solicitudes registradas")).toBeVisible();
  await expect(page.getByText("acceso", { exact: true })).toBeVisible();
  await expect(page.getByText("supresion", { exact: true })).toBeVisible();
  expect(await page.getByText("registrada").count()).toBeGreaterThanOrEqual(2);

  // 4. Descargar exportación JSON
  await page
    .getByRole("button", { name: "Descargar exportación JSON" })
    .click();
  await expect(
    page.getByText("Exportación JSON descargada correctamente."),
  ).toBeVisible();
  expect(jsonExportCount).toBe(1);

  // 5. Descargar archivo ZIP
  await page.getByRole("button", { name: "Descargar archivo ZIP" }).click();
  await expect(
    page.getByText("Archivo ZIP descargado correctamente."),
  ).toBeVisible();
  expect(zipExportCount).toBe(1);
});

test("@phase6 @controlled /mi-cuenta/seguridad: actualizar contraseña (POST /auth/password/change) con feedback accesible", async ({
  page,
}) => {
  let changePasswordCount = 0;
  let lastChangePayload: Record<string, unknown> | null = null;

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: "David Silva",
          phone: "912345678",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/auth/password/change" && method === "POST") {
      changePasswordCount += 1;
      expect(request.headers()["authorization"]).toBeUndefined();
      lastChangePayload = JSON.parse(request.postData() ?? "{}");
      return { status: 200, body: {} };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");
  await page.getByRole("link", { name: "Seguridad" }).click();
  await page.waitForURL((url) => url.pathname === "/mi-cuenta/seguridad");

  const currentPass = page.getByLabel("Contraseña actual");
  const newPass = page.getByLabel("Nueva contraseña", { exact: true });
  const confirmPass = page.getByLabel("Confirmar nueva contraseña");
  const submitBtn = page.getByRole("button", { name: "Cambiar contraseña" });

  await currentPass.fill("OldPass123!");
  await newPass.fill("NewStrongPass456!");
  await confirmPass.fill("NewStrongPass456!");
  await submitBtn.click();

  const feedback = page.locator('[role="status"]');
  await expect(feedback).toContainText(
    "Tu contraseña ha sido actualizada con éxito.",
  );
  expect(changePasswordCount).toBe(1);
  expect(lastChangePayload).toEqual({
    current_password: "OldPass123!",
    new_password: "NewStrongPass456!",
    confirm_password: "NewStrongPass456!",
  });

  // Inputs blanqueados tras envío exitoso
  await expect(currentPass).toHaveValue("");
  await expect(newPass).toHaveValue("");
  await expect(confirmPass).toHaveValue("");
});

test("@phase6 @controlled /mi-cuenta/verificaciones: visualización de elegibilidad KYC/KYB, listado de verificaciones con badges de estado, detalle con historial, reintento de verificación rechazada (POST /verifications/{id}/retry), y carga/inspección de evidencia 'synthetic-png-v1' con Blob URL privada", async ({
  page,
}) => {
  const caseId = "550e8400-e29b-41d4-a716-446655440010";
  let caseState = "rechazada";
  let retryCount = 0;
  let uploadCount = 0;
  const evidenceList: Array<{
    id: string;
    fixture_code: string;
    mime_type: string;
    sha256: string;
    size_bytes: number;
    created_at: string;
  }> = [];

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["anfitrion"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["anfitrion"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (
      pathname === "/api/backend/verifications/eligibility" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          items: [
            { type: "kyc", eligible: true },
            {
              type: "kyb",
              eligible: false,
              revocation_reason_code: "requiere_rut_empresa",
            },
          ],
        },
      };
    }
    if (pathname === "/api/backend/verifications" && method === "GET") {
      return {
        status: 200,
        body: {
          items: [
            {
              id: caseId,
              type: "kyc",
              state: caseState,
              created_at: "2026-10-25T08:00:00Z",
            },
            {
              id: "550e8400-e29b-41d4-a716-446655440099",
              type: "kyb",
              state: "aprobada",
              created_at: "2026-10-25T07:00:00Z",
            },
          ],
        },
      };
    }
    if (
      pathname === `/api/backend/verifications/${caseId}` &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          id: caseId,
          type: "kyc",
          state: caseState,
          created_at: "2026-10-25T08:00:00Z",
        },
      };
    }
    if (
      pathname === `/api/backend/verifications/${caseId}/history` &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          items: [
            {
              sequence: 1,
              action: "revision_documental",
              from_state: "en_revision",
              to_state: "rechazada",
              reason_code: "documento_borroso",
              occurred_at: "2026-10-25T08:30:00Z",
            },
          ],
        },
      };
    }
    if (
      pathname === `/api/backend/verifications/${caseId}/evidence` &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: { items: evidenceList },
      };
    }
    if (
      pathname === `/api/backend/verifications/${caseId}/retry` &&
      method === "POST"
    ) {
      retryCount += 1;
      expect(request.headers()["idempotency-key"]).toBeTruthy();
      const body = JSON.parse(request.postData() ?? "{}");
      expect(body.correction_code).toBe("fixture_vigente_actualizado");
      caseState = "en_revision";
      return {
        status: 200,
        body: {
          id: caseId,
          type: "kyc",
          state: "en_revision",
          created_at: "2026-10-25T08:00:00Z",
        },
      };
    }
    if (
      pathname === `/api/backend/verifications/${caseId}/evidence` &&
      method === "POST"
    ) {
      uploadCount += 1;
      const body = JSON.parse(request.postData() ?? "{}");
      expect(body.fixture_code).toBe("synthetic-png-v1");
      const created = {
        id: "evi-synth-101",
        fixture_code: "synthetic-png-v1",
        mime_type: "image/png",
        sha256: "f".repeat(64),
        size_bytes: samplePngBuffer.length,
        created_at: new Date().toISOString(),
      };
      evidenceList.push(created);
      return { status: 201, body: created };
    }
    if (
      pathname ===
        `/api/backend/verifications/${caseId}/evidence/evi-synth-101` &&
      method === "GET"
    ) {
      return {
        status: 200,
        headers: { "content-type": "image/png" },
        body: samplePngBuffer,
      };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");
  await page.getByRole("link", { name: "Verificaciones" }).click();
  await page.waitForURL((url) => url.pathname === "/mi-cuenta/verificaciones");

  // 1. Elegibilidad KYC / KYB
  await expect(
    page.getByRole("heading", {
      name: "Verificación de usuario (Identidad)",
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", {
      name: "Verificación de espacios",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByText("requiere_rut_empresa")).toBeVisible();
  await expect(page.getByText("Elegible", { exact: true })).toBeVisible();
  await expect(page.getByText("No elegible", { exact: true })).toBeVisible();

  // 2. Listado con badges de estado
  await expect(page.getByText("Rechazada")).toBeVisible();
  await expect(page.getByText("Aprobada")).toBeVisible();

  // 3. Detalle con historial
  const detailButtons = page.getByRole("button", { name: "Ver detalle" });
  await detailButtons.first().click();
  await expect(
    page.getByRole("heading", {
      level: 2,
      name: "Detalle de Verificación: Usuario (Identidad)",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByText("documento_borroso")).toBeVisible();
  await expect(page.getByText("REVISION_DOCUMENTAL")).toBeVisible();

  // 4. Reintentar verificación rechazada
  const retryButton = page.getByRole("button", {
    name: "Reintentar verificación",
  });
  await expect(retryButton).toBeVisible();
  await retryButton.click();
  expect(retryCount).toBe(1);
  await expect(page.getByText("En revisión").first()).toBeVisible();

  // 5. Cargar e inspeccionar evidencia 'synthetic-png-v1'
  const uploadButton = page.getByRole("button", {
    name: "Subir evidencia sintética",
  });
  await expect(uploadButton).toBeVisible();
  await uploadButton.click();
  expect(uploadCount).toBe(1);
  await expect(
    page.getByText(
      "Evidencia sintética ('synthetic-png-v1') cargada exitosamente.",
    ),
  ).toBeVisible();
  await expect(page.getByText("synthetic-png-v1").first()).toBeVisible();

  // Inspeccionar evidencia con Blob URL
  const viewEvidenceButton = page.getByRole("button", {
    name: "Ver",
    exact: true,
  });
  await viewEvidenceButton.first().click();
  await expect(
    page.getByText("Visor de evidencia sintética (Blob privado)"),
  ).toBeVisible();
  const visorImg = page.getByAltText("Evidencia sintética de verificación");
  await expect(visorImg).toBeVisible();

  // Cerrar visor
  await page.getByRole("button", { name: "Cerrar visor" }).click();
  await expect(visorImg).not.toBeVisible();
});

test("@phase6 @controlled Mis espacios: estados de carga, error, vacío y etiquetas singulares", async ({
  page,
}) => {
  let releaseFirstResponse!: (result: MockResult) => void;
  const firstResponse = new Promise<MockResult>((resolve) => {
    releaseFirstResponse = resolve;
  });
  let spacesRequestCount = 0;

  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: "Usuario Ensayo",
          phone: "912345678",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET")
      return apiError(404, "photo_not_found");
    if (pathname === "/api/backend/payout-account" && method === "GET")
      return apiError(404, "payout_not_found");
    if (pathname === "/api/backend/rights-requests" && method === "GET")
      return { status: 200, body: { items: [] } };
    if (pathname === "/api/backend/spaces" && method === "GET") {
      spacesRequestCount += 1;
      if (spacesRequestCount === 1) return firstResponse;
      if (spacesRequestCount === 2) return { status: 200, body: { items: [] } };
      return {
        status: 200,
        body: {
          items: [
            {
              id: "space-summary-draft",
              category_code: "oficina",
              category_name: "Oficinas y Cowork",
              title: "Oficina en borrador",
              state: "borrador",
              base_price_clp: 20000,
              rate_unit: "hora",
              area_m2: 30,
              capacity: 4,
              region_code: "13",
              comuna_code: "Providencia",
            },
            {
              id: "space-summary-published",
              category_code: "oficina",
              category_name: "Oficinas y Cowork",
              title: "Oficina publicada",
              state: "activa",
              base_price_clp: 25000,
              rate_unit: "hora",
              area_m2: 35,
              capacity: 5,
              region_code: "13",
              comuna_code: "Las Condes",
            },
            {
              id: "space-summary-archived",
              category_code: "bodega",
              category_name: "Bodegas y Mini-bodegas",
              title: "Bodega archivada",
              state: "oculta",
              base_price_clp: 30000,
              rate_unit: "dia",
              area_m2: 40,
              capacity: 6,
              region_code: "13",
              comuna_code: "Quilicura",
            },
          ],
        },
      };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");
  const summary = page.getByRole("region", { name: "Mis espacios" });
  await expect(summary.getByRole("status")).toHaveText(
    "Cargando tus espacios…",
  );
  releaseFirstResponse(apiError(503, "service_unavailable"));

  const alert = summary.getByRole("alert");
  await expect(alert).toContainText("No pudimos cargar tus espacios.");
  await alert.getByRole("button", { name: "Intentar de nuevo" }).click();
  await expect(
    summary.getByText("Aún no tienes espacios. Crea uno para comenzar."),
  ).toBeVisible();

  await page.reload();
  await expect(summary.getByText("1 borrador", { exact: true })).toBeVisible();
  await expect(summary.getByText("1 publicado", { exact: true })).toBeVisible();
  await expect(summary.getByText("1 archivado", { exact: true })).toBeVisible();
  expect(spacesRequestCount).toBe(3);
});

test("@phase6 @a11y identity capture form is accessible at desktop and mobile", async ({
  page,
}) => {
  const sessionExpiresAt = new Date(Date.now() + 3_600_000).toISOString();
  const identityCaseId = "990e8400-e29b-41d4-a716-446655440030";
  let created = false;
  const documentMutationPaths: string[] = [];
  const postRequestPaths: string[] = [];
  const identityCase = () => ({
    id: identityCaseId,
    purpose: "identity",
    state: "borrador",
    revision: 1,
    document_format_declared: "unsure",
    files: [],
  });

  await mockGateway(page, async (pathname, method) => {
    if (
      method === "POST" &&
      pathname.startsWith("/api/backend/document-verification/")
    ) {
      documentMutationPaths.push(pathname);
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendador"],
          capabilities: [],
          expires_at: sessionExpiresAt,
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
          expires_at: sessionExpiresAt,
        },
      };
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return { status: 200, body: created ? [identityCase()] : [] };
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      created = true;
      return {
        status: 201,
        body: identityCase(),
        headers: { etag: '"1"' },
      };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta/documentos");
  page.on("request", (request) => {
    if (request.method() === "POST") {
      postRequestPaths.push(new URL(request.url()).pathname);
    }
  });
  await page
    .getByLabel("Formato declarado de la cédula")
    .selectOption("unsure");
  await page
    .getByLabel(
      "Confirmo el aviso de prueba local para un expediente sintético",
    )
    .check();
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();
  await page.getByLabel("Evidencia a cargar").selectOption("identity_back");
  await page.getByLabel("JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-accessibility-back.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic accessibility fixture"),
  });
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeEnabled();

  for (const viewport of [
    { width: 1280, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const { violations } = await new AxeBuilder({ page }).analyze();
    expect(violations.map(({ id, help }) => `${id}: ${help}`)).toEqual([]);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
  }
  expect(documentMutationPaths).toEqual([
    "/api/backend/document-verification/cases",
  ]);
  expect(postRequestPaths).toEqual([
    "/api/backend/document-verification/cases",
  ]);
});
test("@phase6 @a11y account routes fit both viewports and have no Axe violations", async ({
  page,
}) => {
  await mockGateway(page, async (pathname, method) => {
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: "Usuario Ensayo",
          phone: "912345678",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET") {
      return apiError(404, "photo_not_found");
    }
    if (pathname === "/api/backend/payout-account" && method === "GET") {
      return apiError(404, "payout_not_found");
    }
    if (pathname === "/api/backend/rights-requests" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === "/api/backend/verifications/eligibility" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: {
          items: [
            { type: "kyc", eligible: true },
            { type: "kyb", eligible: false },
          ],
        },
      };
    }
    if (pathname === "/api/backend/verifications" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (pathname === "/api/backend/spaces" && method === "GET") {
      return {
        status: 200,
        body: {
          items: [
            {
              id: "space-summary-draft",
              category_code: "oficina",
              category_name: "Oficinas y Cowork",
              title: "Oficina en borrador",
              state: "borrador",
              base_price_clp: 20000,
              rate_unit: "hora",
              area_m2: 30,
              capacity: 4,
              region_code: "13",
              comuna_code: "Providencia",
            },
            {
              id: "space-summary-draft-2",
              category_code: "oficina",
              category_name: "Oficinas y Cowork",
              title: "Oficina en segundo borrador",
              state: "borrador",
              base_price_clp: 22000,
              rate_unit: "hora",
              area_m2: 32,
              capacity: 4,
              region_code: "13",
              comuna_code: "Ñuñoa",
            },
            {
              id: "space-summary-published",
              category_code: "oficina",
              category_name: "Oficinas y Cowork",
              title: "Oficina publicada",
              state: "activa",
              base_price_clp: 25000,
              rate_unit: "hora",
              area_m2: 35,
              capacity: 5,
              region_code: "13",
              comuna_code: "Las Condes",
            },
            {
              id: "space-summary-hidden",
              category_code: "bodega",
              category_name: "Bodegas y Mini-bodegas",
              title: "Bodega oculta",
              state: "oculta",
              base_price_clp: 30000,
              rate_unit: "dia",
              area_m2: 40,
              capacity: 6,
              region_code: "13",
              comuna_code: "Quilicura",
            },
          ],
        },
      };
    }
    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta");

  const summary = page.getByRole("region", { name: "Mis espacios" });
  await expect(summary).toBeVisible();
  await expect(
    summary.getByText("2 borradores", { exact: true }),
  ).toBeVisible();
  await expect(summary.getByText("1 publicado", { exact: true })).toBeVisible();
  await expect(summary.getByText("1 archivado", { exact: true })).toBeVisible();
  const createSpaceLink = summary.getByRole("link", { name: "Crear espacio" });
  await expect(createSpaceLink).toHaveAttribute(
    "href",
    "/anfitrion/espacios/nuevo",
  );
  await expect(
    summary.getByRole("link", { name: "Ver mis espacios" }),
  ).toHaveAttribute("href", "/anfitrion/espacios");

  const espaciosLink = page
    .getByRole("navigation", { name: "Secciones de mi cuenta" })
    .getByRole("link", { name: "Mis espacios" });
  await expect(espaciosLink).toBeVisible();
  await expect(espaciosLink).toHaveAttribute("href", "/anfitrion/espacios");

  const routes = [
    "/mi-cuenta",
    "/mi-cuenta/privacidad",
    "/mi-cuenta/seguridad",
    "/mi-cuenta/verificaciones",
  ];

  for (const route of routes) {
    if (page.url().endsWith(route)) {
      // Already there
    } else {
      const linkName =
        route === "/mi-cuenta"
          ? "General"
          : route === "/mi-cuenta/privacidad"
            ? "Privacidad"
            : route === "/mi-cuenta/seguridad"
              ? "Seguridad"
              : "Verificaciones";
      await page
        .getByRole("navigation", { name: "Secciones de mi cuenta" })
        .getByRole("link", { name: linkName })
        .click();
      await page.waitForURL((url) => url.pathname === route);
    }

    if (route === "/mi-cuenta/verificaciones") {
      await expect(
        page.getByRole("heading", {
          level: 2,
          name: "Verificación de cuenta y espacios",
          exact: true,
        }),
      ).toBeVisible();
    }
    await expect(page.locator("h1")).toHaveCount(1);
    for (const viewport of [
      { width: 1365, height: 900 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      const dimensions = await page.evaluate(() => ({
        width: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.width);
      const results = await new AxeBuilder({ page }).analyze();
      expect(
        results.violations.map((violation) => violation.id),
        JSON.stringify(
          results.violations.map((violation) => ({
            id: violation.id,
            nodes: violation.nodes.map((node) => ({
              target: node.target,
              failureSummary: node.failureSummary,
            })),
          })),
        ),
      ).toEqual([]);
    }
  }
});

test("@phase6 @controlled /mi-cuenta/verificaciones: guarda declaración y muestra que aún no hay expediente propio sin iniciar proveedor externo", async ({
  page,
}) => {
  let savedDeclaration: {
    id: string;
    revision: number;
    first_name: string;
    last_name: string;
    date_of_birth: string;
    rut: string;
    id_country: "CHL";
    document_type: "ID";
    consent_version: string;
    consent_given_at: string;
  } | null = null;
  let providerApiRequests = 0;
  let caseListReads = 0;

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname.includes("/verifications/didit/")) {
      providerApiRequests += 1;
      return apiError(410, "identity_provider_retired");
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        },
      };
    }
    if (
      pathname === "/api/backend/verifications/eligibility" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: { items: [{ type: "kyc", eligible: true }] },
      };
    }
    if (pathname === "/api/backend/verifications" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      if (!savedDeclaration) {
        return apiError(404, "not_found");
      }
      return { status: 200, body: savedDeclaration };
    }
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      caseListReads += 1;
      return { status: 200, body: [] };
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "POST"
    ) {
      const payload = JSON.parse(request.postData() ?? "{}");
      expect(payload.id_country).toBe("CHL");
      expect(payload.document_type).toBe("ID");
      expect(payload.consent_given).toBe(true);
      expect(payload.consent_version).toBe("id-consent-20261108");
      savedDeclaration = {
        id: "550e8400-e29b-41d4-a716-446655440077",
        revision: (savedDeclaration?.revision ?? 0) + 1,
        first_name: payload.first_name,
        last_name: payload.last_name,
        date_of_birth: payload.date_of_birth,
        rut: payload.rut,
        id_country: "CHL",
        document_type: "ID",
        consent_version: payload.consent_version,
        consent_given_at: "2026-11-08T18:00:00Z",
      };
      return { status: 201, body: savedDeclaration };
    }

    return apiError(404, "not_found");
  });

  await loginAndNavigate(page, "/mi-cuenta/verificaciones");

  // Paso 1: formulario de declaración sin llamada a Didit
  await expect(
    page.getByRole("button", { name: "Guardar datos de identidad" }),
  ).toBeVisible();
  await page.getByLabel("Nombres legales completos").fill("Camila Andrea");
  await page.getByLabel("Apellidos legales completos").fill("Soto Muñoz");
  await page.getByLabel("Fecha de nacimiento").fill("1994-06-15");
  await page.getByLabel("RUN / RUT chileno").fill("123456785");
  await page.locator('input[name="consent"]').check();
  await page
    .getByRole("button", { name: "Guardar datos de identidad" })
    .click();

  expect(providerApiRequests).toBe(0);
  await expect(page.getByText("Datos guardados (revisión #1)")).toBeVisible();
  await expect(page.getByText("Camila Andrea")).toBeVisible();
  await expect(page.getByText("12.345.678-5")).toBeVisible();

  // Editar la declaración sin iniciar ningún flujo de verificación externo.
  await page.getByRole("button", { name: "Editar datos" }).click();
  await page.getByLabel("Nombres legales completos").fill("Camila Paz");
  await page.locator('input[name="consent"]').check();
  await page
    .getByRole("button", { name: "Guardar datos de identidad" })
    .click();
  await expect(page.getByText("Datos guardados (revisión #2)")).toBeVisible();
  await expect(page.getByText("Camila Paz")).toBeVisible();
  expect(providerApiRequests).toBe(0);

  await expect(
    page.getByRole("heading", { name: "Verificación de la cuenta" }),
  ).toBeVisible();
  await expect(
    page.getByText("Aún no has iniciado una revisión documental propia."),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Iniciar verificación documental" }),
  ).toHaveAttribute("href", "/mi-cuenta/documentos");
  expect(caseListReads).toBeGreaterThan(0);
  await expect(page.getByText("Verificar identidad con Didit")).toHaveCount(0);
  expect(providerApiRequests).toBe(0);
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones/);
});

test("@phase6 @controlled /mi-cuenta/verificaciones: aislamiento A->logout->B->logout->A y retorno histórico neutral sin consulta externa", async ({
  page,
}) => {
  const accountA = accountId;
  const accountB = "880e8400-e29b-41d4-a716-446655440099";
  let activeAccount: string | null = accountA;
  let providerApiRequests = 0;
  const legacyAttemptId = "7c9e6679-7425-40de-944b-e07fc1f90ae8";
  let rejectedClaims = 0;
  const sessionExpiresAt = new Date(Date.now() + 3600_000).toISOString();

  const declarationsByAccount = new Map<
    string,
    {
      id: string;
      revision: number;
      first_name: string;
      last_name: string;
      date_of_birth: string;
      rut: string;
      id_country: "CHL";
      document_type: "ID";
      consent_version: string;
      consent_given_at: string;
    }
  >();

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname.includes("/verifications/didit/")) {
      providerApiRequests += 1;
      return apiError(410, "identity_provider_retired");
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      const body = JSON.parse(request.postData() ?? "{}") as { email?: string };
      activeAccount =
        body.email === "user-b@example.invalid" ? accountB : accountA;
      return {
        status: 200,
        body: {
          account_id: activeAccount,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          expires_at: sessionExpiresAt,
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/logout" && method === "POST") {
      activeAccount = null;
      return { status: 204 };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      if (!activeAccount) return apiError(401, "unauthorized");
      return {
        status: 200,
        body: {
          account_id: activeAccount,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          restricted_mode: false,
          expires_at: sessionExpiresAt,
        },
      };
    }
    if (pathname === "/api/backend/profile" && method === "GET") {
      return {
        status: 200,
        body: {
          display_name: activeAccount === accountB ? "Usuario B" : "Usuario A",
          phone: "987654321",
          updated_at: new Date().toISOString(),
        },
      };
    }
    if (pathname === "/api/backend/profile/photo" && method === "GET") {
      return apiError(404, "photo_not_found");
    }
    if (
      pathname === "/api/backend/verifications/eligibility" &&
      method === "GET"
    ) {
      return {
        status: 200,
        body: { kyc_eligible: false, kyb_eligible: false, verified: false },
      };
    }
    if (pathname === "/api/backend/verifications" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      if (!activeAccount) return apiError(401, "unauthorized");
      const found = declarationsByAccount.get(activeAccount);
      if (!found) return apiError(404, "not_found");
      return { status: 200, body: found };
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "POST"
    ) {
      if (!activeAccount) return apiError(401, "unauthorized");
      const payload = JSON.parse(request.postData() ?? "{}");
      if (activeAccount === accountB && payload.rut === "12345678-5") {
        rejectedClaims += 1;
        return apiError(409, "rut_already_claimed");
      }
      const saved = {
        id: "550e8400-e29b-41d4-a716-446655440088",
        revision: 1,
        first_name: payload.first_name,
        last_name: payload.last_name,
        date_of_birth: payload.date_of_birth,
        rut: payload.rut,
        id_country: "CHL" as const,
        document_type: "ID" as const,
        consent_version: payload.consent_version,
        consent_given_at: "2026-11-08T18:10:00Z",
      };
      declarationsByAccount.set(activeAccount, saved);
      return { status: 201, body: saved };
    }

    return apiError(404, "not_found");
  });

  // 1. Cuenta A guarda su declaración
  await loginAndNavigate(page, "/mi-cuenta/verificaciones");
  await page
    .getByLabel("Nombres legales", { exact: true })
    .fill("Matías Ignacio");
  await page
    .getByLabel("Apellidos legales", { exact: true })
    .fill("Rojas Díaz");
  await page.getByLabel("Fecha de nacimiento").fill("1991-04-20");
  await page.getByLabel("RUN/RUT", { exact: true }).fill("12345678-5");
  await page
    .getByRole("checkbox", { name: /Confirmo que usaré solo datos sintéticos/ })
    .check();
  await page
    .getByRole("button", { name: "Guardar declaración", exact: true })
    .click();
  await expect(
    page.getByText("Declaración guardada.", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Matías Ignacio")).toBeVisible();

  // Verificar que no se persiste RUT ni fecha en localStorage/sessionStorage
  const storageDump = await page.evaluate(() => ({
    local: JSON.stringify(window.localStorage),
    session: JSON.stringify(window.sessionStorage),
  }));
  expect(storageDump.local).not.toContain("12345678");
  expect(storageDump.session).not.toContain("12345678");
  expect(storageDump.local).not.toContain("1991-04-20");
  expect(storageDump.session).not.toContain("1991-04-20");

  // 2. Logout de Cuenta A -> Login de Cuenta B
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  await page.getByLabel("Correo electrónico").fill("user-b@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("SecurePass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === "/mi-cuenta/verificaciones");

  // Cuenta B no ve los datos de Cuenta A; formulario vacío
  await expect(page.getByText("Matías Ignacio")).toHaveCount(0);
  await expect(page.getByLabel("Nombres legales", { exact: true })).toHaveValue(
    "",
  );
  await expect(page.getByLabel("RUN/RUT", { exact: true })).toHaveValue("");

  // Cuenta B intenta registrar el mismo RUT de Cuenta A -> 409 sin redirección ni llamada a Didit
  await page
    .getByLabel("Nombres legales", { exact: true })
    .fill("Borrador Cuenta B");
  await page.getByLabel("Apellidos legales", { exact: true }).fill("Pérez");
  await page.getByLabel("Fecha de nacimiento").fill("1992-08-10");
  await page.getByLabel("RUN/RUT", { exact: true }).fill("12345678-5");
  await page
    .getByRole("checkbox", { name: /Confirmo que usaré solo datos sintéticos/ })
    .check();
  await page
    .getByRole("button", { name: "Guardar declaración", exact: true })
    .click();
  await expect(
    page.getByText(
      "No pudimos confirmar el guardado. Consulta el estado antes de volver a intentarlo.",
    ),
  ).toBeVisible();
  expect(rejectedClaims).toBe(1);
  expect(providerApiRequests).toBe(0);
  expect(new URL(page.url()).pathname).toBe("/mi-cuenta/verificaciones");

  // 3. Logout de Cuenta B -> Re-login de Cuenta A recupera su declaración desde Go sin rastro de B
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await loginAndNavigate(page, "/mi-cuenta/verificaciones");
  await expect(
    page.getByText("Declaración guardada.", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Matías Ignacio")).toBeVisible();
  await expect(page.getByText("Borrador Cuenta B")).toHaveCount(0);

  // 4. El retorno histórico limpia parámetros y no consulta el proveedor.
  await page.goto(
    `/verificaciones/retorno?attempt_id=${legacyAttemptId}&token=leak`,
  );
  await expect(
    page.getByRole("heading", { name: "Este retorno ya no está activo" }),
  ).toBeVisible();
  await expect(page.getByRole("status")).toContainText(
    "La revisión documental propia aún no está habilitada",
  );
  await expect(page).toHaveURL(/\/verificaciones\/retorno$/);
  expect(providerApiRequests).toBe(0);
});

test("@phase6 @controlled /mi-cuenta/verificaciones: status card is isolated across logout/login A->B->A", async ({
  page,
}) => {
  const accountA = accountId;
  const accountB = "880e8400-e29b-41d4-a716-446655440099";
  let activeAccount: string | null = accountA;
  let providerRouteRequests = 0;
  const caseListReads = new Map<string, number>();
  const sessionExpiresAt = new Date(Date.now() + 3600_000).toISOString();
  const declarationFor = (firstName: string, id: string) => ({
    id,
    revision: 1,
    first_name: firstName,
    last_name: "Synthetic",
    date_of_birth: "1990-01-01",
    rut: "12345678-5",
    id_country: "CHL" as const,
    document_type: "ID" as const,
    consent_version: identityDeclarationConsentVersion,
    consent_given_at: "2026-11-11T12:00:00Z",
  });
  const declarations = new Map([
    [accountA, declarationFor("Ada A", "550e8400-e29b-41d4-a716-446655440091")],
    [
      accountB,
      declarationFor("Beto B", "550e8400-e29b-41d4-a716-446655440092"),
    ],
  ]);
  const states = new Map([
    [accountA, "en_revision"],
    [accountB, "requiere_antecedentes"],
  ]);

  await mockGateway(page, async (pathname, method, request) => {
    if (pathname.includes("/verifications/didit/")) {
      providerRouteRequests += 1;
      return apiError(410, "identity_provider_retired");
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      const body = JSON.parse(request.postData() ?? "{}") as { email?: string };
      activeAccount =
        body.email === "user-b@example.invalid" ? accountB : accountA;
      return {
        status: 200,
        body: {
          account_id: activeAccount,
          roles: ["arrendatario"],
          capabilities: [],
          expires_at: sessionExpiresAt,
          restricted_mode: false,
        },
      };
    }
    if (pathname === "/api/backend/auth/logout" && method === "POST") {
      activeAccount = null;
      return { status: 204 };
    }
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      if (!activeAccount) return apiError(401, "unauthorized");
      return {
        status: 200,
        body: {
          account_id: activeAccount,
          roles: ["arrendatario"],
          capabilities: [],
          restricted_mode: false,
          expires_at: sessionExpiresAt,
        },
      };
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      return activeAccount
        ? { status: 200, body: declarations.get(activeAccount) }
        : apiError(401, "unauthorized");
    }
    if (
      pathname === "/api/backend/verifications/eligibility" &&
      method === "GET"
    )
      return { status: 200, body: { items: [] } };
    if (pathname === "/api/backend/verifications" && method === "GET")
      return { status: 200, body: { items: [] } };
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      if (!activeAccount) return apiError(401, "unauthorized");
      caseListReads.set(
        activeAccount,
        (caseListReads.get(activeAccount) ?? 0) + 1,
      );
      return {
        status: 200,
        body: [
          {
            id: `case-${activeAccount}`,
            purpose: "identity",
            state: states.get(activeAccount),
            revision: 1,
            owner_decision_message:
              activeAccount === accountB
                ? "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente."
                : undefined,
            files: [],
          },
        ],
      };
    }
    return apiError(404, "not_found");
  });

  const assertCard = async (
    firstName: string,
    message: string,
    cta: string,
    absent: string,
  ) => {
    await expect(page.getByText(firstName, { exact: true })).toBeVisible();
    await expect(page.getByText(absent, { exact: true })).toHaveCount(0);
    // Main presents the documentary status in General, separate from the declaration.
    await page.goto("/mi-cuenta");
    await expect(
      page.getByRole("status").filter({ hasText: message }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: cta })).toHaveAttribute(
      "href",
      "/mi-cuenta/documentos",
    );
    await expect(page.getByText(absent, { exact: true })).toHaveCount(0);
  };

  await loginAndNavigate(page, "/mi-cuenta/verificaciones");
  await assertCard(
    "Ada A",
    "Tu solicitud está en revisión documental.",
    "Ver estado de solicitud",
    "Beto B",
  );
  expect(caseListReads.get(accountA)).toBe(1);

  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  await page.getByLabel("Correo electrónico").fill("user-b@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("SecurePass123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === "/mi-cuenta/verificaciones");
  await assertCard(
    "Beto B",
    "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente.",
    "Corregir documentos",
    "Tu solicitud está en revisión documental.",
  );
  expect(caseListReads.get(accountB)).toBe(1);

  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await loginAndNavigate(page, "/mi-cuenta/verificaciones");
  await assertCard(
    "Ada A",
    "Tu solicitud está en revisión documental.",
    "Ver estado de solicitud",
    "Necesitamos que corrijas o completes tus documentos.",
  );
  expect(caseListReads.get(accountA)).toBe(2);
  expect(caseListReads.get(accountB)).toBe(1);
  expect(providerRouteRequests).toBe(0);
});
