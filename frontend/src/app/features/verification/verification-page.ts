import { UiState } from '../../shared/ui/state/state';
import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { VerificationFacade } from './application/verification-facade';

@Component({
  providers: [VerificationFacade],
  selector: 'app-verification-page',
  imports: [UiState, UiButton, RouterLink],
  templateUrl: './verification-page.html',
  styleUrl: './verification-page.scss',
})
export class VerificationPage {
  private readonly facade = inject(VerificationFacade);
  get session() {
    return this.facade.session;
  }
  get cases() {
    return this.facade.cases;
  }
  get eligibility() {
    return this.facade.eligibility;
  }
  get selected() {
    return this.facade.selected;
  }
  get history() {
    return this.facade.history;
  }
  get evidence() {
    return this.facade.evidence;
  }
  get evidenceUrl() {
    return this.facade.evidenceUrl;
  }
  get loading() {
    return this.facade.loading;
  }
  get detailLoading() {
    return this.facade.detailLoading;
  }
  get operation() {
    return this.facade.operation;
  }
  get error() {
    return this.facade.error;
  }
  get notice() {
    return this.facade.notice;
  }
  get createType() {
    return this.facade.createType;
  }
  get correction() {
    return this.facade.correction;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  ngOnDestroy(): void {
    this.facade.ngOnDestroy();
  }
  readonly load = this.facade.load.bind(this.facade);
  readonly select = this.facade.select.bind(this.facade);
  readonly createCase = this.facade.createCase.bind(this.facade);
  readonly retryCase = this.facade.retryCase.bind(this.facade);
  readonly createEvidence = this.facade.createEvidence.bind(this.facade);
  readonly stateLabel = this.facade.stateLabel.bind(this.facade);
  readonly typeLabel = this.facade.typeLabel.bind(this.facade);
  readonly eligibilityFor = this.facade.eligibilityFor.bind(this.facade);
  readonly formatDate = this.facade.formatDate.bind(this.facade);
}
