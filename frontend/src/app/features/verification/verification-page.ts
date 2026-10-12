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
  private generation = 0;
  private detailGeneration = 0;
  private ownerToken = this.identity.accessToken();
  private ownerSession = this.identity.session();
  private destroyed = false;
  private unregister!: () => void;
  private blobUrl: string | null = null;
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
      const token = this.identity.accessToken(),
        session = this.identity.session();
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
    this.generation++;
    this.detailGeneration++;
    this.unregister?.();
    this.revoke();
  }
  async load(): Promise<void> {
    const { token, session } = this.owner();
    if (!token || !session || !isPlatformBrowser(this.platform)) return;
    const generation = ++this.generation;
    this.loading.set(true);
    this.error.set('');
    try {
      const [list, eligibility] = await Promise.all([
        this.api.list(token),
        this.api.eligibility(token),
      ]);
      if (!this.current(generation, token, session)) return;
      this.cases.set(list.items);
      this.eligibility.set(eligibility);
      const old = this.selected();
      const next = list.items.find((item) => item.id === old?.id) ?? list.items[0] ?? null;
      if (next) await this.select(next);
      else this.clearDetail();
    } catch (e) {
      if (this.current(generation, token, session)) this.fail(e, token);
    } finally {
      if (this.current(generation, token, session)) this.loading.set(false);
    }
  }
  async select(item: VerificationCase): Promise<void> {
    const { token, session } = this.owner();
    if (!token || !session) return;
    const seq = ++this.detailGeneration;
    this.revoke();
    this.selected.set(item);
    this.history.set([]);
    this.evidence.set([]);
    this.detailLoading.set(true);
    this.error.set('');
    try {
      const [detail, history, evidence] = await Promise.all([
        this.api.detail(token, item.id),
        this.api.history(token, item.id),
        this.api.evidence(token, item.id),
      ]);
      if (!this.detailCurrent(seq, token, session)) return;
      this.selected.set(detail);
      this.history.set(history.items);
      this.evidence.set(evidence);
      const available = evidence.find((entry) => entry.file_available);
      if (available) await this.loadEvidence(available, seq, token, session);
    } catch (e) {
      if (this.detailCurrent(seq, token, session)) this.fail(e, token);
    } finally {
      if (this.detailCurrent(seq, token, session)) this.detailLoading.set(false);
    }
  }
  async createCase(): Promise<void> {
    const { token, session } = this.owner();
    if (!token || !session) return;
    this.operation.set('create');
    this.error.set('');
    try {
      const item = await this.api.create(token, this.createType(), this.key());
      if (!this.still(token, session)) return;
      this.notice.set('Go creó el caso y lo dejó en revisión.');
      await this.load();
      await this.select(item);
    } catch (e) {
      if (this.still(token, session)) this.fail(e, token);
    } finally {
      if (this.still(token, session)) this.operation.set('');
    }
  }
  async retryCase(): Promise<void> {
    const item = this.selected(),
      { token, session } = this.owner();
    if (!item || item.state !== 'rechazada' || !token || !session) return;
    this.operation.set('retry');
    this.error.set('');
    try {
      const next = await this.api.retry(token, item.id, this.correction(), this.key());
      if (!this.still(token, session)) return;
      this.notice.set('Go recibió la subsanación y creó un nuevo caso en revisión.');
      await this.load();
      await this.select(next);
    } catch (e) {
      if (this.still(token, session)) this.fail(e, token);
    } finally {
      if (this.still(token, session)) this.operation.set('');
    }
  }
  async createEvidence(): Promise<void> {
    const item = this.selected(),
      { token, session } = this.owner();
    if (!item || item.state !== 'en_revision' || !token || !session) return;
    this.operation.set('evidence');
    this.error.set('');
    try {
      await this.api.generateEvidence(token, item.id, this.key());
      if (!this.still(token, session)) return;
      this.notice.set('Go generó y adjuntó el fixture PNG sintético.');
      await this.select(item);
    } catch (e) {
      if (this.still(token, session)) this.fail(e, token);
    } finally {
      if (this.still(token, session)) this.operation.set('');
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
  private async loadEvidence(
    item: SyntheticEvidence,
    seq: number,
    token: string,
    session: object,
  ): Promise<void> {
    if (!item.file_available) return;
    try {
      const blob = await this.api.content(token, item.verification_id, item.id);
      if (!this.detailCurrent(seq, token, session)) return;
      this.revoke();
      this.blobUrl = URL.createObjectURL(blob);
      this.evidenceUrl.set(this.blobUrl);
    } catch (e) {
      if (this.detailCurrent(seq, token, session)) {
        this.revoke();
        if (verificationStatus(e) === 401) this.identity.expireIfCurrent(token);
        else this.notice.set('La evidencia está retirada o ya no disponible.');
      }
    }
  }
  private fail(e: unknown, token: string): void {
    if (verificationStatus(e) === 401) {
      this.identity.expireIfCurrent(token);
      this.clearPrivate();
    } else this.error.set(e instanceof Error ? e.message : 'Go no pudo completar la solicitud.');
  }
  private owner() {
    return { token: this.identity.accessToken(), session: this.identity.session() };
  }
  private still(token: string, session: object): boolean {
    return (
      !this.destroyed &&
      this.identity.accessToken() === token &&
      this.identity.session() === session
    );
  }
  private current(g: number, token: string, session: object): boolean {
    return g === this.generation && this.still(token, session);
  }
  private detailCurrent(g: number, token: string, session: object): boolean {
    return g === this.detailGeneration && this.still(token, session);
  }
  private clearDetail(): void {
    this.detailGeneration++;
    this.revoke();
    this.selected.set(null);
    this.history.set([]);
    this.evidence.set([]);
  }
  private clearPrivate(): void {
    this.generation++;
    this.detailGeneration++;
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
