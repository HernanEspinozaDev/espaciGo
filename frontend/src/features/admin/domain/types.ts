export type AdminRole =
  "arrendatario" | "arrendador" | "administrador" | "administrador_global";

export type AdminCapability =
  | "accounts.list"
  | "accounts.read"
  | "accounts.update"
  | "accounts.block"
  | "accounts.roles"
  | "publications.list"
  | "publications.read"
  | "publications.update"
  | "publications.moderate"
  | "publications.retire"
  | "admin_access.manage"
  | "kpi.read"
  | "audit.read"
  | "audit.export"
  | "legacy.administration";

export type AdminSection =
  | "overview"
  | "accounts"
  | "publications"
  | "kpi"
  | "audit"
  | "verifications"
  | "disputes"
  | "review_reports"
  | "notices"
  | "spaces"
  | "reports"
  | "reservations"
  | "damage_claims";

export interface GlobalAdminMe {
  readonly account_id: string;
  readonly roles: readonly AdminRole[];
  readonly capabilities: readonly AdminCapability[];
  readonly generated_at: string;
}

export interface GlobalAdminOverview {
  readonly total_accounts: number | string;
  readonly administrative_blocked: number | string;
  readonly by_state: Readonly<Record<string, number | string>>;
  readonly by_role: Readonly<Record<string, number | string>>;
  readonly generated_at: string;
  readonly definition: string;
}

export interface GlobalAdminKPIs {
  readonly period: {
    readonly from: string;
    readonly until: string;
    readonly time_zone: string;
  };
  readonly generated_at: string;
  readonly definition: string;
  readonly metrics: Readonly<Record<string, unknown>>;
}

export type GlobalAdminAccountState =
  | "correo_pendiente"
  | "activo"
  | "bloqueado"
  | "baja_solicitada"
  | "desidentificado";

export interface GlobalAdminAccount {
  readonly account_id: string;
  readonly email: string;
  readonly display_name?: string | null;
  readonly phone?: string | null;
  readonly profile_updated_at?: string | null;
  readonly state: GlobalAdminAccountState;
  readonly created_at: string;
  readonly administrative_blocked: boolean;
  readonly roles: readonly AdminRole[];
  readonly publication_count: number | string;
  readonly reservation_count: number | string;
  readonly incident_count: number | string;
}

export interface GlobalAdminAccountQuery {
  readonly account_id?: string;
  readonly email_prefix?: string;
  readonly name_prefix?: string;
  readonly role?: AdminRole;
  readonly state?: GlobalAdminAccountState;
  readonly cursor?: string;
  readonly page_size?: number;
}

export type GlobalAdminPublicationState =
  "borrador" | "activa" | "oculta" | "eliminada";

export interface GlobalAdminPublicationPhoto {
  readonly photo_id: string;
  readonly url?: string;
  readonly is_cover?: boolean;
  readonly is_visible?: boolean;
}

export interface GlobalAdminPublication {
  readonly publication_id: string;
  readonly owner_id: string;
  readonly owner_name?: string | null;
  readonly owner_email: string;
  readonly category_code: string;
  readonly category_name: string;
  readonly title: string;
  readonly description: string;
  readonly area_m2: string;
  readonly capacity: number;
  readonly usage_rules: string;
  readonly address: string;
  readonly state: GlobalAdminPublicationState;
  readonly rate_unit: "hora" | "dia" | "mes";
  readonly base_price_clp: string;
  readonly updated_at: string;
  readonly publicly_visible: boolean;
  readonly public_cover_available: boolean;
  readonly attribute_schema_version: number;
  readonly attributes: Readonly<Record<string, unknown>>;
  readonly photos?: readonly GlobalAdminPublicationPhoto[];
}

export interface GlobalAdminPublicationQuery {
  readonly owner_id?: string;
  readonly category?: string;
  readonly state?: GlobalAdminPublicationState;
  readonly search?: string;
  readonly publicly_visible?: boolean;
  readonly page_size?: number;
  readonly cursor?: string;
}

export interface AuditEvent {
  readonly event_id: string;
  readonly actor_id: string;
  readonly target_account_id?: string | null;
  readonly resource_type?: string | null;
  readonly resource_id?: string | null;
  readonly action: string;
  readonly result: "exito" | "rechazo";
  readonly reason_code?: string | null;
  readonly correlation_id?: string | null;
  readonly timestamp: string;
  readonly details?: Readonly<Record<string, unknown>> | null;
}

