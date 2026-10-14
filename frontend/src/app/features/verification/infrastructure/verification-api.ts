import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { M03Local } from '../../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import type {
  SyntheticEvidence,
  SyntheticEvidenceList,
  VerificationCase,
  VerificationEligibilityList,
  VerificationHistoryList,
  VerificationList,
} from '../../../core/contracts/models';
import { VerificationApiError, VerificationPort } from '../domain/ports/verification-port';

@Injectable({ providedIn: 'root' })
export class VerificationApi extends VerificationPort {
  private readonly sdk = inject(M03Local);
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);

  async list(token: string): Promise<VerificationList> {
    return unwrap(await this.sdk.listOwnVerifications({ headers: auth(token) }));
  }
  async eligibility(token: string): Promise<VerificationEligibilityList> {
    return unwrap(await this.sdk.listOwnSyntheticEligibility({ headers: auth(token) }));
  }
  async detail(token: string, id: string): Promise<VerificationCase> {
    return unwrap(await this.sdk.getOwnVerification({ path: { id }, headers: auth(token) }));
  }
  async history(token: string, id: string): Promise<VerificationHistoryList> {
    return unwrap(
      await this.sdk.listOwnVerificationHistory({ path: { id }, headers: auth(token) }),
    );
  }
  async evidence(token: string, id: string): Promise<SyntheticEvidence[]> {
    return unwrap<SyntheticEvidenceList>(
      await this.sdk.listOwnSyntheticEvidence({ path: { id }, headers: auth(token) }),
    ).items;
  }
  async create(token: string, type: 'kyc' | 'kyb', key: string): Promise<VerificationCase> {
    return unwrap(
      await this.sdk.startSyntheticVerification({
        body: { type },
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
  }
  async retry(
    token: string,
    id: string,
    correction_code:
      | 'fixture_vigente_actualizado'
      | 'antecedentes_fixture_actualizados'
      | 'inicio_actividades_fixture_actualizadas',
    key: string,
  ): Promise<VerificationCase> {
    return unwrap(
      await this.sdk.retryRejectedVerification({
        path: { id },
        body: { correction_code },
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
  }
  async generateEvidence(token: string, id: string, key: string): Promise<SyntheticEvidence> {
    return unwrap(
      await this.sdk.uploadSyntheticEvidence({
        path: { id },
        body: { fixture_code: 'synthetic-png-v1' },
        headers: { ...auth(token), 'Idempotency-Key': key },
      }),
    );
  }
  async content(token: string, id: string, evidenceId: string): Promise<Blob> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new Error('La API todavía no está configurada.');
    return firstValueFrom(
      this.http.get(
        `${base}/verifications/${encodeURIComponent(id)}/evidence/${encodeURIComponent(evidenceId)}`,
        { headers: new HttpHeaders(auth(token)), responseType: 'blob' },
      ),
    );
  }

  async deleteEvidence(token: string, id: string, evidenceId: string): Promise<void> {
    const result = await this.sdk.deleteOwnSyntheticEvidence({
      path: { id, evidenceId },
      headers: auth(token),
    });
    const response = result as {
      error?: unknown;
      response?: { status?: number };
    };
    if (response.error !== undefined || response.response?.status !== 204) {
      throw new VerificationApiError(response.response?.status);
    }
  }
}
function auth(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}
function unwrap<T>(result: unknown): T {
  const response = result as { data?: T; error?: unknown; response?: { status?: number } };
  if (response.error !== undefined) throw new VerificationApiError(response.response?.status);
  if (response.data === undefined || response.data === null)
    throw new VerificationApiError(response.response?.status);
  return response.data;
}

export { VerificationApiError, verificationStatus } from '../domain/ports/verification-port';
