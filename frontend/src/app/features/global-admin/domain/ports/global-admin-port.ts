import type { Account, Overview, Page, AccountBlockUpdate, ProfileUpdate, ProfileUpdateResult, RoleUpdate, SessionRevocation, PublicationPage, PublicationStateUpdate, PublicationContentUpdate, PublicationRetireUpdate, AccountResourceKind, AccountResourcePage, PublicationPhoto, PublicationPhotoMutation, PublicationSchedule, PublicationScheduleUpdate } from '../global-admin-models';
import type { KPIReport, AuditFilter, AuditPage } from '../global-admin-models';

export abstract class GlobalAdminPort {
  abstract overview(token: string): Promise<Overview>;
  abstract kpis(token:string,period:{from:string;until:string;time_zone:string}):Promise<KPIReport>;
  abstract audit(token:string,filters:AuditFilter):Promise<AuditPage>;
  abstract exportAudit(token:string,filters:AuditFilter,format:'json'|'csv'):Promise<Blob>;
  abstract list(
    token: string,
    filters: Record<string, string | number | undefined>,
  ): Promise<Page>;
  abstract account(token: string, id: string): Promise<Account>;
  abstract listAccountResources(token: string, id: string, kind: AccountResourceKind, cursor?: string): Promise<AccountResourcePage>;
  abstract accountResource(token: string, accountId: string, kind: AccountResourceKind, resourceId: string): Promise<Record<string, unknown>>;
  abstract updateProfile(token: string, id: string, input: ProfileUpdate): Promise<ProfileUpdateResult>;
  abstract updateRole(token: string, id: string, role: string, input: RoleUpdate): Promise<void>;
  abstract revokeSessions(token: string, id: string, input: SessionRevocation): Promise<number>;
  abstract listPublications(token:string,filters:Record<string,string|number|undefined>):Promise<PublicationPage>;
  abstract setPublicationState(token:string,id:string,input:PublicationStateUpdate):Promise<void>;
  abstract updatePublication(token:string,id:string,input:PublicationContentUpdate):Promise<void>;
  abstract retirePublication(token:string,id:string,input:PublicationRetireUpdate):Promise<Record<string, unknown>>;
  abstract publicationGallery(token:string,id:string,ownerId:string):Promise<PublicationPhoto[]>;
  abstract setPublicationPhotoVisibility(token:string,id:string,photoId:string,input:PublicationPhotoMutation):Promise<void>;
  abstract selectPublicationCover(token:string,id:string,photoId:string,input:PublicationPhotoMutation):Promise<void>;
  abstract removePublicationPhoto(token:string,id:string,photoId:string,input:PublicationPhotoMutation):Promise<void>;
  abstract publicationSchedule(token:string,id:string,ownerId:string):Promise<PublicationSchedule>;
  abstract savePublicationSchedule(token:string,id:string,input:PublicationScheduleUpdate):Promise<void>;
  abstract setBlocked(token: string,id:string,input:AccountBlockUpdate):Promise<void>;
}
