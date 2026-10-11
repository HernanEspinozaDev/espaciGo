import { HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import type { SessionResponse } from '../../../generated/api/types.gen';
import { IdentityApi } from './identity-api';
import { PrivateSessionState } from './private-session-state';
import { IdentityApiError } from './identity-errors';

export type IdentityStatus = 'anonymous' | 'checking' | 'authenticated';

@Injectable({ providedIn: 'root' })
export class IdentitySession {
  private readonly api = inject(IdentityApi);
  private readonly router = inject(Router);
  private readonly privateState = inject(PrivateSessionState);
  readonly accessToken = signal<string | null>(null);
  readonly session = signal<SessionResponse | null>(null);
  readonly status = signal<IdentityStatus>('anonymous');
  readonly notice = signal('');
  private generation = 0;

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
      if (generation === this.generation) this.resetSession();
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

  async signOut(): Promise<void> {
    const token = this.accessToken();
    ++this.generation;
    this.resetSession();
    if (!token) return;
    try {
      await this.api.logout(token);
      this.notice.set('Sesión cerrada y revocada por Go.');
    } catch (error) {
      if (isUnauthorized(error)) {
        this.notice.set('La sesión ya no estaba vigente en Go. Se limpió el estado local.');
      } else {
        this.notice.set('Se limpió el estado local, pero Go no confirmó la revocación.');
        throw error;
      }
    }
  }

  async changePassword(currentPassword: string, newPassword: string, confirmPassword: string): Promise<void> {
    const token = this.accessToken();
    if (!token) throw new Error('Inicia sesión para cambiar tu contraseña.');
    const generation = this.generation;
    try {
      await this.api.changePassword(token, currentPassword, newPassword, confirmPassword);
      if (generation !== this.generation) throw new StaleIdentityRequest();
      ++this.generation;
      this.resetSession();
      this.notice.set('Go cambió tu contraseña y revocó las sesiones. Inicia sesión nuevamente.');
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
  return (error instanceof HttpErrorResponse || error instanceof IdentityApiError) && error.status === 401;
}
