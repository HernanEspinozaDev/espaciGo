"use client";

import "client-only";

import {
  ApiError,
  ApiProtocolError,
  normalizeApiError,
} from "@/lib/api/errors";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import type {
  AuthPort,
  CurrentAuthSession,
  LoginRequest,
  SessionCapabilities,
  SessionRoles,
} from "./auth.port";
import { broadcastAuthEvent } from "./auth-lifecycle";

export type AuthStatus = "anonymous" | "checking" | "authenticated";

type Role = SessionRoles[number];
type Capability = SessionCapabilities[number];

export interface AuthSnapshot {
  status: AuthStatus;
  accountId: string | null;
  roles: readonly Role[];
  capabilities: readonly Capability[];
  restrictedMode: boolean;
  expiresAt: string | null;
}

const anonymousSnapshot: AuthSnapshot = Object.freeze({
  status: "anonymous",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});
const checkingSnapshot: AuthSnapshot = Object.freeze({
  status: "checking",
  accountId: null,
  roles: Object.freeze([]),
  capabilities: Object.freeze([]),
  restrictedMode: false,
  expiresAt: null,
});
const rfc3339Timestamp =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

const LEGACY_SESSION_STORAGE_KEY = "espacigo_auth_session";
const LOGOUT_PENDING_KEY = "espacigo_logout_pending";

function cleanLegacySessionStorage(): void {
  if (typeof window === "undefined" || !window.sessionStorage) return;
  try {
    window.sessionStorage.removeItem(LEGACY_SESSION_STORAGE_KEY);
  } catch {}
}

function isLogoutPending(): boolean {
  if (typeof window === "undefined" || !window.localStorage) return false;
  try {
    return window.localStorage.getItem(LOGOUT_PENDING_KEY) === "1";
  } catch {
    return false;
  }
}

function setLogoutPending(): void {
  if (typeof window === "undefined" || !window.localStorage) return;
  try {
    window.localStorage.setItem(LOGOUT_PENDING_KEY, "1");
  } catch {}
}

function clearLogoutPending(): void {
  if (typeof window === "undefined" || !window.localStorage) return;
  try {
    window.localStorage.removeItem(LOGOUT_PENDING_KEY);
  } catch {}
}

function expiryMillis(value: string): number {
  if (!rfc3339Timestamp.test(value)) throw new ApiProtocolError();
  const result = Date.parse(value);
  if (!Number.isFinite(result) || result <= Date.now()) {
    throw new ApiProtocolError();
  }
  return result;
}

function apiAuthError(status: 401 | 403): ApiError {
  return new ApiError({
    status,
    code: status === 401 ? "unauthorized" : "forbidden",
    requestId: null,
    retryAfter: null,
    fields: [],
  });
}

function snapshotFromSession(session: CurrentAuthSession): AuthSnapshot {
  return Object.freeze({
    status: "authenticated",
    accountId: session.accountId,
    roles: Object.freeze([...session.roles]),
    capabilities: Object.freeze([...session.capabilities]),
    restrictedMode: session.restrictedMode,
    expiresAt: session.expiresAt,
  });
}

export class AuthFacade {
  #generation = 0;
  #sessionController = new AbortController();
  #loginController: AbortController | null = null;
  #checkController: AbortController | null = null;
  #expiryTimer: ReturnType<typeof setTimeout> | null = null;
  #snapshot: AuthSnapshot = checkingSnapshot;
  #listeners = new Set<() => void>();
  #checkSessionPromise: Promise<boolean> | null = null;

