export type BookingState =
  | "pendiente_de_pago"
  | "pagada"
  | "aprobada_host"
  | "firma_parcial"
  | "lista_para_checkin"
  | "en_curso"
  | "finalizada"
  | "cerrada"
  | "en_disputa"
  | "cancelada_por_firma"
  | "cancelada_por_pago"
  | "rechazada_arrendador"
  | "vencida_pago"
  | "vencida_host"
  | "cancelada_arrendatario";

export type RateUnit = "hora" | "dia" | "mes";

export interface BookingQuote {
  readonly id: string;
  readonly space_id: string;
  readonly rate_version: number;
  readonly rate_unit: RateUnit;
  readonly unit_price_clp: number;
  readonly currency: "CLP";
  readonly units: number;
  readonly subtotal_clp: number;
  readonly start_at: string;
  readonly end_at: string;
  readonly time_zone: string;
  readonly conditions: string;
  readonly category_code: string;
  readonly profile_version: number;
  readonly profile_values: Readonly<Record<string, unknown>>;
  readonly cancellation_policy_version: string;
  readonly guarantee_policy_version: string;
  readonly guarantee_currency: "CLP";
  readonly guarantee_expected_clp: number;
  readonly settlement_policy_version: string;
  readonly settlement_commission_bps: number;
  readonly created_at: string;
  readonly expires_at: string;
}

export interface BookingTransition {
  readonly sequence: number;
  readonly from?: string;
  readonly to: string;
  readonly actor_id?: string;
  readonly reason: string;
  readonly at: string;
}

export interface BookingReservation {
  readonly id: string;
  readonly quote_id: string;
  readonly space_id: string;
  readonly host_id: string;
  readonly renter_id: string;
  readonly state: BookingState;
  readonly rate_unit: RateUnit;
  readonly unit_price_clp: number;
  readonly currency: "CLP";
  readonly units: number;
  readonly subtotal_clp: number;
  readonly start_at: string;
  readonly end_at: string;
  readonly time_zone: string;
  readonly conditions: string;
  readonly cancellation_policy_version: string;
  readonly guarantee_policy_version?: string | null;
  readonly guarantee_currency?: "CLP" | null;
  readonly guarantee_expected_clp?: number | null;
  readonly settlement_policy_version?: string | null;
  readonly settlement_commission_bps?: number | null;
  readonly refund_id?: string;
  readonly refund_operation_id?: string;
  readonly refund_amount_clp?: number;
  readonly refund_state?: "pendiente" | "completada";
  readonly refund_last_result?:
    "exito_simulado" | "fallo_simulado" | "sin_respuesta_simulada";
  readonly refund_updated_at?: string;
  readonly pay_expires_at: string;
  readonly host_expires_at?: string | null;
  readonly created_at: string;
  readonly updated_at: string;
  readonly unread_count: number;
}

export interface BookingDetail extends BookingReservation {
  readonly history: readonly BookingTransition[];
}

export interface CancellationPreview {
  readonly reservation_id: string;
  readonly policy_version: string;
  readonly eligible: boolean;
  readonly refund_ratio: number;
  readonly refund_amount_clp: number;
  readonly penalty_amount_clp: number;
  readonly reason: string;
}

export type SimulatedPaymentOutcome = "exito" | "rechazo" | "sin_respuesta";

export type ContractState =
  "generado" | "firma_parcial" | "firmado" | "anulado";

export type ContractSignatureRole = "anfitrion" | "arrendatario";
export type ContractSignatureState = "pendiente" | "firmada" | "rechazada";

export interface ContractSignature {
  readonly signer_id: string;
  readonly role: ContractSignatureRole;
  readonly state: ContractSignatureState;
  readonly reason?: string;
  readonly updated_at: string;
}

export interface ContractTransition {
  readonly sequence: number;
  readonly actor_id?: string;
  readonly action: string;
  readonly reason?: string;
  readonly occurred_at: string;
}

export interface SyntheticContract {
  readonly id: string;
  readonly reservation_id: string;
  readonly version: number;
  readonly state: ContractState;
  readonly snapshot: string;
  readonly sha256: string;
  readonly signatures: readonly ContractSignature[];
  readonly history: readonly ContractTransition[];
}

export type RentalOperationKind =
  "checkin" | "checkout" | "recepcion" | "descargo";

export type RentalOperationResult =
  "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";

export interface RentalOperationEvidence {
  readonly id: string;
  readonly content_url?: string;
  readonly fixture_code: string;
  readonly mime_type: string;
  readonly sha256: string;
  readonly size_bytes: number;
  readonly created_at: string;
}

