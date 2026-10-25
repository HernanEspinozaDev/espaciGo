"use client";

import { useCallback, useSyncExternalStore } from "react";
import { bookingsFacade } from "../bookings-runtime";
import type { BookingsSnapshot } from "../application/bookings-facade";
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

export interface BookingsView {
  snapshot: BookingsSnapshot;
  quote: BookingQuote | null;
  reservations: readonly BookingReservation[];
  currentReservation: BookingDetail | null;
  contract: SyntheticContract | null;
  operations: readonly RentalOperation[];
  messages: readonly ReservationMessage[];
  reviews: readonly ReservationReview[];
  disputes: readonly TrialDispute[];
  currentDisputeHistory: readonly TrialDisputeTransition[];
  damageClaim: DamageClaim | null;
  guarantee: Guarantee | null;
  settlement: Settlement | null;
  settlementReceipt: SettlementReceipt | null;
  cancellationPreview: CancellationPreview | null;
  businessClock: BusinessClock | null;
  loading: boolean;
  actionLoading: boolean;
  error: string | null;

  clearError(): void;
  clearQuote(): void;
  clearCurrentReservation(): void;
  loadReservations(token: AccessToken): Promise<readonly BookingReservation[]>;
  loadReservationDetail(
    token: AccessToken,
    reservationId: string,
  ): Promise<BookingDetail | null>;
  selectReservation(
    token: AccessToken,
    reservationId: string,
  ): Promise<BookingDetail | null>;
  requestQuote(
    token: AccessToken,
    input: { space_id: string; start_at: string; end_at: string },
  ): Promise<BookingQuote>;
  createReservation(
    token: AccessToken,
    quoteId: string,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  decideReservation(
    token: AccessToken,
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<BookingReservation>;
  acceptReservation(
    token: AccessToken,
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation>;
  rejectReservation(
    token: AccessToken,
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation>;
  requestCancellationPreview(
    token: AccessToken,
    reservationId: string,
  ): Promise<CancellationPreview>;
  previewCancellation(
    token: AccessToken,
    reservationId: string,
  ): Promise<CancellationPreview>;
  cancelReservation(
    token: AccessToken,
    reservationId: string,
    reason?: string,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  submitSimulatedPayment(
    token: AccessToken,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  simulatePayment(
    token: AccessToken,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  simulateRefund(
    token: AccessToken,
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  loadContract(
    token: AccessToken,
    reservationId: string,
  ): Promise<SyntheticContract>;
  signContract(
    token: AccessToken,
    contractId: string,
  ): Promise<SyntheticContract>;
  rejectContract(
    token: AccessToken,
    contractId: string,
  ): Promise<SyntheticContract>;
  downloadContractPdf(
    token: AccessToken,
    contractId: string,
  ): Promise<Blob | null>;
  loadOperations(
    token: AccessToken,
    reservationId: string,
  ): Promise<readonly RentalOperation[]>;
  performCheckIn(
    token: AccessToken,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    idempotencyKey?: string,
  ): Promise<RentalOperation>;
  performCheckOut(
    token: AccessToken,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    idempotencyKey?: string,
  ): Promise<RentalOperation>;
  performReception(
    token: AccessToken,
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    idempotencyKey?: string,
  ): Promise<RentalOperation>;
  loadMessages(
    token: AccessToken,
    reservationId: string,
    query?: { before?: number | string; limit?: number },
  ): Promise<readonly ReservationMessage[]>;
  sendMessage(
    token: AccessToken,
    reservationId: string,
    body: string,
    idempotencyKey?: string,
  ): Promise<ReservationMessage>;
  markMessagesRead(
    token: AccessToken,
    reservationId: string,
    throughSequence: number | string,
  ): Promise<{ sequence: number }>;
  loadReviews(
    token: AccessToken,
    reservationId: string,
  ): Promise<readonly ReservationReview[]>;
  submitReview(
    token: AccessToken,
    reservationId: string,
    input: {
      rating: number;
      comment?: string;
      target_type?: "espacio" | "arrendatario";
    },
    idempotencyKey?: string,
  ): Promise<ReservationReview>;
  reportReview(
    token: AccessToken,
    reservationId: string,
    reviewId: string,
    input: {
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      comment?: string;
    },
    idempotencyKey?: string,
  ): Promise<ReviewReport>;
  loadDisputes(
    token: AccessToken,
    reservationId: string,
  ): Promise<readonly TrialDispute[]>;
  openDispute(
    token: AccessToken,
    reservationId: string,
    reasonCode?: string,
    idempotencyKey?: string,
  ): Promise<TrialDispute>;
  getDisputeHistory(): readonly TrialDisputeTransition[];
  loadDamageClaim(
    token: AccessToken,
    reservationId: string,
  ): Promise<DamageClaim | null>;
  claimSecurityDeposit(
    token: AccessToken,
    reservationId: string,
    description: string,
    idempotencyKey?: string,
  ): Promise<DamageClaim>;
  submitDamageClaimDefense(
    token: AccessToken,
    reservationId: string,
    description: string,
    idempotencyKey?: string,
  ): Promise<DamageClaim>;
  loadGuarantee(
    token: AccessToken,
    reservationId: string,
  ): Promise<Guarantee | null>;
  reconcileGuarantee(
    token: AccessToken,
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    idempotencyKey?: string,
  ): Promise<Guarantee>;
  loadSettlement(
    token: AccessToken,
    reservationId: string,
  ): Promise<Settlement | null>;
  submitFinancialDecision(
    token: AccessToken,
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    idempotencyKey?: string,
  ): Promise<Settlement>;
  loadReceipt(
    token: AccessToken,
    reservationId: string,
  ): Promise<SettlementReceipt | null>;
  reconcileSettlement(
    token: AccessToken,
    reservationId: string,
    outcome: "exito" | "rechazo",
  ): Promise<Settlement>;
  getBusinessClock(token: AccessToken): Promise<BusinessClock>;
  advanceBusinessClock(token: AccessToken): Promise<BusinessClock>;
  fetchFixture(token: AccessToken): Promise<unknown>;
}

export function useBookings(): BookingsView {
  const snapshot = useSyncExternalStore(
    bookingsFacade.subscribe,
    bookingsFacade.getSnapshot,
    bookingsFacade.getServerSnapshot,
  );

  const clearError = useCallback(() => bookingsFacade.clearError(), []);
  const clearQuote = useCallback(() => bookingsFacade.clearQuote(), []);
  const clearCurrentReservation = useCallback(
    () => bookingsFacade.clearCurrentReservation(),
    [],
  );
  const loadReservations = useCallback(
    (token: AccessToken) => bookingsFacade.loadReservations(token),
    [],
  );
  const loadReservationDetail = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadReservationDetail(token, reservationId),
    [],
  );
  const selectReservation = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.selectReservation(token, reservationId),
    [],
  );
  const requestQuote = useCallback(
    (
      token: AccessToken,
      input: { space_id: string; start_at: string; end_at: string },
    ) => bookingsFacade.requestQuote(token, input),
    [],
  );
  const createReservation = useCallback(
    (token: AccessToken, quoteId: string, idempotencyKey?: string) =>
      bookingsFacade.createReservation(token, quoteId, idempotencyKey),
    [],
  );
  const decideReservation = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      decision: "aprobar" | "rechazar",
      reason?: string,
    ) =>
      bookingsFacade.decideReservation(token, reservationId, decision, reason),
    [],
  );
  const acceptReservation = useCallback(
    (token: AccessToken, reservationId: string, reason?: string) =>
      bookingsFacade.acceptReservation(token, reservationId, reason),
    [],
  );
  const rejectReservation = useCallback(
    (token: AccessToken, reservationId: string, reason?: string) =>
      bookingsFacade.rejectReservation(token, reservationId, reason),
    [],
  );
  const requestCancellationPreview = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.requestCancellationPreview(token, reservationId),
    [],
  );
  const previewCancellation = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.previewCancellation(token, reservationId),
    [],
  );
  const cancelReservation = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      reason?: string,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.cancelReservation(
        token,
        reservationId,
        reason,
        idempotencyKey,
      ),
    [],
  );
  const submitSimulatedPayment = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      outcome: SimulatedPaymentOutcome,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.submitSimulatedPayment(
        token,
        reservationId,
        outcome,
        idempotencyKey,
      ),
    [],
  );
  const simulatePayment = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      outcome: SimulatedPaymentOutcome,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.simulatePayment(
        token,
        reservationId,
        outcome,
        idempotencyKey,
      ),
    [],
  );
  const simulateRefund = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      outcome: "exito" | "fallo" | "sin_respuesta",
      idempotencyKey?: string,
    ) =>
      bookingsFacade.simulateRefund(
        token,
        reservationId,
        outcome,
        idempotencyKey,
      ),
    [],
  );
  const loadContract = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadContract(token, reservationId),
    [],
  );
  const signContract = useCallback(
    (token: AccessToken, contractId: string) =>
      bookingsFacade.signContract(token, contractId),
    [],
  );
  const rejectContract = useCallback(
    (token: AccessToken, contractId: string) =>
      bookingsFacade.rejectContract(token, contractId),
    [],
  );
  const downloadContractPdf = useCallback(
    (token: AccessToken, contractId: string) =>
      bookingsFacade.downloadContractPdf(token, contractId),
    [],
  );
  const loadOperations = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadOperations(token, reservationId),
    [],
  );
  const performCheckIn = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      input: {
        occurred_at?: string;
        time_zone?: string;
        comments?: string;
        fixture_evidence_code?:
          "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
      },
      idempotencyKey?: string,
    ) =>
      bookingsFacade.performCheckIn(
        token,
        reservationId,
        input,
        idempotencyKey,
      ),
    [],
  );
  const performCheckOut = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      input: {
        occurred_at?: string;
        time_zone?: string;
        comments?: string;
        fixture_evidence_code?:
          "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
      },
      idempotencyKey?: string,
    ) =>
      bookingsFacade.performCheckOut(
        token,
        reservationId,
        input,
        idempotencyKey,
      ),
    [],
  );
  const performReception = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      input: {
        result?:
          "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
        observations?: string;
      },
      idempotencyKey?: string,
    ) =>
      bookingsFacade.performReception(
        token,
        reservationId,
        input,
        idempotencyKey,
      ),
    [],
  );
  const loadMessages = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      query?: { before?: number | string; limit?: number },
    ) => bookingsFacade.loadMessages(token, reservationId, query),
    [],
  );
  const sendMessage = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      body: string,
      idempotencyKey?: string,
    ) => bookingsFacade.sendMessage(token, reservationId, body, idempotencyKey),
    [],
  );
  const markMessagesRead = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      throughSequence: number | string,
    ) => bookingsFacade.markMessagesRead(token, reservationId, throughSequence),
    [],
  );
  const loadReviews = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadReviews(token, reservationId),
    [],
  );
  const submitReview = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      input: {
        rating: number;
        comment?: string;
        target_type?: "espacio" | "arrendatario";
      },
      idempotencyKey?: string,
    ) =>
      bookingsFacade.submitReview(token, reservationId, input, idempotencyKey),
    [],
  );
  const reportReview = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      reviewId: string,
      input: {
        reason_code:
          "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
        comment?: string;
      },
      idempotencyKey?: string,
    ) =>
      bookingsFacade.reportReview(
        token,
        reservationId,
        reviewId,
        input,
        idempotencyKey,
      ),
    [],
  );
  const loadDisputes = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadDisputes(token, reservationId),
    [],
  );
  const openDispute = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      reasonCode?: string,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.openDispute(
        token,
        reservationId,
        reasonCode,
        idempotencyKey,
      ),
    [],
  );
  const getDisputeHistory = useCallback(
    () => bookingsFacade.getDisputeHistory(),
    [],
  );
  const loadDamageClaim = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadDamageClaim(token, reservationId),
    [],
  );
  const claimSecurityDeposit = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      description: string,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.claimSecurityDeposit(
        token,
        reservationId,
        description,
        idempotencyKey,
      ),
    [],
  );
  const submitDamageClaimDefense = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      description: string,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.submitDamageClaimDefense(
        token,
        reservationId,
        description,
        idempotencyKey,
      ),
    [],
  );
  const loadGuarantee = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadGuarantee(token, reservationId),
    [],
  );
  const reconcileGuarantee = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      input: {
        kind: "autorizacion" | "captura" | "liberacion";
        amount_clp: number;
        outcome?: SimulatedPaymentOutcome;
      },
      idempotencyKey?: string,
    ) =>
      bookingsFacade.reconcileGuarantee(
        token,
        reservationId,
        input,
        idempotencyKey,
      ),
    [],
  );
  const loadSettlement = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadSettlement(token, reservationId),
    [],
  );
  const submitFinancialDecision = useCallback(
    (
      token: AccessToken,
      reservationId: string,
      outcome?: SimulatedPaymentOutcome,
      idempotencyKey?: string,
    ) =>
      bookingsFacade.submitFinancialDecision(
        token,
        reservationId,
        outcome,
        idempotencyKey,
      ),
    [],
  );
  const loadReceipt = useCallback(
    (token: AccessToken, reservationId: string) =>
      bookingsFacade.loadReceipt(token, reservationId),
    [],
  );
  const reconcileSettlement = useCallback(
    (token: AccessToken, reservationId: string, outcome: "exito" | "rechazo") =>
      bookingsFacade.reconcileSettlement(token, reservationId, outcome),
    [],
  );
  const getBusinessClock = useCallback(
    (token: AccessToken) => bookingsFacade.getBusinessClock(token),
    [],
  );
  const advanceBusinessClock = useCallback(
    (token: AccessToken) => bookingsFacade.advanceBusinessClock(token),
    [],
  );
  const fetchFixture = useCallback(
    (token: AccessToken) => bookingsFacade.fetchFixture(token),
    [],
  );

  return {
    snapshot,
    quote: snapshot.quote,
    reservations: snapshot.reservations,
    currentReservation: snapshot.currentReservation,
    contract: snapshot.contract,
    operations: snapshot.operations,
    messages: snapshot.messages,
    reviews: snapshot.reviews,
    disputes: snapshot.disputes,
    currentDisputeHistory: snapshot.currentDisputeHistory,
    damageClaim: snapshot.damageClaim,
    guarantee: snapshot.guarantee,
    settlement: snapshot.settlement,
    settlementReceipt: snapshot.settlementReceipt,
    cancellationPreview: snapshot.cancellationPreview,
    businessClock: snapshot.businessClock,
    loading: snapshot.loading,
    actionLoading: snapshot.actionLoading,
    error: snapshot.error,

    clearError,
    clearQuote,
    clearCurrentReservation,
    loadReservations,
    loadReservationDetail,
    selectReservation,
    requestQuote,
    createReservation,
    decideReservation,
    acceptReservation,
    rejectReservation,
    requestCancellationPreview,
    previewCancellation,
    cancelReservation,
    submitSimulatedPayment,
    simulatePayment,
    simulateRefund,
    loadContract,
    signContract,
    rejectContract,
    downloadContractPdf,
    loadOperations,
    performCheckIn,
    performCheckOut,
    performReception,
    loadMessages,
    sendMessage,
    markMessagesRead,
    loadReviews,
    submitReview,
    reportReview,
    loadDisputes,
    openDispute,
    getDisputeHistory,
    loadDamageClaim,
    claimSecurityDeposit,
    submitDamageClaimDefense,
    loadGuarantee,
    reconcileGuarantee,
    loadSettlement,
    submitFinancialDecision,
    loadReceipt,
    reconcileSettlement,
    getBusinessClock,
    advanceBusinessClock,
    fetchFixture,
  };
}
