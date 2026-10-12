import { isPlatformBrowser } from '@angular/common';
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
  SyntheticEvidence,
  VerificationCase,
  VerificationEligibilityList,
  VerificationHistoryList,
} from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { VerificationApi, verificationStatus } from './verification-api';

type SessionContext = {
  token: string;
  session: NonNullable<ReturnType<IdentitySession['session']>>;
};
type OperationContext = SessionContext & {
  id: number;
  name: 'create' | 'retry' | 'evidence';
  selectionRevision: number;
  selectionId: string | null;
  accountId: string;
};
type SyntheticFixture = { fixture_code: 'synthetic-png-v1' };
type WriteAttempt = { key: string; payload: unknown };

@Component({
  selector: 'app-verification-page',
  imports: [RouterLink],
  templateUrl: './verification-page.html',
  styleUrl: './verification-page.scss',
})
export class VerificationPage implements OnInit, OnDestroy {
  private readonly api = inject(VerificationApi);
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly platform = inject(PLATFORM_ID);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private loadGeneration = 0;
  private detailGeneration = 0;
  private operationGeneration = 0;
  private selectionRevision = 0;
  private ownerToken = this.identity.accessToken();
  private ownerSession = this.identity.session();
  private destroyed = false;
  private unregister!: () => void;
  private blobUrl: string | null = null;
  /** Contains only the exact payload and idempotency key, never an access token. */
  private readonly writeAttempts = new Map<string, WriteAttempt>();

  readonly session = this.identity.session;
  readonly cases = signal<VerificationCase[]>([]);
  readonly eligibility = signal<VerificationEligibilityList | null>(null);
  readonly selected = signal<VerificationCase | null>(null);
  readonly history = signal<VerificationHistoryList['items']>([]);
  readonly evidence = signal<SyntheticEvidence[]>([]);
  readonly evidenceUrl = signal<string | null>(null);
  readonly loading = signal(false);
  readonly detailLoading = signal(false);
  readonly operation = signal('');
  readonly error = signal('');
  readonly notice = signal('');
  readonly createType = signal<'kyc' | 'kyb'>('kyc');
  readonly correction = signal<
    | 'fixture_vigente_actualizado'
    | 'antecedentes_fixture_actualizados'
    | 'inicio_actividades_fixture_actualizadas'
  >('fixture_vigente_actualizado');

