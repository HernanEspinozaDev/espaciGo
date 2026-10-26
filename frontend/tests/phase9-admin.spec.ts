import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createBffFixtureState } from "./helpers/bff-fixture";
import type {
  AdminCapability,
  AdminDamageClaim,
  AdminLocalDispute,
  AdminLocalNotice,
  AdminReviewReport,
  AdminRole,
  AdminVerificationCase,
  AuditEvent,
  GlobalAdminAccount,
  GlobalAdminAccountState,
  GlobalAdminKPIs,
  GlobalAdminMe,
  GlobalAdminOverview,
  GlobalAdminPublication,
  GlobalAdminPublicationState,
} from "@/features/admin/domain/types";

test.use({ trace: "off", screenshot: "off", video: "off" });

interface MockResult {
  status?: number;
  body?: unknown;
  contentType?: string;
}

type MockHandler = (
  route: Route,
  request: Request,
) => Promise<MockResult | null | undefined> | MockResult | null | undefined;

const canonicalAdminId = "990e8400-e29b-41d4-a716-446655440000";
const canonicalUserAccountId = "110e8400-e29b-41d4-a716-446655440000";
const canonicalPublicationId = "pub-test-01";
const canonicalVerificationId = "verif-test-01";
const canonicalDisputeId = "disp-test-01";

const ADMIN_CAPABILITIES: AdminCapability[] = [
  "accounts.list",
  "accounts.read",
  "accounts.update",
  "accounts.block",
  "accounts.roles",
  "publications.list",
  "publications.read",
  "publications.update",
  "publications.moderate",
  "publications.retire",
  "admin_access.manage",
  "kpi.read",
  "audit.read",
  "audit.export",
  "legacy.administration",
];

interface TestAdminStore {
  me: GlobalAdminMe;
  overview: GlobalAdminOverview;
  kpis: GlobalAdminKPIs;
  accounts: GlobalAdminAccount[];
  publications: GlobalAdminPublication[];
  verifications: AdminVerificationCase[];
  disputes: AdminLocalDispute[];
  auditEvents: AuditEvent[];
  damageClaims: AdminDamageClaim[];
  reviewReports: AdminReviewReport[];
  notices: AdminLocalNotice[];
  simulate428OnPath: string | null;
  simulate403OnPath: string | null;
  simulate409OnSpaceModeration: boolean;
  handler: MockHandler;
}

