import { computed, DestroyRef, effect, inject, Injectable, signal } from '@angular/core';
import type {
  LocalBookingDetail,
  LocalRentalOperation,
  LocalRentalOperationInput,
  LocalSyntheticContract,
} from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { BookingLifecyclePort } from '../domain/ports/booking-lifecycle-port';
import { BookingApiError } from '../domain/ports/booking-port';
import { BookingLifecycleAttempts } from './booking-lifecycle-attempts';
type LifecycleContext = {
  token: string;
  accountId: string;
  reservationId: string;
  generation: number;
};
@Injectable()
export class BookingLifecycleFacade {
  private readonly reservationSource = signal<(() => LocalBookingDetail) | null>(null);
  readonly reservation = computed(() => {
    const source = this.reservationSource();
    if (!source) throw new Error('Reservation source must be connected by the view.');
    return source();
  });
  private notifyStateChanged: () => void = () => undefined;
  connectReservation(source: () => LocalBookingDetail, notify: () => void): void {
    this.notifyStateChanged = notify;
    this.reservationSource.set(source);
  }
  readonly stateChanged = { emit: () => this.notifyStateChanged() };
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly api = inject(BookingLifecyclePort);
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
  readonly contract = signal<LocalSyntheticContract | null>(null);
  readonly operations = signal<LocalRentalOperation[]>([]);
  readonly evidenceUrls = signal<Record<string, string>>({});
  readonly busy = signal(false);
  readonly error = signal('');
  readonly success = signal('');
  readonly comments = signal('');
  readonly observations = signal('');

