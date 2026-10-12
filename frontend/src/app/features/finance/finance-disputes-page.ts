import { Component, DestroyRef, effect, inject, signal } from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { FinanceApi, FinanceApiError } from './finance-api';

type RecordValue = Record<string, unknown>;
const unwrap = (value: unknown): unknown => {
  if (typeof value === 'object' && value !== null && 'data' in value) return (value as RecordValue)['data'];
  return value;
};
const arr = (value: unknown): unknown[] => {
  const data = unwrap(value);
  if (Array.isArray(data)) return data;
  return typeof data === 'object' && data !== null && 'items' in data && Array.isArray((data as RecordValue)['items']) ? (data as RecordValue)['items'] as unknown[] : [];
};
const field = (value: unknown, key: string): string => typeof value === 'object' && value !== null ? String((value as RecordValue)[key] ?? '') : '';

@Component({
  selector: 'app-finance-disputes-page',
  imports: [RouterLink],
  templateUrl: './finance-disputes-page.html',
  styleUrl: './finance-disputes-page.scss',
})
export class FinanceDisputesPage {
  private readonly api = inject(FinanceApi);
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly route = inject(ActivatedRoute);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  protected id = '';
  private owner = this.identity.session();
  private token = this.identity.accessToken();
  private generation = 0;
  private writeRevision = 0;
  private destroyed = false;
  private readonly unregister = this.privateState.register(() => this.clear());
  protected readonly admin = this.route.snapshot.data['mode'] === 'admin';
  protected readonly disputes = signal<unknown[]>([]);
  protected readonly adminReservationId = signal('');
  protected readonly claims = signal<unknown[]>([]);
  protected readonly claim = signal<unknown | null>(null);
  protected readonly selectedClaim = signal<unknown | null>(null);
  protected readonly financialDecision = signal<unknown | null>(null);
  protected readonly guarantee = signal<unknown | null>(null);
  protected readonly settlement = signal<unknown | null>(null);
  protected readonly receipt = signal<unknown | null>(null);
  protected readonly evidenceUrl = signal<string | null>(null);
  protected readonly description = signal('');
  protected readonly outcome = signal('sin_respuesta');
  protected readonly reasonCode = signal('evidencia_insuficiente');
  protected readonly decision = signal('rechazado');
  protected readonly guaranteeKind = signal('autorizacion');
  protected readonly guaranteeAmount = signal('50000');
  protected readonly deductionAmount = signal('0');
  protected readonly decisionEvidenceId = signal('');
  protected readonly busy = signal(false);
  protected readonly loading = signal(false);
  protected readonly notice = signal('ENSAYO LOCAL — SIN MOVIMIENTO DE DINERO REAL');
  protected readonly error = signal('');

