import {
  backendOperation,
  type BackendOperation,
  type ReservationFinanceResource,
} from '../../../core/contracts/operations';
import { DestroyRef, effect, inject, Injectable, signal } from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { FinanceApiError, FinancePort } from '../domain/ports/finance-port';

type RecordValue = Record<string, unknown>;
const unwrap = (value: unknown): unknown => {
  if (typeof value === 'object' && value !== null && 'data' in value)
    return (value as RecordValue)['data'];
  return value;
};
const arr = (value: unknown): unknown[] => {
  const data = unwrap(value);
  if (Array.isArray(data)) return data;
  return typeof data === 'object' &&
    data !== null &&
    'items' in data &&
    Array.isArray((data as RecordValue)['items'])
    ? ((data as RecordValue)['items'] as unknown[])
    : [];
};
const field = (value: unknown, key: string): string =>
  typeof value === 'object' && value !== null ? String((value as RecordValue)[key] ?? '') : '';

@Injectable()
export class FinanceDisputesFacade {
  private readonly api = inject(FinancePort);
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly route = inject(ActivatedRoute);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  id = '';
  private owner = this.identity.session();
  private token = this.identity.accessToken();
  private generation = 0;
  private writeRevision = 0;
  private destroyed = false;
  private readonly unregister = this.privateState.register(() => this.clear());
  readonly admin = this.route.snapshot.data['mode'] === 'admin';
  readonly disputes = signal<unknown[]>([]);
  readonly adminReservationId = signal('');
  readonly claims = signal<unknown[]>([]);
  readonly claim = signal<unknown | null>(null);
  readonly selectedClaim = signal<unknown | null>(null);
  readonly financialDecision = signal<unknown | null>(null);
  readonly guarantee = signal<unknown | null>(null);
  readonly settlement = signal<unknown | null>(null);
  readonly receipt = signal<unknown | null>(null);
  readonly evidenceUrl = signal<string | null>(null);
  readonly description = signal('');
  readonly outcome = signal('sin_respuesta');
  readonly reasonCode = signal('evidencia_insuficiente');
  readonly decision = signal('rechazado');
  readonly guaranteeKind = signal('autorizacion');
  readonly guaranteeAmount = signal('50000');
  readonly deductionAmount = signal('0');
  readonly decisionEvidenceId = signal('');
  readonly busy = signal(false);
  readonly loading = signal(false);
  readonly notice = signal('ENSAYO LOCAL — SIN MOVIMIENTO DE DINERO REAL');
  readonly error = signal('');

