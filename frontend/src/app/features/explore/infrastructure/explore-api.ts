import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { M04Local, M06Local } from '../../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import type { SpaceAttributeProfile, SpaceCategoryList } from '../../../core/contracts/models';
import { parseExactClpJson } from '../../../core/exact-json';
import {
  AvailabilityOptions,
  BusinessClock,
  CatalogEnvelope,
  CatalogFilters,
  CatalogItem,
  ExploreApiError,
  ExplorePort,
  QuoteEnvelope,
  PublicCatalogFilters, PublicCategory, PublicSpace, PublicSpacePage,
} from '../domain/ports/explore-port';
export { parseExactClpJson } from '../../../core/exact-json';

@Injectable({ providedIn: 'root' })
export class ExploreApi extends ExplorePort {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly categoriesClient = inject(M04Local);
  private readonly bookingClient = inject(M06Local);

  async publicSearch(filters: PublicCatalogFilters): Promise<PublicSpacePage> {
    const values: Record<string,string|number> = {};
    for (const key of ['category_code','rate_unit','min_unit_price_clp','max_unit_price_clp','sort','page_size','cursor'] as const) {
      const value=filters[key]; if(value!==undefined&&value!=='')values[key]=value;
    }
    return this.publicRaw<PublicSpacePage>('/public/spaces',new HttpParams({fromObject:values}));
  }
  async publicDetail(spaceId:string):Promise<PublicSpace>{return (await this.publicRaw<{data:PublicSpace}>(`/public/spaces/${encodeURIComponent(spaceId)}`)).data;}
  async publicCategories():Promise<PublicCategory[]>{return (await this.publicRaw<{items:PublicCategory[]}>('/public/space-categories')).items;}

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
    if (result.error || !result.data)
      throw new ExploreApiError('No fue posible cargar las categorías.', result.response?.status);
    return result.data as SpaceCategoryList;
  }

  async profile(token: string, categoryCode: string): Promise<SpaceAttributeProfile> {
    const result = await this.categoriesClient.getSpaceAttributeProfile({
      path: { categoryCode },
      headers: { Authorization: `Bearer ${token}` },
    });
    if (result.error || !result.data)
      throw new ExploreApiError(
        'No fue posible cargar el perfil de categoría.',
        result.response?.status,
      );
    return result.data as SpaceAttributeProfile;
  }

  async profileVersion(token: string, categoryCode: string): Promise<number> {
    return (await this.profile(token, categoryCode)).schema_version;
  }

  async businessClock(token: string): Promise<BusinessClock> {
    const result = await this.bookingClient.getLocalBusinessClock({
      headers: { Authorization: `Bearer ${token}` },
    });
    if (result.error || !result.data)
      throw new ExploreApiError('No fue posible consultar la hora de Go.', result.response?.status);
    return result.data as BusinessClock;
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

  private async publicRaw<T>(path:string,params?:HttpParams):Promise<T>{
    const baseUrl=this.runtime.apiBaseUrl();if(!baseUrl)throw new Error('La API todavía no está configurada.');
    const text=await firstValueFrom(this.http.get(`${baseUrl}${path}`,{params,responseType:'text'}));
    return parseExactClpJson<T>(text);
  }
}

export {
  ExploreApiError,
  type AvailabilityInterval,
  type AvailabilityOptions,
  type BusinessClock,
  type BookingQuote,
  type CatalogEnvelope,
  type CatalogFilters,
  type CatalogItem,
  type ClpAmount,
  type QuoteEnvelope,
} from '../domain/ports/explore-port';
