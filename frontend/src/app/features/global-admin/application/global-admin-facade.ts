import { DestroyRef, effect, inject, Injectable, signal, untracked } from '@angular/core';
import { Meta } from '@angular/platform-browser';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { Account, Overview, Page } from '../domain/global-admin-models';

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
  private requestRevision = 0;
  private identityContext = '';
  private unregisterPrivateState: (() => void) | null = null;

  readonly overview = signal<Overview | null>(null);
  readonly page = signal<Page | null>(null);
  readonly selected = signal<Account | null>(null);
  readonly loading = signal(false);
  readonly error = signal('');
  readonly controlsResetRevision = signal(0);

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
    const revision = ++this.requestRevision;
    if (!context || !this.canList()) {
      this.clearData();
      return;
    }

    this.loading.set(true);
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
      if (!this.isCurrent(revision, context)) return;
      this.overview.set(overview);
      this.page.set(page);
    } catch (error) {
      if (!this.isCurrent(revision, context)) return;
      if (statusOf(error) === 403) {
        await this.revalidateDeniedRequest(context, 'Go rechazó el acceso al directorio. Se revalidaron tus permisos.');
        return;
      }
      this.clearData();
      this.error.set(messageOf(error));
    } finally {
      if (this.isCurrent(revision, context)) this.loading.set(false);
    }
  }

  async open(accountId: string): Promise<void> {
    const context = this.currentContext();
    const revision = ++this.requestRevision;
    this.selected.set(null);
    if (!context || !this.canRead()) return;
    this.loading.set(true);
    this.error.set('');
    try {
      const account = await this.api.account(context.token, accountId);
      if (this.isCurrent(revision, context)) this.selected.set(account);
    } catch (error) {
      if (!this.isCurrent(revision, context)) return;
      if (statusOf(error) === 403) {
        await this.revalidateDeniedRequest(context, 'Go rechazó el acceso al detalle. Se revalidaron tus permisos.');
        return;
      }
      this.error.set(messageOf(error));
    } finally {
      if (this.isCurrent(revision, context)) this.loading.set(false);
    }
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

  private isCurrent(revision: number, context: RequestContext): boolean {
    const current = this.currentContext();
    return (
      revision === this.requestRevision &&
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
    ++this.requestRevision;
    this.controlsResetRevision.update((revision) => revision + 1);
    this.clearData();
    this.loading.set(false);
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