  constructor() {
    this.title.setTitle('Disputas y finanzas | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
      this.generation++;
      this.revokeEvidence();
      this.unregister();
      this.meta.removeTag("name='robots'");
    });
    effect(() => {
      const session = this.identity.session();
      const token = this.identity.accessToken();
      if (session !== this.owner || token !== this.token) {
        this.owner = session;
        this.token = token;
        this.clear();
        if (session && token && (this.admin || this.id)) void this.load();
      }
    });
    this.route.paramMap.subscribe((params) => {
      const id = params.get('id') ?? '';
      if (id !== this.id) {
        this.id = id;
        this.clear();
        if (this.id) void this.load();
      }
    });
    if (this.admin && this.identity.session() && this.identity.accessToken()) void this.load();
  }

  async load(): Promise<void> {
    const token = this.identity.accessToken();
    const owner = this.identity.session();
    if (!token || !owner || (!this.admin && !this.id)) return;
    this.revokeEvidence();
    const targetId = this.admin ? this.adminReservationId().trim() : this.id;
    const gen = ++this.generation;
    this.loading.set(true);
    this.error.set('');
    const requests = this.admin
      ? [
          this.api.read(token, backendOperation('openDisputes')),
          this.api.read(token, backendOperation('damageClaims')),
          ...(targetId
            ? [
                this.api.read(
                  token,
                  backendOperation('reservationGuarantee', { reservationId: targetId }),
                ),
                this.api.read(
                  token,
                  backendOperation('reservationSettlement', { reservationId: targetId }),
                ),
                this.api.read(
                  token,
                  backendOperation('reservationReceipt', { reservationId: targetId }),
                ),
              ]
            : []),
        ]
      : [
          this.api.read(token, this.path('incident')),
          this.api.read(token, this.path('claim')),
          this.api.read(token, this.path('guarantee')),
          this.api.read(token, this.path('settlement')),
          this.api.read(token, this.path('receipt')),
        ];
    const results = await Promise.allSettled(requests);
    const unauthorized = results.find(
      (result) =>
        result.status === 'rejected' &&
        result.reason instanceof FinanceApiError &&
        result.reason.status === 401,
    );
    if (unauthorized?.status === 'rejected') this.identity.expireIfCurrent(token);
    if (!this.current(gen, owner, token, targetId)) return;
    if (this.admin) {
      this.disputes.set(results[0].status === 'fulfilled' ? arr(results[0].value) : []);
      this.claims.set(results[1].status === 'fulfilled' ? arr(results[1].value) : []);
      this.guarantee.set(results[2]?.status === 'fulfilled' ? unwrap(results[2].value) : null);
      this.settlement.set(results[3]?.status === 'fulfilled' ? unwrap(results[3].value) : null);
      this.receipt.set(results[4]?.status === 'fulfilled' ? unwrap(results[4].value) : null);
    } else {
      this.disputes.set(results[0].status === 'fulfilled' ? arr(results[0].value) : []);
      this.claim.set(results[1].status === 'fulfilled' ? unwrap(results[1].value) : null);
      this.guarantee.set(results[2].status === 'fulfilled' ? unwrap(results[2].value) : null);
      this.settlement.set(results[3].status === 'fulfilled' ? unwrap(results[3].value) : null);
      this.receipt.set(results[4].status === 'fulfilled' ? unwrap(results[4].value) : null);
      const failure = results.find(
        (result) =>
          result.status === 'rejected' &&
          !(result.reason instanceof FinanceApiError && result.reason.status === 404),
      );
      if (failure?.status === 'rejected') this.error.set(this.message(failure.reason));
    }
    this.loading.set(false);
  }

  async openDispute(): Promise<void> {
    await this.write('incident', { reason_code: 'ensayo_privacidad' }, 'dispute');
  }
  async openClaim(): Promise<void> {
    const description = this.description().trim();
    if (!description) {
      this.error.set('Escribe una descripción sintética.');
      return;
    }
    await this.write('claim', { description }, 'claim');
  }
  async defend(): Promise<void> {
    const description = this.description().trim();
    if (!description) {
      this.error.set('Escribe un descargo sintético.');
      return;
    }
    await this.write('claimDefense', { description }, 'defense');
  }
  async syntheticEvidence(): Promise<void> {
    await this.write('defenseEvidence', {}, 'evidence');
  }
  async guaranteeOperation(): Promise<void> {
    const amount = this.integerAmount(this.guaranteeAmount());
    if (amount === null || amount > 50000) {
      this.error.set('El importe debe ser un entero entre 0 y 50.000 CLP.');
      return;
    }
    await this.write(
      'guarantee',
      { kind: this.guaranteeKind(), amount_clp: amount, outcome: this.outcome() },
      `guarantee:${this.guaranteeKind()}`,
    );
  }
  async settle(): Promise<void> {
    await this.write('settlement', { outcome: this.outcome() }, 'settlement');
  }
  async reconcile(kind: 'guarantee' | 'settlement'): Promise<void> {
    const body =
      kind === 'guarantee'
        ? {
            kind: this.guaranteeKind(),
            outcome: this.outcome() === 'sin_respuesta' ? 'exito' : this.outcome(),
          }
        : { outcome: this.outcome() === 'sin_respuesta' ? 'exito' : this.outcome() };
    await this.write(
      kind === 'guarantee' ? 'guaranteeReconcile' : 'settlementReconcile',
      body,
      `${kind}-reconcile:${this.guaranteeKind()}`,
    );
  }
  async decideClaim(claim: unknown): Promise<void> {
    const claimId = field(claim, 'id') || field(claim, 'claim_id');
    const reservationId = field(claim, 'reservation_id');
    if (!claimId || !reservationId) {
      this.error.set('Go no devolvió identificadores suficientes para esta decisión.');
      return;
    }
    const amount = this.integerAmount(this.deductionAmount());
    if (amount === null || amount > 50000) {
      this.error.set('La deducción debe ser un entero entre 0 y 50.000 CLP.');
      return;
    }
    const resolvedOutcome = this.claimResolution(claim);
    if (!resolvedOutcome) {
      this.error.set('Go aún no confirma una resolución para este reclamo.');
      return;
    }
    const context = this.contextId();
    const body = {
      claim_id: claimId,
      outcome: resolvedOutcome,
      deduction_clp: resolvedOutcome === 'acogido' ? amount : 0,
      reason_code:
        resolvedOutcome === 'acogido' && amount > 0 ? 'dano_acreditado' : 'sin_deduccion',
      ...(resolvedOutcome === 'acogido' && amount > 0 && this.decisionEvidenceId()
        ? { evidence_id: this.decisionEvidenceId() }
        : {}),
    };
    await this.perform(
      () =>
        this.api.execute(
          this.identity.accessToken()!,
          backendOperation('decideReservationFinance', { reservationId: reservationId }),
          body,
          this.attemptId(
            this.identity.session()?.account_id ?? '',
            reservationId,
            `financial-decision:${claimId}`,
          ),
        ),
      (result) => this.financialDecision.set(unwrap(result)),
      context,
    );
  }
  async closeDispute(dispute: unknown): Promise<void> {
    const id = field(dispute, 'id') || field(dispute, 'dispute_id');
    if (!id) {
      this.error.set('Go no devolvió el identificador de incidencia.');
      return;
    }
    const context = this.contextId();
    const reservationId = field(dispute, 'reservation_id') || context;
    await this.perform(
      () =>
        this.api.execute(
          this.identity.accessToken()!,
          backendOperation('closeDispute', { disputeId: id }),
          { reason_code: 'ensayo_finalizado' },
          this.attemptId(
            this.identity.session()?.account_id ?? '',
            reservationId,
            `close-dispute:${id}`,
          ),
        ),
      undefined,
      context,
    );
  }
  evidenceItems(value: unknown): unknown[] {
    if (typeof value !== 'object' || value === null) return [];
    const record = value as RecordValue;
    const items = Array.isArray(record['evidence']) ? (record['evidence'] as unknown[]) : [];
    const defense = record['defense'];
    const defenseItems =
      typeof defense === 'object' &&
      defense !== null &&
      Array.isArray((defense as RecordValue)['evidence'])
        ? ((defense as RecordValue)['evidence'] as unknown[])
        : [];
    return [...items, ...defenseItems];
  }
  async viewEvidence(evidence: unknown, claimId?: string): Promise<void> {
    const token = this.identity.accessToken();
    const owner = this.identity.session();
    const gen = this.generation;
    const evidenceId = field(evidence, 'id');
    const reservationId = this.admin ? this.adminReservationId().trim() : this.id;
    const claimID = claimId ?? field(this.claim(), 'id');
    if (!token || !owner || !evidenceId || !reservationId) return;
    this.revokeEvidence();
    this.error.set('');
    const path =
      this.admin && claimID
        ? backendOperation('claimEvidence', { claimId: claimID, evidenceId: evidenceId })
        : backendOperation('reservationEvidence', {
            reservationId: reservationId,
            evidenceId: evidenceId,
          });
    try {
      const blob = await this.api.download(token, path);
      if (
        this.destroyed ||
        gen !== this.generation ||
        owner !== this.identity.session() ||
        token !== this.identity.accessToken() ||
        reservationId !== (this.admin ? this.adminReservationId().trim() : this.id)
      )
        return;
      this.evidenceUrl.set(URL.createObjectURL(blob));
    } catch (error) {
      if (error instanceof FinanceApiError && error.status === 401)
        this.identity.expireIfCurrent(token);
      if (
        !this.destroyed &&
        owner === this.identity.session() &&
        token === this.identity.accessToken()
      )
        this.error.set(this.message(error));
    }
  }
  async resolve(claim: unknown): Promise<void> {
    const claimId = field(claim, 'id') || field(claim, 'claim_id');
    const context = this.contextId();
    if (!claimId) {
      this.error.set('Go no devolvió el identificador del reclamo.');
      return;
    }
    const outcome = this.decision() === 'rechazado' ? 'rechazado' : 'acogido';
    this.selectedClaim.set(claim);
    await this.perform(
      () =>
        this.api.execute(
          this.identity.accessToken()!,
          backendOperation('resolveClaim', { claimId: claimId }),
          { outcome, reason_code: this.reasonCode() },
          this.attemptId(
            this.identity.session()?.account_id ?? '',
            field(claim, 'reservation_id'),
            `claim-resolution:${claimId}`,
          ),
        ),
      (result) => this.selectedClaim.set(unwrap(result)),
      context,
    );
  }
  claimResolution(claim: unknown): 'acogido' | 'rechazado' | '' {
    const resolution =
      typeof claim === 'object' && claim !== null ? (claim as RecordValue)['resolution'] : null;
    const outcome =
      typeof resolution === 'object' && resolution !== null
        ? String((resolution as RecordValue)['outcome'] ?? '')
        : '';
    return outcome === 'acogido' || outcome === 'rechazado' ? outcome : '';
  }
  async refresh(): Promise<void> {
    await this.load();
  }
  setDescription(value: string): void {
    this.description.set(value);
  }
  setOutcome(value: string): void {
    this.outcome.set(value);
  }
  setReason(value: string): void {
    this.reasonCode.set(value);
  }
  setDecision(value: string): void {
    this.decision.set(value);
  }
  setGuaranteeKind(value: string): void {
    this.guaranteeKind.set(value);
  }
  setGuaranteeAmount(value: string): void {
    this.guaranteeAmount.set(value);
  }
  setDeductionAmount(value: string): void {
    this.deductionAmount.set(value);
  }
  setDecisionEvidenceId(value: string): void {
    this.decisionEvidenceId.set(value);
  }
  setAdminReservationId(value: string): void {
    if (value !== this.adminReservationId()) {
      this.generation++;
      this.writeRevision++;
      this.revokeEvidence();
      this.guarantee.set(null);
      this.settlement.set(null);
      this.receipt.set(null);
      this.selectedClaim.set(null);
      this.financialDecision.set(null);
      this.error.set('');
      this.notice.set('');
      this.busy.set(false);
      this.loading.set(false);
    }
    this.adminReservationId.set(value);
  }
  label(value: unknown, key: string): string {
    return field(value, key) || '—';
  }
  json(value: unknown): string {
    return value == null ? 'Aún no hay registros.' : JSON.stringify(value, null, 2);
  }

  private async write(
    suffix: ReservationFinanceResource,
    body: unknown,
    attempt: string,
  ): Promise<void> {
    const reservationId = this.contextId();
    const accountId = this.identity.session()?.account_id ?? '';
    await this.perform(
      () =>
        this.api.execute(
          this.identity.accessToken()!,
          this.path(suffix),
          body,
          this.attemptId(accountId, reservationId, attempt),
        ),
      undefined,
      reservationId,
    );
  }
  private async perform(
    action: () => Promise<unknown>,
    confirmed?: (result: unknown) => void,
    context = this.contextId(),
  ): Promise<void> {
    const token = this.identity.accessToken();
    const owner = this.identity.session();
    const gen = this.generation;
    if (!token || !owner) return;
    const operation = ++this.writeRevision;
    this.busy.set(true);
    this.error.set('');
    this.notice.set('Operación en curso; Go conserva el estado confirmado.');
    try {
      const result = await action();
      if (!this.current(gen, owner, token, context)) return;
      confirmed?.(result);
      this.notice.set('Go confirmó la operación de ensayo.');
      this.busy.set(false);
      await this.load();
    } catch (error) {
      if (error instanceof FinanceApiError && error.status === 401)
        this.identity.expireIfCurrent(token);
      if (!this.current(gen, owner, token, context)) return;
      this.error.set(this.message(error));
      this.notice.set(
        error instanceof FinanceApiError &&
          (error.status === 0 || error.status === 408 || error.status >= 500)
          ? 'Resultado incierto: conserva la misma clave y el mismo payload hasta conciliar con Go.'
          : '',
      );
    } finally {
      if (
        operation === this.writeRevision &&
        !this.destroyed &&
        this.identity.accessToken() === token
      )
        this.busy.set(false);
    }
  }
  private current(gen: number, owner: unknown, token: string, context = this.contextId()): boolean {
    return (
      !this.destroyed &&
      gen === this.generation &&
      owner === this.identity.session() &&
      token === this.identity.accessToken() &&
      context === this.contextId()
    );
  }
  private contextId(): string {
    return this.admin ? this.adminReservationId().trim() : this.id;
  }
  private attemptId(account: string, reservation: string, operation: string): string {
    return `account:${account}:reservation:${reservation}:operation:${operation}`;
  }
  private path(suffix: ReservationFinanceResource): BackendOperation {
    const id = this.admin ? this.adminReservationId().trim() : this.id;
    return backendOperation('reservationFinance', { reservationId: id, resource: suffix });
  }
  private message(error: unknown): string {
    return error instanceof Error ? error.message : 'No se pudo consultar Go.';
  }
  private integerAmount(value: string): number | null {
    return /^\d+$/.test(value) ? Number(value) : null;
  }
  private clear(): void {
    this.writeRevision++;
    this.revokeEvidence();
    this.generation++;
    this.disputes.set([]);
    this.claims.set([]);
    this.claim.set(null);
    this.selectedClaim.set(null);
    this.guarantee.set(null);
    this.settlement.set(null);
    this.receipt.set(null);
    this.financialDecision.set(null);
    this.description.set('');
    this.error.set('');
    this.notice.set('');
    this.decisionEvidenceId.set('');
    this.busy.set(false);
    this.loading.set(false);
  }
  private revokeEvidence(): void {
    const url = this.evidenceUrl();
    if (url) URL.revokeObjectURL(url);
    this.evidenceUrl.set(null);
  }
}