export interface RentalOperation {
  readonly id: string;
  readonly reservation_id: string;
  readonly kind: RentalOperationKind;
  readonly actor_id: string;
  readonly occurred_at: string;
  readonly time_zone: string;
  readonly comments?: string;
  readonly observations?: string;
  readonly result: RentalOperationResult;
  readonly claim_deadline_at?: string;
  readonly sequence: number;
  readonly reused?: boolean;
  readonly evidence: readonly RentalOperationEvidence[];
}

export interface ReservationMessage {
  readonly id: string;
  readonly reservation_id: string;
  readonly author_id: string;
  readonly sequence: number;
  readonly body: string;
  readonly created_at: string;
}

export interface ReservationReview {
  readonly id: string;
  readonly target_type?: "espacio" | "arrendatario";
  readonly rating: number;
  readonly state?: "publicada" | "reportada";
  readonly comment?: string;
  readonly created_at: string;
  readonly reused?: boolean;
}

export interface ReviewReport {
  readonly id: string;
  readonly review_id: string;
  readonly space_id: string;
  readonly review_comment?: string;
  readonly reason_code:
    "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
  readonly state: "pendiente" | "desestimada" | "oculta";
  readonly created_at: string;
  readonly resolved_at?: string | null;
  readonly resolution_reason_code?: string;
}

export interface TrialDispute {
  readonly id: string;
  readonly reservation_id: string;
  readonly host_id: string;
  readonly renter_id: string;
  readonly opened_by: string;
  readonly opening_reason_code: string;
  readonly state: "abierta" | "cerrada";
  readonly opened_at: string;
  readonly closed_by?: string;
  readonly close_reason_code?: string;
  readonly closed_at?: string;
  readonly reused: boolean;
}

export interface TrialDisputeTransition {
  readonly sequence: number;
  readonly previous_state?: "abierta" | "cerrada";
  readonly new_state: "abierta" | "cerrada";
  readonly actor_id: string;
  readonly reason_code: string;
  readonly occurred_at: string;
}

export interface DamageClaimEvidence {
  readonly id: string;
  readonly operation: "checkin" | "checkout" | "recepcion" | "descargo";
  readonly fixture_code: string;
  readonly mime_type: string;
  readonly sha256: string;
  readonly size_bytes: number;
  readonly created_at: string;
  readonly content_url?: string;
}

export interface DamageClaimDefense {
  readonly comments?: string;
  readonly evidence: readonly DamageClaimEvidence[];
}

export interface DamageClaimResolution {
  readonly outcome: "acogido" | "rechazado";
  readonly reason_code: string;
  readonly actor_id: string | null;
  readonly resolved_at: string;
  readonly reused?: boolean;
}

export interface DamageClaimTransition {
  readonly sequence: number;
  readonly action:
    "reclamo_abierto" | "descargo_registrado" | "reclamo_resuelto";
  readonly actor_id: string | null;
  readonly occurred_at: string;
}

export interface DamageClaim {
  readonly id: string;
  readonly reservation_id: string | null;
  readonly description: string;
  readonly defense?: DamageClaimDefense;
  readonly resolution?: DamageClaimResolution;
  readonly evidence: readonly DamageClaimEvidence[];
  readonly history: readonly DamageClaimTransition[];
}

export interface GuaranteeOperation {
  readonly id: string;
  readonly kind: "autorizacion" | "captura" | "liberacion";
  readonly amount_clp: number;
  readonly status: string;
  readonly created_at: string;
}

export interface Guarantee {
  readonly policy_version: string;
  readonly currency: "CLP";
  readonly expected_clp: number;
  readonly authorized_clp: number;
  readonly captured_clp: number;
  readonly released_clp: number;
  readonly state: string;
  readonly authorization_deadline?: string;
  readonly operations: readonly GuaranteeOperation[];
}

export interface Settlement {
  readonly id: string;
  readonly reservation_id: string;
  readonly policy_version: string;
  readonly currency: "CLP";
  readonly gross_rent_clp: number;
  readonly confirmed_refunds_clp: number;
  readonly commission_bps: number;
  readonly commission_clp: number;
  readonly transfer_clp: number;
  readonly state: "pendiente" | "por_conciliar" | "liquidada" | "rechazada";
  readonly operation_id: string;
  readonly last_result?: string;
  readonly created_at: string;
}

export interface SettlementReceipt {
  readonly receipt_id: string;
  readonly reservation_id: string;
  readonly policy_version: string;
  readonly currency: "CLP";
  readonly gross_rent_clp: number;
  readonly confirmed_refunds_clp: number;
  readonly commission_clp: number;
  readonly transfer_clp: number;
  readonly confirmed_at: string;
  readonly banner: string;
}

export interface BusinessClock {
  readonly current_time: string;
  readonly time_zone: string;
}
