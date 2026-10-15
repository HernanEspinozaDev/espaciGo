import { HttpClient, HttpErrorResponse, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiRuntimeConfig } from '../../../core/api-runtime-config';
import { SessionBoundary } from '../../../core/identity/domain/ports/session-boundary';
import { Account,Overview,Page } from '../domain/global-admin-models';
export type { Account,Overview,Page } from '../domain/global-admin-models';
@Injectable({providedIn:'root'}) export class GlobalAdminApi {
  private readonly http=inject(HttpClient); private readonly runtime=inject(ApiRuntimeConfig); private readonly identity=inject(SessionBoundary);
  async overview(token:string):Promise<Overview>{return (await this.get<{data:Overview}>(token,'/admin/global/overview')).data}
  async list(token:string,filters:Record<string,string|number|undefined>):Promise<Page>{let params=new HttpParams();for(const [k,v] of Object.entries(filters))if(v!==undefined&&v!=='')params=params.set(k,v);return(await this.get<{data:Page}>(token,'/admin/global/accounts',params)).data}
  async account(token:string,id:string):Promise<Account>{return(await this.get<{data:Account}>(token,`/admin/global/accounts/${encodeURIComponent(id)}`)).data}
  private async get<T>(token:string,path:string,params=new HttpParams()):Promise<T>{const base=this.runtime.apiBaseUrl();if(!token||!base)throw Error('La sesión o la API no están disponibles.');try{return await firstValueFrom(this.http.get<T>(`${base}${path}`,{params,headers:new HttpHeaders({Authorization:`Bearer ${token}`}),transferCache:false}))}catch(e){if(e instanceof HttpErrorResponse&&e.status===401)this.identity.expireIfCurrent(token);throw Error(e instanceof HttpErrorResponse&&e.status===403?'Go confirmó que la sesión no tiene acceso al directorio.':'No se pudo cargar el directorio de cuentas.')}}
}
