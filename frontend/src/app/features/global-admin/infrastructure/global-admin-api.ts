import { HttpClient, HttpErrorResponse, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { GlobalAdministration } from '../../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import { SessionBoundary } from '../../../core/identity/domain/ports/session-boundary';
import type { Account, Overview, Page, AccountBlockUpdate, ProfileUpdate, ProfileUpdateResult, RoleUpdate, SessionRevocation, PublicationPage, PublicationStateUpdate, PublicationContentUpdate, PublicationRetireUpdate, AccountResourceKind, AccountResourcePage, PublicationPhoto, PublicationPhotoMutation, PublicationSchedule, PublicationScheduleUpdate } from '../domain/global-admin-models';

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
  private readonly sdk = inject(GlobalAdministration);
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

  async listAccountResources(token: string, id: string, kind: AccountResourceKind, cursor = ''): Promise<AccountResourcePage> {
    const params = new HttpParams().set('page_size', 25).set('cursor', cursor);
    const response = await this.get<{ data: AccountResourcePage }>(token, `/admin/global/accounts/${encodeURIComponent(id)}/resources/${kind}`, params);
    return response.data;
  }

  async accountResource(token: string, accountId: string, kind: AccountResourceKind, resourceId: string): Promise<Record<string, unknown>> {
    const response = await this.get<{ data: Record<string, unknown> }>(token, `/admin/global/accounts/${encodeURIComponent(accountId)}/resources/${kind}/${encodeURIComponent(resourceId)}`);
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

  async listPublications(token:string,filters:Record<string,string|number|undefined>):Promise<PublicationPage>{const visible=filters['publicly_visible'];const query={owner_id:String(filters['owner_id']??'')||undefined,category:String(filters['category']??'')||undefined,state:(String(filters['state']??'')||undefined) as 'activa'|'borrador'|'oculta'|'eliminada'|undefined,search:String(filters['search']??'')||undefined,publicly_visible:visible===''||visible===undefined?undefined:visible==='true',page_size:Number(filters['page_size']??25),cursor:String(filters['cursor']??'')||undefined};const response=await this.sdkRequest<{data:PublicationPage}>(token,this.sdk.listGlobalAdminPublications({query,headers:{Authorization:`Bearer ${token}`}}));return response.data;}

  async setPublicationState(token:string,id:string,input:PublicationStateUpdate):Promise<void>{await this.sdkRequest(token,this.sdk.changeGlobalAdminPublicationState({path:{publicationID:id},body:input,headers:{Authorization:`Bearer ${token}`}}));}
  async updatePublication(token:string,id:string,input:PublicationContentUpdate):Promise<void>{await this.sdkRequest(token,this.sdk.updateGlobalAdminPublicationContent({path:{publicationID:id},body:input,headers:{Authorization:`Bearer ${token}`}}));}

  async retirePublication(token: string, id: string, input: PublicationRetireUpdate): Promise<Record<string, unknown>> {
    const response = await this.sdkRequest<{data: Record<string, unknown>}>(token, this.sdk.retireGlobalAdminPublication({path:{publicationID:id},body:input,headers:{Authorization:`Bearer ${token}`}}));
    return response.data;
  }

  async publicationGallery(token:string,id:string,ownerId:string):Promise<PublicationPhoto[]>{
    return (await this.sdkRequest<{data:PublicationPhoto[]}>(token,this.sdk.listGlobalAdminPublicationGallery({path:{publicationID:id},query:{owner_id:ownerId},headers:{Authorization:`Bearer ${token}`}}))).data;
  }
  async setPublicationPhotoVisibility(token:string,id:string,photoId:string,input:PublicationPhotoMutation):Promise<void>{await this.sdkRequest(token,this.sdk.setGlobalAdminPublicationPhotoVisibility({path:{publicationID:id,photoID:photoId},body:{...input,enabled:input.enabled??false},headers:{Authorization:`Bearer ${token}`}}));}
  async selectPublicationCover(token:string,id:string,photoId:string,input:PublicationPhotoMutation):Promise<void>{await this.sdkRequest(token,this.sdk.selectGlobalAdminPublicationCover({path:{publicationID:id,photoID:photoId},body:{owner_id:input.owner_id,expected_updated_at:input.expected_updated_at,reason:input.reason},headers:{Authorization:`Bearer ${token}`}}));}
  async removePublicationPhoto(token:string,id:string,photoId:string,input:PublicationPhotoMutation):Promise<void>{await this.sdkRequest(token,this.sdk.removeGlobalAdminPublicationPhoto({path:{publicationID:id,photoID:photoId},body:{owner_id:input.owner_id,expected_updated_at:input.expected_updated_at,reason:input.reason},headers:{Authorization:`Bearer ${token}`}}));}
  async publicationSchedule(token:string,id:string,ownerId:string):Promise<PublicationSchedule>{return (await this.sdkRequest<{data:PublicationSchedule}>(token,this.sdk.getGlobalAdminPublicationSchedule({path:{publicationID:id},query:{owner_id:ownerId},headers:{Authorization:`Bearer ${token}`}}))).data;}
  async savePublicationSchedule(token:string,id:string,input:PublicationScheduleUpdate):Promise<void>{await this.sdkRequest(token,this.sdk.updateGlobalAdminPublicationSchedule({path:{publicationID:id},body:input,headers:{Authorization:`Bearer ${token}`}}));}

  private async sdkRequest<T=unknown>(token:string,request:Promise<unknown>):Promise<T>{
    if(!token)throw new Error('La sesión no está disponible.');
    try{const value=await request as {data?:T;error?:unknown;response?:{status?:number}};if(value.error!==undefined||value.data===undefined){const status=value.response?.status??0;if(status===401)this.identity.expireIfCurrent(token);throw new GlobalAdminApiError(status);}return value.data;}
    catch(error){if(error instanceof GlobalAdminApiError)throw error;throw new GlobalAdminApiError(0);}
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
