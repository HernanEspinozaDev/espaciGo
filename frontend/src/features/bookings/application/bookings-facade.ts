"use client";
import "client-only";
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
import type { AuthSnapshot } from "@/features/auth/application/auth-facade";
import type { BookingsPort } from "./bookings.port";
interface AuthSourceLike {
  getSnapshot(): AuthSnapshot;
  subscribe(listener: () => void): () => void;
  checkSession(): Promise<unknown>;
}
interface BookingSessionScope {
  epoch: number;
  signal: AbortSignal;
  signals: AbortSignal[];
}
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
  #epoch = 0;
  #sessionController = new AbortController();
  #authSource?: AuthSourceLike;
  #onPrivatePurge?: () => void;
  constructor(
    port: BookingsPort,
    authSource?: AuthSourceLike,
    onPrivatePurge?: () => void,
  ) {
    this.#port = port;
    this.#authSource = authSource;
    this.#onPrivatePurge = onPrivatePurge;
    if (authSource) {
      let identity = JSON.stringify(authSource.getSnapshot());
      authSource.subscribe(() => {
        const next = JSON.stringify(authSource.getSnapshot());
        if (identity !== next) {
          identity = next;
          this.clear();
        }
      });
    }
  }
  clear(): void {
    this.#epoch++;
    this.#sessionController.abort();
    this.#sessionController = new AbortController();
    this.cancel();
    this.#snapshot = initialSnapshot;
    this.#onPrivatePurge?.();
    for (const listener of this.#listeners) listener();
  }
  #captureSession(): BookingSessionScope {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }
    return {
      epoch: this.#epoch,
      signal: this.#sessionController.signal,
      signals: [],
    };
  }
  #isSessionCurrent(scope: BookingSessionScope): boolean {
    return (
      scope.epoch === this.#epoch &&
      !scope.signal.aborted &&
      !scope.signals.some((signal) => signal.aborted)
    );
  }
  #sessionSignal(
    scope: BookingSessionScope,
    existing?: AbortSignal,
  ): AbortSignal {
    const signal = existing
      ? AbortSignal.any([scope.signal, existing])
      : scope.signal;
    scope.signals.push(signal);
    return signal;
  }
  async #waitSession<T>(
    value: T | Promise<T>,
    scope: BookingSessionScope,
  ): Promise<T> {
    const result = await value;
    if (!this.#isSessionCurrent(scope))
      throw new DOMException("Session changed", "AbortError");
    return result;
  }
  #sessionError(error: unknown): void {
    const status =
      typeof error === "object" && error !== null && "status" in error
        ? error.status
        : null;
    if (status === 401 || status === 403) {
      this.clear();
      try {
        void this.#authSource?.checkSession().catch(() => undefined);
      } catch {
        /* revalidation is best-effort */
      }
    }
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
  async loadReservations(): Promise<readonly BookingReservation[]> {
    const _sessionScope = this.#captureSession();
    this.cancel();
    this.#abortController = new AbortController();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const reservations = await this.#waitSession(
        this.#port.listReservations(
          this.#sessionSignal(_sessionScope, this.#abortController.signal),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        reservations: Object.freeze(reservations),
        loading: false,
      });
      return await this.#waitSession(reservations, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return [];
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar reservas";
      this.#setSnapshot({ loading: false, error });
      throw err;
    }
  }
  async loadReservationDetail(
    reservationId: string,
  ): Promise<BookingDetail | null> {
    const _sessionScope = this.#captureSession();
    this.cancel();
    this.#abortController = new AbortController();
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#waitSession(
        this.#port.getReservation(
          reservationId,
          this.#sessionSignal(_sessionScope, this.#abortController.signal),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        currentReservation: detail,
        loading: false,
      });
      return await this.#waitSession(detail, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return null;
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al cargar el detalle de la reserva";
      this.#setSnapshot({ loading: false, error });
      throw err;
    }
  }
  async requestQuote(input: {
    space_id: string;
    start_at: string;
    end_at: string;
  }): Promise<BookingQuote> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const quote = await this.#waitSession(
        this.#port.quote(input, this.#sessionSignal(_sessionScope, undefined)),
        _sessionScope,
      );
      this.#setSnapshot({
        quote,
        actionLoading: false,
      });
      return await this.#waitSession(quote, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cotizar reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async createReservation(
    quoteId: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const reservation = await this.#waitSession(
        this.#port.createReservation(
          idempotencyKey,
          quoteId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        reservations: Object.freeze([
          reservation,
          ...this.#snapshot.reservations,
        ]),
        actionLoading: false,
      });
      return await this.#waitSession(reservation, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al crear reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async decideReservation(
    reservationId: string,
    decision: "aprobar" | "rechazar",
    reason?: string,
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.decideReservation(
          reservationId,
          decision,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
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
      return await this.#waitSession(updated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al procesar la decisión de reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async previewCancellation(
    reservationId: string,
  ): Promise<CancellationPreview> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const preview = await this.#waitSession(
        this.#port.previewCancellation(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        cancellationPreview: preview,
        actionLoading: false,
      });
      return await this.#waitSession(preview, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al previsualizar cancelación";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async cancelReservation(
    reservationId: string,
    reason?: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.cancelReservation(
          idempotencyKey,
          reservationId,
          reason,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
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
      return await this.#waitSession(updated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cancelar reserva";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async simulatePayment(
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.simulatePayment(
          idempotencyKey,
          reservationId,
          outcome,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
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
      return await this.#waitSession(updated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al procesar pago simulado";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async simulateRefund(
    reservationId: string,
    outcome: "exito" | "fallo" | "sin_respuesta",
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.simulateRefund(
          idempotencyKey,
          reservationId,
          outcome,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
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
      return await this.#waitSession(updated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al procesar devolución simulada";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadContract(reservationId: string): Promise<SyntheticContract> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const contract = await this.#waitSession(
        this.#port.createOrGetContract(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        contract,
        actionLoading: false,
      });
      return await this.#waitSession(contract, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar contrato";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async signContract(contractId: string): Promise<SyntheticContract> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.signContract(
          contractId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        contract: updated,
        actionLoading: false,
      });
      return await this.#waitSession(updated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al firmar contrato";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async rejectContract(contractId: string): Promise<SyntheticContract> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const updated = await this.#waitSession(
        this.#port.rejectContract(
          contractId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        contract: updated,
        actionLoading: false,
      });
      return await this.#waitSession(updated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al rechazar contrato";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async downloadContractPdf(contractId: string): Promise<Blob | null> {
    const _sessionScope = this.#captureSession();
    try {
      return await this.#waitSession(
        this.#port.downloadContractPdf(
          contractId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        return null;
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al descargar PDF del contrato";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async loadOperations(
    reservationId: string,
  ): Promise<readonly RentalOperation[]> {
    const _sessionScope = this.#captureSession();
    try {
      const operations = await this.#waitSession(
        this.#port.listOperations(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        operations: Object.freeze(operations),
      });
      return await this.#waitSession(operations, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al listar operaciones";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async recordCheckIn(
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
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const op = await this.#waitSession(
        this.#port.recordCheckIn(
          idempotencyKey,
          reservationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        operations: Object.freeze([...this.#snapshot.operations, op]),
        actionLoading: false,
      });
      return await this.#waitSession(op, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al registrar check-in";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async recordCheckOut(
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
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const op = await this.#waitSession(
        this.#port.recordCheckOut(
          idempotencyKey,
          reservationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        operations: Object.freeze([...this.#snapshot.operations, op]),
        actionLoading: false,
      });
      return await this.#waitSession(op, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al registrar check-out";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async recordReception(
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const op = await this.#waitSession(
        this.#port.recordReception(
          idempotencyKey,
          reservationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        operations: Object.freeze([...this.#snapshot.operations, op]),
        actionLoading: false,
      });
      return await this.#waitSession(op, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al registrar recepción";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadMessages(
    reservationId: string,
    query?: {
      before?: number | string;
      limit?: number;
    },
  ): Promise<readonly ReservationMessage[]> {
    const _sessionScope = this.#captureSession();
    try {
      const messages = await this.#waitSession(
        this.#port.listMessages(
          reservationId,
          query,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      const deduplicated = deduplicateById([
        ...this.#snapshot.messages,
        ...messages,
      ]);
      this.#setSnapshot({
        messages: Object.freeze(deduplicated),
      });
      return await this.#waitSession(deduplicated, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar mensajes";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async sendMessage(
    reservationId: string,
    body: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<ReservationMessage> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const msg = await this.#waitSession(
        this.#port.sendMessage(
          idempotencyKey,
          reservationId,
          body,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      const deduplicated = deduplicateById([...this.#snapshot.messages, msg]);
      this.#setSnapshot({
        messages: Object.freeze(deduplicated),
        actionLoading: false,
      });
      return await this.#waitSession(msg, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al enviar mensaje";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async markMessagesRead(
    reservationId: string,
    throughSequence: number | string,
  ): Promise<{
    sequence: number;
  }> {
    const _sessionScope = this.#captureSession();
    try {
      const res = await this.#waitSession(
        this.#port.advanceReadCursor(
          reservationId,
          throughSequence,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      if (this.#snapshot.currentReservation?.id === reservationId) {
        this.#setSnapshot({
          currentReservation: {
            ...this.#snapshot.currentReservation,
            unread_count: 0,
          },
        });
      }
      return await this.#waitSession(res, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al actualizar cursor de lectura";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async loadReviews(
    reservationId: string,
  ): Promise<readonly ReservationReview[]> {
    const _sessionScope = this.#captureSession();
    try {
      const reviews = await this.#waitSession(
        this.#port.listReviews(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        reviews: Object.freeze(reviews),
      });
      return await this.#waitSession(reviews, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar reseñas";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async submitReview(
    reservationId: string,
    input: {
      rating: number;
      comment?: string;
      target_type?: "espacio" | "arrendatario";
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<ReservationReview> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const review = await this.#waitSession(
        this.#port.createReview(
          idempotencyKey,
          reservationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        reviews: Object.freeze([...this.#snapshot.reviews, review]),
        actionLoading: false,
      });
      return await this.#waitSession(review, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al publicar reseña";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async reportReview(
    reservationId: string,
    reviewId: string,
    input: {
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      comment?: string;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<ReviewReport> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const report = await this.#waitSession(
        this.#port.reportReview(
          idempotencyKey,
          reservationId,
          reviewId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({ actionLoading: false });
      return await this.#waitSession(report, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al reportar reseña";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadDisputes(reservationId: string): Promise<readonly TrialDispute[]> {
    const _sessionScope = this.#captureSession();
    try {
      const disputes = await this.#waitSession(
        this.#port.listDisputes(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        disputes: Object.freeze(disputes),
      });
      return await this.#waitSession(disputes, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar disputas";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async openDispute(
    reservationId: string,
    reasonCode = "ensayo_privacidad",
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<TrialDispute> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const dispute = await this.#waitSession(
        this.#port.openDispute(
          idempotencyKey,
          reservationId,
          reasonCode,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        disputes: Object.freeze([...this.#snapshot.disputes, dispute]),
        actionLoading: false,
      });
      return await this.#waitSession(dispute, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al abrir disputa";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadDamageClaim(reservationId: string): Promise<DamageClaim | null> {
    const _sessionScope = this.#captureSession();
    try {
      const claim = await this.#waitSession(
        this.#port.getDamageClaim(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        damageClaim: claim,
      });
      return await this.#waitSession(claim, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar reclamo de daños";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async openDamageClaim(
    reservationId: string,
    description: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<DamageClaim> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const claim = await this.#waitSession(
        this.#port.openDamageClaim(
          idempotencyKey,
          reservationId,
          description,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        damageClaim: claim,
        actionLoading: false,
      });
      return await this.#waitSession(claim, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al abrir reclamo de daños";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async submitDamageClaimDefense(
    reservationId: string,
    description: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<DamageClaim> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const claim = await this.#waitSession(
        this.#port.submitDamageClaimDefense(
          idempotencyKey,
          reservationId,
          description,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        damageClaim: claim,
        actionLoading: false,
      });
      return await this.#waitSession(claim, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al registrar descargo";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadGuarantee(reservationId: string): Promise<Guarantee | null> {
    const _sessionScope = this.#captureSession();
    try {
      const guarantee = await this.#waitSession(
        this.#port.getGuarantee(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        guarantee,
      });
      return await this.#waitSession(guarantee, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar garantía";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async executeGuaranteeOperation(
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Guarantee> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const guarantee = await this.#waitSession(
        this.#port.executeGuaranteeOperation(
          idempotencyKey,
          reservationId,
          input,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        guarantee,
        actionLoading: false,
      });
      return await this.#waitSession(guarantee, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al operar sobre la garantía";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadSettlement(reservationId: string): Promise<Settlement | null> {
    const _sessionScope = this.#captureSession();
    try {
      const settlement = await this.#waitSession(
        this.#port.getSettlement(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        settlement,
      });
      return await this.#waitSession(settlement, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al cargar liquidación";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async requestSettlement(
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Settlement> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const settlement = await this.#waitSession(
        this.#port.requestSettlement(
          idempotencyKey,
          reservationId,
          outcome,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        settlement,
        actionLoading: false,
      });
      return await this.#waitSession(settlement, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al solicitar liquidación";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async reconcileSettlement(
    reservationId: string,
    outcome: "exito" | "rechazo",
  ): Promise<Settlement> {
    const _sessionScope = this.#captureSession();
    this.#setSnapshot({ actionLoading: true, error: null });
    try {
      const settlement = await this.#waitSession(
        this.#port.reconcileSettlement(
          reservationId,
          outcome,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        settlement,
        actionLoading: false,
      });
      return await this.#waitSession(settlement, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error ? err.message : "Error al conciliar liquidación";
      this.#setSnapshot({ actionLoading: false, error });
      throw err;
    }
  }
  async loadSettlementReceipt(
    reservationId: string,
  ): Promise<SettlementReceipt | null> {
    const _sessionScope = this.#captureSession();
    try {
      const receipt = await this.#waitSession(
        this.#port.getSettlementReceipt(
          reservationId,
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        settlementReceipt: receipt,
      });
      return await this.#waitSession(receipt, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al cargar comprobante de liquidación";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async loadBusinessClock(): Promise<BusinessClock> {
    const _sessionScope = this.#captureSession();
    try {
      const clock = await this.#waitSession(
        this.#port.getBusinessClock(
          this.#sessionSignal(_sessionScope, undefined),
        ),
        _sessionScope,
      );
      this.#setSnapshot({
        businessClock: clock,
      });
      return await this.#waitSession(clock, _sessionScope);
    } catch (err) {
      if (!this.#isSessionCurrent(_sessionScope)) {
        throw new DOMException("Session changed", "AbortError");
      }
      this.#sessionError(err);
      const error =
        err instanceof Error
          ? err.message
          : "Error al consultar reloj comercial";
      this.#setSnapshot({ error });
      throw err;
    }
  }
  async selectReservation(
    reservationId: string,
  ): Promise<BookingDetail | null> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.loadReservationDetail(reservationId),
      _sessionScope,
    );
  }
  async submitSimulatedPayment(
    reservationId: string,
    outcome: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.simulatePayment(reservationId, outcome, idempotencyKey),
      _sessionScope,
    );
  }
  async acceptReservation(
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.decideReservation(reservationId, "aprobar", reason),
      _sessionScope,
    );
  }
  async rejectReservation(
    reservationId: string,
    reason?: string,
  ): Promise<BookingReservation> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.decideReservation(reservationId, "rechazar", reason),
      _sessionScope,
    );
  }
  async requestCancellationPreview(
    reservationId: string,
  ): Promise<CancellationPreview> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.previewCancellation(reservationId),
      _sessionScope,
    );
  }
  async performCheckIn(
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
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.recordCheckIn(reservationId, input, idempotencyKey),
      _sessionScope,
    );
  }
  async performCheckOut(
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
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.recordCheckOut(reservationId, input, idempotencyKey),
      _sessionScope,
    );
  }
  async performReception(
    reservationId: string,
    input: {
      result?:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      observations?: string;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<RentalOperation> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.recordReception(reservationId, input, idempotencyKey),
      _sessionScope,
    );
  }
  async claimSecurityDeposit(
    reservationId: string,
    description: string,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<DamageClaim> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.openDamageClaim(reservationId, description, idempotencyKey),
      _sessionScope,
    );
  }
  async loadReceipt(reservationId: string): Promise<SettlementReceipt | null> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.loadSettlementReceipt(reservationId),
      _sessionScope,
    );
  }
  async getBusinessClock(): Promise<BusinessClock> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(this.loadBusinessClock(), _sessionScope);
  }
  async advanceBusinessClock(): Promise<BusinessClock> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(this.loadBusinessClock(), _sessionScope);
  }
  async reconcileGuarantee(
    reservationId: string,
    input: {
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      outcome?: SimulatedPaymentOutcome;
    },
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Guarantee> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.executeGuaranteeOperation(reservationId, input, idempotencyKey),
      _sessionScope,
    );
  }
  async submitFinancialDecision(
    reservationId: string,
    outcome?: SimulatedPaymentOutcome,
    idempotencyKey = generateIdempotencyKey(),
  ): Promise<Settlement> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.requestSettlement(reservationId, outcome, idempotencyKey),
      _sessionScope,
    );
  }
  getDisputeHistory(): readonly TrialDisputeTransition[] {
    return this.#snapshot.currentDisputeHistory;
  }
  async fetchFixture(): Promise<unknown> {
    const _sessionScope = this.#captureSession();
    return await this.#waitSession(
      this.#port.getFixture(this.#sessionSignal(_sessionScope, undefined)),
      _sessionScope,
    );
  }
}
