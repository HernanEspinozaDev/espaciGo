"use client";

import { useCallback, useSyncExternalStore } from "react";
import { authFacade } from "@/features/auth/auth-runtime";
import { bookingsFacade } from "../bookings-runtime";
import type { BookingsSnapshot } from "../application/bookings-facade";
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

function withConfirmedAuth<T>(operation: () => Promise<T>): Promise<T> {
  const snapshot = authFacade.getSnapshot();
  if (snapshot.status !== "authenticated") {
    return Promise.reject(new Error("Sesión no autenticada"));
  }
  return operation();
}

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
  loadReservations(): Promise<readonly BookingReservation[]>;
  loadReservationDetail(reservationId: string): Promise<BookingDetail | null>;
  selectReservation(reservationId: string): Promise<BookingDetail | null>;
  requestQuote(input: {
    space_id: string;
    start_at: string;
    end_at: string;
  }): Promise<BookingQuote>;
  createReservation(
    quoteId: string,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  decideReservation(
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<BookingReservation>;
  acceptReservation(
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation>;
  rejectReservation(
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation>;
  requestCancellationPreview(
    reservationId: string,
  ): Promise<CancellationPreview>;
  previewCancellation(reservationId: string): Promise<CancellationPreview>;
  cancelReservation(
    reservationId: string,
    reason?: string,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  submitSimulatedPayment(
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  simulatePayment(
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  simulateRefund(
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    idempotencyKey?: string,
  ): Promise<BookingReservation>;
  loadContract(reservationId: string): Promise<SyntheticContract>;
  signContract(contractId: string): Promise<SyntheticContract>;
  rejectContract(contractId: string): Promise<SyntheticContract>;
  downloadContractPdf(contractId: string): Promise<Blob | null>;
  loadOperations(reservationId: string): Promise<readonly RentalOperation[]>;
  performCheckIn(
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
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    idempotencyKey?: string,
  ): Promise<RentalOperation>;
  loadMessages(
    reservationId: string,
    query?: { before?: number | string; limit?: number },
  ): Promise<readonly ReservationMessage[]>;
  sendMessage(
    reservationId: string,
    body: string,
    idempotencyKey?: string,
  ): Promise<ReservationMessage>;
  markMessagesRead(
    reservationId: string,
    throughSequence: number | string,
  ): Promise<{ sequence: number }>;
  loadReviews(reservationId: string): Promise<readonly ReservationReview[]>;
  submitReview(
    reservationId: string,
    input: {
      rating: number;
      comment?: string;
      target_type?: "espacio" | "arrendatario";
    },
    idempotencyKey?: string,
  ): Promise<ReservationReview>;
  reportReview(
    reservationId: string,
    reviewId: string,
    input: {
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      comment?: string;
    },
    idempotencyKey?: string,
  ): Promise<ReviewReport>;
  loadDisputes(reservationId: string): Promise<readonly TrialDispute[]>;
  openDispute(
    reservationId: string,
    reasonCode?: string,
    idempotencyKey?: string,
  ): Promise<TrialDispute>;
  getDisputeHistory(): readonly TrialDisputeTransition[];
  loadDamageClaim(reservationId: string): Promise<DamageClaim | null>;
  claimSecurityDeposit(
    reservationId: string,
    description: string,
    idempotencyKey?: string,
  ): Promise<DamageClaim>;
  submitDamageClaimDefense(
    reservationId: string,
    description: string,
    idempotencyKey?: string,
  ): Promise<DamageClaim>;
  loadGuarantee(reservationId: string): Promise<Guarantee | null>;
  reconcileGuarantee(
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    idempotencyKey?: string,
  ): Promise<Guarantee>;
  loadSettlement(reservationId: string): Promise<Settlement | null>;
  submitFinancialDecision(
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    idempotencyKey?: string,
  ): Promise<Settlement>;
  loadReceipt(reservationId: string): Promise<SettlementReceipt | null>;
  reconcileSettlement(
    reservationId: string,
    outcome: "exito" | "rechazo",
  ): Promise<Settlement>;
  getBusinessClock(): Promise<BusinessClock>;
  advanceBusinessClock(): Promise<BusinessClock>;
  fetchFixture(): Promise<unknown>;
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
    () => withConfirmedAuth(() => bookingsFacade.loadReservations()),
    [],
  );
  const loadReservationDetail = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.loadReservationDetail(reservationId),
      ),
    [],
  );
  const selectReservation = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.selectReservation(reservationId)),
    [],
  );
  const requestQuote = useCallback(
    (input: { space_id: string; start_at: string; end_at: string }) =>
      withConfirmedAuth(() => bookingsFacade.requestQuote(input)),
    [],
  );
  const createReservation = useCallback(
    (quoteId: string, idempotencyKey?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.createReservation(quoteId, idempotencyKey),
      ),
    [],
  );
  const decideReservation = useCallback(
    (
      reservationId: string,
      decision: "aprobar" | "rechazar",
      reason?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.decideReservation(reservationId, decision, reason),
      ),
    [],
  );
  const acceptReservation = useCallback(
    (reservationId: string, reason?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.acceptReservation(reservationId, reason),
      ),
    [],
  );
  const rejectReservation = useCallback(
    (reservationId: string, reason?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.rejectReservation(reservationId, reason),
      ),
    [],
  );
  const requestCancellationPreview = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.requestCancellationPreview(reservationId),
      ),
    [],
  );
  const previewCancellation = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.previewCancellation(reservationId),
      ),
    [],
  );
  const cancelReservation = useCallback(
    (reservationId: string, reason?: string, idempotencyKey?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.cancelReservation(reservationId, reason, idempotencyKey),
      ),
    [],
  );
  const submitSimulatedPayment = useCallback(
    (
      reservationId: string,
      outcome: SimulatedPaymentOutcome,
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.submitSimulatedPayment(
          reservationId,
          outcome,
          idempotencyKey,
        ),
      ),
    [],
  );
  const simulatePayment = useCallback(
    (
      reservationId: string,
      outcome: SimulatedPaymentOutcome,
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.simulatePayment(reservationId, outcome, idempotencyKey),
      ),
    [],
  );
  const simulateRefund = useCallback(
    (
      reservationId: string,
      outcome: "exito" | "fallo" | "sin_respuesta",
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.simulateRefund(reservationId, outcome, idempotencyKey),
      ),
    [],
  );
  const loadContract = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadContract(reservationId)),
    [],
  );
  const signContract = useCallback(
    (contractId: string) =>
      withConfirmedAuth(() => bookingsFacade.signContract(contractId)),
    [],
  );
  const rejectContract = useCallback(
    (contractId: string) =>
      withConfirmedAuth(() => bookingsFacade.rejectContract(contractId)),
    [],
  );
  const downloadContractPdf = useCallback(
    (contractId: string) =>
      withConfirmedAuth(() => bookingsFacade.downloadContractPdf(contractId)),
    [],
  );
  const loadOperations = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadOperations(reservationId)),
    [],
  );
  const performCheckIn = useCallback(
    (
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
      withConfirmedAuth(() =>
        bookingsFacade.performCheckIn(reservationId, input, idempotencyKey),
      ),
    [],
  );
  const performCheckOut = useCallback(
    (
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
      withConfirmedAuth(() =>
        bookingsFacade.performCheckOut(reservationId, input, idempotencyKey),
      ),
    [],
  );
  const performReception = useCallback(
    (
      reservationId: string,
      input: {
        result?:
          "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
        observations?: string;
      },
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.performReception(reservationId, input, idempotencyKey),
      ),
    [],
  );
  const loadMessages = useCallback(
    (
      reservationId: string,
      query?: { before?: number | string; limit?: number },
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.loadMessages(reservationId, query),
      ),
    [],
  );
  const sendMessage = useCallback(
    (reservationId: string, body: string, idempotencyKey?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.sendMessage(reservationId, body, idempotencyKey),
      ),
    [],
  );
  const markMessagesRead = useCallback(
    (reservationId: string, throughSequence: number | string) =>
      withConfirmedAuth(() =>
        bookingsFacade.markMessagesRead(reservationId, throughSequence),
      ),
    [],
  );
  const loadReviews = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadReviews(reservationId)),
    [],
  );
  const submitReview = useCallback(
    (
      reservationId: string,
      input: {
        rating: number;
        comment?: string;
        target_type?: "espacio" | "arrendatario";
      },
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.submitReview(reservationId, input, idempotencyKey),
      ),
    [],
  );
  const reportReview = useCallback(
    (
      reservationId: string,
      reviewId: string,
      input: {
        reason_code:
          "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
        comment?: string;
      },
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.reportReview(
          reservationId,
          reviewId,
          input,
          idempotencyKey,
        ),
      ),
    [],
  );
  const loadDisputes = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadDisputes(reservationId)),
    [],
  );
  const openDispute = useCallback(
    (reservationId: string, reasonCode?: string, idempotencyKey?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.openDispute(reservationId, reasonCode, idempotencyKey),
      ),
    [],
  );
  const getDisputeHistory = useCallback(
    () => bookingsFacade.getDisputeHistory(),
    [],
  );
  const loadDamageClaim = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadDamageClaim(reservationId)),
    [],
  );
  const claimSecurityDeposit = useCallback(
    (reservationId: string, description: string, idempotencyKey?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.claimSecurityDeposit(
          reservationId,
          description,
          idempotencyKey,
        ),
      ),
    [],
  );
  const submitDamageClaimDefense = useCallback(
    (reservationId: string, description: string, idempotencyKey?: string) =>
      withConfirmedAuth(() =>
        bookingsFacade.submitDamageClaimDefense(
          reservationId,
          description,
          idempotencyKey,
        ),
      ),
    [],
  );
  const loadGuarantee = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadGuarantee(reservationId)),
    [],
  );
  const reconcileGuarantee = useCallback(
    (
      reservationId: string,
      input: {
        kind: "autorizacion" | "captura" | "liberacion";
        amount_clp: number;
        outcome?: SimulatedPaymentOutcome;
      },
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.reconcileGuarantee(reservationId, input, idempotencyKey),
      ),
    [],
  );
  const loadSettlement = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadSettlement(reservationId)),
    [],
  );
  const submitFinancialDecision = useCallback(
    (
      reservationId: string,
      outcome?: SimulatedPaymentOutcome,
      idempotencyKey?: string,
    ) =>
      withConfirmedAuth(() =>
        bookingsFacade.submitFinancialDecision(
          reservationId,
          outcome,
          idempotencyKey,
        ),
      ),
    [],
  );
  const loadReceipt = useCallback(
    (reservationId: string) =>
      withConfirmedAuth(() => bookingsFacade.loadReceipt(reservationId)),
    [],
  );
  const reconcileSettlement = useCallback(
    (reservationId: string, outcome: "exito" | "rechazo") =>
      withConfirmedAuth(() =>
        bookingsFacade.reconcileSettlement(reservationId, outcome),
      ),
    [],
  );
  const getBusinessClock = useCallback(
    () => withConfirmedAuth(() => bookingsFacade.getBusinessClock()),
    [],
  );
  const advanceBusinessClock = useCallback(
    () => withConfirmedAuth(() => bookingsFacade.advanceBusinessClock()),
    [],
  );
  const fetchFixture = useCallback(
    () => withConfirmedAuth(() => bookingsFacade.fetchFixture()),
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