  constructor(private readonly port: AuthPort) {
    cleanLegacySessionStorage();
    if (isLogoutPending()) {
      this.#snapshot = anonymousSnapshot;
    }
  }

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  };

  getSnapshot = (): AuthSnapshot => {
    const expiresAt = this.#snapshot.expiresAt;
    if (
      this.#snapshot.status === "authenticated" &&
      expiresAt &&
      Number.isFinite(Date.parse(expiresAt)) &&
      Date.now() >= Date.parse(expiresAt)
    ) {
      return anonymousSnapshot;
    }
    return this.#snapshot;
  };

  getServerSnapshot = (): AuthSnapshot => checkingSnapshot;

  async login(input: LoginRequest): Promise<boolean> {
    const generation = this.#advanceGeneration();
    this.#publish(checkingSnapshot);
    const controller = new AbortController();
    this.#loginController = controller;

    try {
      const session = await this.port.login(input, controller.signal);
      if (generation !== this.#generation || controller.signal.aborted) {
        return false;
      }
      const exp = expiryMillis(session.expiresAt);
      clearLogoutPending();
      this.#publish(snapshotFromSession(session));
      this.#scheduleExpiry(generation, exp);
      broadcastAuthEvent({ type: "session_changed" });
      return true;
    } catch (error) {
      if (generation === this.#generation) this.#clearIfCurrent(generation);
      throw error;
    } finally {
      if (this.#loginController === controller) {
        this.#loginController = null;
      }
    }
  }

  checkSession(): Promise<boolean> {
    this.#expireIfNeeded();
    if (this.#checkSessionPromise) {
      return this.#checkSessionPromise;
    }

    if (isLogoutPending()) {
      return (async () => {
        const controller = new AbortController();
        try {
          const result = await this.port.logout(controller.signal);
          if (result.cookieCleared) {
            clearLogoutPending();
          }
        } catch {
          // network or server error during pending logout cleanup
        }
        if (this.#snapshot.status !== "anonymous") {
          this.#publish(anonymousSnapshot);
        }
        return false;
      })();
    }

    const generation = this.#generation;
    const controller = new AbortController();
    const signal = AbortSignal.any([
      this.#sessionController.signal,
      controller.signal,
    ]);
    this.#checkController?.abort();
    this.#checkController = controller;

    const promise = (async (): Promise<boolean> => {
      try {
        const session = await this.port.currentSession(signal);
        if (generation !== this.#generation || signal.aborted) {
          return false;
        }
        const prevAccountId = this.#snapshot.accountId;
        if (prevAccountId && prevAccountId !== session.accountId) {
          privateBlobUrls.clear();
        }
        const exp = expiryMillis(session.expiresAt);
        this.#publish(snapshotFromSession(session));
        this.#scheduleExpiry(generation, exp);
        return true;
      } catch (error) {
        if (generation !== this.#generation || signal.aborted) return false;
        if (
          error instanceof ApiError &&
          (error.status === 401 || error.status === 403)
        ) {
          this.#clearIfCurrent(generation);
          return false;
        }
        throw error;
      } finally {
        this.#checkSessionPromise = null;
        if (this.#checkController === controller) {
          this.#checkController = null;
        }
      }
    })();

    this.#checkSessionPromise = promise;
    return promise;
  }

  async logout(): Promise<void> {
    this.#advanceGeneration();
    setLogoutPending();
    this.#publish(anonymousSnapshot);
    broadcastAuthEvent({ type: "logout" });

    const controller = new AbortController();
    try {
      const result = await this.port.logout(controller.signal);
      if (result.cookieCleared) {
        clearLogoutPending();
      }
      if (!result.revocationConfirmed) {
        throw new ApiError({
          status: 502,
          code: "remote_logout_unconfirmed",
          requestId: null,
          retryAfter: null,
          fields: [],
        });
      }
    } catch (error) {
      if (
        error instanceof ApiError &&
        (error.status === 401 || error.status === 403)
      ) {
        clearLogoutPending();
        return;
      }
      throw error;
    }
  }

  handlePeerLogout(): void {
    this.#advanceGeneration();
    this.#publish(anonymousSnapshot);
  }

  hasCapability(capability: Capability): boolean {
    const snapshot = this.getSnapshot();
    return (
      snapshot.status === "authenticated" &&
      snapshot.capabilities.includes(capability)
    );
  }

  canRenderCapability(capability: Capability): boolean {
    return !this.getSnapshot().restrictedMode && this.hasCapability(capability);
  }

  async runAuthenticated<T>(
    request: (signal: AbortSignal) => Promise<{ response: Response; value: T }>,
  ): Promise<T> {
    this.#expireIfNeeded();
    if (this.#snapshot.status !== "authenticated") throw apiAuthError(401);
    const generation = this.#generation;
    const signal = this.#sessionController.signal;

    try {
      const result = await request(signal);
      if (generation !== this.#generation || signal.aborted) {
        throw new DOMException(
          "The session changed during the request.",
          "AbortError",
        );
      }
      if (result.response.status === 401) {
        this.#clearIfCurrent(generation);
        throw normalizeApiError(result.response, null);
      }
      if (result.response.status === 403) {
        privateBlobUrls.clear();
        void this.checkSession().catch(() => false);
        throw normalizeApiError(result.response, null);
      }
      return result.value;
    } catch (error) {
      if (generation === this.#generation && error instanceof ApiError) {
        if (error.status === 401) {
          this.#clearIfCurrent(generation);
        } else if (error.status === 403) {
          privateBlobUrls.clear();
          void this.checkSession().catch(() => false);
        }
      }
      throw error;
    }
  }

  #advanceGeneration(): number {
    this.#generation += 1;
    privateBlobUrls.clear();
    this.#sessionController.abort();
    this.#sessionController = new AbortController();
    this.#loginController?.abort();
    this.#loginController = null;
    this.#checkController?.abort();
    this.#checkController = null;
    if (this.#expiryTimer !== null) clearTimeout(this.#expiryTimer);
    this.#expiryTimer = null;
    return this.#generation;
  }

  #clearIfCurrent(generation: number): void {
    if (generation !== this.#generation) return;
    this.#advanceGeneration();
    this.#publish(anonymousSnapshot);
  }

  #expireIfNeeded(): void {
    const expiresAt = this.#snapshot.expiresAt;
    if (
      this.#snapshot.status === "authenticated" &&
      expiresAt &&
      Number.isFinite(Date.parse(expiresAt)) &&
      Date.now() >= Date.parse(expiresAt)
    ) {
      this.#clearIfCurrent(this.#generation);
    }
  }

  #scheduleExpiry(generation: number, expiresAt: number): void {
    const schedule = () => {
      if (generation !== this.#generation) return;
      const remaining = expiresAt - Date.now();
      if (remaining <= 0) {
        this.#clearIfCurrent(generation);
        return;
      }
      this.#expiryTimer = setTimeout(
        schedule,
        Math.min(remaining, 2_147_000_000),
      );
      if (typeof this.#expiryTimer?.unref === "function") {
        this.#expiryTimer.unref();
      }
    };
    schedule();
  }

  #publish(snapshot: AuthSnapshot): void {
    this.#snapshot = snapshot;
    for (const listener of this.#listeners) listener();
  }
}