export interface AuditEventsQuery {
  readonly from: string;
  readonly until: string;
  readonly actor_id?: string;
  readonly target_account_id?: string;
  readonly resource_type?: string;
  readonly resource_id?: string;
  readonly action?: string;
  readonly result?: "exito" | "rechazo";
  readonly reason_code?: string;
  readonly correlation_id?: string;
  readonly cursor?: string;
  readonly page_size?: number;
  readonly snapshot_cutoff?: string;
  readonly format?: "json" | "csv";
}

export interface AdminVerificationCase {
  readonly id: string;
  readonly type: "kyc" | "kyb";
  readonly state: "en_revision" | "aprobada" | "rechazada" | "revocada";
  readonly applicant_id: string;
  readonly correction_code?: string | null;
  readonly created_at: string;
  readonly updated_at: string;
  readonly notes?: string | null;
}

export interface AdminVerificationReviewInput {
  readonly decision: "aprobar" | "rechazar";
  readonly reason?: string;
}

export interface AdminLocalDispute {
  readonly dispute_id: string;
  readonly reservation_id: string;
  readonly renter_id: string;
  readonly host_id: string;
  readonly state: "abierta" | "en_revision" | "resuelta" | "cerrada";
  readonly reason: string;
  readonly created_at: string;
  readonly closed_at?: string | null;
  readonly close_reason?: string | null;
}

export interface AdminReviewReport {
  readonly report_id: string;
  readonly review_id: string;
  readonly reporter_id: string;
  readonly state: "pendiente" | "desestimado" | "oculto";
  readonly reason: string;
  readonly created_at: string;
  readonly action_reason?: string | null;
}

export interface AdminLocalNotice {
  readonly notice_id: string;
  readonly event_type: string;
  readonly recipient_email: string;
  readonly terminal_status: string;
  readonly created_at: string;
  readonly correlation_id?: string;
  readonly reopen_count?: number;
}

export interface AdminLocalAccount {
  readonly account_id: string;
  readonly email: string;
  readonly display_name?: string | null;
  readonly state: string;
  readonly is_blocked: boolean;
  readonly blocked_reason?: string | null;
  readonly created_at: string;
  readonly updated_at?: string | null;
}

export interface AdminPublishedSpace {
  readonly space_id: string;
  readonly host_id: string;
  readonly title: string;
  readonly category_code: string;
  readonly state: string;
  readonly base_price_clp: number | string;
  readonly area_m2: number;
  readonly capacity: number;
  readonly address?: string;
  readonly created_at: string;
  readonly updated_at: string;
}

export interface AdminReservationsReport {
  readonly generated_at: string;
  readonly from: string;
  readonly until: string;
  readonly timezone: string;
  readonly total_reservations: number;
  readonly by_state: Readonly<Record<string, number>>;
  readonly test_notice: string;
}

export interface AdminFinanceReport {
  readonly generated_at: string;
  readonly from: string;
  readonly until: string;
  readonly timezone: string;
  readonly confirmed_movements_clp: number | string;
  readonly pending_obligations_clp: number | string;
  readonly fee_volume_clp: number | string;
  readonly currency: "CLP";
  readonly test_notice: string;
}

export interface AdminDisputesReport {
  readonly generated_at: string;
  readonly from: string;
  readonly until: string;
  readonly timezone: string;
  readonly total_disputes: number;
  readonly open_disputes: number;
  readonly closed_disputes: number;
  readonly test_notice: string;
}

export interface AdminReservationSummary {
  readonly reservation_id: string;
  readonly quote_id: string;
  readonly space_id: string;
  readonly host_id: string;
  readonly renter_id: string;
  readonly state: string;
  readonly created_at: string;
  readonly start_at: string;
  readonly end_at: string;
  readonly total_clp: number | string;
}

export interface AdminReservationDetail extends AdminReservationSummary {
  readonly history: readonly {
    readonly state: string;
    readonly at: string;
    readonly reason?: string;
  }[];
}

export interface AdminDamageClaim {
  readonly claim_id: string;
  readonly reservation_id: string;
  readonly host_id: string;
  readonly renter_id: string;
  readonly state: "abierto" | "en_evaluacion" | "resuelto" | "rechazado";
  readonly description: string;
  readonly created_at: string;
  readonly evidence_ids?: readonly string[];
  readonly resolution?: {
    readonly decision: string;
    readonly reason: string;
    readonly at: string;
  };
}

export interface AdminDamageClaimResolutionInput {
  readonly decision: "aceptado" | "rechazado" | "parcial";
  readonly resolution_notes: string;
  readonly approved_amount_clp?: number;
}