function createAdminStore(): TestAdminStore {
  const me: GlobalAdminMe = {
    account_id: canonicalAdminId,
    roles: ["administrador_global"],
    capabilities: ADMIN_CAPABILITIES,
    generated_at: "2026-10-25T12:00:00Z",
  };

  const overview: GlobalAdminOverview = {
    total_accounts: 42,
    administrative_blocked: 3,
    by_state: { activo: 35, correo_pendiente: 4, bloqueado: 3 },
    by_role: { arrendatario: 30, arrendador: 10, administrador: 2 },
    generated_at: "2026-10-25T12:00:00Z",
    definition: "Resumen global de métricas del sistema",
  };

  const kpis: GlobalAdminKPIs = {
    period: {
      from: "2026-09-25T00:00:00Z",
      until: "2026-10-25T00:00:00Z",
      time_zone: "America/Santiago",
    },
    generated_at: "2026-10-25T12:00:00Z",
    definition: "KPIs operativos consolidados",
    metrics: {
      total_revenue_clp: 15000000,
      active_reservations: 18,
      occupancy_rate: 0.74,
      dispute_rate: 0.02,
    },
  };

  const accounts: GlobalAdminAccount[] = [
    {
      account_id: canonicalUserAccountId,
      email: "usuario.prueba@espacigo.cl",
      display_name: "Usuario Prueba",
      phone: "+56912345678",
      profile_updated_at: "2026-10-25T10:00:00Z",
      state: "activo",
      created_at: "2026-10-01T10:00:00Z",
      administrative_blocked: false,
      roles: ["arrendatario"],
      publication_count: 0,
      reservation_count: 2,
      incident_count: 0,
    },
    {
      account_id: "220e8400-e29b-41d4-a716-446655440000",
      email: "anfitrion.bloqueado@espacigo.cl",
      display_name: "Anfitrión Bloqueado",
      phone: "+56987654321",
      profile_updated_at: "2026-10-24T10:00:00Z",
      state: "bloqueado",
      created_at: "2026-10-02T10:00:00Z",
      administrative_blocked: true,
      roles: ["arrendador"],
      publication_count: 1,
      reservation_count: 0,
      incident_count: 1,
    },
  ];

  const publications: GlobalAdminPublication[] = [
    {
      publication_id: canonicalPublicationId,
      owner_id: canonicalUserAccountId,
      owner_name: "Usuario Prueba",
      owner_email: "usuario.prueba@espacigo.cl",
      category_code: "bodega",
      category_name: "Bodega Segura",
      title: "Bodega Centro Santiago 25m2",
      description: "Espacio amplio y seguro para almacenamiento.",
      area_m2: "25",
      capacity: 5,
      usage_rules: "Sin sustancias peligrosas.",
      address: "Av. Libertador Bernardo O'Higgins 1234, Santiago",
      state: "activa",
      rate_unit: "mes",
      base_price_clp: "120000",
      updated_at: "2026-10-25T10:00:00Z",
      publicly_visible: true,
      public_cover_available: false,
      attribute_schema_version: 1,
      attributes: {},
      photos: [],
    },
  ];

  const verifications: AdminVerificationCase[] = [
    {
      id: canonicalVerificationId,
      type: "kyc",
      applicant_id: canonicalUserAccountId,
      state: "en_revision",
      created_at: "2026-10-25T09:00:00Z",
      updated_at: "2026-10-25T09:00:00Z",
      notes: "Documentación enviada para revisión",
    },
  ];

  const disputes: AdminLocalDispute[] = [
    {
      dispute_id: canonicalDisputeId,
      reservation_id: "res-test-99",
      renter_id: canonicalUserAccountId,
      host_id: "220e8400-e29b-41d4-a716-446655440000",
      state: "abierta",
      reason: "Discrepancia en devolución de depósito de garantía.",
      created_at: "2026-10-24T15:00:00Z",
    },
  ];

  const auditEvents: AuditEvent[] = [
    {
      event_id: "audit-evt-01",
      actor_id: canonicalAdminId,
      action: "publication.state_update",
      resource_type: "publication",
      resource_id: canonicalPublicationId,
      timestamp: "2026-10-25T12:00:00Z",
      result: "exito",
      details: { reason: "revision_administrativa" },
    },
  ];

  const damageClaims: AdminDamageClaim[] = [];
  const reviewReports: AdminReviewReport[] = [];
  const notices: AdminLocalNotice[] = [];

  const store: TestAdminStore = {
    me,
    overview,
    kpis,
    accounts,
    publications,
    verifications,
    disputes,
    auditEvents,
    damageClaims,
    reviewReports,
    notices,
    simulate428OnPath: null,
    simulate403OnPath: null,
    simulate409OnSpaceModeration: false,
    handler: (route, request) => {
      const url = new URL(request.url());
      const pathname = url.pathname;
      const method = request.method();

      // Security simulation gates
      if (
        store.simulate428OnPath &&
        pathname.includes(store.simulate428OnPath)
      ) {
        return {
          status: 428,
          body: {
            code: "precondition_required",
            message:
              "Se requiere autenticación reciente para realizar esta operación administrativa sensible.",
          },
        };
      }

      if (
        store.simulate403OnPath &&
        pathname.includes(store.simulate403OnPath)
      ) {
        return {
          status: 403,
          body: {
            code: "forbidden",
            message: "Acceso denegado o sesión revocada.",
          },
        };
      }

      // Auth Session Profile & Login
      if (pathname === "/api/backend/auth/login" && method === "POST") {
        return {
          status: 200,
          body: {
            account_id: canonicalAdminId,
            roles: ["administrador_global"],
            capabilities: store.me.capabilities,
            expires_at: new Date(Date.now() + 3600_000).toISOString(),
            restricted_mode: false,
          },
        };
      }

      if (pathname === "/api/backend/auth/session" && method === "GET") {
        return {
          status: 200,
          body: {
            account_id: canonicalAdminId,
            roles: ["administrador_global"],
            capabilities: store.me.capabilities,
            restricted_mode: false,
          },
        };
      }

      // Global Admin Me & Overview & KPIs
      if (pathname === "/api/backend/admin/global/me" && method === "GET") {
        return { status: 200, body: { data: store.me } };
      }

      if (
        pathname === "/api/backend/admin/global/overview" &&
        method === "GET"
      ) {
        return { status: 200, body: { data: store.overview } };
      }

      if (pathname === "/api/backend/admin/global/kpis" && method === "GET") {
        return { status: 200, body: store.kpis };
      }

      // Audit events export & list
      if (
        pathname === "/api/backend/admin/global/audit-events/export" &&
        method === "GET"
      ) {
        return {
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            exported_events: store.auditEvents.length,
            items: store.auditEvents,
            generated_at: new Date().toISOString(),
          }),
        };
      }

      if (
        pathname === "/api/backend/admin/global/audit-events" &&
        method === "GET"
      ) {
        return {
          status: 200,
          body: { data: { items: store.auditEvents } },
        };
      }

      // Global Accounts list & mutations
      if (
        pathname === "/api/backend/admin/global/accounts" &&
        method === "GET"
      ) {
        return { status: 200, body: { data: { items: store.accounts } } };
      }

      const rolesMatch = pathname.match(
        /\/api\/backend\/admin\/global\/accounts\/([^/]+)\/roles\/([^/]+)/,
      );
      if (rolesMatch && method === "PUT") {
        const [, accountId, role] = rolesMatch;
        const json = JSON.parse(request.postData() ?? "{}");
        const targetIdx = store.accounts.findIndex(
          (a) => a.account_id === accountId,
        );
        if (targetIdx >= 0) {
          const target = store.accounts[targetIdx];
          const newRoles = json.granted
            ? [...new Set([...target.roles, role as AdminRole])]
            : target.roles.filter((r) => r !== role);
          const updated: GlobalAdminAccount = {
            ...target,
            roles: newRoles,
          };
          store.accounts[targetIdx] = updated;
          return { status: 200, body: { data: updated } };
        }
        return { status: 404, body: { error: "account_not_found" } };
      }

      const blockMatch = pathname.match(
        /\/api\/backend\/admin\/global\/accounts\/([^/]+)\/block/,
      );
      if (blockMatch && method === "POST") {
        const [, accountId] = blockMatch;
        const json = JSON.parse(request.postData() ?? "{}");
        const targetIdx = store.accounts.findIndex(
          (a) => a.account_id === accountId,
        );
        if (targetIdx >= 0) {
          const target = store.accounts[targetIdx];
          const updated: GlobalAdminAccount = {
            ...target,
            administrative_blocked: !!json.blocked,
            state: (json.blocked
              ? "bloqueado"
              : "activo") as GlobalAdminAccountState,
          };
          store.accounts[targetIdx] = updated;
          return { status: 200, body: { data: updated } };
        }
        return { status: 404, body: { error: "account_not_found" } };
      }

      const revokeMatch = pathname.match(
        /\/api\/backend\/admin\/global\/accounts\/([^/]+)\/sessions\/revoke/,
      );
      if (revokeMatch && method === "POST") {
        return { status: 200, body: { data: { revoked: true } } };
      }

      // Publications list & state change
      if (
        pathname === "/api/backend/admin/global/publications" &&
        method === "GET"
      ) {
        return { status: 200, body: { data: { items: store.publications } } };
      }

      const pubStateMatch = pathname.match(
        /\/api\/backend\/admin\/global\/publications\/([^/]+)\/state/,
      );
      if (pubStateMatch && method === "POST") {
        if (store.simulate409OnSpaceModeration) {
          return {
            status: 409,
            body: {
              code: "conflict",
              message:
                "Conflicto de concurrencia: el recurso fue modificado por otra sesión administrativa.",
            },
          };
        }
        const [, pubId] = pubStateMatch;
        const json = JSON.parse(request.postData() ?? "{}");
        const targetIdx = store.publications.findIndex(
          (p) => p.publication_id === pubId,
        );
        if (targetIdx >= 0) {
          const target = store.publications[targetIdx];
          const updated: GlobalAdminPublication = {
            ...target,
            state: json.state as GlobalAdminPublicationState,
            updated_at: new Date().toISOString(),
          };
          store.publications[targetIdx] = updated;
          return { status: 200, body: { data: updated } };
        }
        return { status: 404, body: { error: "publication_not_found" } };
      }

      // Verifications list & review
      if (pathname === "/api/backend/admin/verifications" && method === "GET") {
        return { status: 200, body: { items: store.verifications } };
      }

      if (
        pathname === "/api/backend/admin/verifications/failed" &&
        method === "GET"
      ) {
        return { status: 200, body: { items: [] } };
      }

      const reviewMatch = pathname.match(
        /\/api\/backend\/admin\/verifications\/([^/]+)\/review/,
      );
      if (reviewMatch && method === "POST") {
        const [, verifId] = reviewMatch;
        const json = JSON.parse(request.postData() ?? "{}");
        const targetIdx = store.verifications.findIndex(
          (v) => v.id === verifId,
        );
        if (targetIdx >= 0) {
          const target = store.verifications[targetIdx];
          const updated: AdminVerificationCase = {
            ...target,
            state: json.decision === "aprobar" ? "aprobada" : "rechazada",
            notes: json.reason ?? target.notes,
            updated_at: new Date().toISOString(),
          };
          store.verifications[targetIdx] = updated;
          return { status: 200, body: updated };
        }
        return { status: 404, body: { error: "verification_not_found" } };
      }

      // Disputes list & close
      if (pathname === "/api/backend/admin/disputes" && method === "GET") {
        return { status: 200, body: { items: store.disputes } };
      }

      const closeMatch = pathname.match(
        /\/api\/backend\/admin\/disputes\/([^/]+)\/close/,
      );
      if (closeMatch && method === "POST") {
        const [, disputeId] = closeMatch;
        const json = JSON.parse(request.postData() ?? "{}");
        const targetIdx = store.disputes.findIndex(
          (d) => d.dispute_id === disputeId,
        );
        if (targetIdx >= 0) {
          const target = store.disputes[targetIdx];
          const updated: AdminLocalDispute = {
            ...target,
            state: "cerrada",
            close_reason: json.reason_code,
            closed_at: new Date().toISOString(),
          };
          store.disputes[targetIdx] = updated;
          return { status: 200, body: updated };
        }
        return { status: 404, body: { error: "dispute_not_found" } };
      }

      // Fallbacks for secondary endpoints
      if (
        pathname === "/api/backend/admin/local/damage-claims" &&
        method === "GET"
      ) {
        return { status: 200, body: { data: { items: store.damageClaims } } };
      }

      if (
        pathname === "/api/backend/admin/local/review-reports" &&
        method === "GET"
      ) {
        return { status: 200, body: { data: { items: store.reviewReports } } };
      }

      if (pathname === "/api/backend/admin/local/notices" && method === "GET") {
        return { status: 200, body: { data: { items: store.notices } } };
      }

      if (
        (pathname === "/api/backend/admin/local/published-spaces" ||
          pathname === "/api/backend/admin/spaces") &&
        method === "GET"
      ) {
        return { status: 200, body: { data: { items: [] } } };
      }

      return null;
    },
  };

  return store;
}

