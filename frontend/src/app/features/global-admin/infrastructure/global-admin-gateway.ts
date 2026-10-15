import { Injectable, inject } from '@angular/core';
import { GlobalAdminPort } from '../domain/ports/global-admin-port';
import { GlobalAdminApi } from './global-admin-api';

@Injectable({ providedIn: 'root' })
export class GlobalAdminGateway extends GlobalAdminPort {
  private readonly api = inject(GlobalAdminApi);

  override overview(token: string) {
    return this.api.overview(token);
  }

  override list(token: string, filters: Record<string, string | number | undefined>) {
    return this.api.list(token, filters);
  }

  override account(token: string, id: string) {
    return this.api.account(token, id);
  }

  override listAccountResources(token: string, id: string, kind: Parameters<GlobalAdminPort['listAccountResources']>[2], cursor = '') {
    return this.api.listAccountResources(token, id, kind, cursor);
  }

  override accountResource(token: string, accountId: string, kind: Parameters<GlobalAdminPort['accountResource']>[2], resourceId: string) {
    return this.api.accountResource(token, accountId, kind, resourceId);
  }

  override updateProfile(token: string, id: string, input: Parameters<GlobalAdminPort['updateProfile']>[2]) { return this.api.updateProfile(token,id,input); }
  override updateRole(token: string, id: string, role: string, input: Parameters<GlobalAdminPort['updateRole']>[3]) { return this.api.updateRole(token,id,role,input); }
  override revokeSessions(token: string, id: string, input: Parameters<GlobalAdminPort['revokeSessions']>[2]) { return this.api.revokeSessions(token,id,input); }
  override setBlocked(token:string,id:string,input:Parameters<GlobalAdminPort['setBlocked']>[2]) { return this.api.setBlocked(token,id,input); }
  override listPublications(token:string,filters:Record<string,string|number|undefined>){return this.api.listPublications(token,filters);}
  override setPublicationState(token:string,id:string,input:Parameters<GlobalAdminPort['setPublicationState']>[2]){return this.api.setPublicationState(token,id,input);}
  override updatePublication(token:string,id:string,input:Parameters<GlobalAdminPort['updatePublication']>[2]){return this.api.updatePublication(token,id,input);}
  override retirePublication(token:string,id:string,input:Parameters<GlobalAdminPort['retirePublication']>[2]){return this.api.retirePublication(token,id,input);}
  override publicationGallery(token:string,id:string,ownerId:string){return this.api.publicationGallery(token,id,ownerId);}
  override setPublicationPhotoVisibility(token:string,id:string,photoId:string,input:Parameters<GlobalAdminPort['setPublicationPhotoVisibility']>[3]){return this.api.setPublicationPhotoVisibility(token,id,photoId,input);}
  override selectPublicationCover(token:string,id:string,photoId:string,input:Parameters<GlobalAdminPort['selectPublicationCover']>[3]){return this.api.selectPublicationCover(token,id,photoId,input);}
  override removePublicationPhoto(token:string,id:string,photoId:string,input:Parameters<GlobalAdminPort['removePublicationPhoto']>[3]){return this.api.removePublicationPhoto(token,id,photoId,input);}
  override publicationSchedule(token:string,id:string,ownerId:string){return this.api.publicationSchedule(token,id,ownerId);}
  override savePublicationSchedule(token:string,id:string,input:Parameters<GlobalAdminPort['savePublicationSchedule']>[2]){return this.api.savePublicationSchedule(token,id,input);}
}