  constructor() {
    this.title.setTitle('Disputas y finanzas | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => { this.destroyed = true; this.generation++; this.revokeEvidence(); this.unregister(); this.meta.removeTag("name='robots'"); });
    effect(() => {
      const session = this.identity.session();
      const token = this.identity.accessToken();
      if (session !== this.owner || token !== this.token) {
        this.owner = session; this.token = token; this.clear();
        if (session && token && (this.admin || this.id)) void this.load();
      }
    });
    this.route.paramMap.subscribe((params) => {
      const id = params.get('id') ?? '';
      if (id !== this.id) { this.id = id; this.clear(); if (this.id) void this.load(); }
    });
    if (this.admin && this.identity.session() && this.identity.accessToken()) void this.load();
  }

  protected async load(): Promise<void> {
    const token = this.identity.accessToken(); const owner = this.identity.session();
    if (!token || !owner || (!this.admin && !this.id)) return;
    this.revokeEvidence();
    const gen = ++this.generation; this.loading.set(true); this.error.set('');
    const targetId = this.admin ? this.adminReservationId().trim() : this.id;
    const requests = this.admin
      ? [this.api.call(token, '/admin/disputes', 'GET'), this.api.call(token, '/admin/local/damage-claims', 'GET'),
        ...(targetId ? [this.api.call(token, `/local/booking-trial/reservations/${encodeURIComponent(targetId)}/guarantee`, 'GET'),
          this.api.call(token, `/local/booking-trial/reservations/${encodeURIComponent(targetId)}/settlement`, 'GET'),
          this.api.call(token, `/local/booking-trial/reservations/${encodeURIComponent(targetId)}/receipt`, 'GET')] : [])]
      : [
          this.api.call(token, this.path('/disputes'), 'GET'),
          this.api.call(token, this.path('/damage-claim'), 'GET'),
          this.api.call(token, this.path('/guarantee'), 'GET'),
          this.api.call(token, this.path('/settlement'), 'GET'),
          this.api.call(token, this.path('/receipt'), 'GET'),
        ];
    const results = await Promise.allSettled(requests);
    const unauthorized = results.find((result) => result.status === 'rejected' && result.reason instanceof FinanceApiError && result.reason.status === 401);
    if (unauthorized?.status === 'rejected') this.identity.expireIfCurrent(token);
    if (!this.current(gen, owner, token)) return;
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
      const failure = results.find((result) => result.status === 'rejected' && !(result.reason instanceof FinanceApiError && result.reason.status === 404));
      if (failure?.status === 'rejected') this.error.set(this.message(failure.reason));
    }
    this.loading.set(false);
  }

