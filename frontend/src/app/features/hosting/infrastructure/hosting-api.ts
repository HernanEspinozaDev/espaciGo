import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { M04Local, M04M06Local, M06Local } from '../../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import type {
  AvailabilityConfigResponse,
  AvailabilityResult,
  LocalWeeklyHoursEnvelope,
  ManualBlock,
  ManualBlockList,
  PriceSimulation,
  PriceSimulationInput,
  PublicLocationList,
  PublishedSpaceContentInput,
  Rate,
  RateInput,
  SpaceAttributeProfile,
  SpaceCategoryList,
  SpaceDeletionReceipt,
  SpaceDraft,
  SpaceDraftInput,
  SpaceDraftList,
  SpacePublicationInput,
  SyntheticSpaceCover,
  SyntheticSpaceGalleryPhoto,
  SyntheticSpaceGalleryRemoval,
} from '../../../core/contracts/models';
import { parseExactClpJson, stringifyExactClpJson } from '../../../core/exact-json';
import { HostingApiError, HostingPort } from '../domain/ports/hosting-port';

const auth = (token: string) => ({ Authorization: `Bearer ${token}` });

@Injectable({ providedIn: 'root' })
export class HostingApi extends HostingPort {
  private readonly sdk = inject(M04Local);
  private readonly calendarSdk = inject(M04M06Local);
  private readonly weeklySdk = inject(M06Local);
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);

  async list(token: string): Promise<SpaceDraftList> {
    return this.raw(token, '/spaces', 'GET');
  }
  async categories(token: string): Promise<SpaceCategoryList> {
    return unwrap(await this.sdk.listSpaceCategories({ headers: auth(token) }));
  }
  async publicLocations(): Promise<PublicLocationList> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new HostingApiError('La API todavía no está configurada.');
    const text = await firstValueFrom(
      this.http.get(`${base}/public/locations`, { responseType: 'text' }),
    );
    return parseExactClpJson<PublicLocationList>(text);
  }
  async profile(
    token: string,
    categoryCode: string,
    version?: number,
  ): Promise<SpaceAttributeProfile> {
    const result = version
      ? await this.sdk.getVersionedSpaceAttributeProfile({
          path: { categoryCode, version },
          headers: auth(token),
        })
      : await this.sdk.getSpaceAttributeProfile({ path: { categoryCode }, headers: auth(token) });
    return unwrap(result);
  }
  async detail(token: string, spaceId: string): Promise<SpaceDraft> {
    return this.raw(token, `/spaces/${encodeURIComponent(spaceId)}`, 'GET');
  }
  async create(token: string, payload: SpaceDraftInput): Promise<SpaceDraft> {
    return this.raw(token, '/spaces', 'POST', payload);
  }
  async update(token: string, spaceId: string, payload: SpaceDraftInput): Promise<SpaceDraft> {
    return this.raw(token, `/spaces/${encodeURIComponent(spaceId)}`, 'PUT', payload);
  }
  async updatePublished(
    token: string,
    spaceId: string,
    payload: PublishedSpaceContentInput,
  ): Promise<SpaceDraft> {
    return this.raw(
      token,
      `/spaces/${encodeURIComponent(spaceId)}/publication-content`,
      'PUT',
      payload,
    );
  }
  async publication(
    token: string,
    spaceId: string,
    payload: SpacePublicationInput,
  ): Promise<SpaceDraft> {
    return this.raw(token, `/spaces/${encodeURIComponent(spaceId)}/publication`, 'PUT', payload);
  }
  async remove(token: string, spaceId: string): Promise<SpaceDeletionReceipt> {
    return unwrap(await this.sdk.deleteOwnPublication({ path: { spaceId }, headers: auth(token) }));
  }
  async gallery(
    token: string,
    spaceId: string,
  ): Promise<{ items: SyntheticSpaceGalleryPhoto[]; max_photos: number; synthetic: true }> {
    return unwrap(
      await this.sdk.listOwnSyntheticSpaceGallery({ path: { spaceId }, headers: auth(token) }),
    );
  }
  async addPhoto(token: string, spaceId: string, key: string): Promise<SyntheticSpaceGalleryPhoto> {
    const result = unwrap<{ photo: SyntheticSpaceGalleryPhoto }>(
      await this.sdk.createSyntheticOwnSpaceGalleryPhoto({
        path: { spaceId },
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
    return result.photo;
  }
  async cover(token: string, spaceId: string, photoId: string): Promise<SyntheticSpaceCover> {
    return unwrap(
      await this.sdk.selectOwnSyntheticSpaceCover({
        path: { spaceId },
        body: { photo_id: photoId },
        headers: auth(token),
      }),
    );
  }
  async removePhoto(
    token: string,
    spaceId: string,
    photoId: string,
  ): Promise<SyntheticSpaceGalleryRemoval> {
    return unwrap(
      await this.sdk.removeOwnSyntheticSpaceGalleryPhoto({
        path: { spaceId, photoId },
        headers: auth(token),
      }),
    );
  }
  async photoContent(token: string, spaceId: string, photoId: string): Promise<Blob> {
    return this.blob(
      token,
      `/spaces/${encodeURIComponent(spaceId)}/gallery/${encodeURIComponent(photoId)}/content`,
    );
  }
  async timeZone(token: string, spaceId: string): Promise<AvailabilityConfigResponse> {
    return unwrap(
      await this.calendarSdk.getOwnDraftTimeZone({ path: { spaceId }, headers: auth(token) }),
    );
  }
  async configureTimeZone(
    token: string,
    spaceId: string,
    time_zone: string,
  ): Promise<AvailabilityConfigResponse> {
    return unwrap(
      await this.calendarSdk.configureOwnDraftAvailability({
        path: { spaceId },
        body: { time_zone },
        headers: auth(token),
      }),
    );
  }
  async checkAvailability(
    token: string,
    spaceId: string,
    from: string,
    to: string,
  ): Promise<AvailabilityResult> {
    return unwrap(
      await this.calendarSdk.queryOwnDraftAvailability({
        path: { spaceId },
        query: { from, to },
        headers: auth(token),
      }),
    );
  }
  async tariff(token: string, spaceId: string): Promise<Rate> {
    return this.raw(token, `/spaces/${encodeURIComponent(spaceId)}/tariff`, 'GET');
  }
  async updateTariff(token: string, spaceId: string, payload: RateInput): Promise<Rate> {
    return this.raw(token, `/spaces/${encodeURIComponent(spaceId)}/tariff`, 'PUT', payload);
  }
  async tariffVersions(token: string, spaceId: string): Promise<{ items: Rate[] }> {
    return this.raw(token, `/spaces/${encodeURIComponent(spaceId)}/tariffs`, 'GET');
  }
  async simulate(
    token: string,
    spaceId: string,
    payload: PriceSimulationInput,
  ): Promise<PriceSimulation> {
    return this.raw(
      token,
      `/spaces/${encodeURIComponent(spaceId)}/price-simulations`,
      'POST',
      payload,
    );
  }
  async blocks(token: string, spaceId: string, from: string, to: string): Promise<ManualBlockList> {
    return unwrap(
      await this.calendarSdk.listOwnDraftManualBlocks({
        path: { spaceId },
        query: { from, to },
        headers: auth(token),
      }),
    );
  }
  async addBlock(
    token: string,
    spaceId: string,
    payload: { start_at: string; end_at: string; reason: string },
  ): Promise<ManualBlock> {
    return unwrap(
      await this.calendarSdk.createOwnDraftManualBlock({
        path: { spaceId },
        body: payload,
        headers: auth(token),
      }),
    );
  }
  async removeBlock(token: string, spaceId: string, blockId: string): Promise<void> {
    const result = await this.calendarSdk.deleteOwnDraftManualBlock({
      path: { spaceId, blockId },
      headers: auth(token),
    });
    const response = result as { error?: unknown };
    if (response.error !== undefined)
      throw new HostingApiError(
        undefined,
        (result as { response?: { status?: number } }).response?.status,
      );
  }
  async weekly(token: string, spaceId: string): Promise<LocalWeeklyHoursEnvelope> {
    return unwrap(
      await this.weeklySdk.getOwnSyntheticHourlyWeeklyHours({
        path: { spaceId },
        headers: auth(token),
      }),
    );
  }
  async saveWeekly(
    token: string,
    spaceId: string,
    payload: { enabled: boolean; days: LocalWeeklyHoursEnvelope['data']['days'] },
  ): Promise<LocalWeeklyHoursEnvelope> {
    return unwrap(
      await this.weeklySdk.replaceOwnSyntheticHourlyWeeklyHours({
        path: { spaceId },
        body: payload,
        headers: auth(token),
      }),
    );
  }

  private async raw<T>(
    token: string,
    path: string,
    method: 'GET' | 'POST' | 'PUT',
    body?: unknown,
  ): Promise<T> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new HostingApiError('La API todavía no está configurada.');
    const headers = new HttpHeaders({
      ...auth(token),
      ...(body ? { 'Content-Type': 'application/json' } : {}),
    });
    const request =
      method === 'GET'
        ? this.http.get(`${base}${path}`, { headers, responseType: 'text' })
        : this.http.request(method, `${base}${path}`, {
            headers,
            body: stringifyExactClpJson(body),
            responseType: 'text',
          });
    const text = await firstValueFrom(request);
    return parseExactClpJson<T>(text);
  }
  private async blob(token: string, path: string): Promise<Blob> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new HostingApiError('La API todavía no está configurada.');
    return firstValueFrom(
      this.http.get(`${base}${path}`, {
        headers: new HttpHeaders(auth(token)),
        responseType: 'blob',
      }),
    );
  }
}

function unwrap<T>(result: unknown): T {
  const response = result as { data?: T; error?: unknown; response?: { status?: number } };
  if (response.error !== undefined || response.data === undefined || response.data === null) {
    throw new HostingApiError(undefined, response.response?.status);
  }
  return response.data;
}

export { HostingApiError } from '../domain/ports/hosting-port';
