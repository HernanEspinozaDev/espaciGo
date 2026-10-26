"use client";

import "client-only";

import type { AuthSnapshot, AuthStatus } from "./auth-facade";
import type { Profile } from "@/features/account/domain/profile";

export type { AuthStatus };

export interface SessionIdentitySnapshot {
  readonly status: AuthStatus;
  readonly accountId: string | null;
  readonly label: string | null;
}

export interface AuthSourceLike {
  getSnapshot(): AuthSnapshot;
  subscribe(listener: () => void): () => void;
  checkSession?(): Promise<unknown>;
}

export interface ProfileFetcherLike {
  getProfile(signal?: AbortSignal): Promise<Profile | null>;
}

export interface ProfileSourceLike {
  getSnapshot(): { readonly profile: Profile | null };
  subscribe(listener: () => void): () => void;
}

const initialAnonymousSnapshot: SessionIdentitySnapshot = Object.freeze({
  status: "anonymous",
  accountId: null,
  label: null,
});

const initialCheckingSnapshot: SessionIdentitySnapshot = Object.freeze({
  status: "checking",
  accountId: null,
  label: null,
});

function haveEqualArrays(
  a?: readonly string[] | null,
  b?: readonly string[] | null,
): boolean {
  if (a === b) return true;
  const arrA = a ?? [];
  const arrB = b ?? [];
  if (arrA.length !== arrB.length) return false;
  for (let i = 0; i < arrA.length; i++) {
    if (arrA[i] !== arrB[i]) return false;
  }
  return true;
}

export function isSameAuthIdentity(
  prev: AuthSnapshot | null,
  current: AuthSnapshot,
): boolean {
  if (!prev) return false;
  if (prev.status !== current.status) return false;
  if (prev.accountId !== current.accountId) return false;
  if (prev.restrictedMode !== current.restrictedMode) return false;
  if (prev.expiresAt !== current.expiresAt) return false;
  if (!haveEqualArrays(prev.roles, current.roles)) return false;
  if (!haveEqualArrays(prev.capabilities, current.capabilities)) return false;
  return true;
}

function is404(err: unknown): boolean {
  if (!err || typeof err !== "object") return false;
  const e = err as { status?: unknown; statusCode?: unknown; code?: unknown };
  return (
    e.status === 404 ||
    e.statusCode === 404 ||
    e.code === "not_found" ||
    e.code === "NOT_FOUND"
  );
}

function isAuthError(err: unknown): boolean {
  if (!err || typeof err !== "object") return false;
  const e = err as { status?: unknown; statusCode?: unknown; code?: unknown };
  return (
    e.status === 401 ||
    e.status === 403 ||
    e.statusCode === 401 ||
    e.statusCode === 403 ||
    e.code === "unauthorized" ||
    e.code === "forbidden" ||
    e.code === "UNAUTHORIZED" ||
    e.code === "FORBIDDEN"
  );
}

export class SessionIdentityStore {
  readonly #authSource: AuthSourceLike;
  readonly #profileFetcher: ProfileFetcherLike;
  readonly #profileSource?: ProfileSourceLike;

  #epoch = 0;
  #profileEpoch = 0;
  #abortController: AbortController | null = null;
  #lastAuthSnapshot: AuthSnapshot | null = null;
  #lastSeenProfile: Profile | null | undefined = undefined;
  #staleProfileName: string | null = null;

  #snapshot: SessionIdentitySnapshot = initialAnonymousSnapshot;
  readonly #listeners = new Set<() => void>();
  #authUnsubscribe: (() => void) | null = null;
  #profileUnsubscribe: (() => void) | null = null;

