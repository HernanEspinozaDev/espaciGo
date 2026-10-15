import { HttpClient, HttpErrorResponse, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import { SessionBoundary } from '../../../core/identity/domain/ports/session-boundary';
import type { Account, Overview, Page } from '../domain/global-admin-models';

export type { Account, Overview, Page } from '../domain/global-admin-models';

export class GlobalAdminApiError extends Error {
  constructor(readonly status: number) {
    super(
      status === 403
        ? 'Go confirmó que la sesión no tiene acceso al directorio.'
        : 'No se pudo cargar el directorio de cuentas.',
    );
  }
}

@Injectable({ providedIn: 'root' })
export class GlobalAdminApi {
  private readonly http = inject(HttpClient);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly identity = inject(SessionBoundary);

  async overview(token: string): Promise<Overview> {
    const response = await this.get<{ data: Overview }>(token, '/admin/global/overview');
    return response.data;
  }

  async list(
    token: string,
    filters: Record<string, string | number | undefined>,
  ): Promise<Page> {
    let params = new HttpParams();
    for (const [key, value] of Object.entries(filters)) {
      if (value !== undefined && value !== '') params = params.set(key, value);
    }
    const response = await this.get<{ data: Page }>(token, '/admin/global/accounts', params);
    return response.data;
  }

  async account(token: string, id: string): Promise<Account> {
    const response = await this.get<{ data: Account }>(
      token,
      `/admin/global/accounts/${encodeURIComponent(id)}`,
    );
    return response.data;
  }

  private async get<T>(token: string, path: string, params = new HttpParams()): Promise<T> {
    const baseUrl = this.runtime.apiBaseUrl();
    if (!token || !baseUrl) throw new Error('La sesión o la API no están disponibles.');

    try {
      return await firstValueFrom(
        this.http.get<T>(`${baseUrl}${path}`, {
          params,
          headers: new HttpHeaders({ Authorization: `Bearer ${token}` }),
          transferCache: false,
        }),
      );
    } catch (error) {
      if (error instanceof HttpErrorResponse && error.status === 401) {
        this.identity.expireIfCurrent(token);
      }
      throw new GlobalAdminApiError(error instanceof HttpErrorResponse ? error.status : 0);
    }
  }
}
