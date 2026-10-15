import { DestroyRef, effect, inject, Injectable, signal, untracked } from '@angular/core';
import { Meta } from '@angular/platform-browser';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { Account, Overview, Page, AccountBlockUpdate, ProfileUpdate, RoleUpdate, SessionRevocation, PublicationPage, PublicationStateUpdate, PublicationContentUpdate } from '../domain/global-admin-models';

export type DirectoryCriteria = Record<string, string | number | undefined>;

type RequestContext = {
  token: string;
  accountId: string;
  capabilities: string;
  privateGeneration: number;
};

@Injectable()
export class GlobalAdminFacade {
  private readonly api = inject(GlobalAdminPort);
  private readonly identity = inject(IdentitySession);
  private readonly privateSession = inject(PrivateSessionState);
  private readonly destroy = inject(DestroyRef);
  private readonly meta = inject(Meta);
  private directoryRevision = 0;
  private detailRevision = 0;
  private publicationRevision = 0;
  private identityContext = '';
  private unregisterPrivateState: (() => void) | null = null;

  readonly overview = signal<Overview | null>(null);
  readonly page = signal<Page | null>(null);
  readonly publications = signal<PublicationPage | null>(null);
  readonly loadingPublications = signal(false);
  readonly selected = signal<Account | null>(null);
  readonly loadingDirectory = signal(false);
  readonly loadingDetail = signal(false);
  readonly error = signal('');
  readonly controlsResetRevision = signal(0);
  readonly saving = signal(false);

  constructor() {
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.unregisterPrivateState = this.privateSession.register(() => this.invalidate());
    this.destroy.onDestroy(() => {
      this.invalidate();
      this.unregisterPrivateState?.();
      this.meta.removeTag("name='robots'");
    });

    effect(() => {
      const token = this.identity.accessToken();
      const session = this.identity.session();
      const capabilities = [...(session?.capabilities ?? [])].sort().join(',');
      const context = [
        token ?? '',
        session?.account_id ?? '',
        capabilities,
        this.privateSession.generation(),
      ].join('|');
      const changed = context !== this.identityContext;
      this.identityContext = context;

      if (changed) this.invalidate();
      if (changed && token && session?.capabilities?.includes('accounts.list')) {
        untracked(() => void this.load({}));
      }
    });
  }

  async load(criteria: DirectoryCriteria, cursor = ''): Promise<void> {
    const context = this.currentContext();
    const revision = ++this.directoryRevision;
    this.page.set(null);
    this.loadingDirectory.set(false);
    if (!context || !this.canList()) {
      this.clearData();
      return;
    }

    this.loadingDirectory.set(true);
    this.error.set('');
    try {
      const [overview, page] = await Promise.all([
        this.api.overview(context.token),
        this.api.list(context.token, {
          ...criteria,
          page_size: 25,
          cursor: cursor || undefined,
        }),
      ]);
      if (!this.isDirectoryCurrent(revision, context)) return;
      this.overview.set(overview);
      this.page.set(page);
    } catch (error) {
      if (!this.isDirectoryCurrent(revision, context)) return;
      if (statusOf(error) === 403) {
        await this.revalidateDeniedRequest(context, 'Go rechazó el acceso al directorio. Se revalidaron tus permisos.');
        return;
      }
      this.clearData();
      this.error.set(messageOf(error));
    } finally {
      if (this.isDirectoryCurrent(revision, context)) this.loadingDirectory.set(false);
    }
  }

  async open(accountId: string): Promise<void> {
    const context = this.currentContext();
    const revision = ++this.detailRevision;
    this.selected.set(null);
    if (!context || !this.canRead()) return;
    this.loadingDetail.set(true);
    this.error.set('');
    try {
      const account = await this.api.account(context.token, accountId);
      if (this.isDetailCurrent(revision, context)) this.selected.set(account);
    } catch (error) {
      if (!this.isDetailCurrent(revision, context)) return;
      if (statusOf(error) === 403) {
        await this.revalidateDeniedRequest(context, 'Go rechazó el acceso al detalle. Se revalidaron tus permisos.');
        return;
      }
      this.error.set(messageOf(error));
    } finally {
      if (this.isDetailCurrent(revision, context)) this.loadingDetail.set(false);
    }
  }

