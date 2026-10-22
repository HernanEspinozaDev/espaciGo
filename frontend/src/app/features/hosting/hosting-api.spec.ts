import { HttpEventType, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Img01Media, M04Local, M04M06Local, M06Local } from '../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { provideFrontendPorts } from '../../frontend.providers';
import { HOSTING_UPLOAD_BACKEND, HostingApi } from './infrastructure/hosting-api';

describe('HostingApi', () => {
  afterEach(() => TestBed.resetTestingModule());

  function setup() {
    const catalog = { listSpaceCategories: vi.fn().mockResolvedValue({ data: { items: [] } }) };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: HOSTING_UPLOAD_BACKEND, useExisting: HttpTestingController },
        { provide: M04Local, useValue: catalog },
        { provide: M04M06Local, useValue: {} },
        { provide: M06Local, useValue: {} },
        { provide: Img01Media, useValue: {} },
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

  it('loads the public region and commune reference without a bearer', async () => {
    const { api, http } = setup();
    const pending = api.publicLocations();
    const request = http.expectOne('http://go.test/api/v1/public/locations');
    expect(request.request.method).toBe('GET');
    expect(request.request.headers.has('Authorization')).toBe(false);
    request.flush('{"items":[{"region_code":"13","region_name":"Metropolitana de Santiago","comuna_code":"13123","comuna_name":"Providencia"}]}');
    expect(await pending).toEqual({ items: [{ region_code: '13', region_name: 'Metropolitana de Santiago', comuna_code: '13123', comuna_name: 'Providencia' }] });
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

  it('uploads each image as authenticated multipart with idempotency and reports progress', async () => {
    const { api, http } = setup();
    const file = new File(['image bytes'], 'room.png', { type: 'image/png' });
    const progress: Array<number | null> = [];
    const pending = api.uploadPhoto('bearer-private', 'space-a', file, 'idem-key-123', (value) => progress.push(value));
    const request = http.expectOne('http://go.test/api/v1/spaces/space-a/gallery');
    expect(request.request.method).toBe('POST');
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-private');
    expect(request.request.headers.get('Idempotency-Key')).toBe('idem-key-123');
    expect(request.request.headers.has('Content-Type')).toBe(false);
    expect(request.request.body).toBeInstanceOf(FormData);
    expect((request.request.body as FormData).get('image')).toMatchObject({ name: file.name, type: file.type });
    request.event({ type: HttpEventType.UploadProgress, loaded: 5, total: 10 } as never);
    request.flush('{"photo":{"id":"photo-a","space_id":"space-a","alt_text":"","visible":false,"cover":false,"position":1,"state":"ready","created_at":"2026-10-22T00:00:00Z","variants":[]}}');
    expect(await pending).toMatchObject({ id: 'photo-a', position: 1 });
    expect(progress).toEqual([50, 100]);
  });
});
