import { Component, DestroyRef, effect, inject, input, signal } from '@angular/core';
import type { LocalBookingDetail, LocalRentalOperation, LocalSyntheticContract } from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApiError } from './booking-api';
import { BookingLifecycleApi, BookingLifecycleAttempts } from './booking-lifecycle-api';

type LifecycleContext = { token: string; accountId: string; reservationId: string; generation: number };

@Component({
  selector: 'app-booking-lifecycle-panel',
  standalone: true,
  template: `
    <section class="panel lifecycle" aria-labelledby="contract-title">
      <h2 id="contract-title">Contrato de ensayo</h2>
      <p>ENSAYO SINTÉTICO LOCAL — SIN VALIDEZ JURÍDICA. Go confirma las firmas y permisos de cada participante.</p>
      @if (contract(); as value) {
        <p class="status" role="status">Versión {{ value.version }} · {{ contractState(value.state) }} · SHA-256 {{ value.sha256 }}</p>
        <h3>Firmas</h3>
        <ul>@for (signature of value.signatures; track signature.signer_id) {
          <li>{{ signature.role === 'anfitrion' ? 'Anfitrión' : 'Arrendatario' }}: {{ signature.state }} @if (signature.reason) { · {{ signature.reason }} }</li>
        }</ul>
        <details><summary>Snapshot del contrato de ensayo</summary><pre>{{ value.snapshot }}</pre></details>
        <h3>Historial de contrato</h3>
        <ol>@for (event of value.history; track event.sequence) { <li>{{ event.action }} · {{ time(event.at) }} @if (event.reason) { · {{ event.reason }} }</li> }</ol>
        <div class="buttons">
          <button type="button" class="button secondary" [disabled]="busy()" (click)="refreshContract()">Consultar estado confirmado</button>
          @if (canSign(value)) { <button type="button" class="button" [disabled]="busy()" (click)="sign()">Firmar versión</button><button type="button" class="button secondary" [disabled]="busy()" (click)="reject()">Rechazar versión</button> }
          @if (value.state === 'firmado') { <button type="button" class="button secondary" [disabled]="busy()" (click)="downloadPdf()">Descargar PDF privado</button> }
        </div>
      } @else {
        <p>La generación/consulta aparece cuando la reserva está aprobada por Go.</p>
        @if (canOpenContract()) { <button type="button" class="button" [disabled]="busy()" (click)="openContract()">Generar o consultar contrato</button> }
      }
      @if (busy()) { <p role="status">Consultando a Go…</p> }
      @if (error()) { <p class="error" role="alert">{{ error() }}</p> }
      @if (success()) { <p class="success" role="status">{{ success() }}</p> }
    </section>

    <section class="panel lifecycle" aria-labelledby="operation-title">
      <h2 id="operation-title">Llegada, salida y recepción</h2>
      <p>Go genera imágenes de ensayo privadas. No adjuntes documentos ni archivos propios.</p>
      @if (canCheckIn()) { <label>Comentario sintético (opcional)<textarea [value]="comments()" (input)="comments.set($any($event.target).value)" rows="2"></textarea></label><button class="button" type="button" [disabled]="busy()" (click)="record('check-in')">Registrar check-in</button> }
      @if (canCheckOut()) { <label>Comentario sintético (opcional)<textarea [value]="comments()" (input)="comments.set($any($event.target).value)" rows="2"></textarea></label><button class="button" type="button" [disabled]="busy()" (click)="record('check-out')">Registrar check-out</button> }
      @if (canReceive()) { <label>Observación de recepción (opcional)<textarea [value]="observations()" (input)="observations.set($any($event.target).value)" rows="2"></textarea></label><button class="button" type="button" [disabled]="busy()" (click)="record('reception')">Registrar recepción</button> }
      @if (operations().length) {
        <h3>Historial de operación confirmado por Go</h3>
        <ol class="operations">@for (operation of operations(); track operation.id) {
          <li><strong>{{ operationLabel(operation.kind) }}</strong> · {{ time(operation.occurred_at) }} · {{ operation.result }}
            @if (operation.claim_deadline_at) { <p>Plazo local informado por Go: {{ time(operation.claim_deadline_at) }}</p> }
            @if (operation.comments) { <p>{{ operation.comments }}</p> } @if (operation.observations) { <p>{{ operation.observations }}</p> }
            @for (file of operation.evidence; track file.id) {
              <button type="button" class="button secondary" [disabled]="busy()" (click)="loadEvidence(file.id)">Ver evidencia sintética · {{ file.mime_type }}</button>
              @if (evidenceUrls()[file.id]; as url) { <img class="evidence" [src]="url" alt="Evidencia sintética privada de la operación" /> }
            }
          </li>
        }</ol>
      } @else if (!busy()) { <p>No hay operaciones registradas para esta reserva.</p> }
    </section>
  `,
  styles: [`
    :host{display:block}.panel{border:1px solid #e8ddd0;border-radius:1rem;background:#fffdf8;padding:clamp(1rem,3vw,1.5rem);margin-bottom:1rem}.panel h2{margin-top:0}.panel p,.panel li{line-height:1.55;color:#62584f;overflow-wrap:anywhere}.panel h3{font-size:1.05rem;margin-bottom:.35rem}.panel ol,.panel ul{padding-left:1.3rem}.panel li{margin:.5rem 0}.buttons{display:flex;flex-wrap:wrap;gap:.7rem}.button{min-height:2.8rem;border:0;border-radius:999px;padding:.65rem 1rem;background:#a9573f;color:#fffaf3;font:inherit;font-weight:800;cursor:pointer;margin:.25rem .25rem .25rem 0}.button.secondary{background:#f1e4d7;color:#713b2c}.button:disabled{opacity:.55;cursor:wait}label{display:grid;gap:.4rem;margin:.6rem 0;color:#51483f;font-weight:700}textarea{width:100%;border:1px solid #c9b9a7;border-radius:.55rem;padding:.6rem;min-height:2.8rem;font:inherit}.status{overflow-wrap:anywhere}pre{max-width:100%;white-space:pre-wrap;overflow-wrap:anywhere;background:#f7efe6;padding:.8rem;border-radius:.6rem}.error{padding:.8rem;background:#fff0ed;color:#842d22}.success{padding:.8rem;background:#e8f3e9;color:#245d36}.evidence{display:block;max-width:min(100%,24rem);height:auto;border-radius:.7rem;border:1px solid #e8ddd0;margin:.7rem 0}
  `],
})
export class BookingLifecyclePanel {
  readonly reservation = input.required<LocalBookingDetail>();
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly api = inject(BookingLifecycleApi);
  private readonly attempts = inject(BookingLifecycleAttempts);
  private readonly destroyRef = inject(DestroyRef);
  private token: string | null = null;
  private accountId: string | null = null;
  private reservationId = '';
  private generation = 0;
  private readRevision = 0;
  private destroyed = false;
  private readonly objectUrls = new Map<string, string>();
  private readonly unregister = this.privateState.register(() => this.clearPrivate());
  protected readonly contract = signal<LocalSyntheticContract | null>(null);
  protected readonly operations = signal<LocalRentalOperation[]>([]);
  protected readonly evidenceUrls = signal<Record<string, string>>({});
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly success = signal('');
  protected readonly comments = signal('');
  protected readonly observations = signal('');

