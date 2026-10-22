import { HttpBackend, HttpClient, HttpEventType, HttpHeaders, HttpResponse, HttpXhrBackend } from '@angular/common/http';
import { isPlatformBrowser, XhrFactory } from '@angular/common';
import { InjectionToken, Injectable, PLATFORM_ID, inject } from '@angular/core';
import { filter, firstValueFrom, tap } from 'rxjs';
import { M02Local } from '../../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import type {
  OwnIdentityExport,
  Profile,
  ProfileUpdate,
  RightsRequest,
  RightsRequestList,
} from '../../../core/contracts/models';
import {
  AccountPrivacyApiError,
  AccountPrivacyPort,
  OwnProfilePhoto,
  SyntheticPayoutAccount,
  statusOf,
} from '../domain/ports/account-privacy-port';

export const ACCOUNT_UPLOAD_BACKEND = new InjectionToken<HttpBackend>('account upload backend', {
  providedIn: 'root',
  factory: () => isPlatformBrowser(inject(PLATFORM_ID))
    ? new HttpXhrBackend(inject(XhrFactory))
    : inject(HttpBackend),
});

@Injectable({ providedIn: 'root' })
export class AccountPrivacyApi extends AccountPrivacyPort {
  private readonly m02 = inject(M02Local);
  private readonly http = inject(HttpClient);
  private readonly uploadHttp = new HttpClient(inject(ACCOUNT_UPLOAD_BACKEND));
  private readonly runtime = inject(ApiRuntimeConfig);

  async profile(token: string): Promise<Profile> {
    return unwrap(await this.m02.getOwnProfile({ headers: auth(token) }));
  }

  async updateProfile(token: string, value: ProfileUpdate): Promise<Profile> {
    return unwrap(await this.m02.updateOwnProfile({ body: value, headers: auth(token) }));
  }

  async photoMetadata(token: string): Promise<OwnProfilePhoto> {
    return unwrap(await this.m02.getOwnSyntheticProfilePhoto({ headers: auth(token) }));
  }

  async uploadPhoto(token: string, file: File, key: string, onProgress: (percent: number | null) => void): Promise<void> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new Error('La API todavía no está configurada.');
    await firstValueFrom(this.uploadHttp.put(`${base}/profile/photo`, (() => {
      const form = new FormData();
      form.append('image', file, file.name);
      return form;
    })(), {
      headers: new HttpHeaders({ ...auth(token), 'Idempotency-Key': key }),
      observe: 'events', reportProgress: true, responseType: 'text',
    }).pipe(
      tap((event) => {
        if (event.type === HttpEventType.UploadProgress) onProgress(event.total ? Math.min(99, Math.round(event.loaded * 100 / event.total)) : null);
      }),
      filter((event): event is HttpResponse<string> => event instanceof HttpResponse),
    ));
    onProgress(100);
  }

  async photoContent(token: string): Promise<Blob> {
    return this.binary(token, '/profile/photo/content');
  }

  async generatePhoto(token: string, key: string): Promise<void> {
    unwrap(
      await this.m02.generateOwnSyntheticProfilePhoto({
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
  }

  async removePhoto(token: string, key: string): Promise<void> {
    unwrap(
      await this.m02.removeOwnSyntheticProfilePhoto({
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
  }

  async payoutAccount(token: string): Promise<SyntheticPayoutAccount | null> {
    try {
      return unwrap(await this.m02.getOwnSyntheticPayoutAccount({ headers: auth(token) }));
    } catch (error) {
      if (statusOf(error) === 404) return null;
      throw error;
    }
  }

  async createPayoutAccount(token: string, key: string): Promise<SyntheticPayoutAccount> {
    const result = unwrap<{ account: SyntheticPayoutAccount }>(
      await this.m02.createOrReplaceOwnSyntheticPayoutAccount({
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
    return result.account;
  }

  async revokePayoutAccount(token: string, key: string): Promise<void> {
    unwrap(
      await this.m02.revokeOwnSyntheticPayoutAccount({
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
  }

  async rightsRequests(token: string): Promise<RightsRequestList> {
    return unwrap(await this.m02.listOwnRightsRequests({ headers: auth(token) }));
  }

  async requestRight(token: string, type: RightsRequest['kind']): Promise<RightsRequest> {
    return unwrap(await this.m02.requestDataRight({ body: { type }, headers: auth(token) }));
  }

  async identityExport(token: string): Promise<OwnIdentityExport> {
    return unwrap(await this.m02.exportOwnIdentityData({ headers: auth(token) }));
  }

  async archive(token: string): Promise<HttpResponse<Blob>> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new Error('La API todavía no está configurada.');
    return firstValueFrom(
      this.http.get(`${base}/privacy/export/archive`, {
        headers: new HttpHeaders(auth(token)),
        observe: 'response',
        responseType: 'blob',
      }),
    );
  }

  private async binary(token: string, path: string): Promise<Blob> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new Error('La API todavía no está configurada.');
    return firstValueFrom(
      this.http.get(`${base}${path}`, {
        headers: new HttpHeaders(auth(token)),
        responseType: 'blob',
      }),
    );
  }
}

function auth(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

function unwrap<T>(result: unknown): T {
  const response = result as {
    data?: T;
    error?: unknown;
    response?: { status?: number };
  };
  if (response.error !== undefined) {
    throw new AccountPrivacyApiError(
      'Go no pudo completar la solicitud.',
      response.response?.status,
    );
  }
  if (response.data === undefined || response.data === null) {
    throw new AccountPrivacyApiError(
      'Go no devolvió el recurso solicitado.',
      response.response?.status,
    );
  }
  return response.data;
}

export {
  AccountPrivacyApiError,
  statusOf,
  type PhotoMetadata,
  type SyntheticPayoutAccount,
} from '../domain/ports/account-privacy-port';
