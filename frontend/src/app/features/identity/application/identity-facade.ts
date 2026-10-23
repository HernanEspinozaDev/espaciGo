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
import {
  identityErrorMessage,
  identityFieldMessages,
} from '../../../core/identity/identity-errors';
import {
  IdentitySession,
  StaleIdentityRequest,
} from '../../../core/identity/application/identity-session';
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
  readonly fieldErrors = signal<Record<string, string>>({});
  readonly successMessage = signal('');
  readonly registrationNeedsRetry = signal(false);
  readonly registrationSubmitted = signal(false);
  readonly recoveryRequested = signal(false);
  readonly reissueRequested = signal(false);
  readonly recoveryComplete = signal(false);
  readonly linkState = signal<'missing' | 'ready' | 'invalid'>('missing');
  readonly verificationSuggested = signal(false);
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
  private actionSerial = 0;
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
    required(path.acceptTerms, {
      message: 'Debes aceptar los Términos y condiciones para continuar.',
    });
    required(path.acceptPrivacy, { message: 'Debes leer y aceptar la Política de privacidad.' });
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
    if (this.mode === 'verify' || this.mode === 'recovery') {
      const fragment = this.route.snapshot.fragment;
      if (fragment) {
        const values = new URLSearchParams(fragment);
        const tokenIds = values.getAll('token_id');
        const tokens = values.getAll('token');
        const valid =
          fragment.length <= 4096 &&
          values.size === 2 &&
          tokenIds.length === 1 &&
          tokens.length === 1 &&
          !!tokenIds[0] &&
          !!tokens[0];
        if (valid) {
          if (this.mode === 'verify')
            this.tokenModel.set({ tokenId: tokenIds[0], token: tokens[0] });
          else
            this.recoveryModel.update((model) => ({
              ...model,
              tokenId: tokenIds[0],
              token: tokens[0],
            }));
          this.linkState.set('ready');
        } else this.linkState.set('invalid');
        void this.router.navigate([], {
          relativeTo: this.route,
          queryParams: this.identityQueryParams(),
          fragment: undefined,
          replaceUrl: true,
        });
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
    if (this.busy()) return;
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
              typeof safeReturn === 'string' ? safeReturn : (safeReturn?.detailUrl ?? '/mi-cuenta'),
            );
            if (!navigated && !this.destroyed) {
              this.errorMessage.set(
                'Iniciaste sesión, pero no pudimos abrir tu cuenta. Inténtalo de nuevo.',
              );
            }
          } catch {
            if (!this.destroyed) {
              this.errorMessage.set(
                'Iniciaste sesión, pero no pudimos abrir tu cuenta. Recarga e inténtalo de nuevo.',
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
    this.fieldErrors.set({});
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
        this.termsState() !== 'ready' ||
        versionIds.length !== 2 ||
        !versionIds.some((id) =>
          this.terms().some((term) => term.id === id && term.type === 'terminos'),
        )
      ) {
        this.errorMessage.set(
          'Los documentos todavía no están disponibles. Vuelve a cargarlos antes de crear tu cuenta.',
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
          this.registrationSubmitted.set(true);
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
        () => {
          this.reissueRequested.set(true);
          this.successMessage.set(
            'Solicitud recibida. Si tu cuenta necesita verificación, recibirás un nuevo enlace. Revisa también spam; no podemos confirmar la entrega.',
          );
        },
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
          const query = new URLSearchParams({ ...this.identityQueryParams(), verificado: '1' });
          const navigated = await this.router.navigateByUrl(`/acceso?${query.toString()}`);
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
        (accepted) => {
          this.recoveryRequested.set(true);
          this.successMessage.set(accepted.message);
        },
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
        (confirmed) => {
          if (!confirmed) return;
          this.recoveryComplete.set(true);
          this.recoveryModel.set({
            tokenId: '',
            token: '',
            newPassword: '',
            confirmPassword: '',
          });
        },
        (confirmed) => confirmed === true,
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

  async onLoginReissue(event: SubmitEvent): Promise<void> {
    this.emailModel.set({ email: this.loginModel().email });
    await this.onReissue(event);
  }

  identityQueryParams(): Record<string, string> {
    const safeReturn = parseIdentityReturn(
      this.route.snapshot.queryParamMap?.getAll('returnUrl') ?? [],
    );
    const returnUrl = typeof safeReturn === 'string' ? safeReturn : safeReturn?.identityReturnUrl;
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

  async loadTerms(): Promise<void> {
    this.termsState.set('loading');
    this.registerModel.update((model) => ({ ...model, acceptTerms: false, acceptPrivacy: false }));
    try {
      const terms = await this.api.terms();
      this.terms.set(terms.items);
      this.termsState.set(
        ['terminos', 'privacidad'].every(
          (type) =>
            terms.items.filter(
              (item) =>
                item.type === type &&
                item.document?.sections?.length &&
                item.document.id === item.id,
            ).length === 1,
        )
          ? 'ready'
          : 'error',
      );
    } catch {
      this.termsState.set('error');
    }
  }

  private begin(): number {
    this.fieldErrors.set({});
    this.errorMessage.set('');
    this.successMessage.set('');
    this.busy.set(true);
    ++this.actionSerial;
    return ++this.requestGeneration;
  }

  private async performRequest<T>(
    request: () => Promise<T>,
    onSuccess: (value: T) => void | Promise<void>,
    acceptsSessionGenerationChange?: (value: T) => boolean,
  ): Promise<void> {
    if (this.busy()) return;
    this.begin();
    let generation = this.requestGeneration;
    let stateGeneration = this.privateSessionState.generation();
    let identityRevision = this.identity.sessionRevision;
    const actionSerial = this.actionSerial;
    let result!: T;
    let hasResult = false;
    const isCurrent = () =>
      this.isCurrentRequest(generation, stateGeneration) ||
      (!this.destroyed &&
        actionSerial === this.actionSerial &&
        hasResult &&
        acceptsSessionGenerationChange?.(result) === true);
    try {
      // IdentitySession may clear private state synchronously when a new login starts.
      const pending = request();
      generation = this.requestGeneration;
      stateGeneration = this.privateSessionState.generation();
      identityRevision = this.identity.sessionRevision;
      this.busy.set(true);
      result = await pending;
      hasResult = true;
      if (!isCurrent()) return;
      await onSuccess(result);
    } catch (error) {
      const ownsLoginFailure =
        this.mode === 'login' &&
        !this.destroyed &&
        actionSerial === this.actionSerial &&
        identityRevision === this.identity.sessionRevision &&
        !(error instanceof StaleIdentityRequest);
      if (isCurrent() || ownsLoginFailure) {
        this.fieldErrors.set(identityFieldMessages(error));
        this.errorMessage.set(
          this.mode === 'login' &&
            typeof error === 'object' &&
            error !== null &&
            'status' in error &&
            error.status === 401
            ? 'Correo o contraseña incorrectos'
            : this.mode === 'verify' &&
                typeof error === 'object' &&
                error !== null &&
                'status' in error &&
                error.status === 422
              ? 'Este enlace no es válido, ha vencido o ya se utilizó. Solicita uno nuevo para continuar.'
              : identityErrorMessage(error),
        );
        if (
          this.mode === 'login' &&
          typeof error === 'object' &&
          error !== null &&
          'status' in error &&
          error.status === 403
        )
          this.verificationSuggested.set(true);
      }
    } finally {
      if (
        isCurrent() ||
        (this.mode === 'login' &&
          !this.destroyed &&
          actionSerial === this.actionSerial &&
          identityRevision === this.identity.sessionRevision)
      )
        this.busy.set(false);
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
      /\p{Nd}/u.test(password) &&
      /[\p{P}\p{S}]/u.test(password);
    return valid
      ? undefined
      : {
          kind: 'password-policy',
          message:
            'Usa mayúscula, número y símbolo. Si la contraseña es muy larga, elige una más corta.',
        };
  });
}
