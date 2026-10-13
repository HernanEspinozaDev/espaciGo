import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
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
  readonly onLogin = this.facade.onLogin.bind(this.facade);
  readonly onRegister = this.facade.onRegister.bind(this.facade);
  readonly onReissue = this.facade.onReissue.bind(this.facade);
  readonly onVerify = this.facade.onVerify.bind(this.facade);
  readonly onRecover = this.facade.onRecover.bind(this.facade);
  readonly onConsumeRecovery = this.facade.onConsumeRecovery.bind(this.facade);
  readonly onChangePassword = this.facade.onChangePassword.bind(this.facade);
  readonly getRoleLabel = this.facade.getRoleLabel.bind(this.facade);
}
