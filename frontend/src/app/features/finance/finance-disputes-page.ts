import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { FinanceDisputesFacade } from './application/finance-disputes-facade';

@Component({
  providers: [FinanceDisputesFacade],
  selector: 'app-finance-disputes-page',
  imports: [UiButton, RouterLink],
  templateUrl: './finance-disputes-page.html',
  styleUrl: './finance-disputes-page.scss',
})
export class FinanceDisputesPage {
  private readonly facade = inject(FinanceDisputesFacade);
  get id() {
    return this.facade.id;
  }
  set id(value: FinanceDisputesFacade['id']) {
    this.facade.id = value;
  }
  get admin() {
    return this.facade.admin;
  }
  get disputes() {
    return this.facade.disputes;
  }
  get adminReservationId() {
    return this.facade.adminReservationId;
  }
  get claims() {
    return this.facade.claims;
  }
  get claim() {
    return this.facade.claim;
  }
  get selectedClaim() {
    return this.facade.selectedClaim;
  }
  get financialDecision() {
    return this.facade.financialDecision;
  }
  get guarantee() {
    return this.facade.guarantee;
  }
  get settlement() {
    return this.facade.settlement;
  }
  get receipt() {
    return this.facade.receipt;
  }
  get evidenceUrl() {
    return this.facade.evidenceUrl;
  }
  get description() {
    return this.facade.description;
  }
  get outcome() {
    return this.facade.outcome;
  }
  get reasonCode() {
    return this.facade.reasonCode;
  }
  get decision() {
    return this.facade.decision;
  }
  get guaranteeKind() {
    return this.facade.guaranteeKind;
  }
  get guaranteeAmount() {
    return this.facade.guaranteeAmount;
  }
  get deductionAmount() {
    return this.facade.deductionAmount;
  }
  get decisionEvidenceId() {
    return this.facade.decisionEvidenceId;
  }
  get busy() {
    return this.facade.busy;
  }
  get loading() {
    return this.facade.loading;
  }
  get notice() {
    return this.facade.notice;
  }
  get error() {
    return this.facade.error;
  }
  readonly load = this.facade.load.bind(this.facade);
  readonly openDispute = this.facade.openDispute.bind(this.facade);
  readonly openClaim = this.facade.openClaim.bind(this.facade);
  readonly defend = this.facade.defend.bind(this.facade);
  readonly syntheticEvidence = this.facade.syntheticEvidence.bind(this.facade);
  readonly guaranteeOperation = this.facade.guaranteeOperation.bind(this.facade);
  readonly settle = this.facade.settle.bind(this.facade);
  readonly reconcile = this.facade.reconcile.bind(this.facade);
  readonly decideClaim = this.facade.decideClaim.bind(this.facade);
  readonly closeDispute = this.facade.closeDispute.bind(this.facade);
  readonly evidenceItems = this.facade.evidenceItems.bind(this.facade);
  readonly viewEvidence = this.facade.viewEvidence.bind(this.facade);
  readonly resolve = this.facade.resolve.bind(this.facade);
  readonly claimResolution = this.facade.claimResolution.bind(this.facade);
  readonly refresh = this.facade.refresh.bind(this.facade);
  readonly setDescription = this.facade.setDescription.bind(this.facade);
  readonly setOutcome = this.facade.setOutcome.bind(this.facade);
  readonly setReason = this.facade.setReason.bind(this.facade);
  readonly setDecision = this.facade.setDecision.bind(this.facade);
  readonly setGuaranteeKind = this.facade.setGuaranteeKind.bind(this.facade);
  readonly setGuaranteeAmount = this.facade.setGuaranteeAmount.bind(this.facade);
  readonly setDeductionAmount = this.facade.setDeductionAmount.bind(this.facade);
  readonly setDecisionEvidenceId = this.facade.setDecisionEvidenceId.bind(this.facade);
  readonly setAdminReservationId = this.facade.setAdminReservationId.bind(this.facade);
  readonly label = this.facade.label.bind(this.facade);
  readonly json = this.facade.json.bind(this.facade);
}
