"use client";

import "client-only";

import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type { AccountPort } from "./account.port";
import type { Profile } from "../domain/profile";
import type { ProfilePhotoMetadata } from "../domain/photo";
import type { SyntheticPayoutAccount } from "../domain/payout-account";
import type {
  OwnIdentityExport,
  RightsRequest,
  RightsRequestKind,
} from "../domain/privacy";

export interface AccountSnapshot {
  readonly profile: Profile | null;
  readonly photo: ProfilePhotoMetadata | null;
  readonly photoUrl: string | null;
  readonly payoutAccount: SyntheticPayoutAccount | null;
  readonly rightsRequests: readonly RightsRequest[];
  readonly loading: boolean;
  readonly error: string | null;
}

const initialSnapshot: AccountSnapshot = Object.freeze({
  profile: null,
  photo: null,
  photoUrl: null,
  payoutAccount: null,
  rightsRequests: Object.freeze([]),
  loading: false,
  error: null,
});

const PHOTO_BLOB_RESOURCE_KEY = "account-profile-photo";

export class AccountFacade {
  #port: AccountPort;
  #abortController: AbortController | null = null;
  #snapshot: AccountSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();

  constructor(port: AccountPort) {
    this.#port = port;
  }

  getSnapshot = (): AccountSnapshot => {
    return this.#snapshot;
  };

  getServerSnapshot = (): AccountSnapshot => {
    return initialSnapshot;
  };

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  cancel(): void {
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
  }

  #setSnapshot(partial: Partial<AccountSnapshot>): void {
    this.#snapshot = Object.freeze({
      ...this.#snapshot,
      ...partial,
    });
    for (const listener of this.#listeners) {
      listener();
    }
  }

  async loadOverview(token: AccessToken): Promise<AccountSnapshot> {
    this.cancel();
    this.#abortController = new AbortController();
    const signal = this.#abortController.signal;

    this.#setSnapshot({ loading: true, error: null });

    try {
      const [profile, photo, payoutAccount, rightsRequests] = await Promise.all(
        [
          this.#port.getProfile(token, signal),
          this.#port.getProfilePhoto(token, signal),
          this.#port.getPayoutAccount(token, signal),
          this.#port.listRightsRequests(token, signal),
        ],
      );

      let photoUrl: string | null = null;
      if (photo) {
        const photoBlob = await this.#port.getProfilePhotoContent(
          token,
          signal,
        );
        if (photoBlob) {
          const registered = privateBlobUrls.create(
            PHOTO_BLOB_RESOURCE_KEY,
            photoBlob,
          );
          photoUrl = registered.url;
        }
      } else {
        privateBlobUrls.revoke(PHOTO_BLOB_RESOURCE_KEY);
      }

      this.#setSnapshot({
        profile,
        photo,
        photoUrl,
        payoutAccount,
        rightsRequests: Object.freeze(rightsRequests),
        loading: false,
        error: null,
      });
      return this.#snapshot;
    } catch (err) {
      if (signal.aborted) throw err;
      const message =
        err instanceof Error ? err.message : "Error al cargar perfil";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#abortController = null;
    }
  }

  async updateProfile(
    token: AccessToken,
    input: { displayName: string; phone?: string },
  ): Promise<Profile> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const updated = await this.#port.updateProfile(token, input);
      this.#setSnapshot({
        profile: updated,
        loading: false,
        error: null,
      });
      return updated;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al actualizar perfil";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async generateSyntheticPhoto(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.generateSyntheticPhoto(token, idempotencyKey);
      const photo = await this.#port.getProfilePhoto(token);
      let photoUrl: string | null = null;
      if (photo) {
        const blob = await this.#port.getProfilePhotoContent(token);
        if (blob) {
          photoUrl = privateBlobUrls.create(PHOTO_BLOB_RESOURCE_KEY, blob).url;
        }
      }
      this.#setSnapshot({
        photo,
        photoUrl,
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error con foto de perfil";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async removeProfilePhoto(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.removeProfilePhoto(token, idempotencyKey);
      privateBlobUrls.revoke(PHOTO_BLOB_RESOURCE_KEY);
      this.#setSnapshot({
        photo: null,
        photoUrl: null,
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al eliminar foto";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async createOrReplacePayoutAccount(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<SyntheticPayoutAccount> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const account = await this.#port.createOrReplacePayoutAccount(
        token,
        idempotencyKey,
      );
      this.#setSnapshot({
        payoutAccount: account,
        loading: false,
        error: null,
      });
      return account;
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Error al configurar cuenta de cobro";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async revokePayoutAccount(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.revokePayoutAccount(token, idempotencyKey);
      this.#setSnapshot({
        payoutAccount: null,
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al revocar cuenta de cobro";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async requestDataRight(
    token: AccessToken,
    kind: RightsRequestKind,
  ): Promise<RightsRequest> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const req = await this.#port.requestDataRight(token, kind);
      const list = await this.#port.listRightsRequests(token);
      this.#setSnapshot({
        rightsRequests: Object.freeze(list),
        loading: false,
        error: null,
      });
      return req;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al solicitar derecho";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async exportIdentityData(
    token: AccessToken,
  ): Promise<OwnIdentityExport | null> {
    return this.#port.exportIdentityData(token);
  }

  async exportIdentityBlob(token: AccessToken): Promise<Blob | null> {
    return this.#port.exportIdentityBlob(token);
  }

  async downloadIdentityArchive(token: AccessToken): Promise<Blob | null> {
    return this.#port.downloadIdentityArchive(token);
  }

  clear(): void {
    this.cancel();
    privateBlobUrls.revoke(PHOTO_BLOB_RESOURCE_KEY);
    this.#setSnapshot(initialSnapshot);
  }
}