  constructor() {
    this.title.setTitle('Verificaciones | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => this.meta.removeTag("name='robots'"));
    this.unregister = this.privateState.register(() => this.clearPrivate());
    effect(() => {
      const token = this.identity.accessToken();
      const session = this.identity.session();
      if (token !== this.ownerToken || session !== this.ownerSession) {
        this.ownerToken = token;
        this.ownerSession = session;
        this.clearPrivate();
        if (token && session) void this.load();
      }
    });
  }

  ngOnInit(): void {
    if (isPlatformBrowser(this.platform)) void this.load();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.loadGeneration++;
    this.detailGeneration++;
    this.operationGeneration++;
    this.writeAttempts.clear();
    this.unregister?.();
    this.revoke();
  }

  async load(): Promise<boolean> {
    const context = this.sessionContext();
    if (!context || !isPlatformBrowser(this.platform)) return false;
    const generation = ++this.loadGeneration;
    const selectionAtStart = this.selectionRevision;
    this.loading.set(true);
    this.error.set('');
    try {
      const [list, eligibility] = await Promise.all([
        this.api.list(context.token),
        this.api.eligibility(context.token),
      ]);
      if (!this.readCurrent(context, generation)) return false;
      this.cases.set(list.items);
      this.eligibility.set(eligibility);

      // A user selection made while this read was pending wins over auto-selection.
      if (selectionAtStart === this.selectionRevision) {
        const currentId = this.selected()?.id;
        const next =
          list.items.find((item) => item.id === currentId) ??
          (currentId ? null : (list.items[0] ?? null));
        if (next) {
          const refresh =
            this.selected()?.id === next.id
              ? this.refreshSelected(context, this.selectionRevision)
              : this.select(next, true);
          const selectionRevision = this.selectionRevision;
          await refresh;
          if (!this.readCurrent(context, generation)) return false;
          if (selectionRevision !== this.selectionRevision) return false;
        } else if (!currentId) {
          this.clearDetail();
        }
      }
      return this.readCurrent(context, generation);
    } catch (error) {
      this.handleFailure(error, context.token, this.readCurrent(context, generation));
      return false;
    } finally {
      if (this.sessionStillCurrent(context) && generation === this.loadGeneration) {
        this.loading.set(false);
      }
    }
  }

  async select(item: VerificationCase, preserveNotice = false): Promise<boolean> {
    const context = this.sessionContext();
    if (!context || !this.cases().some((entry) => entry.id === item.id)) return false;
    const revision = ++this.selectionRevision;
    this.selected.set(item);
    if (!preserveNotice) this.notice.set('');
    this.error.set('');
    return this.readSelected(item.id, context, revision);
  }

  async createCase(): Promise<void> {
    const context = this.beginOperation('create');
    if (!context) return;
    const payload = { type: this.createType() } as const;
    const attempt = this.writeAttempt(context, 'create', 'new-case', payload);
    try {
      const created = await this.api.create(context.token, payload.type, attempt.key);
      this.settleAttempt(attempt.id);
      if (!this.operationCurrent(context) || !this.originSelectionCurrent(context)) return;
      this.notice.set('Go creó el caso y lo dejó en revisión.');

      const refreshed = await this.reloadForOperation(context);
      if (!this.operationCurrent(context) || !this.originSelectionCurrent(context) || !refreshed)
        return;
      const listed = this.cases().find((entry) => entry.id === created.id);
      if (!listed) return;

      const detail = this.select(listed, true);
      const detailRevision = this.selectionRevision;
      await detail;
      if (!this.operationCurrent(context) || !this.selectionStill(detailRevision, listed.id))
        return;
    } catch (error) {
      this.handleWriteFailure(error, context, attempt.id);
    } finally {
      this.finishOperation(context);
    }
  }

  async retryCase(): Promise<void> {
    const item = this.selected();
    if (!item || item.state !== 'rechazada') return;
    const context = this.beginOperation('retry');
    if (!context) return;
    const payload = { correction_code: this.correction() } as const;
    const attempt = this.writeAttempt(context, 'retry', item.id, payload);
    try {
      const retried = await this.api.retry(
        context.token,
        item.id,
        payload.correction_code,
        attempt.key,
      );
      this.settleAttempt(attempt.id);
      if (!this.operationCurrent(context) || !this.originSelectionCurrent(context)) return;
      this.notice.set('Go recibió la subsanación y creó un nuevo caso en revisión.');

      const refreshed = await this.reloadForOperation(context);
      if (!this.operationCurrent(context) || !this.originSelectionCurrent(context) || !refreshed)
        return;
      const listed = this.cases().find((entry) => entry.id === retried.id);
      if (!listed) return;

      const detail = this.select(listed, true);
      const detailRevision = this.selectionRevision;
      await detail;
      if (!this.operationCurrent(context) || !this.selectionStill(detailRevision, listed.id))
        return;
    } catch (error) {
      this.handleWriteFailure(error, context, attempt.id);
    } finally {
      this.finishOperation(context);
    }
  }

  async createEvidence(): Promise<void> {
    const item = this.selected();
    if (!item || item.state !== 'en_revision') return;
    const context = this.beginOperation('evidence');
    if (!context) return;
    const payload: SyntheticFixture = { fixture_code: 'synthetic-png-v1' };
    const attempt = this.writeAttempt(context, 'evidence', item.id, payload);
    try {
      await this.api.generateEvidence(context.token, item.id, attempt.key);
      this.settleAttempt(attempt.id);
      if (!this.operationCurrent(context) || !this.originSelectionCurrent(context)) return;
      const refresh = this.refreshSelected(context, context.selectionRevision);
      await refresh;
      if (!this.operationCurrent(context) || !this.originSelectionCurrent(context)) return;
      this.notice.set('Go generó y adjuntó el fixture PNG sintético.');
    } catch (error) {
      this.handleWriteFailure(error, context, attempt.id);
    } finally {
      this.finishOperation(context);
    }
  }

  stateLabel(state: VerificationCase['state']): string {
    return {
      en_revision: 'Pendiente de revisión',
      aprobada: 'Aprobada por Go',
      rechazada: 'Rechazada por Go',
      revocada: 'Revocada por Go',
    }[state];
  }

  typeLabel(type: 'kyc' | 'kyb'): string {
    return type === 'kyc' ? 'Persona (KYC)' : 'Organización (KYB)';
  }

  eligibilityFor(type: 'kyc' | 'kyb') {
    return this.eligibility()?.items.find((entry) => entry.type === type);
  }

  formatDate(value: string): string {
    return new Intl.DateTimeFormat('es-CL', { dateStyle: 'medium', timeStyle: 'short' }).format(
      new Date(value),
    );
  }

  private async reloadForOperation(context: OperationContext): Promise<boolean> {
    if (!this.operationCurrent(context) || !this.originSelectionCurrent(context)) return false;
    const generation = ++this.loadGeneration;
    this.loading.set(true);
    try {
      const [list, eligibility] = await Promise.all([
        this.api.list(context.token),
        this.api.eligibility(context.token),
      ]);
      if (
        !this.operationCurrent(context) ||
        !this.originSelectionCurrent(context) ||
        generation !== this.loadGeneration
      )
        return false;
      this.cases.set(list.items);
      this.eligibility.set(eligibility);
      return true;
    } catch (error) {
      this.handleFailure(
        error,
        context.token,
        this.operationCurrent(context) && this.originSelectionCurrent(context),
      );
      return false;
    } finally {
      if (this.operationCurrent(context) && generation === this.loadGeneration)
        this.loading.set(false);
    }
  }

  private async refreshSelected(context: SessionContext, revision: number): Promise<boolean> {
    const selected = this.selected();
    if (
      !selected ||
      !this.selectionStill(revision, selected.id) ||
      !this.sessionStillCurrent(context)
    )
      return false;
    return this.readSelected(selected.id, context, revision);
  }

  private async readSelected(
    id: string,
    context: SessionContext,
    revision: number,
  ): Promise<boolean> {
    const seq = ++this.detailGeneration;
    this.revoke();
    this.history.set([]);
    this.evidence.set([]);
    this.detailLoading.set(true);
    try {
      const [detail, history, evidence] = await Promise.all([
        this.api.detail(context.token, id),
        this.api.history(context.token, id),
        this.api.evidence(context.token, id),
      ]);
      if (!this.detailCurrent(seq, id, context, revision)) return false;
      this.selected.set(detail);
      this.history.set(history.items);
      this.evidence.set(evidence);
      const available = evidence.find((entry) => entry.file_available);
      if (available) {
        const loaded = await this.loadEvidence(available, seq, id, context, revision);
        if (!this.detailCurrent(seq, id, context, revision)) return false;
        if (!loaded && !this.sessionStillCurrent(context)) return false;
      }
      return this.detailCurrent(seq, id, context, revision);
    } catch (error) {
      this.handleFailure(error, context.token, this.detailCurrent(seq, id, context, revision));
      return false;
    } finally {
      if (this.detailCurrent(seq, id, context, revision)) this.detailLoading.set(false);
    }
  }

  private async loadEvidence(
    item: SyntheticEvidence,
    seq: number,
    selectedId: string,
    context: SessionContext,
    revision: number,
  ): Promise<boolean> {
    if (!item.file_available) return true;
    try {
      const blob = await this.api.content(context.token, item.verification_id, item.id);
      if (!this.detailCurrent(seq, selectedId, context, revision)) return false;
      this.revoke();
      this.blobUrl = URL.createObjectURL(blob);
      this.evidenceUrl.set(this.blobUrl);
      return true;
    } catch (error) {
      // A current token is expired even when the selection changed while this read was in flight.
      this.handleFailure(
        error,
        context.token,
        this.detailCurrent(seq, selectedId, context, revision),
      );
      if (
        verificationStatus(error) !== 401 &&
        this.detailCurrent(seq, selectedId, context, revision)
      ) {
        this.revoke();
        this.notice.set('La evidencia está retirada o ya no disponible.');
      }
      return false;
    }
  }

  private beginOperation(name: OperationContext['name']): OperationContext | null {
    const context = this.sessionContext();
    if (!context || this.operation()) return null;
    const id = ++this.operationGeneration;
    const operation: OperationContext = {
      ...context,
      id,
      name,
      selectionRevision: this.selectionRevision,
      selectionId: this.selected()?.id ?? null,
      accountId: context.session.account_id,
    };
    this.operation.set(name);
    this.error.set('');
    this.notice.set('');
    return operation;
  }

  private writeAttempt(
    context: OperationContext,
    action: OperationContext['name'],
    resource: string,
    payload: unknown,
  ): { id: string; key: string } {
    const fingerprint = JSON.stringify(payload);
    const id = JSON.stringify([context.accountId, action, resource, fingerprint]);
    let attempt = this.writeAttempts.get(id);
    if (!attempt) {
      attempt = { key: this.key(), payload: JSON.parse(fingerprint) as unknown };
      this.writeAttempts.set(id, attempt);
    }
    return { id, key: attempt.key };
  }

  private settleAttempt(id: string): void {
    this.writeAttempts.delete(id);
  }

  private handleWriteFailure(error: unknown, context: OperationContext, attemptId: string): void {
    const status = verificationStatus(error);
    // Network errors, timeouts and server errors are uncertain; retain the exact key/payload.
    if (status !== undefined && status >= 400 && status < 500 && status !== 408 && status !== 429) {
      this.settleAttempt(attemptId);
    }
    this.handleFailure(
      error,
      context.token,
      this.operationCurrent(context) && this.originSelectionCurrent(context),
    );
  }

  private handleFailure(error: unknown, token: string, canPresent: boolean): void {
    if (verificationStatus(error) === 401) {
      if (this.identity.expireIfCurrent(token)) this.clearPrivate();
      return;
    }
    if (canPresent)
      this.error.set(error instanceof Error ? error.message : 'Go no pudo completar la solicitud.');
  }

  private finishOperation(context: OperationContext): void {
    if (this.operationCurrent(context)) this.operation.set('');
  }

  private sessionContext(): SessionContext | null {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    return token && session ? { token, session } : null;
  }

  private sessionStillCurrent(context: SessionContext): boolean {
    return (
      !this.destroyed &&
      this.identity.accessToken() === context.token &&
      this.identity.session() === context.session
    );
  }

  private readCurrent(context: SessionContext, generation: number): boolean {
    return this.sessionStillCurrent(context) && generation === this.loadGeneration;
  }

  private detailCurrent(
    seq: number,
    id: string,
    context: SessionContext,
    revision: number,
  ): boolean {
    return (
      seq === this.detailGeneration &&
      this.sessionStillCurrent(context) &&
      revision === this.selectionRevision &&
      this.selected()?.id === id
    );
  }

  private operationCurrent(context: OperationContext): boolean {
    return (
      context.id === this.operationGeneration &&
      this.operation() === context.name &&
      this.sessionStillCurrent(context)
    );
  }

  private originSelectionCurrent(context: OperationContext): boolean {
    return (
      context.selectionRevision === this.selectionRevision &&
      context.selectionId === (this.selected()?.id ?? null)
    );
  }

  private selectionStill(revision: number, id: string): boolean {
    return revision === this.selectionRevision && this.selected()?.id === id;
  }

  private clearDetail(): void {
    this.detailGeneration++;
    this.selectionRevision++;
    this.revoke();
    this.selected.set(null);
    this.history.set([]);
    this.evidence.set([]);
  }

  private clearPrivate(): void {
    this.loadGeneration++;
    this.detailGeneration++;
    this.operationGeneration++;
    this.selectionRevision++;
    this.writeAttempts.clear();
    this.revoke();
    this.cases.set([]);
    this.eligibility.set(null);
    this.selected.set(null);
    this.history.set([]);
    this.evidence.set([]);
    this.loading.set(false);
    this.detailLoading.set(false);
    this.operation.set('');
    this.error.set('');
    this.notice.set('');
  }

  private revoke(): void {
    if (this.blobUrl) URL.revokeObjectURL(this.blobUrl);
    this.blobUrl = null;
    this.evidenceUrl.set(null);
  }

  private key(): string {
    return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
  }
}
