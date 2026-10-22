import type {
  OwnIdentityExport,
  Profile,
  ProfileUpdate,
  RightsRequest,
  RightsRequestList,
} from '../../../../core/contracts/models';
export type PhotoMetadata = {
  id: string;
  fixture_code: 'synthetic-png-v1';
  mime_type: 'image/png';
  sha256: string;
  size_bytes: number;
  created_at: string;
};
export type ProfileMediaMetadata = {
  id: string;
  variants: Array<{ mime_type: 'image/webp'; size_bytes: number; name: 'avatar' | 'display' | 'thumbnail' }>;
  created_at: string;
};
export type OwnProfilePhoto = PhotoMetadata | ProfileMediaMetadata;
export type SyntheticPayoutAccount = {
  id: string;
  adapter: string;
  reference: string;
  state: string;
  updated_at: string;
};
export class AccountPrivacyApiError extends Error {
  constructor(
    message: string,
    readonly status?: number,
  ) {
    super(message);
    this.name = 'AccountPrivacyApiError';
  }
}
export function statusOf(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}
export abstract class AccountPrivacyPort {
  abstract profile(token: string): Promise<Profile>;
  abstract updateProfile(token: string, value: ProfileUpdate): Promise<Profile>;
  abstract photoMetadata(token: string): Promise<OwnProfilePhoto>;
  abstract photoContent(token: string): Promise<Blob>;
  abstract uploadPhoto(token: string, file: File, key: string, onProgress: (percent: number | null) => void): Promise<void>;
  abstract generatePhoto(token: string, key: string): Promise<void>;
  abstract removePhoto(token: string, key: string): Promise<void>;
  abstract payoutAccount(token: string): Promise<SyntheticPayoutAccount | null>;
  abstract createPayoutAccount(token: string, key: string): Promise<SyntheticPayoutAccount>;
  abstract revokePayoutAccount(token: string, key: string): Promise<void>;
  abstract rightsRequests(token: string): Promise<RightsRequestList>;
  abstract requestRight(token: string, type: RightsRequest['kind']): Promise<RightsRequest>;
  abstract identityExport(token: string): Promise<OwnIdentityExport>;
  abstract archive(token: string): Promise<{ body: Blob | null }>;
}