  async loadPublications(filters:DirectoryCriteria,cursor=''):Promise<void>{const context=this.currentContext(),revision=++this.publicationRevision;this.publications.set(null);if(!context||!this.identity.session()?.capabilities?.includes('publications.list'))return;this.loadingPublications.set(true);this.error.set('');try{const page=await this.api.listPublications(context.token,{...filters,page_size:25,cursor:cursor||undefined});if(revision===this.publicationRevision&&this.hasCurrentContext(context))this.publications.set(page);}catch(error){if(revision===this.publicationRevision&&this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó el inventario y se revalidaron tus permisos.');else this.error.set(messageOf(error));}}finally{if(revision===this.publicationRevision&&this.hasCurrentContext(context))this.loadingPublications.set(false);}}

  async setPublicationState(id:string,input:PublicationStateUpdate):Promise<void>{const context=this.currentContext();if(!context||this.saving())return;this.saving.set(true);this.error.set('');try{await this.api.setPublicationState(context.token,id,input);}catch(error){if(this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó el cambio de publicación y revalidó tus permisos.');else this.error.set(messageOf(error));}}finally{this.saving.set(false);}}

  async updatePublication(id:string,input:PublicationContentUpdate):Promise<void>{const context=this.currentContext();if(!context||this.saving())return;this.saving.set(true);this.error.set('');try{await this.api.updatePublication(context.token,id,input);}catch(error){if(this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó la edición y revalidó tus permisos.');else this.error.set(messageOf(error));}}finally{this.saving.set(false);}}

  async updateProfile(input: ProfileUpdate): Promise<void> {
    const context=this.currentContext(), account=this.selected();
    if(!context||!account||this.saving())return;
    this.saving.set(true);this.error.set('');
    try { const updated=await this.api.updateProfile(context.token,account.account_id,input);if(this.hasCurrentContext(context))this.selected.set({...account,display_name:updated.display_name,phone:updated.phone,profile_updated_at:updated.updated_at}); }
    catch(error){if(this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó el cambio y se revalidaron tus permisos.');else this.error.set(messageOf(error));}}
    finally{this.saving.set(false);}
  }

  async updateRole(role:string,input:RoleUpdate):Promise<void>{const context=this.currentContext(),account=this.selected();if(!context||!account||this.saving())return;this.saving.set(true);this.error.set('');try{await this.api.updateRole(context.token,account.account_id,role,input);if(this.hasCurrentContext(context)){const roles=new Set(account.roles);input.granted?roles.add(role):roles.delete(role);this.selected.set({...account,roles:[...roles].sort()});}}catch(error){if(this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó la gestión de roles y se revalidaron tus permisos.');else this.error.set(messageOf(error));}}finally{this.saving.set(false);}}

  async revokeSessions(input:SessionRevocation):Promise<number>{const context=this.currentContext(),account=this.selected();if(!context||!account||this.saving())return 0;this.saving.set(true);this.error.set('');try{return await this.api.revokeSessions(context.token,account.account_id,input);}catch(error){if(this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó la revocación y se revalidaron tus permisos.');else this.error.set(messageOf(error));}return 0;}finally{this.saving.set(false);}}

  async setBlocked(input:AccountBlockUpdate):Promise<void>{const context=this.currentContext(),account=this.selected();if(!context||!account||this.saving())return;this.saving.set(true);this.error.set('');try{await this.api.setBlocked(context.token,account.account_id,input);if(this.hasCurrentContext(context))this.selected.set({...account,administrative_blocked:input.blocked});}catch(error){if(this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó el cambio de bloqueo y se revalidaron tus permisos.');else this.error.set(messageOf(error));}}finally{this.saving.set(false);}}

  private currentContext(): RequestContext | null {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!token || !session?.account_id) return null;
    return {
      token,
      accountId: session.account_id,
      capabilities: [...(session.capabilities ?? [])].sort().join(','),
      privateGeneration: this.privateSession.generation(),
    };
  }

  private canList(): boolean {
    return this.identity.session()?.capabilities?.includes('accounts.list') ?? false;
  }

  private canRead(): boolean {
    return this.identity.session()?.capabilities?.includes('accounts.read') ?? false;
  }

  private isDirectoryCurrent(revision: number, context: RequestContext): boolean {
    return revision === this.directoryRevision && this.hasCurrentContext(context);
  }

  private isDetailCurrent(revision: number, context: RequestContext): boolean {
    return revision === this.detailRevision && this.hasCurrentContext(context);
  }

  private hasCurrentContext(context: RequestContext): boolean {
    const current = this.currentContext();
    return (
      current?.token === context.token &&
      current.accountId === context.accountId &&
      current.capabilities === context.capabilities &&
      current.privateGeneration === context.privateGeneration
    );
  }

  private clearData(): void {
    this.overview.set(null);
    this.page.set(null);
    this.selected.set(null);
  }

  private async revalidateDeniedRequest(context: RequestContext, message: string): Promise<void> {
    this.invalidate();
    try {
      await this.identity.refreshSessionIfCurrent(context.token);
    } catch {
      // A valid bearer may lose a capability; the session remains open.
    }
    const current = this.currentContext();
    if (current?.token === context.token && current.accountId === context.accountId) {
      this.error.set(message);
    }
  }

  private invalidate(): void {
    ++this.directoryRevision;
    ++this.detailRevision;
    ++this.publicationRevision;
    this.controlsResetRevision.update((revision) => revision + 1);
    this.clearData();
    this.publications.set(null);
    this.loadingPublications.set(false);
    this.loadingDirectory.set(false);
    this.loadingDetail.set(false);
    this.error.set('');
  }
}

function statusOf(error: unknown): number | undefined {
  return typeof error === 'object' && error !== null && 'status' in error && typeof error.status === 'number'
    ? error.status
    : undefined;
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : 'No se pudo cargar el directorio.';
}