  constructor() {
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
      this.generation++;
      this.unregister();
      this.revokeUrls();
    });
    effect(() => {
      const reservation = this.reservation();
      const token = this.identity.accessToken();
      const account = this.identity.session()?.account_id ?? null;
      if (
        token !== this.token ||
        account !== this.accountId ||
        reservation.id !== this.reservationId
      ) {
        this.token = token;
        this.accountId = account;
        this.reservationId = reservation.id;
        this.generation++;
        this.readRevision++;
        this.clearPrivate();
        if (token && account && this.isParticipant(reservation)) void this.loadOperations();
      }
    });
  }

  retryLabel(action: string, label: string): string {
    return this.attempts.find(this.accountId ?? '', this.reservationId, action)
      ? `Reintentar ${label} original`
      : `Registrar ${label}`;
  }
  hasUncertainAttempt(): boolean {
    return (
      !!this.accountId &&
      ['checkin', 'checkout', 'reception'].some((action) =>
        this.attempts.find(this.accountId!, this.reservationId, action),
      )
    );
  }
  canOpenContract(): boolean {
    return (
      this.isParticipant(this.reservation()) &&
      [
        'aprobada_host',
        'firma_parcial',
        'lista_para_checkin',
        'en_curso',
        'finalizada',
        'cerrada',
        'en_disputa',
      ].includes(this.reservation().state)
    );
  }
  canSign(value: LocalSyntheticContract): boolean {
    const account = this.identity.session()?.account_id;
    return (
      this.isParticipant(this.reservation()) &&
      value.state !== 'firmado' &&
      value.state !== 'anulado' &&
      value.signatures.some((s) => s.signer_id === account && s.state === 'pendiente')
    );
  }
  canCheckIn(): boolean {
    return this.isRenter(this.reservation()) && this.reservation().state === 'lista_para_checkin';
  }
  canCheckOut(): boolean {
    return this.isRenter(this.reservation()) && this.reservation().state === 'en_curso';
  }
  canReceive(): boolean {
    return this.isHost(this.reservation()) && this.reservation().state === 'finalizada';
  }
  contractState(state: string): string {
    return (
      (
        {
          generado: 'Generado',
          firma_parcial: 'Firma parcial',
          firmado: 'Firmado por ambos',
          anulado: 'Anulado por Go',
        } as Record<string, string>
      )[state] ?? state
    );
  }
  operationLabel(kind: string): string {
    return (
      (
        {
          checkin: 'Check-in',
          checkout: 'Check-out',
          recepcion: 'Recepción del anfitrión',
          descargo: 'Descargo',
        } as Record<string, string>
      )[kind] ?? kind
    );
  }
  time(value: string): string {
    return new Intl.DateTimeFormat('es-CL', {
      dateStyle: 'medium',
      timeStyle: 'short',
      timeZone: this.reservation().time_zone,
    }).format(new Date(value));
  }

  async openContract(): Promise<void> {
    const context = this.context();
    if (!context || !this.canOpenContract()) return;
    await this.execute(
      context,
      async () => this.api.createOrGetContract(context.token, context.reservationId),
      (result) => {
        this.contract.set(result.data);
        this.success.set(result.notice);
        this.stateChanged.emit();
      },
    );
  }
  async refreshContract(): Promise<void> {
    const item = this.contract();
    const context = this.context();
    if (!context || !item) return;
    await this.execute(
      context,
      () => this.api.getContract(context.token, item.id),
      (result) => {
        this.contract.set(result.data);
        this.success.set(result.notice);
        this.stateChanged.emit();
      },
    );
  }
  async sign(): Promise<void> {
    await this.contractAction('sign');
  }
  async reject(): Promise<void> {
    await this.contractAction('reject');
  }
  async downloadPdf(): Promise<void> {
    const contract = this.contract();
    const context = this.context();
    if (!contract || contract.state !== 'firmado' || !context) return;
    const generation = context.generation;
    await this.execute(
      context,
      async () => this.api.contractPdf(context.token, contract.id),
      (blob) => {
        const url = URL.createObjectURL(blob);
        const previousUrl = this.objectUrls.get('contract-pdf');
        if (previousUrl) URL.revokeObjectURL(previousUrl);
        this.objectUrls.set('contract-pdf', url);
        const link = document.createElement('a');
        link.href = url;
        link.download = 'contrato-ensayo-sintetico.pdf';
        link.click();
        if (this.generation === generation) this.success.set('PDF privado descargado desde Go.');
      },
    );
  }
  async record(action: 'check-in' | 'check-out' | 'reception'): Promise<void> {
    const context = this.context();
    if (!context) return;
    if (
      (action === 'check-in' && !this.canCheckIn()) ||
      (action === 'check-out' && !this.canCheckOut()) ||
      (action === 'reception' && !this.canReceive())
    )
      return;
    const keyAction =
      action === 'check-in' ? 'checkin' : action === 'check-out' ? 'checkout' : 'reception';
    const prior = this.attempts.find(context.accountId, context.reservationId, keyAction);
    const body = prior
      ? (JSON.parse(prior.payload) as LocalRentalOperationInput)
      : action === 'reception'
        ? { observations: this.observations().trim() }
        : { comments: this.comments().trim() };
    let attempt;
    try {
      attempt = this.attempts.get(context.accountId, context.reservationId, keyAction, body);
    } catch (error) {
      this.error.set(messageFor(error));
      return;
    }
    await this.execute(
      context,
      () => this.api.record(context.token, context.reservationId, action, body, attempt.key),
      (result) => {
        this.attempts.settle(context.accountId, context.reservationId, keyAction, attempt.key);
        this.operations.update((items) =>
          [...items.filter((item) => item.id !== result.data.id), result.data].sort(
            (a, b) => a.sequence - b.sequence,
          ),
        );
        this.success.set(result.notice);
        this.comments.set('');
        this.observations.set('');
        this.readRevision++;
        void this.loadOperations();
        this.stateChanged.emit();
      },
      () => {
        this.attempts.markUncertain(
          context.accountId,
          context.reservationId,
          keyAction,
          attempt.key,
        );
        this.stateChanged.emit();
      },
      true,
      (isCurrent) => {
        this.attempts.settle(context.accountId, context.reservationId, keyAction, attempt.key);
        if (isCurrent) this.stateChanged.emit();
      },
    );
  }
  async loadEvidence(evidenceId: string): Promise<void> {
    if (this.objectUrls.has(evidenceId)) return;
    const context = this.context();
    if (!context) return;
    await this.execute(
      context,
      () => this.api.evidence(context.token, context.reservationId, evidenceId),
      (blob) => {
        if (blob.type !== 'image/png') throw new Error('Go no devolvió la evidencia PNG esperada.');
        const url = URL.createObjectURL(blob);
        this.objectUrls.set(evidenceId, url);
        this.evidenceUrls.update((urls) => ({ ...urls, [evidenceId]: url }));
      },
    );
  }

  private async contractAction(action: 'sign' | 'reject'): Promise<void> {
    const contract = this.contract();
    const context = this.context();
    if (!contract || !context || !this.canSign(contract)) return;
    await this.execute(
      context,
      async () =>
        action === 'sign'
          ? this.api.signContract(context.token, contract.id)
          : this.api.rejectContract(context.token, contract.id),
      (result) => {
        this.contract.set(result.data);
        this.success.set(result.notice);
        this.stateChanged.emit();
      },
      undefined,
      true,
      (isCurrent) => {
        if (isCurrent) this.stateChanged.emit();
      },
    );
  }
  private async loadOperations(): Promise<void> {
    const context = this.context();
    if (!context || !this.isParticipant(this.reservation())) return;
    const revision = ++this.readRevision;
    await this.execute(
      context,
      () => this.api.operations(context.token, context.reservationId),
      (result) => {
        if (revision === this.readRevision) this.operations.set(result.data.items);
      },
      undefined,
      false,
    );
  }
  private async execute<T>(
    context: LifecycleContext,
    request: () => Promise<T>,
    apply: (value: T) => void,
    onUncertain?: () => void,
    showBusy = true,
    onDefinitive?: (isCurrent: boolean) => void,
  ): Promise<void> {
    if (showBusy) this.busy.set(true);
    this.error.set('');
    this.success.set('');
    try {
      const result = await request();
      if (!this.current(context)) return;
      apply(result);
    } catch (error) {
      if (
        error instanceof BookingApiError &&
        error.status !== 401 &&
        error.status !== undefined &&
        error.status >= 400 &&
        error.status < 500
      )
        onDefinitive?.(this.current(context));
      if (error instanceof BookingApiError && error.status === 401) {
        if (
          context.token === this.identity.accessToken() &&
          context.accountId === this.identity.session()?.account_id
        )
          this.identity.expireIfCurrent(context.token);
        return;
      }
      if (this.current(context)) {
        const uncertain =
          !(error instanceof BookingApiError) ||
          error.status === undefined ||
          error.status === 0 ||
          error.status >= 500;
        if (uncertain) onUncertain?.();
        this.error.set(
          uncertain
            ? `${messageFor(error)} La respuesta quedó incierta; reintenta sin cambiar el contenido.`
            : messageFor(error),
        );
      }
    } finally {
      if (showBusy && this.current(context)) this.busy.set(false);
    }
  }
  private context(): LifecycleContext | null {
    const token = this.identity.accessToken();
    const accountId = this.identity.session()?.account_id ?? null;
    return token &&
      accountId &&
      accountId === this.accountId &&
      this.reservation().id === this.reservationId
      ? { token, accountId, reservationId: this.reservationId, generation: this.generation }
      : null;
  }
  private current(context: LifecycleContext): boolean {
    return (
      !this.destroyed &&
      context.generation === this.generation &&
      context.token === this.identity.accessToken() &&
      context.accountId === this.identity.session()?.account_id &&
      context.reservationId === this.reservation().id
    );
  }
  private isParticipant(item: LocalBookingDetail): boolean {
    return this.isRenter(item) || this.isHost(item);
  }
  private isRenter(item: LocalBookingDetail): boolean {
    return this.identity.session()?.account_id === item.renter_id;
  }
  private isHost(item: LocalBookingDetail): boolean {
    return this.identity.session()?.account_id === item.host_id;
  }
  private clearPrivate(): void {
    this.contract.set(null);
    this.operations.set([]);
    this.error.set('');
    this.success.set('');
    this.busy.set(false);
    this.comments.set('');
    this.observations.set('');
    this.revokeUrls();
  }
  private revokeUrls(): void {
    for (const url of this.objectUrls.values()) URL.revokeObjectURL(url);
    this.objectUrls.clear();
    this.evidenceUrls.set({});
  }
}

function messageFor(error: unknown): string {
  return error instanceof Error ? error.message : 'Go no pudo completar esta operación.';
}
