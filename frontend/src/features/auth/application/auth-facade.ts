"use client";

import "client-only";

import {
  ApiError,
  ApiProtocolError,
  normalizeApiError,
} from "@/lib/api/errors";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import type {
  AccessToken,
  AuthPort,
  CurrentAuthSession,
  LoginRequest,
  SessionCapabilities,
  SessionRoles,
} from "./auth.port";

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

function snapshotFromSession(
  session: CurrentAuthSession,
  expiresAt: string,
): AuthSnapshot {
  return Object.freeze({
    status: "authenticated",
    accountId: session.accountId,
    roles: Object.freeze([...session.roles]),
    capabilities: Object.freeze([...session.capabilities]),
    restrictedMode: session.restrictedMode,
    expiresAt,
  });
}

export class AuthFacade {
  #accessToken: AccessToken | null = null;
  #generation = 0;
  #sessionController = new AbortController();
  #operationController: AbortController | null = null;
  #expiryTimer: ReturnType<typeof setTimeout> | null = null;
  #snapshot: AuthSnapshot = anonymousSnapshot;
  #listeners = new Set<() => void>();

  constructor(private readonly port: AuthPort) {}

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  };

  getSnapshot = (): AuthSnapshot => {
    const expiresAt = this.#snapshot.expiresAt;
    if (
      this.#accessToken &&
      expiresAt &&
      Number.isFinite(Date.parse(expiresAt)) &&
      Date.now() >= Date.parse(expiresAt)
    ) {
      return anonymousSnapshot;
    }
    return this.#snapshot;
  };

  getServerSnapshot = (): AuthSnapshot => anonymousSnapshot;

  async login(input: LoginRequest): Promise<boolean> {
    const generation = this.#advanceGeneration();
    this.#accessToken = null;
    this.#publish(checkingSnapshot);
    const controller = new AbortController();
    this.#operationController = controller;

    try {
      const result = await this.port.login(input, controller.signal);
      if (generation !== this.#generation || controller.signal.aborted) {
        return false;
      }
      const session = await this.port.currentSession(
        result.accessToken,
        controller.signal,
      );
      if (generation !== this.#generation || controller.signal.aborted) {
        return false;
      }
      const expiresAt = expiryMillis(result.session.expiresAt);
      this.#accessToken = result.accessToken;
      this.#publish(snapshotFromSession(session, result.session.expiresAt));
      this.#scheduleExpiry(generation, expiresAt);
      return true;
    } catch (error) {
      if (generation === this.#generation) this.#clearIfCurrent(generation);
      throw error;
    } finally {
      if (this.#operationController === controller) {
        this.#operationController = null;
      }
    }
  }

  async checkSession(): Promise<boolean> {
    this.#expireIfNeeded();
    const accessToken = this.#accessToken;
    const expiresAt = this.#snapshot.expiresAt;
    const generation = this.#generation;
    if (!accessToken || !expiresAt) {
      if (this.#snapshot.status !== "anonymous")
        this.#publish(anonymousSnapshot);
      return false;
    }

    const controller = new AbortController();
    const signal = AbortSignal.any([
      this.#sessionController.signal,
      controller.signal,
    ]);
    this.#operationController?.abort();
    this.#operationController = controller;

    try {
      const session = await this.port.currentSession(accessToken, signal);
      if (
        generation !== this.#generation ||
        signal.aborted ||
        accessToken !== this.#accessToken
      ) {
        return false;
      }
      this.#publish(snapshotFromSession(session, expiresAt));
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
      if (this.#operationController === controller) {
        this.#operationController = null;
      }
    }
  }

  async logout(): Promise<void> {
    const accessToken = this.#accessToken;
    if (!accessToken) {
      this.#clearIfCurrent(this.#generation);
      return;
    }

    this.#advanceGeneration();
    this.#accessToken = null;
    this.#publish(anonymousSnapshot);
    const controller = new AbortController();
    try {
      await this.port.logout(accessToken, controller.signal);
    } catch (error) {
      if (
        error instanceof ApiError &&
        (error.status === 401 || error.status === 403)
      ) {
        return;
      }
      throw error;
    }
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
    request: (
      accessToken: AccessToken,
      signal: AbortSignal,
    ) => Promise<{ response: Response; value: T }>,
  ): Promise<T> {
    this.#expireIfNeeded();
    const accessToken = this.#accessToken;
    const generation = this.#generation;
    if (!accessToken) throw apiAuthError(401);
    const signal = this.#sessionController.signal;

    try {
      const result = await request(accessToken, signal);
      if (
        generation !== this.#generation ||
        signal.aborted ||
        accessToken !== this.#accessToken
      ) {
        throw new DOMException(
          "The session changed during the request.",
          "AbortError",
        );
      }
      if (result.response.status === 401 || result.response.status === 403) {
        this.#clearIfCurrent(generation);
        throw normalizeApiError(result.response, null);
      }
      return result.value;
    } catch (error) {
      if (
        generation === this.#generation &&
        error instanceof ApiError &&
        (error.status === 401 || error.status === 403)
      ) {
        this.#clearIfCurrent(generation);
      }
      throw error;
    }
  }

  #advanceGeneration(): number {
    this.#generation += 1;
    privateBlobUrls.clear();
    this.#sessionController.abort();
    this.#sessionController = new AbortController();
    this.#operationController?.abort();
    this.#operationController = null;
    if (this.#expiryTimer !== null) clearTimeout(this.#expiryTimer);
    this.#expiryTimer = null;
    return this.#generation;
  }

  #clearIfCurrent(generation: number): void {
    if (generation !== this.#generation) return;
    this.#advanceGeneration();
    this.#accessToken = null;
    this.#publish(anonymousSnapshot);
  }

  #expireIfNeeded(): void {
    const expiresAt = this.#snapshot.expiresAt;
    if (
      this.#accessToken &&
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
    };
    schedule();
  }

  #publish(snapshot: AuthSnapshot): void {
    this.#snapshot = snapshot;
    for (const listener of this.#listeners) listener();
  }
}
