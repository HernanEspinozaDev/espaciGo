import { Injectable,inject } from '@angular/core';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { GlobalAdminApi } from './global-admin-api';
@Injectable({providedIn:'root'}) export class GlobalAdminGateway extends GlobalAdminPort { private readonly api=inject(GlobalAdminApi);overview(token:string){return this.api.overview(token)}list(token:string,filters:Record<string,string|number|undefined>){return this.api.list(token,filters)}account(token:string,id:string){return this.api.account(token,id)} }
