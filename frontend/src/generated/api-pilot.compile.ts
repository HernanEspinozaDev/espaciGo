import type {
  DownloadCompleteOwnDataArchiveData,
  GetOwnProfileData,
  LoginData,
  SearchAuthorizedLocalSyntheticCatalogData,
} from './api/types.gen';

export interface ApiPilotContracts {
  login: LoginData;
  ownProfile: GetOwnProfileData;
  authorizedCatalog: SearchAuthorizedLocalSyntheticCatalogData;
  privateArchive: DownloadCompleteOwnDataArchiveData;
}
