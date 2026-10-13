import { UiButton } from '../../shared/ui/button/button';
import { Component, inject, input, output } from '@angular/core';
import type { LocalBookingDetail } from '../../core/contracts/models';
import { BookingLifecycleFacade } from './application/booking-lifecycle-facade';
@Component({
  imports: [UiButton],
  providers: [BookingLifecycleFacade],
  selector: 'app-booking-lifecycle-panel',
  template: `
    <section class="panel lifecycle" aria-labelledby="contract-title">
      <h2 id="contract-title">Contrato de ensayo</h2>
      <p>
        ENSAYO SINTÉTICO LOCAL — SIN VALIDEZ JURÍDICA. Go confirma las firmas y permisos de cada
        participante.
      </p>
      @if (contract(); as value) {
        <p class="status" role="status">
          Versión {{ value.version }} · {{ contractState(value.state) }} · SHA-256
          {{ value.sha256 }}
        </p>
        <h3>Firmas</h3>
        <ul>
          @for (signature of value.signatures; track signature.signer_id) {
            <li>
              {{ signature.role === 'anfitrion' ? 'Anfitrión' : 'Arrendatario' }}:
              {{ signature.state }}
              @if (signature.reason) {
                · {{ signature.reason }}
              }
            </li>
          }
        </ul>
        <details>
          <summary>Snapshot del contrato de ensayo</summary>
          <pre>{{ value.snapshot }}</pre>
        </details>
        <h3>Historial de contrato</h3>
        <ol>
          @for (event of value.history; track event.sequence) {
            <li>
              {{ event.action }} · {{ time(event.at) }}
              @if (event.reason) {
                · {{ event.reason }}
              }
            </li>
          }
        </ol>
        <div class="buttons">
          <button
            appButton="secondary"
            type="button"
            class="button secondary"
            [disabled]="busy()"
            (click)="refreshContract()"
          >
            Consultar estado confirmado
          </button>
          @if (canSign(value)) {
            <button
              appButton="primary"
              type="button"
              class="button"
              [disabled]="busy()"
              (click)="sign()"
            >
              Firmar versión</button
            ><button
              appButton="secondary"
              type="button"
              class="button secondary"
              [disabled]="busy()"
              (click)="reject()"
            >
              Rechazar versión
            </button>
          }
          @if (value.state === 'firmado') {
            <button
              appButton="secondary"
              type="button"
              class="button secondary"
              [disabled]="busy()"
              (click)="downloadPdf()"
            >
              Descargar PDF privado
            </button>
          }
        </div>
      } @else {
        <p>La generación/consulta aparece cuando la reserva está aprobada por Go.</p>
        @if (canOpenContract()) {
          <button
            appButton="primary"
            type="button"
            class="button"
            [disabled]="busy()"
            (click)="openContract()"
          >
            Generar o consultar contrato
          </button>
        }
      }
      @if (busy()) {
        <p role="status">Consultando a Go…</p>
      }
      @if (error()) {
        <p class="error" role="alert">{{ error() }}</p>
      }
      @if (success()) {
        <p class="success" role="status">{{ success() }}</p>
      }
    </section>

    <section class="panel lifecycle" aria-labelledby="operation-title">
      <h2 id="operation-title">Llegada, salida y recepción</h2>
      <p>Go genera imágenes de ensayo privadas. No adjuntes documentos ni archivos propios.</p>
      @if (hasUncertainAttempt()) {
        <p role="status">
          Go aún no confirmó una operación. Reintentar reutiliza la clave y el contenido original
          guardado en memoria; los campos privados permanecen limpios.
        </p>
      }
      @if (canCheckIn()) {
        <label
          >Comentario sintético (opcional)<textarea
            [value]="comments()"
            (input)="comments.set($any($event.target).value)"
            rows="2"
          ></textarea></label
        ><button
          appButton="primary"
          class="button"
          type="button"
          [disabled]="busy()"
          (click)="record('check-in')"
        >
          {{ retryLabel('checkin', 'check-in') }}
        </button>
      }
      @if (canCheckOut()) {
        <label
          >Comentario sintético (opcional)<textarea
            [value]="comments()"
            (input)="comments.set($any($event.target).value)"
            rows="2"
          ></textarea></label
        ><button
          appButton="primary"
          class="button"
          type="button"
          [disabled]="busy()"
          (click)="record('check-out')"
        >
          {{ retryLabel('checkout', 'check-out') }}
        </button>
      }
      @if (canReceive()) {
        <label
          >Observación de recepción (opcional)<textarea
            [value]="observations()"
            (input)="observations.set($any($event.target).value)"
            rows="2"
          ></textarea></label
        ><button
          appButton="primary"
          class="button"
          type="button"
          [disabled]="busy()"
          (click)="record('reception')"
        >
          {{ retryLabel('reception', 'recepción') }}
        </button>
      }
      @if (operations().length) {
        <h3>Historial de operación confirmado por Go</h3>
        <ol class="operations">
          @for (operation of operations(); track operation.id) {
            <li>
              <strong>{{ operationLabel(operation.kind) }}</strong> ·
              {{ time(operation.occurred_at) }} · {{ operation.result }}
              @if (operation.claim_deadline_at) {
                <p>Plazo local informado por Go: {{ time(operation.claim_deadline_at) }}</p>
              }
              @if (operation.comments) {
                <p>{{ operation.comments }}</p>
              }
              @if (operation.observations) {
                <p>{{ operation.observations }}</p>
              }
              @for (file of operation.evidence; track file.id) {
                <button
                  appButton="secondary"
                  type="button"
                  class="button secondary"
                  [disabled]="busy()"
                  (click)="loadEvidence(file.id)"
                >
                  Ver evidencia sintética · {{ file.mime_type }}
                </button>
                @if (evidenceUrls()[file.id]; as url) {
                  <img
                    class="evidence"
                    [src]="url"
                    alt="Evidencia sintética privada de la operación"
                  />
                }
              }
            </li>
          }
        </ol>
      } @else if (!busy()) {
        <p>No hay operaciones registradas para esta reserva.</p>
      }
    </section>
  `,
  styleUrl: './booking-lifecycle-panel.scss',
})
export class BookingLifecyclePanel {
  readonly reservation = input.required<LocalBookingDetail>();
  readonly stateChanged = output<void>();
  private readonly facade = inject(BookingLifecycleFacade);
  constructor() {
    this.facade.connectReservation(this.reservation, () => this.stateChanged.emit());
  }
  get contract() {
    return this.facade.contract;
  }
  get operations() {
    return this.facade.operations;
  }
  get evidenceUrls() {
    return this.facade.evidenceUrls;
  }
  get busy() {
    return this.facade.busy;
  }
  get error() {
    return this.facade.error;
  }
  get success() {
    return this.facade.success;
  }
  get comments() {
    return this.facade.comments;
  }
  get observations() {
    return this.facade.observations;
  }
  readonly retryLabel = this.facade.retryLabel.bind(this.facade);
  readonly hasUncertainAttempt = this.facade.hasUncertainAttempt.bind(this.facade);
  readonly canOpenContract = this.facade.canOpenContract.bind(this.facade);
  readonly canSign = this.facade.canSign.bind(this.facade);
  readonly canCheckIn = this.facade.canCheckIn.bind(this.facade);
  readonly canCheckOut = this.facade.canCheckOut.bind(this.facade);
  readonly canReceive = this.facade.canReceive.bind(this.facade);
  readonly contractState = this.facade.contractState.bind(this.facade);
  readonly operationLabel = this.facade.operationLabel.bind(this.facade);
  readonly time = this.facade.time.bind(this.facade);
  readonly openContract = this.facade.openContract.bind(this.facade);
  readonly refreshContract = this.facade.refreshContract.bind(this.facade);
  readonly sign = this.facade.sign.bind(this.facade);
  readonly reject = this.facade.reject.bind(this.facade);
  readonly downloadPdf = this.facade.downloadPdf.bind(this.facade);
  readonly record = this.facade.record.bind(this.facade);
  readonly loadEvidence = this.facade.loadEvidence.bind(this.facade);
}