  constructor(
    authSource: AuthSourceLike,
    profileFetcher: ProfileFetcherLike,
    profileSource?: ProfileSourceLike,
  ) {
    this.#authSource = authSource;
    this.#profileFetcher = profileFetcher;
    this.#profileSource = profileSource;

    const currentAuth = authSource.getSnapshot();
    this.#lastAuthSnapshot = currentAuth;
    this.#snapshot = this.#computeInitialSnapshot(currentAuth);

    this.#authUnsubscribe = authSource.subscribe(() => {
      this.#handleAuthChange();
    });

    if (profileSource) {
      this.#lastSeenProfile = profileSource.getSnapshot().profile;
      this.#profileUnsubscribe = profileSource.subscribe(() => {
        this.#handleProfileSourceChange();
      });
    }

    if (currentAuth.status === "authenticated" && currentAuth.accountId) {
      void this.#fetchProfile(
        currentAuth.accountId,
        this.#epoch,
        this.#profileEpoch,
      );
    }
  }

  #computeInitialSnapshot(auth: AuthSnapshot): SessionIdentitySnapshot {
    if (auth.status === "checking") {
      return initialCheckingSnapshot;
    }
    if (auth.status === "authenticated" && auth.accountId) {
      const profileName = this.#profileSource
        ?.getSnapshot()
        .profile?.displayName?.trim();
      return Object.freeze({
        status: "authenticated",
        accountId: auth.accountId,
        label:
          profileName && profileName.length > 0 ? profileName : auth.accountId,
      });
    }
    return initialAnonymousSnapshot;
  }

  getSnapshot = (): SessionIdentitySnapshot => {
    return this.#snapshot;
  };

  getServerSnapshot = (): SessionIdentitySnapshot => {
    return initialCheckingSnapshot;
  };

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  #setSnapshot(next: SessionIdentitySnapshot): void {
    if (
      this.#snapshot.status === next.status &&
      this.#snapshot.accountId === next.accountId &&
      this.#snapshot.label === next.label
    ) {
      return;
    }
    this.#snapshot = Object.freeze(next);
    for (const listener of this.#listeners) {
      listener();
    }
  }

  #isCurrent(epoch: number, signal?: AbortSignal): boolean {
    return this.#epoch === epoch && !signal?.aborted;
  }

  #handleAuthChange(): void {
    const current = this.#authSource.getSnapshot();
    const prev = this.#lastAuthSnapshot;

    if (isSameAuthIdentity(prev, current)) {
      return;
    }

    this.#lastAuthSnapshot = current;
    this.#epoch++;
    this.#profileEpoch++;
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }

    if (current.status === "checking") {
      this.#setSnapshot(initialCheckingSnapshot);
      return;
    }

    if (current.status !== "authenticated" || !current.accountId) {
      this.#lastSeenProfile =
        this.#profileSource?.getSnapshot().profile ?? null;
      this.#staleProfileName =
        this.#profileSource?.getSnapshot().profile?.displayName?.trim() ?? null;
      this.#setSnapshot(initialAnonymousSnapshot);
      return;
    }

    const isIdentitySwitch = prev?.accountId !== current.accountId;
    if (isIdentitySwitch) {
      this.#lastSeenProfile =
        this.#profileSource?.getSnapshot().profile ?? null;
      this.#staleProfileName =
        this.#profileSource?.getSnapshot().profile?.displayName?.trim() ?? null;
    }

    const cachedName = isIdentitySwitch
      ? null
      : this.#profileSource?.getSnapshot().profile?.displayName?.trim();
    const initialLabel =
      cachedName && cachedName.length > 0 ? cachedName : current.accountId;

    this.#setSnapshot({
      status: "authenticated",
      accountId: current.accountId,
      label: initialLabel,
    });

    void this.#fetchProfile(current.accountId, this.#epoch, this.#profileEpoch);
  }

  #handleProfileSourceChange(): void {
    if (!this.#profileSource) return;
    const currentAuth = this.#authSource.getSnapshot();
    if (currentAuth.status !== "authenticated" || !currentAuth.accountId) {
      return;
    }

    const currentProfile = this.#profileSource.getSnapshot().profile;
    if (currentProfile === this.#lastSeenProfile) {
      return;
    }
    this.#lastSeenProfile = currentProfile;

    const rawName = currentProfile?.displayName?.trim();
    if (this.#staleProfileName && rawName === this.#staleProfileName) {
      return;
    }
    if (this.#staleProfileName && rawName !== this.#staleProfileName) {
      this.#staleProfileName = null;
    }

    this.#profileEpoch++;
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }

    const newLabel =
      rawName && rawName.length > 0 ? rawName : currentAuth.accountId;

    this.#setSnapshot({
      status: "authenticated",
      accountId: currentAuth.accountId,
      label: newLabel,
    });
  }

  async #fetchProfile(
    accountId: string,
    epoch: number,
    profileEpoch: number,
  ): Promise<void> {
    const controller = new AbortController();
    this.#abortController = controller;
    const signal = controller.signal;

    try {
      const profile = await this.#profileFetcher.getProfile(signal);

      if (!this.#isCurrent(epoch, signal)) {
        return;
      }
      if (this.#profileEpoch !== profileEpoch) {
        return;
      }
      const currentAuth = this.#authSource.getSnapshot();
      if (
        currentAuth.status !== "authenticated" ||
        currentAuth.accountId !== accountId
      ) {
        return;
      }

      this.#staleProfileName = null;
      const trimmed = profile?.displayName?.trim();
      const label = trimmed && trimmed.length > 0 ? trimmed : accountId;

      this.#setSnapshot({
        status: "authenticated",
        accountId,
        label,
      });
    } catch (err: unknown) {
      if (!this.#isCurrent(epoch, signal)) {
        return;
      }
      if (this.#profileEpoch !== profileEpoch) {
        return;
      }
      if (signal.aborted) {
        return;
      }

      const currentAuth = this.#authSource.getSnapshot();
      if (
        currentAuth.status !== "authenticated" ||
        currentAuth.accountId !== accountId
      ) {
        return;
      }

      if (is404(err)) {
        this.#setSnapshot({
          status: "authenticated",
          accountId,
          label: accountId,
        });
        return;
      }

      if (isAuthError(err)) {
        try {
          void this.#authSource.checkSession?.().catch(() => undefined);
        } catch {
          /* best effort */
        }
        if (!this.#isCurrent(epoch, signal)) {
          return;
        }
        if (this.#profileEpoch !== profileEpoch) {
          return;
        }
        const currentAuthAfterCheck = this.#authSource.getSnapshot();
        if (
          currentAuthAfterCheck.status !== "authenticated" ||
          currentAuthAfterCheck.accountId !== accountId
        ) {
          return;
        }
        this.#setSnapshot({
          status: "authenticated",
          accountId,
          label: accountId,
        });
        return;
      }

      this.#setSnapshot({
        status: "authenticated",
        accountId,
        label: accountId,
      });
    } finally {
      if (this.#abortController === controller) {
        this.#abortController = null;
      }
    }
  }

  dispose(): void {
    if (this.#authUnsubscribe) {
      this.#authUnsubscribe();
      this.#authUnsubscribe = null;
    }
    if (this.#profileUnsubscribe) {
      this.#profileUnsubscribe();
      this.#profileUnsubscribe = null;
    }
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
    this.#listeners.clear();
  }
}
