import { Component, computed, DestroyRef, inject, signal } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Meta } from '@angular/platform-browser';
import {
  FormField,
  SchemaPathTree,
  email,
  form,
  minLength,
  required,
  submit,
  validate,
} from '@angular/forms/signals';
import { IdentityApi } from '../../core/identity/identity-api';
import { identityErrorMessage } from '../../core/identity/identity-errors';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import type { TermsResponse } from '../../../generated/api/types.gen';

type IdentityMode = 'login' | 'register' | 'verify' | 'recovery' | 'change-password' | 'account';

@Component({
  imports: [FormField, RouterLink],
  selector: 'app-identity-page',
  styleUrl: './identity-page.scss',
  templateUrl: './identity-page.html',
})
export class IdentityPage {
  private readonly api = inject(IdentityApi);
  private readonly identity = inject(IdentitySession);
  private readonly privateSessionState = inject(PrivateSessionState);
  private readonly router = inject(Router);
  private readonly meta = inject(Meta);
  private readonly destroyRef = inject(DestroyRef);
  protected readonly mode = inject(ActivatedRoute).snapshot.data['identityMode'] as IdentityMode;
  protected readonly busy = signal(false);
  protected readonly errorMessage = signal('');
  protected readonly successMessage = signal('');
  protected readonly terms = signal<TermsResponse['items']>([]);
  protected readonly termVersions = computed(() =>
    this.terms().filter((item) => item.type === 'terminos'),
  );
  protected readonly privacyVersions = computed(() =>
    this.terms().filter((item) => item.type === 'privacidad'),
  );
  protected readonly termsState = signal<'loading' | 'ready' | 'error'>('loading');
  protected readonly session = this.identity.session;
  private unregisterSensitiveState!: () => void;
  private requestGeneration = 0;
  private destroyed = false;

  protected readonly loginModel = signal({ email: '', password: '' });
  protected readonly loginForm = form(this.loginModel, (path) => {
    required(path.email, { message: 'Escribe tu correo.' });
    email(path.email, { message: 'Escribe un correo válido.' });
    required(path.password, { message: 'Escribe tu contraseña.' });
  });

  protected readonly registerModel = signal({
    email: '',
    password: '',
    confirmPassword: '',
    usePreference: '' as '' | 'ofrecer' | 'arrendar',
    acceptTerms: false,
    acceptPrivacy: false,
  });
  protected readonly registerForm = form(this.registerModel, (path) => {
    required(path.email, { message: 'Escribe tu correo.' });
    email(path.email, { message: 'Escribe un correo válido.' });
    passwordRules(path.password);
    required(path.confirmPassword, { message: 'Confirma tu contraseña.' });
    validate(path.confirmPassword, ({ value, valueOf }) =>
      value() !== valueOf(path.password)
        ? { kind: 'password-mismatch', message: 'Las contraseñas no coinciden.' }
        : undefined,
    );
    validate(path.usePreference, ({ value }) =>
      value() !== 'ofrecer' && value() !== 'arrendar'
        ? { kind: 'preference-required', message: 'Elige una preferencia inicial.' }
        : undefined,
    );
    required(path.acceptTerms, { message: 'Confirma la versión sintética de términos.' });
    required(path.acceptPrivacy, { message: 'Confirma la versión sintética de privacidad.' });
  });

  protected readonly emailModel = signal({ email: '' });
  protected readonly emailForm = form(this.emailModel, (path) => {
    required(path.email, { message: 'Escribe tu correo.' });
    email(path.email, { message: 'Escribe un correo válido.' });
  });

  protected readonly tokenModel = signal({ tokenId: '', token: '' });
  protected readonly tokenForm = form(this.tokenModel, (path) => {
    required(path.tokenId, { message: 'Escribe el identificador recibido en el correo.' });
    required(path.token, { message: 'Escribe el código recibido en el correo.' });
  });

