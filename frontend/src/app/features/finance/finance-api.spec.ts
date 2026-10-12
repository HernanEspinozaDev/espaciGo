import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { FinanceApi } from './finance-api';

describe('FinanceApi', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('preserves exact CLP values and only keeps the same idempotent payload after a network timeout', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:18180/api/v1');
    const api = TestBed.inject(FinanceApi); const http = TestBed.inject(HttpTestingController);
    const body = { outcome: 'sin_respuesta' };
    const first = api.call<{ data: { transfer_clp: string } }>('bearer-memory-only', '/local/booking-trial/reservations/r-1/settlement', 'POST', body, 'acct:r-1:settlement');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const original = http.expectOne((request) => request.url.endsWith('/settlement'));
    const key = original.request.headers.get('Idempotency-Key');
    expect(original.request.headers.get('Authorization')).toBe('Bearer bearer-memory-only');
    original.error(new ProgressEvent('error'));
    await expect(first).rejects.toMatchObject({ status: 0 });

    const retry = api.call<{ data: { transfer_clp: string } }>('bearer-memory-only', '/local/booking-trial/reservations/r-1/settlement', 'POST', body, 'acct:r-1:settlement');
    const repeated = http.expectOne((request) => request.url.endsWith('/settlement'));
    expect(repeated.request.headers.get('Idempotency-Key')).toBe(key);
    expect(repeated.request.body).toEqual(body);
    const different = api.call('bearer-memory-only', '/local/booking-trial/reservations/r-1/settlement', 'POST', { outcome: 'exito' }, 'acct:r-1:settlement');
    await expect(different).rejects.toMatchObject({ status: 409 });
    repeated.flush('{"data":{"transfer_clp":9223372036854775807}}');
    expect((await retry).data.transfer_clp).toBe('9223372036854775807');
    expect(JSON.stringify({ key, body })).not.toContain('bearer-memory-only');
    http.verify();
  });

  it('keeps the source identity permission status from Go', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:18180/api/v1');
    const api = TestBed.inject(FinanceApi); const http = TestBed.inject(HttpTestingController);
    const request = api.call('third-party', '/admin/local/damage-claims', 'GET');
    await new Promise((resolve) => setTimeout(resolve, 0));
    http.expectOne((candidate) => candidate.url.endsWith('/admin/local/damage-claims'))
      .flush('{"error":"forbidden"}', { status: 403, statusText: 'Forbidden' });
    await expect(request).rejects.toMatchObject({ status: 403 });
    http.verify();
  });

  it('retains the original idempotency attempt after a 5xx ambiguous write result', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:18180/api/v1');
    const api = TestBed.inject(FinanceApi); const http = TestBed.inject(HttpTestingController);
    const body = { kind: 'captura', amount_clp: 1000, outcome: 'sin_respuesta' };
    const first = api.call('bearer-memory-only', '/reservations/r-1/guarantee', 'POST', body, 'acct:r-1:guarantee:captura');
    const request = http.expectOne((candidate) => candidate.url.endsWith('/guarantee'));
    const key = request.request.headers.get('Idempotency-Key');
    request.flush('{"error":"temporary failure"}', { status: 503, statusText: 'Service Unavailable' });
    await expect(first).rejects.toMatchObject({ status: 503 });

    const retry = api.call('bearer-memory-only', '/reservations/r-1/guarantee', 'POST', body, 'acct:r-1:guarantee:captura');
    const retried = http.expectOne((candidate) => candidate.url.endsWith('/guarantee'));
    expect(retried.request.headers.get('Idempotency-Key')).toBe(key);
    expect(retried.request.body).toEqual(body);
    retried.flush('{"data":{"state":"pendiente"}}');
    await retry;
    http.verify();
  });
});
