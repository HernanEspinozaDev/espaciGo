import { HttpClient, HttpHeaders, HttpResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { M02Local } from '../../../generated/api/sdk.gen';
import type {
  OwnIdentityExport,
  Profile,
  ProfileUpdate,
  RightsRequest,
  RightsRequestList,
} from '../../../generated/api/types.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';

export type PhotoMetadata = {
  id: string;
  fixture_code: 'synthetic-png-v1';
  mime_type: 'image/png';
  sha256: string;
  size_bytes: number;
  created_at: string;
};
export type SyntheticPayoutAccount = {
  id: string;
  adapter: string;
  reference: string;
  state: string;
  updated_at: string;
};

@Injectable({ providedIn: 'root' })
export class AccountPrivacyApi {
  private readonly m02 = inject(M02Local);
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);

  async profile(token: string): Promise<Profile> {
    return unwrap(await this.m02.getOwnProfile({ headers: auth(token) }));
  }

  async updateProfile(token: string, value: ProfileUpdate): Promise<Profile> {
    return unwrap(await this.m02.updateOwnProfile({ body: value, headers: auth(token) }));
  }

  async photoMetadata(token: string): Promise<PhotoMetadata> {
    return unwrap(await this.m02.getOwnSyntheticProfilePhoto({ headers: auth(token) }));
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
    return unwrap(
      await this.m02.requestDataRight({ body: { type }, headers: auth(token) }),
    );
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
    throw new AccountPrivacyApiError('Go no pudo completar la solicitud.', response.response?.status);
  }
  if (response.data === undefined || response.data === null) {
    throw new AccountPrivacyApiError('Go no devolvió el recurso solicitado.', response.response?.status);
  }
  return response.data;
}

export class AccountPrivacyApiError extends Error {
  constructor(message: string, readonly status?: number) {
    super(message);
    this.name = 'AccountPrivacyApiError';
  }
}

export function statusOf(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}
