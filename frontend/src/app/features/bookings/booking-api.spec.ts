import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { BookingApi, BookingApiError } from './booking-api';
import { BookingAttempts } from './booking-attempts';

describe('BookingApi', () => {
  it('sends authorization and stable idempotency key, preserving int64 money on request success', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi); const http = TestBed.inject(HttpTestingController);
    const promise = api.payment('bearer-private', 'reservation-1', 'exito', 'attempt-key');
    const request = http.expectOne('http://api.test/api/v1/local/booking-trial/reservations/reservation-1/payment');
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-private');
    expect(request.request.headers.get('Idempotency-Key')).toBe('attempt-key');
    expect(request.request.body).toEqual({ outcome: 'exito' });
    request.flush('{"safety_notice":"ENSAYO LOCAL — SIN COBRO REAL","data":{"id":"r1","subtotal_clp":9223372036854775807}}');
    const result = await promise;
    expect((result.data as unknown as { subtotal_clp: string }).subtotal_clp).toBe('9223372036854775807');
    http.verify(); TestBed.resetTestingModule();
  });

  it('retains timeout status and exact fake outcome payload for an uncertain payment', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi); const http = TestBed.inject(HttpTestingController);
    const promise = api.payment('bearer-private', 'r-2', 'sin_respuesta', 'same-key');
    http.expectOne('http://api.test/api/v1/local/booking-trial/reservations/r-2/payment').flush(
      '{"data":{"subtotal_clp":9007199254740993},"error":{"code":"simulated_payment_timeout","message":"pendiente"}}',
      { status: 504, statusText: 'Gateway Timeout' },
    );
    await expect(promise).rejects.toMatchObject({ name: 'BookingApiError', status: 504, body: { data: { subtotal_clp: '9007199254740993' } } } satisfies Partial<BookingApiError>);
    http.verify(); TestBed.resetTestingModule();
  });

  it('uses the Go refund operation identity as the idempotency key and preserves its int64 amount', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi); const http = TestBed.inject(HttpTestingController);
    const promise = api.refund('bearer-private', 'r-3', 'exito', 'refund-operation-uuid');
    const request = http.expectOne('http://api.test/api/v1/local/booking-trial/reservations/r-3/refund');
    expect(request.request.headers.get('Idempotency-Key')).toBe('refund-operation-uuid');
    request.flush('{"safety_notice":"ENSAYO LOCAL — SIN COBRO REAL","data":{"reservation_id":"r-3","operation_id":"refund-operation-uuid","amount_clp":9223372036854775807,"currency":"CLP","state":"completada","last_result":"exito_simulado","updated_at":"2026-10-12T00:00:00Z","notice_status":"no_disponible_no_durable"}}');
    const result = await promise;
    expect(result.data.amount_clp).toBe('9223372036854775807');
    http.verify(); TestBed.resetTestingModule();
  });
});

describe('BookingAttempts', () => {
  it('reuses the uncertain attempt only for the same payload and isolates reservation operations', () => {
    TestBed.configureTestingModule({});
    const attempts = TestBed.inject(BookingAttempts);
    const first = attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' });
    expect(attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' })).toEqual(first);
    expect(() => attempts.get('account-A', 'reservation:payment', { outcome: 'exito' })).toThrow(/mismos datos/);
    const otherAccount = attempts.get('account-B', 'reservation:payment', { outcome: 'sin_respuesta' });
    expect(otherAccount.key).not.toBe(first.key);
    attempts.settle('account-A', 'reservation:payment', 'old-key');
    expect(attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' })).toEqual(first);
    attempts.settle('account-A', 'reservation:payment', first.key);
    expect(attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' }).key).not.toBe(first.key);
    TestBed.resetTestingModule();
  });
});
