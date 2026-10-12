import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { BookingLifecycleApi, BookingLifecycleAttempts } from './booking-lifecycle-api';

describe('BookingLifecycleApi generated OpenAPI access', () => {
  afterEach(() => { TestBed.resetTestingModule(); });

  it('sends the in-memory Bearer and idempotency key to the generated M08 operation', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:8080/api/v1');
    const api = TestBed.inject(BookingLifecycleApi); const http = TestBed.inject(HttpTestingController);
    const result = api.record('token-in-memory', 'reservation-id', 'check-in', { comments: 'demo' }, 'key-12345678');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const request = http.expectOne((candidate) => candidate.url.endsWith('/local/booking-trial/reservations/reservation-id/check-in'));
    expect(request.request.headers.get('Authorization')).toBe('Bearer token-in-memory');
    expect(request.request.headers.has('Cache-Control')).toBe(false);
    expect(request.request.headers.get('Idempotency-Key')).toBe('key-12345678');
    expect(request.request.body).toEqual({ comments: 'demo' });
    request.flush({ data: { id: 'operation-id' }, notice: 'synthetic' }, { status: 201, statusText: 'Created' });
    expect((await result).data.id).toBe('operation-id');

    const pdfResult = api.contractPdf('token-in-memory', 'contract-id');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const pdfRequest = http.expectOne((candidate) => candidate.url.endsWith('/local/booking-trial/contracts/contract-id/document'));
    expect(pdfRequest.request.headers.get('Authorization')).toBe('Bearer token-in-memory');
    expect(pdfRequest.request.responseType).toBe('blob');
    pdfRequest.flush(new Blob(['%PDF'], { type: 'application/pdf' }), { headers: { 'Content-Type': 'application/pdf' } });
    expect((await pdfResult).type).toBe('application/pdf');
    http.verify();
  });

  it('preserves the Go HTTP status from a generated-client 401', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:8080/api/v1');
    const api = TestBed.inject(BookingLifecycleApi); const http = TestBed.inject(HttpTestingController);
    const result = api.operations('expired-token', 'reservation-id');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const request = http.expectOne((candidate) => candidate.url.endsWith('/reservations/reservation-id/operations'));
    request.flush({ error: { code: 'unauthenticated', message: 'expired' } }, { status: 401, statusText: 'Unauthorized' });
    await expect(result).rejects.toMatchObject({ status: 401 });
    http.verify();
  });

  it('scopes uncertain idempotency attempts by account, reservation, and operation without storing Bearer', () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    const attempts = TestBed.inject(BookingLifecycleAttempts);
    const first = attempts.get('account-a', 'reservation-a', 'checkin', { comments: 'same' });
    const retry = attempts.get('account-a', 'reservation-a', 'checkin', { comments: 'same' });
    const anotherAccount = attempts.get('account-b', 'reservation-a', 'checkin', { comments: 'same' });
    const anotherAction = attempts.get('account-a', 'reservation-a', 'checkout', { comments: 'same' });
    expect(retry.key).toBe(first.key);
    expect(anotherAccount.key).not.toBe(first.key);
    expect(anotherAction.key).not.toBe(first.key);
    expect(JSON.stringify(first)).not.toContain('Bearer');
    expect(() => attempts.get('account-a', 'reservation-a', 'checkin', { comments: 'changed' })).toThrow('intento incierto');
  });
});
