import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { provideFrontendPorts } from '../../frontend.providers';
import { BookingAttempts } from './application/booking-attempts';
import { BookingApi, BookingApiError } from './infrastructure/booking-api';

describe('BookingApi', () => {
  it('sends authorization and stable idempotency key, preserving int64 money on request success', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi);
    const http = TestBed.inject(HttpTestingController);
    const promise = api.payment('bearer-private', 'reservation-1', 'exito', 'attempt-key');
    const request = http.expectOne(
      'http://api.test/api/v1/local/booking-trial/reservations/reservation-1/payment',
    );
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-private');
    expect(request.request.headers.get('Idempotency-Key')).toBe('attempt-key');
    expect(request.request.body).toEqual({ outcome: 'exito' });
    request.flush(
      '{"safety_notice":"ENSAYO LOCAL — SIN COBRO REAL","data":{"id":"r1","subtotal_clp":9223372036854775807}}',
    );
    const result = await promise;
    expect((result.data as unknown as { subtotal_clp: string }).subtotal_clp).toBe(
      '9223372036854775807',
    );
    http.verify();
    TestBed.resetTestingModule();
  });

  it('decodes a full historical reservation response with nullable guarantee and exact amounts', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi);
    const http = TestBed.inject(HttpTestingController);
    const promise = api.detail('bearer-private', 'r-historical');
    const request = http.expectOne(
      'http://api.test/api/v1/local/booking-trial/reservations/r-historical',
    );
    expect(request.request.method).toBe('GET');
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-private');
    request.flush(
      '{"safety_notice":"ENSAYO LOCAL — SIN COBRO REAL","data":{"id":"r-historical","quote_id":"q-1","space_id":"s-1","host_id":"h-1","renter_id":"u-1","state":"finalizada","rate_unit":"dia","unit_price_clp":9223372036854775807,"currency":"CLP","units":1,"subtotal_clp":0,"start_at":"2026-10-01T00:00:00Z","end_at":"2026-10-02T00:00:00Z","time_zone":"America/Santiago","conditions":"Fixture sintético","cancellation_policy_version":"local_flexible_v1","guarantee_expected_clp":null,"pay_expires_at":"2026-10-01T00:00:00Z","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-10-02T00:00:00Z","unread_count":0,"history":[]}}',
    );
    const result = await promise;
    const decoded = result.data as unknown as {
      guarantee_expected_clp: string | null;
      unit_price_clp: string;
      subtotal_clp: string;
    };
    expect(decoded.guarantee_expected_clp).toBeNull();
    expect(decoded.unit_price_clp).toBe('9223372036854775807');
    expect(decoded.subtotal_clp).toBe('0');
    http.verify();
    TestBed.resetTestingModule();
  });

  it('retains timeout status and exact fake outcome payload for an uncertain payment', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi);
    const http = TestBed.inject(HttpTestingController);
    const promise = api.payment('bearer-private', 'r-2', 'sin_respuesta', 'same-key');
    http
      .expectOne('http://api.test/api/v1/local/booking-trial/reservations/r-2/payment')
      .flush(
        '{"data":{"subtotal_clp":9007199254740993},"error":{"code":"simulated_payment_timeout","message":"pendiente"}}',
        { status: 504, statusText: 'Gateway Timeout' },
      );
    await expect(promise).rejects.toMatchObject({
      name: 'BookingApiError',
      status: 504,
      body: { data: { subtotal_clp: '9007199254740993' } },
    } satisfies Partial<BookingApiError>);
    http.verify();
    TestBed.resetTestingModule();
  });

  it('uses the Go refund operation identity as the idempotency key and preserves its int64 amount', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(BookingApi);
    const http = TestBed.inject(HttpTestingController);
    const promise = api.refund('bearer-private', 'r-3', 'exito', 'refund-operation-uuid');
    const request = http.expectOne(
      'http://api.test/api/v1/local/booking-trial/reservations/r-3/refund',
    );
    expect(request.request.headers.get('Idempotency-Key')).toBe('refund-operation-uuid');
    request.flush(
      '{"safety_notice":"ENSAYO LOCAL — SIN COBRO REAL","data":{"reservation_id":"r-3","operation_id":"refund-operation-uuid","amount_clp":9223372036854775807,"currency":"CLP","state":"completada","last_result":"exito_simulado","updated_at":"2026-10-12T00:00:00Z","notice_status":"no_disponible_no_durable"}}',
    );
    const result = await promise;
    expect(result.data.amount_clp).toBe('9223372036854775807');
    http.verify();
    TestBed.resetTestingModule();
  });
});

describe('BookingAttempts', () => {
  it('reuses the uncertain attempt only for the same payload and isolates reservation operations', () => {
    TestBed.configureTestingModule({});
    const attempts = TestBed.inject(BookingAttempts);
    const first = attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' });
    expect(attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' })).toEqual(
      first,
    );
    expect(() => attempts.get('account-A', 'reservation:payment', { outcome: 'exito' })).toThrow(
      /mismos datos/,
    );
    const otherAccount = attempts.get('account-B', 'reservation:payment', {
      outcome: 'sin_respuesta',
    });
    expect(otherAccount.key).not.toBe(first.key);
    attempts.settle('account-A', 'reservation:payment', 'old-key');
    expect(attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' })).toEqual(
      first,
    );
    attempts.settle('account-A', 'reservation:payment', first.key);
    expect(
      attempts.get('account-A', 'reservation:payment', { outcome: 'sin_respuesta' }).key,
    ).not.toBe(first.key);
    TestBed.resetTestingModule();
  });

  it('does not let an older confirmed refund snapshot replace a newer Go result', () => {
    TestBed.configureTestingModule({});
    const attempts = TestBed.inject(BookingAttempts);
    attempts.saveRefundSnapshot('account-A', 'reservation-1', {
      operationId: 'refund-op',
      amount: '12500',
      state: 'completada',
      revision: 12,
    });
    attempts.saveRefundSnapshot('account-A', 'reservation-1', {
      operationId: 'refund-op',
      amount: '12500',
      state: 'pendiente',
      revision: 11,
    });
    expect(attempts.refundSnapshot('account-A', 'reservation-1')?.state).toBe('completada');
    TestBed.resetTestingModule();
  });
});
