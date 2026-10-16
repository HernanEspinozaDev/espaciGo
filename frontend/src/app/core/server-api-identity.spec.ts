import { HttpClient, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { PLATFORM_ID } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { API_SERVER_BASE_URL } from './public-runtime-config';
import { SERVER_API_IDENTITY_HEADERS, serverApiIdentityInterceptor } from './server-api-identity';

describe('server API identity interceptor', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('authenticates only the configured SSR API origin and path and disables transfer cache', async () => {
    const identity = vi.fn().mockResolvedValue({ 'X-Serverless-Authorization': 'Bearer synthetic-service-token' });
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptors([serverApiIdentityInterceptor])),
        provideHttpClientTesting(),
        { provide: PLATFORM_ID, useValue: 'server' },
        { provide: API_SERVER_BASE_URL, useValue: 'https://api.example.test/api/v1' },
        { provide: SERVER_API_IDENTITY_HEADERS, useValue: identity },
      ],
    });
    const http = TestBed.inject(HttpTestingController);
    const client = TestBed.inject(HttpClient);
    client.get('https://api.example.test/api/v1/public/spaces').subscribe();
    await Promise.resolve();
    await Promise.resolve();
    const request = http.expectOne('https://api.example.test/api/v1/public/spaces');
    expect(request.request.headers.get('X-Serverless-Authorization')).toBe('Bearer synthetic-service-token');
    expect(request.request.transferCache).toBe(false);
    request.flush({ items: [] });
    expect(identity).toHaveBeenCalledOnce();
    http.verify();
  });

  it('does not send service identity to another origin or from browser requests', () => {
    const identity = vi.fn().mockResolvedValue({ 'X-Serverless-Authorization': 'Bearer synthetic-service-token' });
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptors([serverApiIdentityInterceptor])),
        provideHttpClientTesting(),
        { provide: PLATFORM_ID, useValue: 'browser' },
        { provide: API_SERVER_BASE_URL, useValue: 'https://api.example.test/api/v1' },
        { provide: SERVER_API_IDENTITY_HEADERS, useValue: identity },
      ],
    });
    const http = TestBed.inject(HttpTestingController);
    const client = TestBed.inject(HttpClient);
    client.get('https://other.example.test/api/v1/public/spaces').subscribe();
    const request = http.expectOne('https://other.example.test/api/v1/public/spaces');
    expect(request.request.headers.has('X-Serverless-Authorization')).toBe(false);
    request.flush({ items: [] });
    expect(identity).not.toHaveBeenCalled();
    http.verify();
  });

  it('does not use Cloud Run identity for SSR writes or private API namespaces', () => {
    const identity = vi.fn().mockResolvedValue({ 'X-Serverless-Authorization': 'Bearer synthetic-service-token' });
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptors([serverApiIdentityInterceptor])),
        provideHttpClientTesting(),
        { provide: PLATFORM_ID, useValue: 'server' },
        { provide: API_SERVER_BASE_URL, useValue: 'https://api.example.test/api/v1' },
        { provide: SERVER_API_IDENTITY_HEADERS, useValue: identity },
      ],
    });
    const http = TestBed.inject(HttpTestingController);
    const client = TestBed.inject(HttpClient);
    client.get('https://api.example.test/api/v1/reservations').subscribe();
    client.post('https://api.example.test/api/v1/public/spaces', {}).subscribe();
    const privateRead = http.expectOne('https://api.example.test/api/v1/reservations');
    const publicWrite = http.expectOne('https://api.example.test/api/v1/public/spaces');
    expect(privateRead.request.headers.has('X-Serverless-Authorization')).toBe(false);
    expect(publicWrite.request.headers.has('X-Serverless-Authorization')).toBe(false);
    privateRead.flush({});
    publicWrite.flush({});
    expect(identity).not.toHaveBeenCalled();
    http.verify();
  });
});
