import { UiButton } from '../../shared/ui/button/button';
import { Component, ElementRef, inject, signal, viewChild } from '@angular/core';
import { FormField } from '@angular/forms/signals';
import { RouterLink } from '@angular/router';
import { IdentityFacade } from './application/identity-facade';

@Component({
  providers: [IdentityFacade],
  imports: [UiButton, FormField, RouterLink],
  selector: 'app-identity-page',
  styleUrl: './identity-page.scss',
  templateUrl: './identity-page.html',
})
export class IdentityPage {
  private readonly facade = inject(IdentityFacade);
  readonly showPassword = signal(false);
  readonly registrationSubmitted = this.facade.registrationSubmitted;
  readonly recoveryRequested = this.facade.recoveryRequested;
  readonly reissueRequested = this.facade.reissueRequested;
  readonly recoveryComplete = this.facade.recoveryComplete;
  readonly linkState = this.facade.linkState;
  readonly verificationSuggested = this.facade.verificationSuggested;
  readonly reloadTerms = this.facade.loadTerms.bind(this.facade);
  readonly onLoginReissue = this.facade.onLoginReissue.bind(this.facade);

  get mode() {
    return this.facade.mode;
  }
  get busy() {
    return this.facade.busy;
  }
  get errorMessage() {
    return this.facade.errorMessage;
  }
  get successMessage() {
    return this.facade.successMessage;
  }
  get registrationNeedsRetry() {
    return this.facade.registrationNeedsRetry;
  }
  get loginConfirmed() {
    return this.facade.loginConfirmed;
  }
  get terms() {
    return this.facade.terms;
  }
  get termVersions() {
    return this.facade.termVersions;
  }
  get privacyVersions() {
    return this.facade.privacyVersions;
  }
  get termsState() {
    return this.facade.termsState;
  }
  get session() {
    return this.facade.session;
  }
  get loginModel() {
    return this.facade.loginModel;
  }
  get loginForm() {
    return this.facade.loginForm;
  }
  get registerModel() {
    return this.facade.registerModel;
  }
  get registerForm() {
    return this.facade.registerForm;
  }
  get emailModel() {
    return this.facade.emailModel;
  }
  get emailForm() {
    return this.facade.emailForm;
  }
  get tokenModel() {
    return this.facade.tokenModel;
  }
  get tokenForm() {
    return this.facade.tokenForm;
  }
  get recoveryModel() {
    return this.facade.recoveryModel;
  }
  get recoveryForm() {
    return this.facade.recoveryForm;
  }
  get changeModel() {
    return this.facade.changeModel;
  }
  get changeForm() {
    return this.facade.changeForm;
  }
  readonly identityQueryParams = this.facade.identityQueryParams.bind(this.facade);
  readonly contextualCancelUrl = this.facade.contextualCancelUrl.bind(this.facade);
  readonly onLogin = this.facade.onLogin.bind(this.facade);
  readonly onLoginInput = this.facade.onLoginInput.bind(this.facade);
  private readonly heading = viewChild<ElementRef<HTMLHeadingElement>>('identityHeading');
  private readonly element = inject<ElementRef<HTMLElement>>(ElementRef);
  readonly onRegister = async (event: SubmitEvent): Promise<void> => {
    await this.facade.onRegister(event);
    setTimeout(() => {
      if (this.registrationSubmitted()) this.heading()?.nativeElement.focus();
      else this.element.nativeElement.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
    });
  };
  readonly onReissue = this.facade.onReissue.bind(this.facade);
  readonly onVerify = this.facade.onVerify.bind(this.facade);
  readonly onRecover = this.facade.onRecover.bind(this.facade);
  readonly onConsumeRecovery = async (event: SubmitEvent): Promise<void> => {
    await this.facade.onConsumeRecovery(event);
    setTimeout(() => {
      if (this.recoveryComplete()) this.heading()?.nativeElement.focus();
      else this.element.nativeElement.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
    });
  };
  readonly onChangePassword = this.facade.onChangePassword.bind(this.facade);
  readonly getRoleLabel = this.facade.getRoleLabel.bind(this.facade);
}