  protected async openDispute(): Promise<void> {
    await this.write('/disputes', { reason_code: 'ensayo_privacidad' }, 'dispute');
  }
  protected async openClaim(): Promise<void> {
    const description = this.description().trim();
    if (!description) { this.error.set('Escribe una descripción sintética.'); return; }
    await this.write('/damage-claim', { description }, 'claim');
  }
  protected async defend(): Promise<void> {
    const description = this.description().trim();
    if (!description) { this.error.set('Escribe un descargo sintético.'); return; }
    await this.write('/damage-claim/defense', { description }, 'defense');
  }
  protected async syntheticEvidence(): Promise<void> {
    await this.write('/damage-claim/defense/evidence', {}, 'evidence');
  }
  protected async guaranteeOperation(): Promise<void> {
    const amount = this.integerAmount(this.guaranteeAmount());
    if (amount === null || amount > 50000) { this.error.set('El importe debe ser un entero entre 0 y 50.000 CLP.'); return; }
    await this.write('/guarantee', { kind: this.guaranteeKind(), amount_clp: amount, outcome: this.outcome() }, `guarantee:${this.guaranteeKind()}`);
  }
  protected async settle(): Promise<void> {
    await this.write('/settlement', { outcome: this.outcome() }, 'settlement');
  }
  protected async reconcile(kind: 'guarantee' | 'settlement'): Promise<void> {
    const body = kind === 'guarantee'
      ? { kind: this.guaranteeKind(), outcome: this.outcome() === 'sin_respuesta' ? 'exito' : this.outcome() }
      : { outcome: this.outcome() === 'sin_respuesta' ? 'exito' : this.outcome() };
    await this.write(kind === 'guarantee' ? '/guarantee-reconcile' : '/settlement-reconcile', body, `${kind}-reconcile:${this.guaranteeKind()}`);
  }
  protected async decideClaim(claim: unknown): Promise<void> {
    const claimId = field(claim, 'id') || field(claim, 'claim_id');
    const reservationId = field(claim, 'reservation_id');
    if (!claimId || !reservationId) { this.error.set('Go no devolvió identificadores suficientes para esta decisión.'); return; }
    const amount = this.integerAmount(this.deductionAmount());
    if (amount === null || amount > 50000) { this.error.set('La deducción debe ser un entero entre 0 y 50.000 CLP.'); return; }
    const resolvedOutcome = this.claimResolution(claim);
    if (!resolvedOutcome) { this.error.set('Go aún no confirma una resolución para este reclamo.'); return; }
    const body = {
      claim_id: claimId,
      outcome: resolvedOutcome,
      deduction_clp: resolvedOutcome === 'acogido' ? amount : 0,
      reason_code: resolvedOutcome === 'acogido' && amount > 0 ? 'dano_acreditado' : 'sin_deduccion',
      ...(resolvedOutcome === 'acogido' && amount > 0 && this.decisionEvidenceId() ? { evidence_id: this.decisionEvidenceId() } : {}),
    };
    await this.perform(
      () => this.api.call(this.identity.accessToken()!, `/local/booking-trial/reservations/${encodeURIComponent(reservationId)}/financial-decision`, 'POST', body, `financial-decision:${reservationId}:${claimId}`),
      (result) => this.financialDecision.set(unwrap(result)),
    );
  }
  protected async closeDispute(dispute: unknown): Promise<void> {
    const id = field(dispute, 'id') || field(dispute, 'dispute_id');
    if (!id) { this.error.set('Go no devolvió el identificador de incidencia.'); return; }
    await this.perform(() => this.api.call(this.identity.accessToken()!, `/admin/disputes/${encodeURIComponent(id)}/close`, 'POST', { reason_code: 'ensayo_finalizado' }, `close-dispute:${id}`));
  }
  protected evidenceItems(value: unknown): unknown[] {
    if (typeof value !== 'object' || value === null) return [];
    const record = value as RecordValue;
    const items = Array.isArray(record['evidence']) ? record['evidence'] as unknown[] : [];
    const defense = record['defense'];
    const defenseItems = typeof defense === 'object' && defense !== null && Array.isArray((defense as RecordValue)['evidence'])
      ? (defense as RecordValue)['evidence'] as unknown[] : [];
    return [...items, ...defenseItems];
  }
  protected async viewEvidence(evidence: unknown, claimId?: string): Promise<void> {
    const token = this.identity.accessToken(); const owner = this.identity.session(); const gen = this.generation;
    const evidenceId = field(evidence, 'id');
    const reservationId = this.admin ? this.adminReservationId().trim() : this.id;
    const claimID = claimId ?? field(this.claim(), 'id');
    if (!token || !owner || !evidenceId || !reservationId) return;
    this.revokeEvidence(); this.error.set('');
    const path = this.admin && claimID
      ? `/admin/local/damage-claims/${encodeURIComponent(claimID)}/evidence/${encodeURIComponent(evidenceId)}`
      : `/local/booking-trial/reservations/${encodeURIComponent(reservationId)}/evidence/${encodeURIComponent(evidenceId)}`;
    try {
      const blob = await this.api.download(token, path);
      if (this.destroyed || gen !== this.generation || owner !== this.identity.session() || token !== this.identity.accessToken() || reservationId !== (this.admin ? this.adminReservationId().trim() : this.id)) return;
      this.evidenceUrl.set(URL.createObjectURL(blob));
    } catch (error) {
      if (error instanceof FinanceApiError && error.status === 401) this.identity.expireIfCurrent(token);
      if (!this.destroyed && owner === this.identity.session() && token === this.identity.accessToken()) this.error.set(this.message(error));
    }
  }
  protected async resolve(claim: unknown): Promise<void> {
    const claimId = field(claim, 'id') || field(claim, 'claim_id');
    if (!claimId) { this.error.set('Go no devolvió el identificador del reclamo.'); return; }
    const outcome = this.decision() === 'rechazado' ? 'rechazado' : 'acogido';
    this.selectedClaim.set(claim);
    await this.perform(
      () => this.api.call(this.identity.accessToken()!, `/admin/local/damage-claims/${encodeURIComponent(claimId)}/resolution`, 'POST', { outcome, reason_code: this.reasonCode() }, `claim-resolution:${claimId}`),
      (result) => this.selectedClaim.set(unwrap(result)),
    );
  }
  protected claimResolution(claim: unknown): 'acogido' | 'rechazado' | '' {
    const resolution = typeof claim === 'object' && claim !== null ? (claim as RecordValue)['resolution'] : null;
    const outcome = typeof resolution === 'object' && resolution !== null ? String((resolution as RecordValue)['outcome'] ?? '') : '';
    return outcome === 'acogido' || outcome === 'rechazado' ? outcome : '';
  }
  protected async refresh(): Promise<void> { await this.load(); }
  protected setDescription(value: string): void { this.description.set(value); }
  protected setOutcome(value: string): void { this.outcome.set(value); }
  protected setReason(value: string): void { this.reasonCode.set(value); }
  protected setDecision(value: string): void { this.decision.set(value); }
  protected setGuaranteeKind(value: string): void { this.guaranteeKind.set(value); }
  protected setGuaranteeAmount(value: string): void { this.guaranteeAmount.set(value); }
  protected setDeductionAmount(value: string): void { this.deductionAmount.set(value); }
  protected setDecisionEvidenceId(value: string): void { this.decisionEvidenceId.set(value); }
  protected setAdminReservationId(value: string): void {
    if (value !== this.adminReservationId()) this.revokeEvidence();
    this.adminReservationId.set(value);
  }
  protected label(value: unknown, key: string): string { return field(value, key) || '—'; }
  protected json(value: unknown): string { return value == null ? 'Aún no hay registros.' : JSON.stringify(value, null, 2); }

