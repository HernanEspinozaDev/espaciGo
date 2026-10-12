import { isPlatformBrowser } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  Component,
  DestroyRef,
  OnDestroy,
  OnInit,
  PLATFORM_ID,
  effect,
  inject,
  signal,
} from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import type {
  OwnIdentityExport,
  Profile,
  RightsRequest,
} from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import {
  AccountPrivacyApi,
  PhotoMetadata,
  SyntheticPayoutAccount,
  statusOf,
} from './account-privacy-api';

@Component({
  selector: 'app-account-privacy-page',
  imports: [RouterLink],
  templateUrl: './account-privacy-page.html',
  styleUrl: './account-privacy-page.scss',
})
export class AccountPrivacyPage implements OnInit, OnDestroy {
  private readonly api = inject(AccountPrivacyApi);
  private readonly identity = inject(IdentitySession);
  private readonly privateSession = inject(PrivateSessionState);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly platformId = inject(PLATFORM_ID);
  private readonly destroyRef = inject(DestroyRef);
  private generation = 0;
  private profileWriteRevision = 0;
  private ownerSession = this.identity.session();
  private ownerToken = this.identity.accessToken();
  private unregisterPrivateState!: () => void;
  private readonly blobUrls = new Set<string>();
  private readonly revokeTimers = new Set<ReturnType<typeof setTimeout>>();
  private readonly idempotencyKeys = new Map<string, string>();
  private destroyed = false;

  protected readonly session = this.identity.session;
  protected readonly profile = signal<Profile | null>(null);
  protected readonly profileReady = signal(false);
  protected readonly displayName = signal('');
  protected readonly phone = signal('');
  protected readonly photo = signal<PhotoMetadata | null>(null);
  protected readonly photoUrl = signal<string | null>(null);
  protected readonly payout = signal<SyntheticPayoutAccount | null>(null);
  protected readonly rights = signal<RightsRequest[]>([]);
  protected readonly busy = signal(false);
  protected readonly savingProfile = signal(false);
  protected readonly error = signal('');
  protected readonly notice = signal('');
  protected readonly exportNotice = signal('');

  constructor() {
    this.title.setTitle('Mi cuenta y privacidad | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => this.meta.removeTag("name='robots'"));
    this.unregisterPrivateState = this.privateSession.register(() => this.clearPrivateState());
    effect(() => {
      const session = this.identity.session();
      const token = this.identity.accessToken();
      if (session !== this.ownerSession || token !== this.ownerToken) {
        this.ownerSession = session;
        this.ownerToken = token;
        this.clearPrivateState();
        if (session && token) void this.load();
      }
    });
  }

  ngOnInit(): void {
    void this.load();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.generation++;
    this.unregisterPrivateState?.();
    this.revokeAllBlobUrls();
  }

  protected async load(): Promise<boolean> {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!token || !session) return false;
    const generation = ++this.generation;
    this.revokePhotoUrl();
    this.photo.set(null);
    this.profileReady.set(false);
    this.error.set('');
    this.busy.set(true);
    try {
      const [profile, photo, payout, rights] = await Promise.all([
        this.api.profile(token).catch((error) => (statusOf(error) === 404 ? null : Promise.reject(error))),
        this.api.photoMetadata(token).catch((error) => (statusOf(error) === 404 ? null : Promise.reject(error))),
        this.api.payoutAccount(token),
        this.api.rightsRequests(token),
      ]);
      if (!this.isCurrent(generation, token, session)) return false;
      this.profile.set(profile);
      this.profileReady.set(true);
      this.displayName.set(profile?.display_name ?? '');
      this.phone.set(profile?.phone ?? '');
      this.photo.set(photo);
      this.payout.set(payout);
      this.rights.set(rights.items);
      if (photo) {
        const content = await this.api.photoContent(token).catch((error) =>
          statusOf(error) === 404 ? null : Promise.reject(error),
        );
        if (!this.isCurrent(generation, token, session)) return false;
        if (content) this.setPhotoUrl(content);
        else this.photo.set(null);
      }
      return true;
    } catch (error) {
      if (this.handleError(error, token)) return false;
      if (this.isCurrent(generation, token, session)) this.error.set(messageFor(error));
      return false;
    } finally {
      if (this.isCurrent(generation, token, session)) this.busy.set(false);
    }
  }

