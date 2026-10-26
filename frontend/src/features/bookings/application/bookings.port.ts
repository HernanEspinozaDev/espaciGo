import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  BookingDetail,
  BookingQuote,
  BookingReservation,
  BusinessClock,
  CancellationPreview,
  DamageClaim,
  Guarantee,
  RentalOperation,
  ReservationMessage,
  ReservationReview,
  ReviewReport,
  Settlement,
  SettlementReceipt,
  SimulatedPaymentOutcome,
  SyntheticContract,
  TrialDispute,
  TrialDisputeTransition,
} from "../domain/types";

export interface BookingsPort {
  quote(
    token: AccessToken,
    input: { space_id: string; start_at: string; end_at: string },
    signal?: AbortSignal,
  ): Promise<BookingQuote>;

  getFixture(token: AccessToken, signal?: AbortSignal): Promise<unknown>;

  listReservations(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<BookingReservation[]>;

  getReservation(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<BookingDetail | null>;

  createReservation(
    token: AccessToken,
    idempotencyKey: string,
    quoteId: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  decideReservation(
    token: AccessToken,
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  previewCancellation(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<CancellationPreview>;

  cancelReservation(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    reason?: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  simulatePayment(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  simulateRefund(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  createOrGetContract(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract>;

  getContract(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract | null>;

  signContract(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract>;

  rejectContract(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract>;

  downloadContractPdf(
    token: AccessToken,
    contractId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  recordCheckIn(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    signal?: AbortSignal,
  ): Promise<RentalOperation>;

  recordCheckOut(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    signal?: AbortSignal,
  ): Promise<RentalOperation>;

  recordReception(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    signal?: AbortSignal,
  ): Promise<RentalOperation>;

  listOperations(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<RentalOperation[]>;

  downloadEvidence(
    token: AccessToken,
    reservationId: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  listMessages(
    token: AccessToken,
    reservationId: string,
    query?: { before?: number | string; limit?: number },
    signal?: AbortSignal,
  ): Promise<ReservationMessage[]>;

  sendMessage(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    body: string,
    signal?: AbortSignal,
  ): Promise<ReservationMessage>;

  advanceReadCursor(
    token: AccessToken,
    reservationId: string,
    throughSequence: number | string,
    signal?: AbortSignal,
  ): Promise<{ sequence: number }>;

  listReviews(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<ReservationReview[]>;

  createReview(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      rating: number;
      comment?: string;
      target_type?: "espacio" | "arrendatario";
    },
    signal?: AbortSignal,
  ): Promise<ReservationReview>;

  reportReview(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    reviewId: string,
    input: {
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      comment?: string;
    },
    signal?: AbortSignal,
  ): Promise<ReviewReport>;

  listDisputes(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<TrialDispute[]>;

  openDispute(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    reasonCode: string,
    signal?: AbortSignal,
  ): Promise<TrialDispute>;

  getDisputeHistory(
    token: AccessToken,
    disputeId: string,
    signal?: AbortSignal,
  ): Promise<TrialDisputeTransition[]>;

  getDamageClaim(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim | null>;

  openDamageClaim(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    description: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim>;

  submitDamageClaimDefense(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    description: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim>;

  createDamageClaimDefenseEvidence(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<RentalOperation>;

  getGuarantee(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<Guarantee | null>;

  executeGuaranteeOperation(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    signal?: AbortSignal,
  ): Promise<Guarantee>;

  getSettlement(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<Settlement | null>;

  requestSettlement(
    token: AccessToken,
    idempotencyKey: string,
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    signal?: AbortSignal,
  ): Promise<Settlement>;

  reconcileSettlement(
    token: AccessToken,
    reservationId: string,
    outcome: "exito" | "rechazo",
    signal?: AbortSignal,
  ): Promise<Settlement>;

  getSettlementReceipt(
    token: AccessToken,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<SettlementReceipt | null>;

  getBusinessClock(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<BusinessClock>;
}
