"use client";

import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import {
  canAccessSection,
  generateIdempotencyKey,
  hasCapability,
  isConcurrencyConflict,
  isForbiddenError,
  isRecentAuthRequired,
  isValidAdminReason,
  isValidExpectedUpdatedAt,
} from "../domain/admin-rules";
import type {
  AdminCapability,
  AdminDamageClaim,
  AdminDamageClaimResolutionInput,
  AdminDisputesReport,
  AdminFinanceReport,
  AdminLocalAccount,
  AdminLocalDispute,
  AdminLocalNotice,
  AdminPublishedSpace,
  AdminReservationDetail,
  AdminReservationSummary,
  AdminReservationsReport,
  AdminReviewReport,
  AdminSection,
  AdminVerificationCase,
  AuditEvent,
  AuditEventsQuery,
  GlobalAdminAccount,
  GlobalAdminAccountQuery,
  GlobalAdminKPIs,
  GlobalAdminMe,
  GlobalAdminOverview,
  GlobalAdminPublication,
  GlobalAdminPublicationQuery,
} from "../domain/types";
import type { AdminPort } from "./admin.port";

export interface AdminSnapshot {
  readonly me: GlobalAdminMe | null;
  readonly overview: GlobalAdminOverview | null;
  readonly kpis: GlobalAdminKPIs | null;
  readonly globalAccounts: readonly GlobalAdminAccount[];
  readonly selectedGlobalAccount: GlobalAdminAccount | null;
  readonly globalPublications: readonly GlobalAdminPublication[];
  readonly globalAuditEvents: readonly AuditEvent[];
  readonly verifications: readonly AdminVerificationCase[];
  readonly rejectedVerifications: readonly AdminVerificationCase[];
  readonly disputes: readonly AdminLocalDispute[];
  readonly reviewReports: readonly AdminReviewReport[];
  readonly notices: readonly AdminLocalNotice[];
  readonly localAccounts: readonly AdminLocalAccount[];
  readonly publishedSpaces: readonly AdminPublishedSpace[];
  readonly reservations: readonly AdminReservationSummary[];
  readonly currentReservation: AdminReservationDetail | null;
  readonly damageClaims: readonly AdminDamageClaim[];
  readonly currentDamageClaim: AdminDamageClaim | null;
  readonly reservationsReport: AdminReservationsReport | null;
  readonly financeReport: AdminFinanceReport | null;
  readonly disputesReport: AdminDisputesReport | null;
  readonly loading: boolean;
  readonly actionLoading: boolean;
  readonly error: string | null;
  readonly recentAuthRequired: boolean;
  readonly concurrencyConflict: boolean;
}

const initialSnapshot: AdminSnapshot = Object.freeze({
  me: null,
  overview: null,
  kpis: null,
  globalAccounts: Object.freeze([]),
  selectedGlobalAccount: null,
  globalPublications: Object.freeze([]),
  globalAuditEvents: Object.freeze([]),
  verifications: Object.freeze([]),
  rejectedVerifications: Object.freeze([]),
  disputes: Object.freeze([]),
  reviewReports: Object.freeze([]),
  notices: Object.freeze([]),
  localAccounts: Object.freeze([]),
  publishedSpaces: Object.freeze([]),
  reservations: Object.freeze([]),
  currentReservation: null,
  damageClaims: Object.freeze([]),
  currentDamageClaim: null,
  reservationsReport: null,
  financeReport: null,
  disputesReport: null,
  loading: false,
  actionLoading: false,
  error: null,
  recentAuthRequired: false,
  concurrencyConflict: false,
});

export class AdminFacade {
  #port: AdminPort;
  #abortController: AbortController | null = null;
  #snapshot: AdminSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();

  constructor(port: AdminPort) {
    this.#port = port;
  }

  getSnapshot = (): AdminSnapshot => {
    return this.#snapshot;
  };

  getServerSnapshot = (): AdminSnapshot => {
    return initialSnapshot;
  };

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  cancel(): void {
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
  }

