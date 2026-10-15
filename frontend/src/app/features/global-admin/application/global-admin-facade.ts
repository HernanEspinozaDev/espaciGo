import { DestroyRef,inject,Injectable,signal } from '@angular/core';
import { Meta } from '@angular/platform-browser';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { Account,Overview,Page } from '../domain/global-admin-models';
@Injectable()
export class GlobalAdminFacade {
 private readonly api=inject(GlobalAdminPort);private readonly identity=inject(IdentitySession);private readonly destroy=inject(DestroyRef);private readonly meta=inject(Meta);private revision=0;
 readonly overview=signal<Overview|null>(null);readonly page=signal<Page|null>(null);readonly selected=signal<Account|null>(null);readonly loading=signal(false);readonly error=signal('');
 constructor(){this.meta.addTag({name:'robots',content:'noindex, nofollow'});this.destroy.onDestroy(()=>{++this.revision;this.clear();this.meta.removeTag("name='robots'")});void this.load({})}
 async load(f:Record<string,string|number|undefined>,cursor=''):Promise<void>{const token=this.identity.accessToken(),account=this.identity.session()?.account_id,revision=++this.revision;if(!token||!account||!this.identity.session()?.capabilities?.includes('accounts.list')){this.clear();return}this.loading.set(true);this.error.set('');try{const [o,p]=await Promise.all([this.overview()?Promise.resolve(this.overview()!):this.api.overview(token),this.api.list(token,{...f,page_size:25,cursor:cursor||undefined})]);if(revision!==this.revision||token!==this.identity.accessToken()||account!==this.identity.session()?.account_id)return;this.overview.set(o);this.page.set(p)}catch(e){if(revision===this.revision){this.clear();this.error.set(e instanceof Error?e.message:'No se pudo cargar el directorio.')}}finally{if(revision===this.revision)this.loading.set(false)}}
 async open(id:string):Promise<void>{const revision=++this.revision,token=this.identity.accessToken();this.selected.set(null);this.loading.set(true);try{if(!token)return;const account=await this.api.account(token,id);if(revision===this.revision&&token===this.identity.accessToken())this.selected.set(account)}catch(e){if(revision===this.revision)this.error.set(e instanceof Error?e.message:'Error de lectura')}finally{if(revision===this.revision)this.loading.set(false)}}
 private clear():void{this.overview.set(null);this.page.set(null);this.selected.set(null);this.loading.set(false)}
}
