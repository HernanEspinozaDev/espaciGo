import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { SpaceDetailFacade } from './application/space-detail-facade';

@Component({
  providers: [SpaceDetailFacade],
  selector: 'app-space-detail-page',
  imports: [UiButton, RouterLink],
  templateUrl: './space-detail-page.html',
  styleUrl: './space-detail-page.scss',
})
export class SpaceDetailPage {
  private readonly facade = inject(SpaceDetailFacade);
  get space() {
    return this.facade.space;
  }
  get profile() {
    return this.facade.profile;
  }
  get date() {
    return this.facade.date;
  }
  get duration() {
    return this.facade.duration;
  }
  get options() {
    return this.facade.options;
  }
  get selected() {
    return this.facade.selected;
  }
  get quote() {
    return this.facade.quote;
  }
  get notice() {
    return this.facade.notice;
  }
  get busy() {
    return this.facade.busy;
  }
  get error() {
    return this.facade.error;
  }
  get requestingReservation() {
    return this.facade.requestingReservation;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  readonly chooseDate = this.facade.chooseDate.bind(this.facade);
  readonly chooseDuration = this.facade.chooseDuration.bind(this.facade);
  readonly chooseInterval = this.facade.chooseInterval.bind(this.facade);
  readonly durationOptions = this.facade.durationOptions.bind(this.facade);
  readonly loadAvailability = this.facade.loadAvailability.bind(this.facade);
  readonly createQuote = this.facade.createQuote.bind(this.facade);
  readonly requestReservation = this.facade.requestReservation.bind(this.facade);
  readonly formatInterval = this.facade.formatInterval.bind(this.facade);
  readonly formatClp = this.facade.formatClp.bind(this.facade);
  readonly formatDateTime = this.facade.formatDateTime.bind(this.facade);
  readonly attributeLabel = this.facade.attributeLabel.bind(this.facade);
  readonly attributeValue = this.facade.attributeValue.bind(this.facade);
}
