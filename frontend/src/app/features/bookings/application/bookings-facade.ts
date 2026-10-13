import { DestroyRef, effect, inject, Injectable, signal } from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, Router } from '@angular/router';
import type { LocalBookingReservation } from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { BookingApiError, BookingPort } from '../domain/ports/booking-port';
import { BookingLifecyclePort } from '../domain/ports/booking-lifecycle-port';

@Injectable()
export class BookingsFacade {
  private readonly api = inject(BookingPort);
  private readonly lifecycle = inject(BookingLifecyclePort);
  private readonly identity = inject(IdentitySession);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly privateState = inject(PrivateSessionState);
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly destroyRef = inject(DestroyRef);
  private generation = 0;
  private inviteGeneration = 0;
  private readonly invitedContractId = this.route.snapshot.queryParamMap.get('contrato');
  private owner = this.identity.session();
  private token = this.identity.accessToken();
  private readonly unregister = this.privateState.register(() => this.clear());
  readonly session = this.identity.session;
  readonly reservations = signal<LocalBookingReservation[]>([]);
  readonly loading = signal(false);
  readonly error = signal('');
  readonly notice = signal('ENSAYO LOCAL — SIN COBRO REAL');

  constructor() {
    this.title.setTitle('Reservas | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => {
      this.meta.removeTag("name='robots'");
      this.inviteGeneration++;
      this.unregister();
    });
    effect(() => {
      const session = this.identity.session();
      const token = this.identity.accessToken();
      if (session !== this.owner || token !== this.token) {
        this.owner = session;
        this.token = token;
        this.clear();
        if (session && token) {
          void this.load();
          if (this.invitedContractId) void this.openContractInvite(this.invitedContractId);
        }
      }
    });
    void this.load();
    if (this.owner && this.token && this.invitedContractId)
      void this.openContractInvite(this.invitedContractId);
  }

  async load(): Promise<void> {
    const session = this.identity.session();
    const token = this.identity.accessToken();
    if (!session || !token) return;
    const generation = ++this.generation;
    this.loading.set(true);
    this.error.set('');
    try {
      const response = await this.api.list(token);
      if (!this.current(generation, token, session)) return;
      this.reservations.set(response.data.items);
      this.notice.set(response.safety_notice);
    } catch (error) {
      if (error instanceof BookingApiError && error.status === 401)
        this.identity.expireIfCurrent(token);
      else if (this.current(generation, token, session)) this.error.set(messageFor(error));
    } finally {
      if (this.current(generation, token, session)) this.loading.set(false);
    }
  }

  private async openContractInvite(contractId: string): Promise<void> {
    const session = this.identity.session();
    const token = this.identity.accessToken();
    if (!session || !token) return;
    const generation = ++this.inviteGeneration;
    try {
      const contract = await this.lifecycle.getContract(token, contractId);
      if (!this.currentInvite(generation, token, session)) return;
      await this.router.navigate(['/reservas', contract.data.reservation_id]);
    } catch (error) {
      if (error instanceof BookingApiError && error.status === 401) {
        this.identity.expireIfCurrent(token);
      } else if (this.currentInvite(generation, token, session)) {
        // Go intentionally uses the same not-found response for a missing
        // contract and a contract the authenticated account cannot access.
        this.error.set('No fue posible abrir esta invitación con esta cuenta.');
      }
    }
  }

  private currentInvite(
    generation: number,
    token: string,
    session: NonNullable<ReturnType<IdentitySession['session']>>,
  ): boolean {
    return (
      generation === this.inviteGeneration &&
      token === this.identity.accessToken() &&
      session === this.identity.session()
    );
  }

  participant(reservation: LocalBookingReservation): string {
    const id = this.session()?.account_id;
    return reservation.host_id === id ? 'Recibida como anfitrión' : 'Enviada como arrendatario';
  }
  stateLabel(state: string): string {
    const labels: Record<string, string> = {
      pendiente_de_pago: 'Pendiente de pago fake',
      pagada: 'Pago fake confirmado; espera decisión',
      aprobada_host: 'Aprobada por anfitrión',
      firma_parcial: 'Firma parcial',
      lista_para_checkin: 'Lista para llegada',
      en_curso: 'En curso',
      finalizada: 'Finalizada',
      cerrada: 'Cerrada',
      en_disputa: 'En disputa',
      cancelada_por_firma: 'Cancelada por firma',
      cancelada_por_pago: 'Cancelada por pago',
      rechazada_arrendador: 'Rechazada por anfitrión',
      vencida_pago: 'Vencida por pago',
      vencida_host: 'Vencida por anfitrión',
      cancelada_arrendatario: 'Cancelada por arrendatario',
    };
    return labels[state] ?? state;
  }
  money(value: number | string): string {
    return `CLP ${BigInt(value).toLocaleString('es-CL')}`;
  }
  when(value: string, zone: string): string {
    return new Intl.DateTimeFormat('es-CL', {
      dateStyle: 'medium',
      timeStyle: 'short',
      timeZone: zone,
    }).format(new Date(value));
  }

  private current(
    generation: number,
    token: string,
    session: NonNullable<ReturnType<IdentitySession['session']>>,
  ): boolean {
    return (
      generation === this.generation &&
      token === this.identity.accessToken() &&
      session === this.identity.session()
    );
  }
  private clear(): void {
    this.generation++;
    this.inviteGeneration++;
    this.reservations.set([]);
    this.error.set('');
    this.loading.set(false);
  }
}

function messageFor(error: unknown): string {
  return error instanceof Error ? error.message : 'No fue posible consultar las reservas.';
}
