import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { CatalogEnvelope, ExploreApi, parseExactClpJson } from './explore-api';

describe('parseExactClpJson', () => {
  it('preserves int64 CLP literals as exact decimal strings before JSON parsing', () => {
    const payload =
      '{"data":{"base_price_clp":9007199254740993,"estimated_total_clp":9223372036854775807,"subtotal_clp":1234567890123456789,"units":3,"area_m2":24.5,"conditions":"base_price_clp: 42"}}';
    const parsed = parseExactClpJson<{
      data: {
        base_price_clp: string;
        estimated_total_clp: string;
        subtotal_clp: string;
        units: number;
        area_m2: number;
        conditions: string;
      };
    }>(payload);

    expect(parsed.data.base_price_clp).toBe('9007199254740993');
    expect(parsed.data.estimated_total_clp).toBe('9223372036854775807');
    expect(parsed.data.subtotal_clp).toBe('1234567890123456789');
    expect(parsed.data.units).toBe(3);
    expect(parsed.data.area_m2).toBe(24.5);
    expect(parsed.data.conditions).toBe('base_price_clp: 42');
  });

  it('leaves JSON strings and non-CLP integer fields unchanged', () => {
    expect(
      parseExactClpJson<{ rate_version: number; unit_price_clp: string }>(
        '{"rate_version":9,"unit_price_clp":10000000000000001}',
      ),
    ).toEqual({ rate_version: 9, unit_price_clp: '10000000000000001' });
  });

  it('reads the authenticated catalog as text and retains an exact int64 CLP amount', async () => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    const runtime = TestBed.inject(ApiRuntimeConfig);
    runtime.apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(ExploreApi);
    const http = TestBed.inject(HttpTestingController);
    const resultPromise = api.search('bearer-synthetic', {
      min_total_clp: '9007199254740993',
      start_at: '2026-11-01T00:00:00Z',
      end_at: '2026-11-02T00:00:00Z',
      latitude: '-33.45',
      longitude: '-70.66',
      radius_km: '3',
    });
    const request = http.expectOne(
      (req) => req.url === 'http://api.test/api/v1/local/booking-trial/catalog',
    );
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-synthetic');
    expect(request.request.params.get('min_total_clp')).toBe('9007199254740993');
    expect(request.request.params.get('start_at')).toBe('2026-11-01T00:00:00Z');
    expect(request.request.params.get('latitude')).toBe('-33.45');
    expect(request.request.params.get('longitude')).toBe('-70.66');
    expect(request.request.params.get('radius_km')).toBe('3');
    request.flush(
      '{"safety_notice":"ENSAYO LOCAL — SIN COBRO REAL","data":{"items":[{"space_id":"space-1","category_code":"oficina","category_name":"Oficina","title":"Sala","description":"","area_m2":12,"rate_unit":"hora","base_price_clp":9007199254740993,"currency":"CLP","time_zone":"America/Santiago","profile_version":1,"profile":{},"attributes":{}}]}}',
    );
    const result = (await resultPromise) as CatalogEnvelope;
    expect(result.data.items[0].base_price_clp).toBe('9007199254740993');
    http.verify();
    TestBed.resetTestingModule();
  });
});
