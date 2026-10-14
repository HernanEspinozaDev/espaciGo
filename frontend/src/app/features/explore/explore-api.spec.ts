import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it, vi } from 'vitest';
import { M04Local, M06Local } from '../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { provideFrontendPorts } from '../../frontend.providers';
import {
  CatalogEnvelope,
  ExploreApi,
  ExploreApiError,
  parseExactClpJson,
} from './infrastructure/explore-api';

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

  it('preserves the nullable estimated total while retaining exact int64 precision', () => {
    expect(parseExactClpJson<{ estimated_total_clp: string | null }>(
      '{"estimated_total_clp":null}',
    )).toEqual({ estimated_total_clp: null });
    expect(parseExactClpJson<{ estimated_total_clp: string | null }>(
      '{"estimated_total_clp":9223372036854775807}',
    )).toEqual({ estimated_total_clp: '9223372036854775807' });
    expect(() => parseExactClpJson('{"base_price_clp":null}')).toThrow();
  });

  it('preserves guarantee null, absence, zero, and int64 literals without coercing other fields', () => {
    const nullable = parseExactClpJson<{
      guarantee_expected_clp: string | null;
      subtotal_clp: string;
      refund_amount_clp: string;
    }>(
      '{"guarantee_expected_clp":null,"subtotal_clp":0,"refund_amount_clp":9223372036854775807}',
    );
    expect(nullable).toEqual({
      guarantee_expected_clp: null,
      subtotal_clp: '0',
      refund_amount_clp: '9223372036854775807',
    });
    const absent = parseExactClpJson<{ guarantee_expected_clp?: string | null }>('{}');
    expect(Object.hasOwn(absent, 'guarantee_expected_clp')).toBe(false);
    expect(
      parseExactClpJson<{ guarantee_expected_clp: string | null }>(
        '{"guarantee_expected_clp":50000}',
      ),
    ).toEqual({ guarantee_expected_clp: '50000' });
    expect(() => parseExactClpJson('{"subtotal_clp":null}')).toThrow();
    expect(() => parseExactClpJson('{"guarantee_expected_clp":50000.5}')).toThrow();
    expect(() => parseExactClpJson('{"base_price_clp":1e3}')).toThrow();
  });

  it('loads public search through HttpClient when an estimate is null and keeps large CLP exact', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(ExploreApi);
    const http = TestBed.inject(HttpTestingController);
    const resultPromise = api.publicSearch({});
    const request = http.expectOne('http://api.test/api/v1/public/spaces');
    expect(request.request.method).toBe('GET');
    expect(request.request.headers.has('Authorization')).toBe(false);
    request.flush('{"items":[{"space_id":"space-1","base_price_clp":9007199254740993,"estimated_total_clp":null}]}');
    const result = await resultPromise;
    expect(result.items[0].base_price_clp).toBe('9007199254740993');
    expect(result.items[0].estimated_total_clp).toBeNull();
    http.verify();
    TestBed.resetTestingModule();
  });

  it('reads the authenticated catalog as text and retains an exact int64 CLP amount', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
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

  it('preserves SDK HTTP status for category and profile authorization failures', async () => {
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        {
          provide: M04Local,
          useValue: {
            listSpaceCategories: async () => ({ error: {}, response: { status: 401 } }),
            getSpaceAttributeProfile: async () => ({ error: {}, response: { status: 401 } }),
          },
        },
      ],
    });
    const api = TestBed.inject(ExploreApi);
    await expect(api.categories('expired')).rejects.toMatchObject({
      name: 'ExploreApiError',
      status: 401,
    } satisfies Partial<ExploreApiError>);
    await expect(api.profile('expired', 'oficina')).rejects.toMatchObject({
      name: 'ExploreApiError',
      status: 401,
    });
    TestBed.resetTestingModule();
  });

  it('reads Go business time through the generated authenticated operation', async () => {
    const getLocalBusinessClock = vi.fn().mockResolvedValue({
      data: { data: { now: '2026-10-13T05:00:00Z' } },
      response: { status: 200 },
    });
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        { provide: M06Local, useValue: { getLocalBusinessClock } },
      ],
    });
    const api = TestBed.inject(ExploreApi);
    await expect(api.businessClock('bearer-synthetic')).resolves.toEqual({
      data: { now: '2026-10-13T05:00:00Z' },
    });
    expect(getLocalBusinessClock).toHaveBeenCalledWith({
      headers: { Authorization: 'Bearer bearer-synthetic' },
    });
    TestBed.resetTestingModule();
  });
});

describe('public typed attribute query encoding',()=>{
  it('keeps legacy booleans and serializes ranges, enums, and enum-lists canonically',async()=>{
    TestBed.configureTestingModule({providers:[...provideFrontendPorts(),provideHttpClient(),provideHttpClientTesting()]});
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://api.test/api/v1');
    const api=TestBed.inject(ExploreApi),http=TestBed.inject(HttpTestingController);
    const result=api.publicSearch({category_code:'bodega',attributes:{wifi:{kind:'boolean',value:false},altura:{kind:'range',minimum:'1.25',maximum:'2.50'},uso:{kind:'enum',value:'privada'},equipo:{kind:'enum_list',values:['wifi','proyector']}}});
    const request=http.expectOne(req=>req.url==='http://api.test/api/v1/public/spaces');
    expect(request.request.params.getAll('attribute')).toEqual(['altura:gte:1.25','altura:lte:2.50','equipo:in:proyector,wifi','uso:eq:privada','wifi:false']);
    request.flush({items:[]});await result;http.verify();
  });
});
