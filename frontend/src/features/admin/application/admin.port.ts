import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
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

export interface AdminPort {
  // Global Admin
  getGlobalAdminMe(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<GlobalAdminMe>;

  getGlobalAdminOverview(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<GlobalAdminOverview>;

  getGlobalAdminKPIs(
    token: AccessToken,
    query: { from: string; until: string; time_zone: string },
    signal?: AbortSignal,
  ): Promise<GlobalAdminKPIs>;

  listGlobalAuditEvents(
    token: AccessToken,
    query: AuditEventsQuery,
    signal?: AbortSignal,
  ): Promise<AuditEvent[]>;

  exportGlobalAuditEvents(
    token: AccessToken,
    query: AuditEventsQuery,
    signal?: AbortSignal,
  ): Promise<Blob>;

  listGlobalAccounts(
    token: AccessToken,
    query?: GlobalAdminAccountQuery,
    signal?: AbortSignal,
  ): Promise<{ items: GlobalAdminAccount[]; next_cursor?: string }>;

  getGlobalAccount(
    token: AccessToken,
    accountId: string,
    signal?: AbortSignal,
  ): Promise<GlobalAdminAccount | null>;

  updateGlobalAccountProfile(
    token: AccessToken,
    accountId: string,
    input: {
      display_name: string;
      phone?: string | null;
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    signal?: AbortSignal,
  ): Promise<void>;

  setGlobalAccountRole(
    token: AccessToken,
    accountId: string,
    role: "arrendatario" | "arrendador",
    granted: boolean,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    signal?: AbortSignal,
  ): Promise<void>;

  setGlobalLegacyAdminRole(
    token: AccessToken,
    accountId: string,
    granted: boolean,
    reason:
      | "aprovisionamiento_sintetico"
      | "revision_administrativa"
      | "retirada_acceso",
    signal?: AbortSignal,
  ): Promise<void>;

  revokeGlobalAccountSessions(
    token: AccessToken,
    accountId: string,
    reason:
      "correccion_perfil" | "revision_administrativa" | "gestion_operativa",
    signal?: AbortSignal,
  ): Promise<void>;

  setGlobalAccountBlocked(
    token: AccessToken,
    accountId: string,
    blocked: boolean,
    reason: string,
    signal?: AbortSignal,
  ): Promise<void>;

  listGlobalPublications(
    token: AccessToken,
    query?: GlobalAdminPublicationQuery,
    signal?: AbortSignal,
  ): Promise<GlobalAdminPublication[]>;

  setGlobalPublicationState(
    token: AccessToken,
    publicationId: string,
    input: {
      state: "activa" | "oculta";
      expected_updated_at: string;
      reason:
        "correccion_perfil" | "revision_administrativa" | "gestion_operativa";
    },
    signal?: AbortSignal,
  ): Promise<void>;

  retireGlobalPublication(
    token: AccessToken,
    publicationId: string,
    input: {
      owner_id: string;
      expected_updated_at: string;
      reason: string;
    },
    signal?: AbortSignal,
  ): Promise<void>;

  // Local / Operativa Admin
  listVerifications(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase[]>;

  listRejectedVerifications(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase[]>;

  reviewVerification(
    token: AccessToken,
    id: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase>;

  revokeVerification(
    token: AccessToken,
    id: string,
    idempotencyKey: string,
    reason_code:
      "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada",
    signal?: AbortSignal,
  ): Promise<AdminVerificationCase>;

  listDisputes(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminLocalDispute[]>;

  closeDispute(
    token: AccessToken,
    disputeId: string,
    reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada",
    signal?: AbortSignal,
  ): Promise<AdminLocalDispute>;

  listReviewReports(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminReviewReport[]>;

  moderateReviewReport(
    token: AccessToken,
    reportId: string,
    action: "dismiss" | "hide",
    idempotencyKey: string,
    input: {
      reason_code:
        | "sin_infraccion"
        | "contenido_inadecuado"
        | "duplicado"
        | "error_registro";
      correlation_id: string;
    },
    signal?: AbortSignal,
  ): Promise<AdminReviewReport>;

  listNotices(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminLocalNotice[]>;

  reopenNotice(
    token: AccessToken,
    noticeId: string,
    idempotencyKey: string,
    input: {
      reason_code: "smtp_restaurado" | "reintento_operativo";
      correlation_id: string;
    },
    signal?: AbortSignal,
  ): Promise<AdminLocalNotice>;

  listLocalAuditEvents(
    token: AccessToken,
    query: {
      from: string;
      until: string;
      actor_id?: string;
      resource_type?: string;
      resource_id?: string;
      action?: string;
      result?: "exito" | "rechazo";
    },
    signal?: AbortSignal,
  ): Promise<AuditEvent[]>;

  exportLocalAuditEvents(
    token: AccessToken,
    query: {
      from: string;
      until: string;
      actor_id?: string;
      resource_type?: string;
      resource_id?: string;
      action?: string;
      result?: "exito" | "rechazo";
    },
    signal?: AbortSignal,
  ): Promise<Blob>;

  getLocalAccount(
    token: AccessToken,
    accountId: string,
    signal?: AbortSignal,
  ): Promise<AdminLocalAccount | null>;

  blockLocalAccount(
    token: AccessToken,
    accountId: string,
    reason: string,
    signal?: AbortSignal,
  ): Promise<AdminLocalAccount>;

  unblockLocalAccount(
    token: AccessToken,
    accountId: string,
    reason: string,
    signal?: AbortSignal,
  ): Promise<AdminLocalAccount>;

  listPublishedSpaces(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminPublishedSpace[]>;

  getPublishedSpace(
    token: AccessToken,
    spaceId: string,
    signal?: AbortSignal,
  ): Promise<AdminPublishedSpace | null>;

  getReservationsReport(
    token: AccessToken,
    query: { from: string; until: string; timezone: string },
    signal?: AbortSignal,
  ): Promise<AdminReservationsReport>;

  getFinanceReport(
    token: AccessToken,
    query: { from: string; until: string; timezone: string },
    signal?: AbortSignal,
  ): Promise<AdminFinanceReport>;

  getDisputesReport(
    token: AccessToken,
    query: { from: string; until: string; timezone: string },
    signal?: AbortSignal,
  ): Promise<AdminDisputesReport>;

  listLocalReservations(
    token: AccessToken,
    query?: {
      reservation_id?: string;
      state?: string;
      created_from?: string;
      created_to?: string;
      page_size?: number;
      cursor?: string;
    },
    signal?: AbortSignal,
  ): Promise<AdminReservationSummary[]>;

  getLocalReservation(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<AdminReservationDetail | null>;

  listDamageClaims(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<AdminDamageClaim[]>;

  getDamageClaim(
    token: AccessToken,
    claimId: string,
    signal?: AbortSignal,
  ): Promise<AdminDamageClaim | null>;

  resolveDamageClaim(
    token: AccessToken,
    claimId: string,
    idempotencyKey: string,
    input: AdminDamageClaimResolutionInput,
    signal?: AbortSignal,
  ): Promise<AdminDamageClaim>;
}
