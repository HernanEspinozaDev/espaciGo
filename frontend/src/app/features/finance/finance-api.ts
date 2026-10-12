import { HttpClient, HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { parseExactClpJson } from '../../core/exact-json';

export class FinanceApiError extends Error {
  constructor(message: string, readonly status = 0) { super(message); this.name = 'FinanceApiError'; }
}

@Injectable({ providedIn: 'root' })
export class FinanceApi {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly uncertain = new Map<string, { key: string; body: unknown }>();

  async download(token: string, path: string): Promise<Blob> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new FinanceApiError('La API todavía no está disponible.');
    try {
      return await firstValueFrom(this.http.get(`${base}${path}`, {
        headers: new HttpHeaders({ Authorization: `Bearer ${token}` }),
        responseType: 'blob',
      }));
    } catch (error) {
      if (error instanceof HttpErrorResponse) throw new FinanceApiError(`Go respondió HTTP ${error.status || 'sin conexión'}.`, error.status);
      throw new FinanceApiError(error instanceof Error ? error.message : 'No se pudo descargar la evidencia.');
    }
  }

  async call<T>(token: string, path: string, method: 'GET' | 'POST', body?: unknown, attemptKey?: string): Promise<T> {
    const base = this.runtime.apiBaseUrl();
    if (!base) throw new FinanceApiError('La API todavía no está disponible.');
    const cacheId = attemptKey ? `${attemptKey}` : '';
    let saved = cacheId ? this.uncertain.get(cacheId) : undefined;
    if (saved && JSON.stringify(saved.body) !== JSON.stringify(body)) {
      throw new FinanceApiError('Hay una operación incierta; reintenta con el mismo formulario antes de cambiar sus datos.', 409);
    }
    if (cacheId && !saved) {
      saved = { key: crypto.randomUUID(), body };
      this.uncertain.set(cacheId, saved);
    }
    let headers = new HttpHeaders({ Authorization: `Bearer ${token}` });
    if (saved) headers = headers.set('Idempotency-Key', saved.key);
    try {
      const raw = await firstValueFrom(this.http.request(method, `${base}${path}`, {
        headers, body: saved?.body ?? body, responseType: 'text',
      }));
      if (cacheId) this.uncertain.delete(cacheId);
      return parseExactClpJson<T>(raw);
    } catch (error) {
      if (error instanceof HttpErrorResponse) {
        // A server error can happen after the write was committed; keep the
        // original key and payload until an authoritative retry/reconciliation.
        if (cacheId && error.status >= 400 && error.status < 500 && error.status !== 408) this.uncertain.delete(cacheId);
        throw new FinanceApiError(`Go respondió HTTP ${error.status || 'sin conexión'}.`, error.status);
      }
      throw new FinanceApiError(error instanceof Error ? error.message : 'No se pudo completar la solicitud.');
    }
  }

  clearAttempts(): void { this.uncertain.clear(); }
}
