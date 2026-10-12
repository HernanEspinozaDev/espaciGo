import { Component, DestroyRef, effect, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import type { LocalBookingCancellationPreview, LocalBookingDetail, LocalBookingReservation } from '../../../generated/api/types.gen';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApi, BookingApiError, PaymentOutcome, RefundOutcome } from './booking-api';
import { BookingAttempts } from './booking-attempts';

@Component({ selector: 'app-booking-detail-page', imports: [RouterLink], templateUrl: './booking-detail-page.html', styleUrl: './booking-detail-page.scss' })
export class BookingDetailPage {
  private readonly api = inject(BookingApi); private readonly attempts = inject(BookingAttempts); private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState); private readonly route = inject(ActivatedRoute); private readonly title = inject(Title);
  private readonly meta = inject(Meta); private readonly destroyRef = inject(DestroyRef); private generation = 0; private destroyed = false;
  private owner = this.identity.session(); private token = this.identity.accessToken(); private id = '';
  private readonly unregister = this.privateState.register(() => this.clear());
  protected readonly reservation = signal<LocalBookingDetail | null>(null); protected readonly loading = signal(false); protected readonly busy = signal(false);
  protected readonly error = signal(''); protected readonly notice = signal('ENSAYO LOCAL — SIN COBRO REAL'); protected readonly preview = signal<LocalBookingCancellationPreview | null>(null);
  protected readonly paymentOutcome = signal<PaymentOutcome>('exito'); protected readonly refundOutcome = signal<RefundOutcome>('exito');
  protected readonly reason = signal(''); protected readonly confirmCancel = signal(false); protected readonly decisionReason = signal('');

  constructor() {
    this.title.setTitle('Detalle de reserva | EspaciGo'); this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => { this.destroyed = true; this.generation++; this.unregister(); this.meta.removeTag("name='robots'"); });
    effect(() => { const session = this.identity.session(); const token = this.identity.accessToken(); if (session !== this.owner || token !== this.token) { this.owner = session; this.token = token; this.clear(); if (session && token && this.id) void this.load(); } });
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => { const id = params.get('id') ?? ''; if (id !== this.id) { this.id = id; this.clear(); if (id) void this.load(); } });
  }

  protected async load(): Promise<void> {
    const token = this.identity.accessToken(); const session = this.identity.session(); const id = this.id;
    if (!token || !session || !id) return; const generation = ++this.generation; this.loading.set(true); this.error.set('');
    try { const result = await this.api.detail(token, id); if (!this.current(generation, token, session, id)) return; this.reservation.set(result.data); this.notice.set(result.safety_notice); }
    catch (error) { if (this.unauthorized(error, token)) return; if (this.current(generation, token, session, id)) this.error.set(messageFor(error)); }
    finally { if (this.current(generation, token, session, id)) this.loading.set(false); }
  }

  protected async pay(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken();
    if (!current || !owner || !token || owner.account_id !== current.renter_id || current.state !== 'pendiente_de_pago') return;
    await this.mutate('payment', { outcome: this.paymentOutcome() }, (key) => this.api.payment(token, current.id, this.paymentOutcome(), key), true);
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
    catch (error) { if (this.unauthorized(error, token)) return; if (this.current(generation, token, owner, current.id)) this.error.set(messageFor(error)); }
    finally { if (this.current(generation, token, owner, current.id)) this.busy.set(false); }
  }
  protected async cancel(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken();
    if (!current || !owner || !token || owner.account_id !== current.renter_id || !this.preview()?.eligible || !this.confirmCancel()) return;
    await this.mutate('cancel', { reason: this.reason().trim() }, (key) => this.api.cancel(token, current.id, this.reason().trim(), key), true);
    this.confirmCancel.set(false);
  }
  protected async refund(): Promise<void> {
    const current = this.reservation(); const owner = this.identity.session(); const token = this.identity.accessToken(); const operationId = current?.refund_operation_id;
    if (!current || !owner || !token || owner.account_id !== current.renter_id || !operationId || current.refund_state !== 'pendiente') return;
    await this.mutate('refund', { outcome: this.refundOutcome(), operationId }, () => this.api.refund(token, current.id, this.refundOutcome(), operationId), true);
  }

  protected isRenter(item: LocalBookingReservation): boolean { return this.identity.session()?.account_id === item.renter_id; }
  protected isHost(item: LocalBookingReservation): boolean { return this.identity.session()?.account_id === item.host_id; }
  protected money(value: number | string | undefined): string { return `CLP ${BigInt(value ?? 0).toLocaleString('es-CL')}`; }
  protected when(value: string, zone: string): string { return new Intl.DateTimeFormat('es-CL', { dateStyle: 'medium', timeStyle: 'short', timeZone: zone }).format(new Date(value)); }
  protected resultLabel(value: string): string { return ({ pendiente_de_pago: 'Pendiente de pago fake', pagada: 'Pago fake confirmado; espera decisión', aprobada_host: 'Aprobada por anfitrión', rechazada_arrendador: 'Rechazada por anfitrión', vencida_pago: 'Vencida por pago', vencida_host: 'Vencida por anfitrión', cancelada_arrendatario: 'Cancelada por arrendatario', cancelada_por_pago: 'Cancelada por pago', finalizada: 'Finalizada', en_curso: 'En curso', cerrada: 'Cerrada', firma_parcial: 'Firma parcial', lista_para_checkin: 'Lista para llegada', en_disputa: 'En disputa', cancelada_por_firma: 'Cancelada por firma' } as Record<string,string>)[value] ?? value; }

  private async mutate<T>(operation: string, payload: unknown, call: (key: string) => Promise<{ safety_notice: string; data: T }>, uncertainOnTimeout: boolean): Promise<void> {
    const owner = this.identity.session(); const token = this.identity.accessToken(); const item = this.reservation(); if (!owner || !token || !item) return;
    let attempt; try { attempt = this.attempts.get(owner.account_id, `${item.id}:${operation}`, payload); } catch (error) { this.error.set(messageFor(error)); return; }
    await this.action(() => call(attempt.key), uncertainOnTimeout, { operation: `${item.id}:${operation}`, key: attempt.key });
  }
  private async action<T>(call: () => Promise<{ safety_notice: string; data: T }>, keepOnTimeout: boolean, attempt?: { operation: string; key: string }): Promise<void> {
    const token = this.identity.accessToken(); const owner = this.identity.session(); const id = this.id; if (!token || !owner) return;
    const generation = ++this.generation; this.busy.set(true); this.error.set('');
    try { const result = await call(); if (!this.current(generation, token, owner, id)) return; if (attempt) this.attempts.settle(owner.account_id, attempt.operation, attempt.key); this.notice.set(result.safety_notice); this.busy.set(false); await this.load(); }
    catch (error) {
      if (this.unauthorized(error, token)) return;
      const uncertain = error instanceof BookingApiError && (error.status === undefined || error.status === 0 || error.status >= 500);
      if (uncertain) {
        if (!keepOnTimeout && attempt) this.attempts.settle(owner.account_id, attempt.operation, attempt.key);
        if (this.current(generation, token, owner, id)) { this.error.set(`${messageFor(error)} Conserva la misma operación para reintentar o consultar su estado en Go.`); this.busy.set(false); await this.load(); }
      } else {
        if (attempt) this.attempts.settle(owner.account_id, attempt.operation, attempt.key);
        if (this.current(generation, token, owner, id)) this.error.set(messageFor(error));
      }
    } finally { if (this.current(generation, token, owner, id)) this.busy.set(false); }
  }
  private unauthorized(error: unknown, token: string): boolean { return error instanceof BookingApiError && error.status === 401 ? this.identity.expireIfCurrent(token) : false; }
  private current(generation: number, token: string, owner: NonNullable<ReturnType<IdentitySession['session']>>, id: string): boolean { return !this.destroyed && generation === this.generation && token === this.identity.accessToken() && owner === this.identity.session() && id === this.id; }
  private clear(): void { this.generation++; this.reservation.set(null); this.preview.set(null); this.error.set(''); this.loading.set(false); this.busy.set(false); this.confirmCancel.set(false); }
}
function messageFor(error: unknown): string { return error instanceof Error ? error.message : 'Go no pudo completar esta operación.'; }
