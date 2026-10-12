import type {
  LocalBookingCancellationPreview,
  LocalBookingDetail,
  LocalBookingReservation,
} from '../../../../core/contracts/models';
export type BookingEnvelope<T> = { safety_notice: string; data: T };
export type CancellationResult = {
  reservation: LocalBookingReservation;
  refund_amount_clp?: string;
  refund_state: 'no_aplica' | 'pendiente' | 'completada';
  notice_status: string;
};
export type RefundResult = {
  reservation_id: string;
  operation_id: string;
  amount_clp: string;
  currency: 'CLP';
  state: 'pendiente' | 'completada';
  last_result: 'exito_simulado' | 'fallo_simulado' | 'sin_respuesta_simulada';
  updated_at: string;
  notice_status: string;
};
export type PaymentOutcome = 'exito' | 'rechazo' | 'sin_respuesta';
export type RefundOutcome = 'exito' | 'fallo' | 'sin_respuesta';
export class BookingApiError extends Error {
  constructor(
    message: string,
    readonly status?: number,
    readonly body?: unknown,
  ) {
    super(message);
    this.name = 'BookingApiError';
  }
}

export abstract class BookingPort {
  abstract list(token: string): Promise<BookingEnvelope<{ items: LocalBookingReservation[] }>>;
  abstract detail(token: string, id: string): Promise<BookingEnvelope<LocalBookingDetail>>;
  abstract requestReservation(
    token: string,
    quoteId: string,
    key: string,
  ): Promise<BookingEnvelope<LocalBookingReservation>>;
  abstract payment(
    token: string,
    id: string,
    outcome: PaymentOutcome,
    key: string,
  ): Promise<BookingEnvelope<LocalBookingReservation>>;
  abstract decision(
    token: string,
    id: string,
    decision: 'aprobar' | 'rechazar',
    reason: string,
  ): Promise<BookingEnvelope<LocalBookingReservation>>;
  abstract cancellationPreview(
    token: string,
    id: string,
  ): Promise<BookingEnvelope<LocalBookingCancellationPreview>>;
  abstract cancel(
    token: string,
    id: string,
    reason: string,
    key: string,
  ): Promise<BookingEnvelope<CancellationResult>>;
  abstract refund(
    token: string,
    id: string,
    outcome: RefundOutcome,
    operationId: string,
  ): Promise<BookingEnvelope<RefundResult>>;
}