  constructor() {
    this.destroyRef.onDestroy(() => { this.destroyed = true; this.generation++; this.unregister(); this.revokeUrls(); });
    effect(() => {
      const reservation = this.reservation();
      const token = this.identity.accessToken();
      const account = this.identity.session()?.account_id ?? null;
      if (token !== this.token || account !== this.accountId || reservation.id !== this.reservationId) {
        this.token = token; this.accountId = account; this.reservationId = reservation.id;
        this.generation++; this.readRevision++; this.clearPrivate();
        if (token && account && this.isParticipant(reservation)) void this.loadOperations();
      }
    });
  }

  protected canOpenContract(): boolean { return this.isParticipant(this.reservation()) && ['aprobada_host','firma_parcial','lista_para_checkin','en_curso','finalizada','cerrada','en_disputa'].includes(this.reservation().state); }
  protected canSign(value: LocalSyntheticContract): boolean {
    const account = this.identity.session()?.account_id;
    return this.isParticipant(this.reservation()) && value.state !== 'firmado' && value.state !== 'anulado' && value.signatures.some((s) => s.signer_id === account && s.state === 'pendiente');
  }
  protected canCheckIn(): boolean { return this.isRenter(this.reservation()) && this.reservation().state === 'lista_para_checkin'; }
  protected canCheckOut(): boolean { return this.isRenter(this.reservation()) && this.reservation().state === 'en_curso'; }
  protected canReceive(): boolean { return this.isHost(this.reservation()) && this.reservation().state === 'finalizada'; }
  protected contractState(state: string): string { return ({ generado:'Generado', firma_parcial:'Firma parcial', firmado:'Firmado por ambos', anulado:'Anulado por Go' } as Record<string,string>)[state] ?? state; }
  protected operationLabel(kind: string): string { return ({ checkin:'Check-in', checkout:'Check-out', recepcion:'Recepción del anfitrión', descargo:'Descargo' } as Record<string,string>)[kind] ?? kind; }
  protected time(value: string): string { return new Intl.DateTimeFormat('es-CL',{dateStyle:'medium',timeStyle:'short',timeZone:this.reservation().time_zone}).format(new Date(value)); }

