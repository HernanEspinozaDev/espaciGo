"use client";

import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import {
  deduplicateById,
  generateIdempotencyKey,
} from "../domain/booking-rules";
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
import type { BookingsPort } from "./bookings.port";

export interface BookingsSnapshot {
  readonly quote: BookingQuote | null;
  readonly reservations: readonly BookingReservation[];
  readonly currentReservation: BookingDetail | null;
  readonly contract: SyntheticContract | null;
  readonly operations: readonly RentalOperation[];
  readonly messages: readonly ReservationMessage[];
  readonly reviews: readonly ReservationReview[];
  readonly disputes: readonly TrialDispute[];
  readonly currentDisputeHistory: readonly TrialDisputeTransition[];
  readonly damageClaim: DamageClaim | null;
  readonly guarantee: Guarantee | null;
  readonly settlement: Settlement | null;
  readonly settlementReceipt: SettlementReceipt | null;
  readonly cancellationPreview: CancellationPreview | null;
  readonly businessClock: BusinessClock | null;
  readonly loading: boolean;
  readonly actionLoading: boolean;
  readonly error: string | null;
}

const initialSnapshot: BookingsSnapshot = Object.freeze({
  quote: null,
  reservations: Object.freeze([]),
  currentReservation: null,
  contract: null,
  operations: Object.freeze([]),
  messages: Object.freeze([]),
  reviews: Object.freeze([]),
  disputes: Object.freeze([]),
  currentDisputeHistory: Object.freeze([]),
  damageClaim: null,
  guarantee: null,
  settlement: null,
  settlementReceipt: null,
  cancellationPreview: null,
  businessClock: null,
  loading: false,
  actionLoading: false,
  error: null,
});

export class BookingsFacade {
  #port: BookingsPort;
  #abortController: AbortController | null = null;
  #snapshot: BookingsSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();

  constructor(port: BookingsPort) {
    this.#port = port;
  }

  getSnapshot = (): BookingsSnapshot => {
    return this.#snapshot;
  };

