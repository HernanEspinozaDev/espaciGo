import type { Account, Overview, Page, AccountBlockUpdate, ProfileUpdate, ProfileUpdateResult, RoleUpdate, SessionRevocation, PublicationPage, PublicationStateUpdate, PublicationContentUpdate } from '../global-admin-models';

export abstract class GlobalAdminPort {
  abstract overview(token: string): Promise<Overview>;
  abstract list(
    token: string,
    filters: Record<string, string | number | undefined>,
  ): Promise<Page>;
  abstract account(token: string, id: string): Promise<Account>;
  abstract updateProfile(token: string, id: string, input: ProfileUpdate): Promise<ProfileUpdateResult>;
  abstract updateRole(token: string, id: string, role: string, input: RoleUpdate): Promise<void>;
  abstract revokeSessions(token: string, id: string, input: SessionRevocation): Promise<number>;
  abstract listPublications(token:string,filters:Record<string,string|number|undefined>):Promise<PublicationPage>;
  abstract setPublicationState(token:string,id:string,input:PublicationStateUpdate):Promise<void>;
  abstract updatePublication(token:string,id:string,input:PublicationContentUpdate):Promise<void>;
  abstract setBlocked(token: string,id:string,input:AccountBlockUpdate):Promise<void>;
}
