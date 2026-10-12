import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { BookingsFacade } from './application/bookings-facade';

@Component({
  providers: [BookingsFacade],
  selector: 'app-bookings-page',
  imports: [UiButton, RouterLink],
  templateUrl: './bookings-page.html',
  styleUrl: './bookings-page.scss',
})
export class BookingsPage {
  private readonly facade = inject(BookingsFacade);
  get session() {
    return this.facade.session;
  }
  get reservations() {
    return this.facade.reservations;
  }
  get loading() {
    return this.facade.loading;
  }
  get error() {
    return this.facade.error;
  }
  get notice() {
    return this.facade.notice;
  }
  readonly load = this.facade.load.bind(this.facade);
  readonly participant = this.facade.participant.bind(this.facade);
  readonly stateLabel = this.facade.stateLabel.bind(this.facade);
  readonly money = this.facade.money.bind(this.facade);
  readonly when = this.facade.when.bind(this.facade);
}
