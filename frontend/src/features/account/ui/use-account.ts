"use client";

import { useCallback, useSyncExternalStore } from "react";
import { accountFacade } from "../account-runtime";
import type { AccountSnapshot } from "../application/account-facade";
import type { RightsRequestKind } from "../domain/privacy";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type { Profile } from "../domain/profile";
import type { SyntheticPayoutAccount } from "../domain/payout-account";
import type { RightsRequest } from "../domain/privacy";

export interface AccountView {
  snapshot: AccountSnapshot;
  loadOverview(token: AccessToken): Promise<AccountSnapshot>;
  updateProfile(
    token: AccessToken,
    input: { displayName: string; phone?: string },
  ): Promise<Profile>;
  generateSyntheticPhoto(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<void>;
  removeProfilePhoto(token: AccessToken, idempotencyKey: string): Promise<void>;
  createOrReplacePayoutAccount(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<SyntheticPayoutAccount>;
  revokePayoutAccount(
    token: AccessToken,
    idempotencyKey: string,
  ): Promise<void>;
  requestDataRight(
    token: AccessToken,
    kind: RightsRequestKind,
  ): Promise<RightsRequest>;
  exportIdentityBlob(token: AccessToken): Promise<Blob | null>;
  downloadIdentityArchive(token: AccessToken): Promise<Blob | null>;
  clear(): void;
}

export function useAccount(): AccountView {
  const snapshot = useSyncExternalStore(
    accountFacade.subscribe,
    accountFacade.getSnapshot,
    accountFacade.getServerSnapshot,
  );

  const loadOverview = useCallback(
    (token: AccessToken) => accountFacade.loadOverview(token),
    [],
  );
  const updateProfile = useCallback(
    (token: AccessToken, input: { displayName: string; phone?: string }) =>
      accountFacade.updateProfile(token, input),
    [],
  );
  const generateSyntheticPhoto = useCallback(
    (token: AccessToken, idempotencyKey: string) =>
      accountFacade.generateSyntheticPhoto(token, idempotencyKey),
    [],
  );
  const removeProfilePhoto = useCallback(
    (token: AccessToken, idempotencyKey: string) =>
      accountFacade.removeProfilePhoto(token, idempotencyKey),
    [],
  );
  const createOrReplacePayoutAccount = useCallback(
    (token: AccessToken, idempotencyKey: string) =>
      accountFacade.createOrReplacePayoutAccount(token, idempotencyKey),
    [],
  );
  const revokePayoutAccount = useCallback(
    (token: AccessToken, idempotencyKey: string) =>
      accountFacade.revokePayoutAccount(token, idempotencyKey),
    [],
  );
  const requestDataRight = useCallback(
    (token: AccessToken, kind: RightsRequestKind) =>
      accountFacade.requestDataRight(token, kind),
    [],
  );
  const exportIdentityBlob = useCallback(
    (token: AccessToken) => accountFacade.exportIdentityBlob(token),
    [],
  );
  const downloadIdentityArchive = useCallback(
    (token: AccessToken) => accountFacade.downloadIdentityArchive(token),
    [],
  );
  const clear = useCallback(() => accountFacade.clear(), []);

  return {
    snapshot,
    loadOverview,
    updateProfile,
    generateSyntheticPhoto,
    removeProfilePhoto,
    createOrReplacePayoutAccount,
    revokePayoutAccount,
    requestDataRight,
    exportIdentityBlob,
    downloadIdentityArchive,
    clear,
  };
}
