import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BookingDetailFacade } from './application/booking-detail-facade';
import { BookingConversationPanel } from './booking-conversation-panel';
import { BookingLifecyclePanel } from './booking-lifecycle-panel';

@Component({
  providers: [BookingDetailFacade],
  selector: 'app-booking-detail-page',
  imports: [UiButton, RouterLink, BookingLifecyclePanel, BookingConversationPanel],
  templateUrl: './booking-detail-page.html',
  styleUrl: './booking-detail-page.scss',
})
export class BookingDetailPage {
  private readonly facade = inject(BookingDetailFacade);
  get reservation() {
    return this.facade.reservation;
  }
  get loading() {
    return this.facade.loading;
  }
  get busy() {
    return this.facade.busy;
  }
  get error() {
    return this.facade.error;
  }
  get notice() {
    return this.facade.notice;
  }
  get preview() {
    return this.facade.preview;
  }
  get uncertainNotice() {
    return this.facade.uncertainNotice;
  }
  get paymentOutcome() {
    return this.facade.paymentOutcome;
  }
  get refundOutcome() {
    return this.facade.refundOutcome;
  }
  get reason() {
    return this.facade.reason;
  }
  get confirmCancel() {
    return this.facade.confirmCancel;
  }
  get decisionReason() {
    return this.facade.decisionReason;
  }
  readonly refreshAfterLifecycle = this.facade.refreshAfterLifecycle.bind(this.facade);
  readonly load = this.facade.load.bind(this.facade);
  readonly pay = this.facade.pay.bind(this.facade);
  readonly decide = this.facade.decide.bind(this.facade);
  readonly loadCancellationPreview = this.facade.loadCancellationPreview.bind(this.facade);
  readonly cancel = this.facade.cancel.bind(this.facade);
  readonly refund = this.facade.refund.bind(this.facade);
  readonly isRenter = this.facade.isRenter.bind(this.facade);
  readonly isHost = this.facade.isHost.bind(this.facade);
  readonly money = this.facade.money.bind(this.facade);
  readonly when = this.facade.when.bind(this.facade);
  readonly resultLabel = this.facade.resultLabel.bind(this.facade);
}