  protected async openContract(): Promise<void> {
    const context = this.context(); if (!context || !this.canOpenContract()) return;
    await this.execute(context, async () => this.api.createOrGetContract(context.token, context.reservationId), (result) => {
      this.contract.set(result.data); this.success.set(result.notice);
    });
  }
  protected async refreshContract(): Promise<void> {
    const item = this.contract(); const context = this.context(); if (!context || !item) return;
    await this.execute(context, () => this.api.getContract(context.token, item.id), (result) => {
      this.contract.set(result.data); this.success.set(result.notice);
    });
  }
  protected async sign(): Promise<void> { await this.contractAction('sign'); }
  protected async reject(): Promise<void> { await this.contractAction('reject'); }
  protected async downloadPdf(): Promise<void> {
    const contract = this.contract(); const context = this.context();
    if (!contract || contract.state !== 'firmado' || !context) return;
    const generation = context.generation;
    await this.execute(context, async () => this.api.contractPdf(context.token, contract.id), (blob) => {
      const url = URL.createObjectURL(blob);
      const previousUrl = this.objectUrls.get('contract-pdf');
      if (previousUrl) URL.revokeObjectURL(previousUrl);
      this.objectUrls.set('contract-pdf', url);
      const link = document.createElement('a'); link.href = url; link.download = 'contrato-ensayo-sintetico.pdf'; link.click();
      if (this.generation === generation) this.success.set('PDF privado descargado desde Go.');
    });
  }
  protected async record(action: 'check-in' | 'check-out' | 'reception'): Promise<void> {
    const context = this.context(); if (!context) return;
    if ((action === 'check-in' && !this.canCheckIn()) || (action === 'check-out' && !this.canCheckOut()) || (action === 'reception' && !this.canReceive())) return;
    const body = action === 'reception' ? { observations: this.observations().trim() } : { comments: this.comments().trim() };
    const keyAction = action === 'check-in' ? 'checkin' : action === 'check-out' ? 'checkout' : 'reception';
    let attempt;
    try { attempt = this.attempts.get(context.accountId, context.reservationId, keyAction, body); }
    catch (error) { this.error.set(messageFor(error)); return; }
    await this.execute(context, () => this.api.record(context.token, context.reservationId, action, body, attempt.key), (result) => {
      this.attempts.settle(context.accountId, context.reservationId, keyAction, attempt.key);
      this.operations.update((items) => [...items.filter((item) => item.id !== result.data.id), result.data].sort((a,b) => a.sequence-b.sequence));
      this.success.set(result.notice); this.comments.set(''); this.observations.set('');
      this.readRevision++;
      void this.loadOperations();
    }, () => this.attempts.markUncertain(context.accountId, context.reservationId, keyAction, attempt.key));
  }
  protected async loadEvidence(evidenceId: string): Promise<void> {
    if (this.objectUrls.has(evidenceId)) return;
    const context = this.context(); if (!context) return;
    await this.execute(context, () => this.api.evidence(context.token, context.reservationId, evidenceId), (blob) => {
      if (blob.type !== 'image/png') throw new Error('Go no devolvió la evidencia PNG esperada.');
      const url = URL.createObjectURL(blob); this.objectUrls.set(evidenceId, url); this.evidenceUrls.update((urls) => ({ ...urls, [evidenceId]: url }));
    });
  }

