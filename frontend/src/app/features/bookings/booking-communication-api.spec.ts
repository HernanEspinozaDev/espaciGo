import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { BookingCommunicationApi, BookingCommunicationAttempts } from './booking-communication-api';

describe('BookingCommunicationApi generated OpenAPI access', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('uses generated M09 paging, in-memory Bearer, and idempotency header', async () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:8080/api/v1');
    const api = TestBed.inject(BookingCommunicationApi); const http = TestBed.inject(HttpTestingController);
    const result = api.messages('memory-token', 'reservation-a', 31);
    await new Promise((resolve) => setTimeout(resolve, 0));
    const request = http.expectOne((candidate) => candidate.url.includes('/reservations/reservation-a/messages'));
    expect(request.request.urlWithParams).toContain('limit=30');
    expect(request.request.urlWithParams).toContain('before=31');
    expect(request.request.headers.get('Authorization')).toBe('Bearer memory-token');
    request.flush({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { items: [], older_cursor: null } });
    expect((await result).data.items).toEqual([]);

    const send = api.sendMessage('memory-token', 'reservation-a', 'plain text', 'idempotency-original');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const sendRequest = http.expectOne((candidate) => candidate.url.endsWith('/reservations/reservation-a/messages'));
    expect(sendRequest.request.headers.get('Authorization')).toBe('Bearer memory-token');
    expect(sendRequest.request.headers.get('Idempotency-Key')).toBe('idempotency-original');
    expect(sendRequest.request.body).toEqual({ body: 'plain text' });
    sendRequest.flush({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { id: 'm1', reservation_id: 'reservation-a', author_id: 'renter-a', sequence: 4, body: 'plain text', created_at: '2026-10-12T10:00:00Z' } });
    await send; http.verify();
  });

  it('keeps idempotency payloads scoped in memory by account and reservation', () => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    const attempts = TestBed.inject(BookingCommunicationAttempts);
    const renter = attempts.get('renter-a:reservation-a:message', { body: 'one' });
    expect(attempts.get('renter-a:reservation-a:message', { body: 'one' }).key).toBe(renter.key);
    expect(attempts.get('host-a:reservation-a:message', { body: 'one' }).key).not.toBe(renter.key);
    expect(attempts.get('renter-a:reservation-b:message', { body: 'one' }).key).not.toBe(renter.key);
    expect(() => attempts.get('renter-a:reservation-a:message', { body: 'two' })).toThrow('solicitud incierta');
    expect(JSON.stringify(renter)).not.toContain('memory-token');
  });
});
