import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import { ModelControls } from '../../shared/forms/model-controls';
import { AdminFacade } from './application/admin-facade';

@Component({
  providers: [ModelControls, AdminFacade],
  selector: 'app-admin-page',
  imports: [UiButton, ReactiveFormsModule],
  templateUrl: './admin-page.html',
  styleUrl: './admin-page.scss',
})
export class AdminPage {
  private readonly controls = inject(ModelControls);
  readonly controlFor = this.controls.forProperty.bind(this.controls);
  readonly signalControl = this.controls.forSignal.bind(this.controls);
  private readonly facade = inject(AdminFacade);
  get sections() {
    return this.facade.sections;
  }
  get active() {
    return this.facade.active;
  }
  get items() {
    return this.facade.items;
  }
  get result() {
    return this.facade.result;
  }
  get cursor() {
    return this.facade.cursor;
  }
  get nextCursor() {
    return this.facade.nextCursor;
  }
  get filter() {
    return this.facade.filter;
  }
  get accountId() {
    return this.facade.accountId;
  }
  get resourceId() {
    return this.facade.resourceId;
  }
  get fromDate() {
    return this.facade.fromDate;
  }
  get untilDate() {
    return this.facade.untilDate;
  }
  get timezone() {
    return this.facade.timezone;
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
  get success() {
    return this.facade.success;
  }
  get reason() {
    return this.facade.reason;
  }
  get selected() {
    return this.facade.selected;
  }
  get selectedAction() {
    return this.facade.selectedAction;
  }
  get evidenceItems() {
    return this.facade.evidenceItems;
  }
  get evidenceCaseId() {
    return this.facade.evidenceCaseId;
  }
  get evidenceUrl() {
    return this.facade.evidenceUrl;
  }
  get evidenceError() {
    return this.facade.evidenceError;
  }
  readonly visibleItems = this.facade.visibleItems.bind(this.facade);
  readonly selectSection = this.facade.selectSection.bind(this.facade);
  readonly setVerificationQueue = this.facade.setVerificationQueue.bind(this.facade);
  readonly load = this.facade.load.bind(this.facade);
  readonly applyFilter = this.facade.applyFilter.bind(this.facade);
  readonly select = this.facade.select.bind(this.facade);
  readonly loadEvidence = this.facade.loadEvidence.bind(this.facade);
  readonly openEvidence = this.facade.openEvidence.bind(this.facade);
  readonly deleteEvidence = this.facade.deleteEvidence.bind(this.facade);
  readonly submitAction = this.facade.submitAction.bind(this.facade);
  readonly lookupAccount = this.facade.lookupAccount.bind(this.facade);
  readonly openDetail = this.facade.openDetail.bind(this.facade);
  readonly runPrivacyOperation = this.facade.runPrivacyOperation.bind(this.facade);
  readonly runReport = this.facade.runReport.bind(this.facade);
  readonly exportAudit = this.facade.exportAudit.bind(this.facade);
  readonly nextPage = this.facade.nextPage.bind(this.facade);
  readonly choosePrivacyRequest = this.facade.choosePrivacyRequest.bind(this.facade);
  readonly pretty = this.facade.pretty.bind(this.facade);
  readonly title = this.facade.title.bind(this.facade);
  readonly itemId = this.facade.itemId.bind(this.facade);
  readonly actionsForTemplate = this.facade.actionsForTemplate.bind(this.facade);
  readonly accountRecord = this.facade.accountRecord.bind(this.facade);
  readonly requiresReason = this.facade.requiresReason.bind(this.facade);
  readonly reasonCodes = this.facade.reasonCodes.bind(this.facade);
}
