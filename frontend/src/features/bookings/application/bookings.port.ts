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
    input: { space_id: string; start_at: string; end_at: string },
    signal?: AbortSignal,
  ): Promise<BookingQuote>;

  getFixture(signal?: AbortSignal): Promise<unknown>;

  listReservations(signal?: AbortSignal): Promise<BookingReservation[]>;

  getReservation(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<BookingDetail | null>;

  createReservation(
    idempotencyKey: string,
    quoteId: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  decideReservation(
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  previewCancellation(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<CancellationPreview>;

  cancelReservation(
    idempotencyKey: string,
    reservationId: string,
    reason?: string,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  simulatePayment(
    idempotencyKey: string,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  simulateRefund(
    idempotencyKey: string,
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    signal?: AbortSignal,
  ): Promise<BookingReservation>;

  createOrGetContract(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract>;

  getContract(
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract | null>;

  signContract(
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract>;

  rejectContract(
    contractId: string,
    signal?: AbortSignal,
  ): Promise<SyntheticContract>;

  downloadContractPdf(
    contractId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  recordCheckIn(
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
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<RentalOperation[]>;

  downloadEvidence(
    reservationId: string,
    evidenceId: string,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  listMessages(
    reservationId: string,
    query?: { before?: number | string; limit?: number },
    signal?: AbortSignal,
  ): Promise<ReservationMessage[]>;

  sendMessage(
    idempotencyKey: string,
    reservationId: string,
    body: string,
    signal?: AbortSignal,
  ): Promise<ReservationMessage>;

  advanceReadCursor(
    reservationId: string,
    throughSequence: number | string,
    signal?: AbortSignal,
  ): Promise<{ sequence: number }>;

  listReviews(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<ReservationReview[]>;

  createReview(
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
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<TrialDispute[]>;

  openDispute(
    idempotencyKey: string,
    reservationId: string,
    reasonCode: string,
    signal?: AbortSignal,
  ): Promise<TrialDispute>;

  getDisputeHistory(
    disputeId: string,
    signal?: AbortSignal,
  ): Promise<TrialDisputeTransition[]>;

  getDamageClaim(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim | null>;

  openDamageClaim(
    idempotencyKey: string,
    reservationId: string,
    description: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim>;

  submitDamageClaimDefense(
    idempotencyKey: string,
    reservationId: string,
    description: string,
    signal?: AbortSignal,
  ): Promise<DamageClaim>;

  createDamageClaimDefenseEvidence(
    idempotencyKey: string,
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<RentalOperation>;

  getGuarantee(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<Guarantee | null>;

  executeGuaranteeOperation(
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
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<Settlement | null>;

  requestSettlement(
    idempotencyKey: string,
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    signal?: AbortSignal,
  ): Promise<Settlement>;

  reconcileSettlement(
    reservationId: string,
    outcome: "exito" | "rechazo",
    signal?: AbortSignal,
  ): Promise<Settlement>;

  getSettlementReceipt(
    reservationId: string,
    signal?: AbortSignal,
  ): Promise<SettlementReceipt | null>;

  getBusinessClock(signal?: AbortSignal): Promise<BusinessClock>;
}
