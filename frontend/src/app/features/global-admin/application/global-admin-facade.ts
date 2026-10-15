import { DestroyRef, effect, inject, Injectable, signal, untracked } from '@angular/core';
import { Meta } from '@angular/platform-browser';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { Account, Overview, Page, AccountBlockUpdate, ProfileUpdate, RoleUpdate, SessionRevocation, PublicationPage, PublicationStateUpdate, PublicationContentUpdate, PublicationRetireUpdate, AccountResourceKind, AccountResourcePage, PublicationPhoto, PublicationPhotoMutation, PublicationSchedule, PublicationScheduleUpdate } from '../domain/global-admin-models';
import type { KPIReport, AuditFilter, AuditPage } from '../domain/global-admin-models';

export type DirectoryCriteria = Record<string, string | number | undefined>;

type RequestContext = {
  token: string;
  accountId: string;
  capabilities: string;
  privateGeneration: number;
};

export type MutationResult<T = void> =
  | { status: 'success'; value: T }
  | { status: 'error' | 'stale' };

type MutationContext = {
  id: number;
  request: RequestContext;
  accountId: string;
  selectionRevision: number;
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
  private selectionRevision = 0;
  private mutationRevision = 0;
  private resourceRevision = 0;
  private galleryRevision = 0;
  private scheduleRevision = 0;
  private reportRevision = 0;
  private auditRevision = 0;
  private identityContext = '';
  private unregisterPrivateState: (() => void) | null = null;

  readonly overview = signal<Overview | null>(null);
  readonly page = signal<Page | null>(null);
  readonly publications = signal<PublicationPage | null>(null);
  readonly loadingPublications = signal(false);
  readonly publicationGallery = signal<PublicationPhoto[] | null>(null);
  readonly loadingPublicationGallery = signal(false);
  readonly publicationSchedule = signal<PublicationSchedule|null>(null);
  readonly loadingPublicationSchedule = signal(false);
  readonly selected = signal<Account | null>(null);
  readonly loadingDirectory = signal(false);
  readonly loadingDetail = signal(false);
  readonly accountResources = signal<AccountResourcePage | null>(null);
  readonly accountResourceDetail = signal<Record<string, unknown> | null>(null);
  readonly loadingResources = signal(false);
  readonly error = signal('');
  readonly controlsResetRevision = signal(0);
  readonly saving = signal(false);
  readonly kpis = signal<KPIReport|null>(null);
  readonly auditPage = signal<AuditPage|null>(null);
  readonly loadingReport = signal(false);
  readonly loadingAudit = signal(false);

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
    ++this.selectionRevision;
    this.cancelMutation();
    this.clearAccountResources();
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

  closeAccount(): void {
    ++this.detailRevision;
    ++this.selectionRevision;
    this.cancelMutation();
    this.selected.set(null);
    this.clearAccountResources();
  }

  async loadAccountResources(kind: AccountResourceKind, cursor = ''): Promise<void> {
    const context = this.currentContext();
    const account = this.selected();
    const revision = ++this.resourceRevision;
    if (!context || !account || !this.identity.session()?.capabilities?.includes('accounts.read')) {
      this.clearAccountResources();
      return;
    }
    const selectionRevision = this.selectionRevision;
    this.loadingResources.set(true);
    this.error.set('');
    try {
      const page = await this.api.listAccountResources(context.token, account.account_id, kind, cursor);
      if (revision === this.resourceRevision && selectionRevision === this.selectionRevision && this.selected()?.account_id === account.account_id && this.hasCurrentContext(context)) {
        this.accountResources.set(page);
        this.accountResourceDetail.set(null);
      }
    } catch (error) {
      if (revision !== this.resourceRevision || selectionRevision !== this.selectionRevision || !this.hasCurrentContext(context)) return;
      if (statusOf(error) === 403) await this.revalidateDeniedRequest(context, 'Go rechazó el acceso a los recursos y revalidó tus permisos.');
      else this.error.set(messageOf(error));
    } finally {
      if (revision === this.resourceRevision && selectionRevision === this.selectionRevision && this.hasCurrentContext(context)) this.loadingResources.set(false);
    }
  }

  async openAccountResource(kind: AccountResourceKind, resourceId: string): Promise<void> {
    const context = this.currentContext();
    const account = this.selected();
    const revision = ++this.resourceRevision;
    if (!context || !account) return;
    const selectionRevision = this.selectionRevision;
    this.accountResourceDetail.set(null);
    this.loadingResources.set(true);
    try {
      const detail = await this.api.accountResource(context.token, account.account_id, kind, resourceId);
      if (revision === this.resourceRevision && selectionRevision === this.selectionRevision && this.selected()?.account_id === account.account_id && this.hasCurrentContext(context)) this.accountResourceDetail.set(detail);
    } catch (error) {
      if (revision === this.resourceRevision && selectionRevision === this.selectionRevision && this.hasCurrentContext(context)) this.error.set(messageOf(error));
    } finally {
      if (revision === this.resourceRevision && selectionRevision === this.selectionRevision && this.hasCurrentContext(context)) this.loadingResources.set(false);
    }
  }

  async loadPublications(filters:DirectoryCriteria,cursor=''):Promise<void>{const context=this.currentContext(),revision=++this.publicationRevision;this.publications.set(null);if(!context||!this.identity.session()?.capabilities?.includes('publications.list'))return;this.loadingPublications.set(true);this.error.set('');try{const page=await this.api.listPublications(context.token,{...filters,page_size:25,cursor:cursor||undefined});if(revision===this.publicationRevision&&this.hasCurrentContext(context))this.publications.set(page);}catch(error){if(revision===this.publicationRevision&&this.hasCurrentContext(context)){if(statusOf(error)===403)await this.revalidateDeniedRequest(context,'Go rechazó el inventario y se revalidaron tus permisos.');else this.error.set(messageOf(error));}}finally{if(revision===this.publicationRevision&&this.hasCurrentContext(context))this.loadingPublications.set(false);}}

  async loadKPIs(period:{from:string;until:string;time_zone:string}):Promise<void>{const context=this.currentContext(),rev=++this.reportRevision;this.kpis.set(null);if(!context||!this.identity.session()?.capabilities?.includes('kpi.read'))return;this.loadingReport.set(true);try{const result=await this.api.kpis(context.token,period);if(rev===this.reportRevision&&this.hasCurrentContext(context))this.kpis.set(result);}catch(e){if(rev===this.reportRevision&&this.hasCurrentContext(context)){if(statusOf(e)===403)await this.revalidateDeniedRequest(context,'Go rechazó el reporte y se revalidaron tus permisos.');else this.error.set(messageOf(e));}}finally{if(rev===this.reportRevision&&this.hasCurrentContext(context))this.loadingReport.set(false);}}
  async loadAudit(filters:AuditFilter):Promise<void>{const context=this.currentContext(),rev=++this.auditRevision;this.auditPage.set(null);if(!context||!this.identity.session()?.capabilities?.includes('audit.read'))return;this.loadingAudit.set(true);try{const result=await this.api.audit(context.token,filters);if(rev===this.auditRevision&&this.hasCurrentContext(context))this.auditPage.set(result);}catch(e){if(rev===this.auditRevision&&this.hasCurrentContext(context)){if(statusOf(e)===403)await this.revalidateDeniedRequest(context,'Go rechazó la consulta de auditoría y se revalidaron tus permisos.');else this.error.set(messageOf(e));}}finally{if(rev===this.auditRevision&&this.hasCurrentContext(context))this.loadingAudit.set(false);}}
  async exportAudit(filters:AuditFilter,format:'json'|'csv'):Promise<{blob:Blob;revision:number}|null>{const context=this.currentContext(),revision=++this.auditRevision;if(!context||!this.identity.session()?.capabilities?.includes('audit.export'))return null;try{const blob=await this.api.exportAudit(context.token,filters,format);return revision===this.auditRevision&&this.hasCurrentContext(context)?{blob,revision}:null;}catch(e){if(revision===this.auditRevision&&this.hasCurrentContext(context)){if(statusOf(e)===403)await this.revalidateDeniedRequest(context,'Go rechazó la exportación y se revalidaron tus permisos.');else this.error.set(messageOf(e));}return null;}}

  async setPublicationState(id: string, input: PublicationStateUpdate): Promise<MutationResult> {
    const operation = this.beginMutation();
    if (!operation) return { status: 'stale' };
    try {
      await this.api.setPublicationState(operation.request.token, id, input);
      return this.finishMutation(operation) ? { status: 'success', value: undefined } : { status: 'stale' };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó el cambio de publicación y revalidó tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  async updatePublication(id: string, input: PublicationContentUpdate): Promise<MutationResult> {
    const operation = this.beginMutation();
    if (!operation) return { status: 'stale' };
    try {
      await this.api.updatePublication(operation.request.token, id, input);
      return this.finishMutation(operation) ? { status: 'success', value: undefined } : { status: 'stale' };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó la edición y revalidó tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  async retirePublication(id: string, input: PublicationRetireUpdate): Promise<MutationResult<Record<string, unknown>>> {
    const operation = this.beginMutation();
    if (!operation) return { status: 'stale' };
    try {
      const receipt = await this.api.retirePublication(operation.request.token, id, input);
      return this.finishMutation(operation) ? { status: 'success', value: receipt } : { status: 'stale' };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó el retiro y revalidó tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  async loadPublicationGallery(item: import('../domain/global-admin-models').Publication): Promise<void> {
    const context=this.currentContext(),revision=++this.galleryRevision;this.publicationGallery.set(null);
    if(!context||!this.identity.session()?.capabilities?.includes('publications.read'))return;
    this.loadingPublicationGallery.set(true);
    try { const photos=await this.api.publicationGallery(context.token,item.publication_id,item.owner_id);if(revision===this.galleryRevision&&this.hasCurrentContext(context))this.publicationGallery.set(photos); }
    catch(error){if(revision===this.galleryRevision&&this.hasCurrentContext(context))this.error.set(messageOf(error));}
    finally{if(revision===this.galleryRevision&&this.hasCurrentContext(context))this.loadingPublicationGallery.set(false);}
  }

  clearPublicationGallery(): void {
    ++this.galleryRevision;
    this.publicationGallery.set(null);
    this.loadingPublicationGallery.set(false);
  }

  async setPublicationPhotoVisibility(item:import('../domain/global-admin-models').Publication,photo:PublicationPhoto,enabled:boolean):Promise<MutationResult>{
    return this.mutateGallery(item,photo,{owner_id:item.owner_id,enabled,expected_updated_at:item.updated_at,reason:'revision_administrativa'},'visibility');
  }
  async selectPublicationCover(item:import('../domain/global-admin-models').Publication,photo:PublicationPhoto):Promise<MutationResult>{
    return this.mutateGallery(item,photo,{owner_id:item.owner_id,expected_updated_at:item.updated_at,reason:'revision_administrativa'},'cover');
  }
  async removePublicationPhoto(item:import('../domain/global-admin-models').Publication,photo:PublicationPhoto):Promise<MutationResult>{
    return this.mutateGallery(item,photo,{owner_id:item.owner_id,expected_updated_at:item.updated_at,reason:'moderacion_contenido'},'remove');
  }

  private async mutateGallery(item:import('../domain/global-admin-models').Publication,photo:PublicationPhoto,input:PublicationPhotoMutation,action:'visibility'|'cover'|'remove'):Promise<MutationResult>{
    const operation=this.beginMutation();if(!operation)return {status:'stale'};
    try {
      if(action==='visibility')await this.api.setPublicationPhotoVisibility(operation.request.token,item.publication_id,photo.id,input);
      else if(action==='cover')await this.api.selectPublicationCover(operation.request.token,item.publication_id,photo.id,input);
      else await this.api.removePublicationPhoto(operation.request.token,item.publication_id,photo.id,input);
      if(!this.finishMutation(operation))return {status:'stale'};
      return {status:'success',value:undefined};
    } catch(error){await this.failMutation(operation,error,'Go rechazó la gestión de galería y revalidó tus permisos.');return this.isMutationCurrent(operation)?{status:'error'}:{status:'stale'};}
    finally{this.endMutation(operation);}
  }

  async loadPublicationSchedule(item:import('../domain/global-admin-models').Publication):Promise<void>{
    const context=this.currentContext(),revision=++this.scheduleRevision;this.publicationSchedule.set(null);
    if(!context||!this.identity.session()?.capabilities?.includes('publications.read'))return;
    this.loadingPublicationSchedule.set(true);
    try{const schedule=await this.api.publicationSchedule(context.token,item.publication_id,item.owner_id);if(revision===this.scheduleRevision&&this.hasCurrentContext(context))this.publicationSchedule.set(schedule);}
    catch(error){if(revision===this.scheduleRevision&&this.hasCurrentContext(context))this.error.set(messageOf(error));}
    finally{if(revision===this.scheduleRevision&&this.hasCurrentContext(context))this.loadingPublicationSchedule.set(false);}
  }
  clearPublicationSchedule(): void {
    ++this.scheduleRevision;
    this.publicationSchedule.set(null);
    this.loadingPublicationSchedule.set(false);
  }
  async savePublicationSchedule(item:import('../domain/global-admin-models').Publication,input:PublicationScheduleUpdate):Promise<MutationResult>{
    const operation=this.beginMutation();if(!operation)return {status:'stale'};
    try{await this.api.savePublicationSchedule(operation.request.token,item.publication_id,input);if(!this.finishMutation(operation))return {status:'stale'};return {status:'success',value:undefined};}
    catch(error){await this.failMutation(operation,error,'Go rechazó el horario y revalidó tus permisos.');return this.isMutationCurrent(operation)?{status:'error'}:{status:'stale'};}
    finally{this.endMutation(operation);}
  }

  async updateProfile(input: ProfileUpdate): Promise<MutationResult> {
    const operation = this.beginMutation(true);
    const account = this.selected();
    if (!operation || !account) return { status: 'stale' };
    try {
      const updated = await this.api.updateProfile(operation.request.token, account.account_id, input);
      if (!this.finishMutation(operation)) return { status: 'stale' };
      this.selected.update((current) => current?.account_id === account.account_id
        ? { ...current, display_name: updated.display_name, phone: updated.phone, profile_updated_at: updated.updated_at }
        : current);
      return { status: 'success', value: undefined };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó el cambio y se revalidaron tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  async updateRole(role: string, input: RoleUpdate): Promise<MutationResult> {
    const operation = this.beginMutation(true);
    const account = this.selected();
    if (!operation || !account) return { status: 'stale' };
    try {
      await this.api.updateRole(operation.request.token, account.account_id, role, input);
      if (!this.finishMutation(operation)) return { status: 'stale' };
      this.selected.update((current) => {
        if (current?.account_id !== account.account_id) return current;
        const roles = new Set(current.roles);
        input.granted ? roles.add(role) : roles.delete(role);
        return { ...current, roles: [...roles].sort() };
      });
      return { status: 'success', value: undefined };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó la gestión de roles y se revalidaron tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  async revokeSessions(input: SessionRevocation): Promise<MutationResult<number>> {
    const operation = this.beginMutation(true);
    const account = this.selected();
    if (!operation || !account) return { status: 'stale' };
    try {
      const revoked = await this.api.revokeSessions(operation.request.token, account.account_id, input);
      return this.finishMutation(operation) ? { status: 'success', value: revoked } : { status: 'stale' };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó la revocación y se revalidaron tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  async setBlocked(input: AccountBlockUpdate): Promise<MutationResult> {
    const operation = this.beginMutation(true);
    const account = this.selected();
    if (!operation || !account) return { status: 'stale' };
    try {
      await this.api.setBlocked(operation.request.token, account.account_id, input);
      if (!this.finishMutation(operation)) return { status: 'stale' };
      this.selected.update((current) => current?.account_id === account.account_id
        ? { ...current, administrative_blocked: input.blocked }
        : current);
      return { status: 'success', value: undefined };
    } catch (error) {
      await this.failMutation(operation, error, 'Go rechazó el cambio de bloqueo y se revalidaron tus permisos.');
      return this.isMutationCurrent(operation) ? { status: 'error' } : { status: 'stale' };
    } finally {
      this.endMutation(operation);
    }
  }

  private beginMutation(requireAccount = false): MutationContext | null {
    const request = this.currentContext();
    const account = this.selected();
    if (!request || this.saving() || (requireAccount && !account)) return null;
    const operation = {
      id: ++this.mutationRevision,
      request,
      accountId: account?.account_id ?? '',
      selectionRevision: this.selectionRevision,
    };
    this.saving.set(true);
    this.error.set('');
    return operation;
  }

  private isMutationCurrent(operation: MutationContext): boolean {
    return operation.id === this.mutationRevision
      && operation.selectionRevision === this.selectionRevision
      && (!operation.accountId || this.selected()?.account_id === operation.accountId)
      && this.hasCurrentContext(operation.request);
  }

  private finishMutation(operation: MutationContext): boolean {
    return this.isMutationCurrent(operation);
  }

  private async failMutation(operation: MutationContext, error: unknown, deniedMessage: string): Promise<void> {
    if (!this.isMutationCurrent(operation)) return;
    if (statusOf(error) === 403) {
      await this.revalidateDeniedRequest(operation.request, deniedMessage);
    } else {
      this.error.set(messageOf(error));
    }
  }

  private endMutation(operation: MutationContext): void {
    if (operation.id === this.mutationRevision) this.saving.set(false);
  }

  private cancelMutation(): void {
    ++this.mutationRevision;
    this.saving.set(false);
  }

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
    this.kpis.set(null);this.auditPage.set(null);
    this.page.set(null);
    this.selected.set(null);
    this.publicationGallery.set(null);
    this.publicationSchedule.set(null);
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
    this.cancelMutation();
    ++this.selectionRevision;
    this.clearAccountResources();
    ++this.directoryRevision;
    ++this.detailRevision;
    ++this.publicationRevision;
    ++this.galleryRevision;
    ++this.scheduleRevision;
    ++this.reportRevision;++this.auditRevision;
    this.controlsResetRevision.update((revision) => revision + 1);
    this.clearData();
    this.publications.set(null);
    this.publicationGallery.set(null);
    this.publicationSchedule.set(null);
    this.loadingPublicationGallery.set(false);
    this.loadingPublicationSchedule.set(false);
    this.loadingPublications.set(false);
    this.loadingDirectory.set(false);
    this.loadingReport.set(false);this.loadingAudit.set(false);
    this.loadingDetail.set(false);
    this.error.set('');
  }

  private clearAccountResources(): void {
    ++this.resourceRevision;
    this.accountResources.set(null);
    this.accountResourceDetail.set(null);
    this.loadingResources.set(false);
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