async function installMockGateway(page: Page, handler: MockHandler) {
  const bff = createBffFixtureState();
  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const result = bff.adapt(request, await handler(route, request));
    if (result) {
      await route.fulfill({
        status: result.status ?? 200,
        contentType: result.contentType ?? "application/json",
        headers: result.headers,
        body:
          typeof result.body === "string"
            ? result.body
            : JSON.stringify(result.body),
      });
      return;
    }
    await route.continue();
  });
}

async function loginAndNavigate(page: Page, destination = "/administracion") {
  const [pathname] = destination.split("?");
  await page.goto(`/acceso?return_to=${encodeURIComponent(pathname)}`);
  await page
    .getByLabel("Correo electrónico")
    .fill("administrador.global@example.invalid");
  await page.getByLabel("Contraseña", { exact: true }).fill("SecureAdmin123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === pathname);
}

test.describe("Fase 9 - Administración, Moderación y Gobernanza", () => {
  test("@phase9 flujo de capacidades, navegación, KPIs y exportación segura de bitácora Blob con revocación", async ({
    page,
  }) => {
    const store = createAdminStore();
    await installMockGateway(page, store.handler);

    // Monitor Blob ObjectURL creation and revocation
    await page.addInitScript(() => {
      const originalCreate = URL.createObjectURL;
      const originalRevoke = URL.revokeObjectURL;
      (window as unknown as Record<string, unknown>).__blobUrlsCreated = [];
      (window as unknown as Record<string, unknown>).__blobUrlsRevoked = [];
      URL.createObjectURL = function (blob: Blob) {
        const url = originalCreate.call(URL, blob);
        (
          (window as unknown as Record<string, unknown>)
            .__blobUrlsCreated as string[]
        ).push(url);
        return url;
      };
      URL.revokeObjectURL = function (url: string) {
        (
          (window as unknown as Record<string, unknown>)
            .__blobUrlsRevoked as string[]
        ).push(url);
        return originalRevoke.call(URL, url);
      };
    });

    // 1. Acceso autenticado al panel de administración
    await loginAndNavigate(page, "/administracion");
    await expect(page.locator("h1")).toHaveText("Panel de Administración");

    // 2. Un solo rol visible por sesión en Badge de navegación
    await expect(page.getByText("Administrador Global")).toBeVisible();

    // 3. Renderizado de navegación por capacidades
    await expect(
      page.getByRole("link", { name: "Resumen / KPIs" }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Verificaciones KYC" }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: "Espacios" })).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Disputas y Reclamos" }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Cuentas y Roles" }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Auditoría y Reportes" }),
    ).toBeVisible();

    // 4. Renderizado de KPIs y métricas consolidadas
    await expect(page.getByText("Total Cuentas")).toBeVisible();
    await expect(page.getByText("42", { exact: true })).toBeVisible();
    await expect(
      page.getByText(/3 bloqueadas administrativamente/i),
    ).toBeVisible();
    await expect(page.getByText("Publicaciones Activas")).toBeVisible();
    await expect(page.getByText("Verificaciones Pendientes")).toBeVisible();
    await expect(page.getByText("Disputas Abiertas")).toBeVisible();

    // 5. Exportación segura de bitácora Blob con revocación de URL
    const exportBtn = page.getByRole("button", {
      name: /Exportar Auditoría/i,
    });
    await expect(exportBtn).toBeVisible();
    await exportBtn.click();

    // Verify Blob creation and scheduled revocation
    await page.waitForTimeout(300);
    const blobTelemetry = await page.evaluate(() => ({
      created: (window as unknown as Record<string, unknown>)
        .__blobUrlsCreated as string[],
      revoked: (window as unknown as Record<string, unknown>)
        .__blobUrlsRevoked as string[],
    }));
    expect(blobTelemetry.created.length).toBeGreaterThan(0);
    expect(blobTelemetry.revoked.length).toBeGreaterThan(0);
    expect(blobTelemetry.revoked).toContain(blobTelemetry.created[0]);
  });

  test("@phase9 flujo moderación KYC (motivo obligatorio 5+ chars) y moderación de espacios con concurrencia optimista 409", async ({
    page,
  }) => {
    const store = createAdminStore();
    await installMockGateway(page, store.handler);

    // --- PARTE 1: Moderación KYC en /administracion/verificaciones ---
    await loginAndNavigate(page, "/administracion/verificaciones");
    await expect(
      page.getByRole("heading", {
        name: /Moderación de Verificaciones KYC/i,
      }),
    ).toBeVisible();
    await expect(page.getByText("En revisión", { exact: true })).toBeVisible();

    // 1.1 Rechazo con motivo obligatorio: validación < 5 caracteres
    await page.getByRole("button", { name: "Rechazar" }).click();
    await expect(page.getByText("Rechazar Verificación")).toBeVisible();

    const reasonInput = page.locator("#review-reason");
    await reasonInput.fill("abc");
    await page.getByRole("button", { name: "Confirmar Rechazo" }).click();

    // Error obligatorio visible
    await expect(
      page.getByText(
        /El motivo de rechazo es obligatorio y debe tener entre 5 y 500 caracteres/i,
      ),
    ).toBeVisible();

    // 1.2 Rechazo con motivo válido (5+ caracteres)
    await reasonInput.fill(
      "Documento ilegible con reflejos que impiden el cotejo.",
    );
    await page.getByRole("button", { name: "Confirmar Rechazo" }).click();

    // Caso pasa a estado rechazada
    await page.getByRole("button", { name: /Todos los Casos/i }).click();
    await expect(page.getByText("Rechazada")).toBeVisible();

    // --- PARTE 2: Moderación de Espacios con Concurrencia Optimista (409) ---
    await page.getByRole("link", { name: "Espacios" }).click();
    await page.waitForURL(/\/administracion\/espacios$/);
    await expect(
      page.getByRole("heading", {
        name: /Moderación de Espacios y Publicaciones/i,
      }),
    ).toBeVisible();
    await expect(page.getByText("Bodega Centro Santiago 25m2")).toBeVisible();
    await expect(page.getByText("Activa", { exact: true })).toBeVisible();

    // 2.1 Simular conflicto HTTP 409
    store.simulate409OnSpaceModeration = true;
    await page.getByRole("button", { name: "Ocultar / Suspender" }).click();
    await expect(
      page.getByText(/Ocultar \/ Suspender Publicación/i),
    ).toBeVisible();
    await page.getByRole("button", { name: "Confirmar Suspensión" }).click();

    // Banner 409 presente
    await expect(
      page.getByText(/Conflicto de concurrencia detectado \(HTTP 409\)/i),
    ).toBeVisible();

    // 2.2 Desactivar conflicto y suspender exitosamente
    store.simulate409OnSpaceModeration = false;
    await page.getByRole("button", { name: "Confirmar Suspensión" }).click();

    // Publicación pasa a Oculta
    await expect(page.getByText("Oculta", { exact: true })).toBeVisible();
  });

  test("@phase9 flujo disputas (cierre irreversible) y cuentas (roles RBAC, bloqueo/desbloqueo, revocación de sesiones)", async ({
    page,
  }) => {
    const store = createAdminStore();
    await installMockGateway(page, store.handler);

    // --- PARTE 1: Disputas en /administracion/disputas ---
    await loginAndNavigate(page, "/administracion/disputas");
    await expect(
      page.getByRole("heading", {
        name: /Disputas, Reclamos y Moderación Operativa/i,
      }),
    ).toBeVisible();
    await expect(page.getByText("Abierta", { exact: true })).toBeVisible();

    // Cierre de disputa con confirmación irreversible
    await page
      .getByRole("button", { name: "Cerrar Disputa (Irreversible)" })
      .click();
    await expect(
      page.getByText(/Cierre Irreversible de Disputa/i),
    ).toBeVisible();

    await page
      .locator("#dispute-close-reason")
      .selectOption("ensayo_finalizado");
    await page
      .getByRole("button", { name: "Confirmar Cierre Irreversible" })
      .click();

    // Disputa pasa a cerrada
    await expect(page.getByText("Cerrada", { exact: true })).toBeVisible();

    // --- PARTE 2: Cuentas y Roles en /administracion/cuentas ---
    await page.getByRole("link", { name: "Cuentas y Roles" }).click();
    await page.waitForURL(/\/administracion\/cuentas$/);
    await expect(
      page.getByRole("heading", {
        name: /Gestión de Cuentas y Asignación de Roles/i,
      }),
    ).toBeVisible();
    await expect(page.getByText("usuario.prueba@espacigo.cl")).toBeVisible();

    // 2.1 Asignación de rol RBAC
    await page.getByRole("button", { name: "Modificar Roles" }).first().click();
    await expect(page.getByText(/Asignación de Roles RBAC/i)).toBeVisible();
    await page.locator("#target-role").selectOption("arrendador");
    await page.locator("#grant-action").selectOption("true");
    await page.getByRole("button", { name: "Guardar Rol" }).click();

    // 2.2 Bloqueo de usuario con motivo
    await page.getByRole("button", { name: "Bloquear" }).first().click();
    await expect(page.getByText(/Bloquear Cuenta de Usuario/i)).toBeVisible();
    await page
      .locator("#block-reason")
      .fill("Bloqueo administrativo por incumplimiento de términos.");
    await page.getByRole("button", { name: "Confirmar Bloqueo" }).click();

    // Badge bloqueada visible
    await expect(page.getByText("Bloqueada").first()).toBeVisible();

    // 2.3 Desbloqueo
    await page.getByRole("button", { name: "Desbloquear" }).first().click();
    await expect(
      page.getByText(/Desbloquear Cuenta de Usuario/i),
    ).toBeVisible();
    await page
      .locator("#block-reason")
      .fill("Desbloqueo tras aclaración y verificación satisfactoria.");
    await page.getByRole("button", { name: "Confirmar Desbloqueo" }).click();

    // Badge activo visible
    await expect(page.getByText("Activo", { exact: true })).toBeVisible();

    // 2.4 Revocación de sesiones
    await page
      .getByRole("button", { name: "Revocar Sesiones" })
      .first()
      .click();
    await expect(page.getByText(/Revocar Sesiones Activas/i)).toBeVisible();
    await page
      .getByRole("button", { name: "Confirmar Revocación Irreversible" })
      .click();
  });

  test("@phase9 flujo gates de seguridad: manejo de HTTP 428 y HTTP 403 con purga inmediata en memoria", async ({
    page,
  }) => {
    const store = createAdminStore();
    await installMockGateway(page, store.handler);

    // --- GATE 428: Precondition Required (Reautenticación requerida) ---
    store.simulate428OnPath = "/accounts/";
    await loginAndNavigate(page, "/administracion/cuentas");

    await page.getByRole("button", { name: "Bloquear" }).first().click();
    await page
      .locator("#block-reason")
      .fill("Bloqueo administrativo sensible que requiere auth reciente.");
    await page.getByRole("button", { name: "Confirmar Bloqueo" }).click();

    // Banner 428 visible
    await expect(
      page.getByText(/Reautenticación requerida \(HTTP 428\)/i),
    ).toBeVisible();

    // Cerrar modal de bloqueo antes de cerrar la alerta
    await page.getByRole("button", { name: "Cancelar" }).click();

    // Cerrar alerta 428
    await page.getByLabel("Cerrar alerta de reautenticación").click();
    await expect(
      page.getByText(/Reautenticación requerida \(HTTP 428\)/i),
    ).not.toBeVisible();

    // --- GATE 403: Forbidden con purga inmediata de datos privados en memoria ---
    store.simulate428OnPath = null;
    store.simulate403OnPath = "/admin/global/accounts";

    // Disparar recarga que retorna 403
    await page.getByRole("button", { name: "Actualizar" }).click();

    // Mensaje de purga inmediata en banner de error
    await expect(
      page.getByText(
        /Acceso denegado o sesión revocada. Los datos privados fueron purgados de la memoria./i,
      ),
    ).toBeVisible();

    // Directorio queda purgado y sin datos en memoria
    await expect(page.getByText(/No se encontraron cuentas\./i)).toBeVisible();
  });

  test("@phase9 @a11y todas las rutas de administración cumplen accesibilidad Axe y viewport sin overflow", async ({
    page,
  }) => {
    const store = createAdminStore();
    await installMockGateway(page, store.handler);

    const targetRoutes = [
      "/administracion",
      "/administracion/verificaciones",
      "/administracion/espacios",
      "/administracion/disputas",
      "/administracion/cuentas",
      "/administracion/auditoria",
    ];

    const viewports = [
      { name: "desktop", width: 1365, height: 900 },
      { name: "mobile", width: 390, height: 844 },
    ];

    for (const vp of viewports) {
      await page.setViewportSize({ width: vp.width, height: vp.height });

      for (const route of targetRoutes) {
        await loginAndNavigate(page, route);
        await page.waitForLoadState("domcontentloaded");

        // Verificar que no hay desbordamiento horizontal
        const hasOverflow = await page.evaluate(() => {
          return (
            document.documentElement.scrollWidth >
            document.documentElement.clientWidth
          );
        });
        expect(
          hasOverflow,
          `Horizontal overflow detected at ${route} on ${vp.name} (${vp.width}x${vp.height})`,
        ).toBe(false);

        // Auditoría Axe WCAG 2 AA
        const results = await new AxeBuilder({ page })
          .exclude("nextjs-portal")
          .analyze();

        expect(
          results.violations,
          `Axe violations found at ${route} on ${vp.name}: ${JSON.stringify(
            results.violations.map((v) => ({
              id: v.id,
              impact: v.impact,
              description: v.description,
            })),
            null,
            2,
          )}`,
        ).toEqual([]);
      }
    }
  });
});
