import { HttpClient, HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import type {
  LocalBookingCancellationPreview,
  LocalBookingDetail,
  LocalBookingReservation,
} from '../../../core/contracts/models';
import { parseExactClpJson } from '../../../core/exact-json';
import {
  BookingApiError,
  BookingEnvelope,
  BookingPort,
  CancellationResult,
  PaymentOutcome,
  RefundOutcome,
  RefundResult,
} from '../domain/ports/booking-port';

@Injectable({ providedIn: 'root' })
export class BookingApi extends BookingPort {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);

  list(token: string): Promise<BookingEnvelope<{ items: LocalBookingReservation[] }>> {
    return this.request(token, '/local/booking-trial/reservations', 'GET');
  }
  detail(token: string, id: string): Promise<BookingEnvelope<LocalBookingDetail>> {
    return this.request(token, this.path(id), 'GET');
  }
  requestReservation(
    token: string,
    quoteId: string,
    key: string,
  ): Promise<BookingEnvelope<LocalBookingReservation>> {
    return this.request(
      token,
      '/local/booking-trial/reservations',
      'POST',
      { quote_id: quoteId },
      key,
    );
  }
  payment(
    token: string,
    id: string,
    outcome: PaymentOutcome,
    key: string,
  ): Promise<BookingEnvelope<LocalBookingReservation>> {
    return this.request(token, `${this.path(id)}/payment`, 'POST', { outcome }, key);
  }
  decision(
    token: string,
    id: string,
    decision: 'aprobar' | 'rechazar',
    reason: string,
  ): Promise<BookingEnvelope<LocalBookingReservation>> {
    return this.request(token, `${this.path(id)}/decision`, 'POST', { decision, reason });
  }
  cancellationPreview(
    token: string,
    id: string,
  ): Promise<BookingEnvelope<LocalBookingCancellationPreview>> {
    return this.request(token, `${this.path(id)}/cancellation-preview`, 'GET');
  }
  cancel(
    token: string,
    id: string,
    reason: string,
    key: string,
  ): Promise<BookingEnvelope<CancellationResult>> {
    return this.request(token, `${this.path(id)}/cancel`, 'POST', { reason }, key);
  }
  refund(
    token: string,
    id: string,
    outcome: RefundOutcome,
    operationId: string,
  ): Promise<BookingEnvelope<RefundResult>> {
    return this.request(token, `${this.path(id)}/refund`, 'POST', { outcome }, operationId);
  }

  private path(id: string): string {
    return `/local/booking-trial/reservations/${encodeURIComponent(id)}`;
  }
  private async request<T>(
    token: string,
    path: string,
    method: 'GET' | 'POST',
    body?: unknown,
    key?: string,
  ): Promise<T> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new BookingApiError('La API todavía no está disponible.');
    let headers = new HttpHeaders({ Authorization: `Bearer ${token}` });
    if (key) headers = headers.set('Idempotency-Key', key);
    try {
      const text = await firstValueFrom(
        this.http.request('' + method, `${base}${path}`, {
          headers,
          body,
          responseType: 'text',
        }),
      );
      return parseExactClpJson<T>(text);
    } catch (error) {
      if (error instanceof HttpErrorResponse) {
        const raw = typeof error.error === 'string' ? error.error : '';
        let parsed: unknown = undefined;
        try {
          if (raw) parsed = parseExactClpJson<unknown>(raw);
        } catch {
          /* malformed body */
        }
        const message =
          typeof parsed === 'object' &&
          parsed &&
          'error' in parsed &&
          typeof parsed.error === 'object' &&
          parsed.error &&
          'message' in parsed.error
            ? String(parsed.error.message)
            : `Go respondió HTTP ${error.status || 'sin conexión'}.`;
        throw new BookingApiError(message, error.status, parsed);
      }
      throw new BookingApiError(
        error instanceof Error ? error.message : 'No se pudo completar la solicitud.',
      );
    }
  }
}

export {
  BookingApiError,
  type BookingEnvelope,
  type CancellationResult,
  type PaymentOutcome,
  type RefundOutcome,
  type RefundResult,
} from '../domain/ports/booking-port';
