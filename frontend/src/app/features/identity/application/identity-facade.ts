import { computed, DestroyRef, inject, Injectable, signal } from '@angular/core';
import {
  email,
  form,
  minLength,
  required,
  SchemaPathTree,
  submit,
  validate,
} from '@angular/forms/signals';
import { Meta } from '@angular/platform-browser';
import { ActivatedRoute, Router, UrlTree } from '@angular/router';
import type { TermsResponse } from '../../../core/contracts/models';
import { IdentityPort } from '../../../core/identity/domain/ports/identity-port';
import { identityErrorMessage } from '../../../core/identity/identity-errors';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { parseIdentityReturn } from '../../../core/identity/contextual-return';

type IdentityMode = 'login' | 'register' | 'verify' | 'recovery' | 'change-password' | 'account';

@Injectable()
export class IdentityFacade {
  private readonly api = inject(IdentityPort);
  private readonly identity = inject(IdentitySession);
  private readonly privateSessionState = inject(PrivateSessionState);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly meta = inject(Meta);
  private readonly destroyRef = inject(DestroyRef);
  readonly mode = inject(ActivatedRoute).snapshot.data['identityMode'] as IdentityMode;
  readonly busy = signal(false);
  readonly errorMessage = signal('');
  readonly successMessage = signal('');
  readonly registrationNeedsRetry = signal(false);
  readonly loginConfirmed = signal(false);
  readonly terms = signal<TermsResponse['items']>([]);
  readonly termVersions = computed(() => this.terms().filter((item) => item.type === 'terminos'));
  readonly privacyVersions = computed(() =>
    this.terms().filter((item) => item.type === 'privacidad'),
  );
  readonly termsState = signal<'loading' | 'ready' | 'error'>('loading');
  readonly session = this.identity.session;
  private unregisterSensitiveState!: () => void;
  private requestGeneration = 0;
  private destroyed = false;

  readonly loginModel = signal({ email: '', password: '' });
  readonly loginForm = form(this.loginModel, (path) => {
    required(path.email, { message: 'Escribe tu correo.' });
    email(path.email, { message: 'Escribe un correo válido.' });
    required(path.password, { message: 'Escribe tu contraseña.' });
  });

  readonly registerModel = signal({
    email: '',
    password: '',
    confirmPassword: '',
    acceptTerms: false,
    acceptPrivacy: false,
  });
  readonly registerForm = form(this.registerModel, (path) => {
    required(path.email, { message: 'Escribe tu correo.' });
    email(path.email, { message: 'Escribe un correo válido.' });
    passwordRules(path.password);
    required(path.confirmPassword, { message: 'Confirma tu contraseña.' });
    validate(path.confirmPassword, ({ value, valueOf }) =>
      value() !== valueOf(path.password)
        ? { kind: 'password-mismatch', message: 'Las contraseñas no coinciden.' }
        : undefined,
    );
    required(path.acceptTerms, { message: 'Confirma la versión sintética de términos.' });
    required(path.acceptPrivacy, { message: 'Confirma la versión sintética de privacidad.' });
  });

  readonly emailModel = signal({ email: '' });
  readonly emailForm = form(this.emailModel, (path) => {
    required(path.email, { message: 'Escribe tu correo.' });
    email(path.email, { message: 'Escribe un correo válido.' });
  });

  readonly tokenModel = signal({ tokenId: '', token: '' });
  readonly tokenForm = form(this.tokenModel, (path) => {
    required(path.tokenId, { message: 'Escribe el identificador recibido en el correo.' });
    required(path.token, { message: 'Escribe el código recibido en el correo.' });
  });

