import { Component, DestroyRef, effect, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import type { LocalBookingCancellationPreview, LocalBookingDetail, LocalBookingReservation } from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApi, BookingApiError, CancellationResult, PaymentOutcome, RefundOutcome, RefundResult } from './booking-api';
import { BookingAttempts } from './booking-attempts';
import { BookingLifecyclePanel } from './booking-lifecycle-panel';

@Component({ selector: 'app-booking-detail-page', imports: [RouterLink, BookingLifecyclePanel], templateUrl: './booking-detail-page.html', styleUrl: './booking-detail-page.scss' })
export class BookingDetailPage {
  private readonly api = inject(BookingApi); private readonly attempts = inject(BookingAttempts); private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState); private readonly route = inject(ActivatedRoute); private readonly title = inject(Title);
  private readonly meta = inject(Meta); private readonly destroyRef = inject(DestroyRef); private generation = 0; private destroyed = false;
  private owner = this.identity.session(); private token = this.identity.accessToken(); private id = '';
  private readRevision = 0; private writeRevision = 0;
  private readonly unregister = this.privateState.register(() => this.clear());
  protected readonly reservation = signal<LocalBookingDetail | null>(null); protected readonly loading = signal(false); protected readonly busy = signal(false);
  protected readonly error = signal(''); protected readonly notice = signal('ENSAYO LOCAL — SIN COBRO REAL'); protected readonly preview = signal<LocalBookingCancellationPreview | null>(null);
  protected readonly uncertainNotice = signal('');
  protected readonly paymentOutcome = signal<PaymentOutcome>('exito'); protected readonly refundOutcome = signal<RefundOutcome>('exito');
  protected readonly reason = signal(''); protected readonly confirmCancel = signal(false); protected readonly decisionReason = signal('');

  constructor() {
    this.title.setTitle('Detalle de reserva | EspaciGo'); this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => { this.destroyed = true; this.generation++; this.unregister(); this.meta.removeTag("name='robots'"); });
    effect(() => { const session = this.identity.session(); const token = this.identity.accessToken(); if (session !== this.owner || token !== this.token) { this.owner = session; this.token = token; this.clear(); if (session && token && this.id) void this.load(); } });
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => { const id = params.get('id') ?? ''; if (id !== this.id) { this.id = id; this.clear(); if (id) void this.load(); } });
  }

  protected refreshAfterLifecycle(): void {
    if (this.identity.accessToken() && this.identity.session() && this.id) void this.load();
  }

  protected async load(): Promise<void> {
    const token = this.identity.accessToken(); const session = this.identity.session(); const id = this.id;
    if (!token || !session || !id) return; const generation = ++this.generation; const refundReadRevision = this.attempts.nextRevision(); this.readRevision++; this.loading.set(true); this.error.set('');
    try {
      const result = await this.api.detail(token, id);
      if (!this.current(generation, token, session, id)) return;
      const read = result.data;
      if (read.refund_operation_id && read.refund_amount_clp !== undefined && (read.refund_state === 'pendiente' || read.refund_state === 'completada')) {
        this.attempts.saveRefundSnapshot(session.account_id, id, {
          operationId: read.refund_operation_id, amount: String(read.refund_amount_clp), state: read.refund_state, revision: refundReadRevision,
        });
      }
      const confirmedRefund = this.attempts.refundSnapshot(session.account_id, id);
      const snapshotIsNewer = !!confirmedRefund && confirmedRefund.revision > refundReadRevision;
      const item = (confirmedRefund ? {
        ...read,
        refund_operation_id: snapshotIsNewer ? confirmedRefund.operationId : read.refund_operation_id ?? confirmedRefund.operationId,
        refund_amount_clp: snapshotIsNewer ? confirmedRefund.amount : read.refund_amount_clp ?? confirmedRefund.amount,
        refund_state: snapshotIsNewer ? confirmedRefund.state : read.refund_state ?? confirmedRefund.state,
      } : read) as LocalBookingDetail;
      this.reservation.set(item); this.notice.set(result.safety_notice); this.syncAttempts(item, session.account_id);
    }
    catch (error) { if (this.unauthorized(error, token, session)) return; if (this.current(generation, token, session, id)) this.error.set(messageFor(error)); }
    finally { if (this.current(generation, token, session, id)) this.loading.set(false); }
  }

  protected async pay(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken();
    if (!current || !owner || !token || owner.account_id !== current.renter_id || current.state !== 'pendiente_de_pago') return;
    const operation = current.id + ':payment';
    const prior = this.attempts.find(owner.account_id, operation);
    const payload = prior?.uncertain ? parsePayload<{ outcome: PaymentOutcome }>(prior.payload) : { outcome: this.paymentOutcome() };
    await this.mutate('payment', payload, (key) => this.api.payment(token, current.id, payload.outcome, key), true);
  }
  protected async decide(value: 'aprobar' | 'rechazar'): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken();
    if (!current || !owner || !token || owner.account_id !== current.host_id || current.state !== 'pagada') return;
    if (value === 'rechazar' && !this.decisionReason().trim()) { this.error.set('Indica el motivo de rechazo.'); return; }
    await this.action(async () => this.api.decision(token, current.id, value, this.decisionReason().trim()), false);
  }
  protected async loadCancellationPreview(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken();
    if (!current || !owner || !token || owner.account_id !== current.renter_id) return;
    const generation = ++this.generation; this.busy.set(true); this.error.set('');
    try { const result = await this.api.cancellationPreview(token, current.id); if (!this.current(generation, token, owner, current.id)) return; this.preview.set(result.data); }
    catch (error) { if (this.unauthorized(error, token, owner)) return; if (this.current(generation, token, owner, current.id)) this.error.set(messageFor(error)); }
    finally { if (this.current(generation, token, owner, current.id)) this.busy.set(false); }
  }
  protected async cancel(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken();
    if (!current || !owner || !token || owner.account_id !== current.renter_id || !this.preview()?.eligible || !this.confirmCancel()) return;
    const operation = current.id + ':cancel';
    const prior = this.attempts.find(owner.account_id, operation);
    const payload = prior?.uncertain ? parsePayload<{ reason: string }>(prior.payload) : { reason: this.reason().trim() };
    await this.mutate('cancel', payload, (key) => this.api.cancel(token, current.id, payload.reason, key), true, (data, revision, applyToView) => this.applyCancellation(data as CancellationResult, revision, applyToView));
    if (this.sameContext(token, owner, current.id)) this.confirmCancel.set(false);
  }
  protected async refund(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken(); const operationId = current?.refund_operation_id;
    if (!current || !owner || !token || owner.account_id !== current.renter_id || !operationId || current.refund_state !== 'pendiente') return;
    const operation = current.id + ':refund';
    const prior = this.attempts.find(owner.account_id, operation);
    const payload = prior?.uncertain ? parsePayload<{ outcome: RefundOutcome; operationId: string }>(prior.payload) : { outcome: this.refundOutcome(), operationId };
    await this.mutate('refund', payload, () => this.api.refund(token, current.id, payload.outcome, operationId), true, (data, revision, applyToView) => this.applyRefund(data as RefundResult, revision, applyToView));
  }

  protected isRenter(item: LocalBookingReservation): boolean { return this.identity.session()?.account_id === item.renter_id; }
  protected isHost(item: LocalBookingReservation): boolean { return this.identity.session()?.account_id === item.host_id; }
  protected money(value: number | string | undefined): string { return `CLP ${BigInt(value ?? 0).toLocaleString('es-CL')}`; }
  protected when(value: string, zone: string): string { return new Intl.DateTimeFormat('es-CL', { dateStyle: 'medium', timeStyle: 'short', timeZone: zone }).format(new Date(value)); }
  protected resultLabel(value: string): string { return ({ pendiente_de_pago: 'Pendiente de pago fake', pagada: 'Pago fake confirmado; espera decisión', aprobada_host: 'Aprobada por anfitrión', rechazada_arrendador: 'Rechazada por anfitrión', vencida_pago: 'Vencida por pago', vencida_host: 'Vencida por anfitrión', cancelada_arrendatario: 'Cancelada por arrendatario', cancelada_por_pago: 'Cancelada por pago', finalizada: 'Finalizada', en_curso: 'En curso', cerrada: 'Cerrada', firma_parcial: 'Firma parcial', lista_para_checkin: 'Lista para llegada', en_disputa: 'En disputa', cancelada_por_firma: 'Cancelada por firma' } as Record<string,string>)[value] ?? value; }

  private async mutate<T>(operation: string, payload: unknown, call: (key: string) => Promise<{ safety_notice: string; data: T }>, uncertainOnTimeout: boolean, onConfirmed?: (data: T, revision: number, applyToView: boolean) => void): Promise<void> {
    const owner = this.identity.session(); const token = this.identity.accessToken(); const item = this.reservation(); if (!owner || !token || !item) return;
    let attempt; try { attempt = this.attempts.get(owner.account_id, `${item.id}:${operation}`, payload); } catch (error) { this.error.set(messageFor(error)); return; }
    await this.action(() => call(attempt.key), uncertainOnTimeout, { operation: item.id + ':' + operation, key: attempt.key }, onConfirmed);
  }
  private async action<T>(call: () => Promise<{ safety_notice: string; data: T }>, keepOnTimeout: boolean, attempt?: { operation: string; key: string }, onConfirmed?: (data: T, revision: number, applyToView: boolean) => void): Promise<void> {
    const token = this.identity.accessToken(); const owner = this.identity.session(); const id = this.id; if (!token || !owner) return;
    const generation = ++this.generation; const actionRevision = ++this.writeRevision; const refundRevision = this.attempts.nextRevision(); this.busy.set(true); this.error.set('');
    try {
      const result = await call();
      if (!this.sameContext(token, owner, id)) return;
      const actionIsCurrent = this.current(generation, token, owner, id) && actionRevision === this.writeRevision;
      if (attempt && actionIsCurrent) this.attempts.settle(owner.account_id, attempt.operation, attempt.key);
      if (actionIsCurrent) {
        this.clearUncertain(attempt?.operation);
        this.notice.set(result.safety_notice); this.busy.set(false);
      }
      onConfirmed?.(result.data, refundRevision, actionIsCurrent);
      if (!actionIsCurrent) return;
      await this.load();
    }
    catch (error) {
      if (attempt && this.current(generation, token, owner, id) && actionRevision === this.writeRevision && error instanceof BookingApiError && error.status !== undefined && error.status < 500 && error.status !== 0) {
        this.attempts.settle(owner.account_id, attempt.operation, attempt.key);
      }
      if (this.unauthorized(error, token, owner)) return;
      const uncertain = error instanceof BookingApiError && (error.status === undefined || error.status === 0 || error.status >= 500);
      if (uncertain) {
        if (!keepOnTimeout && attempt) this.attempts.settle(owner.account_id, attempt.operation, attempt.key);
        if (keepOnTimeout && attempt) this.attempts.markUncertain(owner.account_id, attempt.operation, attempt.key);
        if (this.current(generation, token, owner, id)) {
          if (attempt && keepOnTimeout) this.uncertainNotice.set(messageFor(error) + ' La operación sigue incierta; conserva estos datos y reintenta con la misma solicitud mientras Go no confirme un estado definitivo.');
          else this.error.set(messageFor(error));
          this.busy.set(false); await this.load();
        }
      } else {
        if (this.current(generation, token, owner, id) && actionRevision === this.writeRevision) {
          if (attempt) this.attempts.settle(owner.account_id, attempt.operation, attempt.key);
          this.clearUncertain(attempt?.operation);
          this.error.set(messageFor(error));
        }
      }
    } finally { if (this.current(generation, token, owner, id)) this.busy.set(false); }
  }
  private unauthorized(error: unknown, token: string, owner: NonNullable<ReturnType<IdentitySession['session']>>): boolean {
    return error instanceof BookingApiError && error.status === 401 && owner === this.identity.session() && token === this.identity.accessToken()
      ? this.identity.expireIfCurrent(token)
      : false;
  }
  private current(generation: number, token: string, owner: NonNullable<ReturnType<IdentitySession['session']>>, id: string): boolean { return !this.destroyed && generation === this.generation && token === this.identity.accessToken() && owner === this.identity.session() && id === this.id; }
  private syncAttempts(item: LocalBookingDetail, accountId: string): void {
    const payment = this.attempts.find(accountId, item.id + ':payment');
    if (payment) {
      if (item.state !== 'pendiente_de_pago') { this.attempts.settle(accountId, item.id + ':payment', payment.key); this.clearUncertain(item.id + ':payment'); }
      else if (payment.uncertain) this.uncertainNotice.set('Go todavía muestra el pago como pendiente. Reintenta el mismo intento; su clave y payload se conservan en memoria.');
    }
    const cancel = this.attempts.find(accountId, item.id + ':cancel');
    if (cancel) {
      if (['cancelada_arrendatario', 'cancelada_por_pago', 'cancelada_por_firma'].includes(item.state)) { this.attempts.settle(accountId, item.id + ':cancel', cancel.key); this.clearUncertain(item.id + ':cancel'); }
      else if (cancel.uncertain) this.uncertainNotice.set('Go todavía muestra la reserva sin cancelar. Reintenta la cancelación con el mismo intento.');
    }
    const refund = this.attempts.find(accountId, item.id + ':refund');
    if (refund) {
      if (item.refund_state === 'completada') { this.attempts.settle(accountId, item.id + ':refund', refund.key); this.clearUncertain(item.id + ':refund'); }
      else if (refund.uncertain) this.uncertainNotice.set('Go todavía muestra la devolución pendiente. Reintenta la misma operación.');
    }
  }
  private applyCancellation(data: CancellationResult, revision: number, applyToView: boolean): void {
    const current = this.reservation();
    const accountId = this.identity.session()?.account_id;
    const operationId = data.reservation.refund_operation_id;
    const amount = data.refund_amount_clp ?? data.reservation.refund_amount_clp;
    if (accountId && current && operationId && amount !== undefined && data.refund_state === 'pendiente') {
      this.attempts.saveRefundSnapshot(accountId, current.id, { operationId, amount: String(amount), state: 'pendiente', revision });
    }
    if (applyToView) {
      this.reservation.update((current) => current ? ({
        ...current, ...data.reservation,
        refund_state: data.refund_state === 'no_aplica' ? undefined : data.refund_state,
        refund_amount_clp: data.refund_amount_clp ?? data.reservation.refund_amount_clp,
      } as unknown as LocalBookingDetail) : current);
    } else if (operationId && amount !== undefined) {
      this.reservation.update((current) => current && (!current.refund_operation_id || current.refund_state === undefined)
        ? { ...current, refund_operation_id: operationId, refund_amount_clp: String(amount), refund_state: data.refund_state === 'no_aplica' ? undefined : data.refund_state } as unknown as LocalBookingDetail
        : current);
    }
  }
  private applyRefund(data: RefundResult, revision: number, applyToView: boolean): void {
    const accountId = this.identity.session()?.account_id;
    if (accountId) this.attempts.saveRefundSnapshot(accountId, data.reservation_id, {
      operationId: data.operation_id, amount: String(data.amount_clp), state: data.state, revision,
    });
    if (applyToView) {
      this.reservation.update((current) => current ? ({
        ...current, refund_operation_id: data.operation_id, refund_amount_clp: data.amount_clp,
        refund_state: data.state, refund_last_result: data.last_result, refund_updated_at: data.updated_at,
      } as unknown as LocalBookingDetail) : current);
    } else {
      this.reservation.update((current) => current && (!current.refund_operation_id || current.refund_state === undefined)
        ? { ...current, refund_operation_id: data.operation_id, refund_amount_clp: data.amount_clp, refund_state: data.state, refund_last_result: data.last_result, refund_updated_at: data.updated_at } as unknown as LocalBookingDetail
        : current);
    }
  }
  private clearUncertain(operation?: string): void {
    if (!operation || operation.startsWith(this.id + ':')) this.uncertainNotice.set('');
  }
  private sameContext(token: string, owner: NonNullable<ReturnType<IdentitySession['session']>>, id: string): boolean {
    return !this.destroyed && token === this.identity.accessToken() && owner === this.identity.session() && id === this.id;
  }
  private clear(): void {
    this.generation++; this.reservation.set(null); this.preview.set(null); this.error.set(''); this.uncertainNotice.set('');
    this.loading.set(false); this.busy.set(false); this.confirmCancel.set(false); this.reason.set(''); this.decisionReason.set('');
    this.paymentOutcome.set('exito'); this.refundOutcome.set('exito');
  }
}
function messageFor(error: unknown): string { return error instanceof Error ? error.message : 'Go no pudo completar esta operación.'; }
function parsePayload<T>(serialized: string): T { return JSON.parse(serialized) as T; }