  #setSnapshot(partial: Partial<AdminSnapshot>): void {
    this.#snapshot = Object.freeze({
      ...this.#snapshot,
      ...partial,
    });
    for (const listener of this.#listeners) {
      listener();
    }
  }

  clearError(): void {
    if (
      this.#snapshot.error ||
      this.#snapshot.recentAuthRequired ||
      this.#snapshot.concurrencyConflict
    ) {
      this.#setSnapshot({
        error: null,
        recentAuthRequired: false,
        concurrencyConflict: false,
      });
    }
  }

  /**
   * Gate Fase 9: Purga inmediata de datos privados en memoria tras revocación de sesión o HTTP 403
   */
  purgePrivateData(): void {
    privateBlobUrls.clear();
    this.#setSnapshot({
      overview: null,
      kpis: null,
      globalAccounts: Object.freeze([]),
      selectedGlobalAccount: null,
      globalPublications: Object.freeze([]),
      globalAuditEvents: Object.freeze([]),
      verifications: Object.freeze([]),
      rejectedVerifications: Object.freeze([]),
      disputes: Object.freeze([]),
      reviewReports: Object.freeze([]),
      notices: Object.freeze([]),
      localAccounts: Object.freeze([]),
      publishedSpaces: Object.freeze([]),
      reservations: Object.freeze([]),
      currentReservation: null,
      damageClaims: Object.freeze([]),
      currentDamageClaim: null,
      reservationsReport: null,
      financeReport: null,
      disputesReport: null,
      recentAuthRequired: false,
      concurrencyConflict: false,
    });
  }

  #handleError(err: unknown, fallbackMessage: string): void {
    if (isForbiddenError(err)) {
      this.purgePrivateData();
      this.#setSnapshot({
        error:
          "Acceso denegado o sesión revocada. Los datos privados fueron purgados de la memoria.",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    if (isRecentAuthRequired(err)) {
      this.#setSnapshot({
        recentAuthRequired: true,
        error:
          "Se requiere autenticación reciente para realizar esta operación administrativa sensible (código 428).",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    if (isConcurrencyConflict(err)) {
      this.#setSnapshot({
        concurrencyConflict: true,
        error:
          "Conflicto de concurrencia: el recurso fue modificado por otra sesión administrativa (código 409).",
        loading: false,
        actionLoading: false,
      });
      return;
    }
    const message = err instanceof Error ? err.message : fallbackMessage;
    this.#setSnapshot({
      error: message,
      loading: false,
      actionLoading: false,
    });
  }

  // -----------------------------------------------------
  // Capabilities & Authorization Checks
  // -----------------------------------------------------

  canAccess(section: AdminSection): boolean {
    return canAccessSection(
      this.#snapshot.me?.capabilities,
      section,
      this.#snapshot.me?.roles,
    );
  }

  hasCapability(capability: AdminCapability): boolean {
    return hasCapability(this.#snapshot.me?.capabilities, capability);
  }

  // -----------------------------------------------------
  // Global Admin Actions
  // -----------------------------------------------------

  async loadGlobalMe(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const me = await this.#port.getGlobalAdminMe(token);
      this.#setSnapshot({ me, loading: false });
    } catch (err) {
      this.#handleError(
        err,
        "No pudimos cargar las capacidades administrativas.",
      );
    }
  }

  async loadOverview(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const overview = await this.#port.getGlobalAdminOverview(token);
      this.#setSnapshot({ overview, loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar el resumen global.");
    }
  }

  async loadKPIs(
    token: AccessToken,
    query: { from: string; until: string; time_zone: string },
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const kpis = await this.#port.getGlobalAdminKPIs(token, query);
      this.#setSnapshot({ kpis, loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los KPIs globales.");
    }
  }

  async loadGlobalAccounts(
    token: AccessToken,
    query?: GlobalAdminAccountQuery,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const { items } = await this.#port.listGlobalAccounts(token, query);
      this.#setSnapshot({
        globalAccounts: Object.freeze(items),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar las cuentas globales.");
    }
  }

  async selectGlobalAccount(
    token: AccessToken,
    accountId: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const account = await this.#port.getGlobalAccount(token, accountId);
      this.#setSnapshot({ selectedGlobalAccount: account, loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar el detalle de la cuenta.");
    }
  }

  async updateGlobalAccountProfile(
    token: AccessToken,
    accountId: string,
    input: {
      display_name: string;
      phone?: string | null;
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
  ): Promise<boolean> {
    if (!isValidExpectedUpdatedAt(input.expected_updated_at)) {
      this.#setSnapshot({
        error:
          "La fecha esperada de actualización (expected_updated_at) es inválida.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.updateGlobalAccountProfile(token, accountId, input);
      // Refresh selected account if currently loaded
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.selectGlobalAccount(token, accountId);
      }
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos actualizar el perfil de la cuenta.");
      return false;
    }
  }

  async setGlobalAccountRole(
    token: AccessToken,
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.setGlobalAccountRole(
        token,
        accountId,
        role,
        granted,
        reason,
      );
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.selectGlobalAccount(token, accountId);
      }
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos asignar el rol a la cuenta.");
      return false;
    }
  }

  async setGlobalLegacyAdminRole(
    token: AccessToken,
    accountId: string,
    granted: boolean,
    reason:
      | "aprovisionamiento_sintetico"
      | "revision_administrativa"
      | "retirada_acceso",
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.setGlobalLegacyAdminRole(
        token,
        accountId,
        granted,
        reason,
      );
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.selectGlobalAccount(token, accountId);
      }
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos modificar el rol de administrador.");
      return false;
    }
  }

  async revokeGlobalAccountSessions(
    token: AccessToken,
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.revokeGlobalAccountSessions(token, accountId, reason);
      // If the revoked account is the current session account, purge private memory immediately
      if (this.#snapshot.me?.account_id === accountId) {
        this.purgePrivateData();
      }
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos revocar las sesiones de la cuenta.");
      return false;
    }
  }

  async setGlobalAccountBlocked(
    token: AccessToken,
    accountId: string,
    blocked: boolean,
    reason: string,
  ): Promise<boolean> {
    if (!isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de cambio de bloqueo debe contener al menos 5 caracteres válidos.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.setGlobalAccountBlocked(
        token,
        accountId,
        blocked,
        reason,
      );
      if (this.#snapshot.selectedGlobalAccount?.account_id === accountId) {
        await this.selectGlobalAccount(token, accountId);
      }
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos modificar el bloqueo de la cuenta.");
      return false;
    }
  }

  async loadGlobalPublications(
    token: AccessToken,
    query?: GlobalAdminPublicationQuery,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const pubs = await this.#port.listGlobalPublications(token, query);
      this.#setSnapshot({
        globalPublications: Object.freeze(pubs),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar las publicaciones globales.");
    }
  }

  async setGlobalPublicationState(
    token: AccessToken,
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
  ): Promise<boolean> {
    if (!isValidExpectedUpdatedAt(input.expected_updated_at)) {
      this.#setSnapshot({
        error:
          "La fecha esperada de actualización (expected_updated_at) es inválida.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.setGlobalPublicationState(token, publicationId, input);
      await this.loadGlobalPublications(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos cambiar el estado de la publicación.");
      return false;
    }
  }

  async retireGlobalPublication(
    token: AccessToken,
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
  ): Promise<boolean> {
    if (!isValidAdminReason(input.reason)) {
      this.#setSnapshot({
        error:
          "El motivo de retiro debe contener al menos 5 caracteres válidos.",
      });
      return false;
    }
    if (!isValidExpectedUpdatedAt(input.expected_updated_at)) {
      this.#setSnapshot({
        error:
          "La fecha esperada de actualización (expected_updated_at) es inválida.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.retireGlobalPublication(token, publicationId, input);
      await this.loadGlobalPublications(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos retirar la publicación.");
      return false;
    }
  }

  async loadGlobalAuditEvents(
    token: AccessToken,
    query: AuditEventsQuery,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const events = await this.#port.listGlobalAuditEvents(token, query);
      this.#setSnapshot({
        globalAuditEvents: Object.freeze(events),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los eventos de auditoría.");
    }
  }

  async exportGlobalAuditEvents(
    token: AccessToken,
    query: AuditEventsQuery,
  ): Promise<Blob | null> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const blob = await this.#port.exportGlobalAuditEvents(token, query);
      this.#setSnapshot({ actionLoading: false });
      return blob;
    } catch (err) {
      this.#handleError(err, "No pudimos exportar los eventos de auditoría.");
      return null;
    }
  }

  // -----------------------------------------------------
  // Local Admin Actions
  // -----------------------------------------------------

  async loadVerifications(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const items = await this.#port.listVerifications(token);
      this.#setSnapshot({
        verifications: Object.freeze(items),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar las verificaciones.");
    }
  }

  async loadRejectedVerifications(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const items = await this.#port.listRejectedVerifications(token);
      this.#setSnapshot({
        rejectedVerifications: Object.freeze(items),
        loading: false,
      });
    } catch (err) {
      this.#handleError(
        err,
        "No pudimos cargar las verificaciones rechazadas.",
      );
    }
  }

  async reviewVerification(
    token: AccessToken,
    id: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<boolean> {
    if (decision === "rechazar" && !isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de rechazo es obligatorio y debe tener al menos 5 caracteres válidos.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.reviewVerification(token, id, decision, reason);
      await this.loadVerifications(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos resolver el caso de verificación.");
      return false;
    }
  }

  async revokeVerification(
    token: AccessToken,
    id: string,
    reason_code:
      "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      await this.#port.revokeVerification(
        token,
        id,
        idempotencyKey,
        reason_code,
      );
      await this.loadVerifications(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos revocar la verificación.");
      return false;
    }
  }

  async loadDisputes(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const disputes = await this.#port.listDisputes(token);
      this.#setSnapshot({ disputes: Object.freeze(disputes), loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar las disputas locales.");
    }
  }

  async closeDispute(
    token: AccessToken,
    disputeId: string,
    reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      await this.#port.closeDispute(token, disputeId, reason_code);
      await this.loadDisputes(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos cerrar la disputa.");
      return false;
    }
  }

  async loadReviewReports(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const reports = await this.#port.listReviewReports(token);
      this.#setSnapshot({
        reviewReports: Object.freeze(reports),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los reportes de reseñas.");
    }
  }

  async moderateReviewReport(
    token: AccessToken,
    reportId: string,
    action: "dismiss" | "hide",
    reason_code:
      | "sin_infraccion"
      | "contenido_inadecuado"
      | "duplicado"
      | "error_registro",
    correlation_id?: string,
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      await this.#port.moderateReviewReport(
        token,
        reportId,
        action,
        idempotencyKey,
        {
          reason_code,
          correlation_id: correlation_id ?? generateIdempotencyKey(),
        },
      );
      await this.loadReviewReports(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos moderar el reporte de reseña.");
      return false;
    }
  }

  async loadNotices(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const notices = await this.#port.listNotices(token);
      this.#setSnapshot({ notices: Object.freeze(notices), loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los avisos locales.");
    }
  }

  async reopenNotice(
    token: AccessToken,
    noticeId: string,
    reason_code: "smtp_restaurado" | "reintento_operativo",
    correlation_id?: string,
  ): Promise<boolean> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      await this.#port.reopenNotice(token, noticeId, idempotencyKey, {
        reason_code,
        correlation_id: correlation_id ?? generateIdempotencyKey(),
      });
      await this.loadNotices(token);
      this.#setSnapshot({ actionLoading: false });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos reabrir el aviso.");
      return false;
    }
  }

  async loadLocalAccount(token: AccessToken, accountId: string): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const account = await this.#port.getLocalAccount(token, accountId);
      if (account) {
        this.#setSnapshot({
          localAccounts: Object.freeze([account]),
          loading: false,
        });
      } else {
        this.#setSnapshot({ loading: false });
      }
    } catch (err) {
      this.#handleError(err, "No pudimos cargar la cuenta local.");
    }
  }

  async blockLocalAccount(
    token: AccessToken,
    accountId: string,
    reason: string,
  ): Promise<boolean> {
    if (!isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de bloqueo local debe contener al menos 5 caracteres válidos.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.blockLocalAccount(
        token,
        accountId,
        reason,
      );
      this.#setSnapshot({
        localAccounts: Object.freeze([updated]),
        actionLoading: false,
      });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos bloquear la cuenta local.");
      return false;
    }
  }

  async unblockLocalAccount(
    token: AccessToken,
    accountId: string,
    reason: string,
  ): Promise<boolean> {
    if (!isValidAdminReason(reason)) {
      this.#setSnapshot({
        error:
          "El motivo de desbloqueo debe contener al menos 5 caracteres válidos.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.unblockLocalAccount(
        token,
        accountId,
        reason,
      );
      this.#setSnapshot({
        localAccounts: Object.freeze([updated]),
        actionLoading: false,
      });
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos desbloquear la cuenta local.");
      return false;
    }
  }

  async loadPublishedSpaces(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const spaces = await this.#port.listPublishedSpaces(token);
      this.#setSnapshot({
        publishedSpaces: Object.freeze(spaces),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los espacios publicados.");
    }
  }

  async loadReports(
    token: AccessToken,
    query: { from: string; until: string; timezone: string },
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const [reservationsReport, financeReport, disputesReport] =
        await Promise.all([
          this.#port.getReservationsReport(token, query),
          this.#port.getFinanceReport(token, query),
          this.#port.getDisputesReport(token, query),
        ]);
      this.#setSnapshot({
        reservationsReport,
        financeReport,
        disputesReport,
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los reportes operativos.");
    }
  }

  async loadLocalReservations(
    token: AccessToken,
    query?: {
      reservation_id?: string;
      state?: string;
      created_from?: string;
      created_to?: string;
      page_size?: number;
      cursor?: string;
    },
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const items = await this.#port.listLocalReservations(token, query);
      this.#setSnapshot({ reservations: Object.freeze(items), loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar las reservas administrativas.");
    }
  }

  async loadLocalReservation(
    token: AccessToken,
    reservationId: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.getLocalReservation(token, reservationId);
      this.#setSnapshot({ currentReservation: detail, loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar el detalle de la reserva.");
    }
  }

  async loadDamageClaims(token: AccessToken): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const claims = await this.#port.listDamageClaims(token);
      this.#setSnapshot({
        damageClaims: Object.freeze(claims),
        loading: false,
      });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar los reclamos por daños.");
    }
  }

  async loadDamageClaim(token: AccessToken, claimId: string): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const claim = await this.#port.getDamageClaim(token, claimId);
      this.#setSnapshot({ currentDamageClaim: claim, loading: false });
    } catch (err) {
      this.#handleError(err, "No pudimos cargar el reclamo por daños.");
    }
  }

  async resolveDamageClaim(
    token: AccessToken,
    claimId: string,
    input: AdminDamageClaimResolutionInput,
  ): Promise<boolean> {
    if (!isValidAdminReason(input.resolution_notes)) {
      this.#setSnapshot({
        error:
          "Las notas de resolución de daños deben contener al menos 5 caracteres válidos.",
      });
      return false;
    }
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const idempotencyKey = generateIdempotencyKey();
      const updated = await this.#port.resolveDamageClaim(
        token,
        claimId,
        idempotencyKey,
        input,
      );
      this.#setSnapshot({
        currentDamageClaim: updated,
        actionLoading: false,
      });
      await this.loadDamageClaims(token);
      return true;
    } catch (err) {
      this.#handleError(err, "No pudimos resolver el reclamo por daños.");
      return false;
    }
  }
}