  readonly recoveryModel = signal({
    tokenId: '',
    token: '',
    newPassword: '',
    confirmPassword: '',
  });
  readonly recoveryForm = form(this.recoveryModel, (path) => {
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

  readonly changeModel = signal({
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });
  readonly changeForm = form(this.changeModel, (path) => {
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
    if (this.mode === 'login' && this.route.snapshot.queryParamMap?.get('verificado') === '1') {
      this.successMessage.set('Correo verificado. Ya puedes iniciar sesión.');
    }
    if (this.mode === 'verify') {
      const fragment = this.route.snapshot.fragment ?? null;
      if (fragment) {
        const values = new URLSearchParams(fragment);
        const tokenId = values.get('token_id') ?? '';
        const token = values.get('token') ?? '';
        if (tokenId && token) this.tokenModel.set({ tokenId, token });
        void this.router.navigate([], { relativeTo: this.route, fragment: undefined, replaceUrl: true });
      }
    }
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
      this.requestGeneration++;
      this.unregisterSensitiveState();
      this.meta.removeTag("name='robots'");
    });
    if (this.mode === 'register') void this.loadTerms();
  }

  async onLogin(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    this.loginConfirmed.set(false);
    this.errorMessage.set('');
    this.successMessage.set('');
    await submit(this.loginForm, async () => {
      const { email: address, password } = this.loginModel();
      await this.performRequest(
        () => this.identity.signIn(address, password),
        async () => {
          this.loginConfirmed.set(true);
          const safeReturn = parseIdentityReturn(
            this.route.snapshot.queryParamMap?.getAll('returnUrl') ?? [],
          );
          try {
            const navigated = await this.router.navigateByUrl(
              typeof safeReturn === 'string' ? safeReturn : safeReturn?.detailUrl ?? '/mi-cuenta',
            );
            if (!navigated && !this.destroyed) {
              this.errorMessage.set(
                'Go confirmó tu sesión, pero no pudimos abrir tu cuenta. Inténtalo de nuevo.',
              );
            }
          } catch {
            if (!this.destroyed) {
              this.errorMessage.set(
                'Go confirmó tu sesión, pero no pudimos abrir tu cuenta. Recarga e inténtalo de nuevo.',
              );
            }
          }
        },
        (session) =>
          this.identity.session() === session && this.identity.status() === 'authenticated',
      );
    });
  }

  onLoginInput(): void {
    this.loginConfirmed.set(false);
    this.errorMessage.set('');
    this.successMessage.set('');
  }

  async onRegister(event: SubmitEvent): Promise<void> {
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
            terms_version_ids: versionIds,
          }),
        (result) => {
          this.registrationNeedsRetry.set(result.status === 'verification_pending');
          this.emailModel.set({ email: model.email });
          this.registerModel.update((current) => ({
            ...current,
            password: '',
            confirmPassword: '',
          }));
          this.successMessage.set(
            result.status === 'verification_pending'
              ? 'La cuenta está pendiente de verificación. No pudimos confirmar la entrega del correo; puedes solicitar otro desde aquí.'
              : 'La solicitud de registro fue aceptada para envío. Revisa tu correo para verificar la cuenta; la llegada puede tardar.',
          );
        },
      );
    });
  }

  async onReissue(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.emailForm, async () => {
      const address = this.emailModel().email;
      await this.performRequest(
        () => this.api.reissueVerification(address),
        () =>
          this.successMessage.set(
            'Solicitud procesada. Si la cuenta necesita verificación, puedes intentarlo más tarde; no podemos confirmar la entrega.',
          ),
      );
    });
  }

  async onVerify(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.tokenForm, async () => {
      const model = this.tokenModel();
      await this.performRequest(
        () => this.api.verifyEmail(model.tokenId, model.token),
        async () => {
          this.tokenModel.set({ tokenId: '', token: '' });
          const navigated = await this.router.navigateByUrl('/acceso?verificado=1');
          if (!navigated && !this.destroyed) {
            this.successMessage.set('Correo verificado. Ya puedes iniciar sesión desde Acceso.');
          }
        },
      );
    });
  }

  async onRecover(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    await submit(this.emailForm, async () => {
      const address = this.emailModel().email;
      await this.performRequest(
        () => this.api.recover(address),
        (accepted) => this.successMessage.set(accepted.message),
      );
    });
  }

  async onConsumeRecovery(event: SubmitEvent): Promise<void> {
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
        () =>
          this.recoveryModel.set({
            tokenId: '',
            token: '',
            newPassword: '',
            confirmPassword: '',
          }),
      );
    });
  }

  async onChangePassword(event: SubmitEvent): Promise<void> {
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

  getRoleLabel(role: string): string {
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

  identityQueryParams(): Record<string, string> {
    const safeReturn = parseIdentityReturn(
      this.route.snapshot.queryParamMap?.getAll('returnUrl') ?? [],
    );
    const returnUrl =
      typeof safeReturn === 'string' ? safeReturn : safeReturn?.identityReturnUrl;
    return returnUrl ? { returnUrl } : {};
  }

  contextualCancelUrl(): UrlTree | null {
    const safeReturn = parseIdentityReturn(
      this.route.snapshot.queryParamMap?.getAll('returnUrl') ?? [],
    );
    return typeof safeReturn === 'object' && safeReturn
      ? this.router.parseUrl(safeReturn.listingUrl)
      : null;
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
    acceptsSessionGenerationChange?: (value: T) => boolean,
  ): Promise<void> {
    this.begin();
    let generation = this.requestGeneration;
    let stateGeneration = this.privateSessionState.generation();
    let result!: T;
    let hasResult = false;
    const isCurrent = () =>
      this.isCurrentRequest(generation, stateGeneration) ||
      (!this.destroyed &&
        generation === this.requestGeneration &&
        hasResult &&
        acceptsSessionGenerationChange?.(result) === true);
    try {
      // IdentitySession may clear private state synchronously when a new login starts.
      const pending = request();
      generation = this.requestGeneration;
      stateGeneration = this.privateSessionState.generation();
      result = await pending;
      hasResult = true;
      if (!isCurrent()) return;
      await onSuccess(result);
    } catch (error) {
      if (isCurrent()) this.errorMessage.set(identityErrorMessage(error));
    } finally {
      if (isCurrent()) this.busy.set(false);
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
