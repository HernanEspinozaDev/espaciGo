import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { M04Local } from '../../../generated/api/sdk.gen';
import type { SpaceAttributeProfile, SpaceCategoryList } from '../../../generated/api/types.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';

export type ClpAmount = string;
export type CatalogItem = {
  space_id: string;
  category_code: string;
  category_name: string;
  title: string;
  description: string;
  area_m2: number;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price_clp: ClpAmount;
  currency: 'CLP';
  time_zone: string;
  profile_version: number;
  profile: Record<string, unknown>;
  attributes: Record<string, unknown>;
  available?: boolean;
  estimated_total_clp?: ClpAmount;
  distance_km?: number;
  distance_kind?: 'direct';
  map_position?: { x: number; y: number };
};
export type CatalogEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: { items: CatalogItem[]; next_cursor?: string };
};
export type AvailabilityInterval = { start_at: string; end_at: string };
export type AvailabilityOptions = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: {
    space_id: string;
    time_zone: string;
    rate_unit: CatalogItem['rate_unit'];
    items: AvailabilityInterval[];
  };
};
export type BookingQuote = {
  id: string;
  space_id: string;
  rate_version: number;
  rate_unit: CatalogItem['rate_unit'];
  unit_price_clp: ClpAmount;
  currency: 'CLP';
  units: number;
  subtotal_clp: ClpAmount;
  start_at: string;
  end_at: string;
  time_zone: string;
  conditions: string;
  category_code: string;
  profile_version: number;
  profile_values: Record<string, unknown>;
  cancellation_policy_version: string;
  guarantee_policy_version: string;
  guarantee_currency: 'CLP';
  guarantee_expected_clp: ClpAmount;
  settlement_policy_version: string;
  settlement_commission_bps: number;
  created_at: string;
  expires_at: string;
};
export type QuoteEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: BookingQuote;
};
export type CatalogFilters = {
  q?: string;
  category_code?: string;
  min_area_m2?: string;
  start_at?: string;
  end_at?: string;
  min_total_clp?: string;
  max_total_clp?: string;
  latitude?: string;
  longitude?: string;
  radius_km?: string;
  attributes?: Record<string, unknown>;
  page_size?: number;
  cursor?: string;
};

@Injectable({ providedIn: 'root' })
export class ExploreApi {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly categoriesClient = inject(M04Local);

  async search(token: string, filters: CatalogFilters): Promise<CatalogEnvelope> {
    const paramsObject: Record<string, string | number> = {};
    for (const key of [
      'q',
      'category_code',
      'min_area_m2',
      'start_at',
      'end_at',
      'min_total_clp',
      'max_total_clp',
      'latitude',
      'longitude',
      'radius_km',
      'page_size',
      'cursor',
    ] as const) {
      const value = filters[key];
      if (value !== undefined && value !== '') paramsObject[key] = value;
    }
    if (filters.attributes && Object.keys(filters.attributes).length) {
      paramsObject['attributes'] = JSON.stringify(filters.attributes);
      if (filters.category_code)
        paramsObject['profile_version'] = await this.profileVersion(token, filters.category_code);
    }
    const params = new HttpParams({ fromObject: paramsObject });
    return this.raw<CatalogEnvelope>(
      token,
      '/local/booking-trial/catalog',
      'GET',
      undefined,
      params,
    );
  }

  async detail(token: string, spaceId: string): Promise<CatalogItem> {
    const envelope = await this.raw<{ safety_notice: string; data: CatalogItem }>(
      token,
      `/local/booking-trial/catalog/${encodeURIComponent(spaceId)}`,
      'GET',
    );
    return envelope.data;
  }

  async categories(token: string): Promise<SpaceCategoryList> {
    const result = await this.categoriesClient.listSpaceCategories({
      headers: { Authorization: `Bearer ${token}` },
    });
    if (result.error || !result.data) throw new Error('No fue posible cargar las categorías.');
    return result.data as SpaceCategoryList;
  }

  async profile(token: string, categoryCode: string): Promise<SpaceAttributeProfile> {
    const result = await this.categoriesClient.getSpaceAttributeProfile({
      path: { categoryCode },
      headers: { Authorization: `Bearer ${token}` },
    });
    if (result.error || !result.data)
      throw new Error('No fue posible cargar el perfil de categoría.');
    return result.data as SpaceAttributeProfile;
  }

  async profileVersion(token: string, categoryCode: string): Promise<number> {
    return (await this.profile(token, categoryCode)).schema_version;
  }

  async availability(
    token: string,
    spaceId: string,
    date: string,
    duration: number,
  ): Promise<AvailabilityOptions> {
    const params = new HttpParams().set('date', date).set('duration', String(duration));
    return this.raw<AvailabilityOptions>(
      token,
      `/local/booking-trial/catalog/${encodeURIComponent(spaceId)}/availability-options`,
      'GET',
      undefined,
      params,
    );
  }

  async quote(
    token: string,
    spaceId: string,
    startAt: string,
    endAt: string,
  ): Promise<QuoteEnvelope> {
    return this.raw<QuoteEnvelope>(token, '/local/booking-trial/quotes', 'POST', {
      space_id: spaceId,
      start_at: startAt,
      end_at: endAt,
    });
  }

  private async raw<T>(
    token: string,
    path: string,
    method: 'GET' | 'POST',
    body?: unknown,
    params?: HttpParams,
  ): Promise<T> {
    const baseUrl = this.runtime.apiBaseUrl();
    if (!baseUrl) throw new Error('La API todavía no está configurada.');
    const headers = new HttpHeaders({ Authorization: `Bearer ${token}` });
    const text =
      method === 'GET'
        ? await firstValueFrom(
            this.http.get(`${baseUrl}${path}`, { headers, params, responseType: 'text' }),
          )
        : await firstValueFrom(
            this.http.post(`${baseUrl}${path}`, body, { headers, responseType: 'text' }),
          );
    return parseExactClpJson<T>(text);
  }
}

const exactAmountKeys = new Set([
  'base_price_clp',
  'estimated_total_clp',
  'min_total_clp',
  'max_total_clp',
  'unit_price_clp',
  'subtotal_clp',
  'guarantee_expected_clp',
]);

/** Quotes CLP integer literals before JSON.parse, so int64 values are never rounded as JS numbers. */
export function parseExactClpJson<T>(source: string): T {
  let output = '';
  let index = 0;
  while (index < source.length) {
    if (source[index] !== '"') {
      output += source[index++];
      continue;
    }
    const stringStart = index++;
    while (index < source.length) {
      if (source[index] === '\\') index += 2;
      else if (source[index++] === '"') break;
    }
    const rawString = source.slice(stringStart, index);
    output += rawString;
    let cursor = index;
    while (/\s/.test(source[cursor] ?? '') && cursor < source.length) cursor++;
    if (source[cursor] !== ':') continue;
    const key = rawString.slice(1, -1);
    if (!exactAmountKeys.has(key)) continue;
    cursor++;
    while (/\s/.test(source[cursor] ?? '') && cursor < source.length) cursor++;
    const number = /^-?\d+/.exec(source.slice(cursor))?.[0];
    if (!number) throw new Error(`Literal CLP no entero en JSON: ${key}`);
    if (/[.eE]/.test(source[cursor + number.length] ?? '')) {
      throw new Error(`Notación no decimal para importe CLP: ${key}`);
    }
    output += source.slice(index, cursor);
    index = cursor;
    output += `"${number}"`;
    index += number.length;
  }
  return JSON.parse(output) as T;
}