  protected async saveProfile(event?: Event): Promise<void> {
    event?.preventDefault();
    const token = this.identity.accessToken();
    const session = this.identity.session();
    const display_name = this.displayName().trim();
    const phone = this.phone().trim();
    if (!token || !session) return;
    if (!display_name || display_name.length > 120 || (phone && !/^\d{9}$/.test(phone))) {
      this.error.set('Ingresa un nombre visible y un teléfono vacío o de nueve dígitos.');
      return;
    }
    const revision = ++this.profileWriteRevision;
    this.error.set('');
    this.notice.set('');
    this.savingProfile.set(true);
    try {
      const updated = await this.api.updateProfile(token, {
        display_name,
        ...(phone ? { phone } : { phone: '' }),
      });
      if (!this.isSessionCurrent(token, session) || revision !== this.profileWriteRevision) return;
      this.profile.set(updated);
      this.displayName.set(updated.display_name);
      this.phone.set(updated.phone ?? '');
      this.notice.set('Perfil guardado por Go.');
    } catch (error) {
      if (this.handleError(error, token)) return;
      if (this.isSessionCurrent(token, session) && revision === this.profileWriteRevision) this.error.set(messageFor(error));
    } finally {
      if (this.isSessionCurrent(token, session) && revision === this.profileWriteRevision) this.savingProfile.set(false);
    }
  }

  protected async createPhoto(): Promise<void> {
    await this.mutate('photo-create', (token) => this.api.generatePhoto(token, this.keyFor('photo-create')));
  }

  protected async removePhoto(): Promise<void> {
    await this.mutate('photo-remove', (token) => this.api.removePhoto(token, this.keyFor('photo-remove')));
  }

  protected async createPayout(): Promise<void> {
    await this.mutate('payout-create', (token) =>
      this.api.createPayoutAccount(token, this.keyFor('payout-create')),
    );
  }

  protected async removePayout(): Promise<void> {
    await this.mutate('payout-remove', (token) => this.api.revokePayoutAccount(token, this.keyFor('payout-remove')));
  }

  protected async requestRight(type: RightsRequest['kind']): Promise<void> {
    await this.mutate(`right-${type}`, (token) => this.api.requestRight(token, type));
  }

  protected async downloadJson(): Promise<void> {
    await this.download('json', async (token) => {
      const data = await this.api.identityExport(token);
      return new Blob([JSON.stringify(data, null, 2)], { type: 'application/json;charset=utf-8' });
    });
  }

  protected async downloadZip(): Promise<void> {
    await this.download('zip', async (token) => {
      const response = await this.api.archive(token);
      if (!response.body) throw new Error('Go devolvió un archivo ZIP vacío.');
      return response.body;
    });
  }

  protected formatDate(value: string): string {
    return new Intl.DateTimeFormat('es-CL', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
  }

  private async mutate(
    action: string,
    operation: (token: string) => Promise<unknown>,
  ): Promise<void> {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!token || !session) return;
    const generation = ++this.generation;
    this.error.set('');
    this.notice.set('');
    this.busy.set(true);
    try {
      if (action.startsWith('photo-')) {
        this.revokePhotoUrl();
        this.photo.set(null);
      }
      const result = await operation(token);
      if (!this.isCurrent(generation, token, session)) return;
      this.idempotencyKeys.delete(action);
      if (action === 'photo-create' || action === 'photo-remove') {
        const reloaded = await this.load();
        if (!reloaded || !this.isSessionCurrent(token, session)) return;
        this.notice.set(action === 'photo-create' ? 'Foto sintética privada generada por Go.' : 'Foto retirada. El estado se muestra sin reconstruir el archivo.');
      }
      if (action === 'payout-remove') this.payout.set(null);
      if (action === 'payout-create') {
        this.payout.set(result as SyntheticPayoutAccount);
        this.notice.set('Referencia ficticia creada por Go. No se programó ninguna transferencia.');
      }
      if (action.startsWith('right-')) {
        const created = result as RightsRequest;
        this.rights.update((items) => [created, ...items]);
        this.notice.set(
          created.kind === 'acceso'
            ? 'Solicitud de acceso recibida; Go la dejó en revisión.'
            : 'Solicitud de supresión recibida; Go la dejó en revisión. No se borraron datos.',
        );
        const refreshed = await this.api.rightsRequests(token);
        if (!this.isCurrent(generation, token, session)) return;
        this.rights.set(refreshed.items);
      }
    } catch (error) {
      if (this.handleError(error, token)) return;
      if (this.isCurrent(generation, token, session)) this.error.set(messageFor(error));
    } finally {
      if (this.isCurrent(generation, token, session)) this.busy.set(false);
    }
  }

