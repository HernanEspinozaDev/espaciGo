import { HttpClient, HttpErrorResponse, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import { SessionBoundary } from '../../../core/identity/domain/ports/session-boundary';
import type { Account, Overview, Page, AccountBlockUpdate, ProfileUpdate, ProfileUpdateResult, RoleUpdate, SessionRevocation, PublicationPage, PublicationStateUpdate, PublicationContentUpdate } from '../domain/global-admin-models';

export type { Account, Overview, Page } from '../domain/global-admin-models';

export class GlobalAdminApiError extends Error {
  constructor(readonly status: number) {
    super(
      status === 403
        ? 'Go confirmó que la sesión no tiene acceso al directorio.'
        : status === 409
          ? 'La cuenta cambió desde que abriste la ficha. Recarga antes de intentarlo otra vez.'
          : status === 428
            ? 'Inicia sesión nuevamente para autorizar esta acción sensible.'
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

  async updateProfile(token: string, id: string, input: ProfileUpdate): Promise<ProfileUpdateResult> {
    const response = await this.mutate<{ data: ProfileUpdateResult }>(token, `/admin/global/accounts/${encodeURIComponent(id)}/profile`, 'PATCH', input);
    return response.data;
  }

  async updateRole(token: string, id: string, role: string, input: RoleUpdate): Promise<void> {
    await this.mutate(token, `/admin/global/accounts/${encodeURIComponent(id)}/roles/${encodeURIComponent(role)}`, 'PUT', input);
  }

  async revokeSessions(token: string, id: string, input: SessionRevocation): Promise<number> {
    const response = await this.mutate<{ data: { revoked_sessions: number } }>(token, `/admin/global/accounts/${encodeURIComponent(id)}/sessions/revoke`, 'POST', input);
    return response.data.revoked_sessions;
  }

  async setBlocked(token:string,id:string,input:AccountBlockUpdate):Promise<void>{await this.mutate(token,`/admin/global/accounts/${encodeURIComponent(id)}/block`,'POST',input);}

  async listPublications(token:string,filters:Record<string,string|number|undefined>):Promise<PublicationPage>{let params=new HttpParams();for(const [key,value]of Object.entries(filters)){if(value!==undefined&&value!=='')params=params.set(key,value);}const response=await this.get<{data:PublicationPage}>(token,'/admin/global/publications',params);return response.data;}

  async setPublicationState(token:string,id:string,input:PublicationStateUpdate):Promise<void>{await this.mutate(token,`/admin/global/publications/${encodeURIComponent(id)}/state`,'POST',input);}
  async updatePublication(token:string,id:string,input:PublicationContentUpdate):Promise<void>{await this.mutate(token,`/admin/global/publications/${encodeURIComponent(id)}/content`,'PATCH',input);}

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

  private async mutate<T = unknown>(token: string, path: string, method: 'PATCH' | 'PUT' | 'POST', body: unknown): Promise<T> {
    const baseUrl = this.runtime.apiBaseUrl();
    if (!token || !baseUrl) throw new Error('La sesión o la API no están disponibles.');
    try {
      return await firstValueFrom(this.http.request<T>(method, `${baseUrl}${path}`, {
        body,
        headers: new HttpHeaders({ Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }),
        transferCache: false,
      }));
    } catch (error) {
      if (error instanceof HttpErrorResponse && error.status === 401) this.identity.expireIfCurrent(token);
      throw new GlobalAdminApiError(error instanceof HttpErrorResponse ? error.status : 0);
    }
  }
}
