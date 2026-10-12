import { HttpClient, HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { client } from '../../../generated/api/client.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { IdentitySession } from '../../core/identity/identity-session';

export class AdminApiError extends Error {
  constructor(message: string, readonly status = 0) {
    super(message);
    this.name = 'AdminApiError';
  }
}

type Attempt = { key: string; body: unknown };

/** Transport for the generated OpenAPI client. Tokens are supplied per request and never retained. */
@Injectable({ providedIn: 'root' })
export class AdminApi {
  private readonly identity = inject(IdentitySession);
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly attempts = new Map<string, Attempt>();

  async get<T>(token: string, path: string, query?: Record<string, string | number | undefined>): Promise<T> {
    return this.request<T>(token, path, 'GET', undefined, query);
  }

  async post<T>(
    token: string,
    accountId: string,
    path: string,
    body?: unknown,
    idempotencyScope?: string,
  ): Promise<T> {
    const attemptId = idempotencyScope ? `${accountId}:${idempotencyScope}` : undefined;
    const saved = attemptId ? this.attempts.get(attemptId) : undefined;
    if (saved && stableJson(saved.body) !== stableJson(body)) {
      throw new AdminApiError('Hay un intento incierto. Reintenta sin cambiar sus datos.', 409);
    }
    const attempt = saved ?? (attemptId ? { key: crypto.randomUUID(), body } : undefined);
    if (attemptId && attempt) this.attempts.set(attemptId, attempt);
    try {
      const result = await this.request<T>(token, path, 'POST', attempt?.body ?? body, undefined, attempt?.key);
      if (attemptId && attempt && this.attempts.get(attemptId) === attempt) this.attempts.delete(attemptId);
      return result;
    } catch (error) {
      const status = error instanceof AdminApiError ? error.status : 0;
      if (attemptId && attempt && status >= 400 && status < 500 && status !== 408 && this.attempts.get(attemptId) === attempt) {
        this.attempts.delete(attemptId);
      }
      throw error;
    }
  }

  async delete(token: string, path: string): Promise<void> {
    await this.request<unknown>(token, path, 'DELETE');
  }

  async downloadEvidence(token: string, path: string): Promise<Blob> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new AdminApiError('La API todavía no está disponible.');
    try {
      return await firstValueFrom(this.http.get(`${base}${path}`, {
        headers: new HttpHeaders({ Authorization: `Bearer ${token}` }), responseType: 'blob',
      }));
    } catch (error) {
      const status = error instanceof HttpErrorResponse ? error.status : 0;
      if (status === 401) this.identity.expireIfCurrent(token);
      throw new AdminApiError(status === 403 ? 'Go negó el acceso a la evidencia.' : `Go respondió HTTP ${status || 'sin conexión'}.`, status);
    }
  }

  private async request<T>(
    token: string,
    path: string,
    method: 'GET' | 'POST' | 'DELETE',
    body?: unknown,
    query?: Record<string, string | number | undefined>,
    idempotencyKey?: string,
  ): Promise<T> {
    const headers: Record<string, string> = { Authorization: `Bearer ${token}` };
    if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
    const rawResult: unknown = await client.request<T, unknown, false>({
      method,
      url: path,
      headers,
      ...(body === undefined ? {} : { body }),
      ...(query ? { query } : {}),
    });
    const result = rawResult as { data?: T; error?: unknown; response?: { status: number } };
    if ('error' in result && result.error !== undefined) {
      const status = result.response?.status ?? 0;
      if (status === 401) this.identity.expireIfCurrent(token);
      const message = status === 403
        ? 'Go confirmó que esta cuenta no tiene permiso de administración.'
        : `Go respondió HTTP ${status || 'sin conexión'}.`;
      throw new AdminApiError(message, status);
    }
    return result.data as T;
  }
}

function stableJson(value: unknown): string {
  if (value === undefined) return '';
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableJson).join(',')}]`;
  return `{${Object.entries(value as Record<string, unknown>).sort(([a], [b]) => a.localeCompare(b)).map(([key, item]) => `${JSON.stringify(key)}:${stableJson(item)}`).join(',')}}`;
}