  private async download(
    format: 'json' | 'zip',
    makeBlob: (token: string) => Promise<Blob>,
  ): Promise<void> {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    if (!token || !session || !isPlatformBrowser(this.platformId)) return;
    const generation = ++this.generation;
    this.error.set('');
    this.exportNotice.set('Preparando exportación privada…');
    this.busy.set(true);
    try {
      const blob = await makeBlob(token);
      if (!this.isCurrent(generation, token, session)) return;
      const filename = format === 'json' ? 'mis-datos-identidad.json' : 'mis-datos-espacigo.zip';
      this.triggerDownload(blob, filename);
      this.exportNotice.set(
        format === 'json'
          ? 'Descargado JSON de identidad_local_v1; alcance limitado indicado en el archivo.'
          : 'Descargado ZIP versionado con manifiesto, inclusiones y exclusiones de Go.',
      );
    } catch (error) {
      if (this.handleError(error, token)) return;
      if (this.isCurrent(generation, token, session)) {
        this.error.set(messageFor(error));
        this.exportNotice.set('');
      }
    } finally {
      if (this.isCurrent(generation, token, session)) this.busy.set(false);
    }
  }

  private triggerDownload(blob: Blob, filename: string): void {
    const url = this.createBlobUrl(blob);
    if (!url) throw new Error('El navegador no permite crear la descarga temporal.');
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    anchor.hidden = true;
    document.body.append(anchor);
    anchor.click();
    anchor.remove();
    const timer = setTimeout(() => {
      this.revokeBlobUrl(url);
      this.revokeTimers.delete(timer);
    }, 1500);
    this.revokeTimers.add(timer);
  }

  private setPhotoUrl(blob: Blob): void {
    this.revokePhotoUrl();
    const url = this.createBlobUrl(blob);
    if (url) this.photoUrl.set(url);
  }

  private createBlobUrl(blob: Blob): string | null {
    if (!isPlatformBrowser(this.platformId) || typeof URL.createObjectURL !== 'function') return null;
    const url = URL.createObjectURL(blob);
    this.blobUrls.add(url);
    return url;
  }

  private revokePhotoUrl(): void {
    const url = this.photoUrl();
    if (url) this.revokeBlobUrl(url);
    this.photoUrl.set(null);
  }

  private revokeBlobUrl(url: string): void {
    if (!this.blobUrls.delete(url)) return;
    if (typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(url);
  }

  private revokeAllBlobUrls(): void {
    for (const timer of this.revokeTimers) clearTimeout(timer);
    this.revokeTimers.clear();
    for (const url of this.blobUrls) {
      if (typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(url);
    }
    this.blobUrls.clear();
    this.photoUrl.set(null);
  }

  private clearPrivateState(): void {
    this.generation++;
    this.revokeAllBlobUrls();
    this.profile.set(null);
    this.profileReady.set(false);
    this.displayName.set('');
    this.phone.set('');
    this.photo.set(null);
    this.payout.set(null);
    this.rights.set([]);
    this.busy.set(false);
    this.savingProfile.set(false);
    this.error.set('');
    this.notice.set('');
    this.exportNotice.set('');
    this.idempotencyKeys.clear();
  }

  private keyFor(action: string): string {
    let key = this.idempotencyKeys.get(action);
    if (!key) {
      key = idempotencyKey();
      this.idempotencyKeys.set(action, key);
    }
    return key;
  }

  private handleError(error: unknown, token: string): boolean {
    if (statusOf(error) !== 401 && !(error instanceof HttpErrorResponse && error.status === 401)) return false;
    if (!this.identity.expireIfCurrent(token)) return false;
    this.clearPrivateState();
    this.error.set('Go cerró la sesión. Inicia sesión nuevamente.');
    return true;
  }

  private isCurrent(
    generation: number,
    token: string,
    session: ReturnType<IdentitySession['session']>,
  ): boolean {
    return (
      this.isSessionCurrent(token, session) &&
      generation === this.generation
    );
  }

  private isSessionCurrent(
    token: string,
    session: ReturnType<IdentitySession['session']>,
  ): boolean {
    return !this.destroyed && this.identity.accessToken() === token && this.identity.session() === session;
  }
}

function idempotencyKey(): string {
  return globalThis.crypto?.randomUUID?.() ?? `fe04-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function messageFor(error: unknown): string {
  const status = statusOf(error) ?? (error instanceof HttpErrorResponse ? error.status : undefined);
  if (status === 401) return 'Go cerró la sesión. Inicia sesión nuevamente.';
  if (status === 403) return 'Go confirmó que esta cuenta no tiene permiso para este recurso.';
  if (status === 404) return 'Go no encontró el recurso; puede haber sido retirado.';
  if (status === 409) return 'El recurso cambió. Actualiza la vista antes de reintentar.';
  if (status === 422) return 'Go rechazó los datos. Revisa los campos y la elegibilidad de la cuenta fake.';
  return error instanceof Error ? error.message : 'No fue posible completar la operación. Inténtalo nuevamente.';
}
