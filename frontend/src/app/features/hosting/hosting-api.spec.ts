import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { M04Local, M04M06Local, M06Local } from '../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { HostingApi } from './hosting-api';

describe('HostingApi', () => {
  afterEach(() => TestBed.resetTestingModule());

  function setup() {
    const catalog = { listSpaceCategories: vi.fn().mockResolvedValue({ data: { items: [] } }) };
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: M04Local, useValue: catalog },
        { provide: M04M06Local, useValue: {} },
        { provide: M06Local, useValue: {} },
        { provide: ApiRuntimeConfig, useValue: { apiBaseUrl: () => 'http://go.test/api/v1' } },
      ],
    });
    return {
      api: TestBed.inject(HostingApi),
      http: TestBed.inject(HttpTestingController),
      catalog,
    };
  }

  it('uses the generated category operation with the in-memory bearer', async () => {
    const { api, catalog } = setup();
    await api.categories('bearer-private');
    expect(catalog.listSpaceCategories).toHaveBeenCalledWith({
      headers: { Authorization: 'Bearer bearer-private' },
    });
  });

  it('sends and reads int64 tariffs as exact decimal tokens', async () => {
    const { api, http } = setup();
    const amount = '9223372036854775807';
    const pending = api.updateTariff('bearer-private', 'space-a', {
      rate_unit: 'mes',
      base_price: amount as unknown as number,
    });
    const request = http.expectOne('http://go.test/api/v1/spaces/space-a/tariff');
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-private');
    expect(request.request.body).toContain(`"base_price":${amount}`);
    request.flush(
      `{"space_id":"space-a","version":4,"rate_unit":"mes","base_price":${amount},"currency":"CLP","created_at":"2026-10-12T00:00:00Z"}`,
    );
    const tariff = await pending;
    expect(String(tariff.base_price)).toBe(amount);
  });

  it('fetches synthetic gallery bytes only through an authenticated request', async () => {
    const { api, http } = setup();
    const pending = api.photoContent('bearer-private', 'space-a', 'photo-a');
    const request = http.expectOne('http://go.test/api/v1/spaces/space-a/gallery/photo-a/content');
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-private');
    expect(request.request.responseType).toBe('blob');
    request.flush(new Blob(['synthetic png'], { type: 'image/png' }));
    expect(await pending).toBeInstanceOf(Blob);
  });
});
