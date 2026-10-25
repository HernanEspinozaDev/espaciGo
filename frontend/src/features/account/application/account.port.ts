import type { AccessToken } from "@/features/auth/application/auth.port";
import type { Profile } from "../domain/profile";
import type { ProfilePhotoMetadata } from "../domain/photo";
import type { SyntheticPayoutAccount } from "../domain/payout-account";
import type {
  OwnIdentityExport,
  RightsRequest,
  RightsRequestKind,
} from "../domain/privacy";

export interface AccountPort {
  getProfile(token: AccessToken, signal?: AbortSignal): Promise<Profile | null>;
  updateProfile(
    token: AccessToken,
    payload: { displayName: string; phone?: string },
    signal?: AbortSignal,
  ): Promise<Profile>;

  getProfilePhoto(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<ProfilePhotoMetadata | null>;
  generateSyntheticPhoto(
    token: AccessToken,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<void>;
  uploadProfilePhoto(
    token: AccessToken,
    idempotencyKey: string,
    imageFile: Blob | File,
    signal?: AbortSignal,
  ): Promise<void>;
  removeProfilePhoto(
    token: AccessToken,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<void>;
  getProfilePhotoContent(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<Blob | null>;

  getPayoutAccount(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<SyntheticPayoutAccount | null>;
  createOrReplacePayoutAccount(
    token: AccessToken,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<SyntheticPayoutAccount>;
  revokePayoutAccount(
    token: AccessToken,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<void>;

  listRightsRequests(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<RightsRequest[]>;
  requestDataRight(
    token: AccessToken,
    kind: RightsRequestKind,
    signal?: AbortSignal,
  ): Promise<RightsRequest>;

  exportIdentityData(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<OwnIdentityExport | null>;
  exportIdentityBlob(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<Blob | null>;
  downloadIdentityArchive(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<Blob | null>;
}
