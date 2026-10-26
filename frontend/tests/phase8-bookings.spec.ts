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
  BookingDetail,
  BookingQuote,
  BookingReservation,
  BusinessClock,
  ContractSignature,
  Guarantee,
  RentalOperation,
  ReservationMessage,
  ReservationReview,
  Settlement,
  SyntheticContract,
  TrialDispute,
} from "@/features/bookings/domain/types";

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

const canonicalAccountId = "770e8400-e29b-41d4-a716-446655440000";
const canonicalHostId = "880e8400-e29b-41d4-a716-446655440000";
const canonicalReservationId = "res-test-1";
const canonicalSpaceId = "space-test-01";
const canonicalContractId = "cnt-test-01";

function apiSuccess<T>(data: T): MockResult {
  return {
    status: 200,
    body: { data },
  };
}

async function loginAndNavigate(page: Page, destination = "/reservas") {
  const [pathname] = destination.split("?");

  const header = page.locator("header");
  const isConfirmed =
    (await header
      .getByRole("button", { name: "Cerrar sesión", exact: true })
      .isVisible()
      .catch(() => false)) ||
    (await header
      .getByRole("link", { name: "Mi cuenta", exact: true })
      .isVisible()
      .catch(() => false));

  if (isConfirmed) {
    await page.goto(destination);
    await page.waitForURL((url) => url.pathname === pathname);
    await header
      .getByRole("button", { name: "Cerrar sesión", exact: true })
      .waitFor({ state: "visible" });
    return;
  }

  await page.goto(`/acceso?return_to=${encodeURIComponent(pathname)}`);

  const emailInput = page.getByLabel("Correo electrónico");
  const authOrGuest = await Promise.race([
    page
      .waitForURL((url) => url.pathname === pathname)
      .then(() => "authenticated" as const),
    emailInput.waitFor({ state: "visible" }).then(() => "guest" as const),
  ]);

  if (authOrGuest === "guest") {
    await emailInput.fill("arrendatario.test@example.invalid");
    await page.getByLabel("Contraseña", { exact: true }).fill("SecurePass123!");
    await page.getByRole("button", { name: "Iniciar sesión" }).click();
    await page.waitForURL((url) => url.pathname === pathname);
  }
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

interface TestBookingStoreState {
  reservation: BookingDetail;
  contract: SyntheticContract;
  operations: RentalOperation[];
  messages: ReservationMessage[];
  reviews: ReservationReview[];
  disputes: TrialDispute[];
  guarantee: Guarantee;
  settlement: Settlement;
  businessClock: BusinessClock;
}

function createBookingStore() {
  const reservation: BookingDetail = {
    id: canonicalReservationId,
    quote_id: "quote-uuid-1234",
    space_id: canonicalSpaceId,
    renter_id: canonicalAccountId,
    host_id: canonicalHostId,
    state: "pendiente_de_pago",
    units: 2,
    rate_unit: "dia",
    unit_price_clp: 25000,
    currency: "CLP",
    subtotal_clp: 50000,
    start_at: "2026-11-01T10:00:00Z",
    end_at: "2026-11-03T10:00:00Z",
    time_zone: "America/Santiago",
    conditions: "Normas estándar de uso seguro.",
    cancellation_policy_version: "v1-flexible",
    guarantee_policy_version: "v1-standard",
    guarantee_currency: "CLP",
    guarantee_expected_clp: 15000,
    settlement_policy_version: "v1-direct",
    settlement_commission_bps: 1000,
    pay_expires_at: "2026-11-01T09:00:00Z",
    created_at: "2026-10-25T12:00:00Z",
    updated_at: "2026-10-25T12:00:00Z",
    unread_count: 1,
    history: [
      {
        sequence: 1,
        to: "pendiente_de_pago",
        actor_id: canonicalAccountId,
        reason: "Reserva creada exitosamente",
        at: "2026-10-25T12:00:00Z",
      },
    ],
  };

  const initialSignatures: ContractSignature[] = [
    {
      signer_id: canonicalAccountId,
      role: "arrendatario",
      state: "pendiente",
      updated_at: "2026-10-25T12:00:00Z",
    },
    {
      signer_id: canonicalHostId,
      role: "anfitrion",
      state: "pendiente",
      updated_at: "2026-10-25T12:00:00Z",
    },
  ];

  const contract: SyntheticContract = {
    id: canonicalContractId,
    reservation_id: canonicalReservationId,
    version: 1,
    state: "generado",
    snapshot: "Contrato digital vinculante de prueba EspaciGo",
    sha256: "a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0",
    signatures: initialSignatures,
    history: [
      {
        sequence: 1,
        action: "generacion_automatica",
        occurred_at: "2026-10-25T12:00:00Z",
      },
    ],
  };

  const operations: RentalOperation[] = [];

  const messages: ReservationMessage[] = [
    {
      id: "msg-1",
      reservation_id: canonicalReservationId,
      author_id: canonicalAccountId,
      sequence: 1,
      body: "Hola, ¿está disponible el acceso vehicular?",
      created_at: "2026-10-25T12:05:00Z",
    },
  ];

  const reviews: ReservationReview[] = [];
  const disputes: TrialDispute[] = [];

  const guarantee: Guarantee = {
    policy_version: "v1-standard",
    currency: "CLP",
    expected_clp: 15000,
    authorized_clp: 15000,
    captured_clp: 0,
    released_clp: 15000,
    state: "autorizada",
    operations: [],
  };

  const settlement: Settlement = {
    id: "stl-test-01",
    reservation_id: canonicalReservationId,
    policy_version: "v1-direct",
    currency: "CLP",
    gross_rent_clp: 50000,
    confirmed_refunds_clp: 0,
    commission_bps: 1000,
    commission_clp: 5000,
    transfer_clp: 45000,
    state: "pendiente",
    operation_id: "stl-op-01",
    created_at: "2026-10-25T12:00:00Z",
  };

  const businessClock: BusinessClock = {
    current_time: "2026-10-25T12:30:00Z",
    time_zone: "America/Santiago",
  };

  let lastReadSequence: number | null = null;
  let capturedIdempotencyKey: string | null = null;

  const storeState: TestBookingStoreState = {
    reservation,
    contract,
    operations,
    messages,
    reviews,
    disputes,
    guarantee,
    settlement,
    businessClock,
  };

  return {
    storeState,
    get lastReadSequence() {
      return lastReadSequence;
    },
    get capturedIdempotencyKey() {
      return capturedIdempotencyKey;
    },
    handler: (route: Route, request: Request): MockResult | null => {
      const url = new URL(request.url());
      const pathname = url.pathname;
      const method = request.method();

      // Auth Session Profile & Login
      if (pathname === "/api/backend/auth/login" && method === "POST") {
        return {
          status: 200,
          body: {
            account_id: canonicalAccountId,
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
            account_id: canonicalAccountId,
            roles: ["arrendatario", "anfitrion"],
            capabilities: [],
            restricted_mode: false,
          },
        };
      }

      // 1. Quotes: POST /local/booking-trial/quotes
      if (
        pathname === "/api/backend/local/booking-trial/quotes" &&
        method === "POST"
      ) {
        const body = request.postDataJSON() as Record<string, unknown>;
        const spaceId = String(body?.space_id ?? canonicalSpaceId);
        const quote: BookingQuote = {
          id: "quote-uuid-1234",
          space_id: spaceId,
          rate_version: 1,
          rate_unit: "dia",
          unit_price_clp: 25000,
          currency: "CLP",
          units: 2,
          subtotal_clp: 50000,
          start_at: String(body?.start_at ?? "2026-11-01T10:00:00Z"),
          end_at: String(body?.end_at ?? "2026-11-03T10:00:00Z"),
          time_zone: "America/Santiago",
          conditions: "Uso formal",
          category_code: "bodega",
          profile_version: 1,
          profile_values: {},
          cancellation_policy_version: "v1-flexible",
          guarantee_policy_version: "v1-standard",
          guarantee_currency: "CLP",
          guarantee_expected_clp: 15000,
          settlement_policy_version: "v1-direct",
          settlement_commission_bps: 1000,
          created_at: new Date().toISOString(),
          expires_at: new Date(Date.now() + 15 * 60_000).toISOString(),
        };
        return apiSuccess(quote);
      }

      // 2. Reservations: POST /local/booking-trial/reservations
      if (
        pathname === "/api/backend/local/booking-trial/reservations" &&
        method === "POST"
      ) {
        capturedIdempotencyKey = request.headers()["idempotency-key"] ?? null;
        const res: BookingReservation = {
          ...storeState.reservation,
        };
        return apiSuccess(res);
      }

      // 3. Reservations List: GET /local/booking-trial/reservations
      if (
        pathname === "/api/backend/local/booking-trial/reservations" &&
        method === "GET"
      ) {
        const items: BookingReservation[] = [storeState.reservation];
        return apiSuccess({ items, cursor: null });
      }

      // 4. Reservation Detail: GET /local/booking-trial/reservations/:id
      const detailMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)$/.exec(
          pathname,
        );
      if (detailMatch && method === "GET") {
        return apiSuccess(storeState.reservation);
      }

      // 5. Payment Simulation: POST /local/booking-trial/reservations/:id/payment
      const paymentMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/payment$/.exec(
          pathname,
        );
      if (paymentMatch && method === "POST") {
        storeState.reservation = {
          ...storeState.reservation,
          state: "pagada",
        };
        return apiSuccess(storeState.reservation);
      }

      // 6. Host Decision: POST /local/booking-trial/reservations/:id/decision
      const acceptMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/decision$/.exec(
          pathname,
        );
      if (acceptMatch && method === "POST") {
        storeState.reservation = {
          ...storeState.reservation,
          state: "aprobada_host",
        };
        return apiSuccess(storeState.reservation);
      }

      // 7. Contract create or get: POST /local/booking-trial/reservations/:id/contract
      const getContractMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/contract$/.exec(
          pathname,
        );
      if (getContractMatch && method === "POST") {
        return apiSuccess(storeState.contract);
      }

      // 8. Sign Contract: POST /local/booking-trial/contracts/:id/sign
      const signMatch =
        /^\/api\/backend\/local\/booking-trial\/contracts\/([^/]+)\/sign$/.exec(
          pathname,
        );
      if (signMatch && method === "POST") {
        const updatedSignatures: ContractSignature[] = [
          {
            signer_id: canonicalAccountId,
            role: "arrendatario",
            state: "firmada",
            updated_at: new Date().toISOString(),
          },
          {
            signer_id: canonicalHostId,
            role: "anfitrion",
            state: "firmada",
            updated_at: new Date().toISOString(),
          },
        ];
        storeState.contract = {
          ...storeState.contract,
          state: "firmado",
          signatures: updatedSignatures,
        };
        storeState.reservation = {
          ...storeState.reservation,
          state: "lista_para_checkin",
        };
        return apiSuccess(storeState.contract);
      }

      // 9. Contract PDF: GET /local/booking-trial/contracts/:id/document
      const pdfMatch =
        /^\/api\/backend\/local\/booking-trial\/contracts\/([^/]+)\/document$/.exec(
          pathname,
        );
      if (pdfMatch && method === "GET") {
        return {
          status: 200,
          contentType: "application/pdf",
          body: "%PDF-1.4 Mock Contract Content",
        };
      }

      // 10. Operations: check-in, check-out, reception
      const checkinMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/check-in$/.exec(
          pathname,
        );
      if (checkinMatch && method === "POST") {
        const now = new Date().toISOString();
        storeState.reservation = {
          ...storeState.reservation,
          state: "en_curso",
        };
        const op: RentalOperation = {
          id: `op-${Date.now()}`,
          reservation_id: storeState.reservation.id,
          kind: "checkin",
          actor_id: canonicalAccountId,
          occurred_at: now,
          time_zone: "America/Santiago",
          comments: "Check-in formal realizado",
          result: "registrada",
          sequence: storeState.operations.length + 1,
          evidence: [],
        };
        storeState.operations.push(op);
        return apiSuccess(op);
      }

      const checkoutMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/check-out$/.exec(
          pathname,
        );
      if (checkoutMatch && method === "POST") {
        const now = new Date().toISOString();
        const op: RentalOperation = {
          id: `op-${Date.now()}`,
          reservation_id: storeState.reservation.id,
          kind: "checkout",
          actor_id: canonicalAccountId,
          occurred_at: now,
          time_zone: "America/Santiago",
          comments: "Entrega puntual en perfecto orden",
          result: "registrada",
          sequence: storeState.operations.length + 1,
          evidence: [],
        };
        storeState.operations.push(op);
        return apiSuccess(op);
      }

      const receptionMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/reception$/.exec(
          pathname,
        );
      if (receptionMatch && method === "POST") {
        const now = new Date().toISOString();
        storeState.reservation = {
          ...storeState.reservation,
          state: "finalizada",
        };
        const op: RentalOperation = {
          id: `op-${Date.now()}`,
          reservation_id: storeState.reservation.id,
          kind: "recepcion",
          actor_id: canonicalHostId,
          occurred_at: now,
          time_zone: "America/Santiago",
          observations: "Recepción conforme sin daños",
          result: "recepcion_conforme",
          sequence: storeState.operations.length + 1,
          evidence: [],
        };
        storeState.operations.push(op);
        return apiSuccess(op);
      }

      const operationsMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/operations$/.exec(
          pathname,
        );
      if (operationsMatch && method === "GET") {
        return apiSuccess({ items: storeState.operations });
      }

      // 11. Messages: GET, POST, READ
      const messagesMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/messages$/.exec(
          pathname,
        );
      if (messagesMatch && method === "GET") {
        return apiSuccess({ items: storeState.messages });
      }
      if (messagesMatch && method === "POST") {
        const body = request.postDataJSON() as { body: string };
        const newMsg: ReservationMessage = {
          id: `msg-${Date.now()}`,
          reservation_id: storeState.reservation.id,
          author_id: canonicalAccountId,
          sequence: storeState.messages.length + 1,
          body: body.body,
          created_at: new Date().toISOString(),
        };
        storeState.messages.push(newMsg);
        return apiSuccess(newMsg);
      }

      const readMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/messages\/read$/.exec(
          pathname,
        );
      if (readMatch && method === "POST") {
        const body = request.postDataJSON() as {
          through_sequence?: number;
          last_read_sequence?: number;
        };
        lastReadSequence =
          body?.through_sequence ?? body?.last_read_sequence ?? 1;
        return apiSuccess({ read_through_sequence: lastReadSequence });
      }

      // 12. Reviews: GET, POST
      const reviewsMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/reviews$/.exec(
          pathname,
        );
      if (reviewsMatch && method === "GET") {
        return apiSuccess({ items: storeState.reviews });
      }
      if (reviewsMatch && method === "POST") {
        const body = request.postDataJSON() as {
          rating: number;
          comment?: string;
          target_type?: "espacio" | "arrendatario";
        };
        const newRev: ReservationReview = {
          id: `rev-${Date.now()}`,
          target_type: body.target_type ?? "espacio",
          rating: body.rating,
          state: "publicada",
          comment: body.comment,
          created_at: new Date().toISOString(),
        };
        storeState.reviews.push(newRev);
        return apiSuccess(newRev);
      }

      // 13. Disputes: GET, POST
      const disputesMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/disputes$/.exec(
          pathname,
        );
      if (disputesMatch && method === "GET") {
        return {
          status: 200,
          body: { items: storeState.disputes },
        };
      }
      if (disputesMatch && method === "POST") {
        const body = request.postDataJSON() as { reason_code: string };
        const newDisp: TrialDispute = {
          id: `disp-${Date.now()}`,
          reservation_id: storeState.reservation.id,
          host_id: canonicalHostId,
          renter_id: canonicalAccountId,
          opened_by: canonicalAccountId,
          opening_reason_code: body.reason_code ?? "dano_inmueble",
          state: "abierta",
          opened_at: new Date().toISOString(),
          reused: false,
        };
        storeState.disputes.push(newDisp);
        return {
          status: 200,
          body: newDisp,
        };
      }

      // 14. Damage Claim: GET /local/booking-trial/reservations/:id/damage-claim
      const damageClaimMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/damage-claim$/.exec(
          pathname,
        );
      if (damageClaimMatch && method === "GET") {
        return apiSuccess(null);
      }

      // 15. Guarantee & Settlement & Clock
      const guaranteeMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/guarantee$/.exec(
          pathname,
        );
      if (guaranteeMatch && method === "GET") {
        return apiSuccess(storeState.guarantee);
      }
      if (guaranteeMatch && method === "POST") {
        return apiSuccess(storeState.guarantee);
      }

      const settlementMatch =
        /^\/api\/backend\/local\/booking-trial\/reservations\/([^/]+)\/settlement$/.exec(
          pathname,
        );
      if (settlementMatch && method === "GET") {
        return apiSuccess(storeState.settlement);
      }

      if (pathname === "/api/backend/local/booking-trial/business-clock") {
        return apiSuccess({ now: storeState.businessClock.current_time });
      }

      return null;
    },
  };
}

