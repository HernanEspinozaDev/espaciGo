"use client";

import { useCallback, useSyncExternalStore } from "react";
import { accountFacade } from "../account-runtime";
import type { AccountSnapshot } from "../application/account-facade";
import type { RightsRequestKind } from "../domain/privacy";
import type { Profile } from "../domain/profile";
import type { SyntheticPayoutAccount } from "../domain/payout-account";
import type { RightsRequest } from "../domain/privacy";

export interface AccountView {
  snapshot: AccountSnapshot;
  loadOverview(): Promise<AccountSnapshot>;
  updateProfile(input: {
    displayName: string;
    phone?: string;
  }): Promise<Profile>;
  generateSyntheticPhoto(idempotencyKey: string): Promise<void>;
  removeProfilePhoto(idempotencyKey: string): Promise<void>;
  createOrReplacePayoutAccount(
    idempotencyKey: string,
  ): Promise<SyntheticPayoutAccount>;
  revokePayoutAccount(idempotencyKey: string): Promise<void>;
  requestDataRight(kind: RightsRequestKind): Promise<RightsRequest>;
  exportIdentityBlob(): Promise<Blob | null>;
  downloadIdentityArchive(): Promise<Blob | null>;
  clear(): void;
}

export function useAccount(): AccountView {
  const snapshot = useSyncExternalStore(
    accountFacade.subscribe,
    accountFacade.getSnapshot,
    accountFacade.getServerSnapshot,
  );

  const loadOverview = useCallback(() => accountFacade.loadOverview(), []);
  const updateProfile = useCallback(
    (input: { displayName: string; phone?: string }) =>
      accountFacade.updateProfile(input),
    [],
  );
  const generateSyntheticPhoto = useCallback(
    (idempotencyKey: string) =>
      accountFacade.generateSyntheticPhoto(idempotencyKey),
    [],
  );
  const removeProfilePhoto = useCallback(
    (idempotencyKey: string) =>
      accountFacade.removeProfilePhoto(idempotencyKey),
    [],
  );
  const createOrReplacePayoutAccount = useCallback(
    (idempotencyKey: string) =>
      accountFacade.createOrReplacePayoutAccount(idempotencyKey),
    [],
  );
  const revokePayoutAccount = useCallback(
    (idempotencyKey: string) =>
      accountFacade.revokePayoutAccount(idempotencyKey),
    [],
  );
  const requestDataRight = useCallback(
    (kind: RightsRequestKind) => accountFacade.requestDataRight(kind),
    [],
  );
  const exportIdentityBlob = useCallback(
    () => accountFacade.exportIdentityBlob(),
    [],
  );
  const downloadIdentityArchive = useCallback(
    () => accountFacade.downloadIdentityArchive(),
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