  getServerSnapshot = (): BookingsSnapshot => {
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

  #setSnapshot(partial: Partial<BookingsSnapshot>): void {
    this.#snapshot = Object.freeze({
      ...this.#snapshot,
      ...partial,
    });
    for (const listener of this.#listeners) {
      listener();
    }
  }

  clearError(): void {
    if (this.#snapshot.error) {
      this.#setSnapshot({ error: null });
    }
  }

  clearQuote(): void {
    this.#setSnapshot({ quote: null });
  }

  clearCurrentReservation(): void {
    this.#setSnapshot({
      currentReservation: null,
      contract: null,
      operations: Object.freeze([]),
      messages: Object.freeze([]),
      reviews: Object.freeze([]),
      damageClaim: null,
      guarantee: null,
      settlement: null,
      settlementReceipt: null,
      cancellationPreview: null,
    });
  }

  async loadReservations(
    token: AccessToken,
  ): Promise<readonly BookingReservation[]> {
    this.cancel();
    this.#abortController = new AbortController();
    this.#setSnapshot({ loading: true, error: null });

    try {
      const reservations = await this.#port.listReservations(
        token,
        this.#abortController.signal,
      );
      this.#setSnapshot({
        reservations: Object.freeze(reservations),
        loading: false,
      });
      return reservations;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar reservas";
      this.#setSnapshot({ loading: false, error });
      throw err;
    }
  }

  async loadReservationDetail(
    token: AccessToken,
    reservationId: string,
  ): Promise<BookingDetail | null> {
    this.cancel();
    this.#abortController = new AbortController();
    this.#setSnapshot({ loading: true, error: null });

    try {
      const detail = await this.#port.getReservation(
        token,
        reservationId,
        this.#abortController.signal,
      );
      this.#setSnapshot({
        currentReservation: detail,
        loading: false,
      });
      return detail;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al cargar el detalle de la reserva";
      this.#setSnapshot({ loading: false, error });
      throw err;
    }
  }

  async requestQuote(
    token: AccessToken,
    input: { space_id: string; start_at: string; end_at: string },
  ): Promise<BookingQuote> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const quote = await this.#port.quote(token, input);
      this.#setSnapshot({
        quote,
        actionLoading: false,
      });
      return quote;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cotizar reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async createReservation(
    token: AccessToken,
    quoteId: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const reservation = await this.#port.createReservation(
        token,
        idempotencyKey,
        quoteId,
      );
      this.#setSnapshot({
        reservations: Object.freeze([
          reservation,
          ...this.#snapshot.reservations,
        ]),
        actionLoading: false,
      });
      return reservation;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al crear reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async decideReservation(
    token: AccessToken,
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<BookingReservation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.decideReservation(
        token,
        reservationId,
        decision,
        reason,
      );
      const nextReservations = this.#snapshot.reservations.map((r) =>
        r.id === updated.id ? updated : r,
      );
      const nextCurrent =
        this.#snapshot.currentReservation?.id === updated.id
          ? { ...this.#snapshot.currentReservation, ...updated }
          : this.#snapshot.currentReservation;
      this.#setSnapshot({
        reservations: Object.freeze(nextReservations),
        currentReservation: nextCurrent,
        actionLoading: false,
      });
      return updated;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al procesar la decisión de reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async previewCancellation(
    token: AccessToken,
    reservationId: string,
  ): Promise<CancellationPreview> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const preview = await this.#port.previewCancellation(
        token,
        reservationId,
      );
      this.#setSnapshot({
        cancellationPreview: preview,
        actionLoading: false,
      });
      return preview;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al previsualizar cancelación";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async cancelReservation(
    token: AccessToken,
    reservationId: string,
    reason?: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.cancelReservation(
        token,
        idempotencyKey,
        reservationId,
        reason,
      );
      const nextReservations = this.#snapshot.reservations.map((r) =>
        r.id === updated.id ? updated : r,
      );
      const nextCurrent =
        this.#snapshot.currentReservation?.id === updated.id
          ? { ...this.#snapshot.currentReservation, ...updated }
          : this.#snapshot.currentReservation;
      this.#setSnapshot({
        reservations: Object.freeze(nextReservations),
        currentReservation: nextCurrent,
        actionLoading: false,
      });
      return updated;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cancelar reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async simulatePayment(
    token: AccessToken,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.simulatePayment(
        token,
        idempotencyKey,
        reservationId,
        outcome,
      );
      const nextReservations = this.#snapshot.reservations.map((r) =>
        r.id === updated.id ? updated : r,
      );
      const nextCurrent =
        this.#snapshot.currentReservation?.id === updated.id
          ? { ...this.#snapshot.currentReservation, ...updated }
          : this.#snapshot.currentReservation;
      this.#setSnapshot({
        reservations: Object.freeze(nextReservations),
        currentReservation: nextCurrent,
        actionLoading: false,
      });
      return updated;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al procesar pago simulado";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async simulateRefund(
    token: AccessToken,
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.simulateRefund(
        token,
        idempotencyKey,
        reservationId,
        outcome,
      );
      const nextReservations = this.#snapshot.reservations.map((r) =>
        r.id === updated.id ? updated : r,
      );
      const nextCurrent =
        this.#snapshot.currentReservation?.id === updated.id
          ? { ...this.#snapshot.currentReservation, ...updated }
          : this.#snapshot.currentReservation;
      this.#setSnapshot({
        reservations: Object.freeze(nextReservations),
        currentReservation: nextCurrent,
        actionLoading: false,
      });
      return updated;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al procesar devolución simulada";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadContract(
    token: AccessToken,
    reservationId: string,
  ): Promise<SyntheticContract> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const contract = await this.#port.createOrGetContract(
        token,
        reservationId,
      );
      this.#setSnapshot({
        contract,
        actionLoading: false,
      });
      return contract;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar contrato";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async signContract(
    token: AccessToken,
    contractId: string,
  ): Promise<SyntheticContract> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.signContract(token, contractId);
      this.#setSnapshot({
        contract: updated,
        actionLoading: false,
      });
      return updated;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al firmar contrato";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async rejectContract(
    token: AccessToken,
    contractId: string,
  ): Promise<SyntheticContract> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#port.rejectContract(token, contractId);
      this.#setSnapshot({
        contract: updated,
        actionLoading: false,
      });
      return updated;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al rechazar contrato";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async downloadContractPdf(
    token: AccessToken,
    contractId: string,
  ): Promise<Blob | null> {
    try {
      return await this.#port.downloadContractPdf(token, contractId);
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al descargar PDF del contrato";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async loadOperations(
    token: AccessToken,
    reservationId: string,
  ): Promise<readonly RentalOperation[]> {
    try {
      const operations = await this.#port.listOperations(token, reservationId);
      this.#setSnapshot({
        operations: Object.freeze(operations),
      });
      return operations;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al listar operaciones";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async recordCheckIn(
    token: AccessToken,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const op = await this.#port.recordCheckIn(
        token,
        idempotencyKey,
        reservationId,
        input,
      );
      this.#setSnapshot({
        operations: Object.freeze([...this.#snapshot.operations, op]),
        actionLoading: false,
      });
      return op;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al registrar check-in";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async recordCheckOut(
    token: AccessToken,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const op = await this.#port.recordCheckOut(
        token,
        idempotencyKey,
        reservationId,
        input,
      );
      this.#setSnapshot({
        operations: Object.freeze([...this.#snapshot.operations, op]),
        actionLoading: false,
      });
      return op;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al registrar check-out";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async recordReception(
    token: AccessToken,
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const op = await this.#port.recordReception(
        token,
        idempotencyKey,
        reservationId,
        input,
      );
      this.#setSnapshot({
        operations: Object.freeze([...this.#snapshot.operations, op]),
        actionLoading: false,
      });
      return op;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al registrar recepción";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadMessages(
    token: AccessToken,
    reservationId: string,
    query?: { before?: number | string; limit?: number },
  ): Promise<readonly ReservationMessage[]> {
    try {
      const messages = await this.#port.listMessages(
        token,
        reservationId,
        query,
      );
      const deduplicated = deduplicateById([
        ...this.#snapshot.messages,
        ...messages,
      ]);
      this.#setSnapshot({
        messages: Object.freeze(deduplicated),
      });
      return deduplicated;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar mensajes";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async sendMessage(
    token: AccessToken,
    reservationId: string,
    body: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<ReservationMessage> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const msg = await this.#port.sendMessage(
        token,
        idempotencyKey,
        reservationId,
        body,
      );
      const deduplicated = deduplicateById([...this.#snapshot.messages, msg]);
      this.#setSnapshot({
        messages: Object.freeze(deduplicated),
        actionLoading: false,
      });
      return msg;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al enviar mensaje";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async markMessagesRead(
    token: AccessToken,
    reservationId: string,
    throughSequence: number | string,
  ): Promise<{ sequence: number }> {
    try {
      const res = await this.#port.advanceReadCursor(
        token,
        reservationId,
        throughSequence,
      );
      if (this.#snapshot.currentReservation?.id === reservationId) {
        this.#setSnapshot({
          currentReservation: {
            ...this.#snapshot.currentReservation,
            unread_count: 0,
          },
        });
      }
      return res;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al actualizar cursor de lectura";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async loadReviews(
    token: AccessToken,
    reservationId: string,
  ): Promise<readonly ReservationReview[]> {
    try {
      const reviews = await this.#port.listReviews(token, reservationId);
      this.#setSnapshot({
        reviews: Object.freeze(reviews),
      });
      return reviews;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar reseñas";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async submitReview(
    token: AccessToken,
    reservationId: string,
    input: {
      rating: number;
      comment?: string;
      target_type?: "espacio" | "arrendatario";
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<ReservationReview> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const review = await this.#port.createReview(
        token,
        idempotencyKey,
        reservationId,
        input,
      );
      this.#setSnapshot({
        reviews: Object.freeze([...this.#snapshot.reviews, review]),
        actionLoading: false,
      });
      return review;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al publicar reseña";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async reportReview(
    token: AccessToken,
    reservationId: string,
    reviewId: string,
    input: {
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      comment?: string;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<ReviewReport> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const report = await this.#port.reportReview(
        token,
        idempotencyKey,
        reservationId,
        reviewId,
        input,
      );
      this.#setSnapshot({ actionLoading: false });
      return report;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al reportar reseña";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadDisputes(
    token: AccessToken,
    reservationId: string,
  ): Promise<readonly TrialDispute[]> {
    try {
      const disputes = await this.#port.listDisputes(token, reservationId);
      this.#setSnapshot({
        disputes: Object.freeze(disputes),
      });
      return disputes;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar disputas";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async openDispute(
    token: AccessToken,
    reservationId: string,
    reasonCode = "ensayo_privacidad",
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<TrialDispute> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const dispute = await this.#port.openDispute(
        token,
        idempotencyKey,
        reservationId,
        reasonCode,
      );
      this.#setSnapshot({
        disputes: Object.freeze([...this.#snapshot.disputes, dispute]),
        actionLoading: false,
      });
      return dispute;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al abrir disputa";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadDamageClaim(
    token: AccessToken,
    reservationId: string,
  ): Promise<DamageClaim | null> {
    try {
      const claim = await this.#port.getDamageClaim(token, reservationId);
      this.#setSnapshot({
        damageClaim: claim,
      });
      return claim;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar reclamo de daños";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async openDamageClaim(
    token: AccessToken,
    reservationId: string,
    description: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<DamageClaim> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const claim = await this.#port.openDamageClaim(
        token,
        idempotencyKey,
        reservationId,
        description,
      );
      this.#setSnapshot({
        damageClaim: claim,
        actionLoading: false,
      });
      return claim;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al abrir reclamo de daños";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async submitDamageClaimDefense(
    token: AccessToken,
    reservationId: string,
    description: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<DamageClaim> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const claim = await this.#port.submitDamageClaimDefense(
        token,
        idempotencyKey,
        reservationId,
        description,
      );
      this.#setSnapshot({
        damageClaim: claim,
        actionLoading: false,
      });
      return claim;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al registrar descargo";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadGuarantee(
    token: AccessToken,
    reservationId: string,
  ): Promise<Guarantee | null> {
    try {
      const guarantee = await this.#port.getGuarantee(token, reservationId);
      this.#setSnapshot({
        guarantee,
      });
      return guarantee;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar garantía";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async executeGuaranteeOperation(
    token: AccessToken,
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Guarantee> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const guarantee = await this.#port.executeGuaranteeOperation(
        token,
        idempotencyKey,
        reservationId,
        input,
      );
      this.#setSnapshot({
        guarantee,
        actionLoading: false,
      });
      return guarantee;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al operar sobre la garantía";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadSettlement(
    token: AccessToken,
    reservationId: string,
  ): Promise<Settlement | null> {
    try {
      const settlement = await this.#port.getSettlement(token, reservationId);
      this.#setSnapshot({
        settlement,
      });
      return settlement;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al cargar liquidación";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async requestSettlement(
    token: AccessToken,
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Settlement> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const settlement = await this.#port.requestSettlement(
        token,
        idempotencyKey,
        reservationId,
        outcome,
      );
      this.#setSnapshot({
        settlement,
        actionLoading: false,
      });
      return settlement;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al solicitar liquidación";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async reconcileSettlement(
    token: AccessToken,
    reservationId: string,
    outcome: "exito" | "rechazo",
  ): Promise<Settlement> {
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const settlement = await this.#port.reconcileSettlement(
        token,
        reservationId,
        outcome,
      );
      this.#setSnapshot({
        settlement,
        actionLoading: false,
      });
      return settlement;
    } catch (err) {
      const error =
        err instanceof Error ? err.message : "Error al conciliar liquidación";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }

  async loadSettlementReceipt(
    token: AccessToken,
    reservationId: string,
  ): Promise<SettlementReceipt | null> {
    try {
      const receipt = await this.#port.getSettlementReceipt(
        token,
        reservationId,
      );
      this.#setSnapshot({
        settlementReceipt: receipt,
      });
      return receipt;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al cargar comprobante de liquidación";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async loadBusinessClock(token: AccessToken): Promise<BusinessClock> {
    try {
      const clock = await this.#port.getBusinessClock(token);
      this.#setSnapshot({
        businessClock: clock,
      });
      return clock;
    } catch (err) {
      const error =
        err instanceof Error
          ? err.message
          : "Error al consultar reloj comercial";
      this.#setSnapshot({ error });
      throw err;
    }
  }

  async selectReservation(
    token: AccessToken,
    reservationId: string,
  ): Promise<BookingDetail | null> {
    return this.loadReservationDetail(token, reservationId);
  }

  async submitSimulatedPayment(
    token: AccessToken,
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    return this.simulatePayment(token, reservationId, outcome, idempotencyKey);
  }

  async acceptReservation(
    token: AccessToken,
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation> {
    return this.decideReservation(token, reservationId, "aprobar", reason);
  }

  async rejectReservation(
    token: AccessToken,
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation> {
    return this.decideReservation(token, reservationId, "rechazar", reason);
  }

  async requestCancellationPreview(
    token: AccessToken,
    reservationId: string,
  ): Promise<CancellationPreview> {
    return this.previewCancellation(token, reservationId);
  }

  async performCheckIn(
    token: AccessToken,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    return this.recordCheckIn(token, reservationId, input, idempotencyKey);
  }

  async performCheckOut(
    token: AccessToken,
    reservationId: string,
    input: {
      occurred_at?: string;
      time_zone?: string;
      comments?: string;
      fixture_evidence_code?:
        "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1";
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    return this.recordCheckOut(token, reservationId, input, idempotencyKey);
  }

  async performReception(
    token: AccessToken,
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    return this.recordReception(token, reservationId, input, idempotencyKey);
  }

  async claimSecurityDeposit(
    token: AccessToken,
    reservationId: string,
    description: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<DamageClaim> {
    return this.openDamageClaim(
      token,
      reservationId,
      description,
      idempotencyKey,
    );
  }

  async loadReceipt(
    token: AccessToken,
    reservationId: string,
  ): Promise<SettlementReceipt | null> {
    return this.loadSettlementReceipt(token, reservationId);
  }

  async getBusinessClock(token: AccessToken): Promise<BusinessClock> {
    return this.loadBusinessClock(token);
  }

  async advanceBusinessClock(token: AccessToken): Promise<BusinessClock> {
    return this.loadBusinessClock(token);
  }

  async reconcileGuarantee(
    token: AccessToken,
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Guarantee> {
    return this.executeGuaranteeOperation(
      token,
      reservationId,
      input,
      idempotencyKey,
    );
  }

  async submitFinancialDecision(
    token: AccessToken,
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Settlement> {
    return this.requestSettlement(
      token,
      reservationId,
      outcome,
      idempotencyKey,
    );
  }

  getDisputeHistory(): readonly TrialDisputeTransition[] {
    return this.#snapshot.currentDisputeHistory;
  }

  async fetchFixture(token: AccessToken): Promise<unknown> {
    return this.#port.getFixture(token);
  }
}