  protected readonly recoveryModel = signal({
    tokenId: '',
    token: '',
    newPassword: '',
    confirmPassword: '',
  });
  protected readonly recoveryForm = form(this.recoveryModel, (path) => {
    required(path.tokenId, { message: 'Escribe el identificador recibido en el correo.' });
    required(path.token, { message: 'Escribe el código recibido en el correo.' });
    passwordRules(path.newPassword);
    required(path.confirmPassword, { message: 'Confirma tu nueva contraseña.' });
    validate(path.confirmPassword, ({ value, valueOf }) =>
      value() !== valueOf(path.newPassword)
        ? { kind: 'password-mismatch', message: 'Las contraseñas no coinciden.' }
        : undefined,
    );
  });

  protected readonly changeModel = signal({
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });
  protected readonly changeForm = form(this.changeModel, (path) => {
    required(path.currentPassword, { message: 'Escribe tu contraseña actual.' });
    passwordRules(path.newPassword);
    required(path.confirmPassword, { message: 'Confirma tu nueva contraseña.' });
    validate(path.confirmPassword, ({ value, valueOf }) =>
      value() !== valueOf(path.newPassword)
        ? { kind: 'password-mismatch', message: 'Las contraseñas no coinciden.' }
        : undefined,
    );
  });

  constructor() {
    this.unregisterSensitiveState = this.privateSessionState.register(() => {
      this.requestGeneration++;
      this.loginModel.update((model) => ({ ...model, password: '' }));
      this.registerModel.update((model) => ({ ...model, password: '', confirmPassword: '' }));
      this.tokenModel.set({ tokenId: '', token: '' });
      this.recoveryModel.set({ tokenId: '', token: '', newPassword: '', confirmPassword: '' });
      this.changeModel.set({ currentPassword: '', newPassword: '', confirmPassword: '' });
      this.errorMessage.set('');
      this.successMessage.set('');
      this.busy.set(false);
    });
    this.meta.updateTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
      this.requestGeneration++;
      this.unregisterSensitiveState();
      this.meta.removeTag("name='robots'");
    });
    if (this.mode === 'register') void this.loadTerms();
  }

  protected async onLogin(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.loginForm, async () => {
      const { email: address, password } = this.loginModel();
      await this.performRequest(
        () => this.identity.signIn(address, password),
        async () => {
          this.successMessage.set('Sesión confirmada por Go.');
          await this.router.navigateByUrl('/mi-cuenta');
        },
      );
    });
  }

  protected async onRegister(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.registerForm, async () => {
      const model = this.registerModel();
      const versionIds = this.terms()
        .filter(
          (item) =>
            (item.type === 'terminos' && model.acceptTerms) ||
            (item.type === 'privacidad' && model.acceptPrivacy),
        )
        .map((item) => item.id);
      if (
        !versionIds.some((id) =>
          this.terms().some((term) => term.id === id && term.type === 'terminos'),
        )
      ) {
        this.errorMessage.set(
          'Go no publicó una versión sintética de términos; no es posible crear la cuenta.',
        );
        return;
      }
      await this.performRequest(
        () =>
          this.api.register({
            email: model.email,
            password: model.password,
            use_preference: model.usePreference as 'ofrecer' | 'arrendar',
            terms_version_ids: versionIds,
          }),
        (result) => {
          this.emailModel.set({ email: model.email });
          this.registerModel.update((current) => ({
            ...current,
            password: '',
            confirmPassword: '',
          }));
          this.successMessage.set(
            result.status === 'verification_pending'
              ? 'Go creó la cuenta; el correo no pudo confirmarse. Solicita la verificación de nuevo.'
              : 'Go creó la cuenta. Revisa el buzón local para verificar tu correo antes de iniciar sesión.',
          );
        },
      );
    });
  }

  protected async onReissue(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.emailForm, async () => {
      const address = this.emailModel().email;
      await this.performRequest(
        () => this.api.reissueVerification(address),
        () =>
          this.successMessage.set(
            'Go aceptó la solicitud. Si corresponde, Mailpit recibirá un nuevo correo de verificación.',
          ),
      );
    });
  }

  protected async onVerify(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.tokenForm, async () => {
      const model = this.tokenModel();
      await this.performRequest(
        () => this.api.verifyEmail(model.tokenId, model.token),
        () => {
          this.tokenModel.set({ tokenId: '', token: '' });
          this.successMessage.set('Correo verificado por Go. Ya puedes iniciar sesión.');
        },
      );
    });
  }

  protected async onRecover(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.emailForm, async () => {
      const address = this.emailModel().email;
      await this.performRequest(
        () => this.api.recover(address),
        (accepted) => this.successMessage.set(accepted.message),
      );
    });
  }

  protected async onConsumeRecovery(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.recoveryForm, async () => {
      const model = this.recoveryModel();
      await this.performRequest(
        () =>
          this.identity.consumeRecovery(
            model.tokenId,
            model.token,
            model.newPassword,
            model.confirmPassword,
          ),
        () => undefined,
      );
    });
  }

  protected async onChangePassword(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.changeForm, async () => {
      const generation = this.begin();
      const request = this.identity.changePassword(
        this.changeModel().currentPassword,
        this.changeModel().newPassword,
        this.changeModel().confirmPassword,
      );
      const stateGeneration = this.privateSessionState.generation();
      try {
        await request;
        if (!this.destroyed) await this.router.navigateByUrl('/acceso');
      } catch (error) {
        if (this.isCurrentRequest(generation, stateGeneration)) {
          this.errorMessage.set(identityErrorMessage(error));
        }
      } finally {
        if (this.isCurrentRequest(generation, stateGeneration)) this.busy.set(false);
      }
    });
  }

  protected getRoleLabel(role: string): string {
    return (
      (
        {
          arrendatario: 'Arrendatario',
          arrendador: 'Arrendador',
          administrador: 'Administración',
        } as Record<string, string>
      )[role] ?? 'Rol'
    );
  }

  private async loadTerms(): Promise<void> {
    try {
      const terms = await this.api.terms();
      this.terms.set(terms.items);
      this.termsState.set(terms.items.some((item) => item.type === 'terminos') ? 'ready' : 'error');
    } catch {
      this.termsState.set('error');
    }
  }

  private begin(): number {
    this.errorMessage.set('');
    this.successMessage.set('');
    this.busy.set(true);
    return ++this.requestGeneration;
  }

  private async performRequest<T>(
    request: () => Promise<T>,
    onSuccess: (value: T) => void | Promise<void>,
  ): Promise<void> {
    this.begin();
    let generation = this.requestGeneration;
    let stateGeneration = this.privateSessionState.generation();
    try {
      // IdentitySession may clear private state synchronously when a new login starts.
      const pending = request();
      generation = this.requestGeneration;
      stateGeneration = this.privateSessionState.generation();
      const value = await pending;
      if (!this.isCurrentRequest(generation, stateGeneration)) return;
      await onSuccess(value);
    } catch (error) {
      if (this.isCurrentRequest(generation, stateGeneration)) {
        this.errorMessage.set(identityErrorMessage(error));
      }
    } finally {
      if (this.isCurrentRequest(generation, stateGeneration)) this.busy.set(false);
    }
  }

  private isCurrentRequest(generation: number, stateGeneration: number): boolean {
    return (
      !this.destroyed &&
      generation === this.requestGeneration &&
      stateGeneration === this.privateSessionState.generation()
    );
  }
}

function passwordRules(path: SchemaPathTree<{ password: string }>['password']): void {
  required(path, { message: 'Escribe una contraseña.' });
  minLength(path, 8, { message: 'Usa al menos 8 caracteres.' });
  validate(path, ({ value }) => {
    const password = value();
    const valid =
      Array.from(password).length >= 8 &&
      new TextEncoder().encode(password).length <= 72 &&
      /\p{Lu}/u.test(password) &&
      /\p{N}/u.test(password) &&
      /[\p{P}\p{S}]/u.test(password);
    return valid
      ? undefined
      : {
          kind: 'password-policy',
          message: 'Incluye mayúscula, número y símbolo; máximo 72 bytes UTF-8.',
        };
  });
}
