import { Injectable, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import type { SessionResponse } from '../../contracts/models';
import { IdentityPort } from '../domain/ports/identity-port';
import { IdentityApiError } from '../identity-errors';
import { PrivateSessionState } from '../private-session-state';

export type IdentityStatus = 'anonymous' | 'checking' | 'authenticated';

@Injectable({ providedIn: 'root' })
export class IdentitySession {
  private readonly api = inject(IdentityPort);
  private readonly router = inject(Router);
  private readonly privateState = inject(PrivateSessionState);
  readonly accessToken = signal<string | null>(null);
  readonly session = signal<SessionResponse | null>(null);
  readonly status = signal<IdentityStatus>('anonymous');
  readonly notice = signal('');
  private generation = 0;
  get sessionRevision(): number {
    return this.generation;
  }

  async signIn(email: string, password: string): Promise<SessionResponse> {
    const generation = ++this.generation;
    this.notice.set('');
    this.resetSession();
    this.status.set('checking');

    try {
      const login = await this.api.login(email, password);
      if (generation !== this.generation) throw new StaleIdentityRequest();

      this.accessToken.set(login.access_token);
      const session = await this.api.session(login.access_token);
      if (generation !== this.generation) throw new StaleIdentityRequest();
      if (session.account_id !== login.account_id) {
        throw new Error('La sesión confirmada no coincide con la cuenta autenticada.');
      }

      this.session.set(session);
      this.status.set('authenticated');
      this.privateState.generation.update((value) => value + 1);
      return session;
    } catch (error) {
      if (generation !== this.generation) throw new StaleIdentityRequest();
      this.resetSession();
      if (generation !== this.generation) throw new StaleIdentityRequest();
      throw error;
    }
  }

  async ensureSession(): Promise<SessionResponse | null> {
    if (!this.accessToken()) return null;
    if (this.session()) return this.session();

    const generation = this.generation;
    const token = this.accessToken();
    if (!token) return null;
    this.status.set('checking');
    try {
      const session = await this.api.session(token);
      if (generation !== this.generation || token !== this.accessToken()) return null;
      this.session.set(session);
      this.status.set('authenticated');
      return session;
    } catch (error) {
      if (generation === this.generation && isUnauthorized(error)) this.resetSession();
      else if (generation === this.generation) this.status.set('anonymous');
      throw error;
    }
  }

  /** Refresh Go-confirmed roles/capabilities without replacing a still-valid bearer. */
  async refreshSessionIfCurrent(token: string): Promise<SessionResponse | null> {
    if (!token || token !== this.accessToken()) return null;
    const generation = this.generation;
    const accountId = this.session()?.account_id;
    try {
      const confirmed = await this.api.session(token);
      if (generation !== this.generation || token !== this.accessToken()) return null;
      if (accountId && confirmed.account_id !== accountId) return null;
      this.session.set(confirmed);
      this.status.set('authenticated');
      return confirmed;
    } catch (error) {
      if (generation === this.generation && token === this.accessToken() && isUnauthorized(error)) {
        this.expireIfCurrent(token);
      }
      throw error;
    }
  }

  async consumeRecovery(
    tokenId: string,
    recoveryToken: string,
    newPassword: string,
    confirmPassword: string,
  ): Promise<boolean> {
    const sourceGeneration = this.generation;
    await this.api.consumeRecovery(tokenId, recoveryToken, newPassword, confirmPassword);

    const generation = this.generation;
    const token = this.accessToken();
    const sessionBeforeRevalidation = this.session();
    if (!token) {
      if (sourceGeneration === generation) {
        this.notice.set(
          'Contraseña actualizada. Se cerraron las sesiones anteriores. Ya puedes iniciar sesión.',
        );
      }
      return sourceGeneration === generation;
    }

    // A recovery completion can outlive logout/login. Revalidate whichever bearer
    // is current now; the token submitted for recovery may belong to another account.
    try {
      const confirmed = await this.api.session(token);
      if (generation !== this.generation || token !== this.accessToken()) return false;
      if (
        sessionBeforeRevalidation &&
        confirmed.account_id !== sessionBeforeRevalidation.account_id
      ) {
        ++this.generation;
        this.resetSession();
        this.notice.set('EspaciGo no confirmó la sesión vigente. Inicia sesión nuevamente.');
        return false;
      }
      this.session.set(confirmed);
      this.status.set('authenticated');
      if (sourceGeneration === generation) {
        this.notice.set('EspaciGo cambió la contraseña y confirmó la sesión vigente.');
      }
    } catch (error) {
      if (generation !== this.generation || token !== this.accessToken()) return false;
      if (isUnauthorized(error)) {
        ++this.generation;
        this.resetSession();
        this.notice.set(
          sourceGeneration === generation
            ? 'EspaciGo cambió la contraseña y revocó la sesión vigente. Inicia sesión nuevamente.'
            : 'EspaciGo confirmó que la sesión vigente ya no está activa. Inicia sesión nuevamente.',
        );
        return (
          sourceGeneration === generation &&
          this.generation === generation + 1 &&
          this.accessToken() === null
        );
      } else if (sourceGeneration === generation) {
        this.notice.set(
          'EspaciGo cambió la contraseña; no fue posible revalidar la sesión vigente.',
        );
      }
    }
    return sourceGeneration === this.generation;
  }

  async signOut(): Promise<boolean> {
    const token = this.accessToken();
    const generation = ++this.generation;
    this.resetSession();
    this.notice.set('');
    if (!token) return true;
    try {
      await this.api.logout(token);
      if (generation !== this.generation) return false;
      this.notice.set('Sesión cerrada y revocada por Go.');
    } catch (error) {
      if (generation !== this.generation) return false;
      if (isUnauthorized(error)) {
        this.notice.set('La sesión ya no estaba vigente en Go. Se limpió el estado local.');
      } else {
        this.notice.set('Se limpió el estado local, pero Go no confirmó la revocación.');
      }
    }
    return true;
  }

  /** Expire only the bearer that received the unauthorized response. */
  expireIfCurrent(token: string): boolean {
    if (!token || this.accessToken() !== token) return false;
    ++this.generation;
    this.resetSession();
    this.notice.set('EspaciGo cerró la sesión. Inicia sesión nuevamente.');
    return true;
  }

  async changePassword(
    currentPassword: string,
    newPassword: string,
    confirmPassword: string,
  ): Promise<void> {
    const token = this.accessToken();
    if (!token) throw new Error('Inicia sesión para cambiar tu contraseña.');
    const generation = this.generation;
    try {
      await this.api.changePassword(token, currentPassword, newPassword, confirmPassword);
      if (generation !== this.generation) throw new StaleIdentityRequest();
      ++this.generation;
      this.resetSession();
      this.notice.set(
        'EspaciGo cambió tu contraseña y revocó las sesiones. Inicia sesión nuevamente.',
      );
    } catch (error) {
      if (generation === this.generation && isUnauthorized(error)) this.resetSession();
      throw error;
    }
  }

  private resetSession(): void {
    this.accessToken.set(null);
    this.session.set(null);
    this.status.set('anonymous');
    this.privateState.clear();
  }
}

export class StaleIdentityRequest extends Error {
  constructor() {
    super('La respuesta pertenece a una sesión anterior y fue descartada.');
  }
}

export function isUnauthorized(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'status' in error && error.status === 401;
}