  private async write(suffix: string, body: unknown, attempt: string): Promise<void> {
    await this.perform(() => this.api.call(this.identity.accessToken()!, this.path(suffix), 'POST', body, `${this.identity.session()?.account_id}:${this.id}:${attempt}`));
  }
  private async perform(action: () => Promise<unknown>, confirmed?: (result: unknown) => void): Promise<void> {
    const token = this.identity.accessToken(); const owner = this.identity.session(); const gen = this.generation;
    if (!token || !owner) return;
    const operation = ++this.writeRevision;
    this.busy.set(true); this.error.set(''); this.notice.set('Operación en curso; Go conserva el estado confirmado.');
    try {
      const result = await action();
      if (!this.current(gen, owner, token)) return;
      confirmed?.(result);
      this.notice.set('Go confirmó la operación de ensayo.'); this.busy.set(false); await this.load();
    } catch (error) {
      if (error instanceof FinanceApiError && error.status === 401) this.identity.expireIfCurrent(token);
      if (!this.current(gen, owner, token)) return;
      this.error.set(this.message(error));
      this.notice.set(error instanceof FinanceApiError && (error.status === 0 || error.status === 408 || error.status >= 500)
        ? 'Resultado incierto: conserva la misma clave y el mismo payload hasta conciliar con Go.' : '');
    } finally { if (operation === this.writeRevision && !this.destroyed && this.identity.accessToken() === token) this.busy.set(false); }
  }
  private current(gen: number, owner: unknown, token: string): boolean {
    return !this.destroyed && gen === this.generation && owner === this.identity.session() && token === this.identity.accessToken();
  }
  private path(suffix: string): string {
    const id = this.admin ? this.adminReservationId().trim() : this.id;
    return `/local/booking-trial/reservations/${encodeURIComponent(id)}${suffix}`;
  }
  private message(error: unknown): string { return error instanceof Error ? error.message : 'No se pudo consultar Go.'; }
  private integerAmount(value: string): number | null { return /^\d+$/.test(value) ? Number(value) : null; }
  private clear(): void {
    this.writeRevision++;
    this.revokeEvidence();
    this.generation++; this.disputes.set([]); this.claims.set([]); this.claim.set(null); this.selectedClaim.set(null); this.guarantee.set(null);
    this.settlement.set(null); this.receipt.set(null); this.financialDecision.set(null); this.description.set(''); this.error.set(''); this.notice.set('');
    this.decisionEvidenceId.set('');
    this.busy.set(false); this.loading.set(false);
  }
  private revokeEvidence(): void {
    const url = this.evidenceUrl();
    if (url) URL.revokeObjectURL(url);
    this.evidenceUrl.set(null);
  }
}