  private async contractAction(action: 'sign' | 'reject'): Promise<void> {
    const contract = this.contract(); const context = this.context();
    if (!contract || !context || !this.canSign(contract)) return;
    await this.execute(context, async () => action === 'sign' ? this.api.signContract(context.token, contract.id) : this.api.rejectContract(context.token, contract.id), (result) => {
      this.contract.set(result.data); this.success.set(result.notice);
    });
  }
  private async loadOperations(): Promise<void> {
    const context = this.context(); if (!context || !this.isParticipant(this.reservation())) return;
    const revision = ++this.readRevision;
    await this.execute(context, () => this.api.operations(context.token, context.reservationId), (result) => {
      if (revision === this.readRevision) this.operations.set(result.data.items);
    }, undefined, false);
  }
  private async execute<T>(context: LifecycleContext, request: () => Promise<T>, apply: (value: T) => void, onUncertain?: () => void, showBusy = true): Promise<void> {
    if (showBusy) this.busy.set(true); this.error.set(''); this.success.set('');
    try { const result = await request(); if (!this.current(context)) return; apply(result); }
    catch (error) {
      if (error instanceof BookingApiError && error.status === 401) {
        if (context.token === this.identity.accessToken() && context.accountId === this.identity.session()?.account_id) this.identity.expireIfCurrent(context.token);
        return;
      }
      if (this.current(context)) {
        const uncertain = !(error instanceof BookingApiError) || error.status === undefined || error.status === 0 || error.status >= 500;
        if (uncertain) onUncertain?.();
        this.error.set(uncertain ? `${messageFor(error)} La respuesta quedó incierta; reintenta sin cambiar el contenido.` : messageFor(error));
      }
    } finally { if (showBusy && this.current(context)) this.busy.set(false); }
  }
  private context(): LifecycleContext | null {
    const token = this.identity.accessToken(); const accountId = this.identity.session()?.account_id ?? null;
    return token && accountId && accountId === this.accountId && this.reservation().id === this.reservationId
      ? { token, accountId, reservationId: this.reservationId, generation: this.generation } : null;
  }
  private current(context: LifecycleContext): boolean {
    return !this.destroyed && context.generation === this.generation && context.token === this.identity.accessToken() && context.accountId === this.identity.session()?.account_id && context.reservationId === this.reservation().id;
  }
  private isParticipant(item: LocalBookingDetail): boolean { return this.isRenter(item) || this.isHost(item); }
  private isRenter(item: LocalBookingDetail): boolean { return this.identity.session()?.account_id === item.renter_id; }
  private isHost(item: LocalBookingDetail): boolean { return this.identity.session()?.account_id === item.host_id; }
  private clearPrivate(): void { this.contract.set(null); this.operations.set([]); this.error.set(''); this.success.set(''); this.busy.set(false); this.comments.set(''); this.observations.set(''); this.revokeUrls(); }
  private revokeUrls(): void { for (const url of this.objectUrls.values()) URL.revokeObjectURL(url); this.objectUrls.clear(); this.evidenceUrls.set({}); }
}
function messageFor(error: unknown): string { return error instanceof Error ? error.message : 'Go no pudo completar esta operación.'; }