test.describe("Phase 8 Bookings — Controlled E2E Suite", () => {
  test("@phase8 Quote and booking request calculates CLP breakdown and creates reservation with Idempotency-Key UUIDv4", async ({
    page,
  }) => {
    const store = createBookingStore();
    await installMockGateway(page, store.handler);

    await loginAndNavigate(page, "/reservas/nueva");

    // Pre-fill spaceId if input field is shown
    const spaceInput = page.locator("#quote-space-id");
    if (await spaceInput.isVisible()) {
      await spaceInput.fill("space-test-01");
    }

    // Configure quote duration and unit
    await page.locator("#quote-rate-unit").selectOption("dia");
    await page.locator("#quote-units").fill("2");

    // Request quote
    await page
      .getByRole("button", { name: "Obtener Cotización Vinculante" })
      .click();

    // Verify quote calculated and breakdown visible with exact CLP
    await expect(page.getByText("Cotización Calculada")).toBeVisible();
    await expect(
      page.getByText(/Tarifa base \(2 di.*a.*25\.000 CLP\)/),
    ).toBeVisible();
    await expect(page.getByText("50.000 CLP").first()).toBeVisible();
    await expect(
      page.getByText("Garantía de respaldo (reembolsable)"),
    ).toBeVisible();
    await expect(page.getByText("15.000 CLP")).toBeVisible();

    // Create reservation
    await page.getByRole("button", { name: /Confirmar y Reservar/ }).click();

    // Verify redirect to /reservas/:id
    await expect(page).toHaveURL(/\/reservas\/res-test-1$/);

    // Verify Idempotency-Key was sent and is a valid UUIDv4
    expect(store.capturedIdempotencyKey).toBeTruthy();
    const uuidRegex =
      /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
    expect(store.capturedIdempotencyKey).toMatch(uuidRegex);
  });

  test("@phase8 Reservations inbox lists bookings with filters and navigates to detail with historical snapshot", async ({
    page,
  }) => {
    const store = createBookingStore();
    await installMockGateway(page, store.handler);

    await loginAndNavigate(page, "/reservas");

    // Verify inbox heading and filter tabs
    await expect(page.locator("h1")).toHaveText("Mis Reservas");
    await expect(page.getByRole("button", { name: "Todas" })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Solicitadas" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "En Curso" })).toBeVisible();

    // Check listing card content
    await expect(page.getByText("ID: res-test")).toBeVisible();
    await expect(page.getByText("Pendiente de Pago")).toBeVisible();
    await expect(page.getByText("1 nuevo")).toBeVisible();

    // Filter tabs toggle
    await page.getByRole("button", { name: "En Curso" }).click();
    await expect(page.getByText("No se encontraron reservas")).toBeVisible();
    await page.getByRole("button", { name: "Todas" }).click();
    await expect(page.getByText("ID: res-test")).toBeVisible();

    // Navigate to reservation detail
    await page.getByRole("link", { name: "Gestionar" }).click();
    await expect(page).toHaveURL(/\/reservas\/res-test-1$/);

    // Verify detail snapshot and state
    await expect(page.locator("h1")).toHaveText("Arriendo de Espacio");
    await expect(page.getByText("Pendiente de Pago").first()).toBeVisible();
    await page.getByText(/Ver historial de transiciones/).click();
    await expect(page.getByText("Reserva creada exitosamente")).toBeVisible();
  });

  test("@phase8 Full operational lifecycle: payment notice, host decision, contracts, check-in, check-out, messaging, review and dispute", async ({
    page,
  }) => {
    const store = createBookingStore();
    await installMockGateway(page, store.handler);

    // --- Step 1: Arrendatario enters /reservas/res-test-1 and pays ---
    await loginAndNavigate(page, "/reservas/res-test-1");

    // Verify permanent Mercado Pago sandbox notice
    await expect(page.getByText("Ensayo Sandboxed")).toBeVisible();
    await expect(
      page.getByText("Aviso Permanente de Entorno de Ensayo"),
    ).toBeVisible();

    // Simulate Mercado Pago payment
    await page.getByRole("button", { name: "Simular Pago Exitoso" }).click();
    await expect(
      page.getByText("Pago Realizado / Esperando Anfitrión").first(),
    ).toBeVisible();

    // --- Step 2: Host decision on /anfitrion/reservas/res-test-1 ---
    await loginAndNavigate(page, "/anfitrion/reservas/res-test-1");
    await expect(page.getByText("Decisión del Anfitrión")).toBeVisible();
    await page
      .getByRole("button", { name: "Aceptar y Aprobar Reserva" })
      .click();
    await expect(
      page.getByText("Aprobada por Anfitrión").first(),
    ).toBeVisible();

    // --- Step 3: Digital contract signatures and secure PDF download ---
    await expect(page.getByText("Contrato Digital de Arriendo")).toBeVisible();

    // Download PDF with validation of Blob URL trigger
    const blobDownloadedPromise = page.evaluate(() => {
      return new Promise<boolean>((resolve) => {
        const origCreate = window.URL.createObjectURL;
        window.URL.createObjectURL = function (obj: Blob | MediaSource) {
          const url = origCreate.call(window.URL, obj);
          if (url.startsWith("blob:")) {
            resolve(true);
          }
          return url;
        };
      });
    });

    await page.getByRole("button", { name: "Descargar PDF Seguro" }).click();
    const blobDownloaded = await blobDownloadedPromise;
    expect(blobDownloaded).toBe(true);

    // Sign contract
    await page.getByRole("button", { name: "Firmar Contrato Digital" }).click();
    await expect(page.getByText("Lista para Check-in").first()).toBeVisible();
    await expect(page.getByText(/firmado/i).first()).toBeVisible();

    // --- Step 4: Physical Operations (Check-in, Check-out, Reception) ---
    await expect(page.getByText("Operación Física y Acceso")).toBeVisible();

    // Check-in
    await page
      .getByRole("button", { name: "Registrar Check-In Formal" })
      .click();
    await expect(page.getByText("Arriendo en Curso").first()).toBeVisible();

    // Check-out
    await page
      .getByRole("button", { name: "Registrar Check-Out y Entrega" })
      .click();

    // Reception (Host)
    await page
      .getByRole("button", { name: "Registrar Recepción de Inmueble" })
      .click();
    await expect(
      page.getByText("Finalizada (Check-out)").first(),
    ).toBeVisible();

    // --- Step 5: Messaging (Anti-XSS & Read Cursor Ack) ---
    const msgInput = page.getByPlaceholder(
      "Escribe un mensaje seguro (máx. 2000 caracteres)...",
    );
    const xssPayload = "<script>alert('xss')</script> Todo en orden, gracias";
    await msgInput.fill(xssPayload);
    await page.getByRole("button", { name: "Enviar" }).click();

    // Verify plain text anti-XSS rendering
    await expect(page.getByText(xssPayload)).toBeVisible();
    await expect.poll(() => store.lastReadSequence).toBeTruthy();

    // --- Step 6: Review submission with star rating ---
    await page.getByRole("radio", { name: "5 estrellas" }).click();
    await page
      .locator("#review-comment")
      .fill("Excelente espacio, muy limpio y seguro.");
    await page.getByRole("button", { name: "Publicar Reseña" }).click();
    await expect(page.getByText(/Reseña publicada con éxito/i)).toBeVisible();
    await expect(
      page.getByText("Excelente espacio, muy limpio y seguro."),
    ).toBeVisible();

    // --- Step 7: Dispute opening and Settlement Calculation ---
    await page.locator("#dispute-reason").selectOption("dano_inmueble");
    await page.getByRole("button", { name: "Abrir Disputa Formal" }).click();
    await expect(page.getByText(/Disputa abierta formalmente/i)).toBeVisible();

    // Settlement payout check
    await expect(page.getByText("Liquidación al Anfitrión")).toBeVisible();
    await expect(page.getByText("45.000 CLP")).toBeVisible(); // Net host payout
    await expect(page.getByRole("button", { name: "Liberar" })).toBeVisible();
  });

  test("@phase8 @a11y booking routes fit both viewports and have no Axe violations", async ({
    page,
  }) => {
    const store = createBookingStore();
    await installMockGateway(page, store.handler);

    const targetRoutes = [
      "/reservas",
      "/reservas/nueva",
      "/reservas/res-test-1",
      "/anfitrion/reservas",
      "/anfitrion/reservas/res-test-1",
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

        if (
          route === "/reservas/res-test-1" ||
          route === "/anfitrion/reservas/res-test-1"
        ) {
          await expect(
            page.getByRole("button", {
              name: "Consultar Política de Reembolso",
            }),
          ).toBeVisible();
          await page.evaluate(() => document.fonts.ready);
        }

        // Verify no horizontal overflow
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

        // Run Axe accessibility audit
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
